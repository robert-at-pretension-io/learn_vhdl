package lsp

import (
	"sort"
	"strings"
	"sync"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// SymbolIndex is the symbol data exported by vhdl-lint --symbols-json.
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

// Summary types — each has at minimum Name, File, Line.

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

// symbolEntry is an internal entry in the SymbolStore.
type symbolEntry struct {
	name     string
	kind     string // entity, architecture, package, signal, port, type, constant, function, procedure, instance, component
	file     string
	line     int
	detail   string // type info, return type, direction, etc.
	inParent string // containing entity/package/arch name
}

// SymbolStore provides an in-memory searchable index of VHDL symbols.
type SymbolStore struct {
	mu      sync.RWMutex
	entries []symbolEntry
	byName  map[string][]int // lowercase name -> entry indices
	byFile  map[string][]int // file path -> entry indices
}

// NewSymbolStore creates an empty symbol store.
func NewSymbolStore() *SymbolStore {
	return &SymbolStore{
		byName: make(map[string][]int),
		byFile: make(map[string][]int),
	}
}

// Rebuild replaces all symbols from a fresh SymbolIndex.
func (ss *SymbolStore) Rebuild(idx *SymbolIndex) {
	if idx == nil {
		return
	}

	var entries []symbolEntry
	add := func(name, kind, file string, line int, detail, inParent string) {
		entries = append(entries, symbolEntry{
			name: name, kind: kind, file: file, line: line,
			detail: detail, inParent: inParent,
		})
	}

	for _, e := range idx.Entities {
		add(e.Name, "entity", e.File, e.Line, "", "")
		for _, p := range e.Ports {
			add(p.Name, "port", p.File, p.Line, p.Direction+" "+p.Type, e.Name)
		}
	}
	for _, a := range idx.Architectures {
		add(a.Name, "architecture", a.File, a.Line, a.EntityName, "")
	}
	for _, p := range idx.Packages {
		add(p.Name, "package", p.File, p.Line, "", "")
	}
	for _, s := range idx.Signals {
		add(s.Name, "signal", s.File, s.Line, s.Type, s.InEntity)
	}
	for _, p := range idx.Ports {
		add(p.Name, "port", p.File, p.Line, p.Direction+" "+p.Type, p.InEntity)
	}
	for _, t := range idx.Types {
		add(t.Name, "type", t.File, t.Line, t.Kind, t.InPackage)
	}
	for _, c := range idx.Constants {
		add(c.Name, "constant", c.File, c.Line, c.Type, c.InPackage)
	}
	for _, f := range idx.Functions {
		add(f.Name, "function", f.File, f.Line, f.ReturnType, f.InPackage)
	}
	for _, p := range idx.Procedures {
		add(p.Name, "procedure", p.File, p.Line, "", p.InPackage)
	}
	for _, i := range idx.Instances {
		add(i.Name, "instance", i.File, i.Line, i.Target, i.InArch)
	}
	for _, c := range idx.Components {
		add(c.Name, "component", c.File, c.Line, "", "")
	}

	byName := make(map[string][]int, len(entries))
	byFile := make(map[string][]int)
	for i, e := range entries {
		key := strings.ToLower(e.name)
		byName[key] = append(byName[key], i)
		byFile[e.file] = append(byFile[e.file], i)
	}

	ss.mu.Lock()
	ss.entries = entries
	ss.byName = byName
	ss.byFile = byFile
	ss.mu.Unlock()
}

// FindDefinition returns locations for the given symbol name.
// Prefers definitions (entity, package, type, function, procedure, component)
// over usages (signal, port, instance).
func (ss *SymbolStore) FindDefinition(name string) []protocol.Location {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	indices := ss.byName[strings.ToLower(name)]
	if len(indices) == 0 {
		return nil
	}

	// Prefer "defining" kinds
	defKinds := map[string]bool{
		"entity": true, "package": true, "type": true,
		"function": true, "procedure": true, "component": true,
		"architecture": true,
	}

	var defLocs, allLocs []protocol.Location
	for _, i := range indices {
		e := ss.entries[i]
		loc := entryToLocation(e)
		allLocs = append(allLocs, loc)
		if defKinds[e.kind] {
			defLocs = append(defLocs, loc)
		}
	}

	if len(defLocs) > 0 {
		return defLocs
	}
	return allLocs
}

// FindReferences returns all locations where the given name appears.
func (ss *SymbolStore) FindReferences(name string) []protocol.Location {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	indices := ss.byName[strings.ToLower(name)]
	locs := make([]protocol.Location, 0, len(indices))
	for _, i := range indices {
		locs = append(locs, entryToLocation(ss.entries[i]))
	}
	return locs
}

// WorkspaceSymbols returns symbols matching the query string (fuzzy substring match).
func (ss *SymbolStore) WorkspaceSymbols(query string) []protocol.SymbolInformation {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query = strings.ToLower(query)
	var results []protocol.SymbolInformation

	// Collect matching names
	for key, indices := range ss.byName {
		if query != "" && !strings.Contains(key, query) {
			continue
		}
		for _, i := range indices {
			e := ss.entries[i]
			results = append(results, protocol.SymbolInformation{
				Name: e.name,
				Kind: symbolKindToLSP(e.kind),
				Location: protocol.Location{
					URI: fileToURI(e.file, ""),
					Range: protocol.Range{
						Start: protocol.Position{Line: lineToLSP(e.line), Character: 0},
						End:   protocol.Position{Line: lineToLSP(e.line), Character: 0},
					},
				},
				ContainerName: nilIfEmpty(e.inParent),
			})
		}
	}

	// Sort: exact prefix matches first, then alphabetical
	sort.Slice(results, func(i, j int) bool {
		iPrefix := strings.HasPrefix(strings.ToLower(results[i].Name), query)
		jPrefix := strings.HasPrefix(strings.ToLower(results[j].Name), query)
		if iPrefix != jPrefix {
			return iPrefix
		}
		return results[i].Name < results[j].Name
	})

	// Limit results
	const maxResults = 200
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// LookupAt returns the symbol entry at the given file and line, or nil.
func (ss *SymbolStore) LookupAt(file string, line int) *symbolEntry {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	indices := ss.byFile[file]
	for _, i := range indices {
		if ss.entries[i].line == line {
			e := ss.entries[i]
			return &e
		}
	}
	return nil
}

// LookupByName returns all symbol entries matching the given name.
func (ss *SymbolStore) LookupByName(name string) []symbolEntry {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	indices := ss.byName[strings.ToLower(name)]
	result := make([]symbolEntry, 0, len(indices))
	for _, i := range indices {
		result = append(result, ss.entries[i])
	}
	return result
}

func entryToLocation(e symbolEntry) protocol.Location {
	return protocol.Location{
		URI: fileToURI(e.file, ""),
		Range: protocol.Range{
			Start: protocol.Position{Line: lineToLSP(e.line), Character: 0},
			End:   protocol.Position{Line: lineToLSP(e.line), Character: 0},
		},
	}
}

func lineToLSP(line int) protocol.UInteger {
	if line > 0 {
		return protocol.UInteger(line - 1)
	}
	return 0
}

func symbolKindToLSP(kind string) protocol.SymbolKind {
	switch kind {
	case "entity":
		return protocol.SymbolKindClass
	case "architecture":
		return protocol.SymbolKindModule
	case "package":
		return protocol.SymbolKindPackage
	case "signal":
		return protocol.SymbolKindVariable
	case "port":
		return protocol.SymbolKindField
	case "type":
		return protocol.SymbolKindStruct
	case "constant":
		return protocol.SymbolKindConstant
	case "function":
		return protocol.SymbolKindFunction
	case "procedure":
		return protocol.SymbolKindMethod
	case "instance":
		return protocol.SymbolKindObject
	case "component":
		return protocol.SymbolKindInterface
	default:
		return protocol.SymbolKindVariable
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
