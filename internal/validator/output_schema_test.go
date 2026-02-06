package validator_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/validator"
)

func TestOutputSchemaValidation(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixture := filepath.Join(repoRoot, "testdata", "verification", "output_schema_minimal.vhd")
	absFixture, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Standard = "2008"
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files:        []string{absFixture},
			Exclude:      []string{},
			IsThirdParty: false,
		},
	}
	disabled := false
	cfg.Analysis.Cache.Enabled = &disabled

	idx := indexer.NewWithConfig(cfg)
	idx.JSONOutput = true

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldCwd)
	}()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer

	var outputBuf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(&outputBuf, reader)
		done <- err
	}()

	runErr := idx.Run(repoRoot)
	_ = writer.Close()
	os.Stdout = oldStdout
	if err := <-done; err != nil {
		t.Fatalf("read output: %v", err)
	}
	_ = reader.Close()
	if runErr != nil {
		t.Fatalf("lint failed: %v", runErr)
	}

	var result indexer.LintResult
	if err := json.Unmarshal(outputBuf.Bytes(), &result); err != nil {
		t.Fatalf("parse lint result: %v", err)
	}

	outValidator, err := validator.NewOutputValidator()
	if err != nil {
		t.Fatalf("new output validator: %v", err)
	}
	if err := outValidator.Validate(result); err != nil {
		t.Fatalf("output schema validation failed: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	dir := start
	for {
		candidate := filepath.Join(dir, "testdata", "policy_rules", "manifest.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", start)
		}
		dir = parent
	}
}
