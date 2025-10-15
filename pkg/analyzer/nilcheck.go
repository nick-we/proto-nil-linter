package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// nilChecker checks for nil assignments to proto message fields
type nilChecker struct {
	pass          *analysis.Pass
	protoAnalyzer *protoAnalyzer
	grpcAnalyzer  *grpcAnalyzer

	// Track current function context
	currentFunc *ast.FuncDecl

	// Track variables that might be nil
	nilVars map[string]bool

	// Track nesting depth for context messages
	nestingDepth int

	// Advanced data flow analysis
	summaryCache *SummaryCache

	// Path-sensitive analysis
	pathAnalyzer *PathAnalyzer
}

func newNilChecker(pass *analysis.Pass, pa *protoAnalyzer, ga *grpcAnalyzer) *nilChecker {
	return &nilChecker{
		pass:          pass,
		protoAnalyzer: pa,
		grpcAnalyzer:  ga,
		nilVars:       make(map[string]bool),
		summaryCache:  newSummaryCache(pass),
		pathAnalyzer:  newPathAnalyzer(),
	}
}

// visit analyzes AST nodes to check for nil assignments
func (nc *nilChecker) visit(n ast.Node) {
	switch node := n.(type) {
	case *ast.FuncDecl:
		nc.currentFunc = node
		nc.nilVars = make(map[string]bool)  // Reset for new function
		nc.pathAnalyzer = newPathAnalyzer() // Reset path analyzer

		// Analyze function body with path-sensitive analysis
		if node.Body != nil && nc.grpcAnalyzer.isHandler(node) {
			nc.pathAnalyzer.analyzeBlockStmt(node.Body, nc)
		}

	case *ast.DeclStmt:
		nc.checkDecl(node)

	case *ast.AssignStmt:
		nc.checkAssignment(node)

	case *ast.ReturnStmt:
		nc.checkReturn(node)

	case *ast.CompositeLit:
		nc.checkCompositeLit(node)
	}
}

// checkDecl checks variable declarations for nil initialization
func (nc *nilChecker) checkDecl(decl *ast.DeclStmt) {
	// Only check if we're inside a gRPC handler
	if nc.currentFunc == nil || !nc.grpcAnalyzer.isHandler(nc.currentFunc) {
		return
	}

	genDecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return
	}

	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		// Check each variable in the declaration
		for i, name := range valueSpec.Names {
			if i < len(valueSpec.Values) {
				value := valueSpec.Values[i]
				if nc.isNilValue(value) {
					nc.nilVars[name.Name] = true
				}
			}
		}
	}
}

// checkAssignment checks for nil assignments to proto fields
func (nc *nilChecker) checkAssignment(assign *ast.AssignStmt) {
	// Only check if we're inside a gRPC handler
	if nc.currentFunc == nil || !nc.grpcAnalyzer.isHandler(nc.currentFunc) {
		return
	}

	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			continue
		}
		rhs := assign.Rhs[i]

		// Track variables assigned to nil
		if ident, ok := lhs.(*ast.Ident); ok {
			if nc.isNilValue(rhs) {
				nc.nilVars[ident.Name] = true
			} else {
				// Variable is no longer nil
				delete(nc.nilVars, ident.Name)
			}
		}

		// Check for nil assignment to field
		if nc.isNilValue(rhs) {
			nc.checkNilAssignmentTarget(lhs, assign.Pos())
		}

		// Check if RHS is a composite literal (slice of protos or proto message)
		if comp, ok := rhs.(*ast.CompositeLit); ok {
			// Check the composite literal for issues
			nc.nestingDepth = 0
			nc.checkCompositeLitFields(comp, comp.Pos())
		}
	}
}

// checkReturn checks return statements for nil in proto message fields
func (nc *nilChecker) checkReturn(ret *ast.ReturnStmt) {
	// Only check if we're inside a gRPC handler
	if nc.currentFunc == nil || !nc.grpcAnalyzer.isHandler(nc.currentFunc) {
		return
	}

	if len(ret.Results) == 0 {
		return
	}

	// First return value should be the response
	responseExpr := ret.Results[0]

	// Check if it's a composite literal
	if comp, ok := responseExpr.(*ast.UnaryExpr); ok {
		if comp.Op == token.AND {
			if compLit, ok := comp.X.(*ast.CompositeLit); ok {
				nc.nestingDepth = 0
				nc.checkCompositeLitFields(compLit, ret.Pos())
			}
		}
	}
}

// checkCompositeLit checks composite literals for nil values in proto fields
func (nc *nilChecker) checkCompositeLit(comp *ast.CompositeLit) {
	// Only check if we're inside a gRPC handler
	if nc.currentFunc == nil || !nc.grpcAnalyzer.isHandler(nc.currentFunc) {
		return
	}

	// Don't check here - let checkReturn and checkNestedMessage handle it
	// to avoid duplicate reports
}

// checkCompositeLitFields checks fields in a composite literal
func (nc *nilChecker) checkCompositeLitFields(comp *ast.CompositeLit, pos token.Pos) {
	// Get the actual type from type info
	compType := nc.pass.TypesInfo.TypeOf(comp)
	if compType == nil {
		return
	}

	// Remove pointer if present
	if ptr, ok := compType.(*types.Pointer); ok {
		compType = ptr.Elem()
	}

	// Check if it's a slice type
	if slice, ok := compType.(*types.Slice); ok {
		// Check if it's a slice of proto messages
		elemType := slice.Elem()
		if ptr, ok := elemType.(*types.Pointer); ok {
			elemType = ptr.Elem()
		}

		// Check each element in the slice
		for i, elt := range comp.Elts {
			if nc.isNilValue(elt) {
				// Get the parent field name if possible
				nc.pass.Reportf(
					elt.Pos(),
					"nil item at index %d in repeated field",
					i,
				)
			} else {
				// Recursively check the element if it's a composite literal
				nc.nestingDepth++
				nc.checkNestedMessage(elt, "")
				nc.nestingDepth--
			}
		}
		return
	}

	// Check if it's a proto struct type
	structType, ok := compType.(*types.Struct)
	if !ok {
		// Try named type
		if named, ok := compType.(*types.Named); ok {
			structType, ok = named.Underlying().(*types.Struct)
			if !ok {
				return
			}
		} else {
			return
		}
	}

	// Check if this type is a proto message (checks interface impl + heuristics)
	if !nc.isProtoType(compType) {
		return
	}

	typeName := nc.getTypeName(compType)

	// Determine context based on nesting
	context := "composite literal"
	if nc.nestingDepth > 0 {
		context = "nested message"
	}

	// Track which fields are explicitly set
	explicitFields := make(map[string]bool)

	// Check each field in the composite literal
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		// Get field name
		fieldIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		fieldName := fieldIdent.Name

		// Mark field as explicitly set
		explicitFields[fieldName] = true

		// Find the field in the struct
		fieldInfo := nc.getStructFieldInfo(structType, fieldName)
		if fieldInfo == nil {
			continue
		}

		// Check for nil value
		if nc.isNilValue(kv.Value) {
			if !fieldInfo.isOptional {
				nc.reportNilAssignment(kv.Pos(), typeName, fieldName, context)
			}
		} else if fieldInfo.isRepeated {
			// Check repeated field for nil items
			nc.checkRepeatedField(kv.Value, typeName, fieldName, fieldInfo)
		} else if fieldInfo.isMessage {
			// Check nested message fields - increase nesting depth
			nc.nestingDepth++
			nc.checkNestedMessage(kv.Value, fieldInfo.nestedType)
			nc.nestingDepth--
		}
	}

	// Check for implicitly nil fields (omitted non-optional fields)
	nc.checkMissingFields(structType, explicitFields, typeName, comp.Pos(), context)
}

// checkMissingFields checks for required fields that are missing from composite literal
func (nc *nilChecker) checkMissingFields(structType *types.Struct, explicitFields map[string]bool, typeName string, pos token.Pos, context string) {
	// Iterate through all struct fields
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		fieldName := field.Name()

		// Skip proto-internal fields
		if fieldName == "state" || fieldName == "sizeCache" || fieldName == "unknownFields" ||
			(len(fieldName) >= 3 && fieldName[:3] == "XXX") {
			continue
		}

		// Skip if field was explicitly set
		if explicitFields[fieldName] {
			continue
		}

		// Get field info
		fieldInfo := nc.getStructFieldInfo(structType, fieldName)
		if fieldInfo == nil {
			continue
		}

		// Check if field is non-optional and a message pointer (would be implicitly nil)
		// Note: Repeated fields (slices) being nil is OK in proto3 (treated as empty)
		if !fieldInfo.isOptional && fieldInfo.isMessage && !fieldInfo.isRepeated {
			nc.pass.Reportf(
				pos,
				"missing initialization of non-optional proto field %s.%s in %s (field is implicitly nil)",
				typeName, fieldName, context,
			)
		}
	}
}

// checkRepeatedField checks repeated/slice fields for nil items
func (nc *nilChecker) checkRepeatedField(expr ast.Expr, msgType, fieldName string, fieldInfo *protoFieldInfo) {
	// Handle composite literal for slice
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		// Check if it's an identifier (variable)
		if ident, ok := expr.(*ast.Ident); ok {
			// Try to find the variable definition
			// For now, we can't easily trace variable slice contents
			_ = ident
		}
		return
	}

	// Check each element in the slice
	for i, elt := range comp.Elts {
		if nc.isNilValue(elt) {
			nc.pass.Reportf(
				elt.Pos(),
				"nil item at index %d in repeated field %s.%s",
				i, msgType, fieldName,
			)
		} else if fieldInfo.isMessage {
			// If the repeated field contains messages, check nested fields
			nc.nestingDepth++
			nc.checkNestedMessage(elt, fieldInfo.nestedType)
			nc.nestingDepth--
		}
	}
}

// checkNestedMessage recursively checks nested message fields
func (nc *nilChecker) checkNestedMessage(expr ast.Expr, typeName string) {
	// Handle &Type{...} pattern
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		if unary.Op == token.AND {
			expr = unary.X
		}
	}

	// Must be a composite literal
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}

	// Use the type-based checking
	nc.checkCompositeLitFields(comp, comp.Pos())
}

// checkNilAssignmentTarget checks if a nil assignment target is a proto field
func (nc *nilChecker) checkNilAssignmentTarget(expr ast.Expr, pos token.Pos) {
	// Handle field selector (e.g., resp.User = nil)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return
	}

	fieldName := sel.Sel.Name

	// Get the type of the base expression
	baseType := nc.pass.TypesInfo.TypeOf(sel.X)
	if baseType == nil {
		return
	}

	// Remove pointer if present
	if ptr, ok := baseType.(*types.Pointer); ok {
		baseType = ptr.Elem()
	}

	// Check if it's a proto message type
	if !nc.isProtoType(baseType) {
		return
	}

	// Get the type name
	typeName := nc.getTypeName(baseType)

	// Get struct type
	structType, ok := baseType.(*types.Struct)
	if !ok {
		if named, ok := baseType.(*types.Named); ok {
			structType, ok = named.Underlying().(*types.Struct)
		}
	}

	if structType == nil {
		return
	}

	fieldInfo := nc.getStructFieldInfo(structType, fieldName)

	if fieldInfo != nil && !fieldInfo.isOptional {
		nc.reportNilAssignment(pos, typeName, fieldName, "direct assignment")
	}
}

// isProtoType checks if a type is a proto message
func (nc *nilChecker) isProtoType(t types.Type) bool {
	// First check if it implements proto.Message interface
	if nc.implementsProtoMessage(t) {
		return true
	}

	// Fallback to heuristic check
	structType, ok := t.(*types.Struct)
	if !ok {
		if named, ok := t.(*types.Named); ok {
			structType, ok = named.Underlying().(*types.Struct)
		}
	}

	if structType == nil {
		return false
	}

	return nc.isProtoStruct(structType)
}

// implementsProtoMessage checks if a type implements proto.Message interface
func (nc *nilChecker) implementsProtoMessage(t types.Type) bool {
	// Look for proto.Message interface in common proto packages
	protoPackages := []string{
		"google.golang.org/protobuf/proto",
		"github.com/golang/protobuf/proto",
	}

	for _, pkgPath := range protoPackages {
		// Try to find the proto package
		for _, pkg := range nc.pass.Pkg.Imports() {
			if pkg.Path() == pkgPath {
				// Look for Message interface
				obj := pkg.Scope().Lookup("Message")
				if obj == nil {
					continue
				}

				// Check if it's a type name for an interface
				if typeName, ok := obj.(*types.TypeName); ok {
					if iface, ok := typeName.Type().Underlying().(*types.Interface); ok {
						// Check if our type implements this interface
						if types.Implements(t, iface) {
							return true
						}
						// Also check pointer type
						if ptr, ok := t.(*types.Pointer); !ok {
							if types.Implements(types.NewPointer(t), iface) {
								return true
							}
						} else {
							if types.Implements(ptr, iface) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// isNilValue checks if an expression is a nil value
func (nc *nilChecker) isNilValue(expr ast.Expr) bool {
	// Direct nil identifier
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "nil" {
			return true
		}

		// Check path-sensitive state first (most accurate)
		if nc.pathAnalyzer != nil {
			state := nc.pathAnalyzer.getVarState(ident.Name)
			switch state {
			case NilStateAlwaysNil:
				return true
			case NilStateNeverNil:
				return false
			}
		}

		// Fall back to simple tracking
		return nc.nilVars[ident.Name]
	}

	// Typed nil: (*Type)(nil)
	if call, ok := expr.(*ast.CallExpr); ok {
		if len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok {
				if ident.Name == "nil" {
					return true
				}
			}
		}

		// Advanced: Check if function call returns nil using summary cache
		state := nc.summaryCache.analyzeExpr(call)
		if state == NilStateAlwaysNil {
			return true
		}
		// For MaybeNil, we could warn but not error
		// For now, conservatively treat MaybeNil as potentially nil
	}

	return false
}

// extractTypeName extracts type name from various AST expressions
func (nc *nilChecker) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		// Just return the selector name (e.g., "User" from "pb.User")
		// since proto analyzer stores names without package prefix
		return t.Sel.Name
	case *ast.StarExpr:
		return nc.extractTypeName(t.X)
	}
	return ""
}

// getTypeName gets the string name of a type
func (nc *nilChecker) getTypeName(t types.Type) string {
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		// Just return the type name without package prefix
		// since proto analyzer stores names without package
		return obj.Name()
	}
	return ""
}

// isProtoStruct checks if a struct type has proto characteristics
func (nc *nilChecker) isProtoStruct(st *types.Struct) bool {
	// Check for proto-generated field names
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		name := field.Name()
		if name == "state" || name == "sizeCache" || name == "unknownFields" {
			return true
		}
	}
	return false
}

// getStructFieldInfo extracts field information from a struct type
func (nc *nilChecker) getStructFieldInfo(st *types.Struct, fieldName string) *protoFieldInfo {
	// Skip proto-internal fields
	if fieldName == "state" || fieldName == "sizeCache" || fieldName == "unknownFields" ||
		(len(fieldName) >= 3 && fieldName[:3] == "XXX") {
		return nil
	}

	// Find the field
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field.Name() == fieldName {
			info := &protoFieldInfo{
				name: fieldName,
			}

			fieldType := field.Type()

			// Check if it's a pointer (might be optional or message)
			if ptr, ok := fieldType.(*types.Pointer); ok {
				info.isMessage = true
				info.nestedType = nc.getTypeName(ptr.Elem())
				info.typeName = info.nestedType
				// Check proto tags for optional
				info.isOptional = nc.hasOptionalTag(st, i)
			} else if slice, ok := fieldType.(*types.Slice); ok {
				// Repeated field
				info.isRepeated = true
				if ptr, ok := slice.Elem().(*types.Pointer); ok {
					info.isMessage = true
					info.nestedType = nc.getTypeName(ptr.Elem())
					info.typeName = info.nestedType
				}
			} else {
				info.typeName = fieldType.String()
			}

			return info
		}
	}
	return nil
}

// hasOptionalTag checks if a field has the optional protobuf tag
func (nc *nilChecker) hasOptionalTag(st *types.Struct, fieldIndex int) bool {
	tag := st.Tag(fieldIndex)
	return contains(tag, "optional")
}

// reportNilAssignment reports a nil assignment to a non-optional field
func (nc *nilChecker) reportNilAssignment(pos token.Pos, msgType, fieldName, context string) {
	nc.pass.Reportf(
		pos,
		"nil assignment to non-optional proto field %s.%s in %s (gRPC handler response must not contain nil)",
		msgType, fieldName, context,
	)
}
