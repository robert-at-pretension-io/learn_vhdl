package indexer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestTimingJSONLWritten(t *testing.T) {
	dir := t.TempDir()
	file := writeVHDL(t, dir, "a.vhd", "entity a is end entity; architecture rtl of a is begin end architecture;")
	cacheDir := filepath.Join(dir, ".cache")
	cfg := defaultTestConfig([]string{file}, cacheDir, false)

	timingPath := filepath.Join(dir, "timing.jsonl")

	idx := NewWithConfig(cfg)
	idx.Timing = true
	idx.TimingPath = timingPath
	idx.JSONOutput = true
	idx.extractorFactory = func() FactsExtractor { return extractor.New() }

	runIndexerForTest(t, idx, dir)

	raw, err := os.ReadFile(timingPath)
	if err != nil {
		t.Fatalf("read timing file: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 {
		t.Fatalf("expected timing events, found none")
	}

	var foundScan bool
	var foundTotal bool
	for _, line := range lines {
		var ev timingEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("parse timing event: %v", err)
		}
		if ev.Kind == "stage" && ev.Phase == "scan" {
			foundScan = true
		}
		if ev.Kind == "stage" && ev.Phase == "total" {
			foundTotal = true
		}
	}
	if !foundScan || !foundTotal {
		t.Fatalf("expected scan and total timing events")
	}
}
