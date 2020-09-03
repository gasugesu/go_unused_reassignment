package go_unused_reassignment_test

import (
	"testing"

	"github.com/gasugesu/go_unused_reassignment"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer is a test for Analyzer.
func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, go_unused_reassignment.Analyzer, "a")
}

