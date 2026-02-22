package main

import "fmt"

type SemanticChecker struct {
	modules []*Module
	errors  []string
}

func NewSemanticChecker(mods []*Module) *SemanticChecker {
	return &SemanticChecker{
		modules: mods,
		errors:  make([]string, 0),
	}
}

func (c *SemanticChecker) Check() []string {
	c.checkCrossModuleInstantiations() // <-- Run cross-module checks first
	for _, mod := range c.modules {
		c.checkUndeclaredWires(mod)
		c.checkInputImmutability(mod)
		c.checkMultiDrivenWires(mod)
		c.checkVerificationCoverage(mod)
		c.checkPslDelayBounds(mod)
		c.checkCDCViolations(mod)
	}
	return c.errors
}

func (c *SemanticChecker) checkVerificationCoverage(mod *Module) {
	// Build a set of all signals referenced in ANY PSL assertion.
	verifiedSignals := make(map[string]bool)

	var walkExpr func(expr Expression)
	walkExpr = func(expr Expression) {
		if expr == nil {
			return
		}
		switch v := expr.(type) {
		case IdentifierExpr:
			verifiedSignals[v.Name] = true
		case IndexedNameExpr:
			verifiedSignals[v.Prefix] = true
			walkExpr(v.Index)
		case SelectedNameExpr:
			verifiedSignals[v.Prefix] = true
		case BinaryExpr:
			walkExpr(v.Left)
			walkExpr(v.Right)
		case UnaryExpr:
			walkExpr(v.Right)
		case ParenExpr:
			walkExpr(v.Expr)
		case PslImplicationExpr:
			walkExpr(v.Left)
			walkExpr(v.Right)
		case PslEventuallyExpr:
			walkExpr(v.Left)
			walkExpr(v.Right)
		case PslStableExpr:
			walkExpr(v.Signal)
		case PslSequenceExpr:
			for _, el := range v.Elements {
				walkExpr(el.Expr)
			}
		}
	}

	for _, stmt := range mod.Statements {
		switch s := stmt.(type) {
		case *SymbolicDeclaration:
			for _, name := range s.Names {
				verifiedSignals[name] = true
			}
		case *PslAssertion:
			walkExpr(s.Property)
		case *PslAfterResetAssertion:
			walkExpr(s.Property)
		case *PslAssertNever:
			walkExpr(s.Property)
		case *PslCover:
			walkExpr(s.Property)
		}
	}

	for _, fb := range mod.Formals {
		for _, stmt := range fb.Statements {
			switch s := stmt.(type) {
			case *SymbolicDeclaration:
				for _, name := range s.Names {
					verifiedSignals[name] = true
				}
			case *PslAssertion:
				walkExpr(s.Property)
			case *PslAfterResetAssertion:
				walkExpr(s.Property)
			case *PslAssertNever:
				walkExpr(s.Property)
			case *PslCover:
				walkExpr(s.Property)
			}
		}
	}

	// 1. Output Port Coverage
	for _, p := range mod.Ports {
		if p.Direction == "out" && !verifiedSignals[p.Name] {
			c.errors = append(c.errors, fmt.Sprintf("Line %d: Warning - Output port '%s' is unconstrained by any PSL contract", p.LineNum, p.Name))
		}
	}

	// 2. State/Register Coverage
	// Collect all registers (targets of sequential assignments inside SynchronousProcess)
	registers := make(map[string]uint32)
	var walkSeq func(stmts []SequentialStatement)
	walkSeq = func(stmts []SequentialStatement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *SequentialAssignment:
				registers[s.Target] = s.LineNum
			case *SequentialIf:
				walkSeq(s.Then)
				for _, e := range s.Elsifs {
					walkSeq(e.Then)
				}
				walkSeq(s.Else)
			}
		}
	}

	for _, stmt := range mod.Statements {
		if sp, ok := stmt.(*SynchronousProcess); ok {
			walkSeq(sp.Statements)
		}
	}

	for regName, lineNum := range registers {
		if !verifiedSignals[regName] {
			// Don't warn about outputs that were already warned about above
			isOutput := false
			for _, p := range mod.Ports {
				if p.Name == regName && p.Direction == "out" {
					isOutput = true
					break
				}
			}
			if !isOutput {
				c.errors = append(c.errors, fmt.Sprintf("Line %d: Warning - State register '%s' lacks temporal assertions", lineNum, regName))
			}
		}
	}
	
	// 3. MUX Completeness (Optional/Future: Ensuring 'others' clause logic isn't sweeping invalid states)
}

func (c *SemanticChecker) checkPslDelayBounds(mod *Module) {
	foundNext := false

	var walkExpr func(expr Expression, line uint32, inImplicationRight bool)
	walkExpr = func(expr Expression, line uint32, inImplicationRight bool) {
		if expr == nil {
			return
		}
		switch v := expr.(type) {
		case PslNextExpr:
			foundNext = true
			if v.Delay < 1 {
				c.errorf(line, "PSL next delay must be >= 1")
			}
			walkExpr(v.Arg, line, false)
		case PslNextRangeExpr:
			foundNext = true
			if v.Start < 1 {
				c.errorf(line, "PSL next_a start must be >= 1")
			}
			if v.End < v.Start {
				c.errorf(line, "PSL next_a end must be >= start")
			}
			walkExpr(v.Arg, line, false)
		case PslImplicationExpr:
			walkExpr(v.Left, line, false)
			walkExpr(v.Right, line, true)
		case PslEventuallyExpr:
			walkExpr(v.Left, line, false)
			walkExpr(v.Right, line, false)
		case PslStableExpr:
			walkExpr(v.Signal, line, false)
		case PslSequenceExpr:
			for _, el := range v.Elements {
				walkExpr(el.Expr, line, inImplicationRight)
			}
		case BinaryExpr:
			walkExpr(v.Left, line, false)
			walkExpr(v.Right, line, false)
		case UnaryExpr:
			walkExpr(v.Right, line, false)
		case ParenExpr:
			walkExpr(v.Expr, line, false)
		case IndexedNameExpr:
			walkExpr(v.Index, line, false)
		case SelectedNameExpr:
			// no-op
		case IdentifierExpr, LiteralExpr:
			// no-op
		}
	}

	for _, stmt := range mod.Statements {
		switch s := stmt.(type) {
		case *PslAssertion:
			walkExpr(s.Property, s.Line(), false)
		case *PslAssumption:
			walkExpr(s.Property, s.Line(), false)
		case *PslCover:
			walkExpr(s.Property, s.Line(), false)
		case *PslAssertNever:
			walkExpr(s.Property, s.Line(), false)
		case *PslAfterResetAssertion:
			walkExpr(s.Reset, s.Line(), false)
			walkExpr(s.Property, s.Line(), false)
		}
	}

	for _, fb := range mod.Formals {
		for _, stmt := range fb.Statements {
			switch s := stmt.(type) {
			case *PslAssertion:
				walkExpr(s.Property, s.Line(), false)
			case *PslAssumption:
				walkExpr(s.Property, s.Line(), false)
			case *PslCover:
				walkExpr(s.Property, s.Line(), false)
			case *PslAssertNever:
				walkExpr(s.Property, s.Line(), false)
			case *PslAfterResetAssertion:
				walkExpr(s.Reset, s.Line(), false)
				walkExpr(s.Property, s.Line(), false)
			}
		}
	}

	if foundNext && mod.ClockPort == "" {
		c.errorf(1, "PSL next/next_a requires a clocked module (missing clock port)")
	}
}

func (c *SemanticChecker) errorf(line uint32, format string, args ...interface{}) {
	msg := fmt.Sprintf("Line %d: Semantic Error - %s", line, fmt.Sprintf(format, args...))
	c.errors = append(c.errors, msg)
}

func (c *SemanticChecker) checkUndeclaredWires(mod *Module) {
	var walk func(stmts []Statement)
	walk = func(stmts []Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ConcurrentAssignment:
				if _, ok := mod.Symbols[s.Target]; !ok {
					c.errorf(s.Line(), "Undeclared wire '%s' in concurrent assignment", s.Target)
				}
			case *SelectedAssignment:
				if _, ok := mod.Symbols[s.Target]; !ok {
					c.errorf(s.Line(), "Undeclared wire '%s' in selected assignment", s.Target)
				}
			case *SynchronousProcess:
				for _, sq := range s.Statements {
					c.walkSequentialStmtForUndeclared(sq, mod)
				}
			case *GenerateStatement:
				walk(s.Statements)
			}
		}
	}
	walk(mod.Statements)
}

func (c *SemanticChecker) walkSequentialStmtForUndeclared(stmt SequentialStatement, mod *Module) {
	switch sa := stmt.(type) {
	case *SequentialAssignment:
		if _, ok := mod.Symbols[sa.Target]; !ok {
			c.errorf(sa.Line(), "Undeclared wire '%s' in sequential assignment", sa.Target)
		}
	case *SequentialIf:
		for _, s := range sa.Then {
			c.walkSequentialStmtForUndeclared(s, mod)
		}
		for _, e := range sa.Elsifs {
			for _, s := range e.Then {
				c.walkSequentialStmtForUndeclared(s, mod)
			}
		}
		for _, s := range sa.Else {
			c.walkSequentialStmtForUndeclared(s, mod)
		}
	}
}

func (c *SemanticChecker) checkInputImmutability(mod *Module) {
	inputs := make(map[string]bool)
	for _, p := range mod.Ports {
		if p.Direction == "in" {
			inputs[p.Name] = true
		}
	}

	var walk func(stmts []Statement)
	walk = func(stmts []Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ConcurrentAssignment:
				if inputs[s.Target] {
					c.errorf(s.Line(), "Cannot route value into input port '%s'", s.Target)
				}
			case *SelectedAssignment:
				if inputs[s.Target] {
					c.errorf(s.Line(), "Cannot route value into input port '%s'", s.Target)
				}
			case *SynchronousProcess:
				for _, sq := range s.Statements {
					c.walkSequentialStmtForInput(sq, inputs)
				}
			case *GenerateStatement:
				walk(s.Statements)
			}
		}
	}
	walk(mod.Statements)
}

func (c *SemanticChecker) walkSequentialStmtForInput(stmt SequentialStatement, inputs map[string]bool) {
	switch sa := stmt.(type) {
	case *SequentialAssignment:
		if inputs[sa.Target] {
			c.errorf(sa.Line(), "Cannot route value into input port '%s'", sa.Target)
		}
	case *SequentialIf:
		for _, s := range sa.Then {
			c.walkSequentialStmtForInput(s, inputs)
		}
		for _, e := range sa.Elsifs {
			for _, s := range e.Then {
				c.walkSequentialStmtForInput(s, inputs)
			}
		}
		for _, s := range sa.Else {
			c.walkSequentialStmtForInput(s, inputs)
		}
	}
}

func (c *SemanticChecker) checkMultiDrivenWires(mod *Module) {
	driven := make(map[string]uint32)

	var walk func(stmts []Statement)
	walk = func(stmts []Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *ConcurrentAssignment:
				if line, ok := driven[s.Target]; ok {
					c.errorf(s.Line(), "Wire '%s' is driven multiple times (previously driven at line %d)", s.Target, line)
				} else {
					driven[s.Target] = s.LineNum
				}
			case *SelectedAssignment:
				if line, ok := driven[s.Target]; ok {
					c.errorf(s.Line(), "Wire '%s' is driven multiple times (previously driven at line %d)", s.Target, line)
				} else {
					driven[s.Target] = s.LineNum
				}
			case *SynchronousProcess:
				procDriven := make(map[string]bool)
				c.walkSequentialStmtForDriven(s.Statements, procDriven)
				
				for target := range procDriven {
					if line, ok := driven[target]; ok {
						c.errorf(s.Line(), "Wire '%s' is driven multiple times (previously driven at line %d)", target, line)
					} else {
						driven[target] = s.LineNum
					}
				}
			case *GenerateStatement:
				// Note: Currently tracking multi-driven wires statically.
				// Arrays/indexed targets will need dynamic loop evaluation in the future.
				walk(s.Statements)
			}
		}
	}
	walk(mod.Statements)
}

func (c *SemanticChecker) walkSequentialStmtForDriven(stmts []SequentialStatement, procDriven map[string]bool) {
	for _, stmt := range stmts {
		switch sa := stmt.(type) {
		case *SequentialAssignment:
			procDriven[sa.Target] = true
		case *SequentialIf:
			c.walkSequentialStmtForDriven(sa.Then, procDriven)
			for _, e := range sa.Elsifs {
				c.walkSequentialStmtForDriven(e.Then, procDriven)
			}
			c.walkSequentialStmtForDriven(sa.Else, procDriven)
		}
	}
}

// cdcPrimitives are entity names recognized as safe CDC crossing boundaries.
var cdcPrimitives = map[string]bool{
	"sync_2ff":         true,
	"async_fifo_gray":  true,
}

func (c *SemanticChecker) checkCDCViolations(mod *Module) {
	// Only meaningful in multi-clock designs.
	if len(mod.ClockPorts) < 2 {
		return
	}

	// Step 1: Build signalDomain — signal name → clock domain that drives it.
	signalDomain := make(map[string]string)

	var collectTargets func(stmts []SequentialStatement, clk string)
	collectTargets = func(stmts []SequentialStatement, clk string) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *SequentialAssignment:
				signalDomain[s.Target] = clk
			case *SequentialIf:
				collectTargets(s.Then, clk)
				for _, el := range s.Elsifs {
					collectTargets(el.Then, clk)
				}
				collectTargets(s.Else, clk)
			}
		}
	}

	for _, stmt := range mod.Statements {
		if sp, ok := stmt.(*SynchronousProcess); ok && sp.Clock != "" {
			collectTargets(sp.Statements, sp.Clock)
		}
	}

	// Step 2: Build cdcSafeOutputs — wires that are outputs of recognized CDC primitives.
	cdcSafeOutputs := make(map[string]bool)
	for _, stmt := range mod.Statements {
		inst, ok := stmt.(*EntityInstantiation)
		if !ok || !cdcPrimitives[inst.EntityName] {
			continue
		}
		// Mark any actual wire connected to an output port of the CDC primitive as safe.
		for _, pm := range inst.PortMap {
			// Find if this port is an output on the child entity; we infer by
			// convention: for sync_2ff the output port is "d_out".
			// For any recognized CDC primitive, all actual identifiers on output
			// ports are considered safe. We look at the associated module to
			// determine port directions.
			modMap := make(map[string]*Module)
			for _, m := range c.modules {
				modMap[m.Name] = m
			}
			if child, exists := modMap[inst.EntityName]; exists {
				for _, cp := range child.Ports {
					if cp.Name == pm.Formal && cp.Direction == "out" {
						if id, isId := pm.Actual.(IdentifierExpr); isId {
							cdcSafeOutputs[id.Name] = true
						}
					}
				}
			} else {
				// If the child module isn't in this compilation unit (e.g. a library
				// primitive), conservatively mark all actuals as safe.
				if id, isId := pm.Actual.(IdentifierExpr); isId {
					cdcSafeOutputs[id.Name] = true
				}
			}
		}
	}

	// cdcRead pairs an identifier read with the source line that reads it.
	type cdcRead struct {
		name string
		line uint32
	}

	// Step 3: For each process, walk expressions and flag cross-domain reads.
	// Line numbers are carried per-statement so the error points at the specific
	// assignment or condition, not the process header.
	var collectReads func(expr Expression, line uint32) []cdcRead
	collectReads = func(expr Expression, line uint32) []cdcRead {
		if expr == nil {
			return nil
		}
		var ids []cdcRead
		switch v := expr.(type) {
		case IdentifierExpr:
			ids = append(ids, cdcRead{v.Name, line})
		case IndexedNameExpr:
			ids = append(ids, cdcRead{v.Prefix, line})
			ids = append(ids, collectReads(v.Index, line)...)
		case SelectedNameExpr:
			ids = append(ids, cdcRead{v.Prefix, line})
		case BinaryExpr:
			ids = append(ids, collectReads(v.Left, line)...)
			ids = append(ids, collectReads(v.Right, line)...)
		case UnaryExpr:
			ids = append(ids, collectReads(v.Right, line)...)
		case ParenExpr:
			ids = append(ids, collectReads(v.Expr, line)...)
		case FunctionCallExpr:
			for _, arg := range v.Args {
				ids = append(ids, collectReads(arg, line)...)
			}
		}
		return ids
	}

	var collectReadsSeq func(stmts []SequentialStatement) []cdcRead
	collectReadsSeq = func(stmts []SequentialStatement) []cdcRead {
		var ids []cdcRead
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *SequentialAssignment:
				ids = append(ids, collectReads(s.Value, s.LineNum)...)
			case *SequentialIf:
				ids = append(ids, collectReads(s.Condition, s.LineNum)...)
				ids = append(ids, collectReadsSeq(s.Then)...)
				for _, el := range s.Elsifs {
					ids = append(ids, collectReads(el.Condition, s.LineNum)...)
					ids = append(ids, collectReadsSeq(el.Then)...)
				}
				ids = append(ids, collectReadsSeq(s.Else)...)
			}
		}
		return ids
	}

	// Track which violations we've already reported to avoid duplicate messages
	// for the same signal pair (one per unique signal, not one per read site).
	reported := make(map[string]bool)

	for _, stmt := range mod.Statements {
		sp, ok := stmt.(*SynchronousProcess)
		if !ok || sp.Clock == "" {
			continue
		}
		reads := collectReadsSeq(sp.Statements)
		for _, r := range reads {
			srcDomain, driven := signalDomain[r.name]
			if !driven || srcDomain == sp.Clock {
				continue
			}
			if cdcSafeOutputs[r.name] {
				continue
			}
			key := r.name + ":" + sp.Clock
			if reported[key] {
				continue
			}
			reported[key] = true

			// Suggest a concrete sync wire name and instantiation the LLM can
			// copy-paste verbatim.
			syncWire := r.name + "_sync"
			c.errors = append(c.errors, fmt.Sprintf(
				"Line %d: Semantic Error - Signal '%s' (domain '%s') is read inside the '%s' process without a CDC synchronizer.\n"+
					"  Fix: declare a synchronized wire and add a sync_2ff instance in the architecture declarative region and concurrent body:\n"+
					"    signal %s : std_logic;\n"+
					"    u_sync_%s : entity work.sync_2ff port map (\n"+
					"      clk_dst => %s, d_in => %s, d_out => %s\n"+
					"    );\n"+
					"  Then replace '%s' with '%s' inside the %s process.",
				r.line, r.name, srcDomain, sp.Clock,
				syncWire,
				r.name,
				sp.Clock, r.name, syncWire,
				r.name, syncWire, sp.Clock,
			))
		}
	}
}

func (c *SemanticChecker) checkCrossModuleInstantiations() {
	modMap := make(map[string]*Module)
	for _, m := range c.modules {
		modMap[m.Name] = m
	}

	for _, mod := range c.modules {
		var walk func(stmts []Statement)
		walk = func(stmts []Statement) {
			for _, stmt := range stmts {
				switch s := stmt.(type) {
				case *EntityInstantiation:
					child, exists := modMap[s.EntityName]
					if !exists {
						c.errorf(s.LineNum, "Instantiated entity '%s' not found. Missing .vhd file?", s.EntityName)
						continue
					}
					
					childPorts := make(map[string]*Port)
					for _, p := range child.Ports {
						childPorts[p.Name] = p
					}
					
					mappedPorts := make(map[string]bool)
					for _, pmap := range s.PortMap {
						mappedPorts[pmap.Formal] = true
						if portDef, ok := childPorts[pmap.Formal]; !ok {
							c.errorf(s.LineNum, "Port '%s' does not exist on entity '%s'", pmap.Formal, s.EntityName)
						} else if portDef.Direction == "out" {
							// Output ports must map to a pure wire, not an expression
							if _, isId := pmap.Actual.(IdentifierExpr); !isId {
								c.errorf(s.LineNum, "Output port '%s' of instance '%s' must map to a pure wire identifier, not an expression", pmap.Formal, s.Label)
							}
						}
					}

					for _, p := range child.Ports {
						if p.Direction == "in" && !mappedPorts[p.Name] {
							c.errorf(s.LineNum, "Input port '%s' on entity '%s' is left unconnected", p.Name, s.EntityName)
						}
					}
				case *GenerateStatement:
					walk(s.Statements)
				}
			}
		}
		walk(mod.Statements)
	}
}
