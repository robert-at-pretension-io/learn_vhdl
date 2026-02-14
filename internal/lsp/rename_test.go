package lsp

import (
	"os"
	"path/filepath"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestPrepareRenameReturnsRangeAndPlaceholder(t *testing.T) {
	s := NewServer()
	uri := "file:///workspace/test.vhd"
	s.documentStore.Set(uri, "signal clk : std_logic;")

	res, err := s.textDocumentPrepareRename(nil, &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	})
	if err != nil {
		t.Fatalf("prepare rename error: %v", err)
	}
	p, ok := res.(protocol.RangeWithPlaceholder)
	if !ok {
		t.Fatalf("expected RangeWithPlaceholder, got %T", res)
	}
	if p.Placeholder != "clk" {
		t.Fatalf("expected placeholder clk, got %q", p.Placeholder)
	}
}

func TestRenameBuildsWorkspaceEditAcrossFiles(t *testing.T) {
	tmp := t.TempDir()
	fileA := filepath.Join(tmp, "a.vhd")
	fileB := filepath.Join(tmp, "b.vhd")
	if err := os.WriteFile(fileA, []byte("signal clk : std_logic;\n"), 0o644); err != nil {
		t.Fatalf("write a.vhd: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("signal clk : std_logic;\n"), 0o644); err != nil {
		t.Fatalf("write b.vhd: %v", err)
	}

	s := NewServer()
	s.workspaceRoot = tmp
	uriA := "file://" + filepath.ToSlash(fileA)
	uriB := "file://" + filepath.ToSlash(fileB)
	s.documentStore.Set(uriA, "signal clk : std_logic;\n")
	s.documentStore.Set(uriB, "signal clk : std_logic;\n")
	s.symbolStore.Rebuild(&SymbolIndex{
		Signals: []SignalSummary{
			{Name: "clk", Type: "std_logic", File: fileA, Line: 1},
			{Name: "clk", Type: "std_logic", File: fileB, Line: 1},
		},
	}, s.workspaceRoot)

	edit, err := s.textDocumentRename(nil, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uriA},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
		NewName: "core_clk",
	})
	if err != nil {
		t.Fatalf("rename error: %v", err)
	}
	if edit == nil || len(edit.Changes) == 0 {
		t.Fatal("expected workspace edit changes")
	}
	if len(edit.Changes[uriA]) == 0 || len(edit.Changes[uriB]) == 0 {
		t.Fatal("expected edits in both files")
	}
}

func TestRenameRejectsInvalidIdentifier(t *testing.T) {
	s := NewServer()
	s.documentStore.Set("file:///workspace/test.vhd", "signal clk : std_logic;")

	edit, err := s.textDocumentRename(nil, &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///workspace/test.vhd"},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
		NewName: "1bad",
	})
	if err != nil {
		t.Fatalf("rename error: %v", err)
	}
	if edit != nil {
		t.Fatal("expected nil edit for invalid identifier")
	}
}
