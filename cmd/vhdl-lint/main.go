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
