package lsp

import (
	"strings"
	"testing"
)

func TestFormatHoverEntry(t *testing.T) {
	tests := []struct {
		entry    symbolEntry
		contains []string
	}{
		{
			entry:    symbolEntry{name: "uart_tx", kind: "entity"},
			contains: []string{"entity uart_tx"},
		},
		{
			entry:    symbolEntry{name: "rtl", kind: "architecture", detail: "uart_tx"},
			contains: []string{"architecture rtl of uart_tx"},
		},
		{
			entry:    symbolEntry{name: "state", kind: "signal", detail: "state_t", inParent: "rtl"},
			contains: []string{"signal state : state_t", "Declared in rtl"},
		},
		{
			entry:    symbolEntry{name: "clk", kind: "port", detail: "in std_logic", inParent: "uart_tx"},
			contains: []string{"clk", "in", "std_logic", "Port of entity uart_tx"},
		},
		{
			entry:    symbolEntry{name: "state_t", kind: "type", detail: "enum", inParent: "uart_pkg"},
			contains: []string{"type state_t", "enum", "package uart_pkg"},
		},
		{
			entry:    symbolEntry{name: "to_baud", kind: "function", detail: "integer", inParent: "uart_pkg"},
			contains: []string{"function to_baud return integer"},
		},
		{
			entry:    symbolEntry{name: "u_tx", kind: "instance", detail: "work.uart_tx", inParent: "rtl"},
			contains: []string{"u_tx : work.uart_tx", "instance"},
		},
	}

	for _, tt := range tests {
		result := formatHoverEntry(tt.entry)
		for _, substr := range tt.contains {
			if !strings.Contains(result, substr) {
				t.Errorf("formatHoverEntry(%s/%s) missing %q in:\n%s",
					tt.entry.kind, tt.entry.name, substr, result)
			}
		}
	}
}
