package indexer

import (
	"fmt"
	"os"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

// ClearPolicyCache removes the stored policy cache for the given root path.
// Returns the cache directory that was targeted.
func ClearPolicyCache(rootPath string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("clear policy cache: config is nil")
	}
	cacheDir := resolveCacheDir(rootPath, cfg)
	if err := clearPolicyCache(cacheDir); err != nil {
		return cacheDir, err
	}
	return cacheDir, nil
}

func clearPolicyCache(cacheDir string) error {
	path := policyCachePath(cacheDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove policy cache: %w", err)
	}
	return nil
}
