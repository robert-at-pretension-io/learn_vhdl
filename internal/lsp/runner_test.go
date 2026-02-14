package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
	return path
}

func TestRunnerRun_NonZeroNoJSONReturnsError(t *testing.T) {
	tmp := t.TempDir()
	bin := writeExecutable(t, tmp, "fake_lint.sh", "#!/bin/sh\necho 'fatal error' >&2\nexit 2\n")
	r := &Runner{binaryPath: bin}

	_, err := r.Run(tmp, true)
	if err == nil {
		t.Fatal("expected error for non-zero exit without JSON output")
	}
	if !strings.Contains(err.Error(), "fatal error") {
		t.Fatalf("expected stderr in error, got: %v", err)
	}
}

func TestRunnerRun_NonZeroWithJSONStillParses(t *testing.T) {
	tmp := t.TempDir()
	json := `{"violations":[],"summary":{"total_violations":0,"errors":0,"warnings":0,"info":0}}`
	bin := writeExecutable(t, tmp, "fake_lint.sh", "#!/bin/sh\necho '"+json+"'\nexit 1\n")
	r := &Runner{binaryPath: bin}

	result, err := r.Run(tmp, true)
	if err != nil {
		t.Fatalf("expected JSON output to be parsed despite non-zero exit, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Summary.TotalViolations != 0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
}

func TestRunnerRun_JSONParseErrorIncludesStderr(t *testing.T) {
	tmp := t.TempDir()
	bin := writeExecutable(t, tmp, "fake_lint.sh", "#!/bin/sh\necho 'not-json'\necho 'parser crash' >&2\nexit 1\n")
	r := &Runner{binaryPath: bin}

	_, err := r.Run(tmp, true)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse vhdl-lint output") {
		t.Fatalf("expected parse error context, got: %v", err)
	}
	if !strings.Contains(err.Error(), "parser crash") {
		t.Fatalf("expected stderr context, got: %v", err)
	}
}

func TestRunnerRunWithContextCancellation(t *testing.T) {
	tmp := t.TempDir()
	bin := writeExecutable(t, tmp, "fake_lint.sh", "#!/bin/sh\nexec sleep 2\n")
	r := &Runner{binaryPath: bin}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := r.RunWithContext(ctx, tmp, true)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected prompt cancellation, took %v", elapsed)
	}
}

func TestRunnerRun_UsesVHDLLintConfigEnv(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args.txt")
	bin := writeExecutable(t, tmp, "fake_lint.sh", "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+argsFile+"'\necho '{\"violations\":[],\"summary\":{\"total_violations\":0,\"errors\":0,\"warnings\":0,\"info\":0}}'\n")
	r := &Runner{binaryPath: bin}

	configPath := filepath.Join(tmp, "lsp_config.json")
	t.Setenv("VHDL_LINT_CONFIG", configPath)

	if _, err := r.Run(tmp, false); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	args := strings.Fields(string(data))
	if len(args) < 4 {
		t.Fatalf("expected at least 4 args, got %v", args)
	}
	if args[0] != "-c" || args[1] != configPath {
		t.Fatalf("expected config args '-c %s', got %v", configPath, args)
	}
	if args[2] != "-j" {
		t.Fatalf("expected -j arg, got %v", args)
	}
	if args[3] != tmp {
		t.Fatalf("expected lint target %q, got %q", tmp, args[3])
	}
}
