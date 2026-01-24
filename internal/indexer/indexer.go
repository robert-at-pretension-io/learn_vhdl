package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
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

	// 4. Report results
	fmt.Printf("\nSymbol Table:\n")
	for name, sym := range idx.Symbols.All() {
		fmt.Printf("  %s (%s) -> %s:%d\n", name, sym.Kind, sym.File, sym.Line)
	}

	if len(missing) > 0 {
		fmt.Printf("\nMissing Dependencies:\n")
		for _, m := range missing {
			fmt.Printf("  %s\n", m)
		}
	}

	if len(errs) > 0 {
		fmt.Printf("\nParse Errors:\n")
		for _, e := range errs {
			fmt.Printf("  %v\n", e)
		}
	}

	fmt.Printf("\nSummary: %d files, %d symbols, %d missing deps, %d errors\n",
		len(files), idx.Symbols.Len(), len(missing), len(errs))

	return nil
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
