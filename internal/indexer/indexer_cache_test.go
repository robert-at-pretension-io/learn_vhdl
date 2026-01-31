package indexer

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

type countingExtractor struct {
	inner FactsExtractor
	count *int32
}

func (c *countingExtractor) Extract(path string) (extractor.FileFacts, error) {
	atomic.AddInt32(c.count, 1)
	return c.inner.Extract(path)
}

func writeVHDL(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func defaultTestConfig(files []string, cacheDir string, cacheEnabled bool) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files:        files,
			Exclude:      []string{},
			IsThirdParty: false,
		},
	}
	cfg.Lint.Rules = map[string]string{}
	cfg.Analysis.Cache.Dir = cacheDir
	enabled := cacheEnabled
	cfg.Analysis.Cache.Enabled = &enabled
	return cfg
}

func runIndexerForTest(t *testing.T, idx *Indexer, rootPath string) LintResult {
	t.Helper()
	idx.JSONOutput = true

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer

	runErr := idx.Run(rootPath)
	_ = writer.Close()
	os.Stdout = oldStdout

	if runErr != nil {
		t.Fatalf("lint failed: %v", runErr)
	}

	output, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var result LintResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("parse lint result: %v", err)
	}

	return result
}

func normalizeResult(result LintResult) LintResult {
	sort.Slice(result.Violations, func(i, j int) bool {
		if result.Violations[i].Rule != result.Violations[j].Rule {
			return result.Violations[i].Rule < result.Violations[j].Rule
		}
		if result.Violations[i].File != result.Violations[j].File {
			return result.Violations[i].File < result.Violations[j].File
		}
		return result.Violations[i].Line < result.Violations[j].Line
	})
	sort.Slice(result.Files, func(i, j int) bool {
		return result.Files[i].Path < result.Files[j].Path
	})
	return result
}

func TestCacheReuseAvoidsReextract(t *testing.T) {
	dir := t.TempDir()
	file := writeVHDL(t, dir, "a.vhd", "entity a is end entity; architecture rtl of a is begin end architecture;")
	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{file}, cacheDir, true)

	var count int32
	idx := NewWithConfig(cfg)
	idx.extractorFactory = func() FactsExtractor {
		return &countingExtractor{inner: extractor.New(), count: &count}
	}
	runIndexerForTest(t, idx, dir)
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("expected 1 extract on first run, got %d", got)
	}

	var count2 int32
	idx2 := NewWithConfig(cfg)
	idx2.extractorFactory = func() FactsExtractor {
		return &countingExtractor{inner: extractor.New(), count: &count2}
	}
	runIndexerForTest(t, idx2, dir)
	if got := atomic.LoadInt32(&count2); got != 0 {
		t.Fatalf("expected 0 extracts on cached run, got %d", got)
	}
}

func TestCacheInvalidationOnChange(t *testing.T) {
	dir := t.TempDir()
	file := writeVHDL(t, dir, "a.vhd", "entity a is end entity; architecture rtl of a is begin end architecture;")
	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{file}, cacheDir, true)

	idx := NewWithConfig(cfg)
	runIndexerForTest(t, idx, dir)

	// Modify file contents
	if err := os.WriteFile(file, []byte("entity a is end entity; architecture rtl of a is begin -- change\nend architecture;"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}

	var count int32
	idx2 := NewWithConfig(cfg)
	idx2.extractorFactory = func() FactsExtractor {
		return &countingExtractor{inner: extractor.New(), count: &count}
	}
	runIndexerForTest(t, idx2, dir)
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("expected re-extract after change, got %d", got)
	}
}

func TestCacheInvalidationOnVersionChange(t *testing.T) {
	dir := t.TempDir()
	file := writeVHDL(t, dir, "a.vhd", "entity a is end entity; architecture rtl of a is begin end architecture;")
	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{file}, cacheDir, true)

	idx := NewWithConfig(cfg)
	idx.cacheVersionOverride = &cacheVersions{parser: "p1", extractor: "e1"}
	runIndexerForTest(t, idx, dir)

	var count int32
	idx2 := NewWithConfig(cfg)
	idx2.cacheVersionOverride = &cacheVersions{parser: "p2", extractor: "e1"}
	idx2.extractorFactory = func() FactsExtractor {
		return &countingExtractor{inner: extractor.New(), count: &count}
	}
	runIndexerForTest(t, idx2, dir)
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("expected re-extract after version change, got %d", got)
	}
}

func TestCachedRunMatchesFresh(t *testing.T) {
	dir := t.TempDir()
	file1 := writeVHDL(t, dir, "pkg.vhd", "package my_pkg is constant C : integer := 1; end package;")
	file2 := writeVHDL(t, dir, "ent.vhd", "library work; use work.my_pkg.all; entity e is end entity; architecture rtl of e is signal x : integer := C; begin end architecture;")
	cacheDir := filepath.Join(dir, ".cache")

	cfgNoCache := defaultTestConfig([]string{file1, file2}, cacheDir, false)
	idxNoCache := NewWithConfig(cfgNoCache)
	fresh := normalizeResult(runIndexerForTest(t, idxNoCache, dir))

	cfgCache := defaultTestConfig([]string{file1, file2}, cacheDir, true)
	idxCache := NewWithConfig(cfgCache)
	cached := normalizeResult(runIndexerForTest(t, idxCache, dir))

	if fresh.Summary != cached.Summary {
		t.Fatalf("summary mismatch: fresh=%+v cached=%+v", fresh.Summary, cached.Summary)
	}
	if len(fresh.Violations) != len(cached.Violations) {
		t.Fatalf("violation count mismatch: fresh=%d cached=%d", len(fresh.Violations), len(cached.Violations))
	}
	for i := range fresh.Violations {
		if fresh.Violations[i] != cached.Violations[i] {
			t.Fatalf("violation mismatch at %d: fresh=%+v cached=%+v", i, fresh.Violations[i], cached.Violations[i])
		}
	}
}
