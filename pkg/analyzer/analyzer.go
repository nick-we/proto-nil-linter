package analyzer

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer is the main analysis pass for proto-nil-linter
var Analyzer = &analysis.Analyzer{
	Name:     "protonillinter",
	Doc:      "checks for nil assignments to non-optional proto3 message fields in gRPC service handlers",
	Run:      run,
	Requires: []*analysis.Analyzer{
		// We can add dependencies here if needed
	},
}

// run is the main entry point for the analyzer
func run(pass *analysis.Pass) (any, error) {
	// Initialize our analyzers
	protoAnalyzer := newProtoAnalyzer(pass)
	grpcAnalyzer := newGRPCAnalyzer(pass, protoAnalyzer)
	nilChecker := newNilChecker(pass, protoAnalyzer, grpcAnalyzer)

	// Single pass: do all analysis in one traversal
	for _, file := range pass.Files {
		// First collect proto messages and gRPC handlers
		ast.Inspect(file, func(n ast.Node) bool {
			protoAnalyzer.visit(n)
			grpcAnalyzer.visit(n)
			return true
		})
	}

	// Second pass: now that we have all proto and handler info, check for nil
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			nilChecker.visit(n)
			return true
		})
	}

	return nil, nil
}
