package lsp

import (
	"fmt"

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

	for _, diag := range params.Context.Diagnostics {
		// Only handle our own diagnostics
		if diag.Source == nil || *diag.Source != "vhdl-lint" {
			continue
		}

		ruleName := ""
		if diag.Code != nil {
			if s, ok := diag.Code.Value.(string); ok {
				ruleName = s
			}
		}
		if ruleName == "" {
			continue
		}

		title := fmt.Sprintf("Suppress [%s] for this line", ruleName)

		// Insert "-- vhdl_lint:disable rule_name" before the violation line
		// and "-- vhdl_lint:enable rule_name" after it
		line := diag.Range.Start.Line

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

		action := protocol.CodeAction{
			Title: title,
			Kind:  &kind,
			Diagnostics: []protocol.Diagnostic{diag},
			Edit: &protocol.WorkspaceEdit{
				Changes: map[protocol.DocumentUri][]protocol.TextEdit{
					params.TextDocument.URI: edits,
				},
			},
		}
		actions = append(actions, action)
	}

	if len(actions) == 0 {
		return nil, nil
	}
	return actions, nil
}
