package policy

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func findPolicySourceRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		dir = start
	}
	for {
		if policySourcesExist(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func policySourcesExist(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err != nil {
		return false
	}
	if info, err := os.Stat(filepath.Join(root, "src", "policy")); err != nil || !info.IsDir() {
		return false
	}
	return true
}

func policyBinaryOutdated(binaryPath, sourceRoot string, extraSources []string) (bool, error) {
	binInfo, err := os.Stat(binaryPath)
	if err != nil {
		return false, err
	}
	latest, err := latestPolicySourceTime(sourceRoot, extraSources)
	if err != nil {
		return false, err
	}
	return latest.After(binInfo.ModTime()), nil
}

func latestPolicySourceTime(root string, extra []string) (time.Time, error) {
	latest := time.Time{}
	policyDir := filepath.Join(root, "src", "policy")
	if err := filepath.WalkDir(policyDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rs" {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	}); err != nil {
		return time.Time{}, err
	}

	for _, extraPath := range extra {
		info, err := os.Stat(extraPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	if latest.IsZero() {
		return time.Time{}, os.ErrNotExist
	}
	return latest, nil
}
