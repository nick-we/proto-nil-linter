package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// protoAnalyzer identifies proto3 message types and their field optionality
type protoAnalyzer struct {
	pass *analysis.Pass

	// Map of type name to field information
	protoMessages map[string]*protoMessageInfo

	// Types that implement proto.Message
	protoTypes map[types.Type]bool
}

// protoMessageInfo contains information about a proto message
type protoMessageInfo struct {
	typeName string
	fields   map[string]*protoFieldInfo
}

// protoFieldInfo contains information about a proto field
type protoFieldInfo struct {
	name       string
	typeName   string
	isOptional bool
	isRepeated bool
	isMessage  bool // true if the field is itself a proto message
	nestedType string
}

func newProtoAnalyzer(pass *analysis.Pass) *protoAnalyzer {
	return &protoAnalyzer{
		pass:          pass,
		protoMessages: make(map[string]*protoMessageInfo),
		protoTypes:    make(map[types.Type]bool),
	}
}

// visit analyzes AST nodes to identify proto message types
func (pa *protoAnalyzer) visit(n ast.Node) {
	// Look for type definitions that might be proto messages
	typeSpec, ok := n.(*ast.TypeSpec)
	if !ok {
		return
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return
	}

	// Check if this struct has proto-generated characteristics
	if !pa.isProtoMessage(structType) {
		return
	}

	// Extract field information
	msgInfo := &protoMessageInfo{
		typeName: typeSpec.Name.Name,
		fields:   make(map[string]*protoFieldInfo),
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		fieldInfo := pa.extractFieldInfo(fieldName, field)
		if fieldInfo != nil {
			msgInfo.fields[fieldName] = fieldInfo
		}
	}

	pa.protoMessages[msgInfo.typeName] = msgInfo

	// Also track the type itself
	if obj := pa.pass.TypesInfo.Defs[typeSpec.Name]; obj != nil {
		pa.protoTypes[obj.Type()] = true
	}
}

// isProtoMessage heuristically determines if a struct is a proto message
func (pa *protoAnalyzer) isProtoMessage(st *ast.StructType) bool {
	// Proto-generated structs typically have:
	// 1. unexported fields starting with "state", "sizeCache", "unknownFields"
	// 2. A XXX_ prefix on some fields
	// 3. protoimpl.MessageState field

	hasProtoFields := false
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Check for embedded fields
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "protoimpl" && sel.Sel.Name == "MessageState" {
						return true
					}
				}
			}
			continue
		}

		fieldName := field.Names[0].Name
		if fieldName == "state" || fieldName == "sizeCache" ||
			fieldName == "unknownFields" || (len(fieldName) >= 3 && fieldName[:3] == "XXX") {
			hasProtoFields = true
		}
	}

	return hasProtoFields
}

// extractFieldInfo extracts information about a proto field
func (pa *protoAnalyzer) extractFieldInfo(name string, field *ast.Field) *protoFieldInfo {
	info := &protoFieldInfo{
		name: name,
	}

	// Skip proto-internal fields
	if name == "state" || name == "sizeCache" || name == "unknownFields" ||
		(len(name) >= 3 && name[:3] == "XXX") {
		return nil
	}

	// Determine field type and characteristics
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		// Pointer type - could be optional or a message type
		info.typeName = pa.extractTypeName(t.X)
		info.isOptional = pa.isOptionalField(field)
		info.isMessage = pa.couldBeMessageType(t.X)
		if info.isMessage {
			info.nestedType = info.typeName
		}

	case *ast.ArrayType:
		// Repeated field (slice)
		info.isRepeated = true
		info.typeName = pa.extractTypeName(t.Elt)

		// Check if it's a slice of message pointers
		if star, ok := t.Elt.(*ast.StarExpr); ok {
			info.isMessage = pa.couldBeMessageType(star.X)
			if info.isMessage {
				info.nestedType = pa.extractTypeName(star.X)
			}
		}

	case *ast.Ident:
		// Direct type (scalar)
		info.typeName = t.Name

	case *ast.SelectorExpr:
		// Qualified type (e.g., pb.SomeType)
		info.typeName = pa.extractTypeName(t)
		info.isMessage = true
		info.nestedType = info.typeName
	}

	return info
}

// isOptionalField checks proto tags to determine if field is optional
func (pa *protoAnalyzer) isOptionalField(field *ast.Field) bool {
	if field.Tag == nil {
		return false
	}

	tag := field.Tag.Value
	// Check for protobuf tags with "optional" keyword
	// Proto3 optional fields have specific tag patterns
	return contains(tag, "optional")
}

// couldBeMessageType checks if a type could be a proto message
func (pa *protoAnalyzer) couldBeMessageType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		// Capital letter typically indicates a message type
		return len(t.Name) > 0 && t.Name[0] >= 'A' && t.Name[0] <= 'Z'
	case *ast.SelectorExpr:
		// Qualified names like pb.User
		return true
	}
	return false
}

// extractTypeName extracts the type name from an AST expression
func (pa *protoAnalyzer) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return pa.extractTypeName(t.X)
	}
	return ""
}

// isProtoType checks if a given type is a proto message
func (pa *protoAnalyzer) isProtoType(t types.Type) bool {
	// Remove pointer if present
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	return pa.protoTypes[t]
}

// getMessageInfo retrieves proto message information by type name
func (pa *protoAnalyzer) getMessageInfo(typeName string) *protoMessageInfo {
	return pa.protoMessages[typeName]
}

// getFieldInfo retrieves field information for a proto message field
func (pa *protoAnalyzer) getFieldInfo(typeName, fieldName string) *protoFieldInfo {
	msgInfo := pa.protoMessages[typeName]
	if msgInfo == nil {
		return nil
	}
	return msgInfo.fields[fieldName]
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
