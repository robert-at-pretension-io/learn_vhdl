package indexer

import (
	"strings"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestFormatFactsSummaryIncludesFallbacks(t *testing.T) {
	facts := extractor.FileFacts{
		File:           "a.vhd",
		ErrorNodeCount: 2,
		FallbackStats: map[string]int{
			"proc_call_prefix_fallback": 3,
			"assoc_child_walk":          1,
		},
	}

	lines := formatFactsSummary(facts)
	if len(lines) == 0 {
		t.Fatalf("expected summary lines")
	}
	if !strings.Contains(lines[0], "ERRORS=2") {
		t.Fatalf("expected parse errors in summary, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "FALLBACKS=4") {
		t.Fatalf("expected fallback total in summary, got %q", lines[0])
	}

	foundFallbackLine := false
	for _, line := range lines {
		if strings.HasPrefix(line, "fallbacks: ") {
			foundFallbackLine = true
			if !strings.Contains(line, "proc_call_prefix_fallback=3") {
				t.Fatalf("expected fallback details, got %q", line)
			}
		}
	}
	if !foundFallbackLine {
		t.Fatalf("expected fallback detail line, got %#v", lines)
	}
}

func TestFallbackSummaryOrdering(t *testing.T) {
	summary := fallbackSummary(map[string]int{
		"b": 1,
		"a": 3,
		"c": 3,
	}, 3)
	if summary != "a=3, c=3, b=1" {
		t.Fatalf("unexpected fallback summary ordering: %q", summary)
	}
}
