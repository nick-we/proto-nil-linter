package analyzer

import (
	"go/ast"
	"go/token"
)

// PathState represents the nil state of variables at a specific program point
type PathState struct {
	// Map from variable name to its nil state
	vars map[string]NilState

	// Conditions that hold on this path
	conditions []PathCondition

	// Track which fields are initialized for each variable
	// Key: varName, Value: map of fieldName -> bool
	varFields map[string]map[string]bool
}

// PathCondition represents a condition that holds on a path
type PathCondition struct {
	varName string
	isNil   bool // true if "x == nil", false if "x != nil"
}

// newPathState creates a new path state
func newPathState() *PathState {
	return &PathState{
		vars:       make(map[string]NilState),
		conditions: []PathCondition{},
		varFields:  make(map[string]map[string]bool),
	}
}

// copy creates a copy of the path state
func (ps *PathState) copy() *PathState {
	newState := &PathState{
		vars:       make(map[string]NilState),
		conditions: make([]PathCondition, len(ps.conditions)),
		varFields:  make(map[string]map[string]bool),
	}

	for k, v := range ps.vars {
		newState.vars[k] = v
	}

	for k, v := range ps.varFields {
		newState.varFields[k] = make(map[string]bool)
		for f, present := range v {
			newState.varFields[k][f] = present
		}
	}

	copy(newState.conditions, ps.conditions)

	return newState
}

// setVar sets the nil state of a variable
func (ps *PathState) setVar(name string, state NilState) {
	ps.vars[name] = state
}

// getVar gets the nil state of a variable
func (ps *PathState) getVar(name string) NilState {
	if state, exists := ps.vars[name]; exists {
		return state
	}
	return NilStateUnknown
}

// addCondition adds a condition to this path
func (ps *PathState) addCondition(varName string, isNil bool) {
	ps.conditions = append(ps.conditions, PathCondition{
		varName: varName,
		isNil:   isNil,
	})

	// Update variable state based on condition
	if isNil {
		ps.vars[varName] = NilStateAlwaysNil
	} else {
		ps.vars[varName] = NilStateNeverNil
	}
}

// setVarFields sets the initialized fields for a variable
func (ps *PathState) setVarFields(varName string, fields map[string]bool) {
	ps.varFields[varName] = fields
}

// getVarFields gets the initialized fields for a variable
func (ps *PathState) getVarFields(varName string) map[string]bool {
	return ps.varFields[varName]
}

// merge merges two path states at a join point
func mergePathStates(s1, s2 *PathState) *PathState {
	if s1 == nil {
		return s2
	}
	if s2 == nil {
		return s1
	}

	merged := newPathState()

	// Merge variable states
	allVars := make(map[string]bool)
	for v := range s1.vars {
		allVars[v] = true
	}
	for v := range s2.vars {
		allVars[v] = true
	}

	for v := range allVars {
		state1 := s1.getVar(v)
		state2 := s2.getVar(v)
		merged.setVar(v, combineNilStates([]NilState{state1, state2}))
	}

	// Merge field initialization information
	// Conservative approach: take the most restrictive (fewest initialized fields)
	allFieldVars := make(map[string]bool)
	for v := range s1.varFields {
		allFieldVars[v] = true
	}
	for v := range s2.varFields {
		allFieldVars[v] = true
	}

	for v := range allFieldVars {
		fields1 := s1.getVarFields(v)
		fields2 := s2.getVarFields(v)

		// Merge field maps
		var mergedFields map[string]bool

		if fields1 != nil && fields2 != nil {
			// Both paths have field info - intersect (only fields in both)
			mergedFields = make(map[string]bool)
			for f := range fields1 {
				if fields2[f] {
					mergedFields[f] = true
				}
			}
		} else if fields1 != nil {
			// Only path 1 has field info - use it
			mergedFields = fields1
		} else if fields2 != nil {
			// Only path 2 has field info - use it
			mergedFields = fields2
		}

		if mergedFields != nil {
			merged.setVarFields(v, mergedFields)
		}
	}

	return merged
}

// PathAnalyzer performs path-sensitive analysis
type PathAnalyzer struct {
	// Current path state
	currentState *PathState

	// States at various program points (for debugging/inspection)
	states map[ast.Node]*PathState
}

// newPathAnalyzer creates a new path analyzer
func newPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{
		currentState: newPathState(),
		states:       make(map[ast.Node]*PathState),
	}
}

// analyzeStmt analyzes a statement and updates path state
func (pa *PathAnalyzer) analyzeStmt(stmt ast.Stmt, checker *nilChecker) {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		pa.analyzeIfStmt(s, checker)

	case *ast.SwitchStmt:
		pa.analyzeSwitchStmt(s, checker)

	case *ast.AssignStmt:
		pa.analyzeAssignStmt(s, checker)

	case *ast.DeclStmt:
		pa.analyzeDeclStmt(s, checker)

	case *ast.ReturnStmt:
		// Return statement - check state at this point
		pa.states[s] = pa.currentState.copy()

	case *ast.BlockStmt:
		pa.analyzeBlockStmt(s, checker)
	}
}

// analyzeIfStmt analyzes an if statement with path-sensitive tracking
func (pa *PathAnalyzer) analyzeIfStmt(ifStmt *ast.IfStmt, checker *nilChecker) {
	// Check if condition is a nil check
	varName, isNilCheck, checkForNil := pa.isNilCheck(ifStmt.Cond)

	if isNilCheck && varName != "" {
		// Fork the state
		thenState := pa.currentState.copy()
		elseState := pa.currentState.copy()

		// Update states based on condition
		if checkForNil {
			// if x == nil
			thenState.addCondition(varName, true)  // then: x is nil
			elseState.addCondition(varName, false) // else: x is not nil
		} else {
			// if x != nil
			thenState.addCondition(varName, false) // then: x is not nil
			elseState.addCondition(varName, true)  // else: x is nil
		}

		// Analyze then branch
		pa.currentState = thenState
		if ifStmt.Body != nil {
			pa.analyzeBlockStmt(ifStmt.Body, checker)
		}
		afterThen := pa.currentState.copy()

		// Analyze else branch
		pa.currentState = elseState
		if ifStmt.Else != nil {
			switch e := ifStmt.Else.(type) {
			case *ast.BlockStmt:
				pa.analyzeBlockStmt(e, checker)
			case *ast.IfStmt:
				pa.analyzeIfStmt(e, checker)
			}
		}
		afterElse := pa.currentState.copy()

		// Merge states after if
		pa.currentState = mergePathStates(afterThen, afterElse)
	} else {
		// Not a nil check, analyze normally
		if ifStmt.Body != nil {
			pa.analyzeBlockStmt(ifStmt.Body, checker)
		}
		if ifStmt.Else != nil {
			switch e := ifStmt.Else.(type) {
			case *ast.BlockStmt:
				pa.analyzeBlockStmt(e, checker)
			case *ast.IfStmt:
				pa.analyzeIfStmt(e, checker)
			}
		}
	}
}

// isNilCheck checks if an expression is a nil check (x == nil or x != nil)
func (pa *PathAnalyzer) isNilCheck(expr ast.Expr) (varName string, isCheck bool, checkForNil bool) {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return "", false, false
	}

	// Check for == or !=
	if binary.Op != token.EQL && binary.Op != token.NEQ {
		return "", false, false
	}

	checkForNil = (binary.Op == token.EQL)

	// Check if one side is nil
	var varExpr ast.Expr
	if pa.isNilExpr(binary.X) {
		varExpr = binary.Y
	} else if pa.isNilExpr(binary.Y) {
		varExpr = binary.X
	} else {
		return "", false, false
	}

	// Extract variable name
	if ident, ok := varExpr.(*ast.Ident); ok {
		return ident.Name, true, checkForNil
	}

	return "", false, false
}

// isNilExpr checks if an expression is the nil literal
func (pa *PathAnalyzer) isNilExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

// analyzeAssignStmt analyzes an assignment statement
func (pa *PathAnalyzer) analyzeAssignStmt(assign *ast.AssignStmt, checker *nilChecker) {
	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			continue
		}
		rhs := assign.Rhs[i]

		// Get variable name
		if ident, ok := lhs.(*ast.Ident); ok {
			// Determine nil state of RHS
			state := pa.determineNilState(rhs, checker)
			pa.currentState.setVar(ident.Name, state)

			// If RHS is a composite literal, track which fields are initialized
			if comp, ok := rhs.(*ast.UnaryExpr); ok {
				if comp.Op == token.AND {
					if compLit, ok := comp.X.(*ast.CompositeLit); ok {
						fields := pa.extractInitializedFields(compLit)
						pa.currentState.setVarFields(ident.Name, fields)
					}
				}
			} else if compLit, ok := rhs.(*ast.CompositeLit); ok {
				fields := pa.extractInitializedFields(compLit)
				pa.currentState.setVarFields(ident.Name, fields)
			}
		}
	}
}

// analyzeDeclStmt analyzes a declaration statement
func (pa *PathAnalyzer) analyzeDeclStmt(decl *ast.DeclStmt, checker *nilChecker) {
	genDecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return
	}

	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for i, name := range valueSpec.Names {
			if i < len(valueSpec.Values) {
				state := pa.determineNilState(valueSpec.Values[i], checker)
				pa.currentState.setVar(name.Name, state)

				// Track field initialization for composite literals
				if comp, ok := valueSpec.Values[i].(*ast.UnaryExpr); ok {
					if comp.Op == token.AND {
						if compLit, ok := comp.X.(*ast.CompositeLit); ok {
							fields := pa.extractInitializedFields(compLit)
							pa.currentState.setVarFields(name.Name, fields)
						}
					}
				} else if compLit, ok := valueSpec.Values[i].(*ast.CompositeLit); ok {
					fields := pa.extractInitializedFields(compLit)
					pa.currentState.setVarFields(name.Name, fields)
				}
			}
		}
	}
}

// analyzeBlockStmt analyzes a block statement
func (pa *PathAnalyzer) analyzeBlockStmt(block *ast.BlockStmt, checker *nilChecker) {
	for _, stmt := range block.List {
		pa.analyzeStmt(stmt, checker)
	}
}

// determineNilState determines the nil state of an expression
func (pa *PathAnalyzer) determineNilState(expr ast.Expr, checker *nilChecker) NilState {
	switch e := expr.(type) {
	case *ast.Ident:
		if e.Name == "nil" {
			return NilStateAlwaysNil
		}
		// Check current state
		return pa.currentState.getVar(e.Name)

	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return NilStateNeverNil
		}

	case *ast.CompositeLit:
		return NilStateNeverNil

	case *ast.CallExpr:
		// Use function summary
		return checker.summaryCache.analyzeExpr(e)
	}

	return NilStateUnknown
}

// getVarState gets the current nil state of a variable
func (pa *PathAnalyzer) getVarState(varName string) NilState {
	return pa.currentState.getVar(varName)
}

// extractInitializedFields extracts which fields are explicitly initialized in a composite literal
func (pa *PathAnalyzer) extractInitializedFields(comp *ast.CompositeLit) map[string]bool {
	fields := make(map[string]bool)

	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				fields[ident.Name] = true
			}
		}
	}

	return fields
}

// analyzeSwitchStmt analyzes a switch statement
func (pa *PathAnalyzer) analyzeSwitchStmt(switchStmt *ast.SwitchStmt, checker *nilChecker) {
	// Collect states from all cases
	var caseStates []*PathState

	// Save initial state
	initialState := pa.currentState.copy()

	// Analyze each case
	for _, clause := range switchStmt.Body.List {
		caseClause, ok := clause.(*ast.CaseClause)
		if !ok {
			continue
		}

		// Start with initial state for each case
		pa.currentState = initialState.copy()

		// Analyze case body
		for _, stmt := range caseClause.Body {
			pa.analyzeStmt(stmt, checker)
		}

		// Save state after this case
		caseStates = append(caseStates, pa.currentState.copy())
	}

	// Merge all case states
	if len(caseStates) == 0 {
		return
	}

	merged := caseStates[0]
	for i := 1; i < len(caseStates); i++ {
		merged = mergePathStates(merged, caseStates[i])
	}

	pa.currentState = merged
}

// getVarInitializedFields gets which fields were initialized for a variable
func (pa *PathAnalyzer) getVarInitializedFields(varName string) map[string]bool {
	return pa.currentState.getVarFields(varName)
}
