package lsp

import (
	"log"
	"os"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	glspServer "github.com/tliron/glsp/server"
)

const serverName = "vhdl-lsp"

var version = "0.1.0"

// Server holds the LSP server state.
type Server struct {
	handler       protocol.Handler
	server        *glspServer.Server
	workspaceRoot string
	runner        *Runner
	debouncer     *Debouncer
	symbolStore   *SymbolStore
	documentStore *DocumentStore
	// notifyFunc is captured from the Initialize context for async notifications
	notifyFunc glsp.NotifyFunc
	logger     *log.Logger
	// previousURIs tracks which file URIs had diagnostics in the last lint run,
	// so we can clear stale diagnostics when files are fixed.
	previousURIs map[string]bool
}

// NewServer creates a new LSP server instance.
func NewServer() *Server {
	s := &Server{
		debouncer:     NewDebouncer(),
		symbolStore:   NewSymbolStore(),
		documentStore: NewDocumentStore(),
		logger:        log.New(os.Stderr, "[vhdl-lsp] ", log.LstdFlags),
	}

	s.handler = protocol.Handler{
		Initialize:             s.initialize,
		Initialized:            s.initialized,
		Shutdown:               s.shutdown,
		SetTrace:               s.setTrace,
		TextDocumentDidOpen:    s.textDocumentDidOpen,
		TextDocumentDidSave:    s.textDocumentDidSave,
		TextDocumentDidClose:   s.textDocumentDidClose,
		TextDocumentDidChange:  s.textDocumentDidChange,
		TextDocumentDefinition: s.textDocumentDefinition,
		TextDocumentReferences: s.textDocumentReferences,
		TextDocumentHover:      s.textDocumentHover,
		TextDocumentCodeAction: s.textDocumentCodeAction,
		WorkspaceSymbol:        s.workspaceSymbol,
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

	// Extract workspace root
	if params.RootURI != nil {
		s.workspaceRoot = uriToFile(*params.RootURI)
	} else if params.RootPath != nil {
		s.workspaceRoot = *params.RootPath
	}

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

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: &version,
		},
	}, nil
}

func (s *Server) initialized(_ *glsp.Context, _ *protocol.InitializedParams) error {
	s.logger.Printf("initialized: workspace=%s", s.workspaceRoot)
	// Trigger initial lint
	s.triggerLint()
	return nil
}

func (s *Server) shutdown(_ *glsp.Context) error {
	s.debouncer.Stop()
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func (s *Server) setTrace(_ *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func (s *Server) textDocumentDidOpen(_ *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.documentStore.Set(params.TextDocument.URI, params.TextDocument.Text)
	return nil
}

func (s *Server) textDocumentDidChange(_ *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	// With full sync, the last content change has the full text
	for _, change := range params.ContentChanges {
		if c, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			s.documentStore.Set(params.TextDocument.URI, c.Text)
		}
	}
	return nil
}

func (s *Server) textDocumentDidSave(_ *glsp.Context, _ *protocol.DidSaveTextDocumentParams) error {
	s.triggerLint()
	return nil
}

func (s *Server) textDocumentDidClose(_ *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.documentStore.Delete(params.TextDocument.URI)
	// Clear diagnostics for the closed file
	if s.notifyFunc != nil {
		s.notifyFunc(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         params.TextDocument.URI,
			Diagnostics: []protocol.Diagnostic{},
		})
	}
	return nil
}

// triggerLint schedules a debounced lint run.
func (s *Server) triggerLint() {
	if s.runner == nil || s.workspaceRoot == "" {
		return
	}
	s.debouncer.Trigger(func() {
		s.runLint()
	})
}

// runLint executes vhdl-lint and publishes diagnostics.
func (s *Server) runLint() {
	if s.runner == nil || s.notifyFunc == nil {
		return
	}

	result, err := s.runner.Run(s.workspaceRoot, true)
	if err != nil {
		s.logger.Printf("lint error: %v", err)
		s.logMessage(protocol.MessageTypeError, "vhdl-lint failed: "+err.Error())
		return
	}

	// Rebuild symbol index if available
	if result.SymbolIndex != nil {
		s.symbolStore.Rebuild(result.SymbolIndex)
	}

	// Convert full result (violations + parse errors + missing checks + ambiguous constructs)
	diagsByFile := mapResultToDiagnostics(result, s.workspaceRoot)

	// Publish diagnostics for files with results
	currentURIs := make(map[string]bool, len(diagsByFile))
	for uri, diags := range diagsByFile {
		currentURIs[uri] = true
		s.notifyFunc(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		})
	}

	// Clear diagnostics for files that had violations in the previous run but not this one
	if s.previousURIs != nil {
		for uri := range s.previousURIs {
			if !currentURIs[uri] {
				s.notifyFunc(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
					URI:         uri,
					Diagnostics: []protocol.Diagnostic{},
				})
			}
		}
	}
	s.previousURIs = currentURIs
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
