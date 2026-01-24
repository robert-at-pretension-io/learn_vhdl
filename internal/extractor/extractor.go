package extractor

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
)

// Extractor uses Tree-sitter to parse VHDL files and extract facts
type Extractor struct {
	parser *sitter.Parser
	lang   *sitter.Language
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
}

// New creates a new Extractor
// Note: Tree-sitter VHDL language needs to be loaded
func New() *Extractor {
	parser := sitter.NewParser()
	// Language will be set when we have the VHDL grammar loaded
	return &Extractor{
		parser: parser,
	}
}

// SetLanguage sets the Tree-sitter language (VHDL)
func (e *Extractor) SetLanguage(lang *sitter.Language) {
	e.lang = lang
	e.parser.SetLanguage(lang)
}

// Extract parses a VHDL file and extracts facts
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

	// Parse with Tree-sitter
	tree, err := e.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return facts, fmt.Errorf("parsing: %w", err)
	}
	defer tree.Close()

	// Walk the tree and extract facts
	e.walkTree(tree.RootNode(), content, &facts)

	return facts, nil
}

// walkTree traverses the syntax tree and extracts relevant nodes
func (e *Extractor) walkTree(node *sitter.Node, source []byte, facts *FileFacts) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "entity_declaration":
		entity := e.extractEntity(node, source)
		facts.Entities = append(facts.Entities, entity)

	case "architecture_body":
		arch := e.extractArchitecture(node, source)
		facts.Architectures = append(facts.Architectures, arch)

	case "package_declaration":
		pkg := e.extractPackage(node, source)
		facts.Packages = append(facts.Packages, pkg)

	case "use_clause":
		dep := e.extractUseClause(node, source)
		facts.Dependencies = append(facts.Dependencies, dep)

	case "component_instantiation":
		comp := e.extractComponentInst(node, source)
		facts.Components = append(facts.Components, comp)
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkTree(node.Child(i), source, facts)
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

func (e *Extractor) extractUseClause(node *sitter.Node, source []byte) Dependency {
	return Dependency{
		Target: node.Content(source),
		Kind:   "use",
		Line:   int(node.StartPoint().Row) + 1,
	}
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
