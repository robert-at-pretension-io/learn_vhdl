package facts

// FilterTablesByFiles returns a new Tables object containing only rows whose file
// or path is present in the provided file set.
func FilterTablesByFiles(tables Tables, files map[string]bool) Tables {
	if len(files) == 0 {
		return emptyTables()
	}
	out := emptyTables()

	for _, row := range tables.Files {
		if files[row.Path] {
			out.Files = append(out.Files, row)
		}
	}
	for _, row := range tables.Entities {
		if files[row.File] {
			out.Entities = append(out.Entities, row)
		}
	}
	for _, row := range tables.Architectures {
		if files[row.File] {
			out.Architectures = append(out.Architectures, row)
		}
	}
	for _, row := range tables.Packages {
		if files[row.File] {
			out.Packages = append(out.Packages, row)
		}
	}
	for _, row := range tables.Ports {
		if files[row.File] {
			out.Ports = append(out.Ports, row)
		}
	}
	for _, row := range tables.Signals {
		if files[row.File] {
			out.Signals = append(out.Signals, row)
		}
	}
	for _, row := range tables.Instances {
		if files[row.File] {
			out.Instances = append(out.Instances, row)
		}
	}
	for _, row := range tables.Dependencies {
		if files[row.File] {
			out.Dependencies = append(out.Dependencies, row)
		}
	}
	for _, row := range tables.UseClauses {
		if files[row.File] {
			out.UseClauses = append(out.UseClauses, row)
		}
	}
	for _, row := range tables.LibraryClauses {
		if files[row.File] {
			out.LibraryClauses = append(out.LibraryClauses, row)
		}
	}
	for _, row := range tables.ContextClauses {
		if files[row.File] {
			out.ContextClauses = append(out.ContextClauses, row)
		}
	}
	for _, row := range tables.Processes {
		if files[row.File] {
			out.Processes = append(out.Processes, row)
		}
	}
	for _, row := range tables.Generates {
		if files[row.File] {
			out.Generates = append(out.Generates, row)
		}
	}
	for _, row := range tables.Types {
		if files[row.File] {
			out.Types = append(out.Types, row)
		}
	}
	for _, row := range tables.Subtypes {
		if files[row.File] {
			out.Subtypes = append(out.Subtypes, row)
		}
	}
	for _, row := range tables.Functions {
		if files[row.File] {
			out.Functions = append(out.Functions, row)
		}
	}
	for _, row := range tables.Procedures {
		if files[row.File] {
			out.Procedures = append(out.Procedures, row)
		}
	}
	for _, row := range tables.Constants {
		if files[row.File] {
			out.Constants = append(out.Constants, row)
		}
	}
	for _, row := range tables.Symbols {
		if files[row.File] {
			out.Symbols = append(out.Symbols, row)
		}
	}

	return out
}

// FilterDeltaByFiles returns a new Delta containing only rows for the specified files.
func FilterDeltaByFiles(delta Delta, files map[string]bool) Delta {
	if len(files) == 0 {
		return Delta{
			Added:   emptyTables(),
			Removed: emptyTables(),
		}
	}
	return Delta{
		Added:   FilterTablesByFiles(delta.Added, files),
		Removed: FilterTablesByFiles(delta.Removed, files),
	}
}
