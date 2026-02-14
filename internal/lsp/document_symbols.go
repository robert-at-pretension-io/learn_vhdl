package lsp

import (
	"sort"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentDocumentSymbol(_ *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	file := uriToFile(params.TextDocument.URI)
	entries := s.symbolStore.EntriesInFile(file)
	if len(entries) == 0 {
		return []protocol.DocumentSymbol{}, nil
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].line != entries[j].line {
			return entries[i].line < entries[j].line
		}
		return entries[i].name < entries[j].name
	})

	roots := make([]protocol.DocumentSymbol, 0, len(entries))
	parentIndex := make(map[string]int)
	for _, e := range entries {
		ds := symbolEntryToDocumentSymbol(e)
		if e.inParent == "" || isRootSymbolKind(e.kind) {
			parentIndex[e.name] = len(roots)
			roots = append(roots, ds)
			continue
		}
		if idx, ok := parentIndex[e.inParent]; ok {
			roots[idx].Children = append(roots[idx].Children, ds)
		} else {
			roots = append(roots, ds)
		}
	}
	return roots, nil
}

func symbolEntryToDocumentSymbol(e symbolEntry) protocol.DocumentSymbol {
	line := lineToLSP(e.line)
	detail := e.detail
	if detail == "" {
		detail = e.kind
	}
	sym := protocol.DocumentSymbol{
		Name:           e.name,
		Kind:           symbolKindToLSP(e.kind),
		Range:          lineRange(line),
		SelectionRange: lineRange(line),
	}
	sym.Detail = &detail
	return sym
}

func isRootSymbolKind(kind string) bool {
	switch kind {
	case "entity", "architecture", "package", "component":
		return true
	default:
		return false
	}
}
