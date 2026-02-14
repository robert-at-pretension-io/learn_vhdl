package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcEnvelope struct {
	JSONRPC string           `json:"jsonrpc,omitempty"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

type messageCollector struct {
	ch              <-chan rpcEnvelope
	backlog         []rpcEnvelope
	onServerRequest func(rpcEnvelope) error
}

func (c *messageCollector) waitResponse(t *testing.T, id int, timeout time.Duration) rpcEnvelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for i := 0; i < len(c.backlog); {
			m := c.backlog[i]
			if c.handleServerRequest(t, m) {
				c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
				continue
			}
			if msgID, ok := envelopeIntID(m); ok && msgID == id {
				c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
				return m
			}
			i++
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timeout waiting for response id=%d", id)
		}
		select {
		case m, ok := <-c.ch:
			if !ok {
				t.Fatalf("message stream closed while waiting for response id=%d", id)
			}
			if c.handleServerRequest(t, m) {
				continue
			}
			if msgID, ok := envelopeIntID(m); ok && msgID == id {
				return m
			}
			c.backlog = append(c.backlog, m)
		case <-time.After(remaining):
			t.Fatalf("timeout waiting for response id=%d", id)
		}
	}
}

func (c *messageCollector) waitNotification(t *testing.T, method string, timeout time.Duration, match func(rpcEnvelope) bool) rpcEnvelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for i := 0; i < len(c.backlog); {
			m := c.backlog[i]
			if c.handleServerRequest(t, m) {
				c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
				continue
			}
			if m.Method == method && (match == nil || match(m)) {
				c.backlog = append(c.backlog[:i], c.backlog[i+1:]...)
				return m
			}
			i++
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("timeout waiting for notification method=%s (backlog=%s)", method, backlogMethods(c.backlog))
		}
		select {
		case m, ok := <-c.ch:
			if !ok {
				t.Fatalf("message stream closed while waiting for notification method=%s", method)
			}
			if c.handleServerRequest(t, m) {
				continue
			}
			if m.Method == method && (match == nil || match(m)) {
				return m
			}
			c.backlog = append(c.backlog, m)
		case <-time.After(remaining):
			t.Fatalf("timeout waiting for notification method=%s (backlog=%s)", method, backlogMethods(c.backlog))
		}
	}
}

func (c *messageCollector) handleServerRequest(t *testing.T, msg rpcEnvelope) bool {
	t.Helper()
	if msg.Method == "" || msg.ID == nil || c.onServerRequest == nil {
		return false
	}
	if err := c.onServerRequest(msg); err != nil {
		t.Fatalf("respond to server request %q: %v", msg.Method, err)
	}
	return true
}

func TestLSPStdioE2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	workspace := t.TempDir()
	targetFile := filepath.Join(workspace, "top.vhd")
	if err := os.WriteFile(targetFile, []byte("process(clk) begin null; end process;\n"), 0o644); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	fakeLint := filepath.Join(workspace, "fake_vhdl_lint.sh")
	fakeScript := `#!/bin/sh
flag="$1"
target="$2"
file="$target"
if [ -d "$target" ]; then
  file="$target/top.vhd"
fi
cat <<EOF
{"violations":[{"rule":"unused_signal","severity":"warning","file":"$file","line":1,"message":"unused signal"}],"summary":{"total_violations":1,"errors":0,"warnings":1,"info":0},"symbol_index":{"entities":[{"name":"uart_tx","file":"$file","line":1}]}}
EOF
`
	if err := os.WriteFile(fakeLint, []byte(fakeScript), 0o755); err != nil {
		t.Fatalf("write fake lint script: %v", err)
	}

	serverBin := filepath.Join(workspace, "vhdl-lsp")
	build := exec.Command("go", "build", "-o", serverBin, "./cmd/vhdl-lsp")
	build.Dir = repoRoot
	build.Env = os.Environ()
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build vhdl-lsp failed: %v\n%s", err, string(out))
	}

	cmd := exec.Command(serverBin)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"VHDL_LINT_BIN="+fakeLint,
		"VHDL_LSP_DEBOUNCE_MS=10",
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
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_, _ = io.Copy(io.Discard, stderr)
		_ = cmd.Wait()
	})

	msgCh := make(chan rpcEnvelope, 256)
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

	initializeID := 1
	rootURI := "file://" + filepath.ToSlash(workspace)
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      initializeID,
		"method":  "initialize",
		"params": map[string]any{
			"rootUri":      rootURI,
			"capabilities": map[string]any{},
		},
	}); err != nil {
		t.Fatalf("send initialize: %v", err)
	}
	initResp := collector.waitResponse(t, initializeID, 5*time.Second)
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %+v", initResp.Error)
	}

	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "initialized",
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("send initialized: %v", err)
	}

	targetURI := "file://" + filepath.ToSlash(targetFile)
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":        targetURI,
				"languageId": "vhdl",
				"version":    1,
				"text":       "pro",
			},
		},
	}); err != nil {
		t.Fatalf("send didOpen: %v", err)
	}

	completionID := 2
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      completionID,
		"method":  "textDocument/completion",
		"params": map[string]any{
			"textDocument": map[string]any{"uri": targetURI},
			"position":     map[string]any{"line": 0, "character": 3},
		},
	}); err != nil {
		t.Fatalf("send completion request: %v", err)
	}
	compResp := collector.waitResponse(t, completionID, 5*time.Second)
	if compResp.Error != nil {
		t.Fatalf("completion returned error: %+v", compResp.Error)
	}
	var compList struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(compResp.Result, &compList); err != nil {
		t.Fatalf("decode completion result: %v", err)
	}
	foundProcess := false
	for _, item := range compList.Items {
		if item.Label == "process" {
			foundProcess = true
			break
		}
	}
	if !foundProcess {
		t.Fatalf("expected completion result to contain 'process', got %d items", len(compList.Items))
	}

	diagNotif := collector.waitNotification(t, string(protocol.ServerTextDocumentPublishDiagnostics), 5*time.Second, func(m rpcEnvelope) bool {
		var p protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return false
		}
		return p.URI == targetURI
	})
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(diagNotif.Params, &diagParams); err != nil {
		t.Fatalf("decode diagnostics notification: %v", err)
	}
	if len(diagParams.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for opened file")
	}

	shutdownID := 3
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      shutdownID,
		"method":  "shutdown",
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("send shutdown: %v", err)
	}
	shutdownResp := collector.waitResponse(t, shutdownID, 5*time.Second)
	if shutdownResp.Error != nil {
		t.Fatalf("shutdown returned error: %+v", shutdownResp.Error)
	}
	if err := rpc.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "exit",
		"params":  map[string]any{},
	}); err != nil {
		t.Fatalf("send exit: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("vhdl-lsp process exit: %v", err)
		}
	case err := <-errCh:
		t.Fatalf("rpc stream error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for vhdl-lsp process to exit")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatalf("could not find repo root from %s", wd)
		}
		dir = next
	}
}

func readRPCStream(r io.Reader, out chan<- rpcEnvelope, errCh chan<- error) {
	defer close(out)
	br := bufio.NewReader(r)
	for {
		contentLength := 0
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				errCh <- err
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "content-length:") {
				v := strings.TrimSpace(line[len("Content-Length:"):])
				n, err := strconv.Atoi(v)
				if err != nil {
					errCh <- fmt.Errorf("invalid content-length %q: %w", v, err)
					return
				}
				contentLength = n
			}
		}
		if contentLength <= 0 {
			continue
		}
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(br, body); err != nil {
			errCh <- err
			return
		}
		var msg rpcEnvelope
		if err := json.Unmarshal(body, &msg); err != nil {
			errCh <- err
			return
		}
		out <- msg
	}
}

func writeRPCMessage(w io.Writer, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(b))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func envelopeIntID(msg rpcEnvelope) (int, bool) {
	if msg.ID == nil {
		return 0, false
	}
	var i int
	if err := json.Unmarshal(*msg.ID, &i); err == nil {
		return i, true
	}
	return 0, false
}

type lockedRPCWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *lockedRPCWriter) write(msg any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return writeRPCMessage(w.w, msg)
}

func respondToServerRequest(w *lockedRPCWriter, msg rpcEnvelope) error {
	id, err := rawMessageToAny(msg.ID)
	if err != nil {
		return err
	}
	return w.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]any{},
	})
}

func rawMessageToAny(raw *json.RawMessage) (any, error) {
	if raw == nil {
		return nil, fmt.Errorf("missing request id")
	}
	var v any
	if err := json.Unmarshal(*raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func backlogMethods(backlog []rpcEnvelope) string {
	if len(backlog) == 0 {
		return "<empty>"
	}
	methods := make([]string, 0, len(backlog))
	for _, m := range backlog {
		if m.Method != "" {
			if m.Method == string(protocol.MethodProgress) {
				var p map[string]any
				if err := json.Unmarshal(m.Params, &p); err == nil {
					token := p["token"]
					kind := ""
					if v, ok := p["value"].(map[string]any); ok {
						if k, ok := v["kind"].(string); ok {
							kind = k
						}
					}
					if token == nil {
						methods = append(methods, fmt.Sprintf("%s(raw=%s)", m.Method, strings.ReplaceAll(string(m.Params), "\n", "")))
						continue
					}
					methods = append(methods, fmt.Sprintf("%s(token=%v,kind=%s)", m.Method, token, kind))
					continue
				}
			}
			methods = append(methods, m.Method)
			continue
		}
		if m.ID != nil {
			if id, ok := envelopeIntID(m); ok {
				methods = append(methods, fmt.Sprintf("response:%d", id))
			} else {
				methods = append(methods, "response")
			}
			continue
		}
		methods = append(methods, "<unknown>")
	}
	return strings.Join(methods, ",")
}
