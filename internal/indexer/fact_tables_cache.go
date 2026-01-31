package indexer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
)

const factTablesCacheVersion = 1

type factTablesCache struct {
	Version int          `json:"version"`
	Tables  facts.Tables `json:"tables"`
}

func loadFactTablesCache(dir string) (facts.Tables, bool, error) {
	path := filepath.Join(dir, "fact_tables.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return facts.Tables{}, false, nil
		}
		return facts.Tables{}, false, fmt.Errorf("read fact tables cache: %w", err)
	}
	var cache factTablesCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return facts.Tables{}, false, fmt.Errorf("parse fact tables cache: %w", err)
	}
	if cache.Version != factTablesCacheVersion {
		return facts.Tables{}, false, nil
	}
	return cache.Tables, true, nil
}

func saveFactTablesCache(dir string, tables facts.Tables) error {
	cache := factTablesCache{
		Version: factTablesCacheVersion,
		Tables:  tables,
	}
	if err := writeJSONAtomic(filepath.Join(dir, "fact_tables.json"), cache); err != nil {
		return fmt.Errorf("write fact tables cache: %w", err)
	}
	return nil
}
