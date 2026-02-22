package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	micro_vhdl "micro-vhdl/tree-sitter-micro-vhdl/bindings/go"
)

func main() {
	outMlir := flag.String("o", "build.mlir", "Output MLIR file")
	outIc3 := flag.String("ic3", "build_ic3.mlir", "Output IC3 MLIR file")
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		fmt.Println("Usage: go run . [-o build.mlir] [-ic3 build_ic3.mlir] <file1.vhd> [file2.vhd ...]")
		os.Exit(1)
	}

	lang := sitter.NewLanguage(micro_vhdl.Language())
	parser := sitter.NewParser()
	if err := parser.SetLanguage(lang); err != nil {
		fmt.Printf("Error setting language: %v\n", err)
		os.Exit(1)
	}

	var allModules []*Module

	// 1. Parse ALL files into a global project pool
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", file, err)
			os.Exit(1)
		}

		tree := parser.Parse(content, nil)
		extractor := NewExtractor(content)
		modules, err := extractor.ExtractAll(tree.RootNode())
		if err != nil {
			fmt.Printf("Extraction error in %s: %v\n", file, err)
			os.Exit(1)
		}
		allModules = append(allModules, modules...)
	}

	// 2. Auto-detect Top Module (the one never instantiated)
	instantiated := make(map[string]bool)
	for _, mod := range allModules {
		var walk func(stmts []Statement)
		walk = func(stmts []Statement) {
			for _, stmt := range stmts {
				if inst, ok := stmt.(*EntityInstantiation); ok {
					instantiated[inst.EntityName] = true
				} else if gen, ok := stmt.(*GenerateStatement); ok {
					walk(gen.Statements)
				}
			}
		}
		walk(mod.Statements)
	}

	topModule := ""
	for _, mod := range allModules {
		if !instantiated[mod.Name] {
			topModule = mod.Name
			break
		}
	}
	if topModule == "" && len(allModules) > 0 {
		topModule = allModules[len(allModules)-1].Name // Fallback
	}
	
	// Print it so the bash orchestrator can catch it
	fmt.Printf("TOP_MODULE=%s\n", topModule)

	// 3. Cross-Module Semantic Checking
	checker := NewSemanticChecker(allModules)
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

	fmt.Printf("\nSemantic Checks Passed. Extracted %d modules. Ready for MLIR Emission.\n", len(allModules))

	// 4. Lowering to MLIR
	emitter := NewMLIREmitter()
	emitter.EmitModules(allModules, topModule)
	if err := os.WriteFile(*outMlir, []byte(emitter.String()), 0644); err != nil {
		fmt.Printf("Error writing MLIR: %v\n", err)
		os.Exit(1)
	}
	
	if *outIc3 != "" {
		ic3Emitter := NewMLIREmitterIC3()
		ic3Emitter.EmitModules(allModules, topModule)
		if err := os.WriteFile(*outIc3, []byte(ic3Emitter.String()), 0644); err != nil {
			fmt.Printf("Error writing IC3 MLIR: %v\n", err)
			os.Exit(1)
		}
	}
}