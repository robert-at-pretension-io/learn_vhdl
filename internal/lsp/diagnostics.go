package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// mapResultToDiagnostics converts the full LintResult into LSP diagnostics
// grouped by file URI. This includes violations, parse errors, missing checks,
// and ambiguous constructs — ensuring parity with CLI output.
func mapResultToDiagnostics(result *LintResult, workspaceRoot string) map[string][]protocol.Diagnostic {
	diags := make(map[string][]protocol.Diagnostic)

	// 1. Policy violations (the main output)
	for _, v := range result.Violations {
		uri := fileToURI(v.File, workspaceRoot)
		severity := mapSeverity(v.Severity)
		source := "vhdl-lint"
		code := &protocol.IntegerOrString{Value: v.Rule}
		line := lineToLSP0(v.Line)
		data := map[string]any{
			"kind": "violation",
			"rule": v.Rule,
			"line": v.Line,
			"file": v.File,
		}

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(line),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  v.Message,
			Data:     data,
		})
	}

	// 2. Parse errors — surface as error diagnostics so broken files aren't silent
	for _, pe := range result.ParseErrors {
		file := pe.File
		if file == "" {
			continue
		}
		uri := fileToURI(file, workspaceRoot)
		severity := protocol.DiagnosticSeverityError
		source := "vhdl-lint"
		code := &protocol.IntegerOrString{Value: "parse_error"}
		data := map[string]any{
			"kind": "parse_error",
			"file": pe.File,
		}

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(0),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  pe.Message,
			Data:     data,
		})
	}

	// 3. Missing verification checks — surface as info diagnostics
	for _, mc := range result.MissingChecks {
		if mc.File == "" {
			continue
		}
		uri := fileToURI(mc.File, workspaceRoot)
		severity := protocol.DiagnosticSeverityInformation
		source := "vhdl-lint"
		code := &protocol.IntegerOrString{Value: "missing_check"}
		msg := fmt.Sprintf("missing verification checks: %s", strings.Join(mc.MissingIDs, ", "))
		data := map[string]any{
			"kind":        "missing_check",
			"file":        mc.File,
			"scope":       mc.Scope,
			"missing_ids": mc.MissingIDs,
		}
		related := makeRelatedInformationForMissingCheck(mc, workspaceRoot)

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:              lineRange(0),
			Severity:           &severity,
			Code:               code,
			Source:             &source,
			Message:            msg,
			Data:               data,
			RelatedInformation: related,
		})
	}

	// 4. Ambiguous constructs — surface as hint diagnostics
	for _, ac := range result.AmbiguousConstructs {
		if ac.File == "" {
			continue
		}
		uri := fileToURI(ac.File, workspaceRoot)
		severity := protocol.DiagnosticSeverityHint
		source := "vhdl-lint"
		code := &protocol.IntegerOrString{Value: "ambiguous_construct"}
		line := lineToLSP0(ac.Line)
		msg := fmt.Sprintf("ambiguous %s construct", ac.Kind)
		data := map[string]any{
			"kind":  "ambiguous_construct",
			"file":  ac.File,
			"scope": ac.Scope,
		}
		related := []protocol.DiagnosticRelatedInformation{}
		if ac.Scope != "" {
			related = append(related, protocol.DiagnosticRelatedInformation{
				Location: protocol.Location{
					URI:   uri,
					Range: lineRange(line),
				},
				Message: "scope: " + ac.Scope,
			})
		}

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:              lineRange(line),
			Severity:           &severity,
			Code:               code,
			Source:             &source,
			Message:            msg,
			Data:               data,
			RelatedInformation: related,
		})
	}

	return diags
}

func makeRelatedInformationForMissingCheck(mc MissingCheckTask, workspaceRoot string) []protocol.DiagnosticRelatedInformation {
	uri := fileToURI(mc.File, workspaceRoot)
	var related []protocol.DiagnosticRelatedInformation
	for _, note := range mc.Notes {
		if strings.TrimSpace(note) == "" {
			continue
		}
		related = append(related, protocol.DiagnosticRelatedInformation{
			Location: protocol.Location{
				URI:   uri,
				Range: lineRange(0),
			},
			Message: note,
		})
	}
	for k, v := range mc.Bindings {
		related = append(related, protocol.DiagnosticRelatedInformation{
			Location: protocol.Location{
				URI:   uri,
				Range: lineRange(0),
			},
			Message: fmt.Sprintf("binding %s -> %s", k, v),
		})
	}
	return related
}

func lineRange(line protocol.UInteger) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: 0},
		End:   protocol.Position{Line: line, Character: 0},
	}
}

func lineToLSP0(line int) protocol.UInteger {
	if line > 0 {
		return protocol.UInteger(line - 1)
	}
	return 0
}

// mapSeverity converts vhdl-lint severity strings to LSP severity values.
func mapSeverity(severity string) protocol.DiagnosticSeverity {
	switch strings.ToLower(severity) {
	case "error":
		return protocol.DiagnosticSeverityError
	case "warning":
		return protocol.DiagnosticSeverityWarning
	case "info":
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// fileToURI converts a file path to a file:// URI.
// If the path is relative, it's resolved against the workspace root.
func fileToURI(path, workspaceRoot string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		if workspaceRoot != "" {
			path = filepath.Join(workspaceRoot, path)
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	path = filepath.Clean(path)
	path = filepath.ToSlash(path)
	if looksLikeWindowsDrivePath(path) {
		path = "/" + strings.TrimPrefix(path, "/")
	}
	u := &url.URL{Scheme: "file", Path: path}
	return u.String()
}

// uriToFile converts a file:// URI to a local file path.
func uriToFile(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		path := strings.TrimPrefix(uri, "file://")
		if decoded, err := url.PathUnescape(path); err == nil {
			return filepath.Clean(filepath.FromSlash(decoded))
		}
		return filepath.Clean(filepath.FromSlash(path))
	}

	path := parsed.Path
	if parsed.Host != "" {
		path = "//" + parsed.Host + path
	}
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if looksLikeWindowsDrivePath(path) {
		path = strings.TrimPrefix(path, "/")
	}
	return filepath.Clean(filepath.FromSlash(path))
}

func looksLikeWindowsDrivePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	if path[0] == '/' {
		path = path[1:]
	}
	if len(path) < 2 || path[1] != ':' {
		return false
	}
	drive := path[0]
	return (drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')
}
