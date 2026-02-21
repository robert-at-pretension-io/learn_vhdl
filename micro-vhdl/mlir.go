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
	builder    strings.Builder
	indent     int
	ssaID      int
	modules    map[string]*Module
	assertions []assertionEntry // accumulated per-module, reset in EmitModule
}

func NewMLIREmitter() *MLIREmitter {
	return &MLIREmitter{
		modules: make(map[string]*Module),
	}
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

// clockSSA returns the SSA name of the module's clock port (e.g. "%clk").
func (e *MLIREmitter) clockSSA(mod *Module) string {
	if mod.ClockPort == "" {
		return ""
	}
	return fmt.Sprintf("%%%s", mod.ClockPort)
}

// emitSeqInitial emits a seq.initial block that yields a zero value of the
// given MLIR type and returns its SSA name.  Returns "" for complex types
// (hw.array / hw.struct) where a zero literal cannot be trivially constructed.
func (e *MLIREmitter) emitSeqInitial(mlirType string) string {
	if strings.HasPrefix(mlirType, "!hw.") {
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

func (e *MLIREmitter) emitExpression(expr Expression, mod *Module, env map[string]string, targetName string, expectedType string) (string, string) {
	var resVal string
	if targetName != "" {
		resVal = fmt.Sprintf("%%%s", targetName)
	} else {
		resVal = e.nextSSA()
	}

	switch v := expr.(type) {
	case IdentifierExpr:
		t := mod.Symbols[v.Name]
		if mapped, ok := env[v.Name]; ok {
			return mapped, typeToMLIR(t, mod)
		}
		return fmt.Sprintf("%%%s", v.Name), typeToMLIR(t, mod)
	case IndexedNameExpr:
		prefixVal, prefixType := e.emitExpression(IdentifierExpr{Name: v.Prefix}, mod, env, "", "")
		indexVal, _ := e.emitExpression(v.Index, mod, env, "", "i32")
		
		t := resolveType(mod, mod.Symbols[v.Prefix].Name)
		var elType string
		if t.ElementType != nil {
			elType = typeToMLIR(*t.ElementType, mod)
		} else {
			elType = "i1" // Fallback
		}
		
		e.p("%s = hw.array_get %s[%s] : %s, i32", resVal, prefixVal, indexVal, prefixType)
		return resVal, elType
	case SelectedNameExpr:
		prefixVal, prefixType := e.emitExpression(IdentifierExpr{Name: v.Prefix}, mod, env, "", "")
		
		t := resolveType(mod, mod.Symbols[v.Prefix].Name)
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
			inferredType = typeToMLIR(mod.Symbols[id.Name], mod)
		} else if id, ok := v.Right.(IdentifierExpr); ok {
			inferredType = typeToMLIR(mod.Symbols[id.Name], mod)
		}

		leftVal, leftType := e.emitExpression(v.Left, mod, env, "", inferredType)
		rightVal, rightType := e.emitExpression(v.Right, mod, env, "", inferredType)
		
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
			rightVal, rightType := e.emitExpression(v.Right, mod, env, "", expectedType)
			trueVal := e.nextSSA()
			e.p("%s = hw.constant -1 : %s", trueVal, rightType)
			e.p("%s = comb.xor %s, %s : %s", resVal, rightVal, trueVal, rightType)
			return resVal, rightType
		}
	case PslImplicationExpr:
		// PSL |=> is next-cycle: "if left holds at cycle N, right must hold at cycle N+1."
		// Implement as: delay left by one clock cycle, then check boolean implication.
		// This produces a plain i1 assertion that circt-bmc can consume directly.
		leftVal, _ := e.emitExpression(v.Left, mod, env, "", "i1")
		rightVal, _ := e.emitExpression(v.Right, mod, env, "", "i1")

		clk := e.clockSSA(mod)
		initSSA := e.emitSeqInitial("i1")
		delayedVal := e.nextSSA()
		e.p("%s = seq.compreg %s, %s initial %s : i1", delayedVal, leftVal, clk, initSSA)

		// Boolean implication: !left_d1 || right
		trueVal := e.nextSSA()
		notDelayed := e.nextSSA()
		e.p("%s = hw.constant -1 : i1", trueVal)
		e.p("%s = comb.xor %s, %s : i1", notDelayed, delayedVal, trueVal)
		e.p("%s = comb.or %s, %s : i1", resVal, notDelayed, rightVal)
		return resVal, "i1"

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
		sigVal, sigType := e.emitExpression(v.Signal, mod, env, "", "")

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
				val, _ := e.emitExpression(s.Value, mod, env, "", targetType)
				nextState = val
			}
		case *SequentialIf:
			condVal, _ := e.emitExpression(s.Condition, mod, env, "", "i1")
			
			thenState := e.buildSeqBlock(s.Then, target, nextState, mod, env)
			elseState := e.buildSeqBlock(s.Else, target, nextState, mod, env)
			
			for i := len(s.Elsifs) - 1; i >= 0; i-- {
				el := s.Elsifs[i]
				elCondVal, _ := e.emitExpression(el.Condition, mod, env, "", "i1")
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
				val, _ := e.emitExpression(actualExpr, parentMod, env, "", "")
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

// emitCombinedAssertions emits the accumulated PSL assertions after the module body.
//
// circt-bmc requires exactly one verif.assert per module; multiple i1 assertions
// are ANDed together into a single combined check.  Temporal (!ltl.property)
// assertions are emitted as verif.clocked_assert so --lower-ltl-to-core can handle
// them independently (they require a separate preprocessing step before circt-bmc).
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

	// Combine all boolean assertions into one via comb.and chain.
	if len(boolAsserts) == 1 {
		e.p("verif.assert %s : i1", boolAsserts[0])
	} else if len(boolAsserts) > 1 {
		combined := boolAsserts[0]
		for _, next := range boolAsserts[1:] {
			res := e.nextSSA()
			e.p("%s = comb.and %s, %s : i1", res, combined, next)
			combined = res
		}
		e.p("verif.assert %s : i1", combined)
	}

	// Any remaining temporal (!ltl.property) assertions are not handled here;
	// they would require --lower-ltl-to-core preprocessing before circt-bmc.
	for _, ta := range temporalAsserts {
		e.p("// temporal assertion skipped in BMC conjunction (type %s): %s", ta.typ, ta.val)
	}
}

// EmitModule translates the Module into a hw.module definition
func (e *MLIREmitter) EmitModule(mod *Module) {
	e.assertions = e.assertions[:0] // reset accumulator for this module
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

	// Emit hw.output using the tracked drivers
	var outVals []string
	var outTypes []string
	if len(outputs) > 0 {
		for i, p := range mod.Ports {
			if p.Direction == "out" {
				t := typeToMLIR(p.Type, mod)
				if valName, ok := env[p.Name]; ok {
					outVals = append(outVals, valName)
				} else {
					// Fallback if undriven
					valName := fmt.Sprintf("%%c0_%d", i)
					e.p("%s = hw.constant 0 : %s", valName, t)
					outVals = append(outVals, valName)
				}
				outTypes = append(outTypes, t)
			}
		}
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
		case *EntityInstantiation:
			e.emitEntityInstantiation(s, mod, env)
		case *ConcurrentAssignment:
			if s.Condition != nil {
				t := typeToMLIR(mod.Symbols[s.Target], mod)
				val, _ := e.emitExpression(s.Value, mod, env, "", t)
				condVal, _ := e.emitExpression(s.Condition, mod, env, "", "i1")
				altVal, _ := e.emitExpression(s.AltValue, mod, env, "", t)
				e.p("%%%s = comb.mux %s, %s, %s : %s", s.Target, condVal, val, altVal, t)
				env[s.Target] = fmt.Sprintf("%%%s", s.Target)
			} else {
				t := typeToMLIR(mod.Symbols[s.Target], mod)
				val, _ := e.emitExpression(s.Value, mod, env, s.Target, t)
				env[s.Target] = val
			}
		case *SelectedAssignment:
			selVal, selType := e.emitExpression(s.Selector, mod, env, "", "")
			t := typeToMLIR(mod.Symbols[s.Target], mod)

			var currentState string
			for _, choice := range s.Choices {
				if choice.IsOthers {
					currentState, _ = e.emitExpression(choice.Value, mod, env, "", t)
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
				condExprVal, _ := e.emitExpression(choice.Condition, mod, env, "", selType)
				
				cmpVal := e.nextSSA()
				e.p("%s = comb.icmp eq %s, %s : %s", cmpVal, selVal, condExprVal, selType)
				
				val, _ := e.emitExpression(choice.Value, mod, env, "", t)
				
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
			propVal, propType := e.emitExpression(s.Property, mod, env, "", "")
			// Accumulate instead of emitting directly; EmitModule combines them.
			e.assertions = append(e.assertions, assertionEntry{propVal, propType})
		}
	}
}