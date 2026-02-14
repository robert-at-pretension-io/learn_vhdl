package lsp

import (
	"fmt"
	"sort"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) textDocumentCodeLens(_ *glsp.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	uri := params.TextDocument.URI
	diags := s.cachedDiagnostics(uri)
	var lenses []protocol.CodeLens

	if len(diags) > 0 {
		errs, warns, infos := 0, 0, 0
		missingChecks := 0
		for _, d := range diags {
			if d.Severity != nil {
				switch *d.Severity {
				case protocol.DiagnosticSeverityError:
					errs++
				case protocol.DiagnosticSeverityWarning:
					warns++
				default:
					infos++
				}
			}
			if kind, _, _ := diagnosticActionContext(d); kind == "missing_check" {
				missingChecks++
			}
		}
		lenses = append(lenses, messageCodeLens(
			0,
			fmt.Sprintf("vhdl-lint: %d error(s), %d warning(s), %d info", errs, warns, infos),
		))
		if missingChecks > 0 {
			lenses = append(lenses, messageCodeLens(
				0,
				fmt.Sprintf("verification: %d missing-check hint(s)", missingChecks),
			))
		}
	}

	file := uriToFile(uri)
	entries := s.symbolStore.EntriesInFile(file)
	seen := make(map[string]bool)
	for _, e := range entries {
		key := fmt.Sprintf("%s:%d", e.name, e.line)
		if seen[key] {
			continue
		}
		seen[key] = true
		if !isRootSymbolKind(e.kind) {
			continue
		}
		refs := s.symbolStore.FindReferences(e.name)
		title := fmt.Sprintf("%s: %d symbol occurrence(s)", e.name, len(refs))
		line := lineToLSP(e.line)
		lenses = append(lenses, messageCodeLens(line, title))
	}

	sort.Slice(lenses, func(i, j int) bool {
		return lenses[i].Range.Start.Line < lenses[j].Range.Start.Line
	})
	return lenses, nil
}

func messageCodeLens(line protocol.UInteger, msg string) protocol.CodeLens {
	return protocol.CodeLens{
		Range: lineRange(line),
		Command: &protocol.Command{
			Title:     msg,
			Command:   commandShowMessage,
			Arguments: []any{msg},
		},
	}
}

func (s *Server) workspaceExecuteCommand(_ *glsp.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if params.Command != commandShowMessage {
		return nil, nil
	}
	if len(params.Arguments) > 0 {
		if msg, ok := params.Arguments[0].(string); ok {
			s.logMessage(protocol.MessageTypeInfo, msg)
			return nil, nil
		}
	}
	return nil, nil
}
