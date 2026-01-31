package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

const cacheIndexVersion = 1

type cacheEntry struct {
	ContentHash     string `json:"content_hash"`
	FactsPath       string `json:"facts_path"`
	ParserVersion   string `json:"parser_version"`
	ExtractorVersion string `json:"extractor_version"`
}

type cacheIndex struct {
	Version int                  `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type factsCache struct {
	dir             string
	parserVersion   string
	extractorVersion string
	mu              sync.Mutex
	index           cacheIndex
}

func newFactsCache(dir, parserVersion, extractorVersion string) *factsCache {
	return &factsCache{
		dir:              dir,
		parserVersion:    parserVersion,
		extractorVersion: extractorVersion,
		index: cacheIndex{
			Version: cacheIndexVersion,
			Entries: make(map[string]cacheEntry),
		},
	}
}

func (c *factsCache) indexPath() string {
	return filepath.Join(c.dir, "index.json")
}

func (c *factsCache) factsDir() string {
	return filepath.Join(c.dir, "facts")
}

func (c *factsCache) factsPathForFile(filePath string) string {
	h := sha256.Sum256([]byte(filePath))
	return filepath.Join(c.factsDir(), hex.EncodeToString(h[:]) + ".json")
}

func (c *factsCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("cache mkdir: %w", err)
	}
	path := c.indexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read cache index: %w", err)
	}
	var idx cacheIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("parse cache index: %w", err)
	}
	if idx.Version != cacheIndexVersion {
		// Reset on version mismatch
		c.index = cacheIndex{Version: cacheIndexVersion, Entries: make(map[string]cacheEntry)}
		return nil
	}
	if idx.Entries == nil {
		idx.Entries = make(map[string]cacheEntry)
	}
	c.index = idx
	return nil
}

func (c *factsCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeJSONAtomic(c.indexPath(), c.index)
}

func (c *factsCache) Get(filePath, contentHash string) (extractor.FileFacts, bool, error) {
	c.mu.Lock()
	entry, ok := c.index.Entries[filePath]
	c.mu.Unlock()
	if !ok {
		return extractor.FileFacts{}, false, nil
	}
	if entry.ContentHash != contentHash {
		return extractor.FileFacts{}, false, nil
	}
	if entry.ParserVersion != c.parserVersion || entry.ExtractorVersion != c.extractorVersion {
		return extractor.FileFacts{}, false, nil
	}

	data, err := os.ReadFile(entry.FactsPath)
	if err != nil {
		return extractor.FileFacts{}, false, fmt.Errorf("read cached facts: %w", err)
	}
	var facts extractor.FileFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return extractor.FileFacts{}, false, fmt.Errorf("parse cached facts: %w", err)
	}
	return facts, true, nil
}

func (c *factsCache) Put(filePath, contentHash string, facts extractor.FileFacts) error {
	factsPath := c.factsPathForFile(filePath)
	if err := os.MkdirAll(filepath.Dir(factsPath), 0o755); err != nil {
		return fmt.Errorf("cache facts dir: %w", err)
	}
	if err := writeJSONAtomic(factsPath, facts); err != nil {
		return err
	}

	c.mu.Lock()
	c.index.Entries[filePath] = cacheEntry{
		ContentHash:      contentHash,
		FactsPath:        factsPath,
		ParserVersion:    c.parserVersion,
		ExtractorVersion: c.extractorVersion,
	}
	c.mu.Unlock()
	return nil
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache json: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("temp cache file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("close cache file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rename cache file: %w", err)
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
