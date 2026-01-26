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

type ruleManifest map[string]string

func TestPolicyRuleFixtures(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fixturesDir := filepath.Join(repoRoot, "testdata", "policy_rules")
	manifestPath := filepath.Join(fixturesDir, "manifest.json")

	manifest := loadManifest(t, manifestPath)
	byFile := map[string][]string{}
	for rule, file := range manifest {
		byFile[file] = append(byFile[file], rule)
	}

	for relFile, rules := range byFile {
		relFile := relFile
		rules := rules
		t.Run(relFile, func(t *testing.T) {
			filePath := filepath.Join(fixturesDir, relFile)
			result := lintFile(t, repoRoot, filePath)
			for _, rule := range rules {
				if !hasRule(result, rule) {
					t.Fatalf("expected rule %q for %s; got rules: %v", rule, relFile, collectRules(result))
				}
			}
		})
	}
}

func loadManifest(t *testing.T, path string) ruleManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest ruleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	return manifest
}

func lintFile(t *testing.T, repoRoot, filePath string) indexer.LintResult {
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Lint.Rules = map[string]string{}
	cfg.Libraries = map[string]config.LibraryConfig{
		"work": {
			Files:        []string{absFile},
			Exclude:      []string{},
			IsThirdParty: false,
		},
	}

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

func hasRule(result indexer.LintResult, rule string) bool {
	for _, v := range result.Violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func collectRules(result indexer.LintResult) []string {
	rules := make([]string, 0, len(result.Violations))
	for _, v := range result.Violations {
		rules = append(rules, v.Rule)
	}
	return rules
}

func findRepoRoot(t *testing.T) string {
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
