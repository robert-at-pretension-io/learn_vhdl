package facts

import "testing"

func TestFilterTablesByFiles(t *testing.T) {
	tables := Tables{
		Files: []FileRow{
			{Path: "a.vhd"},
			{Path: "b.vhd"},
		},
		Entities: []EntityRow{
			{Name: "a", File: "a.vhd"},
			{Name: "b", File: "b.vhd"},
		},
		Ports: []PortRow{
			{Entity: "a", Name: "clk", File: "a.vhd"},
			{Entity: "b", Name: "rst", File: "b.vhd"},
		},
		Symbols: []SymbolRow{
			{Name: "work.a", File: "a.vhd"},
			{Name: "work.b", File: "b.vhd"},
		},
	}

	files := map[string]bool{"a.vhd": true}
	filtered := FilterTablesByFiles(tables, files)

	if len(filtered.Files) != 1 || filtered.Files[0].Path != "a.vhd" {
		t.Fatalf("expected only a.vhd file row, got %#v", filtered.Files)
	}
	if len(filtered.Entities) != 1 || filtered.Entities[0].File != "a.vhd" {
		t.Fatalf("expected only a.vhd entity rows, got %#v", filtered.Entities)
	}
	if len(filtered.Ports) != 1 || filtered.Ports[0].File != "a.vhd" {
		t.Fatalf("expected only a.vhd port rows, got %#v", filtered.Ports)
	}
	if len(filtered.Symbols) != 1 || filtered.Symbols[0].File != "a.vhd" {
		t.Fatalf("expected only a.vhd symbol rows, got %#v", filtered.Symbols)
	}
}

func TestFilterDeltaByFilesEmpty(t *testing.T) {
	delta := Delta{
		Added: Tables{
			Files: []FileRow{{Path: "a.vhd"}},
		},
		Removed: Tables{
			Files: []FileRow{{Path: "b.vhd"}},
		},
	}

	filtered := FilterDeltaByFiles(delta, map[string]bool{})
	if len(filtered.Added.Files) != 0 || len(filtered.Removed.Files) != 0 {
		t.Fatalf("expected empty delta, got %#v", filtered)
	}
}
