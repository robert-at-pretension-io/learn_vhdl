package extractor

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// grammarFieldNames are all field('name', ...) names defined in grammar.js.
// Update this list whenever the grammar adds or removes field() annotations.
// Run: grep -oP "field\('([^']+)'" tree-sitter-vhdl/grammar.js | sed "s/field('//;s/'//" | sort -u
var grammarFieldNames = map[string]bool{
	"actual":       true,
	"aliased_name": true,
	"alternative":  true,
	"architecture": true,
	"arguments":    true,
	"attribute":    true,
	"base":         true,
	"class":        true,
	"component":    true,
	"condition":    true,
	"configuration": true,
	"consequence":  true,
	"content":      true,
	"default":      true,
	"definition":   true,
	"direction":    true,
	"end_label":    true,
	"entity":       true,
	"event":        true,
	"exponent":     true,
	"expression":   true,
	"formal":       true,
	"generic_map":  true,
	"high":         true,
	"index":        true,
	"indication":   true,
	"label":        true,
	"left":         true,
	"library":      true,
	"loop_var":     true,
	"low":          true,
	"name":         true,
	"names":        true,
	"operator":     true,
	"ports":        true,
	"prefix":       true,
	"range":        true,
	"resolution":   true,
	"return_type":  true,
	"right":        true,
	"sensitivity":  true,
	"suffix":       true,
	"target":       true,
	"time":         true,
	"type":         true,
	"value":        true,
}

// extractorFieldNames are all field names accessed via ChildByFieldName() in extractor.go.
// Update this list whenever the extractor adds new ChildByFieldName() calls.
// Run: grep -oP 'ChildByFieldName\("([^"]+)"\)' internal/extractor/extractor.go | sed 's/ChildByFieldName("//;s/")//' | sort -u
var extractorFieldNames = []string{
	"actual",
	"architecture",
	"arguments",
	"attribute",
	"base",
	"class",
	"component",
	"condition",
	"configuration",
	"content",
	"default",
	"definition",
	"direction",
	"entity",
	"exponent",
	"expression",
	"formal",
	"high",
	"index",
	"indication",
	"label",
	"left",
	"library",
	"loop_var",
	"low",
	"name",
	"names",
	"operator",
	"ports",
	"prefix",
	"range",
	"resolution",
	"return_type",
	"right",
	"sensitivity",
	"suffix",
	"target",
	"time",
	"type",
}

// TestGrammarExtractorContract validates that:
// 1. Every field name the extractor uses is defined in the grammar
// 2. The static lists stay in sync with the actual source files
func TestGrammarExtractorContract(t *testing.T) {
	t.Run("extractor_fields_exist_in_grammar", func(t *testing.T) {
		for _, field := range extractorFieldNames {
			if !grammarFieldNames[field] {
				t.Errorf("extractor uses field %q via ChildByFieldName() but grammar does not define field('%s', ...)", field, field)
			}
		}
	})

	t.Run("grammar_fields_match_source", func(t *testing.T) {
		grammarSrc, err := os.ReadFile("../../tree-sitter-vhdl/grammar.js")
		if err != nil {
			t.Skipf("cannot read grammar.js: %v (run from project root)", err)
		}
		re := regexp.MustCompile(`field\('([^']+)'`)
		matches := re.FindAllStringSubmatch(string(grammarSrc), -1)
		liveFields := make(map[string]bool)
		for _, m := range matches {
			liveFields[m[1]] = true
		}

		// Check for fields in grammar not in our static list
		for field := range liveFields {
			if !grammarFieldNames[field] {
				t.Errorf("grammar defines field('%s') but grammarFieldNames map is missing it — add it to contract_test.go", field)
			}
		}
		// Check for fields in our static list not in grammar (stale entries)
		for field := range grammarFieldNames {
			if !liveFields[field] {
				t.Errorf("grammarFieldNames has %q but grammar.js no longer defines it — remove from contract_test.go", field)
			}
		}
	})

	t.Run("extractor_fields_match_source", func(t *testing.T) {
		extractorSrc, err := os.ReadFile("extractor.go")
		if err != nil {
			t.Skipf("cannot read extractor.go: %v", err)
		}
		re := regexp.MustCompile(`ChildByFieldName\("([^"]+)"\)`)
		matches := re.FindAllStringSubmatch(string(extractorSrc), -1)
		liveFields := make(map[string]bool)
		for _, m := range matches {
			liveFields[m[1]] = true
		}

		listed := make(map[string]bool)
		for _, f := range extractorFieldNames {
			listed[f] = true
		}

		for field := range liveFields {
			if !listed[field] {
				t.Errorf("extractor.go uses ChildByFieldName(%q) but extractorFieldNames is missing it — add to contract_test.go", field)
			}
		}
		for field := range listed {
			if !liveFields[field] {
				t.Errorf("extractorFieldNames has %q but extractor.go no longer uses it — remove from contract_test.go", field)
			}
		}
	})

	t.Run("no_hidden_node_type_references", func(t *testing.T) {
		// Hidden rules (prefixed with _) in tree-sitter don't produce named nodes.
		// The extractor should not try to match them via Type() == "_something".
		extractorSrc, err := os.ReadFile("extractor.go")
		if err != nil {
			t.Skipf("cannot read extractor.go: %v", err)
		}
		content := string(extractorSrc)

		// Check for Type() == "_..." patterns (hidden node references)
		re := regexp.MustCompile(`\.Type\(\)\s*==\s*"(_[^"]+)"`)
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			// Hidden rules produce anonymous nodes in tree-sitter, so Type() comparisons
			// against them will never match. Flag these as likely bugs.
			if !strings.HasPrefix(m[1], "_kw_") { // _kw_ are keyword rules, those DO produce nodes
				t.Logf("WARNING: extractor references hidden node type %q — tree-sitter hidden rules don't produce named nodes", m[1])
			}
		}
	})
}
