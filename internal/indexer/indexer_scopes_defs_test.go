package indexer

import (
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

func TestBuildPolicyInputScopesDefsUses(t *testing.T) {
	idx := New()
	idx.Config = config.DefaultConfig()
	idx.FileLibraries = map[string]config.FileLibraryInfo{
		"a.vhd": {LibraryName: "work"},
	}
	idx.ThirdPartyFiles = map[string]bool{}

	facts := extractor.FileFacts{
		File: "a.vhd",
		Entities: []extractor.Entity{{
			Name: "ent",
			Line: 1,
		}},
		Architectures: []extractor.Architecture{{
			Name:       "rtl",
			EntityName: "ent",
			Line:       5,
		}},
		Packages: []extractor.Package{{
			Name: "pkg",
			Line: 2,
		}},
		Signals: []extractor.Signal{{
			Name:     "sig",
			Type:     "integer",
			Line:     10,
			InEntity: "rtl",
		}},
		Functions: []extractor.FunctionDeclaration{{
			Name:      "fn",
			InPackage: "pkg",
			Line:      3,
		}},
		Procedures: []extractor.ProcedureDeclaration{{
			Name:      "proc",
			InPackage: "pkg",
			Line:      4,
		}},
		Processes: []extractor.Process{{
			Label:  "p1",
			InArch: "rtl",
			Line:   20,
			FunctionCalls: []extractor.FunctionCall{{
				Name: "pkg.fn",
				Line: 21,
			}},
			ProcedureCalls: []extractor.ProcedureCall{{
				Name:     "proc",
				FullName: "pkg.proc",
				Line:     22,
			}},
		}},
	}

	idx.Facts = []extractor.FileFacts{facts}
	input := idx.buildPolicyInput()

	if len(input.Scopes) == 0 {
		t.Fatalf("expected scopes to be populated")
	}

	if !hasScopeKind(input.Scopes, "package") {
		t.Fatalf("expected package scope in input.Scopes")
	}

	if !hasSymbolDef(input.SymbolDefs, "fn", "function") {
		t.Fatalf("expected function symbol def")
	}

	if !hasNameUse(input.NameUses, "pkg.fn", "function_call") {
		t.Fatalf("expected function_call name use")
	}
}

func hasScopeKind(scopes []policy.Scope, kind string) bool {
	for _, scope := range scopes {
		if scope.Kind == kind {
			return true
		}
	}
	return false
}

func hasSymbolDef(defs []policy.SymbolDef, name, kind string) bool {
	for _, def := range defs {
		if def.Name == name && def.Kind == kind {
			return true
		}
	}
	return false
}

func hasNameUse(uses []policy.NameUse, name, kind string) bool {
	for _, use := range uses {
		if use.Name == name && use.Kind == kind {
			return true
		}
	}
	return false
}
