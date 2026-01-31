package policy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func TestPolicyRuleManifestsCoverRustRules(t *testing.T) {
	repoRoot := findRepoRoot(t)
	rustRules := collectRustPolicyRules(t, repoRoot)

	fixturesDir := filepath.Join(repoRoot, "testdata", "policy_rules")
	manifestPath := filepath.Join(fixturesDir, "manifest.json")
	negativeManifestPath := filepath.Join(fixturesDir, "manifest_negative.json")
	multifileManifestPath := filepath.Join(fixturesDir, "manifest_multifile.json")
	manifest := loadManifest(t, manifestPath)
	negativeManifest := loadManifest(t, negativeManifestPath)
	ensureManifestParity(t, manifest, negativeManifest)
	multifile := loadRuleList(t, multifileManifestPath)

	var missing []string
	for rule := range rustRules {
		if _, ok := manifest[rule]; !ok {
			if _, ok := multifile[rule]; ok {
				continue
			}
			missing = append(missing, rule)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("policy rules missing from manifests: %v", missing)
	}

	var extra []string
	for rule := range manifest {
		if _, ok := rustRules[rule]; !ok {
			extra = append(extra, rule)
		}
	}
	for rule := range multifile {
		if _, ok := rustRules[rule]; !ok {
			extra = append(extra, rule)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Fatalf("manifests contain rules not found in Rust policy sources: %v", extra)
	}
}

func collectRustPolicyRules(t *testing.T, repoRoot string) map[string]struct{} {
	t.Helper()
	root := filepath.Join(repoRoot, "src", "policy")
	re := regexp.MustCompile(`rule:\s*"([a-z0-9_]+)"`)
	rules := make(map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rs" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		matches := re.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			if len(match) > 1 {
				rules[match[1]] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan policy sources: %v", err)
	}
	if len(rules) == 0 {
		t.Fatalf("no policy rules found in Rust sources under %s", root)
	}
	return rules
}

func loadRuleList(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}
		}
		t.Fatalf("stat %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rule list: %v", err)
	}
	var rules []string
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("parse rule list: %v", err)
	}
	out := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		out[rule] = struct{}{}
	}
	return out
}
