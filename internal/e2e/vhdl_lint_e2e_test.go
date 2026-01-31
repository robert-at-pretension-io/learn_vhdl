package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func TestVhdlLintE2E_Testdata(t *testing.T) {
	repoRoot := findRepoRoot(t)

	policyBin := ensurePolicyBinary(t, repoRoot)
	lintBin := buildLintBinary(t, repoRoot)

	home := t.TempDir()
	env := append(os.Environ(),
		"VHDL_POLICY_BIN="+policyBin,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)

	paths := []string{
		filepath.Join(repoRoot, "testdata", "vhdl"),
		filepath.Join(repoRoot, "testdata", "extractor_e2e"),
		filepath.Join(repoRoot, "testdata", "policy_rules"),
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			result := runLintJSON(t, lintBin, path, env)
			if len(result.ParseErrors) > 0 {
				t.Fatalf("parse errors in %s: %v", path, result.ParseErrors)
			}
		})
	}
}

func runLintJSON(t *testing.T, lintBin, path string, env []string) indexer.LintResult {
	t.Helper()

	cmd := exec.Command(lintBin, "--json", path)
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("vhdl-lint failed for %s: %v\nstderr:\n%s", path, err, stderr.String())
	}

	var result indexer.LintResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON output for %s: %v\nstdout:\n%s", path, err, stdout.String())
	}
	return result
}

func buildLintBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "vhdl-lint")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/vhdl-lint")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build vhdl-lint failed: %v\n%s", err, string(out))
	}
	return binPath
}

func ensurePolicyBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	binPath := filepath.Join(repoRoot, "target", "release", "vhdl_policy")
	if info, err := os.Stat(binPath); err == nil && info.Mode()&0111 != 0 {
		return binPath
	}
	cmd := exec.Command("cargo", "build", "--release", "--bin", "vhdl_policy")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build vhdl_policy failed: %v\n%s", err, string(out))
	}
	return binPath
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
