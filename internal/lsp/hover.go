package lsp

import (
	"fmt"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// textDocumentHover handles textDocument/hover requests.
func (s *Server) textDocumentHover(_ *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	word := s.documentStore.WordAtPosition(
		params.TextDocument.URI,
		int(params.Position.Line),
		int(params.Position.Character),
	)
	if word == "" {
		return nil, nil
	}

	entries := s.symbolStore.LookupByName(word)
	if len(entries) == 0 {
		return nil, nil
	}

	// Build markdown hover content from all matching entries
	var parts []string
	seen := make(map[string]bool)

	for _, e := range entries {
		content := formatHoverEntry(e)
		if content != "" && !seen[content] {
			seen[content] = true
			parts = append(parts, content)
		}
	}

	if len(parts) == 0 {
		return nil, nil
	}

	md := strings.Join(parts, "\n\n---\n\n")
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: md,
		},
	}, nil
}

// formatHoverEntry creates a markdown hover string for a symbol entry.
func formatHoverEntry(e symbolEntry) string {
	switch e.kind {
	case "entity":
		return fmt.Sprintf("```vhdl\nentity %s\n```", e.name)

	case "architecture":
		if e.detail != "" {
			return fmt.Sprintf("```vhdl\narchitecture %s of %s\n```", e.name, e.detail)
		}
		return fmt.Sprintf("```vhdl\narchitecture %s\n```", e.name)

	case "package":
		return fmt.Sprintf("```vhdl\npackage %s\n```", e.name)

	case "signal":
		decl := fmt.Sprintf("signal %s : %s;", e.name, e.detail)
		md := fmt.Sprintf("```vhdl\n%s\n```", decl)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Declared in %s*", e.inParent)
		}
		return md

	case "port":
		parts := strings.SplitN(e.detail, " ", 2)
		dir := ""
		typ := e.detail
		if len(parts) == 2 {
			dir = parts[0]
			typ = parts[1]
		}
		decl := fmt.Sprintf("%s : %s %s", e.name, dir, typ)
		md := fmt.Sprintf("```vhdl\n%s\n```", strings.TrimSpace(decl))
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Port of entity %s*", e.inParent)
		}
		return md

	case "type":
		kind := e.detail
		if kind == "" {
			kind = "type"
		}
		md := fmt.Sprintf("```vhdl\ntype %s  -- %s\n```", e.name, kind)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Declared in package %s*", e.inParent)
		}
		return md

	case "constant":
		decl := fmt.Sprintf("constant %s : %s;", e.name, e.detail)
		md := fmt.Sprintf("```vhdl\n%s\n```", decl)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Declared in package %s*", e.inParent)
		}
		return md

	case "function":
		sig := fmt.Sprintf("function %s return %s;", e.name, e.detail)
		md := fmt.Sprintf("```vhdl\n%s\n```", sig)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Declared in package %s*", e.inParent)
		}
		return md

	case "procedure":
		md := fmt.Sprintf("```vhdl\nprocedure %s;\n```", e.name)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*Declared in package %s*", e.inParent)
		}
		return md

	case "instance":
		target := e.detail
		if target == "" {
			target = "?"
		}
		md := fmt.Sprintf("```vhdl\n%s : %s  -- instance\n```", e.name, target)
		if e.inParent != "" {
			md += fmt.Sprintf("\n\n*In architecture %s*", e.inParent)
		}
		return md

	case "component":
		return fmt.Sprintf("```vhdl\ncomponent %s\n```", e.name)

	default:
		return fmt.Sprintf("```vhdl\n%s  -- %s\n```", e.name, e.kind)
	}
}
