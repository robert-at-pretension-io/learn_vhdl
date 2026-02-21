package main

import (
	"fmt"
	"os"
	"strings"

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

	fatalErrors := false
	if len(errors) > 0 {
		for _, err := range errors {
			if strings.Contains(err, "Warning -") {
				fmt.Println("  -", err)
			} else {
				if !fatalErrors {
					fmt.Println("\nSemantic Checks Failed:")
				}
				fmt.Println("  -", err)
				fatalErrors = true
			}
		}
		if fatalErrors {
			os.Exit(1)
		}
	}

	fmt.Println("\nSemantic Checks Passed. Ready for MLIR Emission.")

	// Step 3: Lowering to MLIR
	emitter := NewMLIREmitter()
	emitter.EmitModules(modules)

	outputFile := "build.mlir"
	if len(os.Args) >= 3 {
		outputFile = os.Args[2]
	}
	err = os.WriteFile(outputFile, []byte(emitter.String()), 0644)
	if err != nil {
		fmt.Printf("Error writing MLIR to file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nMLIR written to %s\n", outputFile)

	// Emit IC3 MLIR (bad-state output for ABC pdr) when a path is provided.
	if len(os.Args) >= 4 {
		ic3File := os.Args[3]
		ic3Emitter := NewMLIREmitterIC3()
		ic3Emitter.EmitModules(modules)
		err = os.WriteFile(ic3File, []byte(ic3Emitter.String()), 0644)
		if err != nil {
			fmt.Printf("Error writing IC3 MLIR to file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("IC3 MLIR written to %s\n", ic3File)
	}

	fmt.Println("\n--- Generated MLIR ---")
	fmt.Println(emitter.String())
}
