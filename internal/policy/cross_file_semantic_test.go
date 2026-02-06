package policy_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

type semanticExpected struct {
	Rules []string `json:"rules"`
}

func TestCrossFileSemanticFixtures(t *testing.T) {
	repoRoot := findRepoRoot(t)
	policyBin := ensurePolicyBinary(t, repoRoot)
	t.Setenv("VHDL_POLICY_BIN", policyBin)
	baseDir := filepath.Join(repoRoot, "testdata", "cross_file_semantic")
	allRules := collectRustPolicyRules(t, repoRoot)
	sem := make(chan struct{}, maxParallelTests())
	filters := parseFixtureFilter(os.Getenv("VHDL_CROSS_FILE_FILTER"))
	timingEnabled := os.Getenv("VHDL_CROSS_FILE_TIMING") != ""

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		conceptDir := filepath.Join(baseDir, entry.Name())
		variantEntries, err := os.ReadDir(conceptDir)
		if err != nil {
			t.Fatalf("read concept dir %s: %v", conceptDir, err)
		}

		for _, variant := range variantEntries {
			if !variant.IsDir() {
				continue
			}
			variantDir := filepath.Join(conceptDir, variant.Name())
			testName := entry.Name() + "/" + variant.Name()
			if !fixtureIncluded(testName, filters) {
				continue
			}
			t.Run(testName, func(t *testing.T) {
				t.Parallel()
				sem <- struct{}{}
				defer func() { <-sem }()
				start := time.Now()

				expectedPath := filepath.Join(variantDir, "expected.json")
				expected := loadSemanticExpected(t, expectedPath)

				files := collectVHDLFiles(t, variantDir)

				cfg := loadSemanticConfig(t, variantDir)
				cfg.Standard = "2008"
				requireMapping := false
				cfg.Analysis.RequireLibraryMapping = &requireMapping
				cfg.Lint.Rules = make(map[string]string, len(allRules))
				for rule := range allRules {
					cfg.Lint.Rules[rule] = "off"
				}
				for _, rule := range expected.Rules {
					cfg.Lint.Rules[rule] = "warning"
				}
				disabled := false
				cfg.Analysis.Cache.Enabled = &disabled
				cfg.Libraries = map[string]config.LibraryConfig{
					"work": {
						Files:        files,
						Exclude:      []string{},
						IsThirdParty: false,
					},
				}

				result := lintWithConfigForSemantic(t, repoRoot, cfg)
				actual := collectRules(result)
				if !rulesMatch(actual, expected.Rules) {
					t.Fatalf("expected rules %v, got %v", expected.Rules, actual)
				}
				if timingEnabled {
					t.Logf("duration=%s", time.Since(start))
				}
			})
		}
	}
}

func maxParallelTests() int {
	if raw := os.Getenv("VHDL_CROSS_FILE_PARALLEL"); raw != "" {
		if val, err := strconv.Atoi(raw); err == nil && val > 0 {
			return val
		}
	}
	limit := 1
	if p := runtime.GOMAXPROCS(0); p < limit {
		return p
	}
	return limit
}

func parseFixtureFilter(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func fixtureIncluded(name string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.Contains(name, filter) {
			return true
		}
	}
	return false
}

func ensurePolicyBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	latest := latestPolicySourceTime(t, repoRoot)
	candidates := []string{
		filepath.Join(repoRoot, "target", "debug", "vhdl_policy"),
		filepath.Join(repoRoot, "target", "release", "vhdl_policy"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode()&0111 != 0 {
			if !latest.IsZero() && latest.After(info.ModTime()) {
				break
			}
			return candidate
		}
	}
	cmd := exec.Command("cargo", "build", "--bin", "vhdl_policy")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build vhdl_policy failed: %v\n%s", err, string(out))
	}
	if info, err := os.Stat(candidates[0]); err == nil && info.Mode()&0111 != 0 {
		return candidates[0]
	}
	if info, err := os.Stat(candidates[1]); err == nil && info.Mode()&0111 != 0 {
		return candidates[1]
	}
	t.Fatalf("vhdl_policy binary not found after build")
	return ""
}

func latestPolicySourceTime(t *testing.T, repoRoot string) time.Time {
	t.Helper()
	latest := time.Time{}
	policyDir := filepath.Join(repoRoot, "src", "policy")
	_ = filepath.WalkDir(policyDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rs" {
			return nil
		}
		if info, err := os.Stat(path); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
		return nil
	})
	extra := []string{
		filepath.Join(repoRoot, "Cargo.toml"),
		filepath.Join(repoRoot, "Cargo.lock"),
		filepath.Join(repoRoot, "src", "bin", "vhdl_policy.rs"),
	}
	for _, path := range extra {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func loadSemanticExpected(t *testing.T, path string) semanticExpected {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read expected.json: %v", err)
	}
	var exp semanticExpected
	if err := json.Unmarshal(data, &exp); err != nil {
		t.Fatalf("parse expected.json: %v", err)
	}
	return exp
}

func collectVHDLFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".vhd" && ext != ".vhdl" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no VHDL files found in %s", dir)
	}
	return files
}

func rulesMatch(actual []string, expected []string) bool {
	a := append([]string{}, actual...)
	e := append([]string{}, expected...)
	sort.Strings(a)
	sort.Strings(e)
	if len(a) != len(e) {
		return false
	}
	for i := range a {
		if a[i] != e[i] {
			return false
		}
	}
	return true
}

func lintWithConfigForSemantic(t *testing.T, repoRoot string, cfg *config.Config) indexer.LintResult {
	t.Helper()

	idx := indexer.NewWithConfig(cfg)
	idx.JSONOutput = true
	var outputBuf bytes.Buffer
	idx.Output = &outputBuf

	runErr := idx.Run(repoRoot)
	if runErr != nil {
		t.Fatalf("lint failed: %v", runErr)
	}

	var result indexer.LintResult
	if err := json.Unmarshal(outputBuf.Bytes(), &result); err != nil {
		t.Fatalf("parse lint result: %v", err)
	}

	return result
}

func loadSemanticConfig(t *testing.T, dir string) *config.Config {
	t.Helper()
	path := filepath.Join(dir, "vhdl_lint.json")
	if _, err := os.Stat(path); err == nil {
		cfg, err := config.LoadFile(path)
		if err != nil {
			t.Fatalf("load fixture config %s: %v", path, err)
		}
		return cfg
	}
	return config.DefaultConfig()
}
