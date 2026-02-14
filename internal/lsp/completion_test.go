package lsp

import (
	"strconv"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCompletionIncludesKeywordsSymbolsAndSnippets(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	s.documentStore.Set("file:///workspace/test.vhd", "pro")
	s.symbolStore.Rebuild(&SymbolIndex{
		Signals: []SignalSummary{
			{Name: "proj_clk", Type: "std_logic", File: "/workspace/test.vhd", Line: 1},
		},
	}, s.workspaceRoot)

	res, err := s.textDocumentCompletion(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///workspace/test.vhd"},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})
	if err != nil {
		t.Fatalf("completion error: %v", err)
	}
	list, ok := res.(protocol.CompletionList)
	if !ok {
		t.Fatalf("expected completion list, got %T", res)
	}
	if len(list.Items) == 0 {
		t.Fatal("expected completion items")
	}
	labels := make(map[string]bool)
	for _, it := range list.Items {
		labels[it.Label] = true
	}
	if !labels["process"] {
		t.Fatal("expected 'process' keyword completion")
	}
	if !labels["proj_clk"] {
		t.Fatal("expected symbol completion for proj_clk")
	}
	if !labels["process (clocked)"] {
		t.Fatal("expected process snippet completion")
	}
}

func TestCompletionLimitAndIncompleteFlag(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	s.documentStore.Set("file:///workspace/test.vhd", "")

	signals := make([]SignalSummary, 0, 260)
	for i := 0; i < 260; i++ {
		signals = append(signals, SignalSummary{
			Name: "sig_" + strconv.Itoa(i),
			Type: "std_logic",
			File: "/workspace/test.vhd",
			Line: i + 1,
		})
	}
	s.symbolStore.Rebuild(&SymbolIndex{Signals: signals}, s.workspaceRoot)

	res, err := s.textDocumentCompletion(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///workspace/test.vhd"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	if err != nil {
		t.Fatalf("completion error: %v", err)
	}
	list := res.(protocol.CompletionList)
	if len(list.Items) != 200 {
		t.Fatalf("expected capped 200 items, got %d", len(list.Items))
	}
	if !list.IsIncomplete {
		t.Fatal("expected incomplete flag for capped results")
	}
}
