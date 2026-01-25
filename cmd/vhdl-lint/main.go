// =============================================================================
// VHDL Linter - Main Entry Point
// =============================================================================
//
// This tool transforms VHDL from "text files" into a "queryable database,"
// enabling safety checks that were previously impossible without expensive
// proprietary tools.
//
// THE PIPELINE:
//   1. Tree-sitter parses VHDL into syntax tree (grammar.js)
//   2. Extractor extracts semantic facts (entities, signals, processes...)
//   3. Indexer builds cross-file symbol table and resolves dependencies
//   4. CUE Validator enforces data contract (crash on schema mismatch)
//   5. OPA evaluates policy rules against the extracted data
//   6. Violations are reported with file/line locations
//
// WHEN INVESTIGATING FALSE POSITIVES:
//   Start at the beginning of the pipeline, not the end!
//   Grammar issues → Extractor issues → Policy issues
//
// See: AGENTS.md for the complete architecture and improvement workflow.
// =============================================================================

package main

import (
	"fmt"
	"os"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: vhdl-lint [-v] <path>")
		os.Exit(1)
	}

	verbose := false
	path := os.Args[1]
	if path == "-v" && len(os.Args) > 2 {
		verbose = true
		path = os.Args[2]
	}

	idx := indexer.New()
	idx.Verbose = verbose
	if err := idx.Run(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
