package indexer

import (
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestImpactExpansion(t *testing.T) {
	factsA := extractor.FileFacts{
		File: "a.vhd",
		Packages: []extractor.Package{{Name: "pkg", Line: 1}},
	}
	factsB := extractor.FileFacts{
		File: "b.vhd",
		Dependencies: []extractor.Dependency{{Target: "work.pkg", Line: 1}},
	}
	factsC := extractor.FileFacts{
		File: "c.vhd",
		Dependencies: []extractor.Dependency{{Target: "work.pkg", Line: 1}},
	}

	factsByFile := map[string]extractor.FileFacts{
		"a.vhd": factsA,
		"b.vhd": factsB,
		"c.vhd": factsC,
	}

	symbols := &SymbolTable{symbols: make(map[string]Symbol)}
	symbols.Add(Symbol{Name: "work.pkg", Kind: "package", File: "a.vhd", Line: 1})

	fileLibs := map[string]config.FileLibraryInfo{
		"a.vhd": {LibraryName: "work"},
		"b.vhd": {LibraryName: "work"},
		"c.vhd": {LibraryName: "work"},
	}

	deps := buildDependentsGraph(factsByFile, symbols, fileLibs)
	report := computeImpact("a.vhd", deps)

	if len(report.Levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(report.Levels))
	}
	level := report.Levels[0]
	if len(level) != 2 {
		t.Fatalf("expected 2 dependents, got %d", len(level))
	}
	if level[0] != "b.vhd" || level[1] != "c.vhd" {
		t.Fatalf("unexpected dependents: %v", level)
	}
}
