package gounusedreassignment_test

import (
	"testing"

	"github.com/gasugesu/go_unused_reassignment"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer is a test for Analyzer.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, gounusedreassignment.Analyzer, "a")
}

