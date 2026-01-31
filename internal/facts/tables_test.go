package facts

import (
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestBuildTablesPopulatesCoreRelations(t *testing.T) {
	facts := []extractor.FileFacts{
		{
			File: "test/a.vhd",
			Entities: []extractor.Entity{{
				Name: "e",
				Line: 1,
			}},
			UseClauses: []extractor.UseClause{{
				Items: []string{"ieee.std_logic_1164.all"},
				Line:  2,
			}},
		},
	}

	libs := map[string]config.FileLibraryInfo{
		"test/a.vhd": {LibraryName: "work"},
	}
	thirdParty := map[string]bool{"test/a.vhd": false}
	symbols := []SymbolRow{{
		Name: "work.e",
		Kind: "entity",
		File: "test/a.vhd",
		Line: 1,
	}}

	tables := BuildTables(facts, libs, thirdParty, symbols)

	if len(tables.Files) != 1 {
		t.Fatalf("expected 1 file row, got %d", len(tables.Files))
	}
	if len(tables.Entities) != 1 {
		t.Fatalf("expected 1 entity row, got %d", len(tables.Entities))
	}
	if len(tables.UseClauses) != 1 {
		t.Fatalf("expected 1 use clause row, got %d", len(tables.UseClauses))
	}
	if len(tables.Symbols) != 1 {
		t.Fatalf("expected 1 symbol row, got %d", len(tables.Symbols))
	}
}
