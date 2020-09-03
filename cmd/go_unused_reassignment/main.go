package main

import (
	"github.com/gasugesu/go_unused_reassignment"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() { unitchecker.Main(gounusedreassignment.Analyzer) }

