package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/validator"
)

// Indexer is the cross-file linker that builds the symbol table
// and resolves dependencies between VHDL files.
type Indexer struct {
	// Library map: logical name -> physical path
	Libraries map[string]string

	// Global symbol table: qualified name -> location
	Symbols *SymbolTable

	// Extracted facts from all files
	Facts []extractor.FileFacts

	// Verbose output
	Verbose bool
}

// SymbolTable holds all exported symbols across files
type SymbolTable struct {
	mu      sync.RWMutex
	symbols map[string]Symbol
}

// Symbol represents an exported VHDL construct
type Symbol struct {
	Name     string // Qualified name: work.my_entity
	Kind     string // entity, package, component, etc.
	File     string // Source file path
	Line     int    // Line number
}

// New creates a new Indexer
func New() *Indexer {
	return &Indexer{
		Libraries: map[string]string{
			"work": ".", // Default: work library is current directory
		},
		Symbols: &SymbolTable{
			symbols: make(map[string]Symbol),
		},
	}
}

// Run executes the indexing pipeline
func (idx *Indexer) Run(rootPath string) error {
	// 1. Find all VHDL files
	files, err := idx.findVHDLFiles(rootPath)
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}

	fmt.Printf("Found %d VHDL files\n", len(files))

	// 2. Pass 1: Parallel extraction
	ext := extractor.New()
	var wg sync.WaitGroup
	factsChan := make(chan extractor.FileFacts, len(files))
	errChan := make(chan error, len(files))

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			facts, err := ext.Extract(f)
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", f, err)
				return
			}
			factsChan <- facts

			// Register exports in symbol table
			for _, entity := range facts.Entities {
				idx.Symbols.Add(Symbol{
					Name: fmt.Sprintf("work.%s", strings.ToLower(entity.Name)),
					Kind: "entity",
					File: f,
					Line: entity.Line,
				})
			}
			for _, pkg := range facts.Packages {
				idx.Symbols.Add(Symbol{
					Name: fmt.Sprintf("work.%s", strings.ToLower(pkg.Name)),
					Kind: "package",
					File: f,
					Line: pkg.Line,
				})
			}
		}(file)
	}

	wg.Wait()
	close(factsChan)
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	// Collect facts
	for facts := range factsChan {
		idx.Facts = append(idx.Facts, facts)
	}

	// Verbose output for debugging
	if idx.Verbose {
		fmt.Printf("\n=== Verbose: Extracted Ports ===\n")
		for _, facts := range idx.Facts {
			for _, p := range facts.Ports {
				fmt.Printf("  %s.%s: direction=%q type=%q\n", p.InEntity, p.Name, p.Direction, p.Type)
			}
		}
		fmt.Printf("\n=== Verbose: Extracted Processes ===\n")
		for _, facts := range idx.Facts {
			for _, p := range facts.Processes {
				kind := "combinational"
				if p.IsSequential {
					kind = "sequential"
				}
				fmt.Printf("  %s.%s: %s, sensitivity=%v\n", p.InArch, p.Label, kind, p.SensitivityList)
				if p.ClockSignal != "" {
					fmt.Printf("    clock: %s (%s_edge)\n", p.ClockSignal, p.ClockEdge)
				}
				if p.HasReset {
					asyncStr := "sync"
					if p.ResetAsync {
						asyncStr = "async"
					}
					fmt.Printf("    reset: %s (%s)\n", p.ResetSignal, asyncStr)
				}
				if len(p.AssignedSignals) > 0 {
					fmt.Printf("    writes: %v\n", p.AssignedSignals)
				}
				if len(p.ReadSignals) > 0 {
					fmt.Printf("    reads: %v\n", p.ReadSignals)
				}
			}
		}
		fmt.Printf("\n=== Verbose: Clock Domains ===\n")
		for _, facts := range idx.Facts {
			for _, cd := range facts.ClockDomains {
				fmt.Printf("  %s (%s): drives %v\n", cd.Clock, cd.Edge, cd.Registers)
			}
		}
	}

	// 3. Pass 2: Resolution (check imports)
	var missing []string
	for _, facts := range idx.Facts {
		for _, dep := range facts.Dependencies {
			qualName := strings.ToLower(dep.Target)
			if !idx.Symbols.Has(qualName) && !isStandardLibrary(qualName) {
				missing = append(missing, fmt.Sprintf("%s: missing import %q", facts.File, dep.Target))
			}
		}
	}

	// 4. Build OPA input
	opaInput := idx.buildPolicyInput()

	// 5. Validate data structure before policy evaluation
	v := validator.New()
	if err := v.Validate(opaInput); err != nil {
		return fmt.Errorf("CRITICAL: Invalid data structure: %w", err)
	}

	// 6. Run policy evaluation
	policyEngine, err := policy.New("policies")
	if err != nil {
		fmt.Printf("Warning: Could not load policies: %v\n", err)
	} else {
		result, err := policyEngine.Evaluate(opaInput)
		if err != nil {
			fmt.Printf("Warning: Policy evaluation failed: %v\n", err)
		} else {
			// Report violations
			if len(result.Violations) > 0 {
				fmt.Printf("\n=== Policy Violations ===\n")
				for _, v := range result.Violations {
					icon := "ℹ"
					if v.Severity == "error" {
						icon = "✗"
					} else if v.Severity == "warning" {
						icon = "⚠"
					}
					fmt.Printf("%s [%s] %s:%d - %s\n", icon, v.Rule, v.File, v.Line, v.Message)
				}
			}

			fmt.Printf("\n=== Policy Summary ===\n")
			fmt.Printf("  Errors:   %d\n", result.Summary.Errors)
			fmt.Printf("  Warnings: %d\n", result.Summary.Warnings)
			fmt.Printf("  Info:     %d\n", result.Summary.Info)
		}
	}

	// 6. Report basic stats
	fmt.Printf("\n=== Extraction Summary ===\n")
	fmt.Printf("  Files:    %d\n", len(files))
	fmt.Printf("  Symbols:  %d\n", idx.Symbols.Len())
	fmt.Printf("  Entities: %d\n", len(opaInput.Entities))
	fmt.Printf("  Packages: %d\n", len(opaInput.Packages))
	fmt.Printf("  Signals:  %d\n", len(opaInput.Signals))
	fmt.Printf("  Ports:    %d\n", len(opaInput.Ports))

	if len(errs) > 0 {
		fmt.Printf("\n=== Parse Errors ===\n")
		for _, e := range errs {
			fmt.Printf("  %v\n", e)
		}
	}

	return nil
}

// buildPolicyInput converts extracted facts to OPA input format
func (idx *Indexer) buildPolicyInput() policy.Input {
	input := policy.Input{}

	// Aggregate facts from all files
	for _, facts := range idx.Facts {
		for _, e := range facts.Entities {
			// Find ports for this entity
			var ports []policy.Port
			for _, p := range facts.Ports {
				if p.InEntity == e.Name {
					ports = append(ports, policy.Port{
						Name:      p.Name,
						Direction: p.Direction,
						Type:      p.Type,
						Line:      p.Line,
						InEntity:  p.InEntity,
					})
				}
			}
			input.Entities = append(input.Entities, policy.Entity{
				Name:  e.Name,
				File:  facts.File,
				Line:  e.Line,
				Ports: ports,
			})
		}

		for _, a := range facts.Architectures {
			input.Architectures = append(input.Architectures, policy.Architecture{
				Name:       a.Name,
				EntityName: a.EntityName,
				File:       facts.File,
				Line:       a.Line,
			})
		}

		for _, p := range facts.Packages {
			input.Packages = append(input.Packages, policy.Package{
				Name: p.Name,
				File: facts.File,
				Line: p.Line,
			})
		}

		for _, c := range facts.Components {
			input.Components = append(input.Components, policy.Component{
				Name:       c.Name,
				EntityRef:  c.EntityRef,
				File:       facts.File,
				Line:       c.Line,
				IsInstance: c.IsInstance,
			})
		}

		for _, s := range facts.Signals {
			input.Signals = append(input.Signals, policy.Signal{
				Name:     s.Name,
				Type:     s.Type,
				File:     facts.File,
				Line:     s.Line,
				InEntity: s.InEntity,
			})
		}

		for _, p := range facts.Ports {
			input.Ports = append(input.Ports, policy.Port{
				Name:      p.Name,
				Direction: p.Direction,
				Type:      p.Type,
				Line:      p.Line,
				InEntity:  p.InEntity,
			})
		}

		for _, d := range facts.Dependencies {
			resolved := idx.Symbols.Has(strings.ToLower(d.Target)) || isStandardLibrary(d.Target)
			input.Dependencies = append(input.Dependencies, policy.Dependency{
				Source:   d.Source,
				Target:   d.Target,
				Kind:     d.Kind,
				Line:     d.Line,
				Resolved: resolved,
			})
		}
	}

	// Add symbols
	for name, sym := range idx.Symbols.All() {
		input.Symbols = append(input.Symbols, policy.Symbol{
			Name: name,
			Kind: sym.Kind,
			File: sym.File,
			Line: sym.Line,
		})
	}

	return input
}

func (idx *Indexer) findVHDLFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".vhd" || ext == ".vhdl" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// SymbolTable methods

func (st *SymbolTable) Add(sym Symbol) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.symbols[sym.Name] = sym
}

func (st *SymbolTable) Has(name string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	_, ok := st.symbols[name]
	return ok
}

func (st *SymbolTable) All() map[string]Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()
	// Return a copy
	result := make(map[string]Symbol)
	for k, v := range st.symbols {
		result[k] = v
	}
	return result
}

func (st *SymbolTable) Len() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.symbols)
}

// isStandardLibrary checks if a library is a standard/vendor library
func isStandardLibrary(name string) bool {
	standard := []string{
		"ieee.", "std.", "std_logic_1164", "numeric_std",
		"textio", "math_real", "math_complex",
	}
	for _, prefix := range standard {
		if strings.HasPrefix(name, prefix) || strings.Contains(name, prefix) {
			return true
		}
	}
	return false
}
