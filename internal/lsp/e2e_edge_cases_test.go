package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

var (
	e2eServerBuildOnce sync.Once
	e2eServerBinPath   string
	e2eServerBuildErr  error
)

type e2eSession struct {
	t                *testing.T
	workspace        string
	rpc              *lockedRPCWriter
	collector        *messageCollector
	errCh            <-chan error
	cmd              *exec.Cmd
	nextID           int
	initResult       map[string]any
	initialLintToken string
	textsByURI       map[protocol.DocumentUri]string
	versions         map[protocol.DocumentUri]int
}

func TestLSPStdioE2EEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*testing.T, *e2eSession)
	}{
		{
			name: "initialize_completion_trigger_characters",
			fn: func(t *testing.T, s *e2eSession) {
				provider := mustMapField(t, mustMapField(t, s.initResult, "capabilities"), "completionProvider")
				triggers := mustStringSliceField(t, provider, "triggerCharacters")
				if !containsString(triggers, ".") || !containsString(triggers, ":") {
					t.Fatalf("expected completion trigger chars to include '.' and ':', got %v", triggers)
				}
			},
		},
		{
			name: "initialize_rename_prepare_provider",
			fn: func(t *testing.T, s *e2eSession) {
				renameProvider := mustMapField(t, mustMapField(t, s.initResult, "capabilities"), "renameProvider")
				prepare, ok := renameProvider["prepareProvider"].(bool)
				if !ok || !prepare {
					t.Fatalf("expected prepareProvider=true, got %v", renameProvider["prepareProvider"])
				}
			},
		},
		{
			name: "initialize_semantic_tokens_legend_contains_comment",
			fn: func(t *testing.T, s *e2eSession) {
				semanticProvider := mustMapField(t, mustMapField(t, s.initResult, "capabilities"), "semanticTokensProvider")
				legend := mustMapField(t, semanticProvider, "legend")
				types := mustStringSliceField(t, legend, "tokenTypes")
				if !containsString(types, "comment") {
					t.Fatalf("expected semantic token types to include comment, got %v", types)
				}
			},
		},
		{
			name: "document_symbol_unknown_file_returns_empty",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.uri("unknown.vhd")
				resp := s.request(t, "textDocument/documentSymbol", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				})
				var symbols []protocol.DocumentSymbol
				mustDecodeResult(t, resp, &symbols)
				if len(symbols) != 0 {
					t.Fatalf("expected no symbols, got %d", len(symbols))
				}
			},
		},
		{
			name: "completion_unknown_prefix_returns_empty",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocument(t, "scratch.vhd", "zzz")
				resp := s.request(t, "textDocument/completion", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 0, "character": 3},
				})
				var list struct {
					Items []protocol.CompletionItem `json:"items"`
				}
				mustDecodeResult(t, resp, &list)
				if len(list.Items) != 0 {
					t.Fatalf("expected no completion items, got %d", len(list.Items))
				}
			},
		},
		{
			name: "completion_prefix_matching_is_case_insensitive",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocument(t, "scratch.vhd", "Pr")
				resp := s.request(t, "textDocument/completion", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 0, "character": 2},
				})
				var list struct {
					Items []protocol.CompletionItem `json:"items"`
				}
				mustDecodeResult(t, resp, &list)
				if !completionContainsLabel(list.Items, "process") {
					t.Fatalf("expected 'process' completion item, got %d items", len(list.Items))
				}
			},
		},
		{
			name: "completion_includes_snippet_items",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocument(t, "scratch.vhd", "")
				resp := s.request(t, "textDocument/completion", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 0, "character": 0},
				})
				var list struct {
					Items []protocol.CompletionItem `json:"items"`
				}
				mustDecodeResult(t, resp, &list)
				if !completionContainsLabel(list.Items, "process (clocked)") {
					t.Fatalf("expected snippet completion label, got %d items", len(list.Items))
				}
			},
		},
		{
			name: "hover_unknown_symbol_returns_null",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocument(t, "scratch.vhd", "zzz")
				resp := s.request(t, "textDocument/hover", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 0, "character": 1},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null hover result, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "hover_known_signal_includes_type",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "state", 1)
				resp := s.request(t, "textDocument/hover", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
				})
				var hover struct {
					Contents struct {
						Value string `json:"value"`
					} `json:"contents"`
				}
				mustDecodeResult(t, resp, &hover)
				if !strings.Contains(hover.Contents.Value, "signal state : state_t;") {
					t.Fatalf("expected hover to include signal declaration, got %q", hover.Contents.Value)
				}
			},
		},
		{
			name: "definition_on_whitespace_returns_null",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/definition", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 3, "character": 0},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null definition result, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "definition_for_entity_returns_location",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "uart_tx", 1)
				resp := s.request(t, "textDocument/definition", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
				})
				locs := mustDecodeLocations(t, resp.Result)
				if len(locs) == 0 {
					t.Fatal("expected at least one definition location")
				}
				foundTop := false
				for _, loc := range locs {
					if samePath(uriToFile(loc.URI), filepath.Join(s.workspace, "top.vhd")) {
						foundTop = true
						break
					}
				}
				if !foundTop {
					t.Fatalf("expected one definition in top.vhd, got %+v", locs)
				}
			},
		},
		{
			name: "references_for_entity_returns_multiple_locations",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "uart_tx", 1)
				resp := s.request(t, "textDocument/references", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
					"context":      map[string]any{"includeDeclaration": true},
				})
				var refs []protocol.Location
				mustDecodeResult(t, resp, &refs)
				if len(refs) < 2 {
					t.Fatalf("expected at least 2 refs for uart_tx, got %d", len(refs))
				}
			},
		},
		{
			name: "references_stream_partial_results",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "uart_tx", 1)
				token := "refs-partial"
				resp := s.request(t, "textDocument/references", map[string]any{
					"textDocument":       map[string]any{"uri": uri},
					"position":           map[string]any{"line": pos.Line, "character": pos.Character},
					"context":            map[string]any{"includeDeclaration": true},
					"partialResultToken": token,
				})
				var refs []protocol.Location
				mustDecodeResult(t, resp, &refs)
				if len(refs) == 0 {
					t.Fatal("expected non-empty references")
				}
				if got := s.waitPartialArrayProgress(t, "", 5*time.Second); got == 0 {
					t.Fatal("expected non-empty partial references chunk")
				}
			},
		},
		{
			name: "workspace_symbol_no_match_returns_empty",
			fn: func(t *testing.T, s *e2eSession) {
				resp := s.request(t, "workspace/symbol", map[string]any{"query": "does_not_exist"})
				var syms []protocol.SymbolInformation
				mustDecodeResult(t, resp, &syms)
				if len(syms) != 0 {
					t.Fatalf("expected zero symbols, got %d", len(syms))
				}
			},
		},
		{
			name: "workspace_symbol_streams_partial_results",
			fn: func(t *testing.T, s *e2eSession) {
				token := "workspace-partial"
				resp := s.request(t, "workspace/symbol", map[string]any{
					"query":              "uart",
					"partialResultToken": token,
				})
				var syms []protocol.SymbolInformation
				mustDecodeResult(t, resp, &syms)
				if len(syms) == 0 {
					t.Fatal("expected symbol results")
				}
				if got := s.waitPartialArrayProgress(t, "", 5*time.Second); got == 0 {
					t.Fatal("expected non-empty partial workspace symbol chunk")
				}
			},
		},
		{
			name: "type_definition_from_signal_resolves_type",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "state", 1)
				resp := s.request(t, "textDocument/typeDefinition", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
				})
				locs := mustDecodeLocations(t, resp.Result)
				if len(locs) == 0 {
					t.Fatal("expected at least one type definition location")
				}
				foundPkg := false
				for _, loc := range locs {
					if samePath(uriToFile(loc.URI), filepath.Join(s.workspace, "pkg.vhd")) {
						foundPkg = true
						break
					}
				}
				if !foundPkg {
					t.Fatalf("expected one type definition in pkg.vhd, got %+v", locs)
				}
			},
		},
		{
			name: "implementation_for_entity_resolves_architecture",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "uart_tx", 1)
				resp := s.request(t, "textDocument/implementation", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
				})
				var loc protocol.Location
				mustDecodeResult(t, resp, &loc)
				if !samePath(uriToFile(loc.URI), filepath.Join(s.workspace, "top.vhd")) {
					t.Fatalf("expected implementation in top.vhd, got %s", loc.URI)
				}
			},
		},
		{
			name: "prepare_rename_on_whitespace_returns_null",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/prepareRename", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": 3, "character": 0},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null prepareRename result, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "prepare_rename_returns_range_with_placeholder",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "state", 1)
				resp := s.request(t, "textDocument/prepareRename", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
				})
				var out struct {
					Placeholder string         `json:"placeholder"`
					Range       protocol.Range `json:"range"`
				}
				mustDecodeResult(t, resp, &out)
				if out.Placeholder != "state" {
					t.Fatalf("expected placeholder 'state', got %q", out.Placeholder)
				}
			},
		},
		{
			name: "rename_rejects_invalid_new_identifier",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "state", 1)
				resp := s.request(t, "textDocument/rename", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
					"newName":      "1invalid",
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null rename result for invalid identifier, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "rename_returns_workspace_edits_for_valid_identifier",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				pos := s.mustPositionInDocument(t, uri, "state", 1)
				resp := s.request(t, "textDocument/rename", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"position":     map[string]any{"line": pos.Line, "character": pos.Character},
					"newName":      "state_next",
				})
				var edit protocol.WorkspaceEdit
				mustDecodeResult(t, resp, &edit)
				if len(edit.Changes) == 0 {
					t.Fatal("expected rename workspace edit changes")
				}
			},
		},
		{
			name: "semantic_tokens_missing_file_returns_empty",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.uri("missing_on_disk.vhd")
				resp := s.request(t, "textDocument/semanticTokens/full", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				})
				var tokens protocol.SemanticTokens
				mustDecodeResult(t, resp, &tokens)
				if len(tokens.Data) != 0 {
					t.Fatalf("expected empty token data, got %d", len(tokens.Data))
				}
			},
		},
		{
			name: "semantic_tokens_open_document_non_empty",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/semanticTokens/full", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				})
				var tokens protocol.SemanticTokens
				mustDecodeResult(t, resp, &tokens)
				if len(tokens.Data) == 0 {
					t.Fatal("expected semantic token data")
				}
			},
		},
		{
			name: "code_action_ignores_non_vhdl_source",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/codeAction", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 1},
					},
					"context": map[string]any{
						"diagnostics": []map[string]any{{
							"source":  "other-tool",
							"code":    "unused_signal",
							"message": "x",
							"range": map[string]any{
								"start": map[string]any{"line": 0, "character": 0},
								"end":   map[string]any{"line": 0, "character": 1},
							},
						}},
					},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null code action result, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "code_action_ignores_parse_error_kind",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/codeAction", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 1},
					},
					"context": map[string]any{
						"diagnostics": []map[string]any{{
							"source":  "vhdl-lint",
							"code":    "parse_error",
							"message": "x",
							"data":    map[string]any{"kind": "parse_error"},
							"range": map[string]any{
								"start": map[string]any{"line": 0, "character": 0},
								"end":   map[string]any{"line": 0, "character": 1},
							},
						}},
					},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null code action result for parse_error, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "code_action_returns_quickfix_for_violation",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				resp := s.request(t, "textDocument/codeAction", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 1},
					},
					"context": map[string]any{
						"diagnostics": []map[string]any{{
							"source":  "vhdl-lint",
							"code":    "unused_signal",
							"message": "unused signal",
							"data": map[string]any{
								"kind": "violation",
								"rule": "unused_signal",
								"line": float64(1),
							},
							"range": map[string]any{
								"start": map[string]any{"line": 0, "character": 0},
								"end":   map[string]any{"line": 0, "character": 1},
							},
						}},
					},
				})
				var actions []protocol.CodeAction
				mustDecodeResult(t, resp, &actions)
				if len(actions) != 1 {
					t.Fatalf("expected 1 code action, got %d", len(actions))
				}
				if !strings.Contains(actions[0].Title, "Suppress [unused_signal]") {
					t.Fatalf("unexpected code action title: %q", actions[0].Title)
				}
			},
		},
		{
			name: "code_action_deduplicates_same_rule_and_line",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				diag := map[string]any{
					"source":  "vhdl-lint",
					"code":    "unused_signal",
					"message": "unused signal",
					"data": map[string]any{
						"kind": "violation",
						"rule": "unused_signal",
						"line": float64(1),
					},
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 1},
					},
				}
				resp := s.request(t, "textDocument/codeAction", map[string]any{
					"textDocument": map[string]any{"uri": uri},
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						"end":   map[string]any{"line": 0, "character": 1},
					},
					"context": map[string]any{
						"diagnostics": []map[string]any{diag, diag},
					},
				})
				var actions []protocol.CodeAction
				mustDecodeResult(t, resp, &actions)
				if len(actions) != 1 {
					t.Fatalf("expected deduped single action, got %d", len(actions))
				}
			},
		},
		{
			name: "codelens_unknown_file_returns_empty",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.uri("unknown.vhd")
				resp := s.request(t, "textDocument/codeLens", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				})
				var lenses []protocol.CodeLens
				mustDecodeResult(t, resp, &lenses)
				if len(lenses) != 0 {
					t.Fatalf("expected no code lenses, got %d", len(lenses))
				}
			},
		},
		{
			name: "codelens_after_diagnostics_includes_summary",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "multi.vhd")
				d := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(d.Diagnostics) < 2 {
					t.Fatalf("expected >=2 diagnostics for multi.vhd, got %d", len(d.Diagnostics))
				}
				resp := s.request(t, "textDocument/codeLens", map[string]any{
					"textDocument": map[string]any{"uri": uri},
				})
				var lenses []protocol.CodeLens
				mustDecodeResult(t, resp, &lenses)
				if len(lenses) == 0 {
					t.Fatal("expected code lenses")
				}
				found := false
				for _, lens := range lenses {
					if lens.Command != nil && strings.Contains(lens.Command.Title, "vhdl-lint:") {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected summary codelens, got %v", lenses)
				}
			},
		},
		{
			name: "workspace_execute_command_unknown_returns_null",
			fn: func(t *testing.T, s *e2eSession) {
				resp := s.request(t, "workspace/executeCommand", map[string]any{
					"command":   "unknown.command",
					"arguments": []any{"hello"},
				})
				if !resultIsNull(resp.Result) {
					t.Fatalf("expected null executeCommand result, got %s", string(resp.Result))
				}
			},
		},
		{
			name: "workspace_execute_command_show_message_logs",
			fn: func(t *testing.T, s *e2eSession) {
				msg := "hello from e2e"
				_ = s.request(t, "workspace/executeCommand", map[string]any{
					"command":   commandShowMessage,
					"arguments": []any{msg},
				})
				log := s.waitLogMessage(t, 5*time.Second, func(p protocol.LogMessageParams) bool {
					return strings.Contains(p.Message, msg)
				})
				if log.Message != msg {
					t.Fatalf("expected log message %q, got %q", msg, log.Message)
				}
			},
		},
		{
			name: "did_open_clean_file_publishes_empty_diagnostics",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "clean.vhd")
				d := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(d.Diagnostics) != 0 {
					t.Fatalf("expected no diagnostics for clean.vhd, got %d", len(d.Diagnostics))
				}
			},
		},
		{
			name: "did_change_republishes_incremental_diagnostics",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				first := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(first.Diagnostics) == 0 {
					t.Fatal("expected initial diagnostics after didOpen")
				}
				s.didChangeDocument(t, uri, s.textsByURI[uri]+"\n-- change")
				second := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(second.Diagnostics) == 0 {
					t.Fatal("expected diagnostics after didChange")
				}
			},
		},
		{
			name: "did_open_parse_error_file_publishes_parse_diagnostic",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "parseerr.vhd")
				d := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(d.Diagnostics) != 1 {
					t.Fatalf("expected 1 parse diagnostic, got %d", len(d.Diagnostics))
				}
				kind, _, _ := diagnosticActionContext(d.Diagnostics[0])
				if kind != "parse_error" {
					t.Fatalf("expected parse_error diagnostic kind, got %q", kind)
				}
			},
		},
		{
			name: "did_open_missing_check_file_includes_related_info",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "missing.vhd")
				d := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(d.Diagnostics) != 1 {
					t.Fatalf("expected 1 missing-check diagnostic, got %d", len(d.Diagnostics))
				}
				diag := d.Diagnostics[0]
				kind, _, _ := diagnosticActionContext(diag)
				if kind != "missing_check" {
					t.Fatalf("expected missing_check kind, got %q", kind)
				}
				if len(diag.RelatedInformation) == 0 {
					t.Fatal("expected related information for missing check")
				}
			},
		},
		{
			name: "did_open_ambiguous_file_publishes_hint_diagnostic",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "ambig.vhd")
				d := s.waitDiagnostics(t, uri, 5*time.Second)
				if len(d.Diagnostics) != 1 {
					t.Fatalf("expected 1 ambiguous diagnostic, got %d", len(d.Diagnostics))
				}
				diag := d.Diagnostics[0]
				kind, _, _ := diagnosticActionContext(diag)
				if kind != "ambiguous_construct" {
					t.Fatalf("expected ambiguous_construct kind, got %q", kind)
				}
				if diag.Severity == nil || *diag.Severity != protocol.DiagnosticSeverityHint {
					t.Fatalf("expected hint severity, got %#v", diag.Severity)
				}
			},
		},
		{
			name: "did_open_badjson_logs_incremental_failure",
			fn: func(t *testing.T, s *e2eSession) {
				_ = s.openDocumentFixture(t, "badjson.vhd")
				log := s.waitLogMessage(t, 5*time.Second, func(p protocol.LogMessageParams) bool {
					return strings.Contains(strings.ToLower(p.Message), "incremental failed")
				})
				if log.Type != protocol.MessageTypeWarning {
					t.Fatalf("expected warning log, got type %d", log.Type)
				}
			},
		},
		{
			name: "did_open_hardfail_logs_incremental_failure",
			fn: func(t *testing.T, s *e2eSession) {
				_ = s.openDocumentFixture(t, "hardfail.vhd")
				log := s.waitLogMessage(t, 5*time.Second, func(p protocol.LogMessageParams) bool {
					return strings.Contains(strings.ToLower(p.Message), "incremental failed")
				})
				if log.Type != protocol.MessageTypeWarning {
					t.Fatalf("expected warning log, got type %d", log.Type)
				}
			},
		},
		{
			name: "incremental_lint_emits_progress_begin_report_end",
			fn: func(t *testing.T, s *e2eSession) {
				_ = s.openDocumentFixture(t, "top.vhd")
				begin := s.waitWorkDoneProgress(t, 5*time.Second, func(token, kind, _ string) bool {
					return kind == "begin" &&
						strings.HasPrefix(token, "vhdl-lsp/lint/") &&
						token != s.initialLintToken
				})
				token := begin.Token
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(tok, kind, message string) bool {
					return tok == token && kind == "report"
				})
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(tok, kind, _ string) bool {
					return tok == token && kind == "end"
				})
			},
		},
		{
			name: "did_save_triggers_full_lint_progress_report",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(token, kind, _ string) bool {
					return kind == "end" && strings.HasPrefix(token, "vhdl-lsp/lint/")
				})
				s.didSaveDocument(t, uri)
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(_ string, kind, message string) bool {
					return kind == "report" && strings.Contains(message, "Running full lint")
				})
			},
		},
		{
			name: "did_close_triggers_full_lint_progress_report",
			fn: func(t *testing.T, s *e2eSession) {
				uri := s.openDocumentFixture(t, "top.vhd")
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(token, kind, _ string) bool {
					return kind == "end" && strings.HasPrefix(token, "vhdl-lsp/lint/")
				})
				s.didCloseDocument(t, uri)
				_ = s.waitWorkDoneProgress(t, 5*time.Second, func(_ string, kind, message string) bool {
					return kind == "report" && strings.Contains(message, "Running full lint")
				})
			},
		},
	}

	if len(tests) < 30 {
		t.Fatalf("expected at least 30 edge cases, got %d", len(tests))
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := startE2ESession(t)
			tc.fn(t, s)
		})
	}
}

func startE2ESession(t *testing.T) *e2eSession {
	t.Helper()

	repoRoot := findRepoRoot(t)
	workspace := t.TempDir()
	files := defaultE2EFiles()
	for rel, content := range files {
		path := filepath.Join(workspace, rel)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", rel, err)
		}
	}

	fakeLint := filepath.Join(workspace, "fake_vhdl_lint.sh")
	if err := os.WriteFile(fakeLint, []byte(defaultFakeLintScript()), 0o755); err != nil {
		t.Fatalf("write fake lint script: %v", err)
	}

	serverBin := mustBuildLSPServerBinary(t, repoRoot)
	cmd := exec.Command(serverBin)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"VHDL_LINT_BIN="+fakeLint,
		"VHDL_LSP_DEBOUNCE_MS=5",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vhdl-lsp: %v", err)
	}

	msgCh := make(chan rpcEnvelope, 512)
	errCh := make(chan error, 1)
	go readRPCStream(stdout, msgCh, errCh)
	go func() {
		_, _ = io.Copy(io.Discard, stderr)
	}()

	rpc := &lockedRPCWriter{w: stdin}
	collector := &messageCollector{
		ch: msgCh,
		onServerRequest: func(msg rpcEnvelope) error {
			return respondToServerRequest(rpc, msg)
		},
	}

	s := &e2eSession{
		t:          t,
		workspace:  workspace,
		rpc:        rpc,
		collector:  collector,
		errCh:      errCh,
		cmd:        cmd,
		nextID:     2,
		textsByURI: make(map[protocol.DocumentUri]string),
		versions:   make(map[protocol.DocumentUri]int),
	}

	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	rootURI := "file://" + filepath.ToSlash(workspace)
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"rootUri":      rootURI,
			"capabilities": map[string]any{},
		},
	}); err != nil {
		t.Fatalf("send initialize: %v", err)
	}
	initResp := collector.waitResponse(t, 1, 5*time.Second)
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %+v", initResp.Error)
	}
	mustDecodeResult(t, initResp, &s.initResult)

	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialized",
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("send initialized: %v", err)
	}

	// Wait for initial full lint to finish so symbol-based features are stable.
	initialEnd := s.waitWorkDoneProgress(t, 5*time.Second, func(token, kind, _ string) bool {
		return kind == "end" && strings.HasPrefix(token, "vhdl-lsp/lint/")
	})
	s.initialLintToken = initialEnd.Token

	return s
}

func (s *e2eSession) request(t *testing.T, method string, params any) rpcEnvelope {
	t.Helper()
	id := s.nextID
	s.nextID++
	if err := s.rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		t.Fatalf("send request %s: %v", method, err)
	}
	resp := s.collector.waitResponse(t, id, 5*time.Second)
	if resp.Error != nil {
		t.Fatalf("request %s returned error: %+v", method, resp.Error)
	}
	return resp
}

func (s *e2eSession) notify(t *testing.T, method string, params any) {
	t.Helper()
	if err := s.rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}); err != nil {
		t.Fatalf("send notification %s: %v", method, err)
	}
}

func (s *e2eSession) uri(rel string) protocol.DocumentUri {
	return protocol.DocumentUri("file://" + filepath.ToSlash(filepath.Join(s.workspace, rel)))
}

func (s *e2eSession) openDocumentFixture(t *testing.T, rel string) protocol.DocumentUri {
	t.Helper()
	text, ok := defaultE2EFiles()[rel]
	if !ok {
		t.Fatalf("fixture not found: %s", rel)
	}
	return s.openDocument(t, rel, text)
}

func (s *e2eSession) openDocument(t *testing.T, rel, text string) protocol.DocumentUri {
	t.Helper()
	uri := s.uri(rel)
	s.versions[uri] = 1
	s.textsByURI[uri] = text
	s.notify(t, "textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": "vhdl",
			"version":    1,
			"text":       text,
		},
	})
	return uri
}

func (s *e2eSession) didChangeDocument(t *testing.T, uri protocol.DocumentUri, text string) {
	t.Helper()
	version := s.versions[uri] + 1
	s.versions[uri] = version
	s.textsByURI[uri] = text
	s.notify(t, "textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]any{{
			"text": text,
		}},
	})
}

func (s *e2eSession) didSaveDocument(t *testing.T, uri protocol.DocumentUri) {
	t.Helper()
	s.notify(t, "textDocument/didSave", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

func (s *e2eSession) didCloseDocument(t *testing.T, uri protocol.DocumentUri) {
	t.Helper()
	s.notify(t, "textDocument/didClose", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
	delete(s.textsByURI, uri)
	delete(s.versions, uri)
}

func (s *e2eSession) waitDiagnostics(t *testing.T, uri protocol.DocumentUri, timeout time.Duration) protocol.PublishDiagnosticsParams {
	t.Helper()
	n := s.collector.waitNotification(t, string(protocol.ServerTextDocumentPublishDiagnostics), timeout, func(m rpcEnvelope) bool {
		var p protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return false
		}
		return p.URI == uri
	})
	var out protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(n.Params, &out); err != nil {
		t.Fatalf("decode diagnostics notification: %v", err)
	}
	return out
}

func (s *e2eSession) waitLogMessage(t *testing.T, timeout time.Duration, match func(protocol.LogMessageParams) bool) protocol.LogMessageParams {
	t.Helper()
	n := s.collector.waitNotification(t, "window/logMessage", timeout, func(m rpcEnvelope) bool {
		var p protocol.LogMessageParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return false
		}
		if match == nil {
			return true
		}
		return match(p)
	})
	var out protocol.LogMessageParams
	if err := json.Unmarshal(n.Params, &out); err != nil {
		t.Fatalf("decode log message notification: %v", err)
	}
	return out
}

type workDoneProgressEvent struct {
	Token   string
	Kind    string
	Message string
}

func (s *e2eSession) waitWorkDoneProgress(t *testing.T, timeout time.Duration, match func(token, kind, message string) bool) workDoneProgressEvent {
	t.Helper()
	n := s.collector.waitNotification(t, string(protocol.MethodProgress), timeout, func(m rpcEnvelope) bool {
		tok, kind, msg, ok := decodeWorkDoneProgress(m.Params)
		if !ok {
			return false
		}
		if match == nil {
			return true
		}
		return match(tok, kind, msg)
	})
	tok, kind, msg, ok := decodeWorkDoneProgress(n.Params)
	if !ok {
		t.Fatalf("expected work-done progress payload, got %s", string(n.Params))
	}
	return workDoneProgressEvent{Token: tok, Kind: kind, Message: msg}
}

func (s *e2eSession) waitPartialArrayProgress(t *testing.T, token string, timeout time.Duration) int {
	t.Helper()
	n := s.collector.waitNotification(t, string(protocol.MethodProgress), timeout, func(m rpcEnvelope) bool {
		ptok, arrLen, ok := decodePartialProgressArray(m.Params)
		return ok && ptok == token && arrLen > 0
	})
	_, arrLen, ok := decodePartialProgressArray(n.Params)
	if !ok {
		t.Fatalf("expected partial array progress payload, got %s", string(n.Params))
	}
	return arrLen
}

func (s *e2eSession) mustPositionInDocument(t *testing.T, uri protocol.DocumentUri, needle string, occurrence int) protocol.Position {
	t.Helper()
	text := s.textsByURI[uri]
	line, ch, ok := findOccurrencePosition(text, needle, occurrence)
	if !ok {
		t.Fatalf("needle %q occurrence %d not found in %s", needle, occurrence, uri)
	}
	return protocol.Position{Line: protocol.UInteger(line), Character: protocol.UInteger(ch)}
}

func findOccurrencePosition(text, needle string, occurrence int) (int, int, bool) {
	if occurrence <= 0 {
		occurrence = 1
	}
	idx := -1
	from := 0
	for i := 0; i < occurrence; i++ {
		next := strings.Index(text[from:], needle)
		if next < 0 {
			return 0, 0, false
		}
		idx = from + next
		from = idx + len(needle)
	}
	line := strings.Count(text[:idx], "\n")
	lastNL := strings.LastIndex(text[:idx], "\n")
	if lastNL < 0 {
		return line, idx, true
	}
	return line, idx - lastNL - 1, true
}

func mustBuildLSPServerBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	e2eServerBuildOnce.Do(func() {
		buildDir, err := os.MkdirTemp("", "vhdl_lsp_e2e_bin_*")
		if err != nil {
			e2eServerBuildErr = err
			return
		}
		bin := filepath.Join(buildDir, "vhdl-lsp")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/vhdl-lsp")
		cmd.Dir = repoRoot
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			e2eServerBuildErr = fmt.Errorf("build vhdl-lsp: %w: %s", err, string(out))
			return
		}
		e2eServerBinPath = bin
	})
	if e2eServerBuildErr != nil {
		t.Fatalf("build server binary: %v", e2eServerBuildErr)
	}
	return e2eServerBinPath
}

func decodeWorkDoneProgress(params json.RawMessage) (token, kind, message string, ok bool) {
	var p map[string]any
	if err := json.Unmarshal(params, &p); err != nil {
		return "", "", "", false
	}
	token = extractProgressToken(mapValue(p, "token", "Token"))
	if token == "" || token == "<nil>" {
		return "", "", "", false
	}
	v, ok := mapValue(p, "value", "Value").(map[string]any)
	if !ok {
		return "", "", "", false
	}
	k, _ := mapValue(v, "kind", "Kind").(string)
	if k == "" {
		return "", "", "", false
	}
	m, _ := mapValue(v, "message", "Message").(string)
	return token, k, m, true
}

func decodePartialProgressArray(params json.RawMessage) (token string, arrLen int, ok bool) {
	var p map[string]any
	if err := json.Unmarshal(params, &p); err != nil {
		return "", 0, false
	}
	token = extractProgressToken(mapValue(p, "token", "Token"))
	if token == "<nil>" {
		token = ""
	}
	arr, ok := mapValue(p, "value", "Value").([]any)
	if !ok {
		return "", 0, false
	}
	return token, len(arr), true
}

func extractProgressToken(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case map[string]any:
		if inner := mapValue(v, "value", "Value"); inner != nil {
			return extractProgressToken(inner)
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

func mapValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func resultIsNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func mustDecodeResult(t *testing.T, resp rpcEnvelope, out any) {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("response error: %+v", resp.Error)
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		t.Fatalf("decode response result: %v (%s)", err, string(resp.Result))
	}
}

func mustDecodeLocations(t *testing.T, raw json.RawMessage) []protocol.Location {
	t.Helper()
	if resultIsNull(raw) {
		return nil
	}
	var one protocol.Location
	if err := json.Unmarshal(raw, &one); err == nil && one.URI != "" {
		return []protocol.Location{one}
	}
	var many []protocol.Location
	if err := json.Unmarshal(raw, &many); err != nil {
		t.Fatalf("decode location(s): %v (%s)", err, string(raw))
	}
	return many
}

func completionContainsLabel(items []protocol.CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}

func mustMapField(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in map %v", key, m)
	}
	out, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected key %q to be map, got %T", key, v)
	}
	return out
}

func mustStringSliceField(t *testing.T, m map[string]any, key string) []string {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("missing key %q in map %v", key, m)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected %q to be []any, got %T", key, raw)
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s, ok := x.(string)
		if !ok {
			t.Fatalf("expected string in %q array, got %T", key, x)
		}
		out = append(out, s)
	}
	return out
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func defaultE2EFiles() map[string]string {
	return map[string]string{
		"top.vhd": `entity uart_tx is
  port (clk : in std_logic);
end entity;

architecture rtl of uart_tx is
  signal state : state_t;
begin
  u_child : entity work.uart_tx
    port map (clk => clk);
  process(clk)
  begin
    if rising_edge(clk) then
      state <= state;
    end if;
  end process;
end architecture;
`,
		"pkg.vhd": `package util_pkg is
  type state_t is (idle, busy);
  constant WIDTH_C : integer := 8;
  function to_slv return std_logic_vector;
  procedure tick;
end package;
`,
		"scratch.vhd": "",
		"clean.vhd":   "entity clean is end entity;",
		"multi.vhd":   "signal a : std_logic;\nsignal b : std_logic;",
		"parseerr.vhd": `entity parseerr is
begin
  ???
end entity;
`,
		"missing.vhd":  "architecture rtl of missing is begin null; end architecture;",
		"ambig.vhd":    "architecture rtl of ambig is begin null; end architecture;",
		"badjson.vhd":  "entity badjson is end entity;",
		"hardfail.vhd": "entity hardfail is end entity;",
	}
}

func defaultFakeLintScript() string {
	return `#!/bin/sh
flag="$1"
target="$2"

if [ -d "$target" ]; then
  root="$target"
  out_file="$target/top.vhd"
else
  root=$(dirname "$target")
  out_file="$target"
fi

base=$(basename "$target")

	case "$base" in
	  *badjson.vhd)
	    echo "not-json"
	    exit 0
	    ;;
	  *hardfail.vhd)
	    echo "lint crashed" >&2
	    exit 2
	    ;;
	esac

violations='[{"rule":"unused_signal","severity":"warning","file":"'"$out_file"'","line":1,"message":"unused signal"}]'
parse_errors='[]'
missing_checks='[]'
ambiguous='[]'

if [ -d "$target" ]; then
  violations='[]'
else
	  case "$base" in
	    *clean.vhd)
	      violations='[]'
	      ;;
	    *parseerr.vhd)
	      violations='[]'
	      parse_errors='[{"file":"'"$out_file"'","message":"syntax error near token"}]'
	      ;;
	    *missing.vhd)
	      violations='[]'
	      missing_checks='[{"file":"'"$out_file"'","scope":"arch","missing_ids":["REQ_RESET"],"notes":["add reset assertion"],"bindings":{"clk":"clk_i"}}]'
	      ;;
	    *ambig.vhd)
	      violations='[]'
	      ambiguous='[{"kind":"priority","scope":"arch","file":"'"$out_file"'","line":2}]'
	      ;;
	    *multi.vhd)
	      violations='[{"rule":"unused_signal","severity":"warning","file":"'"$out_file"'","line":1,"message":"unused signal"},{"rule":"cdc_crossing","severity":"error","file":"'"$out_file"'","line":2,"message":"cdc issue"}]'
	      ;;
	  esac
fi

top="$root/top.vhd"
pkg="$root/pkg.vhd"

	symbol_json='{"entities":[{"name":"uart_tx","file":"'"$top"'","line":1,"ports":[{"name":"clk","direction":"in","type":"std_logic","file":"'"$top"'","line":2}]}],"architectures":[{"name":"rtl","entity_name":"uart_tx","file":"'"$top"'","line":5}],"packages":[{"name":"util_pkg","file":"'"$pkg"'","line":1}],"signals":[{"name":"state","type":"state_t","file":"'"$top"'","line":6,"in_entity":"uart_tx"}],"ports":[{"name":"clk","direction":"in","type":"std_logic","file":"'"$top"'","line":2,"in_entity":"uart_tx"}],"types":[{"name":"state_t","kind":"enum","file":"'"$pkg"'","line":2,"in_package":"util_pkg"}],"constants":[{"name":"WIDTH_C","type":"integer","file":"'"$pkg"'","line":3,"in_package":"util_pkg"}],"functions":[{"name":"to_slv","return_type":"std_logic_vector","file":"'"$pkg"'","line":4,"in_package":"util_pkg"}],"procedures":[{"name":"tick","file":"'"$pkg"'","line":5,"in_package":"util_pkg"}],"instances":[{"name":"u_child","target":"work.uart_tx","file":"'"$top"'","line":8,"in_arch":"rtl"}],"components":[{"name":"uart_tx","file":"'"$pkg"'","line":11}]}'

echo "{\"violations\":$violations,\"missing_checks\":$missing_checks,\"ambiguous_constructs\":$ambiguous,\"parse_errors\":$parse_errors,\"summary\":{\"total_violations\":1,\"errors\":0,\"warnings\":1,\"info\":0},\"symbol_index\":$symbol_json}"
`
}
