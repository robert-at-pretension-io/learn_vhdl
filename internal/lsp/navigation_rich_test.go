package lsp

import (
	"fmt"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestTypeDefinitionFindsTypeFromSignal(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	uri := "file:///workspace/top.vhd"
	s.documentStore.Set(uri, "signal state : state_t;")
	s.symbolStore.Rebuild(&SymbolIndex{
		Signals: []SignalSummary{
			{Name: "state", Type: "state_t", File: "/workspace/top.vhd", Line: 1},
		},
		Types: []TypeSummary{
			{Name: "state_t", Kind: "enum", File: "/workspace/pkg.vhd", Line: 4},
		},
	}, s.workspaceRoot)

	res, err := s.textDocumentTypeDefinition(nil, &protocol.TypeDefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	})
	if err != nil {
		t.Fatalf("type definition error: %v", err)
	}
	found := false
	switch v := res.(type) {
	case protocol.Location:
		found = v.URI == "file:///workspace/pkg.vhd"
	case []protocol.Location:
		for _, loc := range v {
			if loc.URI == "file:///workspace/pkg.vhd" {
				found = true
				break
			}
		}
	default:
		t.Fatalf("unexpected result type %T", res)
	}
	if !found {
		t.Fatalf("expected type definition in pkg.vhd, got %#v", res)
	}
}

func TestImplementationFindsArchitectureForEntity(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	uri := "file:///workspace/top.vhd"
	s.documentStore.Set(uri, "uart_tx")
	s.symbolStore.Rebuild(&SymbolIndex{
		Entities: []EntitySummary{
			{Name: "uart_tx", File: "/workspace/uart_tx.vhd", Line: 1},
		},
		Architectures: []ArchSummary{
			{Name: "rtl", EntityName: "uart_tx", File: "/workspace/uart_tx.vhd", Line: 10},
		},
	}, s.workspaceRoot)

	res, err := s.textDocumentImplementation(nil, &protocol.ImplementationParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 2},
		},
	})
	if err != nil {
		t.Fatalf("implementation error: %v", err)
	}
	loc, ok := res.(protocol.Location)
	if !ok {
		t.Fatalf("expected single location, got %T", res)
	}
	if loc.URI != "file:///workspace/uart_tx.vhd" {
		t.Fatalf("unexpected implementation URI: %s", loc.URI)
	}
}

func TestWorkspaceSymbolStreamsPartialResults(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	var signals []SignalSummary
	for i := 0; i < 250; i++ {
		signals = append(signals, SignalSummary{
			Name: fmt.Sprintf("sig_%03d", i),
			Type: "std_logic",
			File: fmt.Sprintf("/workspace/f_%03d.vhd", i),
			Line: 1,
		})
	}
	s.symbolStore.Rebuild(&SymbolIndex{Signals: signals}, s.workspaceRoot)

	progressCalls := 0
	s.notifyFunc = func(method string, _ any) {
		if method == string(protocol.MethodProgress) {
			progressCalls++
		}
	}
	token := protocol.ProgressToken{Value: "partial-symbols"}
	res, err := s.workspaceSymbol(nil, &protocol.WorkspaceSymbolParams{
		Query: "",
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: &token,
		},
	})
	if err != nil {
		t.Fatalf("workspace symbol error: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected workspace symbols")
	}
	if progressCalls == 0 {
		t.Fatal("expected partial-result progress notifications")
	}
}

func TestReferencesStreamsPartialResults(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	uri := "file:///workspace/top.vhd"
	s.documentStore.Set(uri, "clk")
	var signals []SignalSummary
	for i := 0; i < 220; i++ {
		signals = append(signals, SignalSummary{
			Name: "clk",
			Type: "std_logic",
			File: fmt.Sprintf("/workspace/f_%03d.vhd", i),
			Line: 2,
		})
	}
	s.symbolStore.Rebuild(&SymbolIndex{Signals: signals}, s.workspaceRoot)

	progressCalls := 0
	s.notifyFunc = func(method string, _ any) {
		if method == string(protocol.MethodProgress) {
			progressCalls++
		}
	}
	token := protocol.ProgressToken{Value: "partial-refs"}
	locs, err := s.textDocumentReferences(nil, &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 0, Character: 1},
		},
		PartialResultParams: protocol.PartialResultParams{
			PartialResultToken: &token,
		},
	})
	if err != nil {
		t.Fatalf("references error: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("expected references")
	}
	if progressCalls == 0 {
		t.Fatal("expected partial-result progress notifications for references")
	}
}
