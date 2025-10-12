package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/nick-we/proto-nil-linter/pkg/analyzer"
)

func main() {
	singlechecker.Main(analyzer.Analyzer)
}
