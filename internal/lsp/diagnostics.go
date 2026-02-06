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

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(line),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  v.Message,
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

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(0),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  pe.Message,
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

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(0),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  msg,
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

		diags[uri] = append(diags[uri], protocol.Diagnostic{
			Range:    lineRange(line),
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  msg,
		})
	}

	return diags
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
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, path)
	}
	path = filepath.Clean(path)
	u := &url.URL{Scheme: "file", Path: path}
	return u.String()
}

// uriToFile converts a file:// URI to a local file path.
func uriToFile(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		path := strings.TrimPrefix(uri, "file://")
		if decoded, err := url.PathUnescape(path); err == nil {
			return decoded
		}
		return path
	}
	return uri
}
