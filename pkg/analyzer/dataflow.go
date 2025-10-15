package analyzer

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// NilState represents the nil state of a value
type NilState int

const (
	// NilStateUnknown means we don't know the nil state
	NilStateUnknown NilState = iota
	// NilStateAlwaysNil means the value is always nil
	NilStateAlwaysNil
	// NilStateNeverNil means the value is never nil
	NilStateNeverNil
	// NilStateMaybeNil means the value may or may not be nil
	NilStateMaybeNil
)

// String returns the string representation of NilState
func (n NilState) String() string {
	switch n {
	case NilStateUnknown:
		return "Unknown"
	case NilStateAlwaysNil:
		return "AlwaysNil"
	case NilStateNeverNil:
		return "NeverNil"
	case NilStateMaybeNil:
		return "MaybeNil"
	default:
		return "Unknown"
	}
}

// FunctionSummary contains information about a function's nil behavior
type FunctionSummary struct {
	funcDecl    *ast.FuncDecl
	funcType    *types.Func
	returnState NilState
	analyzed    bool
}

// SummaryCache caches function summaries for reuse
type SummaryCache struct {
	pass      *analysis.Pass
	summaries map[*types.Func]*FunctionSummary
	analyzing map[*types.Func]bool // Detect recursion
}

// newSummaryCache creates a new summary cache
func newSummaryCache(pass *analysis.Pass) *SummaryCache {
	return &SummaryCache{
		pass:      pass,
		summaries: make(map[*types.Func]*FunctionSummary),
		analyzing: make(map[*types.Func]bool),
	}
}

// getSummary gets or creates a summary for a function
func (sc *SummaryCache) getSummary(fn *types.Func, decl *ast.FuncDecl) *FunctionSummary {
	// Check cache
	if summary, exists := sc.summaries[fn]; exists {
		return summary
	}

	// Check for recursion
	if sc.analyzing[fn] {
		// Recursive call - return conservative estimate
		return &FunctionSummary{
			funcDecl:    decl,
			funcType:    fn,
			returnState: NilStateMaybeNil,
			analyzed:    true,
		}
	}

	// Mark as analyzing
	sc.analyzing[fn] = true
	defer delete(sc.analyzing, fn)

	// Analyze the function
	summary := sc.analyzeFunction(decl, fn)
	sc.summaries[fn] = summary
	return summary
}

// analyzeFunction analyzes a function to determine its nil return behavior
func (sc *SummaryCache) analyzeFunction(decl *ast.FuncDecl, fn *types.Func) *FunctionSummary {
	summary := &FunctionSummary{
		funcDecl:    decl,
		funcType:    fn,
		returnState: NilStateUnknown,
		analyzed:    false,
	}

	if decl == nil || decl.Body == nil {
		// External function or interface method
		summary.returnState = NilStateUnknown
		summary.analyzed = true
		return summary
	}

	// Check if function returns a pointer type
	sig := fn.Type().(*types.Signature)
	results := sig.Results()
	if results.Len() == 0 {
		summary.returnState = NilStateNeverNil
		summary.analyzed = true
		return summary
	}

	// We're interested in the first return value (typically the response)
	firstResult := results.At(0).Type()

	// Check if it's a pointer type
	if _, ok := firstResult.(*types.Pointer); !ok {
		// Not a pointer, can't be nil
		summary.returnState = NilStateNeverNil
		summary.analyzed = true
		return summary
	}

	// Analyze all return statements
	returnStates := sc.analyzeReturnStatements(decl.Body)

	if len(returnStates) == 0 {
		// No explicit returns (might panic or infinite loop)
		summary.returnState = NilStateUnknown
	} else {
		// Combine return states
		summary.returnState = combineNilStates(returnStates)
	}

	summary.analyzed = true
	return summary
}

// analyzeReturnStatements analyzes all return statements in a function body
func (sc *SummaryCache) analyzeReturnStatements(body *ast.BlockStmt) []NilState {
	states := []NilState{}

	ast.Inspect(body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		if len(ret.Results) == 0 {
			return true
		}

		// Analyze first return value
		state := sc.analyzeExpr(ret.Results[0])
		states = append(states, state)

		return true
	})

	return states
}

// analyzeExpr analyzes an expression to determine its nil state
func (sc *SummaryCache) analyzeExpr(expr ast.Expr) NilState {
	switch e := expr.(type) {
	case *ast.Ident:
		// Check if it's nil literal
		if e.Name == "nil" {
			return NilStateAlwaysNil
		}
		// Otherwise unknown (could track variables)
		return NilStateUnknown

	case *ast.UnaryExpr:
		if e.Op == 38 { // token.AND (&)
			// Taking address of something - never nil
			return NilStateNeverNil
		}
		return sc.analyzeExpr(e.X)

	case *ast.CompositeLit:
		// Composite literal is never nil
		return NilStateNeverNil

	case *ast.CallExpr:
		// Function call - need to analyze the called function
		return sc.analyzeCallExpr(e)

	case *ast.SelectorExpr:
		// Field or method access
		return NilStateUnknown

	case *ast.TypeAssertExpr:
		// Type assertion
		return NilStateUnknown

	case *ast.ParenExpr:
		return sc.analyzeExpr(e.X)

	default:
		return NilStateUnknown
	}
}

// analyzeCallExpr analyzes a function call expression
func (sc *SummaryCache) analyzeCallExpr(call *ast.CallExpr) NilState {
	// Get the function being called
	var fn *types.Func

	switch f := call.Fun.(type) {
	case *ast.Ident:
		// Direct function call
		if obj := sc.pass.TypesInfo.Uses[f]; obj != nil {
			if funcObj, ok := obj.(*types.Func); ok {
				fn = funcObj
			}
		}

	case *ast.SelectorExpr:
		// Method call or qualified function
		if obj := sc.pass.TypesInfo.Uses[f.Sel]; obj != nil {
			if funcObj, ok := obj.(*types.Func); ok {
				fn = funcObj
			}
		}
	}

	if fn == nil {
		return NilStateUnknown
	}

	// Find the function declaration
	var decl *ast.FuncDecl
	for _, file := range sc.pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				if obj := sc.pass.TypesInfo.Defs[fd.Name]; obj == fn {
					decl = fd
					return false
				}
			}
			return true
		})
		if decl != nil {
			break
		}
	}

	// Get or create summary
	summary := sc.getSummary(fn, decl)
	return summary.returnState
}

// combineNilStates combines multiple nil states into one
func combineNilStates(states []NilState) NilState {
	if len(states) == 0 {
		return NilStateUnknown
	}

	hasNil := false
	hasNonNil := false
	hasUnknown := false
	hasMaybe := false

	for _, state := range states {
		switch state {
		case NilStateAlwaysNil:
			hasNil = true
		case NilStateNeverNil:
			hasNonNil = true
		case NilStateMaybeNil:
			hasMaybe = true
		case NilStateUnknown:
			hasUnknown = true
		}
	}

	// If any path is unknown, overall is unknown
	if hasUnknown {
		return NilStateUnknown
	}

	// If any path may return nil, overall may return nil
	if hasMaybe {
		return NilStateMaybeNil
	}

	// If all paths return nil
	if hasNil && !hasNonNil {
		return NilStateAlwaysNil
	}

	// If all paths return non-nil
	if hasNonNil && !hasNil {
		return NilStateNeverNil
	}

	// Mixed nil and non-nil paths
	if hasNil && hasNonNil {
		return NilStateMaybeNil
	}

	return NilStateUnknown
}
