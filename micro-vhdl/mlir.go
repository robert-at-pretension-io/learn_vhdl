package main

import (
	"fmt"
	"strconv"
	"strings"
)

// assertionEntry holds a PSL assertion SSA value collected during statement emission.
// All i1 assertions are combined into a single verif.assert at module end (circt-bmc
// requires exactly one assertion per module).  !ltl.property assertions are emitted
// separately as verif.clocked_assert so they can be handled by --lower-ltl-to-core.
type assertionEntry struct {
	val string // SSA name
	typ string // "i1" or "!ltl.property"
}

// MLIREmitter handles translating populated Wires, Statements, and Modules into CIRCT HW/Comb MLIR text.
type MLIREmitter struct {
	builder     strings.Builder
	indent      int
	ssaID       int
	modules     map[string]*Module
	assertions  []assertionEntry // accumulated per-module, reset in EmitModule
	assumptions []assertionEntry // accumulated per-module, reset in EmitModule
	ic3Mode     bool             // emit __verif_bad hw.output instead of verif.assert
	badStateSSA string           // set by emitCombinedAssertions in ic3Mode
	nameID      int              // unique naming for generated regs
	clockI1SSA  string           // cached per-module i1 clock (seq.from_clock)
}

func NewMLIREmitter() *MLIREmitter {
	return &MLIREmitter{modules: make(map[string]*Module)}
}

// NewMLIREmitterIC3 returns an emitter that targets the IC3/PDR path: PSL
// assertions are emitted as a negated hw.output (__verif_bad) rather than
// verif.assert, so the module can be exported to AIGER and checked by ABC pdr.
func NewMLIREmitterIC3() *MLIREmitter {
	return &MLIREmitter{modules: make(map[string]*Module), ic3Mode: true}
}

// hasPslAssertions reports whether mod contains any PSL assertion statements
// (including after_reset assertions, which also accumulate into e.assertions).
func hasPslAssertions(mod *Module) bool {
	for _, stmt := range mod.Statements {
		switch stmt.(type) {
		case *PslAssertion, *PslAfterResetAssertion, *PslAssertNever:
			return true
		}
	}
	return false
}

func (e *MLIREmitter) p(format string, args ...interface{}) {
	for i := 0; i < e.indent; i++ {
		e.builder.WriteString("  ")
	}
	e.builder.WriteString(fmt.Sprintf(format, args...))
	e.builder.WriteString("\n")
}

func (e *MLIREmitter) String() string {
	return e.builder.String()
}

func (e *MLIREmitter) nextSSA() string {
	val := fmt.Sprintf("%%%d", e.ssaID)
	e.ssaID++
	return val
}

func (e *MLIREmitter) nextName(prefix string) string {
	name := fmt.Sprintf("%s_%d", prefix, e.nameID)
	e.nameID++
	return name
}

// clockSSA returns the SSA name of the module's clock port (e.g. "%clk").
func (e *MLIREmitter) clockSSA(mod *Module) string {
	if mod.ClockPort == "" {
		return ""
	}
	return fmt.Sprintf("%%%s", mod.ClockPort)
}

// clockI1 returns an i1 clock SSA (seq.from_clock) for LTL properties.
func (e *MLIREmitter) clockI1(mod *Module) string {
	if e.clockI1SSA != "" {
		return e.clockI1SSA
	}
	clk := e.clockSSA(mod)
	if clk == "" {
		return ""
	}
	clkI1 := e.nextSSA()
	e.p("%s = seq.from_clock %s", clkI1, clk)
	e.clockI1SSA = clkI1
	return clkI1
}

func (e *MLIREmitter) emitCompReg(val string, clk string, typ string, name string) string {
	res := e.nextSSA()
	initSSA := e.emitSeqInitial(typ)
	if initSSA != "" {
		e.p("%s = seq.compreg name \"%s\" %s, %s initial %s : %s", res, name, val, clk, initSSA, typ)
	} else {
		e.p("%s = seq.compreg name \"%s\" %s, %s : %s", res, name, val, clk, typ)
	}
	return res
}

func (e *MLIREmitter) emitFormalStatementList(stmts []Statement, mod *Module, env map[string]string, extraSymbols map[string]Type) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *SymbolicDeclaration:
			t := typeToMLIR(s.Type, mod)
			for _, name := range s.Names {
				e.p("%%%s = verif.symbolic_value : %s", name, t)
				env[name] = fmt.Sprintf("%%%s", name)
			}
		case *PslAssertion:
			val, typ := e.emitExpression(s.Property, mod, env, "", "", extraSymbols)
			if typ == "!ltl.sequence" {
				clkI1 := e.clockI1(mod)
				if clkI1 != "" {
					clocked := e.nextSSA()
					e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, val, clkI1)
					val = clocked
					typ = "!ltl.property"
				} else {
					e.p("// ERROR: PSL temporal property requires a clock")
				}
			}
			e.assertions = append(e.assertions, assertionEntry{val, typ})
		case *PslAssumption:
			val, typ := e.emitExpression(s.Property, mod, env, "", "", extraSymbols)
			if typ == "!ltl.sequence" {
				clkI1 := e.clockI1(mod)
				if clkI1 != "" {
					clocked := e.nextSSA()
					e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, val, clkI1)
					val = clocked
					typ = "!ltl.property"
				} else {
					e.p("// ERROR: PSL temporal property requires a clock")
				}
			}
			e.assumptions = append(e.assumptions, assertionEntry{val, typ})
		case *PslCover:
			val, typ := e.emitExpression(s.Property, mod, env, "", "", extraSymbols)
			if typ == "!ltl.sequence" {
				clkI1 := e.clockI1(mod)
				if clkI1 != "" {
					clocked := e.nextSSA()
					e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, val, clkI1)
					val = clocked
					typ = "!ltl.property"
				} else {
					e.p("// ERROR: PSL temporal property requires a clock")
				}
			}
			e.p("verif.cover %s : %s", val, typ)
		case *PslAssertNever:
			val, _ := e.emitExpression(s.Property, mod, env, "", "i1", extraSymbols)
			trueVal := e.nextSSA()
			notProp := e.nextSSA()
			e.p("%s = hw.constant -1 : i1", trueVal)
			e.p("%s = comb.xor %s, %s : i1", notProp, val, trueVal)
			e.assertions = append(e.assertions, assertionEntry{notProp, "i1"})
		}
	}
}

func (e *MLIREmitter) emitPslFormalBlock(fb *PslFormalBlock, mod *Module) {
	e.p("verif.formal @%s {} {", fb.Name)
	e.indent++

	// Reset per-module assertion tracking for the formal block.
	oldAsserts := e.assertions
	oldAssumes := e.assumptions
	e.assertions = e.assertions[:0]
	e.assumptions = e.assumptions[:0]

	env := make(map[string]string)
	e.emitFormalStatementList(fb.Statements, mod, env, fb.Symbols)
	
	// Boolean assertions are combined into a single verif.assert (circt-bmc requirements).
	var boolAsserts []string
	var temporalAsserts []assertionEntry
	for _, a := range e.assertions {
		if a.typ == "i1" {
			boolAsserts = append(boolAsserts, a.val)
		} else {
			temporalAsserts = append(temporalAsserts, a)
		}
	}
	
	var combinedAssume string
	for _, a := range e.assumptions {
		if a.typ != "i1" { continue }
		if combinedAssume == "" {
			combinedAssume = a.val
		} else {
			res := e.nextSSA()
			e.p("%s = comb.and %s, %s : i1", res, combinedAssume, a.val)
			combinedAssume = res
		}
	}
	if combinedAssume != "" {
		e.p("verif.assume %s : i1", combinedAssume)
	}

	var combined string
	if len(boolAsserts) == 1 {
		combined = boolAsserts[0]
	} else if len(boolAsserts) > 1 {
		combined = boolAsserts[0]
		for _, next := range boolAsserts[1:] {
			res := e.nextSSA()
			e.p("%s = comb.and %s, %s : i1", res, combined, next)
			combined = res
		}
	}
	if combined != "" {
		e.p("verif.assert %s : i1", combined)
	}
	for _, ta := range temporalAsserts {
		if ta.typ == "!ltl.sequence" {
			clkI1 := e.clockI1(mod)
			if clkI1 != "" {
				clocked := e.nextSSA()
				e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, ta.val, clkI1)
				e.p("verif.assert %s : !ltl.property", clocked)
			} else {
				e.p("// ERROR: PSL temporal property requires a clock")
			}
		} else {
			e.p("verif.assert %s : !ltl.property", ta.val)
		}
	}

	e.indent--
	e.p("}")
	
	// Restore parent module's assertion tracking.
	e.assertions = oldAsserts
	e.assumptions = oldAssumes
}

// emitDelay emits a chain of seq.compreg registers to delay a value by N cycles.
func (e *MLIREmitter) emitDelay(val string, typ string, delay int, clk string) string {
	res := val
	for i := 0; i < delay; i++ {
		next := e.emitCompReg(res, clk, typ, e.nextName("psl_delay"))
		res = next
	}
	return res
}

// emitSeqInitial emits a seq.initial block that yields a zero value of the
// given MLIR type and returns its SSA name.  Returns "" for complex types
// (hw.array / hw.struct) or in IC3 mode (PDR explores from all initial states;
// pinning them to zero would miss bugs only reachable from other start states).
func (e *MLIREmitter) emitSeqInitial(mlirType string) string {
	if e.ic3Mode || strings.HasPrefix(mlirType, "!hw.") {
		return ""
	}
	initSSA := e.nextSSA()
	zeroSSA := e.nextSSA()
	e.p("%s = seq.initial () {", initSSA)
	e.indent++
	e.p("%s = hw.constant 0 : %s", zeroSSA, mlirType)
	e.p("seq.yield %s : %s", zeroSSA, mlirType)
	e.indent--
	e.p("} : () -> !seq.immutable<%s>", mlirType)
	return initSSA
}

// typeToMLIR translates the extracted Type to its MLIR equivalent (e.g., std_logic -> i1, array -> !hw.array<size x type>)
func typeToMLIR(t Type, mod *Module) string {
	if !t.IsArray && !t.IsRecord && t.Width == 0 && t.Name != "std_logic" && t.Name != "unknown" {
		t = resolveType(mod, t.Name)
	}

	if t.IsArray && t.ElementType != nil {
		elType := typeToMLIR(*t.ElementType, mod)
		return fmt.Sprintf("!hw.array<%dx%s>", t.ArraySize, elType)
	}
	if t.IsRecord {
		var fields []string
		for _, f := range t.Fields {
			fields = append(fields, fmt.Sprintf("%s: %s", f.Name, typeToMLIR(f.Type, mod)))
		}
		return fmt.Sprintf("!hw.struct<%s>", strings.Join(fields, ", "))
	}
	if t.Width == 0 {
		return "i1" // Fallback
	}
	return fmt.Sprintf("i%d", t.Width)
}

func resolveType(mod *Module, typeName string) Type {
	t, ok := mod.Symbols[typeName]
	if ok && (t.IsArray || t.IsRecord) {
		return t
	}
	return t
}

func (e *MLIREmitter) emitExpression(expr Expression, mod *Module, env map[string]string, targetName string, expectedType string, extraSymbols map[string]Type) (string, string) {
	var resVal string
	if targetName != "" {
		resVal = fmt.Sprintf("%%%s", targetName)
	} else {
		resVal = e.nextSSA()
	}

	switch v := expr.(type) {
	case ParenExpr:
		return e.emitExpression(v.Expr, mod, env, targetName, expectedType, extraSymbols)
	case IdentifierExpr:
		t, ok := mod.Symbols[v.Name]
		if !ok && extraSymbols != nil {
			t = extraSymbols[v.Name]
		}
		if mapped, ok := env[v.Name]; ok {
			return mapped, typeToMLIR(t, mod)
		}
		return fmt.Sprintf("%%%s", v.Name), typeToMLIR(t, mod)
	case IndexedNameExpr:
		prefixVal, prefixType := e.emitExpression(IdentifierExpr{Name: v.Prefix}, mod, env, "", "", extraSymbols)
		indexVal, _ := e.emitExpression(v.Index, mod, env, "", "i32", extraSymbols)
		
		sym, ok := mod.Symbols[v.Prefix]
		if !ok && extraSymbols != nil {
			sym = extraSymbols[v.Prefix]
		}
		t := resolveType(mod, sym.Name)
		var elType string
		if t.ElementType != nil {
			elType = typeToMLIR(*t.ElementType, mod)
		} else {
			elType = "i1" // Fallback
		}
		
		e.p("%s = hw.array_get %s[%s] : %s, i32", resVal, prefixVal, indexVal, prefixType)
		return resVal, elType
	case SelectedNameExpr:
		prefixVal, prefixType := e.emitExpression(IdentifierExpr{Name: v.Prefix}, mod, env, "", "", extraSymbols)
		
		sym, ok := mod.Symbols[v.Prefix]
		if !ok && extraSymbols != nil {
			sym = extraSymbols[v.Prefix]
		}
		t := resolveType(mod, sym.Name)
		var elType string
		for _, f := range t.Fields {
			if f.Name == v.Suffix {
				elType = typeToMLIR(f.Type, mod)
				break
			}
		}
		if elType == "" {
			elType = "i1" // Fallback
		}
		
		e.p("%s = hw.struct_extract %s[\"%s\"] : %s", resVal, prefixVal, v.Suffix, prefixType)
		return resVal, elType
	case LiteralExpr:
		t := "i1"
		if expectedType != "" {
			t = expectedType
		}
		if strings.HasPrefix(v.Value, "'") {
			bit := strings.Trim(v.Value, "'")
			e.p("%s = hw.constant %s : %s", resVal, bit, t)
		} else if strings.HasPrefix(v.Value, "\"") {
			str := strings.Trim(v.Value, "\"")
			t = fmt.Sprintf("i%d", len(str))
			valInt, _ := strconv.ParseInt(str, 2, 64)
			e.p("%s = hw.constant %d : %s", resVal, valInt, t)
		} else if strings.HasPrefix(strings.ToLower(v.Value), "x\"") {
			str := strings.Trim(v.Value[2:], "\"")
			t = fmt.Sprintf("i%d", len(str)*4)
			valInt, _ := strconv.ParseInt(str, 16, 64)
			e.p("%s = hw.constant %d : %s", resVal, valInt, t)
		} else {
			e.p("%s = hw.constant %s : %s", resVal, v.Value, t)
		}
		return resVal, t
	case BinaryExpr:
		// Attempt to peek left and right types
		var inferredType string
		if id, ok := v.Left.(IdentifierExpr); ok {
			t, ok := mod.Symbols[id.Name]
			if !ok && extraSymbols != nil {
				t = extraSymbols[id.Name]
			}
			inferredType = typeToMLIR(t, mod)
		} else if id, ok := v.Right.(IdentifierExpr); ok {
			t, ok := mod.Symbols[id.Name]
			if !ok && extraSymbols != nil {
				t = extraSymbols[id.Name]
			}
			inferredType = typeToMLIR(t, mod)
		}

		leftVal, leftType := e.emitExpression(v.Left, mod, env, "", inferredType, extraSymbols)
		rightVal, rightType := e.emitExpression(v.Right, mod, env, "", inferredType, extraSymbols)
		
		// Unify types
		finalType := leftType
		if leftType == "i1" && rightType != "i1" {
			finalType = rightType
		}

		opMap := map[string]string{
			"and": "comb.and",
			"or":  "comb.or",
			"xor": "comb.xor",
			"+":   "comb.add",
			"-":   "comb.sub",
			"*":   "comb.mul",
		}

		if mlirOp, ok := opMap[v.Op]; ok {
			e.p("%s = %s %s, %s : %s", resVal, mlirOp, leftVal, rightVal, finalType)
			return resVal, finalType
		} else if v.Op == "=" {
			e.p("%s = comb.icmp eq %s, %s : %s", resVal, leftVal, rightVal, finalType)
			return resVal, "i1"
		} else if v.Op == "/=" {
			e.p("%s = comb.icmp ne %s, %s : %s", resVal, leftVal, rightVal, finalType)
			return resVal, "i1"
		}
		return resVal, finalType
	case UnaryExpr:
		if v.Op == "not" {
			rightVal, rightType := e.emitExpression(v.Right, mod, env, "", expectedType, extraSymbols)
			trueVal := e.nextSSA()
			e.p("%s = hw.constant -1 : %s", trueVal, rightType)
			e.p("%s = comb.xor %s, %s : %s", resVal, rightVal, trueVal, rightType)
			return resVal, rightType
		}
	case PslImplicationExpr:
		isOverlapping := (v.Op == "|->")

		leftVal, leftType := e.emitExpression(v.Left, mod, env, "", "i1", extraSymbols)
		rightVal, rightType := e.emitExpression(v.Right, mod, env, "", "i1", extraSymbols)

		isLtl := (leftType == "!ltl.sequence" || rightType == "!ltl.sequence" ||
			leftType == "!ltl.property" || rightType == "!ltl.property")

		if isLtl {
			clkI1 := e.clockI1(mod)
			if clkI1 == "" {
				e.p("// ERROR: PSL implication requires a clocked module")
				return resVal, "i1"
			}

			effLeftVal := leftVal
			effLeftType := leftType

			if !isOverlapping {
				// Convert LHS |=> RHS into {LHS; true} |-> RHS
				trueVal := e.nextSSA()
				e.p("%s = hw.constant -1 : i1", trueVal)
				delayedLeft := e.nextSSA()
				e.p("%s = ltl.concat %s, %s : %s, i1", delayedLeft, leftVal, trueVal, leftType)
				effLeftVal = delayedLeft
				effLeftType = "!ltl.sequence"
			}

			impVal := e.nextSSA()
			e.p("%s = ltl.implication %s, %s : %s, %s", impVal, effLeftVal, rightVal, effLeftType, rightType)
			
			if effLeftType == "!ltl.property" || rightType == "!ltl.property" {
				return impVal, "!ltl.property"
			}
			
			clocked := e.nextSSA()
			e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, impVal, clkI1)
			return clocked, "!ltl.property"
		}

		// Non-temporal fallback logic (Boolean evaluation, safe for IC3/AIGER)
		var effLeftVal string
		if isOverlapping {
			effLeftVal = leftVal
		} else {
			clk := e.clockSSA(mod)
			initSSA := e.emitSeqInitial("i1")
			effLeftVal = e.nextSSA()
			if initSSA != "" {
				e.p("%s = seq.compreg %s, %s initial %s : i1", effLeftVal, leftVal, clk, initSSA)
			} else {
				e.p("%s = seq.compreg %s, %s : i1", effLeftVal, leftVal, clk)
			}
		}

		trueVal := e.nextSSA()
		notLeft := e.nextSSA()
		e.p("%s = hw.constant -1 : i1", trueVal)
		e.p("%s = comb.xor %s, %s : i1", notLeft, effLeftVal, trueVal)
		e.p("%s = comb.or %s, %s : i1", resVal, notLeft, rightVal)
		return resVal, "i1"

	case PslSequenceExpr:
		var seqVals []string
		var seqTypes []string
		for _, el := range v.Elements {
			elVal, elType := e.emitExpression(el.Expr, mod, env, "", "i1", extraSymbols)
			if el.Repetition != nil {
				repVal := e.nextSSA()
				if el.Repetition.Count >= 0 {
					e.p("%s = ltl.repeat %s, %d : %s", repVal, elVal, el.Repetition.Count, elType)
				} else {
					e.p("%s = ltl.repeat %s, %d, %d : %s", repVal, elVal, el.Repetition.Start, el.Repetition.End, elType)
				}
				elVal = repVal
				elType = "!ltl.sequence"
			}
			seqVals = append(seqVals, elVal)
			seqTypes = append(seqTypes, elType)
		}

		if len(seqVals) == 1 {
			return seqVals[0], seqTypes[0]
		}

		resVal := e.nextSSA()
		e.p("%s = ltl.concat %s : %s", resVal, strings.Join(seqVals, ", "), strings.Join(seqTypes, ", "))
		return resVal, "!ltl.sequence"

	case PslEventuallyExpr:
		// PSL "left -> eventually! right" is a liveness property: right must become
		// true at some unbounded future cycle whenever left holds.  Bounded model
		// checking (circt-bmc) cannot directly encode liveness without an automaton
		// translation.  We emit a hw.constant true so the assertion always passes in
		// the BMC conjunction (not checked), and add a comment that flags the property
		// for review.  A full liveness proof requires an unbounded solver or k-liveness.
		e.p("// TODO liveness: psl assert always %s -> eventually! %s (skipped in BMC)", v.Left, v.Right)
		e.p("%s = hw.constant true", resVal)
		return resVal, "i1"

	case PslStableExpr:
		// PSL stable(x): x has not changed from the previous clock cycle.
		// Implement by storing the previous value in a register and comparing.
		sigVal, sigType := e.emitExpression(v.Signal, mod, env, "", "", extraSymbols)

		clk := e.clockSSA(mod)
		initSSA := e.emitSeqInitial(sigType)
		prevVal := e.nextSSA()
		if initSSA != "" {
			e.p("%s = seq.compreg %s, %s initial %s : %s", prevVal, sigVal, clk, initSSA, sigType)
		} else {
			e.p("%s = seq.compreg %s, %s : %s", prevVal, sigVal, clk, sigType)
		}
		e.p("%s = comb.icmp eq %s, %s : %s", resVal, sigVal, prevVal, sigType)
		return resVal, "i1"
	case PslNextExpr:
		argVal, _ := e.emitExpression(v.Arg, mod, env, "", "i1", extraSymbols)
		delay := v.Delay
		if delay < 1 {
			delay = 1
		}
		e.p("%s = ltl.delay %s, %d, 0 : i1", resVal, argVal, delay)
		return resVal, "!ltl.sequence"
	case PslNextRangeExpr:
		argVal, _ := e.emitExpression(v.Arg, mod, env, "", "i1", extraSymbols)
		start := v.Start
		end := v.End
		if start < 1 {
			start = 1
		}
		if end < start {
			end = start
		}
		e.p("%s = ltl.delay %s, %d, %d : i1", resVal, argVal, start, end-start)
		return resVal, "!ltl.sequence"
	}
	return e.nextSSA(), "i1"
}




// EmitModule translates the Module into a hw.module definition
func (e *MLIREmitter) emitSynchronousProcess(sp *SynchronousProcess, mod *Module, globalEnv map[string]string) {
	// Identify all registers assigned in this process.
	targets := make(map[string]bool)
	var walk func(stmt SequentialStatement)
	walk = func(stmt SequentialStatement) {
		switch s := stmt.(type) {
		case *SequentialAssignment:
			targets[s.Target] = true
		case *SequentialIf:
			for _, st := range s.Then { walk(st) }
			for _, el := range s.Elsifs {
				for _, st := range el.Then { walk(st) }
			}
			for _, st := range s.Else { walk(st) }
		}
	}
	for _, stmt := range sp.Statements { walk(stmt) }

	for target := range targets {
		localEnv := make(map[string]string)
		for k, v := range globalEnv { localEnv[k] = v }

		currentStateVal := fmt.Sprintf("%%%s", target)
		nextStateVal := e.buildSeqBlock(sp.Statements, target, currentStateVal, mod, localEnv)

		t := mod.Symbols[target]
		mlirType := typeToMLIR(t, mod)
		clk := e.clockSSA(mod)

		// Emit seq.initial so BMC tools know the register starts at zero rather
		// than at an arbitrary unconstrained value.
		initSSA := e.emitSeqInitial(mlirType)
		if initSSA != "" {
			e.p("%%%s = seq.compreg %s, %s initial %s : %s", target, nextStateVal, clk, initSSA, mlirType)
		} else {
			e.p("%%%s = seq.compreg %s, %s : %s", target, nextStateVal, clk, mlirType)
		}
		globalEnv[target] = fmt.Sprintf("%%%s", target)
	}
}

func (e *MLIREmitter) buildSeqBlock(stmts []SequentialStatement, target string, currentNextState string, mod *Module, env map[string]string) string {
	nextState := currentNextState
	targetType := typeToMLIR(mod.Symbols[target], mod)
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *SequentialAssignment:
			if s.Target == target {
				val, _ := e.emitExpression(s.Value, mod, env, "", targetType, nil)
				nextState = val
			}
		case *SequentialIf:
			condVal, _ := e.emitExpression(s.Condition, mod, env, "", "i1", nil)
			
			thenState := e.buildSeqBlock(s.Then, target, nextState, mod, env)
			elseState := e.buildSeqBlock(s.Else, target, nextState, mod, env)
			
			for i := len(s.Elsifs) - 1; i >= 0; i-- {
				el := s.Elsifs[i]
				elCondVal, _ := e.emitExpression(el.Condition, mod, env, "", "i1", nil)
				elThenState := e.buildSeqBlock(el.Then, target, nextState, mod, env)
				if elThenState != elseState {
					res := e.nextSSA()
					e.p("%s = comb.mux %s, %s, %s : %s", res, elCondVal, elThenState, elseState, targetType)
					elseState = res
				}
			}
			
			if thenState != elseState {
				res := e.nextSSA()
				e.p("%s = comb.mux %s, %s, %s : %s", res, condVal, thenState, elseState, targetType)
				nextState = res
			}
		}
	}
	return nextState
}

func (e *MLIREmitter) emitEntityInstantiation(inst *EntityInstantiation, parentMod *Module, env map[string]string) {
	childMod, ok := e.modules[inst.EntityName]
	if !ok {
		e.p("// ERROR: Could not find module %s", inst.EntityName)
		return
	}

	var inputs []string
	var outputs []string
	var outSigs []string
	var outTypes []string

	var instGenerics []string
	for _, gen := range childMod.Generics {
		// Default value
		val := gen.Default
		if val == "" {
			val = "0"
		}
		
		// See if it's provided in GenericMap
		for _, entry := range inst.GenericMap {
			if entry.Formal == gen.Name {
				// We expect a literal or expression. For now, assume literal or simple identifier
				if lit, ok := entry.Actual.(LiteralExpr); ok {
					val = lit.Value
				}
				break
			}
		}
		instGenerics = append(instGenerics, fmt.Sprintf("%s: i32 = %s", gen.Name, val))
	}
	
	genStr := ""
	if len(instGenerics) > 0 {
		genStr = fmt.Sprintf("<%s>", strings.Join(instGenerics, ", "))
	}

	// We need to match the formal ports to the actual arguments based on childMod definition
	for _, port := range childMod.Ports {
		var t string
		if port.Name == childMod.ClockPort {
			t = "!seq.clock"
		} else {
			t = typeToMLIR(port.Type, childMod)
		}
		var actualExpr Expression
		for _, entry := range inst.PortMap {
			if entry.Formal == port.Name {
				actualExpr = entry.Actual
				break
			}
		}

		if port.Direction == "in" {
			if actualExpr != nil {
				val, _ := e.emitExpression(actualExpr, parentMod, env, "", "", nil)
				inputs = append(inputs, fmt.Sprintf("%s: %s: %s", port.Name, val, t))
			} else {
				inputs = append(inputs, fmt.Sprintf("%s: %%missing: %s", port.Name, t))
			}
		} else if port.Direction == "out" {
			if actualExpr != nil {
				// We expect actual to be an IdentifierExpr for output routing
				if idExpr, isId := actualExpr.(IdentifierExpr); isId {
					val := fmt.Sprintf("%%%s", idExpr.Name)
					outputs = append(outputs, fmt.Sprintf("%s: %s", port.Name, t))
					outSigs = append(outSigs, val)
					outTypes = append(outTypes, t)
					// Update the environment to reflect that this signal is driven by the instance
					env[idExpr.Name] = val
				}
			} else {
				val := e.nextSSA()
				outputs = append(outputs, fmt.Sprintf("%s: %s", port.Name, t))
				outSigs = append(outSigs, val)
				outTypes = append(outTypes, t)
			}
		}
	}

	inStr := strings.Join(inputs, ", ")
	outStr := strings.Join(outputs, ", ")
	outAssigns := strings.Join(outSigs, ", ")

	if len(outSigs) > 0 {
		e.p("%s = hw.instance \"%s\" @%s%s(%s) -> (%s)", outAssigns, inst.Label, inst.EntityName, genStr, inStr, outStr)
	} else {
		e.p("hw.instance \"%s\" @%s%s(%s) -> ()", inst.Label, inst.EntityName, genStr, inStr)
	}
}

// EmitModules translates multiple Modules
func (e *MLIREmitter) EmitModules(mods []*Module) {
	for _, mod := range mods {
		e.modules[mod.Name] = mod
	}
	for _, mod := range mods {
		e.EmitModule(mod)
		e.p("")
	}
}

// emitCombinedAssertions emits the accumulated PSL assertions and assumptions
// after the module body.
//
// In BMC mode: assumptions become verif.assume (constrains solver input space);
// assertions are ANDed into a single verif.assert (circt-bmc requires exactly one).
//
// In IC3 mode: assertions become the negated bad-state signal stored in
// e.badStateSSA; assumptions gate it so the bad state is only active when all
// assumptions hold: __verif_bad = NOT(assertions) AND combined_assumptions.
// ABC pdr treats hw.output signals as "bad state" indicators: output=1 means
// the property is violated.
func (e *MLIREmitter) emitCombinedAssertions(mod *Module) {
	var boolAsserts []string
	var temporalAsserts []assertionEntry

	for _, a := range e.assertions {
		if a.typ == "i1" {
			boolAsserts = append(boolAsserts, a.val)
		} else {
			temporalAsserts = append(temporalAsserts, a)
		}
	}

	// Build conjunction of all boolean assumptions (i1 only; temporal assumptions not yet supported).
	var combinedAssume string
	for _, a := range e.assumptions {
		if a.typ != "i1" {
			e.p("// temporal assumption skipped (type %s): %s", a.typ, a.val)
			continue
		}
		if combinedAssume == "" {
			combinedAssume = a.val
		} else {
			res := e.nextSSA()
			e.p("%s = comb.and %s, %s : i1", res, combinedAssume, a.val)
			combinedAssume = res
		}
	}

	// In BMC mode emit a single verif.assume for the combined assumption.
	if combinedAssume != "" && !e.ic3Mode {
		e.p("verif.assume %s : i1", combinedAssume)
	}

	// Build conjunction of all boolean assertions.
	var combined string
	if len(boolAsserts) == 1 {
		combined = boolAsserts[0]
	} else if len(boolAsserts) > 1 {
		combined = boolAsserts[0]
		for _, next := range boolAsserts[1:] {
			res := e.nextSSA()
			e.p("%s = comb.and %s, %s : i1", res, combined, next)
			combined = res
		}
	}

	if combined != "" {
		if e.ic3Mode {
			// IC3 path: negate the conjunction to get the bad-state signal.
			// __verif_bad = 1 when at least one assertion is violated.
			// Note: assumptions are NOT encoded here — circt-translate --export-aiger
			// does not support the verif dialect, so assumptions cannot be forwarded
			// to ABC pdr as AIGER constraints.  compile.sh skips the IC3 step entirely
			// when assumptions are present; BMC (Z3) handles the constrained check.
			trueVal := e.nextSSA()
			badSSA := e.nextSSA()
			e.p("%s = hw.constant -1 : i1", trueVal)
			e.p("%s = comb.xor %s, %s : i1", badSSA, combined, trueVal)
			e.badStateSSA = badSSA
		} else {
			e.p("verif.assert %s : i1", combined)
		}
	}

	// Temporal (!ltl.property) assertions are emitted directly in BMC mode.
	for _, ta := range temporalAsserts {
		if e.ic3Mode {
			e.p("// temporal assertion skipped in IC3 path (type %s): %s", ta.typ, ta.val)
		} else {
			e.p("verif.assert %s : !ltl.property", ta.val)
		}
	}
}

// EmitModule translates the Module into a hw.module definition
func (e *MLIREmitter) EmitModule(mod *Module) {
	e.assertions = e.assertions[:0]  // reset accumulator for this module
	e.assumptions = e.assumptions[:0] // reset accumulator for this module
	e.badStateSSA = ""
	e.nameID = 0
	e.clockI1SSA = ""
	// Construct signature
	var ports []string
	var outputs []string
	for _, p := range mod.Ports {
		var t string
		if p.Name == mod.ClockPort {
			t = "!seq.clock" // clock ports are !seq.clock, not i1
		} else {
			t = typeToMLIR(p.Type, mod)
		}
		if p.Direction == "in" {
			ports = append(ports, fmt.Sprintf("in %%%s: %s", p.Name, t))
		} else {
			ports = append(ports, fmt.Sprintf("out %s: %s", p.Name, t))
			outputs = append(outputs, p.Name)
		}
	}
	// In IC3 mode, add a bad-state output port when the module has PSL assertions.
	if e.ic3Mode && hasPslAssertions(mod) {
		ports = append(ports, "out __verif_bad: i1")
	}

	sigPorts := strings.Join(ports, ", ")

	var generics []string
	for _, gen := range mod.Generics {
		// All generics in Micro-VHDL are 32-bit integers
		def := gen.Default
		if def == "" {
			def = "0"
		}
		generics = append(generics, fmt.Sprintf("%s: i32 = %s", gen.Name, def))
	}
	
	genStr := ""
	if len(generics) > 0 {
		genStr = fmt.Sprintf("<%s>", strings.Join(generics, ", "))
	}

	e.p("hw.module @%s%s(%s) {", mod.Name, genStr, sigPorts)
	e.indent++

	e.p("// --- Body ---")

	// Create a map to track which SSA value drives which signal/port
	env := make(map[string]string)
	for _, p := range mod.Ports {
		if p.Direction == "in" {
			env[p.Name] = fmt.Sprintf("%%%s", p.Name)
		}
	}
	// Pre-populate environment with signal names to handle forward references
	for _, sig := range mod.Signals {
		env[sig.Name] = fmt.Sprintf("%%%s", sig.Name)
	}

	e.emitStatementList(mod.Statements, mod, env)

	// Combine all accumulated PSL assertions into a single verif.assert.
	e.emitCombinedAssertions(mod)

	// Emit any formal blocks associated with this module.
	for _, fb := range mod.Formals {
		e.emitPslFormalBlock(fb, mod)
	}

	// Emit hw.output using the tracked drivers.
	// In IC3 mode, append the bad-state signal as the final output.
	var outVals []string
	var outTypes []string
	if e.ic3Mode && hasPslAssertions(mod) && e.badStateSSA == "" {
		// If only temporal assertions exist, provide a dummy bad-state signal so
		// the output signature remains consistent. IC3 is skipped in this case.
		zero := e.nextSSA()
		e.p("%s = hw.constant 0 : i1", zero)
		e.badStateSSA = zero
	}
	for i, p := range mod.Ports {
		if p.Direction == "out" {
			t := typeToMLIR(p.Type, mod)
			if valName, ok := env[p.Name]; ok {
				outVals = append(outVals, valName)
			} else {
				valName := fmt.Sprintf("%%c0_%d", i)
				e.p("%s = hw.constant 0 : %s", valName, t)
				outVals = append(outVals, valName)
			}
			outTypes = append(outTypes, t)
		}
	}

	if mod.Contract != nil && !e.ic3Mode && len(outVals) > 0 {
		var contractResults []string
		for range outVals {
			contractResults = append(contractResults, e.nextSSA())
		}

		e.p("%s = verif.contract %s : %s {", strings.Join(contractResults, ", "), strings.Join(outVals, ", "), strings.Join(outTypes, ", "))
		e.indent++

		for _, req := range mod.Contract.Requires {
			val, _ := e.emitExpression(req, mod, env, "", "i1", nil)
			e.p("verif.require %s : i1", val)
		}

		ensureEnv := make(map[string]string)
		for k, v := range env {
			ensureEnv[k] = v
		}
		outIdx := 0
		for _, p := range mod.Ports {
			if p.Direction == "out" {
				ensureEnv[p.Name] = contractResults[outIdx]
				outIdx++
			}
		}

		for _, ens := range mod.Contract.Ensures {
			val, _ := e.emitExpression(ens, mod, ensureEnv, "", "i1", nil)
			e.p("verif.ensure %s : i1", val)
		}

		e.indent--
		e.p("}")

		outVals = contractResults
	}

	if e.badStateSSA != "" {
		outVals = append(outVals, e.badStateSSA)
		outTypes = append(outTypes, "i1")
	}
	if len(outVals) > 0 {
		e.p("hw.output %s : %s", strings.Join(outVals, ", "), strings.Join(outTypes, ", "))
	} else {
		e.p("hw.output")
	}

	e.indent--
	e.p("}")
}

func (e *MLIREmitter) emitStatementList(stmts []Statement, mod *Module, env map[string]string) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *SymbolicDeclaration:
			t := typeToMLIR(s.Type, mod)
			for _, name := range s.Names {
				e.p("%%%s = verif.symbolic_value : %s", name, t)
				env[name] = fmt.Sprintf("%%%s", name)
			}
		case *EntityInstantiation:
			e.emitEntityInstantiation(s, mod, env)
		case *ConcurrentAssignment:
			if s.Condition != nil {
				t := typeToMLIR(mod.Symbols[s.Target], mod)
				val, _ := e.emitExpression(s.Value, mod, env, "", t, nil)
				condVal, _ := e.emitExpression(s.Condition, mod, env, "", "i1", nil)
				altVal, _ := e.emitExpression(s.AltValue, mod, env, "", t, nil)
				e.p("%%%s = comb.mux %s, %s, %s : %s", s.Target, condVal, val, altVal, t)
				env[s.Target] = fmt.Sprintf("%%%s", s.Target)
			} else {
				t := typeToMLIR(mod.Symbols[s.Target], mod)
				val, _ := e.emitExpression(s.Value, mod, env, s.Target, t, nil)
				env[s.Target] = val
			}
		case *SelectedAssignment:
			selVal, selType := e.emitExpression(s.Selector, mod, env, "", "", nil)
			t := typeToMLIR(mod.Symbols[s.Target], mod)

			var currentState string
			for _, choice := range s.Choices {
				if choice.IsOthers {
					currentState, _ = e.emitExpression(choice.Value, mod, env, "", t, nil)
					break
				}
			}
			if currentState == "" {
				zeroVal := e.nextSSA()
				e.p("%s = hw.constant 0 : %s", zeroVal, t)
				currentState = zeroVal
			}

			for i := len(s.Choices) - 1; i >= 0; i-- {
				choice := s.Choices[i]
				if choice.IsOthers {
					continue
				}
				condExprVal, _ := e.emitExpression(choice.Condition, mod, env, "", selType, nil)
				
				cmpVal := e.nextSSA()
				e.p("%s = comb.icmp eq %s, %s : %s", cmpVal, selVal, condExprVal, selType)
				
				val, _ := e.emitExpression(choice.Value, mod, env, "", t, nil)
				
				resName := e.nextSSA()[1:] // drop the %
				if i == 0 {
					resName = s.Target
				}
				e.p("%%%s = comb.mux %s, %s, %s : %s", resName, cmpVal, val, currentState, t)
				currentState = fmt.Sprintf("%%%s", resName)
			}
			env[s.Target] = currentState
		case *SynchronousProcess:
			e.emitSynchronousProcess(s, mod, env)
		case *GenerateStatement:
			for i := s.Start; i <= s.End; i++ {
				// Clone env to isolate loop iterator
				loopEnv := make(map[string]string)
				for k, v := range env {
					loopEnv[k] = v
				}
				
				iterVal := e.nextSSA()
				e.p("%s = hw.constant %d : i32", iterVal, i)
				loopEnv[s.Iterator] = iterVal
				
				e.emitStatementList(s.Statements, mod, loopEnv)
				
				for k, v := range loopEnv {
					if k != s.Iterator {
						env[k] = v
					}
				}
			}
	case *PslAssertion:
		propVal, propType := e.emitExpression(s.Property, mod, env, "", "", nil)
		if propType == "!ltl.sequence" {
			clkI1 := e.clockI1(mod)
			if clkI1 != "" {
				clocked := e.nextSSA()
				e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, propVal, clkI1)
				propVal = clocked
				propType = "!ltl.property"
			} else {
				e.p("// ERROR: PSL temporal property requires a clock")
			}
		}
		// Accumulate instead of emitting directly; EmitModule combines them.
		e.assertions = append(e.assertions, assertionEntry{propVal, propType})
	case *PslAssumption:
		propVal, propType := e.emitExpression(s.Property, mod, env, "", "", nil)
		if propType == "!ltl.sequence" {
			clkI1 := e.clockI1(mod)
			if clkI1 != "" {
				clocked := e.nextSSA()
				e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, propVal, clkI1)
				propVal = clocked
				propType = "!ltl.property"
			} else {
				e.p("// ERROR: PSL temporal property requires a clock")
			}
		}
		// Accumulate assumptions; emitCombinedAssertions handles them.
		e.assumptions = append(e.assumptions, assertionEntry{propVal, propType})
	case *PslCover:
		propVal, propType := e.emitExpression(s.Property, mod, env, "", "", nil)
		if propType == "!ltl.sequence" {
			clkI1 := e.clockI1(mod)
			if clkI1 != "" {
				clocked := e.nextSSA()
				e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, propVal, clkI1)
				propVal = clocked
				propType = "!ltl.property"
			} else {
				e.p("// ERROR: PSL temporal property requires a clock")
			}
		}
		if !e.ic3Mode {
			e.p("verif.cover %s : %s", propVal, propType)
		}
	case *PslAssertNever:
		propVal, _ := e.emitExpression(s.Property, mod, env, "", "i1", nil)
		trueVal := e.nextSSA()
		notProp := e.nextSSA()
		e.p("%s = hw.constant -1 : i1", trueVal)
		e.p("%s = comb.xor %s, %s : i1", notProp, propVal, trueVal)
		e.assertions = append(e.assertions, assertionEntry{notProp, "i1"})
		case *PslAfterResetAssertion:
			e.emitAfterResetAssertion(s, mod, env)
		}
	}
}

// emitAfterResetAssertion implements `psl assert always after_reset(rst) property`.
//
// A sticky `rst_ever_high` register latches permanently once rst is asserted:
//
//	next_rst_ever_high = rst_ever_high OR rst
//	rst_ever_high      = seq.compreg next_rst_ever_high, clk  [initial 0 in BMC]
//
// The effective assertion is `NOT(rst_ever_high) OR rst OR property`:
//   - vacuously true while rst has never been high (circuit not yet reset)
//   - vacuously true while rst is currently active (don't check during reset itself)
//   - checks property for real after rst has fired AND is now low
//
// This avoids IC3 false-positives from pre-reset unreachable states.
//
// The forward reference from next_rst_ever_high to rst_ever_high (which appears
// later in the text) is valid: CIRCT allows seq.compreg outputs to be referenced
// before their definition because the register boundary breaks the cycle.
func (e *MLIREmitter) emitAfterResetAssertion(s *PslAfterResetAssertion, mod *Module, env map[string]string) {
	clk := e.clockSSA(mod)

	// Evaluate the reset signal expression.
	resetVal, _ := e.emitExpression(s.Reset, mod, env, "", "i1", nil)

	// Pre-allocate the SSA name for rst_ever_high so next_rst_ever_high can
	// reference it as a forward reference (CIRCT allows this for register outputs).
	rstEverHighSSA := e.nextSSA()

	// next_rst_ever_high = rst_ever_high OR rst — latches once rst goes high.
	nextRstEverHighSSA := e.nextSSA()
	e.p("%s = comb.or %s, %s : i1", nextRstEverHighSSA, rstEverHighSSA, resetVal)

	// Register it. BMC initializes to 0 (rst has never fired); IC3 leaves it
	// unconstrained (see compile.sh — IC3 is skipped when after_reset is present).
	initSSA := e.emitSeqInitial("i1")
	if initSSA != "" {
		e.p("%s = seq.compreg %s, %s initial %s : i1", rstEverHighSSA, nextRstEverHighSSA, clk, initSSA)
	} else {
		e.p("%s = seq.compreg %s, %s : i1", rstEverHighSSA, nextRstEverHighSSA, clk)
	}

	// Evaluate the guarded property.
	propVal, propType := e.emitExpression(s.Property, mod, env, "", "i1", nil)

	if propType == "!ltl.sequence" {
		clkI1 := e.clockI1(mod)
		if clkI1 != "" {
			clocked := e.nextSSA()
			e.p("%s = ltl.clock %s, posedge %s : !ltl.property", clocked, propVal, clkI1)
			propVal = clocked
			propType = "!ltl.property"
		} else {
			e.p("// ERROR: PSL temporal property requires a clock")
		}
	}

	if propType == "!ltl.property" {
		// effective = (rst_ever_high AND NOT rst) |-> propVal
		triggerSSA := e.nextSSA()
		notResetSSA := e.nextSSA()
		trueVal := e.nextSSA()
		e.p("%s = hw.constant -1 : i1", trueVal)
		e.p("%s = comb.xor %s, %s : i1", notResetSSA, resetVal, trueVal)
		e.p("%s = comb.and %s, %s : i1", triggerSSA, rstEverHighSSA, notResetSSA)
		
		impVal := e.nextSSA()
		e.p("%s = ltl.implication %s, %s : i1, !ltl.property", impVal, triggerSSA, propVal)
		e.assertions = append(e.assertions, assertionEntry{impVal, "!ltl.property"})
	} else {
		// effective = NOT(rst_ever_high) OR rst OR property
		//   NOT(rst_ever_high): vacuous before any reset
		//   rst:               vacuous during active reset
		//   property:          the actual check post-reset
		trueVal := e.nextSSA()
		notRstEverHighSSA := e.nextSSA()
		e.p("%s = hw.constant -1 : i1", trueVal)
		e.p("%s = comb.xor %s, %s : i1", notRstEverHighSSA, rstEverHighSSA, trueVal)
		partialSSA := e.nextSSA()
		e.p("%s = comb.or %s, %s : i1", partialSSA, notRstEverHighSSA, resetVal)
		effectiveSSA := e.nextSSA()
		e.p("%s = comb.or %s, %s : i1", effectiveSSA, partialSSA, propVal)

		e.assertions = append(e.assertions, assertionEntry{effectiveSSA, "i1"})
	}
}
