package main

import (
	"fmt"
	"os"

	sitter "github.com/tree-sitter/go-tree-sitter"
	micro_vhdl "micro-vhdl/tree-sitter-micro-vhdl/bindings/go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: main <file.vhd>")
		os.Exit(1)
	}
	content, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	lang := sitter.NewLanguage(micro_vhdl.Language())
	parser := sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		fmt.Printf("Error setting language: %v\n", err)
		os.Exit(1)
	}

	tree := parser.Parse(content, nil)

	fmt.Println("AST:", tree.RootNode().ToSexp())

	// Step 1: Extraction
	extractor := NewExtractor(content)
	modules, err := extractor.ExtractAll(tree.RootNode())
	if err != nil {
		fmt.Printf("Extraction error: %v\n", err)
		os.Exit(1)
	}

	for _, module := range modules {
		fmt.Printf("Extracted Module: %s\n", module.Name)
		fmt.Printf("  Symbols (%d):\n", len(module.Symbols))
		for name, typ := range module.Symbols {
			fmt.Printf("    %s: %s (width %d)\n", name, typ.Name, typ.Width)
		}
	}

	// Step 2: Semantic Checking
	checker := NewSemanticChecker(modules)
	errors := checker.Check()

	if len(errors) > 0 {
		fmt.Println("\nSemantic Checks Failed:")
		for _, err := range errors {
			fmt.Println("  -", err)
		}
		os.Exit(1)
	}

	fmt.Println("\nSemantic Checks Passed. Ready for MLIR Emission.")

	// Step 3: Lowering to MLIR
	emitter := NewMLIREmitter()
	emitter.EmitModules(modules)

	outputFile := "build.mlir"
	err = os.WriteFile(outputFile, []byte(emitter.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing MLIR to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nMLIR successfully generated and written to %s\n", outputFile)
	fmt.Println("\n--- Generated MLIR ---")
	fmt.Println(emitter.String())
}
