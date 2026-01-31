package validator

import (
	"path/filepath"
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func TestVerificationTagSchemaValidation(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	fixture := filepath.Join(repoRoot, "testdata", "verification", "tag_schema_fixture.vhd")

	ex := extractor.New()
	facts, err := ex.Extract(fixture)
	if err != nil {
		t.Fatalf("extract fixture: %v", err)
	}
	if len(facts.VerificationTags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(facts.VerificationTags))
	}

	v, err := New()
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}

	type cueTag struct {
		ID       string            `json:"id"`
		Scope    string            `json:"scope"`
		Bindings map[string]string `json:"bindings,omitempty"`
		File     string            `json:"file"`
		Line     int               `json:"line"`
		Raw      string            `json:"raw"`
		InArch   string            `json:"in_arch"`
	}
	toCue := func(tag extractor.VerificationTag) cueTag {
		return cueTag{
			ID:       tag.ID,
			Scope:    tag.Scope,
			Bindings: tag.Bindings,
			File:     fixture,
			Line:     tag.Line,
			Raw:      tag.Raw,
			InArch:   tag.InArch,
		}
	}

	// First tag should validate (arch scope)
	if err := v.ValidateVerificationTag(toCue(facts.VerificationTags[0])); err != nil {
		t.Fatalf("expected valid tag, got error: %v", err)
	}

	// Second tag should fail (invalid scope pattern)
	if err := v.ValidateVerificationTag(toCue(facts.VerificationTags[1])); err == nil {
		t.Fatalf("expected invalid tag to fail schema validation")
	}
}
