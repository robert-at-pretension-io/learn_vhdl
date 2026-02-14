package lsp

import (
	"fmt"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// textDocumentCodeAction handles textDocument/codeAction requests.
// Offers "Suppress [rule_name]" quickfix for each diagnostic in the range.
func (s *Server) textDocumentCodeAction(_ *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	if len(params.Context.Diagnostics) == 0 {
		return nil, nil
	}

	var actions []protocol.CodeAction
	kind := protocol.CodeActionKindQuickFix
	seen := make(map[string]bool)

	for _, diag := range params.Context.Diagnostics {
		// Only handle our own diagnostics
		if diag.Source == nil || *diag.Source != "vhdl-lint" {
			continue
		}

		diagKind, ruleName, line := diagnosticActionContext(diag)
		if diagKind != "violation" || ruleName == "" {
			continue
		}
		key := fmt.Sprintf("%s:%d", strings.ToLower(ruleName), line)
		if seen[key] {
			continue
		}
		seen[key] = true

		actions = append(actions, suppressionCodeAction(params.TextDocument.URI, diag, ruleName, line, kind))
	}

	if len(actions) == 0 {
		return nil, nil
	}
	return actions, nil
}

func suppressionCodeAction(uri protocol.DocumentUri, diag protocol.Diagnostic, ruleName string, line protocol.UInteger, kind protocol.CodeActionKind) protocol.CodeAction {
	title := fmt.Sprintf("Suppress [%s] for this line", ruleName)
	disableComment := fmt.Sprintf("-- vhdl_lint:disable %s\n", ruleName)
	enableComment := fmt.Sprintf("-- vhdl_lint:enable %s\n", ruleName)
	edits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: line, Character: 0},
				End:   protocol.Position{Line: line, Character: 0},
			},
			NewText: disableComment,
		},
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: line + 1, Character: 0},
				End:   protocol.Position{Line: line + 1, Character: 0},
			},
			NewText: enableComment,
		},
	}
	return protocol.CodeAction{
		Title:       title,
		Kind:        &kind,
		Diagnostics: []protocol.Diagnostic{diag},
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentUri][]protocol.TextEdit{
				uri: edits,
			},
		},
	}
}

func diagnosticActionContext(diag protocol.Diagnostic) (kind string, ruleName string, line protocol.UInteger) {
	line = diag.Range.Start.Line
	if diag.Code != nil {
		if s, ok := diag.Code.Value.(string); ok {
			ruleName = s
		}
	}
	if diag.Data == nil {
		return "violation", ruleName, line
	}
	m, ok := diag.Data.(map[string]any)
	if !ok {
		return "violation", ruleName, line
	}
	if v, ok := m["kind"].(string); ok && v != "" {
		kind = v
	}
	if v, ok := m["rule"].(string); ok && v != "" {
		ruleName = v
	}
	if v, ok := m["line"]; ok {
		switch n := v.(type) {
		case int:
			if n > 0 {
				line = protocol.UInteger(n - 1)
			}
		case float64:
			if n > 0 {
				line = protocol.UInteger(int(n) - 1)
			}
		}
	}
	if kind == "" {
		kind = "violation"
	}
	return kind, ruleName, line
}
