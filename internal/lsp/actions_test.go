package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCodeActionUsesDiagnosticDataForRuleAndLine(t *testing.T) {
	s := NewServer()
	source := "vhdl-lint"
	diag := protocol.Diagnostic{
		Source: &source,
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
		Data: map[string]any{
			"kind": "violation",
			"rule": "unused_signal",
			"line": 5, // 1-based
		},
	}

	res, err := s.textDocumentCodeAction(nil, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///workspace/test.vhd"},
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{diag},
		},
	})
	if err != nil {
		t.Fatalf("code action error: %v", err)
	}
	actions := res.([]protocol.CodeAction)
	if len(actions) != 1 {
		t.Fatalf("expected one code action, got %d", len(actions))
	}
	edits := actions[0].Edit.Changes["file:///workspace/test.vhd"]
	if len(edits) != 2 {
		t.Fatalf("expected two edits, got %d", len(edits))
	}
	if edits[0].Range.Start.Line != 4 { // 1-based 5 converted to 0-based 4
		t.Fatalf("expected disable edit at line 4, got %d", edits[0].Range.Start.Line)
	}
}
