package lsp

import (
	"fmt"
	"strings"

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

	results := s.symbolStore.FindReferences(word)
	s.streamLocationPartials(params.PartialResultToken, chunkLocations(results, 100))
	return results, nil
}

// textDocumentTypeDefinition handles textDocument/typeDefinition requests.
func (s *Server) textDocumentTypeDefinition(_ *glsp.Context, params *protocol.TypeDefinitionParams) (any, error) {
	word := s.documentStore.WordAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if word == "" {
		return nil, nil
	}

	entries := s.symbolStore.LookupByName(word)
	typeNames := make([]string, 0, len(entries)+1)
	for _, e := range entries {
		if t := extractTypeNameFromEntry(e); t != "" {
			typeNames = append(typeNames, t)
		}
	}
	// If the symbol itself is a type name, allow direct jump.
	typeNames = append(typeNames, word)

	var locs []protocol.Location
	seen := make(map[string]bool)
	for _, t := range typeNames {
		for _, loc := range s.symbolStore.FindDefinition(t) {
			key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
			if !seen[key] {
				seen[key] = true
				locs = append(locs, loc)
			}
		}
	}
	if len(locs) == 0 {
		return nil, nil
	}
	if len(locs) == 1 {
		return locs[0], nil
	}
	return locs, nil
}

// textDocumentImplementation handles textDocument/implementation requests.
func (s *Server) textDocumentImplementation(_ *glsp.Context, params *protocol.ImplementationParams) (any, error) {
	word := s.documentStore.WordAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if word == "" {
		return nil, nil
	}

	all := s.symbolStore.AllEntries()
	entityName := ""
	for _, e := range s.symbolStore.LookupByName(word) {
		switch e.kind {
		case "entity":
			entityName = e.name
		case "component":
			entityName = e.name
		case "instance":
			entityName = targetBaseName(e.detail)
		}
	}
	if entityName == "" {
		entityName = word
	}

	var impls []protocol.Location
	for _, e := range all {
		if e.kind == "architecture" && strings.EqualFold(e.detail, entityName) {
			impls = append(impls, entryToLocation(e, s.workspaceRoot))
		}
	}
	if len(impls) == 0 {
		return nil, nil
	}
	if len(impls) == 1 {
		return impls[0], nil
	}
	return impls, nil
}

// workspaceSymbol handles workspace/symbol requests.
func (s *Server) workspaceSymbol(_ *glsp.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	results := s.symbolStore.WorkspaceSymbols(params.Query)
	s.streamSymbolPartials(params.PartialResultToken, chunkSymbols(results, 100))
	return results, nil
}

func (s *Server) streamSymbolPartials(token *protocol.ProgressToken, chunks [][]protocol.SymbolInformation) {
	if token == nil || s.notifyFunc == nil {
		return
	}
	for _, chunk := range chunks {
		s.notifyFunc(string(protocol.MethodProgress), protocol.ProgressParams{
			Token: *token,
			Value: chunk,
		})
	}
}

func (s *Server) streamLocationPartials(token *protocol.ProgressToken, chunks [][]protocol.Location) {
	if token == nil || s.notifyFunc == nil {
		return
	}
	for _, chunk := range chunks {
		s.notifyFunc(string(protocol.MethodProgress), protocol.ProgressParams{
			Token: *token,
			Value: chunk,
		})
	}
}

func chunkSymbols(items []protocol.SymbolInformation, size int) [][]protocol.SymbolInformation {
	if len(items) == 0 || size <= 0 {
		return nil
	}
	out := make([][]protocol.SymbolInformation, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

func chunkLocations(items []protocol.Location, size int) [][]protocol.Location {
	if len(items) == 0 || size <= 0 {
		return nil
	}
	out := make([][]protocol.Location, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

func extractTypeNameFromEntry(e symbolEntry) string {
	switch e.kind {
	case "signal", "constant":
		return strings.TrimSpace(e.detail)
	case "port":
		parts := strings.Fields(e.detail)
		if len(parts) == 0 {
			return ""
		}
		return parts[len(parts)-1]
	default:
		return ""
	}
}

func targetBaseName(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	parts := strings.Split(target, ".")
	return strings.TrimSpace(parts[len(parts)-1])
}
