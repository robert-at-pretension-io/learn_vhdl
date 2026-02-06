package indexer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
)

// policyWidth returns the width for policy input, clamping -1 (unknown) to 0.
// The Rust policy engine treats 0 as "unknown width".
func policyWidth(typeStr string) int {
	w := extractor.CalculateWidth(typeStr)
	if w < 0 {
		return 0
	}
	return w
}

func normalizeStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (idx *Indexer) newPolicyInput(files []string) policy.Input {
	sort.Strings(files)
	input := policy.Input{
		Standard:                 idx.Config.Standard,
		FileCount:                len(files),
		Entities:                 []policy.Entity{},
		Architectures:            []policy.Architecture{},
		Packages:                 []policy.Package{},
		PackageBodies:            []policy.PackageBody{},
		Components:               []policy.Component{},
		UseClauses:               []policy.UseClause{},
		LibraryClauses:           []policy.LibraryClause{},
		ContextClauses:           []policy.ContextClause{},
		ContextDeclarations:      []policy.ContextDeclaration{},
		Signals:                  []policy.Signal{},
		Ports:                    []policy.Port{},
		Dependencies:             []policy.Dependency{},
		Symbols:                  []policy.Symbol{},
		Scopes:                   []policy.Scope{},
		SymbolDefs:               []policy.SymbolDef{},
		NameUses:                 []policy.NameUse{},
		Files:                    []policy.FileInfo{},
		VerificationBlocks:       []policy.VerificationBlock{},
		VerificationTags:         []policy.VerificationTag{},
		VerificationTagErrors:    []policy.VerificationTagError{},
		VerificationWaivers:      []policy.VerificationWaiver{},
		VerificationWaiverErrors: []policy.VerificationWaiverError{},
		Instances:                []policy.Instance{},
		CaseStatements:           []policy.CaseStatement{},
		Processes:                []policy.Process{},
		ConcurrentAssignments:    []policy.ConcurrentAssignment{},
		Generates:                []policy.GenerateStatement{},
		Configurations:           []policy.Configuration{},
		ConfigurationBindings:    []policy.ConfigurationBinding{},
		Types:                    []policy.TypeDeclaration{},
		Subtypes:                 []policy.SubtypeDeclaration{},
		Functions:                []policy.FunctionDeclaration{},
		Procedures:               []policy.ProcedureDeclaration{},
		ConstantDecls:            []policy.ConstantDeclaration{},
		EnumLiterals:             []string{},
		Constants:                []string{},
		SharedVariables:          []string{},
		Comparisons:              []policy.Comparison{},
		ArithmeticOps:            []policy.ArithmeticOp{},
		SignalDeps:               []policy.SignalDep{},
		CDCCrossings:             []policy.CDCCrossing{},
		SignalUsages:             []policy.SignalUsage{},
		LintConfig: policy.LintRuleConfig{
			Rules: idx.Config.Lint.Rules,
		},
		ThirdPartyFiles: []string{},
	}

	thirdParty := make([]string, 0, len(idx.ThirdPartyFiles))
	for f := range idx.ThirdPartyFiles {
		thirdParty = append(thirdParty, f)
	}
	sort.Strings(thirdParty)
	input.ThirdPartyFiles = append(input.ThirdPartyFiles, thirdParty...)

	for _, file := range files {
		lib := "work"
		if info, ok := idx.FileLibraries[file]; ok && info.LibraryName != "" {
			lib = strings.ToLower(info.LibraryName)
		}
		input.Files = append(input.Files, policy.FileInfo{
			Path:         file,
			Library:      lib,
			IsThirdParty: idx.ThirdPartyFiles[file],
		})
	}

	return input
}

func (idx *Indexer) finalizePolicyInput(input *policy.Input, mode string) {
	if mode == "minimal" {
		return
	}
	for name, sym := range idx.Symbols.All() {
		input.Symbols = append(input.Symbols, policy.Symbol{
			Name: name,
			Kind: sym.Kind,
			File: sym.File,
			Line: sym.Line,
		})
	}
	idx.populateScopesDefsUses(input)
}

func (idx *Indexer) buildPolicyInputAndTables(
	store *factsStore,
	files []string,
) (policy.Input, facts.Tables, error) {
	mode := normalizePolicyInputMode(idx.Config.Analysis.PolicyInputMode)
	builder := facts.NewBuilder(idx.FileLibraries, idx.ThirdPartyFiles)
	tablesOnly := mode == "tables"
	input := policy.Input{}
	if !tablesOnly {
		input = idx.newPolicyInput(files)
	}

	for _, file := range files {
		fileFacts, err := store.Get(file)
		if err != nil {
			return input, facts.Tables{}, fmt.Errorf("load facts: %w", err)
		}
		if !tablesOnly {
			idx.appendFactsToPolicyInput(&input, fileFacts, mode)
		}
		builder.AppendFacts(fileFacts)
	}

	factTables := builder.Finalize(idx.buildSymbolRows())
	if tablesOnly {
		input = idx.buildPolicyInputFromTables(files, factTables)
	}
	idx.finalizePolicyInput(&input, mode)
	return input, factTables, nil
}

func (idx *Indexer) appendFactsToPolicyInput(input *policy.Input, facts extractor.FileFacts, mode string) {
	mode = normalizePolicyInputMode(mode)
	full := mode == "full"
	crossFile := mode == "cross_file"
	_ = crossFile
	isTP := idx.ThirdPartyFiles[facts.File]
	for _, e := range facts.Entities {
		ports := []policy.Port{}
		generics := []policy.GenericDecl{}
		if full || crossFile {
			for _, p := range facts.Ports {
				if p.InEntity == e.Name {
					ports = append(ports, policy.Port{
						Name:      p.Name,
						Direction: p.Direction,
						Type:      p.Type,
						Default:   p.Default,
						File:      facts.File,
						Line:      p.Line,
						InEntity:  p.InEntity,
						Width:     policyWidth(p.Type),
					})
				}
			}
		}
		for _, g := range e.Generics {
			generics = append(generics, policy.GenericDecl{
				Name:        g.Name,
				Kind:        g.Kind,
				Type:        g.Type,
				Class:       g.Class,
				Default:     g.Default,
				Line:        g.Line,
				InEntity:    g.InEntity,
				InComponent: g.InComponent,
			})
		}
		input.Entities = append(input.Entities, policy.Entity{
			Name:     e.Name,
			File:     facts.File,
			Line:     e.Line,
			Ports:    ports,
			Generics: generics,
		})
	}

	for _, a := range facts.Architectures {
		input.Architectures = append(input.Architectures, policy.Architecture{
			Name:       a.Name,
			EntityName: a.EntityName,
			File:       facts.File,
			Line:       a.Line,
		})
	}

	for _, p := range facts.Packages {
		input.Packages = append(input.Packages, policy.Package{
			Name:   p.Name,
			File:   facts.File,
			Line:   p.Line,
			InArch: p.InArch,
		})
	}
	for _, b := range facts.PackageBodies {
		input.PackageBodies = append(input.PackageBodies, policy.PackageBody{
			Name: b.Name,
			File: facts.File,
			Line: b.Line,
		})
	}

	for _, u := range facts.UseClauses {
		input.UseClauses = append(input.UseClauses, policy.UseClause{
			Items: u.Items,
			File:  facts.File,
			Line:  u.Line,
		})
	}
	for _, l := range facts.LibraryClauses {
		input.LibraryClauses = append(input.LibraryClauses, policy.LibraryClause{
			Libraries: l.Libraries,
			File:      facts.File,
			Line:      l.Line,
		})
	}
	for _, c := range facts.ContextClauses {
		input.ContextClauses = append(input.ContextClauses, policy.ContextClause{
			Name: c.Name,
			File: facts.File,
			Line: c.Line,
		})
	}
	for _, decl := range facts.ContextDecls {
		input.ContextDeclarations = append(input.ContextDeclarations, policy.ContextDeclaration{
			Name:        decl.Name,
			File:        facts.File,
			Line:        decl.Line,
			Libraries:   normalizeStringSlice(decl.Libraries),
			UseItems:    normalizeStringSlice(decl.UseItems),
			ContextRefs: normalizeStringSlice(decl.ContextRefs),
		})
	}

	if full && !isTP {
		for _, sharedName := range facts.SharedVariables {
			input.SharedVariables = append(input.SharedVariables, sharedName)
		}
	}

	for _, c := range facts.Components {
		componentPorts := []policy.Port{}
		componentGenerics := []policy.GenericDecl{}
		if full || crossFile {
			for _, p := range c.Ports {
				componentPorts = append(componentPorts, policy.Port{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Default:   p.Default,
					File:      facts.File,
					Line:      p.Line,
					InEntity:  p.InEntity,
					Width:     policyWidth(p.Type),
				})
			}
		}
		for _, g := range c.Generics {
			componentGenerics = append(componentGenerics, policy.GenericDecl{
				Name:        g.Name,
				Kind:        g.Kind,
				Type:        g.Type,
				Class:       g.Class,
				Default:     g.Default,
				Line:        g.Line,
				InEntity:    g.InEntity,
				InComponent: g.InComponent,
			})
		}
		input.Components = append(input.Components, policy.Component{
			Name:       c.Name,
			EntityRef:  c.EntityRef,
			File:       facts.File,
			Line:       c.Line,
			IsInstance: c.IsInstance,
			Ports:      componentPorts,
			Generics:   componentGenerics,
		})
	}

	if full && !isTP {
		for _, s := range facts.Signals {
			if s.Type == "" {
				continue
			}
			input.Signals = append(input.Signals, policy.Signal{
				Name:     s.Name,
				Type:     s.Type,
				File:     facts.File,
				Line:     s.Line,
				InEntity: s.InEntity,
				Width:    policyWidth(s.Type),
			})
		}

		for _, p := range facts.Ports {
			input.Ports = append(input.Ports, policy.Port{
				Name:      p.Name,
				Direction: p.Direction,
				Type:      p.Type,
				Default:   p.Default,
				File:      facts.File,
				Line:      p.Line,
				InEntity:  p.InEntity,
				Width:     policyWidth(p.Type),
			})
		}
	}

	for _, d := range facts.Dependencies {
		fileLib := "work"
		if libInfo, ok := idx.FileLibraries[facts.File]; ok && libInfo.LibraryName != "" {
			fileLib = strings.ToLower(libInfo.LibraryName)
		}

		resolved := false
		switch d.Kind {
		case "library":
			libName := strings.ToLower(strings.TrimSpace(d.Target))
			if libName != "" {
				resolved = isStandardLibraryName(libName) || libName == "work" || idx.hasLibrary(libName)
			}
		case "use":
			base := normalizeUseTarget(d.Target, fileLib)
			if base != "" {
				resolved = idx.Symbols.Has(base) || isStandardLibrary(base)
			}
		case "context":
			base := normalizeContextTarget(d.Target, fileLib)
			if base != "" {
				resolved = idx.Symbols.Has(base) || isStandardLibrary(base)
			}
		default:
			qualName := strings.ToLower(d.Target)
			if strings.HasPrefix(qualName, "work.") {
				qualName = fileLib + qualName[4:]
			}
			if !strings.Contains(qualName, ".") {
				qualName = fileLib + "." + qualName
			}

			resolved = idx.Symbols.Has(qualName) || isStandardLibrary(d.Target)
			if !resolved && d.Kind == "instantiation" {
				baseName := qualName
				if idxDot := strings.LastIndex(baseName, "."); idxDot != -1 {
					baseName = baseName[idxDot+1:]
				}
				if baseName != "" && idx.Symbols.HasSuffix(baseName) {
					resolved = true
				}
			}
		}
		input.Dependencies = append(input.Dependencies, policy.Dependency{
			Source:   d.Source,
			Target:   d.Target,
			Kind:     d.Kind,
			Line:     d.Line,
			Resolved: resolved,
		})
	}

	for _, inst := range facts.Instances {
		portMap := inst.PortMap
		if portMap == nil {
			portMap = make(map[string]string)
		}
		genericMap := inst.GenericMap
		if genericMap == nil {
			genericMap = make(map[string]string)
		}
		associations := []policy.Association{}
		for _, assoc := range inst.Associations {
			associations = append(associations, policy.Association{
				Kind:          assoc.Kind,
				Formal:        assoc.Formal,
				Actual:        assoc.Actual,
				IsPositional:  assoc.IsPositional,
				ActualKind:    assoc.ActualKind,
				ActualBase:    assoc.ActualBase,
				ActualFull:    assoc.ActualFull,
				Line:          assoc.Line,
				PositionIndex: assoc.PositionIndex,
			})
		}
		input.Instances = append(input.Instances, policy.Instance{
			Name:         inst.Name,
			Target:       inst.Target,
			TargetArch:   inst.TargetArch,
			PortMap:      portMap,
			GenericMap:   genericMap,
			Associations: associations,
			File:         facts.File,
			Line:         inst.Line,
			InArch:       inst.InArch,
		})
	}

	if full && !isTP {
		for _, cs := range facts.CaseStatements {
			choices := cs.Choices
			if choices == nil {
				choices = []string{}
			}
			input.CaseStatements = append(input.CaseStatements, policy.CaseStatement{
				Expression: cs.Expression,
				Choices:    choices,
				HasOthers:  cs.HasOthers,
				File:       facts.File,
				Line:       cs.Line,
				InProcess:  cs.InProcess,
				InArch:     cs.InArch,
				IsComplete: cs.IsComplete,
			})
		}

		for _, proc := range facts.Processes {
			sensList := proc.SensitivityList
			if sensList == nil {
				sensList = []string{}
			}
			assigned := proc.AssignedSignals
			if assigned == nil {
				assigned = []string{}
			}
			read := proc.ReadSignals
			if read == nil {
				read = []string{}
			}
			assignedVars := proc.AssignedVariables
			if assignedVars == nil {
				assignedVars = []string{}
			}
			readVars := proc.ReadVariables
			if readVars == nil {
				readVars = []string{}
			}
			vars := []policy.VariableDecl{}
			for _, v := range proc.Variables {
				vars = append(vars, policy.VariableDecl{
					Name: v.Name,
					Type: v.Type,
					Line: v.Line,
				})
			}
			procCalls := []policy.ProcedureCall{}
			for _, c := range proc.ProcedureCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				procCalls = append(procCalls, policy.ProcedureCall{
					Name:      c.Name,
					FullName:  c.FullName,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			funcCalls := []policy.FunctionCall{}
			for _, c := range proc.FunctionCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				funcCalls = append(funcCalls, policy.FunctionCall{
					Name:      c.Name,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			waitStmts := []policy.WaitStatement{}
			for _, w := range proc.WaitStatements {
				onSignals := w.OnSignals
				if onSignals == nil {
					onSignals = []string{}
				}
				waitStmts = append(waitStmts, policy.WaitStatement{
					OnSignals: onSignals,
					UntilExpr: w.UntilExpr,
					ForExpr:   w.ForExpr,
					Line:      w.Line,
				})
			}
			input.Processes = append(input.Processes, policy.Process{
				Label:             proc.Label,
				SensitivityList:   sensList,
				IsSequential:      proc.IsSequential,
				IsCombinational:   proc.IsCombinational,
				ClockSignal:       proc.ClockSignal,
				ClockEdge:         proc.ClockEdge,
				HasReset:          proc.HasReset,
				ResetSignal:       proc.ResetSignal,
				ResetAsync:        proc.ResetAsync,
				AssignedSignals:   assigned,
				ReadSignals:       read,
				AssignedVariables: assignedVars,
				ReadVariables:     readVars,
				Variables:         vars,
				ProcedureCalls:  procCalls,
				FunctionCalls:   funcCalls,
				WaitStatements:  waitStmts,
				File:            facts.File,
				Line:            proc.Line,
				InArch:          proc.InArch,
			})
		}

		for _, ca := range facts.ConcurrentAssignments {
			if ca.Target == "" {
				continue
			}
			readSigs := ca.ReadSignals
			if readSigs == nil {
				readSigs = []string{}
			}
			input.ConcurrentAssignments = append(input.ConcurrentAssignments, policy.ConcurrentAssignment{
				Target:        ca.Target,
				ReadSignals:   readSigs,
				File:          facts.File,
				Line:          ca.Line,
				InArch:        ca.InArch,
				Kind:          ca.Kind,
				InGenerate:    ca.InGenerate,
				GenerateLabel: ca.GenerateLabel,
			})
		}

		for _, comp := range facts.Comparisons {
			input.Comparisons = append(input.Comparisons, policy.Comparison{
				LeftOperand:  comp.LeftOperand,
				Operator:     comp.Operator,
				RightOperand: comp.RightOperand,
				IsLiteral:    comp.IsLiteral,
				LiteralValue: comp.LiteralValue,
				LiteralBits:  comp.LiteralBits,
				ResultDrives: comp.ResultDrives,
				File:         facts.File,
				Line:         comp.Line,
				InProcess:    comp.InProcess,
				InArch:       comp.InArch,
			})
		}

		for _, op := range facts.ArithmeticOps {
			input.ArithmeticOps = append(input.ArithmeticOps, policy.ArithmeticOp{
				Operator:    op.Operator,
				Operands:    op.Operands,
				Result:      op.Result,
				IsGuarded:   op.IsGuarded,
				GuardSignal: op.GuardSignal,
				File:        facts.File,
				Line:        op.Line,
				InProcess:   op.InProcess,
				InArch:      op.InArch,
			})
		}

		for _, dep := range facts.SignalDeps {
			input.SignalDeps = append(input.SignalDeps, policy.SignalDep{
				Source:       dep.Source,
				Target:       dep.Target,
				IsSequential: dep.IsSequential,
				File:         facts.File,
				Line:         dep.Line,
				InArch:       dep.InArch,
				InProcess:    dep.InProcess,
			})
		}

		for _, cdc := range facts.CDCCrossings {
			input.CDCCrossings = append(input.CDCCrossings, policy.CDCCrossing{
				Signal:         cdc.Signal,
				SourceClock:    cdc.SourceClock,
				DestClock:      cdc.DestClock,
				SourceProc:     cdc.SourceProc,
				DestProc:       cdc.DestProc,
				IsSynchronized: cdc.IsSynchronized,
				SyncStages:     cdc.SyncStages,
				IsMultiBit:     cdc.IsMultiBit,
				File:           cdc.File,
				Line:           cdc.Line,
				InArch:         cdc.InArch,
			})
		}

		for _, usage := range facts.SignalUsages {
			input.SignalUsages = append(input.SignalUsages, policy.SignalUsage{
				Signal:       usage.Signal,
				IsRead:       usage.IsRead,
				IsWritten:    usage.IsWritten,
				InProcess:    usage.InProcess,
				InPortMap:    usage.InPortMap,
				InstanceName: usage.InstanceName,
				InPSL:        usage.InPSL,
				Line:         usage.Line,
			})
		}
	}

	if crossFile && !isTP {
		for _, proc := range facts.Processes {
			vars := []policy.VariableDecl{}
			for _, v := range proc.Variables {
				vars = append(vars, policy.VariableDecl{
					Name: v.Name,
					Type: v.Type,
					Line: v.Line,
				})
			}
			procCalls := []policy.ProcedureCall{}
			for _, c := range proc.ProcedureCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				procCalls = append(procCalls, policy.ProcedureCall{
					Name:      c.Name,
					FullName:  c.FullName,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			funcCalls := []policy.FunctionCall{}
			for _, c := range proc.FunctionCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				funcCalls = append(funcCalls, policy.FunctionCall{
					Name:      c.Name,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			input.Processes = append(input.Processes, policy.Process{
				Label:          proc.Label,
				Variables:      vars,
				ProcedureCalls: procCalls,
				FunctionCalls:  funcCalls,
				File:           facts.File,
				Line:           proc.Line,
				InArch:         proc.InArch,
			})
		}
	}

	if full && !isTP {
		for _, gen := range facts.Generates {
			input.Generates = append(input.Generates, policy.GenerateStatement{
				Label:          gen.Label,
				Kind:           gen.Kind,
				File:           facts.File,
				Line:           gen.Line,
				InArch:         gen.InArch,
				LoopVar:        gen.LoopVar,
				RangeLow:       gen.RangeLow,
				RangeHigh:      gen.RangeHigh,
				RangeDir:       gen.RangeDir,
				IterationCount: gen.IterationCount,
				CanElaborate:   gen.CanElaborate,
				Condition:      gen.Condition,
				SignalCount:    len(gen.Signals),
				InstanceCount:  len(gen.Instances),
				ProcessCount:   len(gen.Processes),
			})
		}
	}

	if full || crossFile {
		for _, cfg := range facts.Configurations {
			input.Configurations = append(input.Configurations, policy.Configuration{
				Name:       cfg.Name,
				EntityName: cfg.EntityName,
				ArchName:   cfg.ArchName,
				File:       facts.File,
				Line:       cfg.Line,
			})
			for _, binding := range cfg.Bindings {
				input.ConfigurationBindings = append(input.ConfigurationBindings, policy.ConfigurationBinding{
					ScopePath:     binding.ScopePath,
					InstanceLabel: binding.InstanceLabel,
					ComponentName: binding.ComponentName,
					TargetEntity:  binding.TargetEntity,
					TargetArch:    binding.TargetArch,
					File:          facts.File,
					Line:          binding.Line,
				})
			}
		}
	}

	if full && !isTP {
		for _, block := range facts.VerificationBlocks {
			input.VerificationBlocks = append(input.VerificationBlocks, policy.VerificationBlock{
				Label:     block.Label,
				LineStart: block.LineStart,
				LineEnd:   block.LineEnd,
				File:      facts.File,
				InArch:    block.InArch,
			})
		}

		for _, tag := range facts.VerificationTags {
			bindings := tag.Bindings
			if bindings == nil {
				bindings = map[string]string{}
			}
			input.VerificationTags = append(input.VerificationTags, policy.VerificationTag{
				ID:       tag.ID,
				Scope:    tag.Scope,
				Bindings: bindings,
				File:     facts.File,
				Line:     tag.Line,
				Raw:      tag.Raw,
				InArch:   tag.InArch,
			})
		}

		for _, terr := range facts.VerificationTagErrors {
			input.VerificationTagErrors = append(input.VerificationTagErrors, policy.VerificationTagError{
				File:    facts.File,
				Line:    terr.Line,
				Raw:     terr.Raw,
				Message: terr.Message,
				InArch:  terr.InArch,
			})
		}

		for _, waiver := range facts.VerificationWaivers {
			input.VerificationWaivers = append(input.VerificationWaivers, policy.VerificationWaiver{
				ID:      waiver.ID,
				Scope:   waiver.Scope,
				Reason:  waiver.Reason,
				Owner:   waiver.Owner,
				Expires: waiver.Expires,
				File:    facts.File,
				Line:    waiver.Line,
				Raw:     waiver.Raw,
				InArch:  waiver.InArch,
			})
		}

		for _, werr := range facts.VerificationWaiverErrors {
			input.VerificationWaiverErrors = append(input.VerificationWaiverErrors, policy.VerificationWaiverError{
				File:    facts.File,
				Line:    werr.Line,
				Raw:     werr.Raw,
				Message: werr.Message,
				InArch:  werr.InArch,
			})
		}
	}

	if full || crossFile {
		for _, t := range facts.Types {
			enumLits := t.EnumLiterals
			if enumLits == nil {
				enumLits = []string{}
			}
			var fields []policy.RecordField
			for _, f := range t.Fields {
				fields = append(fields, policy.RecordField{
					Name: f.Name,
					Type: f.Type,
					Line: f.Line,
				})
			}
			if fields == nil {
				fields = []policy.RecordField{}
			}
			indexTypes := t.IndexTypes
			if indexTypes == nil {
				indexTypes = []string{}
			}
			input.Types = append(input.Types, policy.TypeDeclaration{
				Name:          t.Name,
				Kind:          t.Kind,
				File:          facts.File,
				Line:          t.Line,
				InPackage:     t.InPackage,
				InArch:        t.InArch,
				EnumLiterals:  enumLits,
				Fields:        fields,
				ElementType:   t.ElementType,
				IndexTypes:    indexTypes,
				Unconstrained: t.Unconstrained,
				BaseUnit:      t.BaseUnit,
				RangeLow:      t.RangeLow,
				RangeHigh:     t.RangeHigh,
				RangeDir:      t.RangeDir,
			})
		}

		for _, st := range facts.Subtypes {
			input.Subtypes = append(input.Subtypes, policy.SubtypeDeclaration{
				Name:       st.Name,
				BaseType:   st.BaseType,
				Constraint: st.Constraint,
				Resolution: st.Resolution,
				File:       facts.File,
				Line:       st.Line,
				InPackage:  st.InPackage,
				InArch:     st.InArch,
			})
		}

		for _, fn := range facts.Functions {
			var params []policy.SubprogramParameter
			for _, p := range fn.Parameters {
				params = append(params, policy.SubprogramParameter{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Class:     p.Class,
					Default:   p.Default,
					Line:      p.Line,
				})
			}
			if params == nil {
				params = []policy.SubprogramParameter{}
			}
			input.Functions = append(input.Functions, policy.FunctionDeclaration{
				Name:       fn.Name,
				ReturnType: fn.ReturnType,
				Parameters: params,
				IsPure:     fn.IsPure,
				HasBody:    fn.HasBody,
				File:       facts.File,
				Line:       fn.Line,
				InPackage:  fn.InPackage,
				InArch:     fn.InArch,
			})
		}

		for _, pr := range facts.Procedures {
			var params []policy.SubprogramParameter
			for _, p := range pr.Parameters {
				params = append(params, policy.SubprogramParameter{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Class:     p.Class,
					Default:   p.Default,
					Line:      p.Line,
				})
			}
			if params == nil {
				params = []policy.SubprogramParameter{}
			}
			input.Procedures = append(input.Procedures, policy.ProcedureDeclaration{
				Name:       pr.Name,
				Parameters: params,
				HasBody:    pr.HasBody,
				File:       facts.File,
				Line:       pr.Line,
				InPackage:  pr.InPackage,
				InArch:     pr.InArch,
			})
		}

		for _, c := range facts.ConstantDecls {
			input.ConstantDecls = append(input.ConstantDecls, policy.ConstantDeclaration{
				Name:      c.Name,
				Type:      c.Type,
				Value:     c.Value,
				File:      facts.File,
				Line:      c.Line,
				InPackage: c.InPackage,
				InArch:    c.InArch,
			})
		}
	}

	if full && !isTP {
		input.EnumLiterals = append(input.EnumLiterals, facts.EnumLiterals...)
		input.Constants = append(input.Constants, facts.Constants...)
	}
}

func (idx *Indexer) buildPolicyInputFromTables(files []string, tables facts.Tables) policy.Input {
	input := idx.newPolicyInput(files)

	for _, ent := range tables.Entities {
		input.Entities = append(input.Entities, policy.Entity{
			Name:     ent.Name,
			File:     ent.File,
			Line:     ent.Line,
			Ports:    []policy.Port{},
			Generics: []policy.GenericDecl{},
		})
	}

	for _, arch := range tables.Architectures {
		input.Architectures = append(input.Architectures, policy.Architecture{
			Name:       arch.Name,
			EntityName: arch.EntityName,
			File:       arch.File,
			Line:       arch.Line,
		})
	}

	for _, pkg := range tables.Packages {
		input.Packages = append(input.Packages, policy.Package{
			Name:   pkg.Name,
			File:   pkg.File,
			Line:   pkg.Line,
			InArch: pkg.InArch,
		})
	}

	for _, useRow := range tables.UseClauses {
		input.UseClauses = append(input.UseClauses, policy.UseClause{
			Items: []string{useRow.Item},
			File:  useRow.File,
			Line:  useRow.Line,
		})
	}

	for _, libRow := range tables.LibraryClauses {
		input.LibraryClauses = append(input.LibraryClauses, policy.LibraryClause{
			Libraries: []string{libRow.Library},
			File:      libRow.File,
			Line:      libRow.Line,
		})
	}

	for _, ctx := range tables.ContextClauses {
		input.ContextClauses = append(input.ContextClauses, policy.ContextClause{
			Name: ctx.Name,
			File: ctx.File,
			Line: ctx.Line,
		})
	}

	for _, dep := range tables.Dependencies {
		fileLib := "work"
		if libInfo, ok := idx.FileLibraries[dep.File]; ok && libInfo.LibraryName != "" {
			fileLib = strings.ToLower(libInfo.LibraryName)
		}
		resolved := resolveDependency(idx, fileLib, dep.Target, dep.Kind)
		input.Dependencies = append(input.Dependencies, policy.Dependency{
			Source:   dep.File,
			Target:   dep.Target,
			Kind:     dep.Kind,
			Line:     dep.Line,
			Resolved: resolved,
		})
	}

	for _, inst := range tables.Instances {
		input.Instances = append(input.Instances, policy.Instance{
			Name:         inst.Name,
			Target:       inst.Target,
			PortMap:      map[string]string{},
			GenericMap:   map[string]string{},
			Associations: []policy.Association{},
			File:         inst.File,
			Line:         inst.Line,
			InArch:       inst.InArch,
		})
	}

	for _, proc := range tables.Processes {
		input.Processes = append(input.Processes, policy.Process{
			Label:           proc.Label,
			IsSequential:    proc.IsSequential,
			IsCombinational: proc.IsComb,
			File:            proc.File,
			Line:            proc.Line,
			InArch:          proc.InArch,
		})
	}

	for _, gen := range tables.Generates {
		input.Generates = append(input.Generates, policy.GenerateStatement{
			Label:        gen.Label,
			Kind:         gen.Kind,
			File:         gen.File,
			Line:         gen.Line,
			InArch:       gen.InArch,
			CanElaborate: gen.CanElab,
		})
	}

	for _, ty := range tables.Types {
		input.Types = append(input.Types, policy.TypeDeclaration{
			Name:      ty.Name,
			Kind:      ty.Kind,
			File:      ty.File,
			Line:      ty.Line,
			InPackage: ty.InPackage,
			InArch:    ty.InArch,
		})
	}

	for _, st := range tables.Subtypes {
		input.Subtypes = append(input.Subtypes, policy.SubtypeDeclaration{
			Name:      st.Name,
			BaseType:  st.BaseType,
			File:      st.File,
			Line:      st.Line,
			InPackage: st.InPackage,
			InArch:    st.InArch,
		})
	}

	for _, fn := range tables.Functions {
		input.Functions = append(input.Functions, policy.FunctionDeclaration{
			Name:       fn.Name,
			ReturnType: fn.ReturnType,
			File:       fn.File,
			Line:       fn.Line,
			InPackage:  fn.InPackage,
			InArch:     fn.InArch,
			IsPure:     fn.IsPure,
			HasBody:    fn.HasBody,
		})
	}

	for _, pr := range tables.Procedures {
		input.Procedures = append(input.Procedures, policy.ProcedureDeclaration{
			Name:      pr.Name,
			File:      pr.File,
			Line:      pr.Line,
			InPackage: pr.InPackage,
			InArch:    pr.InArch,
			HasBody:   pr.HasBody,
		})
	}

	for _, c := range tables.Constants {
		input.ConstantDecls = append(input.ConstantDecls, policy.ConstantDeclaration{
			Name:      c.Name,
			Type:      c.Type,
			Value:     c.Value,
			File:      c.File,
			Line:      c.Line,
			InPackage: c.InPackage,
			InArch:    c.InArch,
		})
	}

	return input
}

func normalizePolicyInputMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "full":
		return "full"
	case "crossfile", "cross_file":
		return "cross_file"
	case "minimal", "tables", "table":
		return "tables"
	default:
		return "full"
	}
}

func resolveDependency(idx *Indexer, fileLib string, target string, kind string) bool {
	resolved := false
	switch kind {
	case "library":
		libName := strings.ToLower(strings.TrimSpace(target))
		if libName != "" {
			resolved = isStandardLibraryName(libName) || libName == "work" || idx.hasLibrary(libName)
		}
	case "use":
		base := normalizeUseTarget(target, fileLib)
		if base != "" {
			resolved = idx.Symbols.Has(base) || isStandardLibrary(base)
		}
	case "context":
		base := normalizeContextTarget(target, fileLib)
		if base != "" {
			resolved = idx.Symbols.Has(base) || isStandardLibrary(base)
		}
	default:
		qualName := strings.ToLower(target)
		if strings.HasPrefix(qualName, "work.") {
			qualName = fileLib + qualName[4:]
		}
		if !strings.Contains(qualName, ".") {
			qualName = fileLib + "." + qualName
		}
		resolved = idx.Symbols.Has(qualName) || isStandardLibrary(target)
		if !resolved && kind == "instantiation" {
			baseName := qualName
			if idxDot := strings.LastIndex(baseName, "."); idxDot != -1 {
				baseName = baseName[idxDot+1:]
			}
			if baseName != "" && idx.Symbols.HasSuffix(baseName) {
				resolved = true
			}
		}
	}
	return resolved
}

func (idx *Indexer) emitVerboseFromStore(store *factsStore, files []string) error {
	loadFacts := func(file string) (extractor.FileFacts, error) {
		return store.Get(file)
	}

	fmt.Printf("\n=== Verbose: Extracted Ports ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, p := range facts.Ports {
			fmt.Printf("  %s.%s: direction=%q type=%q\n", p.InEntity, p.Name, p.Direction, p.Type)
		}
	}

	fmt.Printf("\n=== Verbose: Extracted Processes ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, p := range facts.Processes {
			kind := "combinational"
			if p.IsSequential {
				kind = "sequential"
			}
			fmt.Printf("  %s.%s: %s, sensitivity=%v\n", p.InArch, p.Label, kind, p.SensitivityList)
			if p.ClockSignal != "" {
				fmt.Printf("    clock: %s (%s_edge)\n", p.ClockSignal, p.ClockEdge)
			}
			if p.HasReset {
				asyncStr := "sync"
				if p.ResetAsync {
					asyncStr = "async"
				}
				fmt.Printf("    reset: %s (%s)\n", p.ResetSignal, asyncStr)
			}
			if len(p.AssignedSignals) > 0 {
				fmt.Printf("    writes: %v\n", p.AssignedSignals)
			}
			if len(p.ReadSignals) > 0 {
				fmt.Printf("    reads: %v\n", p.ReadSignals)
			}
		}
	}

	fmt.Printf("\n=== Verbose: Clock Domains ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, cd := range facts.ClockDomains {
			fmt.Printf("  %s (%s): drives %v\n", cd.Clock, cd.Edge, cd.Registers)
		}
	}

	fmt.Printf("\n=== Verbose: Instances ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, inst := range facts.Instances {
			fmt.Printf("  %s: %s\n", inst.Name, inst.Target)
			if len(inst.GenericMap) > 0 {
				fmt.Printf("    generics: %v\n", inst.GenericMap)
			}
			if len(inst.PortMap) > 0 {
				fmt.Printf("    ports: %v\n", inst.PortMap)
			}
		}
	}

	fmt.Printf("\n=== Verbose: Case Statements ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, cs := range facts.CaseStatements {
			status := "INCOMPLETE (potential latch)"
			if cs.HasOthers {
				status = "complete (has others)"
			}
			fmt.Printf("  case %s [%s] line %d\n", cs.Expression, status, cs.Line)
			if cs.InProcess != "" {
				fmt.Printf("    in process: %s\n", cs.InProcess)
			}
			fmt.Printf("    choices: %v\n", cs.Choices)
		}
	}

	fmt.Printf("\n=== Verbose: Concurrent Assignments ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, ca := range facts.ConcurrentAssignments {
			fmt.Printf("  %s <= [%s] (kind: %s, line %d)\n", ca.Target, strings.Join(ca.ReadSignals, ", "), ca.Kind, ca.Line)
		}
	}

	fmt.Printf("\n=== Verbose: Comparisons (Security Analysis) ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, comp := range facts.Comparisons {
			litInfo := ""
			if comp.IsLiteral {
				litInfo = fmt.Sprintf(" [LITERAL: %s, %d bits]", comp.LiteralValue, comp.LiteralBits)
			}
			fmt.Printf("  %s %s %s%s (line %d, drives: %s)\n", comp.LeftOperand, comp.Operator, comp.RightOperand, litInfo, comp.Line, comp.ResultDrives)
		}
	}

	fmt.Printf("\n=== Verbose: Arithmetic Ops (Power Analysis) ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, op := range facts.ArithmeticOps {
			guardInfo := "unguarded"
			if op.IsGuarded {
				guardInfo = fmt.Sprintf("guarded by %s", op.GuardSignal)
			}
			fmt.Printf("  %s: %s (%s, line %d)\n", op.Operator, strings.Join(op.Operands, ", "), guardInfo, op.Line)
		}
	}

	fmt.Printf("\n=== Verbose: Signal Dependencies (Loop Detection) ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, dep := range facts.SignalDeps {
			seqInfo := "combinational"
			if dep.IsSequential {
				seqInfo = "sequential"
			}
			fmt.Printf("  %s -> %s (%s, line %d)\n", dep.Source, dep.Target, seqInfo, dep.Line)
		}
	}

	fmt.Printf("\n=== Verbose: Type Declarations ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, t := range facts.Types {
			scope := t.InPackage
			if scope == "" {
				scope = t.InArch
			}
			fmt.Printf("  %s.%s: kind=%s line=%d\n", scope, t.Name, t.Kind, t.Line)
			if t.Kind == "enum" && len(t.EnumLiterals) > 0 {
				fmt.Printf("    literals: %v\n", t.EnumLiterals)
			}
			if t.Kind == "record" && len(t.Fields) > 0 {
				fmt.Printf("    fields:\n")
				for _, f := range t.Fields {
					fmt.Printf("      %s: %s\n", f.Name, f.Type)
				}
			}
			if t.Kind == "array" {
				unc := ""
				if t.Unconstrained {
					unc = " (unconstrained)"
				}
				fmt.Printf("    element: %s%s\n", t.ElementType, unc)
			}
		}
	}

	fmt.Printf("\n=== Verbose: Subtype Declarations ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, st := range facts.Subtypes {
			scope := st.InPackage
			if scope == "" {
				scope = st.InArch
			}
			constraint := ""
			if st.Constraint != "" {
				constraint = " " + st.Constraint
			}
			fmt.Printf("  %s.%s: %s%s\n", scope, st.Name, st.BaseType, constraint)
		}
	}

	fmt.Printf("\n=== Verbose: Function Declarations ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, fn := range facts.Functions {
			scope := fn.InPackage
			if scope == "" {
				scope = fn.InArch
			}
			purity := "pure"
			if !fn.IsPure {
				purity = "impure"
			}
			hasBody := ""
			if fn.HasBody {
				hasBody = " [body]"
			}
			fmt.Printf("  %s.%s: %s returns %s%s\n", scope, fn.Name, purity, fn.ReturnType, hasBody)
			if len(fn.Parameters) > 0 {
				fmt.Printf("    params:\n")
				for _, p := range fn.Parameters {
					dir := p.Direction
					if dir == "" {
						dir = "in"
					}
					fmt.Printf("      %s: %s %s\n", p.Name, dir, p.Type)
				}
			}
		}
	}

	fmt.Printf("\n=== Verbose: Procedure Declarations ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, pr := range facts.Procedures {
			scope := pr.InPackage
			if scope == "" {
				scope = pr.InArch
			}
			hasBody := ""
			if pr.HasBody {
				hasBody = " [body]"
			}
			fmt.Printf("  %s.%s%s\n", scope, pr.Name, hasBody)
			if len(pr.Parameters) > 0 {
				fmt.Printf("    params:\n")
				for _, p := range pr.Parameters {
					dir := p.Direction
					if dir == "" {
						dir = "in"
					}
					fmt.Printf("      %s: %s %s\n", p.Name, dir, p.Type)
				}
			}
		}
	}

	fmt.Printf("\n=== Verbose: Constant Declarations ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, c := range facts.ConstantDecls {
			scope := c.InPackage
			if scope == "" {
				scope = c.InArch
			}
			value := ""
			if c.Value != "" {
				value = fmt.Sprintf(" := %s", c.Value)
			}
			fmt.Printf("  %s.%s: %s%s\n", scope, c.Name, c.Type, value)
		}
	}

	fmt.Printf("\n=== Verbose: Generate Statements ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, gen := range facts.Generates {
			switch gen.Kind {
			case "for":
				elaboration := "cannot elaborate"
				if gen.CanElaborate {
					elaboration = fmt.Sprintf("%d iterations", gen.IterationCount)
				}
				fmt.Printf("  %s: for %s in %s %s %s (%s)\n",
					gen.Label, gen.LoopVar, gen.RangeLow, gen.RangeDir, gen.RangeHigh, elaboration)
			case "if":
				fmt.Printf("  %s: if %s\n", gen.Label, gen.Condition)
			case "case":
				fmt.Printf("  %s: case %s\n", gen.Label, gen.Condition)
			default:
				fmt.Printf("  %s: %s\n", gen.Label, gen.Kind)
			}
			if len(gen.Signals) > 0 || len(gen.Instances) > 0 || len(gen.Processes) > 0 {
				fmt.Printf("    contains: %d signals, %d instances, %d processes\n",
					len(gen.Signals), len(gen.Instances), len(gen.Processes))
			}
		}
	}

	fmt.Printf("\n=== Verbose: CDC Crossings ===\n")
	for _, file := range files {
		facts, err := loadFacts(file)
		if err != nil {
			return err
		}
		for _, cdc := range facts.CDCCrossings {
			syncStatus := "unsynchronized"
			if cdc.IsSynchronized {
				syncStatus = fmt.Sprintf("synchronized (%d stages)", cdc.SyncStages)
			}
			bitWidth := "single-bit"
			if cdc.IsMultiBit {
				bitWidth = "multi-bit"
			}
			fmt.Printf("  %s: %s -> %s (%s, %s) [%s]\n",
				cdc.Signal, cdc.SourceClock, cdc.DestClock, bitWidth, syncStatus,
				fmt.Sprintf("%s writes, %s reads", cdc.SourceProc, cdc.DestProc))
		}
	}

	return nil
}
