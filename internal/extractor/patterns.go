package extractor

import (
	"regexp"
	"strings"
)

var (
	// Pattern: entity <name> is
	entityPattern = regexp.MustCompile(`(?i)^\s*entity\s+(\w+)\s+is`)

	// Pattern: architecture <name> of <entity> is
	archPattern = regexp.MustCompile(`(?i)^\s*architecture\s+(\w+)\s+of\s+(\w+)\s+is`)

	// Pattern: package <name> is
	packagePattern = regexp.MustCompile(`(?i)^\s*package\s+(\w+)\s+is`)

	// Pattern: use <library>.<package>.all
	usePattern = regexp.MustCompile(`(?i)^\s*use\s+([\w.]+)`)

	// Pattern: library <name>
	libraryPattern = regexp.MustCompile(`(?i)^\s*library\s+(\w+)`)

	// Pattern: component <name>
	componentPattern = regexp.MustCompile(`(?i)^\s*component\s+(\w+)`)

	// Pattern: signal <name> : <type>
	signalPattern = regexp.MustCompile(`(?i)^\s*signal\s+(\w+)\s*:\s*(\w+)`)

	// Pattern: <name> : entity <lib>.<entity>
	entityInstPattern = regexp.MustCompile(`(?i)^\s*(\w+)\s*:\s*entity\s+([\w.]+)`)

	// Pattern: <name> : <component>
	compInstPattern = regexp.MustCompile(`(?i)^\s*(\w+)\s*:\s*(\w+)\s*(?:generic|port)`)
)

// matchEntity returns [name] if line declares an entity
func matchEntity(line string) []string {
	if m := entityPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	return nil
}

// matchArchitecture returns [name, entity] if line declares an architecture
func matchArchitecture(line string) []string {
	if m := archPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1], m[2]}
	}
	return nil
}

// matchPackage returns [name] if line declares a package
func matchPackage(line string) []string {
	// Exclude "package body"
	if strings.Contains(strings.ToLower(line), "package body") {
		return nil
	}
	if m := packagePattern.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	return nil
}

// matchUseClause returns [target] if line is a use clause
func matchUseClause(line string) []string {
	if m := usePattern.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	return nil
}

// matchLibrary returns [name] if line is a library clause
func matchLibrary(line string) []string {
	if m := libraryPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	return nil
}

// matchComponent returns [name] if line declares a component
func matchComponent(line string) []string {
	if m := componentPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	return nil
}

// matchSignal returns [name, type] if line declares a signal
func matchSignal(line string) []string {
	if m := signalPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1], m[2]}
	}
	return nil
}

// matchEntityInstantiation returns [label, entity_ref] if line is a direct entity instantiation
func matchEntityInstantiation(line string) []string {
	if m := entityInstPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1], m[2]}
	}
	return nil
}

// matchComponentInstantiation returns [label, component] if line is a component instantiation
func matchComponentInstantiation(line string) []string {
	if m := compInstPattern.FindStringSubmatch(line); m != nil {
		return []string{m[1], m[2]}
	}
	return nil
}
