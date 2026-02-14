package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestSemanticTokensFullIncludesKeywordAndComment(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	uri := "file:///workspace/test.vhd"
	s.documentStore.Set(uri, "entity uart_tx is\nsignal clk : std_logic;\n-- comment\n")
	s.symbolStore.Rebuild(&SymbolIndex{
		Entities: []EntitySummary{{Name: "uart_tx", File: "/workspace/test.vhd", Line: 1}},
		Signals:  []SignalSummary{{Name: "clk", Type: "std_logic", File: "/workspace/test.vhd", Line: 2}},
	}, s.workspaceRoot)

	tokens, err := s.textDocumentSemanticTokensFull(nil, &protocol.SemanticTokensParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		t.Fatalf("semantic tokens error: %v", err)
	}
	if tokens == nil || len(tokens.Data) == 0 {
		t.Fatal("expected semantic token data")
	}
	types := decodeTokenTypes(tokens.Data)
	if !types[semanticTokenTypeIndex["keyword"]] {
		t.Fatal("expected keyword token")
	}
	if !types[semanticTokenTypeIndex["comment"]] {
		t.Fatal("expected comment token")
	}
}

func TestEncodeSemanticTokensProducesRelativeFormat(t *testing.T) {
	data := encodeSemanticTokens([]semanticToken{
		{line: 0, startCol: 0, length: 6, typ: semanticTokenTypeIndex["keyword"]},
		{line: 1, startCol: 2, length: 3, typ: semanticTokenTypeIndex["variable"]},
	})
	if len(data) != 10 {
		t.Fatalf("expected 10 uint entries, got %d", len(data))
	}
	if data[0] != 0 || data[1] != 0 {
		t.Fatalf("unexpected first token deltas: %v %v", data[0], data[1])
	}
	if data[5] != 1 || data[6] != 2 {
		t.Fatalf("unexpected second token deltas: %v %v", data[5], data[6])
	}
}

func decodeTokenTypes(data []protocol.UInteger) map[uint32]bool {
	types := make(map[uint32]bool)
	for i := 0; i+4 < len(data); i += 5 {
		types[uint32(data[i+3])] = true
	}
	return types
}
