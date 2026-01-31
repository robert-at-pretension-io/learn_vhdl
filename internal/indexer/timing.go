package indexer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type timingEvent struct {
	Phase      string  `json:"phase"`
	Kind       string  `json:"kind"`
	File       string  `json:"file,omitempty"`
	Status     string  `json:"status,omitempty"`
	StartMS    float64 `json:"start_ms"`
	DurationMS float64 `json:"duration_ms"`
	EndMS      float64 `json:"end_ms"`
}

type timingRecorder struct {
	enabled bool
	start   time.Time
	mu      sync.Mutex
	events  []timingEvent
	file    *os.File
	enc     *json.Encoder
	err     error
}

func newTimingRecorder(start time.Time, path string) *timingRecorder {
	tr := &timingRecorder{start: start}
	if path == "" {
		return tr
	}
	f, err := os.Create(path)
	if err != nil {
		tr.err = err
		return tr
	}
	tr.enabled = true
	tr.file = f
	tr.enc = json.NewEncoder(f)
	return tr
}

func (tr *timingRecorder) Enabled() bool {
	return tr != nil && tr.enabled
}

func (tr *timingRecorder) Err() error {
	if tr == nil {
		return nil
	}
	return tr.err
}

func (tr *timingRecorder) Close() {
	if tr == nil || tr.file == nil {
		return
	}
	_ = tr.file.Close()
}

func (tr *timingRecorder) record(phase, kind, file, status string, start time.Time, duration time.Duration) {
	if tr == nil || !tr.enabled {
		return
	}
	startMS := durationToMS(start.Sub(tr.start))
	durationMS := durationToMS(duration)
	event := timingEvent{
		Phase:      phase,
		Kind:       kind,
		File:       file,
		Status:     status,
		StartMS:    startMS,
		DurationMS: durationMS,
		EndMS:      startMS + durationMS,
	}
	tr.mu.Lock()
	tr.events = append(tr.events, event)
	if tr.enc != nil {
		_ = tr.enc.Encode(event)
	}
	tr.mu.Unlock()
}

func (tr *timingRecorder) RecordStage(phase string, start time.Time, duration time.Duration, status string) {
	tr.record(phase, "stage", "", status, start, duration)
}

func (tr *timingRecorder) RecordFile(phase, file, status string, start time.Time, duration time.Duration) {
	tr.record(phase, "file", file, status, start, duration)
}

func durationToMS(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1_000_000.0
}

func (idx *Indexer) resolveTimingPath(rootPath string) string {
	if idx == nil {
		return ""
	}
	if envPath := os.Getenv("VHDL_TIMING_JSONL"); envPath != "" {
		return envPath
	}
	if idx.Timing {
		if idx.TimingPath != "" {
			return idx.TimingPath
		}
		if rootPath == "" {
			return "timing.jsonl"
		}
		return filepath.Join(rootPath, "timing.jsonl")
	}
	if envBool("VHDL_TIMING") {
		if rootPath == "" {
			return "timing.jsonl"
		}
		return filepath.Join(rootPath, "timing.jsonl")
	}
	return ""
}
