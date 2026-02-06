package indexer

import (
	"testing"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

func TestBuildSymbolIndex(t *testing.T) {
	input := &policy.Input{
		Entities: []policy.Entity{
			{
				Name: "uart_tx",
				File: "/src/uart_tx.vhd",
				Line: 5,
				Ports: []policy.Port{
					{Name: "clk", Direction: "in", Type: "std_logic", File: "/src/uart_tx.vhd", Line: 6, InEntity: "uart_tx"},
					{Name: "data_out", Direction: "out", Type: "std_logic_vector(7 downto 0)", File: "/src/uart_tx.vhd", Line: 7, InEntity: "uart_tx"},
				},
			},
		},
		Architectures: []policy.Architecture{
			{Name: "rtl", EntityName: "uart_tx", File: "/src/uart_tx.vhd", Line: 20},
		},
		Packages: []policy.Package{
			{Name: "uart_pkg", File: "/src/uart_pkg.vhd", Line: 1},
		},
		Signals: []policy.Signal{
			{Name: "state", Type: "state_t", File: "/src/uart_tx.vhd", Line: 25, InEntity: "uart_tx"},
		},
		Ports: []policy.Port{
			{Name: "clk", Direction: "in", Type: "std_logic", File: "/src/uart_tx.vhd", Line: 6, InEntity: "uart_tx"},
		},
		Types: []policy.TypeDeclaration{
			{Name: "state_t", Kind: "enum", File: "/src/uart_pkg.vhd", Line: 10, InPackage: "uart_pkg"},
		},
		ConstantDecls: []policy.ConstantDeclaration{
			{Name: "BAUD_DIV", Type: "integer", File: "/src/uart_pkg.vhd", Line: 12, InPackage: "uart_pkg"},
		},
		Functions: []policy.FunctionDeclaration{
			{Name: "to_baud", ReturnType: "integer", File: "/src/uart_pkg.vhd", Line: 15, InPackage: "uart_pkg"},
		},
		Procedures: []policy.ProcedureDeclaration{
			{Name: "send_byte", File: "/src/uart_pkg.vhd", Line: 20, InPackage: "uart_pkg"},
		},
		Instances: []policy.Instance{
			{Name: "u_tx", Target: "work.uart_tx", File: "/src/top.vhd", Line: 30, InArch: "rtl"},
		},
		Components: []policy.Component{
			{Name: "uart_tx", File: "/src/top.vhd", Line: 10},
		},
	}

	idx := buildSymbolIndex(input)

	if len(idx.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(idx.Entities))
	}
	if idx.Entities[0].Name != "uart_tx" {
		t.Errorf("expected entity uart_tx, got %s", idx.Entities[0].Name)
	}
	if len(idx.Entities[0].Ports) != 2 {
		t.Errorf("expected 2 ports on entity, got %d", len(idx.Entities[0].Ports))
	}

	if len(idx.Architectures) != 1 {
		t.Fatalf("expected 1 architecture, got %d", len(idx.Architectures))
	}
	if idx.Architectures[0].EntityName != "uart_tx" {
		t.Errorf("expected entity_name uart_tx, got %s", idx.Architectures[0].EntityName)
	}

	if len(idx.Packages) != 1 {
		t.Fatalf("expected 1 package, got %d", len(idx.Packages))
	}

	if len(idx.Signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(idx.Signals))
	}
	if idx.Signals[0].Type != "state_t" {
		t.Errorf("expected signal type state_t, got %s", idx.Signals[0].Type)
	}

	if len(idx.Types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(idx.Types))
	}
	if idx.Types[0].InPackage != "uart_pkg" {
		t.Errorf("expected type in_package uart_pkg, got %s", idx.Types[0].InPackage)
	}

	if len(idx.Constants) != 1 {
		t.Fatalf("expected 1 constant, got %d", len(idx.Constants))
	}

	if len(idx.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(idx.Functions))
	}
	if idx.Functions[0].ReturnType != "integer" {
		t.Errorf("expected return type integer, got %s", idx.Functions[0].ReturnType)
	}

	if len(idx.Procedures) != 1 {
		t.Fatalf("expected 1 procedure, got %d", len(idx.Procedures))
	}

	if len(idx.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(idx.Instances))
	}
	if idx.Instances[0].Target != "work.uart_tx" {
		t.Errorf("expected instance target work.uart_tx, got %s", idx.Instances[0].Target)
	}

	if len(idx.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(idx.Components))
	}
}

func TestBuildSymbolIndexEmpty(t *testing.T) {
	input := &policy.Input{}
	idx := buildSymbolIndex(input)

	if len(idx.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(idx.Entities))
	}
	if idx.Entities == nil {
		t.Error("expected empty slice, not nil")
	}
}
