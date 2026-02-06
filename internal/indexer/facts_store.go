package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

type factsStore struct {
	dir   string
	temp  bool
	mu    sync.Mutex
	files map[string]string
}

func newFactsStore(cacheDir string) (*factsStore, error) {
	var dir string
	temp := false
	if cacheDir == "" {
		tmp, err := os.MkdirTemp("", "vhdl-facts-")
		if err != nil {
			return nil, fmt.Errorf("facts temp dir: %w", err)
		}
		dir = tmp
		temp = true
	} else {
		dir = filepath.Join(cacheDir, "stream_facts")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("facts dir: %w", err)
		}
	}
	return &factsStore{
		dir:   dir,
		temp:  temp,
		files: make(map[string]string),
	}, nil
}

func (s *factsStore) Close() error {
	if s == nil || !s.temp {
		return nil
	}
	return os.RemoveAll(s.dir)
}

func (s *factsStore) Put(file string, facts extractor.FileFacts) error {
	path := s.pathForFile(file)
	if err := writeJSONAtomic(path, facts); err != nil {
		return err
	}
	s.mu.Lock()
	s.files[file] = path
	s.mu.Unlock()
	return nil
}

func (s *factsStore) Get(file string) (extractor.FileFacts, error) {
	s.mu.Lock()
	path, ok := s.files[file]
	s.mu.Unlock()
	if !ok {
		return extractor.FileFacts{}, fmt.Errorf("facts not found for %s", file)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return extractor.FileFacts{}, fmt.Errorf("read facts: %w", err)
	}
	var facts extractor.FileFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return extractor.FileFacts{}, fmt.Errorf("parse facts: %w", err)
	}
	return facts, nil
}

func (s *factsStore) Files() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	files := make([]string, 0, len(s.files))
	for file := range s.files {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func (s *factsStore) pathForFile(file string) string {
	sum := sha256.Sum256([]byte(file))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}
