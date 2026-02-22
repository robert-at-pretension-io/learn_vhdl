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
	for _, mod := range c.modules {
		c.checkUndeclaredWires(mod)
		c.checkInputImmutability(mod)
		c.checkMultiDrivenWires(mod)
		c.checkVerificationCoverage(mod)
		c.checkPslDelayBounds(mod)
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
		}
	}

	for _, stmt := range mod.Statements {
		switch s := stmt.(type) {
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
			if !inImplicationRight {
				c.errorf(line, "PSL next is only supported on the right side of |=>")
			}
			if v.Delay < 1 {
				c.errorf(line, "PSL next delay must be >= 1")
			}
			walkExpr(v.Arg, line, false)
		case PslNextRangeExpr:
			foundNext = true
			if !inImplicationRight {
				c.errorf(line, "PSL next_a is only supported on the right side of |=>")
			}
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
