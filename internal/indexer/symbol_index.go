package indexer

import "github.com/robert-at-pretension-io/vhdl-lint/internal/policy"

// SymbolIndex contains flattened symbol summaries for IDE navigation.
// Populated when Indexer.SymbolsJSON is set, included in LintResult JSON.
type SymbolIndex struct {
	Entities      []EntitySummary    `json:"entities"`
	Architectures []ArchSummary      `json:"architectures"`
	Packages      []PackageSummary   `json:"packages"`
	Signals       []SignalSummary    `json:"signals"`
	Ports         []PortSummary      `json:"ports"`
	Types         []TypeSummary      `json:"types"`
	Constants     []ConstantSummary  `json:"constants"`
	Functions     []FunctionSummary  `json:"functions"`
	Procedures    []ProcedureSummary `json:"procedures"`
	Instances     []InstanceSummary  `json:"instances"`
	Components    []ComponentSummary `json:"components"`
}

type EntitySummary struct {
	Name  string        `json:"name"`
	File  string        `json:"file"`
	Line  int           `json:"line"`
	Ports []PortSummary `json:"ports,omitempty"`
}

type ArchSummary struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

type PackageSummary struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type SignalSummary struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	InEntity string `json:"in_entity,omitempty"`
}

type PortSummary struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InEntity  string `json:"in_entity,omitempty"`
}

type TypeSummary struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package,omitempty"`
}

type ConstantSummary struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package,omitempty"`
}

type FunctionSummary struct {
	Name       string `json:"name"`
	ReturnType string `json:"return_type"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	InPackage  string `json:"in_package,omitempty"`
}

type ProcedureSummary struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package,omitempty"`
}

type InstanceSummary struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	InArch string `json:"in_arch,omitempty"`
}

type ComponentSummary struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// buildSymbolIndex populates a SymbolIndex from the policy.Input data.
func buildSymbolIndex(input *policy.Input) *SymbolIndex {
	idx := &SymbolIndex{
		Entities:      make([]EntitySummary, 0, len(input.Entities)),
		Architectures: make([]ArchSummary, 0, len(input.Architectures)),
		Packages:      make([]PackageSummary, 0, len(input.Packages)),
		Signals:       make([]SignalSummary, 0, len(input.Signals)),
		Ports:         make([]PortSummary, 0, len(input.Ports)),
		Types:         make([]TypeSummary, 0, len(input.Types)),
		Constants:     make([]ConstantSummary, 0, len(input.ConstantDecls)),
		Functions:     make([]FunctionSummary, 0, len(input.Functions)),
		Procedures:    make([]ProcedureSummary, 0, len(input.Procedures)),
		Instances:     make([]InstanceSummary, 0, len(input.Instances)),
		Components:    make([]ComponentSummary, 0, len(input.Components)),
	}

	for _, e := range input.Entities {
		es := EntitySummary{
			Name: e.Name,
			File: e.File,
			Line: e.Line,
		}
		for _, p := range e.Ports {
			es.Ports = append(es.Ports, PortSummary{
				Name:      p.Name,
				Direction: p.Direction,
				Type:      p.Type,
				File:      p.File,
				Line:      p.Line,
				InEntity:  e.Name,
			})
		}
		idx.Entities = append(idx.Entities, es)
	}

	for _, a := range input.Architectures {
		idx.Architectures = append(idx.Architectures, ArchSummary{
			Name:       a.Name,
			EntityName: a.EntityName,
			File:       a.File,
			Line:       a.Line,
		})
	}

	for _, p := range input.Packages {
		idx.Packages = append(idx.Packages, PackageSummary{
			Name: p.Name,
			File: p.File,
			Line: p.Line,
		})
	}

	for _, s := range input.Signals {
		idx.Signals = append(idx.Signals, SignalSummary{
			Name:     s.Name,
			Type:     s.Type,
			File:     s.File,
			Line:     s.Line,
			InEntity: s.InEntity,
		})
	}

	for _, p := range input.Ports {
		idx.Ports = append(idx.Ports, PortSummary{
			Name:      p.Name,
			Direction: p.Direction,
			Type:      p.Type,
			File:      p.File,
			Line:      p.Line,
			InEntity:  p.InEntity,
		})
	}

	for _, t := range input.Types {
		idx.Types = append(idx.Types, TypeSummary{
			Name:      t.Name,
			Kind:      t.Kind,
			File:      t.File,
			Line:      t.Line,
			InPackage: t.InPackage,
		})
	}

	for _, c := range input.ConstantDecls {
		idx.Constants = append(idx.Constants, ConstantSummary{
			Name:      c.Name,
			Type:      c.Type,
			File:      c.File,
			Line:      c.Line,
			InPackage: c.InPackage,
		})
	}

	for _, f := range input.Functions {
		idx.Functions = append(idx.Functions, FunctionSummary{
			Name:       f.Name,
			ReturnType: f.ReturnType,
			File:       f.File,
			Line:       f.Line,
			InPackage:  f.InPackage,
		})
	}

	for _, p := range input.Procedures {
		idx.Procedures = append(idx.Procedures, ProcedureSummary{
			Name:      p.Name,
			File:      p.File,
			Line:      p.Line,
			InPackage: p.InPackage,
		})
	}

	for _, i := range input.Instances {
		idx.Instances = append(idx.Instances, InstanceSummary{
			Name:   i.Name,
			Target: i.Target,
			File:   i.File,
			Line:   i.Line,
			InArch: i.InArch,
		})
	}

	for _, c := range input.Components {
		idx.Components = append(idx.Components, ComponentSummary{
			Name: c.Name,
			File: c.File,
			Line: c.Line,
		})
	}

	return idx
}
