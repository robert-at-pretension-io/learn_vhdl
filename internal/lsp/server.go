package lsp

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspServer "github.com/tliron/glsp/server"
)

const serverName = "vhdl-lsp"
const commandShowMessage = "vhdl-lsp.showMessage"

var version = "0.1.0"

// Server holds the LSP server state.
type Server struct {
	handler       protocol.Handler
	server        *glspServer.Server
	workspaceRoot string
	runner        lintRunner
	debouncer     *Debouncer
	symbolStore   *SymbolStore
	documentStore *DocumentStore
	// notifyFunc is captured from the Initialize context for async notifications
	notifyFunc glsp.NotifyFunc
	callFunc   glsp.CallFunc
	logger     *log.Logger
	// previousURIs tracks which file URIs had diagnostics in the last lint run,
	// so we can clear stale diagnostics when files are fixed.
	previousURIs map[string]bool
	diagMu       sync.RWMutex
	diagCache    map[string][]protocol.Diagnostic
	lintMu       sync.Mutex
	lintCancel   context.CancelFunc
	lintJobID    uint64
}

type lintRunner interface {
	RunWithContext(ctx context.Context, workspaceRoot string, symbolsJSON bool) (*LintResult, error)
	RunTargetWithContext(ctx context.Context, targetPath, workingDir string, symbolsJSON bool) (*LintResult, error)
}

// NewServer creates a new LSP server instance.
func NewServer() *Server {
	s := &Server{
		debouncer:     NewDebouncer(),
		symbolStore:   NewSymbolStore(),
		documentStore: NewDocumentStore(),
		diagCache:     make(map[string][]protocol.Diagnostic),
		logger:        log.New(os.Stderr, "[vhdl-lsp] ", log.LstdFlags),
	}

	s.handler = protocol.Handler{
		CancelRequest:                  s.cancelRequest,
		Initialize:                     s.initialize,
		Initialized:                    s.initialized,
		Shutdown:                       s.shutdown,
		SetTrace:                       s.setTrace,
		WindowWorkDoneProgressCancel:   s.windowWorkDoneProgressCancel,
		TextDocumentDidOpen:            s.textDocumentDidOpen,
		TextDocumentDidSave:            s.textDocumentDidSave,
		TextDocumentDidClose:           s.textDocumentDidClose,
		TextDocumentDidChange:          s.textDocumentDidChange,
		TextDocumentCompletion:         s.textDocumentCompletion,
		TextDocumentDefinition:         s.textDocumentDefinition,
		TextDocumentTypeDefinition:     s.textDocumentTypeDefinition,
		TextDocumentImplementation:     s.textDocumentImplementation,
		TextDocumentReferences:         s.textDocumentReferences,
		TextDocumentHover:              s.textDocumentHover,
		TextDocumentDocumentSymbol:     s.textDocumentDocumentSymbol,
		TextDocumentRename:             s.textDocumentRename,
		TextDocumentPrepareRename:      s.textDocumentPrepareRename,
		TextDocumentCodeAction:         s.textDocumentCodeAction,
		TextDocumentCodeLens:           s.textDocumentCodeLens,
		TextDocumentSemanticTokensFull: s.textDocumentSemanticTokensFull,
		WorkspaceSymbol:                s.workspaceSymbol,
		WorkspaceExecuteCommand:        s.workspaceExecuteCommand,
	}

	s.server = glspServer.NewServer(&s.handler, serverName, debugEnabled())
	return s
}

// RunStdio starts the LSP server on stdin/stdout.
func (s *Server) RunStdio() error {
	return s.server.RunStdio()
}

func (s *Server) initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	s.notifyFunc = ctx.Notify
	s.callFunc = ctx.Call

	s.workspaceRoot = resolveWorkspaceRoot(params)

	// Initialize the lint runner
	runner, err := NewRunner()
	if err != nil {
		s.logger.Printf("WARNING: %v (diagnostics disabled until vhdl-lint is available)", err)
	} else {
		s.runner = runner
	}

	capabilities := s.handler.CreateServerCapabilities()

	// Override text document sync to be explicit
	change := protocol.TextDocumentSyncKindFull
	openClose := true
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: &openClose,
		Change:    &change,
		Save:      &protocol.SaveOptions{IncludeText: boolPtr(false)},
	}
	capabilities.CompletionProvider = &protocol.CompletionOptions{
		TriggerCharacters: []string{".", ":"},
	}
	capabilities.DocumentSymbolProvider = &protocol.DocumentSymbolOptions{}
	capabilities.SemanticTokensProvider = &protocol.SemanticTokensOptions{
		Legend: semanticTokensLegend(),
		Full:   true,
	}
	capabilities.TypeDefinitionProvider = true
	capabilities.ImplementationProvider = true
	capabilities.RenameProvider = &protocol.RenameOptions{
		PrepareProvider: boolPtr(true),
	}
	capabilities.CodeLensProvider = &protocol.CodeLensOptions{
		ResolveProvider: boolPtr(false),
	}
	capabilities.CodeActionProvider = &protocol.CodeActionOptions{
		CodeActionKinds: []protocol.CodeActionKind{
			protocol.CodeActionKindQuickFix,
		},
	}
	capabilities.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{
		Commands: []string{commandShowMessage},
	}
	capabilities.Workspace = &protocol.ServerCapabilitiesWorkspace{
		WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
			Supported:           boolPtr(true),
			ChangeNotifications: &protocol.BoolOrString{Value: true},
		},
	}

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: &version,
		},
	}, nil
}

func resolveWorkspaceRoot(params *protocol.InitializeParams) string {
	if params == nil {
		return ""
	}
	if params.RootURI != nil && *params.RootURI != "" {
		return uriToFile(*params.RootURI)
	}
	if params.RootPath != nil && *params.RootPath != "" {
		return *params.RootPath
	}
	for _, folder := range params.WorkspaceFolders {
		if folder.URI != "" {
			return uriToFile(folder.URI)
		}
	}
	return ""
}

func (s *Server) initialized(_ *glsp.Context, _ *protocol.InitializedParams) error {
	s.logger.Printf("initialized: workspace=%s", s.workspaceRoot)
	// Trigger initial lint
	s.triggerLintFull()
	return nil
}

func (s *Server) shutdown(_ *glsp.Context) error {
	s.debouncer.Stop()
	s.cancelActiveLint()
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func (s *Server) setTrace(_ *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func (s *Server) textDocumentDidOpen(_ *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.documentStore.Set(params.TextDocument.URI, params.TextDocument.Text)
	s.triggerLintIncremental(params.TextDocument.URI)
	return nil
}

func (s *Server) textDocumentDidChange(_ *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	// With full sync, the last content change has the full text
	updated := false
	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			s.documentStore.Set(params.TextDocument.URI, c.Text)
			updated = true
		}
	}
	if updated {
		s.triggerLintIncremental(params.TextDocument.URI)
	}
	return nil
}

func (s *Server) textDocumentDidSave(_ *glsp.Context, _ *protocol.DidSaveTextDocumentParams) error {
	s.triggerLintFull()
	return nil
}

func (s *Server) textDocumentDidClose(_ *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.documentStore.Delete(params.TextDocument.URI)
	s.triggerLintFull()
	return nil
}

// triggerLintFull schedules a debounced full-workspace lint run.
func (s *Server) triggerLintFull() {
	if s.runner == nil || s.workspaceRoot == "" {
		return
	}
	s.debouncer.Trigger(func() {
		s.startLintJob(func(ctx context.Context, jobID uint64) {
			s.runLintFull(ctx, jobID)
		})
	})
}

// triggerLintIncremental schedules a debounced single-file lint run.
func (s *Server) triggerLintIncremental(uri string) {
	if s.runner == nil || s.workspaceRoot == "" {
		return
	}
	if uri == "" {
		return
	}
	s.debouncer.Trigger(func() {
		s.startLintJob(func(ctx context.Context, jobID uint64) {
			s.runLintIncremental(ctx, jobID, uri)
		})
	})
}

func (s *Server) startLintJob(run func(ctx context.Context, jobID uint64)) {
	ctx, jobID := s.beginLintJob()
	token := lintProgressToken(jobID)
	s.progressBegin(token, "Linting VHDL project")
	go func() {
		defer s.finishLintJob(jobID)
		run(ctx, jobID)
		if errors.Is(ctx.Err(), context.Canceled) {
			s.progressEnd(token, "Lint canceled")
			return
		}
		s.progressEnd(token, "Lint complete")
	}()
}

func (s *Server) beginLintJob() (context.Context, uint64) {
	s.lintMu.Lock()
	defer s.lintMu.Unlock()
	if s.lintCancel != nil {
		s.lintCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.lintCancel = cancel
	s.lintJobID++
	return ctx, s.lintJobID
}

func (s *Server) finishLintJob(jobID uint64) {
	s.lintMu.Lock()
	defer s.lintMu.Unlock()
	if s.lintJobID == jobID {
		s.lintCancel = nil
	}
}

func (s *Server) cancelActiveLint() {
	s.lintMu.Lock()
	defer s.lintMu.Unlock()
	if s.lintCancel != nil {
		s.lintCancel()
		s.lintCancel = nil
	}
}

func (s *Server) lintJobActive(jobID uint64) bool {
	s.lintMu.Lock()
	defer s.lintMu.Unlock()
	return s.lintJobID == jobID
}

// runLintFull executes full-workspace lint and publishes diagnostics.
func (s *Server) runLintFull(ctx context.Context, jobID uint64) {
	if s.runner == nil || s.notifyFunc == nil {
		return
	}

	s.progressReport(lintProgressToken(jobID), "Running full lint", uintPtr(20))
	result, err := s.runner.RunWithContext(ctx, s.workspaceRoot, true)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		s.logger.Printf("lint error: %v", err)
		s.logMessage(protocol.MessageTypeError, "vhdl-lint failed: "+err.Error())
		return
	}
	if ctx.Err() != nil || !s.lintJobActive(jobID) {
		return
	}

	// Rebuild symbol index if available
	if result.SymbolIndex != nil {
		s.symbolStore.Rebuild(result.SymbolIndex, s.workspaceRoot)
	}

	// Convert full result (violations + parse errors + missing checks + ambiguous constructs)
	s.progressReport(lintProgressToken(jobID), "Building diagnostics", uintPtr(80))
	diagsByFile := mapResultToDiagnostics(result, s.workspaceRoot)

	// Publish diagnostics for files with results
	currentURIs := make(map[string]bool, len(diagsByFile))
	for uri, diags := range diagsByFile {
		if ctx.Err() != nil || !s.lintJobActive(jobID) {
			return
		}
		currentURIs[uri] = true
		s.publishDiagnostics(uri, diags)
	}

	// Clear diagnostics for files that had violations in the previous run but not this one
	if s.previousURIs != nil {
		for uri := range s.previousURIs {
			if ctx.Err() != nil || !s.lintJobActive(jobID) {
				return
			}
			if !currentURIs[uri] {
				s.publishDiagnostics(uri, []protocol.Diagnostic{})
			}
		}
	}
	s.previousURIs = currentURIs
	s.progressReport(lintProgressToken(jobID), "Publishing diagnostics", uintPtr(100))
}

// runLintIncremental executes file-targeted lint and publishes diagnostics for touched files.
func (s *Server) runLintIncremental(ctx context.Context, jobID uint64, uri string) {
	if s.runner == nil || s.notifyFunc == nil {
		return
	}
	targetPath := uriToFile(uri)
	if targetPath == "" {
		return
	}
	lintPath := targetPath
	cleanup := func() {}
	if text, ok := s.documentStore.Get(uri); ok {
		if overlayPath, removeOverlay, err := createOverlayFile(targetPath, text); err == nil {
			lintPath = overlayPath
			cleanup = removeOverlay
		} else {
			s.logger.Printf("overlay creation failed, falling back to on-disk file: %v", err)
		}
	}
	defer cleanup()

	s.progressReport(lintProgressToken(jobID), "Running incremental lint", uintPtr(25))
	result, err := s.runner.RunTargetWithContext(ctx, lintPath, s.workspaceRoot, false)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		s.logger.Printf("incremental lint error: %v", err)
		s.logMessage(protocol.MessageTypeWarning, "vhdl-lint incremental failed: "+err.Error())
		return
	}
	if ctx.Err() != nil || !s.lintJobActive(jobID) {
		return
	}
	if lintPath != targetPath {
		remapLintResultFile(result, lintPath, targetPath)
	}

	diagsByFile := mapResultToDiagnostics(result, s.workspaceRoot)
	s.progressReport(lintProgressToken(jobID), "Publishing incremental diagnostics", uintPtr(100))
	publishedTarget := false
	for outURI, diags := range diagsByFile {
		if ctx.Err() != nil || !s.lintJobActive(jobID) {
			return
		}
		s.publishDiagnostics(outURI, diags)
		if samePath(uriToFile(outURI), targetPath) {
			publishedTarget = true
		}
	}

	// If the targeted file has no diagnostics in this incremental run, clear it.
	if !publishedTarget {
		if ctx.Err() != nil || !s.lintJobActive(jobID) {
			return
		}
		s.publishDiagnostics(uri, []protocol.Diagnostic{})
	}
}

// logMessage sends a window/logMessage notification to the client.
func (s *Server) logMessage(typ protocol.MessageType, msg string) {
	if s.notifyFunc != nil {
		s.notifyFunc("window/logMessage", protocol.LogMessageParams{
			Type:    typ,
			Message: msg,
		})
	}
}

func debugEnabled() bool {
	level := strings.ToLower(os.Getenv("VHDL_LSP_LOG_LEVEL"))
	return level == "debug"
}

func boolPtr(b bool) *bool {
	return &b
}

func uintPtr(v uint) *protocol.UInteger {
	u := protocol.UInteger(v)
	return &u
}

func (s *Server) publishDiagnostics(uri protocol.DocumentUri, diags []protocol.Diagnostic) {
	if s.notifyFunc == nil {
		return
	}
	s.setCachedDiagnostics(uri, diags)
	s.notifyFunc(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})
}

func (s *Server) setCachedDiagnostics(uri protocol.DocumentUri, diags []protocol.Diagnostic) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	out := make([]protocol.Diagnostic, len(diags))
	copy(out, diags)
	s.diagCache[uri] = out
}

func (s *Server) cachedDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	s.diagMu.RLock()
	defer s.diagMu.RUnlock()
	diags := s.diagCache[uri]
	out := make([]protocol.Diagnostic, len(diags))
	copy(out, diags)
	return out
}

func (s *Server) windowWorkDoneProgressCancel(_ *glsp.Context, _ *protocol.WorkDoneProgressCancelParams) error {
	s.cancelActiveLint()
	return nil
}

// cancelRequest handles base-protocol cancellation notifications ($/cancelRequest).
// We currently run at most one active lint job, so a cancellation request is treated
// as a hint to stop in-flight work if any.
func (s *Server) cancelRequest(_ *glsp.Context, _ *protocol.CancelParams) error {
	s.cancelActiveLint()
	return nil
}

func lintProgressToken(jobID uint64) protocol.ProgressToken {
	return protocol.IntegerOrString{Value: "vhdl-lsp/lint/" + strconv.FormatUint(jobID, 10)}
}

func (s *Server) progressBegin(token protocol.ProgressToken, title string) {
	if s.callFunc != nil {
		var out any
		s.callFunc(string(protocol.ServerWindowWorkDoneProgressCreate), protocol.WorkDoneProgressCreateParams{
			Token: token,
		}, &out)
	}
	cancellable := true
	s.progressNotify(token, protocol.WorkDoneProgressBegin{
		Kind:        "begin",
		Title:       title,
		Cancellable: &cancellable,
	})
}

func (s *Server) progressReport(token protocol.ProgressToken, message string, percentage *protocol.UInteger) {
	msg := message
	s.progressNotify(token, protocol.WorkDoneProgressReport{
		Kind:       "report",
		Message:    &msg,
		Percentage: percentage,
	})
}

func (s *Server) progressEnd(token protocol.ProgressToken, message string) {
	msg := message
	s.progressNotify(token, protocol.WorkDoneProgressEnd{
		Kind:    "end",
		Message: &msg,
	})
}

func (s *Server) progressNotify(token protocol.ProgressToken, value any) {
	if s.notifyFunc == nil {
		return
	}
	s.notifyFunc(string(protocol.MethodProgress), protocol.ProgressParams{
		Token: token,
		Value: value,
	})
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func createOverlayFile(targetPath, content string) (string, func(), error) {
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)
	pattern := ".vhdl_lsp_overlay_*_" + base
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", func() {}, err
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}
	return path, cleanup, nil
}

func remapLintResultFile(result *LintResult, fromPath, toPath string) {
	if result == nil {
		return
	}
	if samePath(fromPath, toPath) {
		return
	}
	for i := range result.Violations {
		if samePath(result.Violations[i].File, fromPath) {
			result.Violations[i].File = toPath
		}
	}
	for i := range result.ParseErrors {
		if samePath(result.ParseErrors[i].File, fromPath) {
			result.ParseErrors[i].File = toPath
		}
	}
	for i := range result.MissingChecks {
		if samePath(result.MissingChecks[i].File, fromPath) {
			result.MissingChecks[i].File = toPath
		}
	}
	for i := range result.AmbiguousConstructs {
		if samePath(result.AmbiguousConstructs[i].File, fromPath) {
			result.AmbiguousConstructs[i].File = toPath
		}
	}
	for i := range result.Waivers {
		if samePath(result.Waivers[i].File, fromPath) {
			result.Waivers[i].File = toPath
		}
	}
}
