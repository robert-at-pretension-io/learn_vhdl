package lsp

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Debouncer coalesces rapid triggers into a single execution after a quiet period.
type Debouncer struct {
	delay time.Duration
	mu    sync.Mutex
	timer *time.Timer
}

// NewDebouncer creates a debouncer with the configured delay.
// Default is 500ms; override with VHDL_LSP_DEBOUNCE_MS.
func NewDebouncer() *Debouncer {
	delay := 500 * time.Millisecond
	if ms, err := strconv.Atoi(os.Getenv("VHDL_LSP_DEBOUNCE_MS")); err == nil && ms > 0 {
		delay = time.Duration(ms) * time.Millisecond
	}
	return &Debouncer{delay: delay}
}

// Trigger schedules fn to run after the debounce delay.
// If Trigger is called again before the delay expires, the previous call is cancelled.
func (d *Debouncer) Trigger(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.delay, fn)
}

// Stop cancels any pending trigger.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
