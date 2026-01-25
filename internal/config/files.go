package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolvedLibrary contains the expanded file list for a library
type ResolvedLibrary struct {
	Name         string
	Files        []string
	IsThirdParty bool
}

// ResolveLibraries expands all glob patterns and returns resolved file lists
func (c *Config) ResolveLibraries(rootPath string) ([]ResolvedLibrary, error) {
	var result []ResolvedLibrary

	for libName, libCfg := range c.Libraries {
		resolved := ResolvedLibrary{
			Name:         libName,
			IsThirdParty: libCfg.IsThirdParty,
		}

		// Expand all file patterns
		fileSet := make(map[string]bool)
		for _, pattern := range libCfg.Files {
			// Make pattern absolute if relative
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(rootPath, pattern)
			}

			// Use doublestar-style glob expansion
			matches, err := expandGlob(pattern)
			if err != nil {
				// Silently skip invalid patterns
				continue
			}

			for _, match := range matches {
				// Only include VHDL files
				ext := strings.ToLower(filepath.Ext(match))
				if ext == ".vhd" || ext == ".vhdl" {
					fileSet[match] = true
				}
			}
		}

		// Remove excluded files
		for _, pattern := range libCfg.Exclude {
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(rootPath, pattern)
			}

			matches, err := expandGlob(pattern)
			if err != nil {
				continue
			}

			for _, match := range matches {
				delete(fileSet, match)
			}
		}

		// Convert set to slice
		for f := range fileSet {
			resolved.Files = append(resolved.Files, f)
		}

		result = append(result, resolved)
	}

	return result, nil
}

// expandGlob expands a glob pattern, handling ** for recursive matching
func expandGlob(pattern string) ([]string, error) {
	// Check if pattern contains **
	if strings.Contains(pattern, "**") {
		return expandDoubleStarGlob(pattern)
	}

	// Simple glob
	return filepath.Glob(pattern)
}

// expandDoubleStarGlob handles ** patterns by walking the directory tree
func expandDoubleStarGlob(pattern string) ([]string, error) {
	var results []string

	// Split pattern at **
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return filepath.Glob(pattern)
	}

	baseDir := filepath.Clean(parts[0])
	if baseDir == "" {
		baseDir = "."
	}
	suffix := parts[1]
	if strings.HasPrefix(suffix, string(filepath.Separator)) {
		suffix = suffix[1:]
	}

	// Walk the directory tree
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Check if file matches the suffix pattern
		if suffix == "" {
			results = append(results, path)
			return nil
		}

		// Build the pattern for this specific path
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}

		// Try to match the suffix pattern against the relative path
		if matchSuffix(relPath, suffix) {
			results = append(results, path)
		}

		return nil
	})

	return results, err
}

// matchSuffix checks if a path matches a suffix pattern (after **)
func matchSuffix(path, pattern string) bool {
	// Handle patterns like "/*.vhd" or "*.vhd"
	pattern = strings.TrimPrefix(pattern, string(filepath.Separator))

	// If pattern has no directory component, match against filename
	if !strings.Contains(pattern, string(filepath.Separator)) {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		return matched
	}

	// For patterns with directory components, try matching
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	// Also try matching just the suffix
	if len(path) > len(pattern) {
		suffix := path[len(path)-len(pattern):]
		matched, _ = filepath.Match(pattern, suffix)
		return matched
	}

	return false
}

// GetAllFiles returns all VHDL files from all libraries (flattened)
func (c *Config) GetAllFiles(rootPath string) ([]string, error) {
	libs, err := c.ResolveLibraries(rootPath)
	if err != nil {
		return nil, err
	}

	fileSet := make(map[string]bool)
	for _, lib := range libs {
		for _, f := range lib.Files {
			fileSet[f] = true
		}
	}

	var result []string
	for f := range fileSet {
		result = append(result, f)
	}

	return result, nil
}

// FileLibraryInfo contains library information for a specific file
type FileLibraryInfo struct {
	LibraryName  string
	IsThirdParty bool
}

// GetFileLibrary returns the library information for a file
func (c *Config) GetFileLibrary(filePath string, rootPath string) FileLibraryInfo {
	libs, err := c.ResolveLibraries(rootPath)
	if err != nil {
		return FileLibraryInfo{LibraryName: "work", IsThirdParty: false}
	}

	absPath, _ := filepath.Abs(filePath)

	for _, lib := range libs {
		for _, f := range lib.Files {
			absF, _ := filepath.Abs(f)
			if absPath == absF {
				return FileLibraryInfo{
					LibraryName:  lib.Name,
					IsThirdParty: lib.IsThirdParty,
				}
			}
		}
	}

	// Default to work library
	return FileLibraryInfo{LibraryName: "work", IsThirdParty: false}
}
