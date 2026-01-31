package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

const policyCacheVersion = 2

type policyCacheEntry struct {
	Version    int           `json:"version"`
	ConfigHash string        `json:"config_hash"`
	Files      []string      `json:"files"`
	Result     policy.Result `json:"result"`
}

func loadPolicyCache(dir string) (*policyCacheEntry, error) {
	path := policyCachePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entry policyCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parse policy cache: %w", err)
	}
	return &entry, nil
}

func savePolicyCache(dir string, entry policyCacheEntry) error {
	path := policyCachePath(dir)
	if err := writeJSONAtomic(path, entry); err != nil {
		return fmt.Errorf("write policy cache: %w", err)
	}
	return nil
}

func policyCachePath(dir string) string {
	return filepath.Join(dir, "policy_cache.json")
}

func policyCacheValid(entry *policyCacheEntry, input policy.Input, files []string) (bool, error) {
	if entry == nil {
		return false, nil
	}
	if entry.Version != policyCacheVersion {
		return false, nil
	}
	hash, err := policyConfigHash(input)
	if err != nil {
		return false, err
	}
	if entry.ConfigHash != hash {
		return false, nil
	}
	if len(entry.Files) != len(files) {
		return false, nil
	}
	for i := range files {
		if entry.Files[i] != files[i] {
			return false, nil
		}
	}
	return true, nil
}

func policyConfigHash(input policy.Input) (string, error) {
	policyVersion, err := policyRulesHash()
	if err != nil {
		return "", err
	}
	thirdParty := append([]string{}, input.ThirdPartyFiles...)
	sort.Strings(thirdParty)
	payload := struct {
		Standard        string                `json:"standard"`
		LintConfig      policy.LintRuleConfig `json:"lint_config"`
		ThirdPartyFiles []string              `json:"third_party_files"`
		PolicyVersion   string                `json:"policy_version"`
	}{
		Standard:        input.Standard,
		LintConfig:      input.LintConfig,
		ThirdPartyFiles: thirdParty,
		PolicyVersion:   policyVersion,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal policy config hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func policyRulesHash() (string, error) {
	repoRoot := findRepoRootForCache()
	if repoRoot == "" {
		return "", fmt.Errorf("policy rules hash: repo root not found")
	}
	root := filepath.Join(repoRoot, "src", "policy")
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("policy rules hash: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("policy rules hash: %s is not a directory", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rs" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("policy rules hash walk: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("policy rules hash: no policy sources found")
	}
	sort.Strings(files)

	hasher := sha256.New()
	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		if _, err := hasher.Write([]byte(rel)); err != nil {
			return "", fmt.Errorf("policy rules hash: %w", err)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", fmt.Errorf("policy rules hash: %w", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("policy rules hash read: %w", err)
		}
		if _, err := hasher.Write(data); err != nil {
			return "", fmt.Errorf("policy rules hash: %w", err)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return "", fmt.Errorf("policy rules hash: %w", err)
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
