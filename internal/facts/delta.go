package facts

// Delta captures added and removed fact rows between two snapshots.
type Delta struct {
	Added   Tables `json:"added"`
	Removed Tables `json:"removed"`
}

// ComputeDelta computes row-level additions and removals between two snapshots.
func ComputeDelta(prev, next Tables) Delta {
	return Delta{
		Added:   diffTables(prev, next),
		Removed: diffTables(next, prev),
	}
}

func diffTables(from, to Tables) Tables {
	out := emptyTables()

	out.Files = diffFileRows(from.Files, to.Files)
	out.Entities = diffEntityRows(from.Entities, to.Entities)
	out.Architectures = diffArchitectureRows(from.Architectures, to.Architectures)
	out.Packages = diffPackageRows(from.Packages, to.Packages)
	out.Ports = diffPortRows(from.Ports, to.Ports)
	out.Signals = diffSignalRows(from.Signals, to.Signals)
	out.Instances = diffInstanceRows(from.Instances, to.Instances)
	out.Dependencies = diffDependencyRows(from.Dependencies, to.Dependencies)
	out.UseClauses = diffUseClauseRows(from.UseClauses, to.UseClauses)
	out.LibraryClauses = diffLibraryClauseRows(from.LibraryClauses, to.LibraryClauses)
	out.ContextClauses = diffContextClauseRows(from.ContextClauses, to.ContextClauses)
	out.Processes = diffProcessRows(from.Processes, to.Processes)
	out.Generates = diffGenerateRows(from.Generates, to.Generates)
	out.Types = diffTypeRows(from.Types, to.Types)
	out.Subtypes = diffSubtypeRows(from.Subtypes, to.Subtypes)
	out.Functions = diffFunctionRows(from.Functions, to.Functions)
	out.Procedures = diffProcedureRows(from.Procedures, to.Procedures)
	out.Constants = diffConstantRows(from.Constants, to.Constants)
	out.Symbols = diffSymbolRows(from.Symbols, to.Symbols)

	return out
}

func emptyTables() Tables {
	return Tables{
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
}

func diffFileRows(from, to []FileRow) []FileRow {
	return diffRows(from, to, func(r FileRow) string {
		return r.Path + "|" + r.Library + "|" + boolKey(r.IsThirdParty)
	})
}

func diffEntityRows(from, to []EntityRow) []EntityRow {
	return diffRows(from, to, func(r EntityRow) string {
		return r.Name + "|" + r.File + "|" + intKey(r.Line)
	})
}

func diffArchitectureRows(from, to []ArchitectureRow) []ArchitectureRow {
	return diffRows(from, to, func(r ArchitectureRow) string {
		return r.Name + "|" + r.EntityName + "|" + r.File + "|" + intKey(r.Line)
	})
}

func diffPackageRows(from, to []PackageRow) []PackageRow {
	return diffRows(from, to, func(r PackageRow) string {
		return r.Name + "|" + r.File + "|" + intKey(r.Line)
	})
}

func diffPortRows(from, to []PortRow) []PortRow {
	return diffRows(from, to, func(r PortRow) string {
		return r.Entity + "|" + r.Name + "|" + r.Direction + "|" + r.Type + "|" + r.File + "|" + intKey(r.Line)
	})
}

func diffSignalRows(from, to []SignalRow) []SignalRow {
	return diffRows(from, to, func(r SignalRow) string {
		return r.Name + "|" + r.Type + "|" + r.File + "|" + intKey(r.Line) + "|" + r.Scope
	})
}

func diffInstanceRows(from, to []InstanceRow) []InstanceRow {
	return diffRows(from, to, func(r InstanceRow) string {
		return r.Name + "|" + r.Target + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InArch
	})
}

func diffDependencyRows(from, to []DependencyRow) []DependencyRow {
	return diffRows(from, to, func(r DependencyRow) string {
		return r.File + "|" + r.Target + "|" + r.Kind + "|" + intKey(r.Line)
	})
}

func diffUseClauseRows(from, to []UseClauseRow) []UseClauseRow {
	return diffRows(from, to, func(r UseClauseRow) string {
		return r.File + "|" + r.Item + "|" + intKey(r.Line)
	})
}

func diffLibraryClauseRows(from, to []LibraryClauseRow) []LibraryClauseRow {
	return diffRows(from, to, func(r LibraryClauseRow) string {
		return r.File + "|" + r.Library + "|" + intKey(r.Line)
	})
}

func diffContextClauseRows(from, to []ContextClauseRow) []ContextClauseRow {
	return diffRows(from, to, func(r ContextClauseRow) string {
		return r.File + "|" + r.Name + "|" + intKey(r.Line)
	})
}

func diffProcessRows(from, to []ProcessRow) []ProcessRow {
	return diffRows(from, to, func(r ProcessRow) string {
		return r.Label + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InArch + "|" + boolKey(r.IsSequential) + "|" + boolKey(r.IsComb)
	})
}

func diffGenerateRows(from, to []GenerateRow) []GenerateRow {
	return diffRows(from, to, func(r GenerateRow) string {
		return r.Label + "|" + r.Kind + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InArch + "|" + boolKey(r.CanElab)
	})
}

func diffTypeRows(from, to []TypeRow) []TypeRow {
	return diffRows(from, to, func(r TypeRow) string {
		return r.Name + "|" + r.Kind + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InPackage + "|" + r.InArch
	})
}

func diffSubtypeRows(from, to []SubtypeRow) []SubtypeRow {
	return diffRows(from, to, func(r SubtypeRow) string {
		return r.Name + "|" + r.BaseType + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InPackage + "|" + r.InArch
	})
}

func diffFunctionRows(from, to []FunctionRow) []FunctionRow {
	return diffRows(from, to, func(r FunctionRow) string {
		return r.Name + "|" + r.ReturnType + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InPackage + "|" + r.InArch + "|" + boolKey(r.IsPure) + "|" + boolKey(r.HasBody)
	})
}

func diffProcedureRows(from, to []ProcedureRow) []ProcedureRow {
	return diffRows(from, to, func(r ProcedureRow) string {
		return r.Name + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InPackage + "|" + r.InArch + "|" + boolKey(r.HasBody)
	})
}

func diffConstantRows(from, to []ConstantRow) []ConstantRow {
	return diffRows(from, to, func(r ConstantRow) string {
		return r.Name + "|" + r.Type + "|" + r.Value + "|" + r.File + "|" + intKey(r.Line) + "|" + r.InPackage + "|" + r.InArch
	})
}

func diffSymbolRows(from, to []SymbolRow) []SymbolRow {
	return diffRows(from, to, func(r SymbolRow) string {
		return r.Name + "|" + r.Kind + "|" + r.File + "|" + intKey(r.Line)
	})
}

func diffRows[T any](from, to []T, key func(T) string) []T {
	fromSet := make(map[string]T, len(from))
	for _, row := range from {
		fromSet[key(row)] = row
	}
	var diff []T
	for _, row := range to {
		rowKey := key(row)
		if _, ok := fromSet[rowKey]; !ok {
			diff = append(diff, row)
		}
	}
	if diff == nil {
		diff = []T{}
	}
	return diff
}

func boolKey(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func intKey(v int) string {
	if v == 0 {
		return "0"
	}
	return itoa(v)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
