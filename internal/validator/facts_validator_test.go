package validator

import (
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
)

func TestFactsValidatorAcceptsValidTables(t *testing.T) {
	v, err := NewFactsValidator()
	if err != nil {
		t.Fatalf("new facts validator: %v", err)
	}

	tables := facts.Tables{
		Files: []facts.FileRow{{
			Path:         "test/a.vhd",
			Library:      "work",
			IsThirdParty: false,
		}},
		Entities: []facts.EntityRow{{
			Name: "my_entity",
			File: "test/a.vhd",
			Line: 1,
		}},
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

	if err := v.Validate(tables); err != nil {
		t.Fatalf("expected valid tables, got error: %v", err)
	}
}

func TestFactsValidatorRejectsInvalidTables(t *testing.T) {
	v, err := NewFactsValidator()
	if err != nil {
		t.Fatalf("new facts validator: %v", err)
	}

	tables := facts.Tables{
		Files: []facts.FileRow{{
			Path:         "test/a.txt",
			Library:      "work",
			IsThirdParty: false,
		}},
		Entities: []facts.EntityRow{{
			Name: "my_entity",
			File: "test/a.txt",
			Line: 0,
		}},
	}

	if err := v.Validate(tables); err == nil {
		t.Fatalf("expected validation error, got nil")
	}
}
