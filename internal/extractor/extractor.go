package extractor

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tree_sitter_vhdl "github.com/tree-sitter/tree-sitter-vhdl"
)

// Extractor uses Tree-sitter to parse VHDL files and extract facts
type Extractor struct {
	lang *sitter.Language
}

// FileFacts contains all extracted information from a single VHDL file
type FileFacts struct {
	File          string
	Entities      []Entity
	Architectures []Architecture
	Packages      []Package
	Components    []Component
	Dependencies  []Dependency
	Signals       []Signal
	Ports         []Port
	Processes     []Process
}

// Process represents a VHDL process statement
type Process struct {
	Label           string   // Optional label
	SensitivityList []string // Signals in sensitivity list (or "all" for VHDL-2008)
	Line            int
	InArch          string // Which architecture this process belongs to
}

// Entity represents a VHDL entity declaration
type Entity struct {
	Name  string
	Line  int
	Ports []Port
}

// Architecture represents a VHDL architecture body
type Architecture struct {
	Name       string
	EntityName string
	Line       int
}

// Package represents a VHDL package declaration
type Package struct {
	Name string
	Line int
}

// Component represents a component declaration or instantiation
type Component struct {
	Name       string
	EntityRef  string // The entity it references
	Line       int
	IsInstance bool
}

// Dependency represents a use/library clause or instantiation
type Dependency struct {
	Source string // The file/entity that has the dependency
	Target string // What it depends on (e.g., "work.my_pkg")
	Kind   string // "use", "library", "instantiation"
	Line   int
}

// Signal represents a signal declaration
type Signal struct {
	Name     string
	Type     string
	Line     int
	InEntity string // Which entity/arch it belongs to
}

// Port represents an entity port
type Port struct {
	Name      string
	Direction string // in, out, inout, buffer
	Type      string
	Line      int
	InEntity  string // Which entity this port belongs to
}

// New creates a new Extractor with VHDL language loaded
func New() *Extractor {
	// Load the VHDL language (thread-safe, can be shared)
	lang := sitter.NewLanguage(tree_sitter_vhdl.Language())

	return &Extractor{
		lang: lang,
	}
}

// Extract parses a VHDL file and extracts facts
// Creates a new parser per call for thread safety
func (e *Extractor) Extract(filePath string) (FileFacts, error) {
	facts := FileFacts{File: filePath}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return facts, fmt.Errorf("reading file: %w", err)
	}

	// If no language set, use simple regex-based extraction as fallback
	if e.lang == nil {
		return e.extractSimple(filePath, content)
	}

	// Create a new parser for this extraction (thread-safe)
	parser := sitter.NewParser()
	parser.SetLanguage(e.lang)

	// Parse with Tree-sitter
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return facts, fmt.Errorf("parsing: %w", err)
	}
	defer tree.Close()

	// Walk the tree and extract facts
	e.walkTree(tree.RootNode(), content, &facts, "")

	return facts, nil
}

// walkTree traverses the syntax tree and extracts relevant nodes
func (e *Extractor) walkTree(node *sitter.Node, source []byte, facts *FileFacts, context string) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "entity_declaration":
		entity := e.extractEntity(node, source)
		facts.Entities = append(facts.Entities, entity)
		// Extract ports from entity
		e.extractPortsFromEntity(node, source, entity.Name, facts)
		context = entity.Name

	case "architecture_body":
		arch := e.extractArchitecture(node, source)
		facts.Architectures = append(facts.Architectures, arch)
		context = arch.Name

	case "package_declaration":
		pkg := e.extractPackage(node, source)
		facts.Packages = append(facts.Packages, pkg)
		context = pkg.Name

	case "use_clause":
		dep := e.extractUseClause(node, source, facts.File)
		facts.Dependencies = append(facts.Dependencies, dep)

	case "library_clause":
		dep := e.extractLibraryClause(node, source, facts.File)
		if dep.Target != "" {
			facts.Dependencies = append(facts.Dependencies, dep)
		}

	case "component_instantiation":
		comp := e.extractComponentInst(node, source)
		facts.Components = append(facts.Components, comp)
		// Add as dependency
		if comp.EntityRef != "" {
			facts.Dependencies = append(facts.Dependencies, Dependency{
				Source: facts.File,
				Target: comp.EntityRef,
				Kind:   "instantiation",
				Line:   comp.Line,
			})
		}

	case "signal_declaration":
		signals := e.extractSignals(node, source, context)
		facts.Signals = append(facts.Signals, signals...)

	case "component_declaration":
		comp := e.extractComponentDecl(node, source)
		facts.Components = append(facts.Components, comp)

	case "process_statement":
		proc := e.extractProcess(node, source, context)
		facts.Processes = append(facts.Processes, proc)
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkTree(node.Child(i), source, facts, context)
	}
}

func (e *Extractor) extractEntity(node *sitter.Node, source []byte) Entity {
	entity := Entity{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Find the name field
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		entity.Name = nameNode.Content(source)
	}

	return entity
}

func (e *Extractor) extractArchitecture(node *sitter.Node, source []byte) Architecture {
	arch := Architecture{
		Line: int(node.StartPoint().Row) + 1,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		arch.Name = nameNode.Content(source)
	}
	if entityNode := node.ChildByFieldName("entity"); entityNode != nil {
		arch.EntityName = entityNode.Content(source)
	}

	return arch
}

func (e *Extractor) extractPackage(node *sitter.Node, source []byte) Package {
	pkg := Package{
		Line: int(node.StartPoint().Row) + 1,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		pkg.Name = nameNode.Content(source)
	}

	return pkg
}

func (e *Extractor) extractUseClause(node *sitter.Node, source []byte, file string) Dependency {
	// Extract the library.package path from use clause
	target := ""
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "selector_clause" {
			target += child.Content(source)
		}
	}
	return Dependency{
		Source: file,
		Target: target,
		Kind:   "use",
		Line:   int(node.StartPoint().Row) + 1,
	}
}

func (e *Extractor) extractLibraryClause(node *sitter.Node, source []byte, file string) Dependency {
	// Extract library name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			return Dependency{
				Source: file,
				Target: child.Content(source),
				Kind:   "library",
				Line:   int(node.StartPoint().Row) + 1,
			}
		}
	}
	return Dependency{}
}

func (e *Extractor) extractComponentInst(node *sitter.Node, source []byte) Component {
	comp := Component{
		Line:       int(node.StartPoint().Row) + 1,
		IsInstance: true,
	}

	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		comp.Name = labelNode.Content(source)
	}
	if compNode := node.ChildByFieldName("component"); compNode != nil {
		comp.EntityRef = compNode.Content(source)
	}

	return comp
}

func (e *Extractor) extractComponentDecl(node *sitter.Node, source []byte) Component {
	comp := Component{
		Line:       int(node.StartPoint().Row) + 1,
		IsInstance: false,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		comp.Name = nameNode.Content(source)
	}

	return comp
}

func (e *Extractor) extractSignals(node *sitter.Node, source []byte, context string) []Signal {
	var signals []Signal
	line := int(node.StartPoint().Row) + 1

	// Find signal names and type
	var names []string
	var sigType string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			// Could be name or type - collect all
			if nameNode := node.ChildByFieldName("name"); nameNode != nil && child == nameNode {
				names = append(names, child.Content(source))
			}
		}
	}

	// If we didn't get names from field, try to find them
	if len(names) == 0 {
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			names = append(names, nameNode.Content(source))
		}
	}

	// Get type
	if typeNode := node.ChildByFieldName("type"); typeNode != nil {
		sigType = typeNode.Content(source)
	}

	for _, name := range names {
		signals = append(signals, Signal{
			Name:     name,
			Type:     sigType,
			Line:     line,
			InEntity: context,
		})
	}

	return signals
}

func (e *Extractor) extractPortsFromEntity(node *sitter.Node, source []byte, entityName string, facts *FileFacts) {
	// Walk through entity looking for parameters (ports)
	var walkForPorts func(n *sitter.Node)
	walkForPorts = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "parameter" {
			ports := e.extractPorts(n, source)
			for i := range ports {
				ports[i].InEntity = entityName
			}
			facts.Ports = append(facts.Ports, ports...)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walkForPorts(n.Child(i))
		}
	}
	walkForPorts(node)
}

// extractPorts extracts one or more ports from a parameter node
// A single parameter can declare multiple ports: a, b : in std_logic
func (e *Extractor) extractPorts(node *sitter.Node, source []byte) []Port {
	line := int(node.StartPoint().Row) + 1
	direction := ""

	// Extract direction from field (now visible in grammar as port_direction)
	if dirNode := node.ChildByFieldName("direction"); dirNode != nil {
		direction = strings.ToLower(dirNode.Content(source))
	}

	// Collect all identifiers and find where direction appears
	// Structure: [name1, name2, ...] direction type
	var identifiers []string
	directionIndex := -1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == "port_direction" {
			directionIndex = len(identifiers)
		} else if childType == "identifier" {
			identifiers = append(identifiers, child.Content(source))
		}
	}

	// Determine which identifiers are names vs type
	// Names come before direction, type comes after
	var names []string
	var portType string

	if directionIndex >= 0 && len(identifiers) > directionIndex {
		// Direction was found - names are before direction, type is after
		names = identifiers[:directionIndex]
		if directionIndex < len(identifiers) {
			portType = identifiers[directionIndex] // First identifier after direction is the type
		}
	} else if len(identifiers) >= 2 {
		// No direction - all but last are names, last is type
		names = identifiers[:len(identifiers)-1]
		portType = identifiers[len(identifiers)-1]
	} else if len(identifiers) == 1 {
		// Just one identifier, treat as name
		names = identifiers
	}

	// Create a port for each name
	var ports []Port
	for _, name := range names {
		ports = append(ports, Port{
			Name:      name,
			Direction: direction,
			Type:      portType,
			Line:      line,
		})
	}

	return ports
}

func (e *Extractor) extractProcess(node *sitter.Node, source []byte, context string) Process {
	proc := Process{
		Line:   int(node.StartPoint().Row) + 1,
		InArch: context,
	}

	// Walk through children to find label and sensitivity list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "identifier":
			// First identifier before ':' is the label
			if proc.Label == "" {
				// Check if this is followed by ':'
				if i+1 < int(node.ChildCount()) {
					next := node.Child(i + 1)
					if next.Content(source) == ":" {
						proc.Label = child.Content(source)
					}
				}
			}
		case "sensitivity_list":
			// Extract signals from sensitivity list
			proc.SensitivityList = e.extractSensitivityList(child, source)
		}
	}

	return proc
}

func (e *Extractor) extractSensitivityList(node *sitter.Node, source []byte) []string {
	var signals []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "identifier":
			signals = append(signals, child.Content(source))
		case "selected_name", "indexed_name":
			// For complex signal names like rec.field or arr(i)
			signals = append(signals, child.Content(source))
		}
	}

	// Check for VHDL-2008 "all" - represented as the raw content
	content := strings.TrimSpace(node.Content(source))
	if strings.ToLower(content) == "all" {
		return []string{"all"}
	}

	return signals
}

// extractSimple is a fallback regex-based extractor when Tree-sitter isn't available
func (e *Extractor) extractSimple(filePath string, content []byte) (FileFacts, error) {
	facts := FileFacts{File: filePath}

	// Simple line-by-line parsing for basic fact extraction
	// This is a placeholder - real implementation would use regex patterns

	text := string(content)
	lines := splitLines(text)

	for i, line := range lines {
		lineNum := i + 1

		// Very basic pattern matching
		if matches := matchEntity(line); matches != nil {
			facts.Entities = append(facts.Entities, Entity{
				Name: matches[0],
				Line: lineNum,
			})
		}

		if matches := matchArchitecture(line); matches != nil {
			facts.Architectures = append(facts.Architectures, Architecture{
				Name:       matches[0],
				EntityName: matches[1],
				Line:       lineNum,
			})
		}

		if matches := matchPackage(line); matches != nil {
			facts.Packages = append(facts.Packages, Package{
				Name: matches[0],
				Line: lineNum,
			})
		}
	}

	return facts, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
