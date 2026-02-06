package lsp

import (
	"testing"
)

func testSymbolIndex() *SymbolIndex {
	return &SymbolIndex{
		Entities: []EntitySummary{
			{Name: "uart_tx", File: "/src/uart_tx.vhd", Line: 5, Ports: []PortSummary{
				{Name: "clk", Direction: "in", Type: "std_logic", File: "/src/uart_tx.vhd", Line: 6, InEntity: "uart_tx"},
				{Name: "data_out", Direction: "out", Type: "std_logic_vector(7 downto 0)", File: "/src/uart_tx.vhd", Line: 7, InEntity: "uart_tx"},
			}},
			{Name: "uart_rx", File: "/src/uart_rx.vhd", Line: 3},
		},
		Architectures: []ArchSummary{
			{Name: "rtl", EntityName: "uart_tx", File: "/src/uart_tx.vhd", Line: 20},
		},
		Packages: []PackageSummary{
			{Name: "uart_pkg", File: "/src/uart_pkg.vhd", Line: 1},
		},
		Signals: []SignalSummary{
			{Name: "state", Type: "state_t", File: "/src/uart_tx.vhd", Line: 25, InEntity: "uart_tx"},
		},
		Types: []TypeSummary{
			{Name: "state_t", Kind: "enum", File: "/src/uart_pkg.vhd", Line: 10, InPackage: "uart_pkg"},
		},
		Functions: []FunctionSummary{
			{Name: "to_baud", ReturnType: "integer", File: "/src/uart_pkg.vhd", Line: 15, InPackage: "uart_pkg"},
		},
		Instances: []InstanceSummary{
			{Name: "u_tx", Target: "work.uart_tx", File: "/src/top.vhd", Line: 30, InArch: "rtl"},
		},
	}
}

func TestSymbolStoreRebuild(t *testing.T) {
	ss := NewSymbolStore()
	ss.Rebuild(testSymbolIndex())

	// Check byName lookups
	entries := ss.LookupByName("uart_tx")
	if len(entries) == 0 {
		t.Fatal("expected entries for uart_tx")
	}
	found := false
	for _, e := range entries {
		if e.kind == "entity" {
			found = true
		}
	}
	if !found {
		t.Error("expected entity kind for uart_tx")
	}
}

func TestSymbolStoreFindDefinition(t *testing.T) {
	ss := NewSymbolStore()
	ss.Rebuild(testSymbolIndex())

	// Entity definition
	locs := ss.FindDefinition("uart_tx")
	if len(locs) == 0 {
		t.Fatal("expected definition for uart_tx")
	}

	// Signal — no "defining" kind, should still return
	locs = ss.FindDefinition("state")
	if len(locs) == 0 {
		t.Fatal("expected location for state signal")
	}

	// Non-existent
	locs = ss.FindDefinition("nonexistent_symbol_xyz")
	if len(locs) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(locs))
	}
}

func TestSymbolStoreFindReferences(t *testing.T) {
	ss := NewSymbolStore()
	ss.Rebuild(testSymbolIndex())

	locs := ss.FindReferences("uart_tx")
	// entity + architecture(entityName mapped to detail, not name) + instance target
	if len(locs) == 0 {
		t.Fatal("expected references for uart_tx")
	}
}

func TestSymbolStoreWorkspaceSymbols(t *testing.T) {
	ss := NewSymbolStore()
	ss.Rebuild(testSymbolIndex())

	// Partial match
	results := ss.WorkspaceSymbols("uart")
	if len(results) == 0 {
		t.Fatal("expected symbols matching 'uart'")
	}
	for _, r := range results {
		if r.Name == "" {
			t.Error("symbol name should not be empty")
		}
	}

	// Empty query returns all
	all := ss.WorkspaceSymbols("")
	if len(all) < len(results) {
		t.Error("empty query should return at least as many as partial query")
	}
}

func TestSymbolStoreNilIndex(t *testing.T) {
	ss := NewSymbolStore()
	ss.Rebuild(nil) // should not panic
	locs := ss.FindDefinition("anything")
	if len(locs) != 0 {
		t.Error("expected no results from empty store")
	}
}
