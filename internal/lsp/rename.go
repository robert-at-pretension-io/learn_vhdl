package lsp

import (
	"os"
	"strings"
	"unicode"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentPrepareRename(_ *glsp.Context, params *protocol.PrepareRenameParams) (any, error) {
	word, r, ok := s.identifierRangeAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if !ok || word == "" {
		return nil, nil
	}
	return protocol.RangeWithPlaceholder{
		Range:       r,
		Placeholder: word,
	}, nil
}

func (s *Server) textDocumentRename(_ *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	if !isValidIdentifierName(params.NewName) {
		return nil, nil
	}
	oldName, _, ok := s.identifierRangeAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if !ok || oldName == "" {
		return nil, nil
	}

	refs := s.symbolStore.FindReferences(oldName)
	if len(refs) == 0 {
		refs = s.symbolStore.FindDefinition(oldName)
	}
	if len(refs) == 0 {
		refs = []protocol.Location{{
			URI: params.TextDocument.URI,
			Range: protocol.Range{
				Start: params.Position,
				End:   params.Position,
			},
		}}
	}

	linesByURI := make(map[protocol.DocumentUri]map[int]bool)
	for _, loc := range refs {
		if _, ok := linesByURI[loc.URI]; !ok {
			linesByURI[loc.URI] = make(map[int]bool)
		}
		linesByURI[loc.URI][int(loc.Range.Start.Line)] = true
	}

	changes := make(map[protocol.DocumentUri][]protocol.TextEdit)
	for uri, lineSet := range linesByURI {
		text, ok := s.loadDocumentText(uri)
		if !ok {
			continue
		}
		lines := strings.Split(text, "\n")
		for lineNo := range lineSet {
			if lineNo < 0 || lineNo >= len(lines) {
				continue
			}
			for _, rr := range findIdentifierRangesUTF16(lines[lineNo], oldName) {
				changes[uri] = append(changes[uri], protocol.TextEdit{
					Range: protocol.Range{
						Start: protocol.Position{Line: protocol.UInteger(lineNo), Character: protocol.UInteger(rr.start)},
						End:   protocol.Position{Line: protocol.UInteger(lineNo), Character: protocol.UInteger(rr.end)},
					},
					NewText: params.NewName,
				})
			}
		}
	}

	if len(changes) == 0 {
		return nil, nil
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func (s *Server) identifierRangeAtPosition(uri protocol.DocumentUri, line, character int) (string, protocol.Range, bool) {
	if word, start, end, ok := s.documentStore.WordRangeAtPosition(uri, line, character); ok {
		return word, protocol.Range{
			Start: protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(start)},
			End:   protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(end)},
		}, true
	}
	text, ok := s.loadDocumentText(uri)
	if !ok {
		return "", protocol.Range{}, false
	}
	word, start, end, ok := wordRangeAtPosition(text, line, character)
	if !ok {
		return "", protocol.Range{}, false
	}
	return word, protocol.Range{
		Start: protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(start)},
		End:   protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(end)},
	}, true
}

func (s *Server) loadDocumentText(uri protocol.DocumentUri) (string, bool) {
	if text, ok := s.documentStore.Get(uri); ok {
		return text, true
	}
	b, err := os.ReadFile(uriToFile(uri))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func isValidIdentifierName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	runes := []rune(name)
	if len(runes) == 0 {
		return false
	}
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return false
	}
	for _, r := range runes[1:] {
		if !isVHDLIdentChar(r) {
			return false
		}
	}
	return true
}
