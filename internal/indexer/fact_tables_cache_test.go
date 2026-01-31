package indexer

import (
	"reflect"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
)

func TestFactTablesCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tables := facts.Tables{
		Files: []facts.FileRow{
			{Path: "a.vhd", Library: "work", IsThirdParty: false},
		},
		Entities:       []facts.EntityRow{},
		Architectures:  []facts.ArchitectureRow{},
		Packages:       []facts.PackageRow{},
		Ports:          []facts.PortRow{},
		Signals:        []facts.SignalRow{},
		Instances:      []facts.InstanceRow{},
		Dependencies:   []facts.DependencyRow{},
		UseClauses:     []facts.UseClauseRow{},
		LibraryClauses: []facts.LibraryClauseRow{},
		ContextClauses: []facts.ContextClauseRow{},
		Processes:      []facts.ProcessRow{},
		Generates:      []facts.GenerateRow{},
		Types:          []facts.TypeRow{},
		Subtypes:       []facts.SubtypeRow{},
		Functions:      []facts.FunctionRow{},
		Procedures:     []facts.ProcedureRow{},
		Constants:      []facts.ConstantRow{},
		Symbols:        []facts.SymbolRow{},
	}

	if err := saveFactTablesCache(dir, tables); err != nil {
		t.Fatalf("saveFactTablesCache error: %v", err)
	}

	loaded, ok, err := loadFactTablesCache(dir)
	if err != nil {
		t.Fatalf("loadFactTablesCache error: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache to be present")
	}
	if !reflect.DeepEqual(tables, loaded) {
		t.Fatalf("tables mismatch: expected %#v got %#v", tables, loaded)
	}
}
