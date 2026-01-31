package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func main() {
	output := flag.String("output", "", "write facts JSON to file (default: stdout)")
	flag.StringVar(output, "o", "", "write facts JSON to file (shorthand)")
	deltaFrom := flag.String("delta-from", "", "previous facts JSON to compute delta from")
	deltaOut := flag.String("delta-out", "", "write delta JSON to file (requires --delta-from)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: vhdl-facts [--output file] [--delta-from prev.json --delta-out delta.json] <path>")
		os.Exit(1)
	}

	path := args[0]
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	idx := indexer.NewWithConfig(cfg)
	idx.JSONOutput = false

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", os.DevNull, err)
		os.Exit(1)
	}
	oldStdout := os.Stdout
	os.Stdout = devNull
	runErr := idx.Run(path)
	_ = devNull.Close()
	os.Stdout = oldStdout
	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}

	symbolRows := make([]facts.SymbolRow, 0)
	for _, sym := range idx.Symbols.All() {
		symbolRows = append(symbolRows, facts.SymbolRow{
			Name: sym.Name,
			Kind: sym.Kind,
			File: sym.File,
			Line: sym.Line,
		})
	}

	tables := facts.BuildTables(idx.Facts, idx.FileLibraries, idx.ThirdPartyFiles, symbolRows)

	if *output != "" {
		if err := writeJSON(*output, tables); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing facts: %v\n", err)
			os.Exit(1)
		}
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(tables); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding facts: %v\n", err)
			os.Exit(1)
		}
	}

	if *deltaFrom != "" || *deltaOut != "" {
		if *deltaFrom == "" || *deltaOut == "" {
			fmt.Fprintln(os.Stderr, "Error: --delta-from and --delta-out must be used together")
			os.Exit(1)
		}
		prev, err := readTables(*deltaFrom)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading delta-from: %v\n", err)
			os.Exit(1)
		}
		delta := facts.ComputeDelta(prev, tables)
		if err := writeJSON(*deltaOut, delta); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing delta: %v\n", err)
			os.Exit(1)
		}
	}
}

func readTables(path string) (facts.Tables, error) {
	f, err := os.Open(path)
	if err != nil {
		return facts.Tables{}, err
	}
	defer func() { _ = f.Close() }()

	var tables facts.Tables
	if err := json.NewDecoder(f).Decode(&tables); err != nil {
		return facts.Tables{}, err
	}
	return tables, nil
}

func writeJSON(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
