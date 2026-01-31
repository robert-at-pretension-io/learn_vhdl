package facts

import (
	"sort"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

// Tables is the relational fact model for Datalog engines.
// Each slice is a relation (table) with flat rows.
type Tables struct {
	Files          []FileRow          `json:"files"`
	Entities       []EntityRow        `json:"entities"`
	Architectures  []ArchitectureRow  `json:"architectures"`
	Packages       []PackageRow       `json:"packages"`
	Ports          []PortRow          `json:"ports"`
	Signals        []SignalRow        `json:"signals"`
	Instances      []InstanceRow      `json:"instances"`
	Dependencies   []DependencyRow    `json:"dependencies"`
	UseClauses     []UseClauseRow     `json:"use_clauses"`
	LibraryClauses []LibraryClauseRow `json:"library_clauses"`
	ContextClauses []ContextClauseRow `json:"context_clauses"`
	Processes      []ProcessRow       `json:"processes"`
	Generates      []GenerateRow      `json:"generates"`
	Types          []TypeRow          `json:"types"`
	Subtypes       []SubtypeRow       `json:"subtypes"`
	Functions      []FunctionRow      `json:"functions"`
	Procedures     []ProcedureRow     `json:"procedures"`
	Constants      []ConstantRow      `json:"constants"`
	Symbols        []SymbolRow        `json:"symbols"`
}

type FileRow struct {
	Path         string `json:"path"`
	Library      string `json:"library"`
	IsThirdParty bool   `json:"is_third_party"`
}

type EntityRow struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ArchitectureRow struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

type PackageRow struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type PortRow struct {
	Entity    string `json:"entity"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	File      string `json:"file"`
	Line      int    `json:"line"`
}

type SignalRow struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Scope string `json:"scope"`
}

type InstanceRow struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	InArch string `json:"in_arch"`
}

type DependencyRow struct {
	File   string `json:"file"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Line   int    `json:"line"`
}

type UseClauseRow struct {
	File string `json:"file"`
	Item string `json:"item"`
	Line int    `json:"line"`
}

type LibraryClauseRow struct {
	File    string `json:"file"`
	Library string `json:"library"`
	Line    int    `json:"line"`
}

type ContextClauseRow struct {
	File string `json:"file"`
	Name string `json:"name"`
	Line int    `json:"line"`
}

type ProcessRow struct {
	Label        string `json:"label"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	InArch       string `json:"in_arch"`
	IsSequential bool   `json:"is_sequential"`
	IsComb       bool   `json:"is_combinational"`
}

type GenerateRow struct {
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	InArch  string `json:"in_arch"`
	CanElab bool   `json:"can_elaborate"`
}

type TypeRow struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package"`
	InArch    string `json:"in_arch"`
}

type SubtypeRow struct {
	Name      string `json:"name"`
	BaseType  string `json:"base_type"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package"`
	InArch    string `json:"in_arch"`
}

type FunctionRow struct {
	Name       string `json:"name"`
	ReturnType string `json:"return_type"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	InPackage  string `json:"in_package"`
	InArch     string `json:"in_arch"`
	IsPure     bool   `json:"is_pure"`
	HasBody    bool   `json:"has_body"`
}

type ProcedureRow struct {
	Name      string `json:"name"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package"`
	InArch    string `json:"in_arch"`
	HasBody   bool   `json:"has_body"`
}

type ConstantRow struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package"`
	InArch    string `json:"in_arch"`
}

type SymbolRow struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// BuildTables converts extractor FileFacts into a normalized relational model.
func BuildTables(facts []extractor.FileFacts, fileLibs map[string]config.FileLibraryInfo, thirdParty map[string]bool, symbols []SymbolRow) Tables {
	tables := Tables{
		Files:          []FileRow{},
		Entities:       []EntityRow{},
		Architectures:  []ArchitectureRow{},
		Packages:       []PackageRow{},
		Ports:          []PortRow{},
		Signals:        []SignalRow{},
		Instances:      []InstanceRow{},
		Dependencies:   []DependencyRow{},
		UseClauses:     []UseClauseRow{},
		LibraryClauses: []LibraryClauseRow{},
		ContextClauses: []ContextClauseRow{},
		Processes:      []ProcessRow{},
		Generates:      []GenerateRow{},
		Types:          []TypeRow{},
		Subtypes:       []SubtypeRow{},
		Functions:      []FunctionRow{},
		Procedures:     []ProcedureRow{},
		Constants:      []ConstantRow{},
		Symbols:        []SymbolRow{},
	}

	seenFiles := make(map[string]bool)
	for _, f := range facts {
		if !seenFiles[f.File] {
			seenFiles[f.File] = true
			libName := ""
			if info, ok := fileLibs[f.File]; ok {
				libName = info.LibraryName
			}
			tables.Files = append(tables.Files, FileRow{
				Path:         f.File,
				Library:      libName,
				IsThirdParty: thirdParty[f.File],
			})
		}

		for _, e := range f.Entities {
			tables.Entities = append(tables.Entities, EntityRow{
				Name: e.Name,
				File: f.File,
				Line: e.Line,
			})
		}

		for _, a := range f.Architectures {
			tables.Architectures = append(tables.Architectures, ArchitectureRow{
				Name:       a.Name,
				EntityName: a.EntityName,
				File:       f.File,
				Line:       a.Line,
			})
		}

		for _, p := range f.Packages {
			tables.Packages = append(tables.Packages, PackageRow{
				Name: p.Name,
				File: f.File,
				Line: p.Line,
			})
		}

		for _, p := range f.Ports {
			tables.Ports = append(tables.Ports, PortRow{
				Entity:    p.InEntity,
				Name:      p.Name,
				Direction: p.Direction,
				Type:      p.Type,
				File:      f.File,
				Line:      p.Line,
			})
		}

		for _, s := range f.Signals {
			tables.Signals = append(tables.Signals, SignalRow{
				Name:  s.Name,
				Type:  s.Type,
				File:  f.File,
				Line:  s.Line,
				Scope: s.InEntity,
			})
		}

		for _, inst := range f.Instances {
			tables.Instances = append(tables.Instances, InstanceRow{
				Name:   inst.Name,
				Target: inst.Target,
				File:   f.File,
				Line:   inst.Line,
				InArch: inst.InArch,
			})
		}

		for _, dep := range f.Dependencies {
			tables.Dependencies = append(tables.Dependencies, DependencyRow{
				File:   f.File,
				Target: dep.Target,
				Kind:   dep.Kind,
				Line:   dep.Line,
			})
		}

		for _, use := range f.UseClauses {
			for _, item := range use.Items {
				tables.UseClauses = append(tables.UseClauses, UseClauseRow{
					File: f.File,
					Item: item,
					Line: use.Line,
				})
			}
		}

		for _, lib := range f.LibraryClauses {
			for _, name := range lib.Libraries {
				tables.LibraryClauses = append(tables.LibraryClauses, LibraryClauseRow{
					File:    f.File,
					Library: name,
					Line:    lib.Line,
				})
			}
		}

		for _, ctx := range f.ContextClauses {
			tables.ContextClauses = append(tables.ContextClauses, ContextClauseRow{
				File: f.File,
				Name: ctx.Name,
				Line: ctx.Line,
			})
		}

		for _, proc := range f.Processes {
			tables.Processes = append(tables.Processes, ProcessRow{
				Label:        proc.Label,
				File:         f.File,
				Line:         proc.Line,
				InArch:       proc.InArch,
				IsSequential: proc.IsSequential,
				IsComb:       proc.IsCombinational,
			})
		}

		for _, gen := range f.Generates {
			tables.Generates = append(tables.Generates, GenerateRow{
				Label:   gen.Label,
				Kind:    gen.Kind,
				File:    f.File,
				Line:    gen.Line,
				InArch:  gen.InArch,
				CanElab: gen.CanElaborate,
			})
		}

		for _, t := range f.Types {
			tables.Types = append(tables.Types, TypeRow{
				Name:      t.Name,
				Kind:      t.Kind,
				File:      f.File,
				Line:      t.Line,
				InPackage: t.InPackage,
				InArch:    t.InArch,
			})
		}

		for _, st := range f.Subtypes {
			tables.Subtypes = append(tables.Subtypes, SubtypeRow{
				Name:      st.Name,
				BaseType:  st.BaseType,
				File:      f.File,
				Line:      st.Line,
				InPackage: st.InPackage,
				InArch:    st.InArch,
			})
		}

		for _, fn := range f.Functions {
			tables.Functions = append(tables.Functions, FunctionRow{
				Name:       fn.Name,
				ReturnType: fn.ReturnType,
				File:       f.File,
				Line:       fn.Line,
				InPackage:  fn.InPackage,
				InArch:     fn.InArch,
				IsPure:     fn.IsPure,
				HasBody:    fn.HasBody,
			})
		}

		for _, pr := range f.Procedures {
			tables.Procedures = append(tables.Procedures, ProcedureRow{
				Name:      pr.Name,
				File:      f.File,
				Line:      pr.Line,
				InPackage: pr.InPackage,
				InArch:    pr.InArch,
				HasBody:   pr.HasBody,
			})
		}

		for _, c := range f.ConstantDecls {
			tables.Constants = append(tables.Constants, ConstantRow{
				Name:      c.Name,
				Type:      c.Type,
				Value:     c.Value,
				File:      f.File,
				Line:      c.Line,
				InPackage: c.InPackage,
				InArch:    c.InArch,
			})
		}
	}

	if len(symbols) > 0 {
		tables.Symbols = append(tables.Symbols, symbols...)
	}

	sort.Slice(tables.Files, func(i, j int) bool { return tables.Files[i].Path < tables.Files[j].Path })

	return tables
}
