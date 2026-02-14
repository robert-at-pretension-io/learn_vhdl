package lsp

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestLintProgressBeginReportEndNotifications(t *testing.T) {
	s := NewServer()
	s.workspaceRoot = t.TempDir()
	s.debouncer = &Debouncer{delay: 10 * time.Millisecond}
	s.runner = &mockLintRunner{result: &LintResult{}}

	createCalled := false
	var mu sync.Mutex
	s.callFunc = func(method string, _ any, _ any) {
		if method == string(protocol.ServerWindowWorkDoneProgressCreate) {
			mu.Lock()
			createCalled = true
			mu.Unlock()
		}
	}

	beginSeen := false
	reportSeen := false
	endSeen := false
	s.notifyFunc = func(method string, params any) {
		if method != string(protocol.MethodProgress) {
			return
		}
		p, ok := params.(protocol.ProgressParams)
		if !ok {
			return
		}
		switch v := p.Value.(type) {
		case protocol.WorkDoneProgressBegin:
			if v.Kind == "begin" {
				mu.Lock()
				beginSeen = true
				mu.Unlock()
			}
		case protocol.WorkDoneProgressReport:
			if v.Kind == "report" {
				mu.Lock()
				reportSeen = true
				mu.Unlock()
			}
		case protocol.WorkDoneProgressEnd:
			if v.Kind == "end" {
				mu.Lock()
				endSeen = true
				mu.Unlock()
			}
		}
	}

	if err := s.textDocumentDidSave(nil, &protocol.DidSaveTextDocumentParams{}); err != nil {
		t.Fatalf("didSave returned error: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	s.debouncer.Stop()

	mu.Lock()
	defer mu.Unlock()
	if !createCalled {
		t.Fatal("expected workDoneProgress/create call")
	}
	if !beginSeen || !reportSeen || !endSeen {
		t.Fatalf("expected begin/report/end progress events, got begin=%v report=%v end=%v", beginSeen, reportSeen, endSeen)
	}
}

func TestIncrementalDiagnosticsCachePersistsAcrossFiles(t *testing.T) {
	tmp := t.TempDir()
	uriA := "file://" + filepath.ToSlash(filepath.Join(tmp, "a.vhd"))
	uriB := "file://" + filepath.ToSlash(filepath.Join(tmp, "b.vhd"))

	s := NewServer()
	s.workspaceRoot = tmp
	s.notifyFunc = func(_ string, _ any) {}
	s.documentStore.Set(uriA, "signal a : std_logic;")
	s.documentStore.Set(uriB, "signal b : std_logic;")

	mock := &mockLintRunner{}
	mock.runTargetHook = func(_ context.Context, targetPath, _ string) (*LintResult, error) {
		return &LintResult{
			Violations: []Violation{
				{Rule: "test_rule", Severity: "warning", File: targetPath, Line: 1, Message: "x"},
			},
		}, nil
	}
	s.runner = mock

	ctx1, id1 := s.beginLintJob()
	s.runLintIncremental(ctx1, id1, uriA)
	s.finishLintJob(id1)

	if len(s.cachedDiagnostics(uriA)) == 0 {
		t.Fatal("expected cached diagnostics for uriA after first incremental run")
	}

	ctx2, id2 := s.beginLintJob()
	s.runLintIncremental(ctx2, id2, uriB)
	s.finishLintJob(id2)

	if len(s.cachedDiagnostics(uriB)) == 0 {
		t.Fatal("expected cached diagnostics for uriB after second incremental run")
	}
	if len(s.cachedDiagnostics(uriA)) == 0 {
		t.Fatal("expected uriA diagnostics to remain cached after uriB incremental run")
	}
}
