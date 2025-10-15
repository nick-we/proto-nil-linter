package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// grpcAnalyzer identifies gRPC service handler methods
type grpcAnalyzer struct {
	pass          *analysis.Pass
	protoAnalyzer *protoAnalyzer

	// Map of function declarations that are gRPC handlers
	// Key: function name, Value: response type information
	handlers map[*ast.FuncDecl]*handlerInfo

	// Map of helper functions that should also be checked
	// Functions called from handlers or that return proto responses
	helpers map[*ast.FuncDecl]bool
}

// handlerInfo contains information about a gRPC handler
type handlerInfo struct {
	funcDecl     *ast.FuncDecl
	responseType types.Type
	responseName string
	isHandler    bool
}

func newGRPCAnalyzer(pass *analysis.Pass, pa *protoAnalyzer) *grpcAnalyzer {
	return &grpcAnalyzer{
		pass:          pass,
		protoAnalyzer: pa,
		handlers:      make(map[*ast.FuncDecl]*handlerInfo),
		helpers:       make(map[*ast.FuncDecl]bool),
	}
}

// visit analyzes AST nodes to identify gRPC service handlers and helpers
func (ga *grpcAnalyzer) visit(n ast.Node) {
	funcDecl, ok := n.(*ast.FuncDecl)
	if !ok {
		return
	}

	// Check if this is a gRPC handler method
	if ga.isGRPCHandler(funcDecl) {
		// Extract response type information
		info := ga.extractHandlerInfo(funcDecl)
		if info != nil {
			ga.handlers[funcDecl] = info
		}
		return
	}

	// Check if this is a helper that returns a proto message
	if ga.returnsProtoMessage(funcDecl) {
		ga.helpers[funcDecl] = true
	}
}

// returnsProtoMessage checks if a function returns a proto message
func (ga *grpcAnalyzer) returnsProtoMessage(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}

	// Check if any return value is a pointer to what could be a proto message
	for _, result := range fn.Type.Results.List {
		if ga.isProtoPointerType(result.Type) {
			return true
		}
	}

	return false
}

// isHandlerOrHelper checks if a function is a handler or helper
func (ga *grpcAnalyzer) isHandlerOrHelper(fn *ast.FuncDecl) bool {
	return ga.isHandler(fn) || ga.helpers[fn]
}

// isGRPCHandler checks if a function is a gRPC service handler
func (ga *grpcAnalyzer) isGRPCHandler(fn *ast.FuncDecl) bool {
	// gRPC handlers have the signature:
	// func (s *Service) MethodName(ctx context.Context, req *RequestType) (*ResponseType, error)

	// Must be a method (have a receiver)
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}

	// Must have exactly 2 parameters
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 2 {
		return false
	}

	// Must have exactly 2 return values
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 2 {
		return false
	}

	// First parameter should be context.Context
	firstParam := fn.Type.Params.List[0]
	if !ga.isContextType(firstParam.Type) {
		return false
	}

	// Second parameter should be a pointer to a proto message (request)
	secondParam := fn.Type.Params.List[1]
	if !ga.isProtoPointerType(secondParam.Type) {
		return false
	}

	// First return value should be a pointer to a proto message (response)
	firstReturn := fn.Type.Results.List[0]
	if !ga.isProtoPointerType(firstReturn.Type) {
		return false
	}

	// Second return value should be error
	secondReturn := fn.Type.Results.List[1]
	return ga.isErrorType(secondReturn.Type)
}

// extractHandlerInfo extracts information about a gRPC handler
func (ga *grpcAnalyzer) extractHandlerInfo(fn *ast.FuncDecl) *handlerInfo {
	if fn.Type.Results == nil || len(fn.Type.Results.List) < 1 {
		return nil
	}

	// Get the response type (first return value)
	responseExpr := fn.Type.Results.List[0].Type

	// It should be a pointer type
	starExpr, ok := responseExpr.(*ast.StarExpr)
	if !ok {
		return nil
	}

	// Get the actual type
	var responseTypeName string
	switch t := starExpr.X.(type) {
	case *ast.Ident:
		responseTypeName = t.Name
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			responseTypeName = ident.Name + "." + t.Sel.Name
		}
	}

	// Get the actual type from type info
	var responseType types.Type
	if obj := ga.pass.TypesInfo.Uses[extractIdent(starExpr.X)]; obj != nil {
		responseType = obj.Type()
	}

	return &handlerInfo{
		funcDecl:     fn,
		responseType: responseType,
		responseName: responseTypeName,
		isHandler:    true,
	}
}

// isContextType checks if the expression is context.Context
func (ga *grpcAnalyzer) isContextType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "context" && sel.Sel.Name == "Context"
}

// isErrorType checks if the expression is error type
func (ga *grpcAnalyzer) isErrorType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "error"
}

// isProtoPointerType checks if the expression is a pointer to a proto message
func (ga *grpcAnalyzer) isProtoPointerType(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}

	// Check if it's likely a proto message based on naming
	switch t := star.X.(type) {
	case *ast.Ident:
		// Simple type name - should start with capital letter
		return len(t.Name) > 0 && t.Name[0] >= 'A' && t.Name[0] <= 'Z'
	case *ast.SelectorExpr:
		// Qualified name like pb.Request
		return true
	}

	return false
}

// isHandler checks if a function declaration is a gRPC handler
func (ga *grpcAnalyzer) isHandler(fn *ast.FuncDecl) bool {
	_, exists := ga.handlers[fn]
	return exists
}

// getHandlerInfo retrieves handler information
func (ga *grpcAnalyzer) getHandlerInfo(fn *ast.FuncDecl) *handlerInfo {
	return ga.handlers[fn]
}

// extractIdent extracts an identifier from an expression
func extractIdent(expr ast.Expr) *ast.Ident {
	switch t := expr.(type) {
	case *ast.Ident:
		return t
	case *ast.SelectorExpr:
		return t.Sel
	}
	return nil
}
