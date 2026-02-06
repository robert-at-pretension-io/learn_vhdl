package lsp

import (
	"encoding/json"
	"testing"
)

// TestLintResultContractSync verifies that the LSP's LintResult can parse all
// fields that the indexer's LintResult produces. This catches type drift between
// the two packages (the LSP re-declares these types to avoid CGO deps).
//
// If this test fails, it means the indexer changed its JSON output shape and
// the LSP needs to be updated to match.
func TestLintResultContractSync(t *testing.T) {
	// This JSON represents a complete LintResult from vhdl-lint --symbols-json.
	// It includes every top-level field that the indexer can produce.
	fullJSON := `{
		"violations": [
			{"rule": "unused_signal", "severity": "warning", "file": "/test.vhd", "line": 10, "message": "signal 'foo' is unused"}
		],
		"missing_checks": [
			{"file": "/test.vhd", "scope": "arch:rtl", "missing_ids": ["check_reset"], "bindings": {"clk": "sys_clk"}, "notes": ["auto-detected"]}
		],
		"ambiguous_constructs": [
			{"kind": "counter", "scope": "arch:rtl", "file": "/test.vhd", "line": 42}
		],
		"waivers": [
			{"id": "check_reset", "scope": "arch:rtl", "reason": "external reset", "file": "/test.vhd", "line": 5}
		],
		"summary": {"total_violations": 1, "errors": 0, "warnings": 1, "info": 0},
		"parse_errors": [
			{"file": "/broken.vhd", "message": "unexpected token"}
		],
		"symbol_index": {
			"entities": [{"name": "uart_tx", "file": "/test.vhd", "line": 1, "ports": [{"name": "clk", "direction": "in", "type": "std_logic", "file": "/test.vhd", "line": 2}]}],
			"architectures": [{"name": "rtl", "entity_name": "uart_tx", "file": "/test.vhd", "line": 10}],
			"packages": [{"name": "pkg", "file": "/pkg.vhd", "line": 1}],
			"signals": [{"name": "state", "type": "state_t", "file": "/test.vhd", "line": 15}],
			"ports": [{"name": "clk", "direction": "in", "type": "std_logic", "file": "/test.vhd", "line": 2}],
			"types": [{"name": "state_t", "kind": "enum", "file": "/pkg.vhd", "line": 5}],
			"constants": [{"name": "BAUD", "type": "integer", "file": "/pkg.vhd", "line": 8}],
			"functions": [{"name": "to_int", "return_type": "integer", "file": "/pkg.vhd", "line": 10}],
			"procedures": [{"name": "send", "file": "/pkg.vhd", "line": 12}],
			"instances": [{"name": "u_tx", "target": "work.uart_tx", "file": "/top.vhd", "line": 20}],
			"components": [{"name": "uart_tx", "file": "/top.vhd", "line": 5}]
		}
	}`

	var result LintResult
	if err := json.Unmarshal([]byte(fullJSON), &result); err != nil {
		t.Fatalf("failed to parse contract JSON: %v", err)
	}

	// Verify violations
	if len(result.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(result.Violations))
	} else {
		v := result.Violations[0]
		if v.Rule != "unused_signal" || v.File != "/test.vhd" || v.Line != 10 {
			t.Errorf("violation fields mismatch: %+v", v)
		}
	}

	// Verify missing checks
	if len(result.MissingChecks) != 1 {
		t.Errorf("expected 1 missing check, got %d", len(result.MissingChecks))
	} else {
		mc := result.MissingChecks[0]
		if mc.File != "/test.vhd" || len(mc.MissingIDs) != 1 || mc.MissingIDs[0] != "check_reset" {
			t.Errorf("missing check fields mismatch: %+v", mc)
		}
	}

	// Verify ambiguous constructs
	if len(result.AmbiguousConstructs) != 1 {
		t.Errorf("expected 1 ambiguous construct, got %d", len(result.AmbiguousConstructs))
	} else {
		ac := result.AmbiguousConstructs[0]
		if ac.Kind != "counter" || ac.File != "/test.vhd" || ac.Line != 42 {
			t.Errorf("ambiguous construct fields mismatch: %+v", ac)
		}
	}

	// Verify waivers
	if len(result.Waivers) != 1 {
		t.Errorf("expected 1 waiver, got %d", len(result.Waivers))
	} else {
		w := result.Waivers[0]
		if w.ID != "check_reset" || w.Reason != "external reset" {
			t.Errorf("waiver fields mismatch: %+v", w)
		}
	}

	// Verify summary
	if result.Summary.TotalViolations != 1 || result.Summary.Warnings != 1 {
		t.Errorf("summary mismatch: %+v", result.Summary)
	}

	// Verify parse errors
	if len(result.ParseErrors) != 1 {
		t.Errorf("expected 1 parse error, got %d", len(result.ParseErrors))
	}

	// Verify symbol index
	if result.SymbolIndex == nil {
		t.Fatal("expected non-nil symbol index")
	}
	si := result.SymbolIndex
	if len(si.Entities) != 1 || si.Entities[0].Name != "uart_tx" {
		t.Errorf("entity mismatch: %+v", si.Entities)
	}
	if len(si.Entities[0].Ports) != 1 || si.Entities[0].Ports[0].Name != "clk" {
		t.Errorf("entity ports mismatch: %+v", si.Entities[0].Ports)
	}
	if len(si.Architectures) != 1 || si.Architectures[0].EntityName != "uart_tx" {
		t.Errorf("architecture mismatch")
	}
	if len(si.Packages) != 1 || si.Packages[0].Name != "pkg" {
		t.Errorf("package mismatch")
	}
	if len(si.Signals) != 1 || si.Signals[0].Type != "state_t" {
		t.Errorf("signal mismatch")
	}
	if len(si.Types) != 1 || si.Types[0].Kind != "enum" {
		t.Errorf("type mismatch")
	}
	if len(si.Constants) != 1 || si.Constants[0].Type != "integer" {
		t.Errorf("constant mismatch")
	}
	if len(si.Functions) != 1 || si.Functions[0].ReturnType != "integer" {
		t.Errorf("function mismatch")
	}
	if len(si.Procedures) != 1 || si.Procedures[0].Name != "send" {
		t.Errorf("procedure mismatch")
	}
	if len(si.Instances) != 1 || si.Instances[0].Target != "work.uart_tx" {
		t.Errorf("instance mismatch")
	}
	if len(si.Components) != 1 || si.Components[0].Name != "uart_tx" {
		t.Errorf("component mismatch")
	}
}

// TestLintResultRoundTrip ensures that a LintResult can be serialized and
// deserialized without data loss.
func TestLintResultRoundTrip(t *testing.T) {
	original := LintResult{
		Violations: []Violation{
			{Rule: "test_rule", Severity: "error", File: "/a.vhd", Line: 1, Message: "msg"},
		},
		MissingChecks: []MissingCheckTask{
			{File: "/a.vhd", Scope: "s", MissingIDs: []string{"id1"}},
		},
		AmbiguousConstructs: []AmbiguousConstruct{
			{Kind: "k", File: "/a.vhd", Line: 5},
		},
		Waivers: []Waiver{
			{ID: "w1", Scope: "s", Reason: "r", File: "/a.vhd", Line: 10},
		},
		Summary:     ResultSummary{TotalViolations: 1, Errors: 1},
		ParseErrors: []ParseError{{File: "/b.vhd", Message: "err"}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded LintResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Violations) != 1 || decoded.Violations[0].Rule != "test_rule" {
		t.Error("violation lost in round trip")
	}
	if len(decoded.MissingChecks) != 1 || decoded.MissingChecks[0].MissingIDs[0] != "id1" {
		t.Error("missing check lost in round trip")
	}
	if len(decoded.AmbiguousConstructs) != 1 || decoded.AmbiguousConstructs[0].Kind != "k" {
		t.Error("ambiguous construct lost in round trip")
	}
	if len(decoded.Waivers) != 1 || decoded.Waivers[0].ID != "w1" {
		t.Error("waiver lost in round trip")
	}
	if len(decoded.ParseErrors) != 1 {
		t.Error("parse error lost in round trip")
	}
}
