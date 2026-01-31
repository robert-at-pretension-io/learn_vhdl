package indexer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

type dependentsGraph map[string]map[string]bool

func buildDependentsGraph(
	factsByFile map[string]extractor.FileFacts,
	symbols *SymbolTable,
	fileLibs map[string]config.FileLibraryInfo,
) dependentsGraph {
	graph := make(dependentsGraph)
	for file, facts := range factsByFile {
		deps := resolveDependencies(facts, file, symbols, fileLibs)
		for _, depFile := range deps {
			if depFile == "" || depFile == file {
				continue
			}
			if graph[depFile] == nil {
				graph[depFile] = make(map[string]bool)
			}
			graph[depFile][file] = true
		}
	}
	return graph
}

func resolveDependencies(facts extractor.FileFacts, filePath string, symbols *SymbolTable, fileLibs map[string]config.FileLibraryInfo) []string {
	var deps []string
	fileLib := "work"
	if libInfo, ok := fileLibs[filePath]; ok && libInfo.LibraryName != "" {
		fileLib = strings.ToLower(libInfo.LibraryName)
	}
	for _, dep := range facts.Dependencies {
		qualName := strings.ToLower(dep.Target)
		if strings.HasPrefix(qualName, "work.") {
			qualName = fileLib + qualName[4:]
		}
		if sym, ok := symbols.Get(qualName); ok {
			deps = append(deps, sym.File)
		}
	}
	return deps
}

type impactReport struct {
	Root   string
	Levels [][]string
}

func computeImpact(root string, dependents dependentsGraph) impactReport {
	visited := map[string]bool{root: true}
	frontier := []string{root}
	var levels [][]string

	for len(frontier) > 0 {
		var next []string
		for _, f := range frontier {
			for dep := range dependents[f] {
				if visited[dep] {
					continue
				}
				visited[dep] = true
				next = append(next, dep)
			}
		}
		if len(next) == 0 {
			break
		}
		sort.Strings(next)
		levels = append(levels, next)
		frontier = next
	}

	return impactReport{Root: root, Levels: levels}
}

func formatImpactReport(report impactReport) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s\n", report.Root))
	for i, level := range report.Levels {
		b.WriteString(fmt.Sprintf("    level %d (%d): %s\n", i+1, len(level), strings.Join(level, ", ")))
	}
	return b.String()
}
