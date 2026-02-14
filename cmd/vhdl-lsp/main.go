package main

import (
	"fmt"
	"os"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/lsp"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			printUsage()
			return
		case "--version":
			fmt.Println("vhdl-lsp 0.1.0")
			return
		}
	}

	server := lsp.NewServer()
	if err := server.RunStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "vhdl-lsp fatal: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vhdl-lsp - VHDL Language Server Protocol implementation

Usage: vhdl-lsp [options]

The server communicates via JSON-RPC over stdin/stdout (LSP stdio transport).

Options:
  -h, --help     Show this help message
  --version      Show version

Environment Variables:
  VHDL_LINT_BIN        Override vhdl-lint binary path
  VHDL_LINT_CONFIG     Override vhdl-lint config path (passed as -c <config>)
  VHDL_LSP_DEBOUNCE_MS Debounce delay in milliseconds (default: 500)
  VHDL_LSP_LOG_LEVEL   Log level: debug, info, warn, error

Editor Setup:
  Neovim:   vim.lsp.start({cmd={"vhdl-lsp"}, root_dir=vim.fn.getcwd()})
  VS Code:  Use a generic LSP extension pointing to vhdl-lsp`)
}
