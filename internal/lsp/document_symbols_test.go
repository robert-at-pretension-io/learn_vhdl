package lsp

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestDocumentSymbolsHierarchy(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	s.symbolStore.Rebuild(&SymbolIndex{
		Entities: []EntitySummary{
			{
				Name: "uart_tx", File: "/workspace/uart_tx.vhd", Line: 3,
				Ports: []PortSummary{
					{Name: "clk", Direction: "in", Type: "std_logic", File: "/workspace/uart_tx.vhd", Line: 4, InEntity: "uart_tx"},
				},
			},
		},
		Signals: []SignalSummary{
			{Name: "state", Type: "state_t", File: "/workspace/uart_tx.vhd", Line: 12, InEntity: "uart_tx"},
		},
		Architectures: []ArchSummary{
			{Name: "rtl", EntityName: "uart_tx", File: "/workspace/uart_tx.vhd", Line: 20},
		},
	}, s.workspaceRoot)

	res, err := s.textDocumentDocumentSymbol(nil, &protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///workspace/uart_tx.vhd"},
	})
	if err != nil {
		t.Fatalf("document symbols error: %v", err)
	}
	syms, ok := res.([]protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("expected []DocumentSymbol, got %T", res)
	}
	if len(syms) == 0 {
		t.Fatal("expected document symbols")
	}
	foundEntity := false
	foundChild := false
	for _, sym := range syms {
		if sym.Name == "uart_tx" {
			foundEntity = true
			for _, child := range sym.Children {
				if child.Name == "clk" || child.Name == "state" {
					foundChild = true
				}
			}
		}
	}
	if !foundEntity {
		t.Fatal("expected entity symbol uart_tx")
	}
	if !foundChild {
		t.Fatal("expected child symbol under entity")
	}
}
