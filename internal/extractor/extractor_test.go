package extractor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractorDeclarationsAndTypes(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;
use work.my_pkg.all;

package my_pkg is
  type state_t is (IDLE, RUN, STOP);
  type rec_t is record
    valid : std_logic;
    data  : std_logic_vector(7 downto 0);
  end record;
  type mem_t is array (0 to 3) of std_logic_vector(7 downto 0);
  type counter_t is range 0 to 15;
  subtype small_int is integer range 0 to 7;
  constant WIDTH : integer := 8;
  constant MODE  : string := "fast";
  shared variable shared_var : integer;
  function add(a : integer; b : integer) return integer;
  impure function f_impure return integer;
  procedure do_it(signal x : in std_logic; y : out std_logic);
end package;

package body my_pkg is
  function add(a : integer; b : integer) return integer is
  begin
    return a + b;
  end function;
  impure function f_impure return integer is
  begin
    return 1;
  end function;
  procedure do_it(signal x : in std_logic; y : out std_logic) is
  begin
    y <= x;
  end procedure;
end package body;

entity top is
  port(
    clk   : in std_logic;
    rst_n : in std_logic;
    a, b  : in std_logic_vector(7 downto 0);
    y     : out std_logic_vector(7 downto 0)
  );
end entity;

architecture rtl of top is
  constant ARCH_CONST : integer := 3;
  signal s1, s2 : std_logic_vector(7 downto 0);
  component child
    port(
      cclk : in std_logic;
      din  : in std_logic_vector(7 downto 0);
      dout : out std_logic_vector(7 downto 0)
    );
  end component;
begin
  u_child : entity work.child
    generic map (G_WIDTH => 8)
    port map (cclk => clk, din => s1, dout => s2);

  y <= s2;
end architecture;

configuration cfg_top of top is
  for rtl
  end for;
end cfg_top;
`

	facts := parseVHDL(t, vhdl)

	if _, ok := findEntity(facts.Entities, "top"); !ok {
		t.Fatalf("expected entity top")
	}
	if arch, ok := findArchitecture(facts.Architectures, "rtl"); !ok || !strings.EqualFold(arch.EntityName, "top") {
		t.Fatalf("expected architecture rtl of top")
	}
	if _, ok := findPackageByName(facts.Packages, "my_pkg"); !ok {
		t.Fatalf("expected package my_pkg")
	}
	if _, ok := findConfigurationByName(facts.Configurations, "cfg_top"); !ok {
		t.Fatalf("expected configuration cfg_top")
	}

	clkPort := mustFindPort(t, facts.Ports, "clk")
	if clkPort.Direction != "in" || !typeEq(clkPort.Type, "std_logic") {
		t.Fatalf("expected clk port to be in std_logic, got %q %q", clkPort.Direction, clkPort.Type)
	}
	yPort := mustFindPort(t, facts.Ports, "y")
	if yPort.Direction != "out" || !typeEq(yPort.Type, "std_logic_vector(7 downto 0)") {
		t.Fatalf("expected y port to be out std_logic_vector(7 downto 0), got %q %q", yPort.Direction, yPort.Type)
	}

	s1 := mustFindSignal(t, facts.Signals, "s1")
	if !typeEq(s1.Type, "std_logic_vector(7 downto 0)") || !strings.EqualFold(s1.InEntity, "rtl") {
		t.Fatalf("expected s1 signal type std_logic_vector(7 downto 0) in rtl, got %q in %q", s1.Type, s1.InEntity)
	}

	if comp, ok := findComponentDecl(facts.Components, "child"); !ok || comp.IsInstance {
		t.Fatalf("expected component declaration child")
	}
	if comp, ok := findComponentInst(facts.Components, "u_child"); !ok || comp.EntityRef == "" {
		t.Fatalf("expected component instance u_child with entity ref")
	}

	inst := mustFindInstance(t, facts.Instances, "u_child")
	if !strings.EqualFold(inst.Target, "work.child") {
		t.Fatalf("expected instance target work.child, got %q", inst.Target)
	}
	if inst.PortMap["cclk"] != "clk" || inst.PortMap["din"] != "s1" || inst.PortMap["dout"] != "s2" {
		t.Fatalf("unexpected port map: %#v", inst.PortMap)
	}
	if inst.GenericMap["G_WIDTH"] != "8" {
		t.Fatalf("unexpected generic map: %#v", inst.GenericMap)
	}

	if !hasDependencyKind(facts.Dependencies, "library", "ieee") {
		t.Fatalf("expected library dependency ieee")
	}
	if !hasDependencyKind(facts.Dependencies, "use", "ieee.std_logic_1164") {
		t.Fatalf("expected use dependency ieee.std_logic_1164")
	}
	if !hasDependencyKind(facts.Dependencies, "use", "work.my_pkg") {
		t.Fatalf("expected use dependency work.my_pkg")
	}
	if !hasDependencyKind(facts.Dependencies, "instantiation", "work.child") {
		t.Fatalf("expected instantiation dependency work.child")
	}

	enumType := mustFindType(t, facts.Types, "state_t")
	if enumType.Kind != "enum" || !hasString(enumType.EnumLiterals, "IDLE") || !hasString(enumType.EnumLiterals, "RUN") {
		t.Fatalf("expected enum type state_t with literals, got %#v", enumType)
	}
	recType := mustFindType(t, facts.Types, "rec_t")
	if recType.Kind != "record" || !hasRecordField(recType.Fields, "valid") || !hasRecordField(recType.Fields, "data") {
		t.Fatalf("expected record type rec_t with fields, got %#v", recType)
	}
	arrType := mustFindType(t, facts.Types, "mem_t")
	if arrType.Kind != "array" || !strings.HasPrefix(strings.ToLower(arrType.ElementType), "std_logic_vector") {
		t.Fatalf("expected array type mem_t element std_logic_vector, got %#v", arrType)
	}
	rangeType := mustFindType(t, facts.Types, "counter_t")
	if rangeType.Kind == "range" {
		if rangeType.RangeLow != "0" || rangeType.RangeHigh != "15" {
			t.Fatalf("expected range type counter_t 0 to 15, got %#v", rangeType)
		}
	} else if rangeType.Kind != "alias" {
		t.Fatalf("expected counter_t kind range or alias, got %#v", rangeType)
	}

	subtype := mustFindSubtype(t, facts.Subtypes, "small_int")
	if !strings.EqualFold(subtype.BaseType, "integer") || !strings.Contains(strings.ToLower(subtype.Constraint), "range 0 to 7") {
		t.Fatalf("expected subtype small_int integer range 0 to 7, got %#v", subtype)
	}

	if !hasString(facts.EnumLiterals, "IDLE") {
		t.Fatalf("expected legacy enum literals to include IDLE")
	}
	if !hasString(facts.Constants, "WIDTH") {
		t.Fatalf("expected legacy constants to include WIDTH")
	}
	if !hasString(facts.SharedVariables, "shared_var") {
		t.Fatalf("expected shared variables to include shared_var")
	}

	if !hasConstantDecl(facts.ConstantDecls, "WIDTH", "my_pkg") {
		t.Fatalf("expected constant WIDTH in package my_pkg")
	}
	if !hasConstantDecl(facts.ConstantDecls, "ARCH_CONST", "") {
		t.Fatalf("expected constant ARCH_CONST in architecture")
	}

	if !hasFunction(facts.Functions, "add", true) {
		t.Fatalf("expected function add with body")
	}
	if !hasFunction(facts.Functions, "f_impure", false) {
		t.Fatalf("expected impure function f_impure")
	}
	if !hasProcedure(facts.Procedures, "do_it", true) {
		t.Fatalf("expected procedure do_it with body")
	}

	if !hasConcurrentAssignment(facts.ConcurrentAssignments, "y", "simple") {
		t.Fatalf("expected simple concurrent assignment to y")
	}
	if !hasSignalUsagePortMap(facts.SignalUsages, "s1", "u_child") {
		t.Fatalf("expected signal usage for s1 in port map")
	}
}

func TestExtractorPhysicalUnits(t *testing.T) {
	vhdl := `package phys_pkg is
  type duration_t is range 0 to 1E6
    units
      fsec;
      nsec = 1000 fsec;
    end units;
end package;`

	facts := parseVHDL(t, vhdl)
	if !hasString(facts.EnumLiterals, "fsec") {
		t.Fatalf("expected physical unit fsec to be recorded as literal")
	}
	if !hasString(facts.EnumLiterals, "nsec") {
		t.Fatalf("expected physical unit nsec to be recorded as literal")
	}
}

func TestExtractorGroupDeclarationReads(t *testing.T) {
	vhdl := `entity e is
end entity;

architecture rtl of e is
  signal ck : bit;
  signal q  : bit;
  group G1 : PIN2PIN(ck, q);
begin
end architecture;`

	facts := parseVHDL(t, vhdl)
	if !hasSignalUsageRead(facts.SignalUsages, "ck") {
		t.Fatalf("expected group declaration to read ck")
	}
	if !hasSignalUsageRead(facts.SignalUsages, "q") {
		t.Fatalf("expected group declaration to read q")
	}
}

func TestExtractorPortDefaults(t *testing.T) {
	vhdl := `entity top is
  port(
    a : in std_logic := '1';
    b : in std_logic
  );
end entity;
`
	facts := parseVHDL(t, vhdl)
	portA := mustFindPort(t, facts.Ports, "a")
	if portA.Default != "'1'" {
		t.Fatalf("expected port a default '1', got %q", portA.Default)
	}
	portB := mustFindPort(t, facts.Ports, "b")
	if portB.Default != "" {
		t.Fatalf("expected port b default empty, got %q", portB.Default)
	}
}

func TestExtractorProcessesAndSemantics(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity proc_top is
  port(
    clk : in std_logic;
    rst : in std_logic;
    en  : in std_logic;
    a   : in std_logic;
    b   : in std_logic;
    y   : out std_logic
  );
end;

architecture rtl of proc_top is
  type state_t is (IDLE, RUN);
  signal r1, r2, comb : std_logic;
  signal state : state_t;
begin
  p_seq : process(clk, rst)
  begin
    if rst = '1' then
      r1 <= '0';
    elsif rising_edge(clk) then
      r1 <= a;
      if en = '1' then
        r2 <= r1 * 2;
        r2 <= r1 ** 2;
      end if;
    end if;
  end process;

  p_comb : process(all)
  begin
    case state is
      when IDLE => comb <= a;
      when RUN  => comb <= b;
      when others => comb <= '0';
    end case;
    if r1 = '1' then
      comb <= r1;
    end if;
  end process;

  p_wait : process
  begin
    wait until rising_edge(clk);
    y <= r2;
  end process;
end;
`

	facts := parseVHDL(t, vhdl)

	pSeq := mustFindProcess(t, facts.Processes, "p_seq")
	if !pSeq.IsSequential || !strings.EqualFold(pSeq.ClockSignal, "clk") || pSeq.ClockEdge != "rising" {
		t.Fatalf("expected p_seq to be sequential on rising clk, got %#v", pSeq)
	}
	if !pSeq.HasReset || !strings.EqualFold(pSeq.ResetSignal, "rst") || !pSeq.ResetAsync {
		t.Fatalf("expected p_seq to have async reset rst, got %#v", pSeq)
	}
	if !hasString(pSeq.AssignedSignals, "r1") || !hasString(pSeq.ReadSignals, "a") {
		t.Fatalf("expected p_seq to assign r1 and read a, got %#v", pSeq)
	}

	pComb := mustFindProcess(t, facts.Processes, "p_comb")
	if !pComb.IsCombinational || !hasString(pComb.SensitivityList, "all") {
		t.Fatalf("expected p_comb to be combinational with sensitivity all, got %#v", pComb)
	}

	pWait := mustFindProcess(t, facts.Processes, "p_wait")
	if !pWait.HasWait || pWait.IsCombinational {
		t.Fatalf("expected p_wait to use wait and not be combinational, got %#v", pWait)
	}

	if !hasClockDomainForProc(facts.ClockDomains, "clk", "p_seq") {
		t.Fatalf("expected clock domain for clk in p_seq")
	}
	if !hasResetInfo(facts.ResetInfos, "rst", "p_seq") {
		t.Fatalf("expected reset info for rst in p_seq")
	}

	if !hasCaseStatement(facts.CaseStatements, "state") {
		t.Fatalf("expected case statement on state")
	}
	if !hasComparisonLiteral(facts.Comparisons, "rst") {
		t.Fatalf("expected comparison involving rst literal")
	}
	if !hasArithmeticOp(facts.ArithmeticOps, "*", "en") {
		t.Fatalf("expected guarded arithmetic op with * and guard en")
	}
	if !hasSignalDep(facts.SignalDeps, "a", "r1") {
		t.Fatalf("expected signal dependency a -> r1")
	}
}

func TestExtractorPortMapPreservesSlices(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity child is
  port (
    shamt_i : in std_logic_vector(4 downto 0);
    bit_i   : in std_logic
  );
end entity;

entity top is
  port (
    clk : in std_logic
  );
end entity;

architecture rtl of top is
  signal opb : std_logic_vector(31 downto 0);
begin
  u_child: entity work.child
    port map (
      shamt_i => opb(4 downto 0),
      bit_i   => opb(0)
    );
end architecture;
`

	facts := parseVHDL(t, vhdl)
	inst := mustFindInstance(t, facts.Instances, "u_child")
	if got := inst.PortMap["shamt_i"]; got != "opb(4 downto 0)" {
		t.Fatalf("expected shamt_i slice, got %q", got)
	}
	if got := inst.PortMap["bit_i"]; got != "opb(0)" {
		t.Fatalf("expected bit_i index, got %q", got)
	}
}

func TestExtractorReadsIndexedNameInExpression(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;
use ieee.numeric_std.all;

entity top is
  port (
    clk     : in std_logic;
    rstn    : in std_logic;
    en_i    : in std_logic;
    opa_i   : in std_logic_vector(7 downto 0);
    opa_sn_i: in std_logic
  );
end entity;

architecture rtl of top is
  signal opa : signed(8 downto 0);
begin
  p1: process(rstn, clk)
  begin
    if (rstn = '0') then
      opa <= (others => '0');
    elsif rising_edge(clk) then
      if (en_i = '1') then
        opa <= signed((opa_i(opa_i'left) and opa_sn_i) & opa_i);
      end if;
    end if;
  end process;
end architecture;
`

	facts := parseVHDL(t, vhdl)
	p := mustFindProcess(t, facts.Processes, "p1")
	if !hasString(p.ReadSignals, "opa_i") || !hasString(p.ReadSignals, "opa_sn_i") {
		t.Fatalf("expected reads to include opa_i and opa_sn_i, got %v", p.ReadSignals)
	}
}

func TestExtractorSignalDepsPreserveRecordFields(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity top is end;

architecture rtl of top is
  type t is record
    start : std_logic;
    done  : std_logic;
  end record;
  signal fu_mul : t;
  signal multiplier : t;
begin
  fu_mul.done <= multiplier.done;
end architecture;
`

	facts := parseVHDL(t, vhdl)
	if !hasSignalDep(facts.SignalDeps, "multiplier.done", "fu_mul.done") {
		t.Fatalf("expected signal dep multiplier.done -> fu_mul.done, got %#v", facts.SignalDeps)
	}
}

func TestExtractorGenerateStatements(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity gen_top is
  generic (N : integer := 2);
  port(
    clk : in std_logic;
    din : in std_logic_vector(1 downto 0);
    dout : out std_logic_vector(1 downto 0);
    sel : in std_logic_vector(1 downto 0)
  );
end;

architecture rtl of gen_top is
  signal gsig : std_logic_vector(1 downto 0);
begin
  gen_for : for i in 0 to 1 generate
    signal s_local : std_logic;
  begin
    gsig(i) <= din(i);
  end generate;

  gen_if : if N = 2 generate
  begin
    u_buf : entity work.child
      port map (cclk => clk, din => gsig, dout => dout);
  end generate;

  gen_case : case sel generate
    when "00" =>
      gsig <= (others => '0');
    when others =>
      gsig <= (others => '1');
  end generate;
end;
`

	facts := parseVHDL(t, vhdl)

	genFor := mustFindGenerate(t, facts.Generates, "gen_for")
	if genFor.Kind != "for" || genFor.LoopVar != "i" || genFor.RangeLow != "0" || genFor.RangeHigh != "1" || genFor.RangeDir != "to" {
		t.Fatalf("expected for-generate details, got %#v", genFor)
	}
	genIf := mustFindGenerate(t, facts.Generates, "gen_if")
	if genIf.Kind != "if" || !strings.Contains(strings.ToLower(genIf.Condition), "n") {
		t.Fatalf("expected if-generate condition to include N, got %#v", genIf)
	}
	genCase := mustFindGenerate(t, facts.Generates, "gen_case")
	if genCase.Kind != "case" || !strings.Contains(genCase.Condition, "sel") {
		t.Fatalf("expected case-generate on sel, got %#v", genCase)
	}

	if !hasGenerateSignal(facts.Signals, "s_local", "rtl.gen_for") {
		t.Fatalf("expected signal s_local scoped to rtl.gen_for")
	}
	if !hasGenerateConcurrentAssignment(facts.ConcurrentAssignments, "gen_for") {
		t.Fatalf("expected concurrent assignment inside generate gen_for")
	}
	if !hasGenerateInstance(facts.Instances, "u_buf", "rtl.gen_if") {
		t.Fatalf("expected instance u_buf scoped to rtl.gen_if")
	}
}

func TestExtractorCDCCrossings(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity cdc_top is
  port(
    clk_a : in std_logic;
    clk_b : in std_logic;
    din   : in std_logic;
    din_bus : in std_logic_vector(3 downto 0);
    dout  : out std_logic
  );
end;

architecture rtl of cdc_top is
  signal reg_a : std_logic;
  signal sync1, sync2 : std_logic;
  signal reg_b : std_logic;
  signal bus_a : std_logic_vector(3 downto 0);
  signal bus_b : std_logic_vector(3 downto 0);
begin
  p_a : process(clk_a)
  begin
    if rising_edge(clk_a) then
      reg_a <= din;
      bus_a <= din_bus;
    end if;
  end process;

  p_sync1 : process(clk_b)
  begin
    if rising_edge(clk_b) then
      sync1 <= reg_a;
    end if;
  end process;

  p_sync2 : process(clk_b)
  begin
    if rising_edge(clk_b) then
      sync2 <= sync1;
    end if;
  end process;

  p_b : process(clk_b)
  begin
    if rising_edge(clk_b) then
      reg_b <= sync2;
      bus_b <= bus_a;
    end if;
  end process;
end;
`

	facts := parseVHDL(t, vhdl)

	regCross, ok := findCDCCrossingBySignal(facts.CDCCrossings, "reg_a", "clk_a", "clk_b")
	if !ok {
		t.Fatalf("expected CDC crossing for reg_a, got %#v", facts.CDCCrossings)
	}
	if !regCross.IsSynchronized || regCross.SyncStages < 1 {
		t.Fatalf("expected synchronized CDC crossing for reg_a, got %#v", regCross)
	}

	busCross, ok := findCDCCrossingBySignal(facts.CDCCrossings, "bus_a", "clk_a", "clk_b")
	if !ok {
		t.Fatalf("expected CDC crossing for bus_a, got %#v", facts.CDCCrossings)
	}
	if busCross.IsSynchronized {
		t.Fatalf("expected unsynchronized CDC crossing for bus_a, got %#v", busCross)
	}
}

func TestExtractorConcurrentAssignmentKinds(t *testing.T) {
	vhdl := `library ieee;
use ieee.std_logic_1164.all;

entity ca_top is
  port(
    a   : in std_logic;
    b   : in std_logic;
    sel : in std_logic;
    y   : out std_logic;
    z   : out std_logic
  );
end;

architecture rtl of ca_top is
begin
  y <= a when sel = '1' else b;
  with sel select z <= a when '0', b when others;
end;
`

	facts := parseVHDL(t, vhdl)
	if !hasConcurrentAssignment(facts.ConcurrentAssignments, "y", "conditional") {
		t.Fatalf("expected conditional assignment to y")
	}
	if !hasConcurrentAssignment(facts.ConcurrentAssignments, "z", "selected") {
		t.Fatalf("expected selected assignment to z")
	}
}

func parseVHDL(t *testing.T, src string) FileFacts {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.vhd")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write vhdl: %v", err)
	}

	ext := New()
	facts, err := ext.Extract(path)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return facts
}

func typeEq(actual, expected string) bool {
	return normalizeSpace(actual) == normalizeSpace(expected)
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func findEntity(entities []Entity, name string) (Entity, bool) {
	for _, e := range entities {
		if strings.EqualFold(e.Name, name) {
			return e, true
		}
	}
	return Entity{}, false
}

func findArchitecture(archs []Architecture, name string) (Architecture, bool) {
	for _, a := range archs {
		if strings.EqualFold(a.Name, name) {
			return a, true
		}
	}
	return Architecture{}, false
}

func findPackageByName(pkgs []Package, name string) (Package, bool) {
	for _, p := range pkgs {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Package{}, false
}

func findConfigurationByName(cfgs []ConfigurationDeclaration, name string) (ConfigurationDeclaration, bool) {
	for _, c := range cfgs {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return ConfigurationDeclaration{}, false
}

func mustFindPort(t *testing.T, ports []Port, name string) Port {
	t.Helper()
	for _, p := range ports {
		if strings.EqualFold(p.Name, name) {
			return p
		}
	}
	t.Fatalf("port not found: %s", name)
	return Port{}
}

func mustFindSignal(t *testing.T, signals []Signal, name string) Signal {
	t.Helper()
	for _, s := range signals {
		if strings.EqualFold(s.Name, name) {
			return s
		}
	}
	t.Fatalf("signal not found: %s", name)
	return Signal{}
}

func findComponentDecl(components []Component, name string) (Component, bool) {
	for _, c := range components {
		if !c.IsInstance && strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Component{}, false
}

func findComponentInst(components []Component, name string) (Component, bool) {
	for _, c := range components {
		if c.IsInstance && strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Component{}, false
}

func mustFindInstance(t *testing.T, insts []Instance, name string) Instance {
	t.Helper()
	for _, inst := range insts {
		if strings.EqualFold(inst.Name, name) {
			return inst
		}
	}
	t.Fatalf("instance not found: %s", name)
	return Instance{}
}

func hasDependencyKind(deps []Dependency, kind, targetSubstr string) bool {
	targetSubstr = strings.ToLower(targetSubstr)
	for _, d := range deps {
		if strings.EqualFold(d.Kind, kind) && strings.Contains(strings.ToLower(d.Target), targetSubstr) {
			return true
		}
	}
	return false
}

func mustFindType(t *testing.T, types []TypeDeclaration, name string) TypeDeclaration {
	t.Helper()
	for _, td := range types {
		if strings.EqualFold(td.Name, name) {
			return td
		}
	}
	t.Fatalf("type not found: %s", name)
	return TypeDeclaration{}
}

func mustFindSubtype(t *testing.T, subs []SubtypeDeclaration, name string) SubtypeDeclaration {
	t.Helper()
	for _, st := range subs {
		if strings.EqualFold(st.Name, name) {
			return st
		}
	}
	t.Fatalf("subtype not found: %s", name)
	return SubtypeDeclaration{}
}

func hasRecordField(fields []RecordField, name string) bool {
	for _, f := range fields {
		if strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

func hasString(list []string, value string) bool {
	for _, s := range list {
		if strings.EqualFold(s, value) {
			return true
		}
	}
	return false
}

func hasConstantDecl(constants []ConstantDeclaration, name, inPackage string) bool {
	for _, c := range constants {
		if strings.EqualFold(c.Name, name) {
			if inPackage == "" || strings.EqualFold(c.InPackage, inPackage) {
				return true
			}
		}
	}
	return false
}

func hasFunction(funcs []FunctionDeclaration, name string, hasBody bool) bool {
	for _, f := range funcs {
		if strings.EqualFold(f.Name, name) {
			if !hasBody || f.HasBody {
				if strings.EqualFold(name, "f_impure") && f.IsPure {
					continue
				}
				return true
			}
		}
	}
	return false
}

func hasProcedure(procs []ProcedureDeclaration, name string, hasBody bool) bool {
	for _, p := range procs {
		if strings.EqualFold(p.Name, name) {
			if !hasBody || p.HasBody {
				return true
			}
		}
	}
	return false
}

func hasConcurrentAssignment(cas []ConcurrentAssignment, target, kind string) bool {
	for _, ca := range cas {
		if strings.EqualFold(ca.Target, target) && strings.EqualFold(ca.Kind, kind) {
			return true
		}
	}
	return false
}

func hasSignalUsagePortMap(usages []SignalUsage, signal, inst string) bool {
	for _, u := range usages {
		if u.InPortMap && strings.EqualFold(u.Signal, signal) && strings.EqualFold(u.InstanceName, inst) {
			return true
		}
	}
	return false
}

func mustFindProcess(t *testing.T, procs []Process, label string) Process {
	t.Helper()
	for _, p := range procs {
		if strings.EqualFold(p.Label, label) {
			return p
		}
	}
	t.Fatalf("process not found: %s", label)
	return Process{}
}

func hasClockDomainForProc(domains []ClockDomain, clock, proc string) bool {
	for _, d := range domains {
		if strings.EqualFold(d.Clock, clock) && strings.EqualFold(d.Process, proc) {
			return true
		}
	}
	return false
}

func hasResetInfo(infos []ResetInfo, signal, proc string) bool {
	for _, r := range infos {
		if strings.EqualFold(r.Signal, signal) && strings.EqualFold(r.Process, proc) {
			return true
		}
	}
	return false
}

func hasCaseStatement(cases []CaseStatement, expr string) bool {
	for _, c := range cases {
		if strings.EqualFold(c.Expression, expr) && c.HasOthers {
			return true
		}
	}
	return false
}

func hasComparisonLiteral(comps []Comparison, left string) bool {
	for _, c := range comps {
		if strings.EqualFold(c.LeftOperand, left) && c.Operator == "=" && c.IsLiteral {
			return true
		}
	}
	return false
}

func hasArithmeticOp(ops []ArithmeticOp, operator, guard string) bool {
	for _, op := range ops {
		if op.Operator == operator && op.IsGuarded && strings.EqualFold(op.GuardSignal, guard) {
			return true
		}
	}
	return false
}

func hasSignalDep(deps []SignalDep, source, target string) bool {
	for _, d := range deps {
		if strings.EqualFold(d.Source, source) && strings.EqualFold(d.Target, target) {
			return true
		}
	}
	return false
}

func hasSignalUsageRead(usages []SignalUsage, name string) bool {
	for _, usage := range usages {
		if usage.IsRead && strings.EqualFold(usage.Signal, name) {
			return true
		}
	}
	return false
}

func mustFindGenerate(t *testing.T, gens []GenerateStatement, label string) GenerateStatement {
	t.Helper()
	for _, g := range gens {
		if strings.EqualFold(g.Label, label) {
			return g
		}
	}
	t.Fatalf("generate not found: %s", label)
	return GenerateStatement{}
}

func hasGenerateSignal(signals []Signal, name, inEntity string) bool {
	for _, s := range signals {
		if strings.EqualFold(s.Name, name) && strings.EqualFold(s.InEntity, inEntity) {
			return true
		}
	}
	return false
}

func hasGenerateConcurrentAssignment(cas []ConcurrentAssignment, label string) bool {
	for _, ca := range cas {
		if ca.InGenerate && strings.EqualFold(ca.GenerateLabel, label) {
			return true
		}
	}
	return false
}

func hasGenerateInstance(insts []Instance, name, inArch string) bool {
	for _, inst := range insts {
		if strings.EqualFold(inst.Name, name) && strings.EqualFold(inst.InArch, inArch) {
			return true
		}
	}
	return false
}

func findCDCCrossingBySignal(crossings []CDCCrossing, signal, srcClock, dstClock string) (CDCCrossing, bool) {
	for _, c := range crossings {
		if strings.EqualFold(c.Signal, signal) && strings.EqualFold(c.SourceClock, srcClock) && strings.EqualFold(c.DestClock, dstClock) {
			return c, true
		}
	}
	return CDCCrossing{}, false
}

func hasCDCCrossing(crossings []CDCCrossing, signal, srcClock, dstClock string, synchronized bool, stages int) bool {
	for _, c := range crossings {
		if strings.EqualFold(c.Signal, signal) && strings.EqualFold(c.SourceClock, srcClock) && strings.EqualFold(c.DestClock, dstClock) {
			if c.IsSynchronized == synchronized {
				if !synchronized || c.SyncStages == stages {
					return true
				}
			}
		}
	}
	return false
}
