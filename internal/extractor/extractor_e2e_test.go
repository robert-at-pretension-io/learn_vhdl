package extractor

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractorE2EPortsSignalsProcesses(t *testing.T) {
	fixture := fixturePath(t, "ports_signals_process.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertPort(t, facts.Ports, "clk", "in", "std_logic")
	assertPort(t, facts.Ports, "rst_n", "in", "std_logic")
	assertPort(t, facts.Ports, "din", "in", "std_logic_vector(7 downto 0)")
	assertPort(t, facts.Ports, "dout", "out", "std_logic_vector(7 downto 0)")

	if len(facts.Ports) != 4 {
		t.Fatalf("expected 4 ports, got %d: %#v", len(facts.Ports), facts.Ports)
	}

	assertSignal(t, facts.Signals, "a", "std_logic_vector(7 downto 0)")
	assertSignal(t, facts.Signals, "b", "std_logic_vector(7 downto 0)")
	assertSignal(t, facts.Signals, "flag", "std_logic")

	proc := findProcess(t, facts.Processes, "p_sync")
	if len(proc.SensitivityList) != 2 {
		t.Fatalf("expected 2 sensitivity signals, got %d: %#v", len(proc.SensitivityList), proc.SensitivityList)
	}
	assertContains(t, proc.SensitivityList, "clk")
	assertContains(t, proc.SensitivityList, "rst_n")

	fn := findFunction(t, facts.Functions, "add")
	if fn.ReturnType != "integer" {
		t.Fatalf("expected return type integer, got %q", fn.ReturnType)
	}
	if len(fn.Parameters) != 2 {
		t.Fatalf("expected 2 function parameters, got %d: %#v", len(fn.Parameters), fn.Parameters)
	}
	assertParam(t, fn.Parameters, "a", "", "integer", "")
	assertParam(t, fn.Parameters, "b", "", "integer", "")

	procDecl := findProcedure(t, facts.Procedures, "touch")
	if len(procDecl.Parameters) != 1 {
		t.Fatalf("expected 1 procedure parameter, got %d: %#v", len(procDecl.Parameters), procDecl.Parameters)
	}
	assertParam(t, procDecl.Parameters, "s", "in", "std_logic", "signal")
}

func TestExtractorE2ETypesAndPackages(t *testing.T) {
	fixture := fixturePath(t, "types_and_packages.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if _, ok := findPackageByName(facts.Packages, "util"); !ok {
		t.Fatalf("expected package util, got %#v", facts.Packages)
	}

	width := findConstant(t, facts.ConstantDecls, "WIDTH")
	if width.Type != "integer" {
		t.Fatalf("constant WIDTH type: expected integer, got %q", width.Type)
	}
	if width.Value != "8" {
		t.Fatalf("constant WIDTH value: expected 8, got %q", width.Value)
	}
	if width.InPackage != "util" {
		t.Fatalf("constant WIDTH package: expected util, got %q", width.InPackage)
	}

	state := mustFindType(t, facts.Types, "state_t")
	if state.Kind != "enum" {
		t.Fatalf("state_t kind: expected enum, got %q", state.Kind)
	}
	assertContains(t, state.EnumLiterals, "IDLE")
	assertContains(t, state.EnumLiterals, "RUN")

	rec := mustFindType(t, facts.Types, "rec_t")
	if rec.Kind != "record" {
		t.Fatalf("rec_t kind: expected record, got %q", rec.Kind)
	}
	assertRecordField(t, rec.Fields, "a", "std_logic")
	assertRecordField(t, rec.Fields, "b", "std_logic_vector(WIDTH-1 downto 0)")

	arr := mustFindType(t, facts.Types, "arr_t")
	if arr.Kind != "array" {
		t.Fatalf("arr_t kind: expected array, got %q", arr.Kind)
	}
	if arr.ElementType != "std_logic" {
		t.Fatalf("arr_t element type: expected std_logic, got %q", arr.ElementType)
	}

	idx := mustFindSubtype(t, facts.Subtypes, "idx_t")
	if idx.BaseType != "integer" {
		t.Fatalf("idx_t base type: expected integer, got %q", idx.BaseType)
	}

	inc := findFunctionWithPackage(t, facts.Functions, "inc", "util")
	if inc.ReturnType != "integer" {
		t.Fatalf("inc return type: expected integer, got %q", inc.ReturnType)
	}

	findProcedureWithPackage(t, facts.Procedures, "poke", "util")
}

func TestExtractorE2EAssignmentsInstancesGenerates(t *testing.T) {
	fixture := fixturePath(t, "assignments_instances_generates.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if !hasDependencyKind(facts.Dependencies, "library", "ieee") {
		t.Fatalf("expected library dependency on ieee, got %#v", facts.Dependencies)
	}
	if !hasDependencyKind(facts.Dependencies, "use", "ieee.std_logic_1164") {
		t.Fatalf("expected use dependency on ieee.std_logic_1164.all, got %#v", facts.Dependencies)
	}
	if !hasDependencyKind(facts.Dependencies, "instantiation", "work.child") {
		t.Fatalf("expected instantiation dependency on work.child, got %#v", facts.Dependencies)
	}

	cfg, ok := findConfigurationByName(facts.Configurations, "cfg_top")
	if !ok {
		t.Fatalf("expected configuration cfg_top, got %#v", facts.Configurations)
	}
	if cfg.EntityName != "top" {
		t.Fatalf("cfg_top entity: expected top, got %q", cfg.EntityName)
	}

	u1 := mustFindInstance(t, facts.Instances, "u1")
	if u1.Target != "work.child" {
		t.Fatalf("u1 target: expected work.child, got %q", u1.Target)
	}
	assertMap(t, u1.PortMap, "clk", "clk")
	assertMap(t, u1.PortMap, "d", "a")
	assertMap(t, u1.PortMap, "q", "b")

	assertConcurrentAssignment(t, facts.ConcurrentAssignments, "a", "simple", false)
	assertConcurrentAssignment(t, facts.ConcurrentAssignments, "c", "conditional", false)
	assertConcurrentAssignment(t, facts.ConcurrentAssignments, "vec", "selected", false)

	genFor := mustFindGenerate(t, facts.Generates, "gen_for")
	if genFor.Kind != "for" {
		t.Fatalf("gen_for kind: expected for, got %q", genFor.Kind)
	}
	if genFor.LoopVar != "i" {
		t.Fatalf("gen_for loop var: expected i, got %q", genFor.LoopVar)
	}
	if genFor.RangeLow != "0" || genFor.RangeHigh != "1" || genFor.RangeDir != "to" {
		t.Fatalf("gen_for range: expected 0 to 1, got %q %q %q", genFor.RangeLow, genFor.RangeDir, genFor.RangeHigh)
	}

	genIf := mustFindGenerate(t, facts.Generates, "gen_if")
	if genIf.Kind != "if" {
		t.Fatalf("gen_if kind: expected if, got %q", genIf.Kind)
	}
	if !strings.Contains(genIf.Condition, "a") {
		t.Fatalf("gen_if condition: expected to contain a, got %q", genIf.Condition)
	}

	genCase := mustFindGenerate(t, facts.Generates, "gen_case")
	if genCase.Kind != "case" {
		t.Fatalf("gen_case kind: expected case, got %q", genCase.Kind)
	}
	if strings.TrimSpace(genCase.Condition) != "sel" {
		t.Fatalf("gen_case condition: expected sel, got %q", genCase.Condition)
	}

	assertSignalInScope(t, facts.Signals, "gsig", "rtl.gen_for")
	assertSignalInScope(t, facts.Signals, "ifsig", "rtl.gen_if")
}

func TestExtractorE2ECDCARithCompare(t *testing.T) {
	fixture := fixturePath(t, "cdc_arith_compare.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if !hasClockDomainForProc(facts.ClockDomains, "clk_a", "p_a") {
		t.Fatalf("expected clock domain clk_a for p_a, got %#v", facts.ClockDomains)
	}
	if !hasClockDomainForProc(facts.ClockDomains, "clk_b", "p_b1") {
		t.Fatalf("expected clock domain clk_b for p_b1, got %#v", facts.ClockDomains)
	}

	cross := findCDCCrossing(t, facts.CDCCrossings, "async2")
	if !cross.IsSynchronized || cross.SyncStages < 2 {
		t.Fatalf("expected async2 to be synchronized with >=2 stages, got %#v", cross)
	}
	if !cross.IsMultiBit {
		t.Fatalf("expected async2 to be multi-bit, got %#v", cross)
	}

	op := findArithmeticOp(t, facts.ArithmeticOps, "*")
	if !op.IsGuarded || op.GuardSignal != "en" {
		t.Fatalf("expected guarded arithmetic op on en, got %#v", op)
	}

	if !hasComparison(facts.Comparisons, "=", "7") {
		t.Fatalf("expected comparison with literal 7, got %#v", facts.Comparisons)
	}

	caseStmt := findCaseStatement(t, facts.CaseStatements, "cnt")
	if !caseStmt.HasOthers {
		t.Fatalf("expected case statement with others, got %#v", caseStmt)
	}
}

func TestExtractorE2EExternalPhysicalAndBlock(t *testing.T) {
	fixture := fixturePath(t, "external_physical_block.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	blkAssign := mustFindConcurrentAssignment(t, facts.ConcurrentAssignments, "bx")
	if !hasReadSignal(blkAssign.ReadSignals, "a") {
		t.Fatalf("expected bx assignment to read a, got %#v", blkAssign.ReadSignals)
	}

	if hasSignalUsage(facts.SignalUsages, "ns") {
		t.Fatalf("expected no signal usage for physical unit ns, got %#v", facts.SignalUsages)
	}

	proc := mustFindProcess(t, facts.Processes, "p_ext")
	assertNoSignal(t, proc.ReadSignals, "tb")
	assertNoSignal(t, proc.ReadSignals, "dut")
	assertNoSignal(t, proc.ReadSignals, "ext_sig")
}

func TestExtractorE2EProtectedPkgInstantiationAndConfigSpec(t *testing.T) {
	fixture := fixturePath(t, "protected_pkg_inst_cfg.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if !hasProtectedType(facts.Types, "prot_t") {
		t.Fatalf("expected protected type prot_t, got %#v", facts.Types)
	}

	if !hasDependencyKind(facts.Dependencies, "package_instantiation", "work.base_pkg") {
		t.Fatalf("expected package instantiation dependency on work.base_pkg, got %#v", facts.Dependencies)
	}
	if !hasDependencyKind(facts.Dependencies, "configuration_specification", "work.child") {
		t.Fatalf("expected configuration specification dependency on work.child, got %#v", facts.Dependencies)
	}
}

func TestExtractorE2EGenericsComponentsAssociations(t *testing.T) {
	fixture := fixturePath(t, "generics_components_associations.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(facts.UseClauses) == 0 || len(facts.LibraryClauses) == 0 || len(facts.ContextClauses) == 0 {
		t.Fatalf("expected use/library/context clauses, got %#v %#v %#v", facts.UseClauses, facts.LibraryClauses, facts.ContextClauses)
	}

	genTop := findEntityE2E(t, facts.Entities, "gen_top")
	if !hasGeneric(genTop.Generics, "WIDTH", "constant") {
		t.Fatalf("expected generic WIDTH on entity gen_top, got %#v", genTop.Generics)
	}
	if !hasGeneric(genTop.Generics, "MODE", "constant") {
		t.Fatalf("expected generic MODE on entity gen_top, got %#v", genTop.Generics)
	}

	comp := findComponentE2E(t, facts.Components, "child")
	if len(comp.Ports) == 0 || len(comp.Generics) == 0 {
		t.Fatalf("expected component child with ports/generics, got %#v", comp)
	}

	inst := findInstanceE2E(t, facts.Instances, "u_child")
	if len(inst.Associations) == 0 {
		t.Fatalf("expected associations on u_child, got %#v", inst)
	}
	if !hasAssociationKind(inst.Associations, "generic", "G_WIDTH") {
		t.Fatalf("expected generic association for G_WIDTH, got %#v", inst.Associations)
	}
	if !hasAssociationActualKind(inst.Associations, "port", "name") {
		t.Fatalf("expected port association with name actual, got %#v", inst.Associations)
	}

	instPos := findInstanceE2E(t, facts.Instances, "u_child_pos")
	if !hasAssociationActualKind(instPos.Associations, "port", "open") {
		t.Fatalf("expected positional port association with open, got %#v", instPos.Associations)
	}
}

func TestExtractorE2EProcessVarsCallsWaits(t *testing.T) {
	fixture := fixturePath(t, "process_vars_calls_waits.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	proc := findProcess(t, facts.Processes, "p_call")
	if !hasVariable(proc.Variables, "v", "integer") {
		t.Fatalf("expected variable v: integer, got %#v", proc.Variables)
	}
	if len(proc.ProcedureCalls) == 0 || proc.ProcedureCalls[0].Name != "poke" {
		t.Fatalf("expected procedure call poke, got %#v", proc.ProcedureCalls)
	}
	if len(proc.FunctionCalls) == 0 || proc.FunctionCalls[0].Name != "f" {
		t.Fatalf("expected function call f, got %#v", proc.FunctionCalls)
	}
	if len(proc.WaitStatements) == 0 || proc.WaitStatements[0].UntilExpr == "" {
		t.Fatalf("expected wait until with condition, got %#v", proc.WaitStatements)
	}
}

func TestExtractorE2EFunctionCallsAndArrayIndex(t *testing.T) {
	fixture := fixturePath(t, "process_calls_arrays.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	proc := findProcess(t, facts.Processes, "p1")
	if !hasFunctionCall(proc.FunctionCalls, "p.f") {
		t.Fatalf("expected function call p.f, got %#v", proc.FunctionCalls)
	}
	if !hasFunctionCallArg(proc.FunctionCalls, "p.f", "arr(1)") {
		t.Fatalf("expected p.f args to include arr(1), got %#v", proc.FunctionCalls)
	}
	if hasFunctionCall(proc.FunctionCalls, "arr") {
		t.Fatalf("did not expect array index arr(1) to be treated as function call, got %#v", proc.FunctionCalls)
	}
}

func TestExtractorE2EWaitOnForNamedArgs(t *testing.T) {
	fixture := fixturePath(t, "process_calls_named_args.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	proc := findProcess(t, facts.Processes, "p_waits")
	if !hasFunctionCall(proc.FunctionCalls, "g") {
		t.Fatalf("expected function call g, got %#v", proc.FunctionCalls)
	}
	if !hasFunctionCallArg(proc.FunctionCalls, "g", "y => b") || !hasFunctionCallArg(proc.FunctionCalls, "g", "x => a") {
		t.Fatalf("expected named args in g call, got %#v", proc.FunctionCalls)
	}

	onWait, ok := findWaitWithOn(proc.WaitStatements)
	if !ok {
		t.Fatalf("expected wait on statement, got %#v", proc.WaitStatements)
	}
	assertContains(t, onWait.OnSignals, "a")
	assertContains(t, onWait.OnSignals, "b")

	forWait, ok := findWaitWithFor(proc.WaitStatements)
	if !ok {
		t.Fatalf("expected wait for statement, got %#v", proc.WaitStatements)
	}
	if !strings.Contains(forWait.ForExpr, "10") {
		t.Fatalf("expected wait for to contain 10, got %#v", forWait)
	}
}

func TestExtractorE2EFunctionCallsNestedArgs(t *testing.T) {
	fixture := fixturePath(t, "process_calls_nested_args.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	proc := findProcess(t, facts.Processes, "p_nested")
	if !hasFunctionCall(proc.FunctionCalls, "math_pkg.f") {
		t.Fatalf("expected function call math_pkg.f, got %#v", proc.FunctionCalls)
	}
	if !hasFunctionCall(proc.FunctionCalls, "g") || !hasFunctionCall(proc.FunctionCalls, "h") {
		t.Fatalf("expected nested calls g and h, got %#v", proc.FunctionCalls)
	}
	if !hasFunctionCallArg(proc.FunctionCalls, "math_pkg.f", "a => g(b)") {
		t.Fatalf("expected math_pkg.f args to include a => g(b), got %#v", proc.FunctionCalls)
	}
	if !hasFunctionCallArg(proc.FunctionCalls, "math_pkg.f", "c => h(1)") {
		t.Fatalf("expected math_pkg.f args to include c => h(1), got %#v", proc.FunctionCalls)
	}
}

func TestExtractorE2EContextPSLWaitAll(t *testing.T) {
	fixture := fixturePath(t, "context_psl_wait_all.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if !hasDependencyKind(facts.Dependencies, "context", "ieee.std_logic_1164") {
		t.Fatalf("expected context dependency ieee.std_logic_1164, got %#v", facts.Dependencies)
	}

	pWait := mustFindProcess(t, facts.Processes, "p_wait")
	if !pWait.HasWait {
		t.Fatalf("expected p_wait to have wait statement")
	}
	if pWait.IsCombinational {
		t.Fatalf("expected p_wait to be non-combinational")
	}

	pAll := mustFindProcess(t, facts.Processes, "p_all")
	if len(pAll.SensitivityList) != 1 || strings.ToLower(pAll.SensitivityList[0]) != "all" {
		t.Fatalf("expected p_all sensitivity list to be all, got %#v", pAll.SensitivityList)
	}
}

func TestExtractorE2EAnonTypesAndSubprogramInstantiation(t *testing.T) {
	fixture := fixturePath(t, "anon_types_and_subprogram_inst.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	portA := mustFindPort(t, facts.Ports, "a")
	if !strings.Contains(strings.ToLower(portA.Type), "type is private") {
		t.Fatalf("expected port a to use anonymous type, got %q", portA.Type)
	}

	portB := mustFindPort(t, facts.Ports, "b")
	if !strings.Contains(strings.ToLower(portB.Type), "type is <>") {
		t.Fatalf("expected port b to use anonymous type, got %q", portB.Type)
	}

	portC := mustFindPort(t, facts.Ports, "c")
	if portC.Direction != "linkage" {
		t.Fatalf("expected port c direction linkage, got %q", portC.Direction)
	}

	if !hasDependencyKind(facts.Dependencies, "subprogram_instantiation", "work.f1") {
		t.Fatalf("expected subprogram instantiation dependency on work.f1, got %#v", facts.Dependencies)
	}
	if !hasDependencyKind(facts.Dependencies, "subprogram_instantiation", "work.p1") {
		t.Fatalf("expected subprogram instantiation dependency on work.p1, got %#v", facts.Dependencies)
	}
}

func TestExtractorE2ETrailingSemicolonAndEventClock(t *testing.T) {
	fixture := fixturePath(t, "trailing_semicolon_and_event.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// Trailing semicolon in port list should still extract ports.
	assertPort(t, facts.Ports, "a", "in", "std_logic")
	assertPort(t, facts.Ports, "b", "out", "std_logic")

	pEvt := mustFindProcess(t, facts.Processes, "p_evt")
	if !pEvt.IsSequential || pEvt.ClockSignal != "clk" {
		t.Fatalf("expected p_evt to be sequential with clk, got %#v", pEvt)
	}
	if !pEvt.HasReset || pEvt.ResetSignal != "rst" {
		t.Fatalf("expected p_evt to detect reset rst, got %#v", pEvt)
	}
}

func TestExtractorE2EForceReleaseOpenFile(t *testing.T) {
	fixture := fixturePath(t, "force_release_open_file.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if hasSignalNamed(facts.Signals, "f") {
		t.Fatalf("expected file declaration not to be treated as signal, got %#v", facts.Signals)
	}

	if countAssignmentsTo(facts.ConcurrentAssignments, "x") < 3 {
		t.Fatalf("expected multiple concurrent assignments to x, got %#v", facts.ConcurrentAssignments)
	}

	u1 := mustFindInstance(t, facts.Instances, "u1")
	if actual, ok := u1.PortMap["d"]; !ok || actual != "open" {
		t.Fatalf("expected port d mapped to open, got %#v", u1.PortMap)
	}
	if hasSignalUsage(facts.SignalUsages, "open") {
		t.Fatalf("expected no signal usage for open, got %#v", facts.SignalUsages)
	}
}

func TestExtractorE2EPhysicalType(t *testing.T) {
	fixture := fixturePath(t, "physical_type.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	td := mustFindType(t, facts.Types, "time_t")
	if td.Kind != "physical" {
		t.Fatalf("expected physical type time_t, got %#v", td)
	}
	if td.BaseUnit != "ns" {
		t.Fatalf("expected base unit ns, got %#v", td)
	}
}

func TestExtractorE2ECDCSyncChain(t *testing.T) {
	fixture := fixturePath(t, "cdc_sync_chain.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	cross := findCDCCrossing(t, facts.CDCCrossings, "async_sig")
	if !cross.IsSynchronized || cross.SyncStages < 2 {
		t.Fatalf("expected async_sig to be synchronized with >=2 stages, got %#v", cross)
	}
}

func TestExtractorE2EViewGroupAttributeMisc(t *testing.T) {
	fixture := fixturePath(t, "misc_2019_view_group_attr.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if _, ok := findPackageByName(facts.Packages, "misc_2019"); !ok {
		t.Fatalf("expected package misc_2019, got %#v", facts.Packages)
	}
}

func TestExtractorE2EBasedLiteralAggregate(t *testing.T) {
	fixture := fixturePath(t, "based_literal_aggregate.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if countAssignmentsTo(facts.ConcurrentAssignments, "s") != 1 {
		t.Fatalf("expected one assignment to s, got %#v", facts.ConcurrentAssignments)
	}

	if hasSignalUsage(facts.SignalUsages, "FF") {
		t.Fatalf("expected no signal usage for based literal FF, got %#v", facts.SignalUsages)
	}
}

func TestExtractorE2EPackageGeneric2008(t *testing.T) {
	fixture := fixturePath(t, "package_generic_2008.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if _, ok := findPackageByName(facts.Packages, "gen_pkg"); !ok {
		t.Fatalf("expected package gen_pkg, got %#v", facts.Packages)
	}
	if !hasTypeName(facts.Types, "arr_t") {
		t.Fatalf("expected type arr_t, got %#v", facts.Types)
	}
}

func TestExtractorE2EEdgeConstructs2008_2019(t *testing.T) {
	fixture := fixturePath(t, "edge_constructs_2008_2019.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	cfg, ok := findConfigurationByName(facts.Configurations, "cfg_edge")
	if !ok {
		t.Fatalf("expected configuration cfg_edge, got %#v", facts.Configurations)
	}
	if cfg.EntityName != "edge_top" {
		t.Fatalf("cfg_edge entity: expected edge_top, got %q", cfg.EntityName)
	}

	genFor := mustFindGenerate(t, facts.Generates, "gen_for")
	if genFor.Kind != "for" {
		t.Fatalf("gen_for kind: expected for, got %q", genFor.Kind)
	}

	caseStmt := findCaseStatement(t, facts.CaseStatements, "s")
	if !caseStmt.HasOthers {
		t.Fatalf("expected matching case to have others, got %#v", caseStmt)
	}

	binding := findConfigBinding(t, cfg.Bindings, "u1")
	if binding.ComponentName != "child" {
		t.Fatalf("binding component: expected child, got %q", binding.ComponentName)
	}
	if binding.TargetEntity != "work.child" || binding.TargetArch != "rtl" {
		t.Fatalf("binding target: expected work.child(rtl), got %q(%q)", binding.TargetEntity, binding.TargetArch)
	}
	if !hasScope(binding.ScopePath, "rtl") || !hasScope(binding.ScopePath, "gen_for(0)") {
		t.Fatalf("binding scope: expected rtl -> gen_for(0), got %#v", binding.ScopePath)
	}

	disc := mustFindDisconnection(t, facts.Disconnections, "all")
	if disc.Type != "std_logic" || disc.Time != "5 ns" {
		t.Fatalf("disconnection: expected std_logic after 5 ns, got %#v", disc)
	}

	if !hasSignalUsageReadInPSL(facts.SignalUsages, "a") {
		t.Fatalf("expected PSL to read a, got %#v", facts.SignalUsages)
	}
	if hasSignalUsage(facts.SignalUsages, "next") {
		t.Fatalf("expected no signal usage for PSL next, got %#v", facts.SignalUsages)
	}
	if hasSignalUsage(facts.SignalUsages, "ns") {
		t.Fatalf("expected no signal usage for physical unit ns, got %#v", facts.SignalUsages)
	}
}

func TestExtractorE2ENamesAndAttributes(t *testing.T) {
	fixture := fixturePath(t, "names_and_attributes.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	proc := findProcess(t, facts.Processes, "p_read")
	assertContains(t, proc.AssignedSignals, "r")
	assertContains(t, proc.AssignedSignals, "s")
	assertContains(t, proc.ReadSignals, "a")
	assertContains(t, proc.ReadSignals, "r")
	assertNoSignal(t, proc.ReadSignals, "length")
	assertNoSignal(t, proc.ReadSignals, "ext_sig")

	if hasSignalUsage(facts.SignalUsages, "ns") {
		t.Fatalf("expected no signal usage for time unit ns, got %#v", facts.SignalUsages)
	}
}

func TestExtractorE2ESubprogramParamDefaults(t *testing.T) {
	fixture := fixturePath(t, "subprogram_params_defaults.vhd")

	ext := New()
	facts, err := ext.Extract(fixture)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	fn := findFunction(t, facts.Functions, "f1")
	paramA := findParam(t, fn.Parameters, "a")
	if paramA.Class != "constant" || paramA.Direction != "in" || paramA.Type != "integer" || paramA.Default != "3" {
		t.Fatalf("param a: expected constant in integer := 3, got %#v", paramA)
	}
	paramB := findParam(t, fn.Parameters, "b")
	if paramB.Type != "integer" || paramB.Default != "" {
		t.Fatalf("param b: expected integer with no default, got %#v", paramB)
	}

	proc := findProcedure(t, facts.Procedures, "p1")
	paramClk := findParam(t, proc.Parameters, "clk")
	if paramClk.Class != "signal" || paramClk.Direction != "in" || paramClk.Type != "std_logic" {
		t.Fatalf("param clk: expected signal in std_logic, got %#v", paramClk)
	}
	paramV := findParam(t, proc.Parameters, "v")
	if paramV.Class != "variable" || paramV.Direction != "inout" || paramV.Type != "integer" {
		t.Fatalf("param v: expected variable inout integer, got %#v", paramV)
	}
	paramC := findParam(t, proc.Parameters, "c")
	if paramC.Class != "constant" || paramC.Type != "integer" || paramC.Default != "7" {
		t.Fatalf("param c: expected constant integer := 7, got %#v", paramC)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "extractor_e2e", name)
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	return abs
}

func assertPort(t *testing.T, ports []Port, name, direction, typ string) {
	t.Helper()
	for _, p := range ports {
		if p.Name == name {
			if p.Direction != direction {
				t.Fatalf("port %s direction: expected %q, got %q", name, direction, p.Direction)
			}
			if p.Type != typ {
				t.Fatalf("port %s type: expected %q, got %q", name, typ, p.Type)
			}
			return
		}
	}
	t.Fatalf("port %s not found in %#v", name, ports)
}

func assertSignal(t *testing.T, signals []Signal, name, typ string) {
	t.Helper()
	for _, s := range signals {
		if s.Name == name {
			if s.Type != typ {
				t.Fatalf("signal %s type: expected %q, got %q", name, typ, s.Type)
			}
			return
		}
	}
	t.Fatalf("signal %s not found in %#v", name, signals)
}

func assertContains(t *testing.T, list []string, want string) {
	t.Helper()
	for _, item := range list {
		if item == want {
			return
		}
	}
	t.Fatalf("expected %q in %#v", want, list)
}

func findProcess(t *testing.T, processes []Process, label string) Process {
	t.Helper()
	for _, p := range processes {
		if p.Label == label {
			return p
		}
	}
	t.Fatalf("process %q not found in %#v", label, processes)
	return Process{}
}

func findFunction(t *testing.T, funcs []FunctionDeclaration, name string) FunctionDeclaration {
	t.Helper()
	for _, f := range funcs {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("function %q not found in %#v", name, funcs)
	return FunctionDeclaration{}
}

func findProcedure(t *testing.T, procs []ProcedureDeclaration, name string) ProcedureDeclaration {
	t.Helper()
	for _, p := range procs {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("procedure %q not found in %#v", name, procs)
	return ProcedureDeclaration{}
}

func findConstant(t *testing.T, decls []ConstantDeclaration, name string) ConstantDeclaration {
	t.Helper()
	for _, c := range decls {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("constant %q not found in %#v", name, decls)
	return ConstantDeclaration{}
}

func findEntityE2E(t *testing.T, entities []Entity, name string) Entity {
	t.Helper()
	for _, e := range entities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("entity %q not found in %#v", name, entities)
	return Entity{}
}

func findComponentE2E(t *testing.T, comps []Component, name string) Component {
	t.Helper()
	for _, c := range comps {
		if c.Name == name && !c.IsInstance {
			return c
		}
	}
	t.Fatalf("component %q not found in %#v", name, comps)
	return Component{}
}

func findInstanceE2E(t *testing.T, insts []Instance, name string) Instance {
	t.Helper()
	for _, inst := range insts {
		if inst.Name == name {
			return inst
		}
	}
	t.Fatalf("instance %q not found in %#v", name, insts)
	return Instance{}
}

func hasGeneric(generics []GenericDecl, name, kind string) bool {
	for _, g := range generics {
		if g.Name == name && g.Kind == kind {
			return true
		}
	}
	return false
}

func hasAssociationKind(assocs []Association, kind, formal string) bool {
	for _, a := range assocs {
		if a.Kind == kind && a.Formal == formal {
			return true
		}
	}
	return false
}

func hasAssociationActualKind(assocs []Association, kind, actualKind string) bool {
	for _, a := range assocs {
		if a.Kind == kind && a.ActualKind == actualKind {
			return true
		}
	}
	return false
}

func hasVariable(vars []VariableDecl, name, typ string) bool {
	for _, v := range vars {
		if v.Name == name && v.Type == typ {
			return true
		}
	}
	return false
}

func hasFunctionCall(calls []FunctionCall, name string) bool {
	for _, c := range calls {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func hasFunctionCallArg(calls []FunctionCall, name, arg string) bool {
	for _, c := range calls {
		if !strings.EqualFold(c.Name, name) {
			continue
		}
		for _, a := range c.Args {
			if a == arg {
				return true
			}
		}
	}
	return false
}

func findWaitWithOn(waits []WaitStatement) (WaitStatement, bool) {
	for _, w := range waits {
		if len(w.OnSignals) > 0 {
			return w, true
		}
	}
	return WaitStatement{}, false
}

func findWaitWithFor(waits []WaitStatement) (WaitStatement, bool) {
	for _, w := range waits {
		if w.ForExpr != "" {
			return w, true
		}
	}
	return WaitStatement{}, false
}

func findParam(t *testing.T, params []SubprogramParameter, name string) SubprogramParameter {
	t.Helper()
	for _, p := range params {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("param %q not found in %#v", name, params)
	return SubprogramParameter{}
}

func findFunctionWithPackage(t *testing.T, funcs []FunctionDeclaration, name, pkg string) FunctionDeclaration {
	t.Helper()
	for _, f := range funcs {
		if f.Name == name && f.InPackage == pkg {
			return f
		}
	}
	t.Fatalf("function %q with package %q not found in %#v", name, pkg, funcs)
	return FunctionDeclaration{}
}

func findProcedureWithPackage(t *testing.T, procs []ProcedureDeclaration, name, pkg string) ProcedureDeclaration {
	t.Helper()
	for _, p := range procs {
		if p.Name == name && p.InPackage == pkg {
			return p
		}
	}
	t.Fatalf("procedure %q with package %q not found in %#v", name, pkg, procs)
	return ProcedureDeclaration{}
}

func findArithmeticOp(t *testing.T, ops []ArithmeticOp, operator string) ArithmeticOp {
	t.Helper()
	for _, op := range ops {
		if op.Operator == operator {
			return op
		}
	}
	t.Fatalf("arithmetic op %q not found in %#v", operator, ops)
	return ArithmeticOp{}
}

func findCDCCrossing(t *testing.T, crossings []CDCCrossing, signal string) CDCCrossing {
	t.Helper()
	for _, c := range crossings {
		if c.Signal == signal {
			return c
		}
	}
	t.Fatalf("cdc crossing %q not found in %#v", signal, crossings)
	return CDCCrossing{}
}

func findCaseStatement(t *testing.T, cases []CaseStatement, expr string) CaseStatement {
	t.Helper()
	for _, c := range cases {
		if strings.TrimSpace(c.Expression) == expr {
			return c
		}
	}
	t.Fatalf("case statement %q not found in %#v", expr, cases)
	return CaseStatement{}
}

func mustFindConcurrentAssignment(t *testing.T, assigns []ConcurrentAssignment, target string) ConcurrentAssignment {
	t.Helper()
	for _, a := range assigns {
		if a.Target == target {
			return a
		}
	}
	t.Fatalf("concurrent assignment %q not found in %#v", target, assigns)
	return ConcurrentAssignment{}
}

func hasReadSignal(reads []string, want string) bool {
	for _, r := range reads {
		if r == want {
			return true
		}
	}
	return false
}

func hasSignalUsage(usages []SignalUsage, signal string) bool {
	for _, u := range usages {
		if u.Signal == signal {
			return true
		}
	}
	return false
}

func hasSignalUsageReadInPSL(usages []SignalUsage, signal string) bool {
	for _, u := range usages {
		if u.Signal == signal && u.IsRead && u.InPSL {
			return true
		}
	}
	return false
}

func assertNoSignal(t *testing.T, list []string, unwanted string) {
	t.Helper()
	for _, item := range list {
		if strings.EqualFold(item, unwanted) {
			t.Fatalf("did not expect %q in %#v", unwanted, list)
		}
	}
}

func assertParam(t *testing.T, params []SubprogramParameter, name, direction, typ, class string) {
	t.Helper()
	for _, p := range params {
		if p.Name == name {
			if p.Direction != direction {
				t.Fatalf("param %s direction: expected %q, got %q", name, direction, p.Direction)
			}
			if p.Type != typ {
				t.Fatalf("param %s type: expected %q, got %q", name, typ, p.Type)
			}
			if p.Class != class {
				t.Fatalf("param %s class: expected %q, got %q", name, class, p.Class)
			}
			return
		}
	}
	t.Fatalf("param %s not found in %#v", name, params)
}

func assertRecordField(t *testing.T, fields []RecordField, name, typ string) {
	t.Helper()
	for _, f := range fields {
		if f.Name == name {
			if f.Type != typ {
				t.Fatalf("record field %s type: expected %q, got %q", name, typ, f.Type)
			}
			return
		}
	}
	t.Fatalf("record field %s not found in %#v", name, fields)
}

func assertMap(t *testing.T, m map[string]string, key, value string) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Fatalf("map missing key %q in %#v", key, m)
	}
	if got != value {
		t.Fatalf("map %s: expected %q, got %q", key, value, got)
	}
}

func assertConcurrentAssignment(t *testing.T, assigns []ConcurrentAssignment, target, kind string, inGenerate bool) {
	t.Helper()
	for _, a := range assigns {
		if a.Target == target && a.Kind == kind && a.InGenerate == inGenerate {
			return
		}
	}
	t.Fatalf("concurrent assignment target %q kind %q inGenerate=%v not found in %#v", target, kind, inGenerate, assigns)
}

func assertSignalInScope(t *testing.T, signals []Signal, name, scope string) {
	t.Helper()
	for _, s := range signals {
		if s.Name == name && s.InEntity == scope {
			return
		}
	}
	t.Fatalf("signal %s in scope %s not found in %#v", name, scope, signals)
}

func hasComparison(comps []Comparison, operator, literal string) bool {
	for _, c := range comps {
		if c.Operator == operator && c.IsLiteral && strings.TrimSpace(c.LiteralValue) == literal {
			return true
		}
	}
	return false
}

func findConfigBinding(t *testing.T, bindings []ConfigurationBinding, instance string) ConfigurationBinding {
	t.Helper()
	for _, b := range bindings {
		if b.InstanceLabel == instance {
			return b
		}
	}
	t.Fatalf("configuration binding %q not found in %#v", instance, bindings)
	return ConfigurationBinding{}
}

func hasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

func mustFindDisconnection(t *testing.T, specs []DisconnectionSpecification, target string) DisconnectionSpecification {
	t.Helper()
	for _, spec := range specs {
		if strings.EqualFold(spec.Target, target) {
			return spec
		}
	}
	t.Fatalf("disconnection target %q not found in %#v", target, specs)
	return DisconnectionSpecification{}
}

func hasProtectedType(types []TypeDeclaration, name string) bool {
	for _, td := range types {
		if td.Name == name && td.Kind == "protected" {
			return true
		}
	}
	return false
}

func hasTypeName(types []TypeDeclaration, name string) bool {
	for _, td := range types {
		if strings.EqualFold(td.Name, name) {
			return true
		}
	}
	return false
}

func hasSignalNamed(signals []Signal, name string) bool {
	for _, s := range signals {
		if strings.EqualFold(s.Name, name) {
			return true
		}
	}
	return false
}

func countAssignmentsTo(assigns []ConcurrentAssignment, target string) int {
	count := 0
	for _, a := range assigns {
		if strings.EqualFold(a.Target, target) {
			count++
		}
	}
	return count
}
