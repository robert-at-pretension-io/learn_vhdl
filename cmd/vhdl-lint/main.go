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

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit()
	case "-v", "--verbose":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		runLintWithOptions(os.Args[2], true, false)
	case "-j", "--json":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		runLintWithOptions(os.Args[2], false, true)
	case "-h", "--help", "help":
		printUsage()
	case "-c", "--config":
		if len(os.Args) < 4 {
			printUsage()
			os.Exit(1)
		}
		runLintWithConfig(os.Args[2], os.Args[3], false, false)
	default:
		runLintWithOptions(cmd, false, false)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: vhdl-lint [command] [options] <path>

Commands:
  init              Create a vhdl_lint.json configuration file
  <path>            Lint VHDL files in the given path

Options:
  -v, --verbose     Enable verbose output (extraction details)
  -j, --json        Output results as JSON (for programmatic parsing)
  -c, --config      Specify config file: vhdl-lint -c config.json <path>
  -h, --help        Show this help message

Configuration:
  vhdl-lint looks for configuration in:
    1. ./vhdl_lint.json
    2. ./.vhdl_lint.json
    3. ~/.config/vhdl_lint/config.json

  Run 'vhdl-lint init' to create a default configuration file.`)
}

func runInit() {
	configPath := "vhdl_lint.json"

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config file %s already exists. Overwrite? [y/N]: ", configPath)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	cfg := config.DefaultConfig()
	if err := cfg.Save(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", configPath)
	fmt.Println("\nEdit this file to configure:")
	fmt.Println("  - Library file patterns")
	fmt.Println("  - Third-party library detection")
	fmt.Println("  - Lint rule severities")
}

func runLintWithOptions(path string, verbose, jsonOutput bool) {
	// Load config from default locations
	cfg, err := config.Load(path)
	if err != nil {
		if !jsonOutput {
			fmt.Printf("Warning: Could not load config: %v (using defaults)\n", err)
		}
		cfg = config.DefaultConfig()
	}

	idx := indexer.NewWithConfig(cfg)
	idx.Verbose = verbose
	idx.JSONOutput = jsonOutput
	if err := idx.Run(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runLintWithConfig(configPath, lintPath string, verbose, jsonOutput bool) {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config %s: %v\n", configPath, err)
		os.Exit(1)
	}

	idx := indexer.NewWithConfig(cfg)
	idx.Verbose = verbose
	idx.JSONOutput = jsonOutput
	if err := idx.Run(lintPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
