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
	File         string
	Entities     []Entity
	Architectures []Architecture
	Packages     []Package
	Components   []Component
	Dependencies []Dependency
	Signals      []Signal
	Ports        []Port
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
			port := e.extractPort(n, source)
			port.InEntity = entityName
			facts.Ports = append(facts.Ports, port)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walkForPorts(n.Child(i))
		}
	}
	walkForPorts(node)
}

func (e *Extractor) extractPort(node *sitter.Node, source []byte) Port {
	port := Port{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Extract name, direction, type from parameter node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		content := child.Content(source)
		childType := child.Type()

		switch childType {
		case "identifier":
			if port.Name == "" {
				port.Name = content
			} else if port.Type == "" {
				port.Type = content
			}
		}

		// Check for direction keywords
		switch strings.ToLower(content) {
		case "in":
			port.Direction = "in"
		case "out":
			port.Direction = "out"
		case "inout":
			port.Direction = "inOut"
		case "buffer":
			port.Direction = "buffer"
		}
	}

	return port
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
