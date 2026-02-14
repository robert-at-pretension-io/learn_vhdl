package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type mockLintRunner struct {
	mu             sync.Mutex
	result         *LintResult
	err            error
	runCalls       int
	runTargetCalls int
	lastWorkspace  string
	lastTarget     string
	lastWorkingDir string
	runHook        func(ctx context.Context, workspaceRoot string) (*LintResult, error)
	runTargetHook  func(ctx context.Context, targetPath, workingDir string) (*LintResult, error)
}

func (m *mockLintRunner) RunWithContext(ctx context.Context, workspaceRoot string, _ bool) (*LintResult, error) {
	m.mu.Lock()
	m.runCalls++
	m.lastWorkspace = workspaceRoot
	hook := m.runHook
	result := m.result
	err := m.err
	m.mu.Unlock()
	if hook != nil {
		return hook(ctx, workspaceRoot)
	}
	if result == nil {
		return &LintResult{}, err
	}
	return result, err
}

func (m *mockLintRunner) RunTargetWithContext(ctx context.Context, targetPath, workingDir string, _ bool) (*LintResult, error) {
	m.mu.Lock()
	m.runTargetCalls++
	m.lastTarget = targetPath
	m.lastWorkingDir = workingDir
	hook := m.runTargetHook
	result := m.result
	err := m.err
	m.mu.Unlock()
	if hook != nil {
		return hook(ctx, targetPath, workingDir)
	}
	if result == nil {
		return &LintResult{}, err
	}
	return result, err
}

func TestResolveWorkspaceRoot_PrefersRootURI(t *testing.T) {
	rootURI := protocol.DocumentUri("file:///workspace/from-root-uri")
	rootPath := "/workspace/from-root-path"
	params := &protocol.InitializeParams{
		RootURI:  &rootURI,
		RootPath: &rootPath,
		WorkspaceFolders: []protocol.WorkspaceFolder{
			{URI: "file:///workspace/from-folder", Name: "project"},
		},
	}

	got := resolveWorkspaceRoot(params)
	if got != "/workspace/from-root-uri" {
		t.Fatalf("expected root URI path, got %q", got)
	}
}

func TestResolveWorkspaceRoot_UsesWorkspaceFoldersWhenNoRootURIOrRootPath(t *testing.T) {
	params := &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{
			{URI: "file:///workspace/from-folder", Name: "project"},
		},
	}

	got := resolveWorkspaceRoot(params)
	if got != "/workspace/from-folder" {
		t.Fatalf("expected workspace folder path, got %q", got)
	}
}

func TestResolveWorkspaceRoot_EmptyWhenNoInputs(t *testing.T) {
	got := resolveWorkspaceRoot(&protocol.InitializeParams{})
	if got != "" {
		t.Fatalf("expected empty root, got %q", got)
	}
}

func TestNewServerRegistersCancelRequestHandler(t *testing.T) {
	s := NewServer()
	if s.handler.CancelRequest == nil {
		t.Fatal("expected CancelRequest handler to be registered")
	}
}

func TestTextDocumentDidOpenStoresDocumentAndTriggersLint(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	s.runner = &Runner{binaryPath: "/bin/true"}
	s.debouncer = &Debouncer{delay: time.Hour}

	uri := protocol.DocumentUri("file:///workspace/test.vhd")
	err := s.textDocumentDidOpen(nil, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  uri,
			Text: "entity test is end entity;",
		},
	})
	if err != nil {
		t.Fatalf("didOpen returned error: %v", err)
	}
	if got, ok := s.documentStore.Get(uri); !ok || got == "" {
		t.Fatalf("expected document to be stored, got ok=%v content=%q", ok, got)
	}

	s.debouncer.mu.Lock()
	hasTimer := s.debouncer.timer != nil
	s.debouncer.mu.Unlock()
	if !hasTimer {
		t.Fatal("expected lint trigger timer after didOpen")
	}
	s.debouncer.Stop()
}

func TestTextDocumentDidChangeUpdatesDocumentAndTriggersLint(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	s.runner = &Runner{binaryPath: "/bin/true"}
	s.debouncer = &Debouncer{delay: time.Hour}

	uri := protocol.DocumentUri("file:///workspace/test.vhd")
	s.documentStore.Set(uri, "old")
	err := s.textDocumentDidChange(nil, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: 2},
		ContentChanges: []any{
			protocol.TextDocumentContentChangeEventWhole{Text: "new content"},
		},
	})
	if err != nil {
		t.Fatalf("didChange returned error: %v", err)
	}
	if got, ok := s.documentStore.Get(uri); !ok || got != "new content" {
		t.Fatalf("expected updated content, got ok=%v content=%q", ok, got)
	}

	s.debouncer.mu.Lock()
	hasTimer := s.debouncer.timer != nil
	s.debouncer.mu.Unlock()
	if !hasTimer {
		t.Fatal("expected lint trigger timer after didChange")
	}
	s.debouncer.Stop()
}

func TestTextDocumentDidCloseDoesNotClearDiagnosticsAndTriggersLint(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	s.runner = &Runner{binaryPath: "/bin/true"}
	s.debouncer = &Debouncer{delay: time.Hour}

	uri := protocol.DocumentUri("file:///workspace/test.vhd")
	s.documentStore.Set(uri, "entity test is end entity;")
	notified := false
	s.notifyFunc = func(method string, params any) {
		if method == protocol.ServerTextDocumentPublishDiagnostics {
			notified = true
		}
	}

	err := s.textDocumentDidClose(nil, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	if err != nil {
		t.Fatalf("didClose returned error: %v", err)
	}
	if _, ok := s.documentStore.Get(uri); ok {
		t.Fatal("expected document to be removed from store")
	}
	if notified {
		t.Fatal("didClose should not publish empty diagnostics directly")
	}

	s.debouncer.mu.Lock()
	hasTimer := s.debouncer.timer != nil
	s.debouncer.mu.Unlock()
	if !hasTimer {
		t.Fatal("expected lint trigger timer after didClose")
	}
	s.debouncer.Stop()
}

func TestDidChangeUsesIncrementalRunner(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	mock := &mockLintRunner{result: &LintResult{}}
	s.runner = mock
	s.debouncer = &Debouncer{delay: 10 * time.Millisecond}
	s.notifyFunc = func(_ string, _ any) {}

	target := filepath.Join(s.workspaceRoot, "live.vhd")
	uri := protocol.DocumentUri("file://" + filepath.ToSlash(target))
	err := s.textDocumentDidChange(nil, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                1,
		},
		ContentChanges: []any{protocol.TextDocumentContentChangeEventWhole{Text: "entity live is end;"}},
	})
	if err != nil {
		t.Fatalf("didChange returned error: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	s.debouncer.Stop()

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.runCalls != 0 {
		t.Fatalf("didChange should not trigger full lint, got %d full runs", mock.runCalls)
	}
	if mock.runTargetCalls != 1 {
		t.Fatalf("didChange should trigger one incremental run, got %d", mock.runTargetCalls)
	}
	if mock.lastTarget == target {
		t.Fatalf("expected overlay target for live lint, got original path %q", mock.lastTarget)
	}
	if !strings.Contains(filepath.Base(mock.lastTarget), "vhdl_lsp_overlay") {
		t.Fatalf("expected overlay target name, got %q", mock.lastTarget)
	}
	if mock.lastWorkingDir != s.workspaceRoot {
		t.Fatalf("unexpected incremental working dir: %q", mock.lastWorkingDir)
	}
}

func TestDidSaveUsesFullRunner(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = "/workspace"
	mock := &mockLintRunner{result: &LintResult{}}
	s.runner = mock
	s.debouncer = &Debouncer{delay: 10 * time.Millisecond}
	s.notifyFunc = func(_ string, _ any) {}

	err := s.textDocumentDidSave(nil, &protocol.DidSaveTextDocumentParams{})
	if err != nil {
		t.Fatalf("didSave returned error: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	s.debouncer.Stop()

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.runCalls != 1 {
		t.Fatalf("didSave should trigger full lint, got %d", mock.runCalls)
	}
	if mock.runTargetCalls != 0 {
		t.Fatalf("didSave should not trigger incremental lint, got %d", mock.runTargetCalls)
	}
	if mock.lastWorkspace != "/workspace" {
		t.Fatalf("unexpected workspace root: %q", mock.lastWorkspace)
	}
}

func TestRunLintIncremental_OverlayAndRemapToOriginalURI(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "live.vhd")
	if err := os.WriteFile(target, []byte("entity old is end entity;"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	uri := "file://" + filepath.ToSlash(target)

	s := NewServer()
	s.workspaceRoot = tmp
	s.documentStore.Set(uri, "entity new is end entity;")

	var gotDiagURI string
	s.notifyFunc = func(method string, params any) {
		if method != protocol.ServerTextDocumentPublishDiagnostics {
			return
		}
		p := params.(protocol.PublishDiagnosticsParams)
		gotDiagURI = p.URI
	}

	mock := &mockLintRunner{}
	mock.runTargetHook = func(_ context.Context, targetPath, _ string) (*LintResult, error) {
		return &LintResult{
			Violations: []Violation{
				{Rule: "test_rule", Severity: "warning", File: targetPath, Line: 1, Message: "live"},
			},
		}, nil
	}
	s.runner = mock

	ctx, jobID := s.beginLintJob()
	defer s.finishLintJob(jobID)
	s.runLintIncremental(ctx, jobID, uri)

	mock.mu.Lock()
	lastTarget := mock.lastTarget
	mock.mu.Unlock()
	if lastTarget == target {
		t.Fatalf("expected incremental lint to use overlay file, got target %q", lastTarget)
	}
	if !strings.Contains(filepath.Base(lastTarget), "vhdl_lsp_overlay") {
		t.Fatalf("expected overlay filename, got %q", lastTarget)
	}
	if gotDiagURI != uri {
		t.Fatalf("expected diagnostics URI %q, got %q", uri, gotDiagURI)
	}
}

func TestNewLintCancelsInFlightLint(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	mock := &mockLintRunner{result: &LintResult{}}
	s.runner = mock
	s.debouncer = &Debouncer{delay: 10 * time.Millisecond}
	s.notifyFunc = func(_ string, _ any) {}

	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	mock.runHook = func(ctx context.Context, _ string) (*LintResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		canceled <- struct{}{}
		return nil, ctx.Err()
	}

	if err := s.textDocumentDidSave(nil, &protocol.DidSaveTextDocumentParams{}); err != nil {
		t.Fatalf("didSave returned error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("full lint did not start")
	}

	target := filepath.Join(s.workspaceRoot, "live.vhd")
	uri := protocol.DocumentUri("file://" + filepath.ToSlash(target))
	if err := s.textDocumentDidChange(nil, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []any{protocol.TextDocumentContentChangeEventWhole{Text: "entity live is end;"}},
	}); err != nil {
		t.Fatalf("didChange returned error: %v", err)
	}

	select {
	case <-canceled:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected in-flight full lint to be canceled by new incremental lint")
	}

	time.Sleep(40 * time.Millisecond)
	s.debouncer.Stop()
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if mock.runTargetCalls == 0 {
		t.Fatal("expected incremental lint after cancellation")
	}
}

func TestCancelRequestCancelsActiveLint(t *testing.T) {
	s := NewServer()
	ctx, jobID := s.beginLintJob()
	defer s.finishLintJob(jobID)

	if err := s.cancelRequest(nil, &protocol.CancelParams{}); err != nil {
		t.Fatalf("cancelRequest returned error: %v", err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected cancelRequest to cancel active lint context")
	}

	if ctx.Err() != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", ctx.Err())
	}
}
