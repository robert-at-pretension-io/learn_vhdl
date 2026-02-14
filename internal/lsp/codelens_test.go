package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCodeLensUsesCachedDiagnosticsAndSymbols(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	uri := "file:///workspace/top.vhd"
	errSev := protocol.DiagnosticSeverityError
	warnSev := protocol.DiagnosticSeverityWarning
	s.setCachedDiagnostics(uri, []protocol.Diagnostic{
		{Severity: &errSev},
		{Severity: &warnSev},
		{Data: map[string]any{"kind": "missing_check"}},
	})
	s.symbolStore.Rebuild(&SymbolIndex{
		Entities: []EntitySummary{{Name: "uart_tx", File: "/workspace/top.vhd", Line: 1}},
	}, s.workspaceRoot)

	lenses, err := s.textDocumentCodeLens(nil, &protocol.CodeLensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		t.Fatalf("code lens error: %v", err)
	}
	if len(lenses) == 0 {
		t.Fatal("expected code lenses")
	}
	foundMessageCommand := false
	for _, lens := range lenses {
		if lens.Command != nil && lens.Command.Command == commandShowMessage {
			foundMessageCommand = true
		}
	}
	if !foundMessageCommand {
		t.Fatal("expected code lens command entries")
	}
}

func TestWorkspaceExecuteCommandLogsMessage(t *testing.T) {
	s := NewServer()
	logged := false
	s.notifyFunc = func(method string, _ any) {
		if method == "window/logMessage" {
			logged = true
		}
	}
	_, err := s.workspaceExecuteCommand(nil, &protocol.ExecuteCommandParams{
		Command:   commandShowMessage,
		Arguments: []any{"hello"},
	})
	if err != nil {
		t.Fatalf("execute command error: %v", err)
	}
	if !logged {
		t.Fatal("expected window/logMessage notification")
	}
}
