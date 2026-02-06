package lsp

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerSingleTrigger(t *testing.T) {
	d := &Debouncer{delay: 50 * time.Millisecond}
	var called int32
	d.Trigger(func() { atomic.AddInt32(&called, 1) })
	time.Sleep(100 * time.Millisecond)
	if c := atomic.LoadInt32(&called); c != 1 {
		t.Fatalf("expected 1 call, got %d", c)
	}
}

func TestDebouncerCoalesces(t *testing.T) {
	d := &Debouncer{delay: 50 * time.Millisecond}
	var called int32
	for i := 0; i < 5; i++ {
		d.Trigger(func() { atomic.AddInt32(&called, 1) })
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if c := atomic.LoadInt32(&called); c != 1 {
		t.Fatalf("expected 1 coalesced call, got %d", c)
	}
}

func TestDebouncerStop(t *testing.T) {
	d := &Debouncer{delay: 50 * time.Millisecond}
	var called int32
	d.Trigger(func() { atomic.AddInt32(&called, 1) })
	d.Stop()
	time.Sleep(100 * time.Millisecond)
	if c := atomic.LoadInt32(&called); c != 0 {
		t.Fatalf("expected 0 calls after Stop, got %d", c)
	}
}
