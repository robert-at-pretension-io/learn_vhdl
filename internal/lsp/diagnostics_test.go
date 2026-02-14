package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestMapResultToDiagnostics_Violations(t *testing.T) {
	result := &LintResult{
		Violations: []Violation{
			{Rule: "missing_reset", Severity: "error", File: "/tmp/test.vhd", Line: 10, Message: "missing reset"},
			{Rule: "unused_signal", Severity: "warning", File: "/tmp/test.vhd", Line: 20, Message: "unused signal"},
			{Rule: "naming_convention", Severity: "info", File: "/tmp/other.vhd", Line: 5, Message: "bad name"},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")

	// Should have 2 file URIs
	if len(diags) != 2 {
		t.Fatalf("expected 2 file URIs, got %d", len(diags))
	}

	testURI := fileToURI("/tmp/test.vhd", "/tmp")
	fileDiags := diags[testURI]
	if len(fileDiags) != 2 {
		t.Fatalf("expected 2 diagnostics for test.vhd, got %d", len(fileDiags))
	}

	// Check first diagnostic
	if fileDiags[0].Range.Start.Line != 9 { // 10 - 1 = 9 (0-based)
		t.Errorf("expected line 9, got %d", fileDiags[0].Range.Start.Line)
	}
	if *fileDiags[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected error severity")
	}
	if fileDiags[0].Message != "missing reset" {
		t.Errorf("expected 'missing reset', got %q", fileDiags[0].Message)
	}

	// Check second diagnostic
	if *fileDiags[1].Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("expected warning severity")
	}
}

func TestMapResultToDiagnostics_ParseErrors(t *testing.T) {
	result := &LintResult{
		ParseErrors: []ParseError{
			{File: "/tmp/broken.vhd", Message: "unexpected token at line 5"},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")
	uri := fileToURI("/tmp/broken.vhd", "/tmp")
	fileDiags := diags[uri]

	if len(fileDiags) != 1 {
		t.Fatalf("expected 1 parse error diagnostic, got %d", len(fileDiags))
	}
	if *fileDiags[0].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("expected error severity for parse error")
	}
	if fileDiags[0].Code.Value != "parse_error" {
		t.Errorf("expected code 'parse_error', got %v", fileDiags[0].Code.Value)
	}
}

func TestMapResultToDiagnostics_ParseErrorNoFile(t *testing.T) {
	result := &LintResult{
		ParseErrors: []ParseError{
			{File: "", Message: "generic error"},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")
	if len(diags) != 0 {
		t.Fatalf("expected 0 URIs for parse error with empty file, got %d", len(diags))
	}
}

func TestMapResultToDiagnostics_MissingChecks(t *testing.T) {
	result := &LintResult{
		MissingChecks: []MissingCheckTask{
			{
				File:       "/tmp/dut.vhd",
				Scope:      "arch:rtl",
				MissingIDs: []string{"check_reset_hygiene", "check_ready_valid"},
			},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")
	uri := fileToURI("/tmp/dut.vhd", "/tmp")
	fileDiags := diags[uri]

	if len(fileDiags) != 1 {
		t.Fatalf("expected 1 missing-check diagnostic, got %d", len(fileDiags))
	}
	if *fileDiags[0].Severity != protocol.DiagnosticSeverityInformation {
		t.Errorf("expected info severity for missing check")
	}
	if fileDiags[0].Code.Value != "missing_check" {
		t.Errorf("expected code 'missing_check', got %v", fileDiags[0].Code.Value)
	}
}

func TestMapResultToDiagnostics_AmbiguousConstructs(t *testing.T) {
	result := &LintResult{
		AmbiguousConstructs: []AmbiguousConstruct{
			{Kind: "counter", Scope: "arch:rtl", File: "/tmp/dut.vhd", Line: 42},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")
	uri := fileToURI("/tmp/dut.vhd", "/tmp")
	fileDiags := diags[uri]

	if len(fileDiags) != 1 {
		t.Fatalf("expected 1 ambiguous construct diagnostic, got %d", len(fileDiags))
	}
	if *fileDiags[0].Severity != protocol.DiagnosticSeverityHint {
		t.Errorf("expected hint severity for ambiguous construct")
	}
	if fileDiags[0].Range.Start.Line != 41 { // 42 - 1
		t.Errorf("expected line 41, got %d", fileDiags[0].Range.Start.Line)
	}
}

func TestMapResultToDiagnostics_AllCombined(t *testing.T) {
	result := &LintResult{
		Violations: []Violation{
			{Rule: "unused_signal", Severity: "warning", File: "/tmp/dut.vhd", Line: 10, Message: "unused"},
		},
		ParseErrors: []ParseError{
			{File: "/tmp/broken.vhd", Message: "parse failed"},
		},
		MissingChecks: []MissingCheckTask{
			{File: "/tmp/dut.vhd", MissingIDs: []string{"check_a"}},
		},
		AmbiguousConstructs: []AmbiguousConstruct{
			{Kind: "counter", File: "/tmp/dut.vhd", Line: 20},
		},
	}

	diags := mapResultToDiagnostics(result, "/tmp")

	// dut.vhd should have 3 diagnostics (violation + missing check + ambiguous)
	dutURI := fileToURI("/tmp/dut.vhd", "/tmp")
	if len(diags[dutURI]) != 3 {
		t.Fatalf("expected 3 diagnostics for dut.vhd, got %d", len(diags[dutURI]))
	}

	// broken.vhd should have 1 (parse error)
	brokenURI := fileToURI("/tmp/broken.vhd", "/tmp")
	if len(diags[brokenURI]) != 1 {
		t.Fatalf("expected 1 diagnostic for broken.vhd, got %d", len(diags[brokenURI]))
	}
}

func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected protocol.DiagnosticSeverity
	}{
		{"error", protocol.DiagnosticSeverityError},
		{"warning", protocol.DiagnosticSeverityWarning},
		{"info", protocol.DiagnosticSeverityInformation},
		{"unknown", protocol.DiagnosticSeverityInformation},
		{"ERROR", protocol.DiagnosticSeverityError},
	}

	for _, tt := range tests {
		got := mapSeverity(tt.input)
		if got != tt.expected {
			t.Errorf("mapSeverity(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestFileToURI(t *testing.T) {
	uri := fileToURI("/home/user/test.vhd", "/home/user")
	if uri != "file:///home/user/test.vhd" {
		t.Errorf("unexpected URI: %s", uri)
	}

	// Relative path
	uri = fileToURI("src/test.vhd", "/home/user")
	if uri != "file:///home/user/src/test.vhd" {
		t.Errorf("unexpected URI for relative path: %s", uri)
	}
}

func TestFileToURI_RelativeWithoutWorkspaceRootBecomesAbsolute(t *testing.T) {
	uri := fileToURI("src/test.vhd", "")
	if !strings.HasPrefix(uri, "file:///") {
		t.Fatalf("expected absolute file URI, got: %s", uri)
	}
	if !strings.HasSuffix(uri, "/src/test.vhd") {
		t.Fatalf("expected src/test.vhd suffix, got: %s", uri)
	}
}

func TestURIToFile(t *testing.T) {
	path := uriToFile("file:///home/user/test.vhd")
	if path != "/home/user/test.vhd" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestURIToFile_DecodesEscapes(t *testing.T) {
	path := uriToFile("file:///tmp/my%20file.vhd")
	expected := filepath.Clean("/tmp/my file.vhd")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestLineZeroBased(t *testing.T) {
	result := &LintResult{
		Violations: []Violation{
			{Rule: "test", Severity: "error", File: "/test.vhd", Line: 1, Message: "first line"},
			{Rule: "test", Severity: "error", File: "/test.vhd", Line: 0, Message: "zero line"},
		},
	}

	diags := mapResultToDiagnostics(result, "")
	uri := fileToURI("/test.vhd", "")
	fileDiags := diags[uri]

	if len(fileDiags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(fileDiags))
	}
	if fileDiags[0].Range.Start.Line != 0 { // line 1 -> 0
		t.Errorf("line 1 should map to 0, got %d", fileDiags[0].Range.Start.Line)
	}
	if fileDiags[1].Range.Start.Line != 0 { // line 0 stays 0
		t.Errorf("line 0 should map to 0, got %d", fileDiags[1].Range.Start.Line)
	}
}

func TestMapResultToDiagnostics_AttachesDataAndRelatedInfo(t *testing.T) {
	result := &LintResult{
		Violations: []Violation{
			{Rule: "unused_signal", Severity: "warning", File: "/tmp/dut.vhd", Line: 7, Message: "unused"},
		},
		MissingChecks: []MissingCheckTask{
			{
				File:       "/tmp/dut.vhd",
				Scope:      "arch:rtl",
				MissingIDs: []string{"check_reset"},
				Bindings:   map[string]string{"clk": "sys_clk"},
				Notes:      []string{"inferred scope"},
			},
		},
	}
	diags := mapResultToDiagnostics(result, "/tmp")
	uri := fileToURI("/tmp/dut.vhd", "/tmp")
	fileDiags := diags[uri]
	if len(fileDiags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(fileDiags))
	}
	if fileDiags[0].Data == nil {
		t.Fatal("expected diagnostic data for violation")
	}
	if len(fileDiags[1].RelatedInformation) == 0 {
		t.Fatal("expected related information for missing check")
	}
}
