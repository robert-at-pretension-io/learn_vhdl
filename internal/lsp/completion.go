package lsp

import (
	"sort"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var vhdlKeywords = []string{
	"architecture", "begin", "block", "case", "component", "configuration",
	"constant", "downto", "else", "elsif", "end", "entity", "for", "function",
	"generate", "generic", "if", "in", "is", "library", "loop", "map", "of",
	"out", "package", "port", "procedure", "process", "record", "report", "return",
	"signal", "subtype", "then", "to", "type", "use", "variable", "wait", "when",
}

func (s *Server) textDocumentCompletion(_ *glsp.Context, params *protocol.CompletionParams) (any, error) {
	prefix := strings.ToLower(s.documentStore.PrefixAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	))

	items := make([]protocol.CompletionItem, 0, 128)
	seen := make(map[string]bool)

	add := func(item protocol.CompletionItem) {
		if item.Label == "" {
			return
		}
		key := strings.ToLower(item.Label)
		if seen[key] {
			return
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(item.Label), prefix) {
			return
		}
		seen[key] = true
		items = append(items, item)
	}

	for _, kw := range vhdlKeywords {
		kind := protocol.CompletionItemKindKeyword
		detail := "VHDL keyword"
		add(protocol.CompletionItem{
			Label:  kw,
			Kind:   &kind,
			Detail: &detail,
		})
	}

	for _, e := range s.symbolStore.AllEntries() {
		kind := symbolKindToCompletionKind(e.kind)
		detail := e.kind
		if e.detail != "" {
			detail = e.kind + " " + e.detail
		}
		add(protocol.CompletionItem{
			Label:  e.name,
			Kind:   &kind,
			Detail: &detail,
		})
	}

	for _, snippet := range snippetCompletions() {
		add(snippet)
	}

	sort.Slice(items, func(i, j int) bool {
		a := strings.ToLower(items[i].Label)
		b := strings.ToLower(items[j].Label)
		aPrefix := strings.HasPrefix(a, prefix)
		bPrefix := strings.HasPrefix(b, prefix)
		if aPrefix != bPrefix {
			return aPrefix
		}
		return a < b
	})

	const maxItems = 200
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	return protocol.CompletionList{
		IsIncomplete: len(items) == maxItems,
		Items:        items,
	}, nil
}

func snippetCompletions() []protocol.CompletionItem {
	snippetKind := protocol.CompletionItemKindSnippet
	snippetFormat := protocol.InsertTextFormatSnippet
	return []protocol.CompletionItem{
		{
			Label:            "process (clocked)",
			Kind:             &snippetKind,
			InsertText:       strPtr("process(${1:clk})\nbegin\n  if rising_edge(${1:clk}) then\n    ${0}\n  end if;\nend process;"),
			InsertTextFormat: &snippetFormat,
			Detail:           strPtr("Clocked process snippet"),
		},
		{
			Label:            "entity instantiation",
			Kind:             &snippetKind,
			InsertText:       strPtr("${1:u_inst} : entity work.${2:entity_name}\n  port map (\n    ${0}\n  );"),
			InsertTextFormat: &snippetFormat,
			Detail:           strPtr("Entity instantiation snippet"),
		},
	}
}

func symbolKindToCompletionKind(kind string) protocol.CompletionItemKind {
	switch kind {
	case "entity":
		return protocol.CompletionItemKindClass
	case "architecture", "package":
		return protocol.CompletionItemKindModule
	case "signal", "port":
		return protocol.CompletionItemKindVariable
	case "type":
		return protocol.CompletionItemKindStruct
	case "constant":
		return protocol.CompletionItemKindConstant
	case "function":
		return protocol.CompletionItemKindFunction
	case "procedure":
		return protocol.CompletionItemKindMethod
	case "component":
		return protocol.CompletionItemKindInterface
	default:
		return protocol.CompletionItemKindText
	}
}

func strPtr(s string) *string {
	return &s
}
