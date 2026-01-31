package indexer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestVerboseImpactOutput(t *testing.T) {
	dir := t.TempDir()
	fileA := writeVHDL(t, dir, "a.vhd", "package my_pkg is end package;")
	fileB := writeVHDL(t, dir, "b.vhd", "library work; use work.my_pkg.all; entity b is end entity; architecture rtl of b is begin end architecture;")

	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{fileA, fileB}, cacheDir, true)

	idx := NewWithConfig(cfg)
	idx.Verbose = true
	idx.JSONOutput = true
	idx.extractorFactory = func() FactsExtractor { return extractor.New() }

	// First run to populate cache (manual capture to avoid JSON parsing)
	{
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe stdout: %v", err)
		}
		oldStdout := os.Stdout
		os.Stdout = writer
		if err := idx.Run(dir); err != nil {
			_ = writer.Close()
			os.Stdout = oldStdout
			t.Fatalf("run: %v", err)
		}
		_ = writer.Close()
		os.Stdout = oldStdout
		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}

	// Second run should be cached (no changed files, no impact output)
	buf := &bytes.Buffer{}
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	idx2 := NewWithConfig(cfg)
	idx2.Verbose = true
	idx2.JSONOutput = true
	idx2.extractorFactory = func() FactsExtractor { return extractor.New() }
	if err := idx2.Run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(buf, r)
	_ = r.Close()

	if bytes.Contains(buf.Bytes(), []byte("=== Cache Impact ===")) {
		t.Fatalf("did not expect cache impact output on cached run")
	}
}

func TestProgressOutputIncludesDeps(t *testing.T) {
	dir := t.TempDir()
	fileA := writeVHDL(t, dir, "pkg.vhd", "package my_pkg is end package;")
	fileB := writeVHDL(t, dir, "use.vhd", "library work; use work.my_pkg.all; entity e is end entity; architecture rtl of e is begin end architecture;")

	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{fileA, fileB}, cacheDir, false)

	idx := NewWithConfig(cfg)
	idx.Progress = true

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer
	if err := idx.Run(dir); err != nil {
		_ = writer.Close()
		os.Stdout = oldStdout
		t.Fatalf("run: %v", err)
	}
	_ = writer.Close()
	os.Stdout = oldStdout

	output, _ := io.ReadAll(reader)
	_ = reader.Close()

	if !bytes.Contains(output, []byte("=== Extraction Progress ===")) {
		t.Fatalf("expected progress header in output")
	}
	if !bytes.Contains(output, []byte("deps:")) {
		t.Fatalf("expected deps line in progress output")
	}
}
