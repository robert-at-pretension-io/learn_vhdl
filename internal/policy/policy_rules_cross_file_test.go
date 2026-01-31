package policy_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func TestCrossFileDuplicatePrimaryUnits(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixturesDir := filepath.Join(repoRoot, "testdata", "policy_rules")
	fileA := filepath.Join(fixturesDir, "cross_file_dup_a.vhd")
	fileB := filepath.Join(fixturesDir, "cross_file_dup_b.vhd")

	rules := map[string]string{
		"duplicate_entity_in_library":  "error",
		"duplicate_package_in_library": "error",
	}

	cfg := config.DefaultConfig()
	cfg.Lint.Rules = rules
	disabled := false
	cfg.Analysis.Cache.Enabled = &disabled
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files:        []string{fileA, fileB},
			IsThirdParty: false,
		},
	}

	result := lintWithConfig(t, repoRoot, cfg)
	if !hasRule(result, "duplicate_entity_in_library") {
		t.Fatalf("expected duplicate_entity_in_library, got rules: %v", collectRules(result))
	}
	if !hasRule(result, "duplicate_package_in_library") {
		t.Fatalf("expected duplicate_package_in_library, got rules: %v", collectRules(result))
	}
}

func TestCrossFileDuplicatePrimaryUnitsDifferentLibraries(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixturesDir := filepath.Join(repoRoot, "testdata", "policy_rules")
	fileA := filepath.Join(fixturesDir, "cross_file_dup_a.vhd")
	fileB := filepath.Join(fixturesDir, "cross_file_dup_b.vhd")

	rules := map[string]string{
		"duplicate_entity_in_library":  "error",
		"duplicate_package_in_library": "error",
	}

	cfg := config.DefaultConfig()
	cfg.Lint.Rules = rules
	disabled := false
	cfg.Analysis.Cache.Enabled = &disabled
	cfg.Libraries = map[string]config.LibraryConfig{
		"lib_a": {
			Files:        []string{fileA},
			IsThirdParty: false,
		},
		"lib_b": {
			Files:        []string{fileB},
			IsThirdParty: false,
		},
	}

	result := lintWithConfig(t, repoRoot, cfg)
	if hasRule(result, "duplicate_entity_in_library") {
		t.Fatalf("did not expect duplicate_entity_in_library, got rules: %v", collectRules(result))
	}
	if hasRule(result, "duplicate_package_in_library") {
		t.Fatalf("did not expect duplicate_package_in_library, got rules: %v", collectRules(result))
	}
}

func lintWithConfig(t *testing.T, repoRoot string, cfg *config.Config) indexer.LintResult {
	t.Helper()

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

	runErr := idx.Run(repoRoot)
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

	var result indexer.LintResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("parse lint result: %v", err)
	}

	return result
}
