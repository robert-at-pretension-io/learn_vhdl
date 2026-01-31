package validator

import (
	"encoding/json"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
)

func emptyFactTables() facts.Tables {
	return facts.Tables{
		Files:          []facts.FileRow{},
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
}

func TestPolicyDaemonValidator(t *testing.T) {
	v, err := NewPolicyDaemonValidator()
	if err != nil {
		t.Fatalf("new daemon validator: %v", err)
	}

	cmd := struct {
		Kind   string       `json:"kind"`
		Tables facts.Tables `json:"tables"`
	}{
		Kind:   "init",
		Tables: emptyFactTables(),
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	if err := v.ValidateCommandJSON(payload); err != nil {
		t.Fatalf("expected valid command, got %v", err)
	}

	respPayload, err := json.Marshal(map[string]any{
		"kind": "snapshot",
		"summary": map[string]int{
			"total_violations": 0,
			"errors":           0,
			"warnings":         0,
			"info":             0,
		},
		"violations": []map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if err := v.ValidateResponseJSON(respPayload); err != nil {
		t.Fatalf("expected valid response, got %v", err)
	}
}
