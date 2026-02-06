package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// textDocumentDefinition handles textDocument/definition requests.
func (s *Server) textDocumentDefinition(_ *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	word := s.documentStore.WordAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if word == "" {
		return nil, nil
	}

	locs := s.symbolStore.FindDefinition(word)
	if len(locs) == 0 {
		return nil, nil
	}
	if len(locs) == 1 {
		return locs[0], nil
	}
	return locs, nil
}

// textDocumentReferences handles textDocument/references requests.
func (s *Server) textDocumentReferences(_ *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	word := s.documentStore.WordAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if word == "" {
		return nil, nil
	}

	return s.symbolStore.FindReferences(word), nil
}

// workspaceSymbol handles workspace/symbol requests.
func (s *Server) workspaceSymbol(_ *glsp.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return s.symbolStore.WorkspaceSymbols(params.Query), nil
}
