package main

// Type represents a VHDL type with its bit width and potential advanced structure.
type Type struct {
	Name  string
	Width int // 1 for std_logic, N for std_logic_vector
	
	IsArray     bool
	ArraySize   int
	ElementType *Type
	
	IsRecord    bool
	Fields      []RecordField
}

type RecordField struct {
	Name string
	Type Type
}

// Port represents an entity port.
type Port struct {
	Name      string
	Direction string // "in" or "out"
	Type      Type
	LineNum   uint32
}

// Signal represents a declared signal (copper wire).
type Signal struct {
	Name string
	Type Type
}

// Generic represents a parameterized integer.
type Generic struct {
	Name    string
	Default string
}

// Contract represents the verif.contract attached to an entity.
type Contract struct {
	Requires []Expression
	Ensures  []Expression
	LineNum  uint32
}

// Module represents the complete Micro-VHDL design unit (Entity + Architecture).
type Module struct {
	Name       string
	ClockPort  string   // primary clock (first detected; backward compat)
	ClockPorts []string // all clock ports (populated when multi-clock detected)
	Generics   []*Generic
	Ports      []*Port
	Signals    []*Signal
	Statements []Statement
	Contract   *Contract
	Formals    []*PslFormalBlock

	// Symbol Table
	Symbols map[string]Type
}

func NewModule(name string) *Module {
	return &Module{
		Name:    name,
		Symbols: make(map[string]Type),
	}
}

// PslFormalBlock represents a verif.formal standalone verification block.
type PslFormalBlock struct {
	Name       string
	Symbols    map[string]Type
	Statements []Statement
}

type SymbolicDeclaration struct {
	Names   []string
	Type    Type
	LineNum uint32
}

func (s *SymbolicDeclaration) isStatement()   {}
func (s *SymbolicDeclaration) Line() uint32 { return s.LineNum }


// Statement is the interface for all statements in the architecture body.
type Statement interface {
	isStatement()
	Line() uint32
}

// ConcurrentAssignment represents `target <= value [when condition else alt_value];`
type ConcurrentAssignment struct {
	Target    string
	Value     Expression
	Condition Expression // optional, nil if not present
	AltValue  Expression // optional, nil if not present
	LineNum   uint32
}

func (c *ConcurrentAssignment) isStatement()   {}
func (c *ConcurrentAssignment) Line() uint32 { return c.LineNum }

// SelectedAssignment represents `with selector select target <= value when condition, ...;`
type SelectedAssignment struct {
	Selector Expression
	Target   string
	Choices  []Choice
	LineNum  uint32
}

func (s *SelectedAssignment) isStatement()   {}
func (s *SelectedAssignment) Line() uint32 { return s.LineNum }

type Choice struct {
	Value     Expression
	Condition Expression // nil if "others"
	IsOthers  bool
}

// GenerateStatement represents `for iterator in start to end generate ... end generate;`
type GenerateStatement struct {
	Label      string
	Iterator   string
	Start      int
	End        int
	Statements []Statement
	LineNum    uint32
}

func (g *GenerateStatement) isStatement()   {}
func (g *GenerateStatement) Line() uint32 { return g.LineNum }

// SynchronousProcess represents the strictly clocked process.
type SynchronousProcess struct {
	Clock      string             // name of the clock port this process uses
	Statements []SequentialStatement
	LineNum    uint32
}

func (s *SynchronousProcess) isStatement()   {}
func (s *SynchronousProcess) Line() uint32 { return s.LineNum }

// SequentialStatement is the interface for statements inside a synchronous process.
type SequentialStatement interface {
	isSequentialStatement()
	Line() uint32
}

// SequentialAssignment represents `target <= value;` inside a process.
type SequentialAssignment struct {
	Target  string
	Value   Expression
	LineNum uint32
}

func (s *SequentialAssignment) isSequentialStatement() {}
func (s *SequentialAssignment) Line() uint32           { return s.LineNum }

// SequentialIf represents an `if ... then ... elsif ... else ... end if;` inside a process.
type SequentialIf struct {
	Condition Expression
	Then      []SequentialStatement
	Elsifs    []Elsif
	Else      []SequentialStatement // optional
	LineNum   uint32
}

func (s *SequentialIf) isSequentialStatement() {}
func (s *SequentialIf) Line() uint32           { return s.LineNum }

type Elsif struct {
	Condition Expression
	Then      []SequentialStatement
}

// EntityInstantiation represents direct component instantiation.
type EntityInstantiation struct {
	Label      string
	EntityName string
	GenericMap []PortMapEntry
	PortMap    []PortMapEntry
	LineNum    uint32
}

func (e *EntityInstantiation) isStatement()   {}
func (e *EntityInstantiation) Line() uint32 { return e.LineNum }

// PslAssertion represents `psl assert always property;`
type PslAssertion struct {
	Property Expression
	LineNum  uint32
}

func (p *PslAssertion) isStatement()   {}
func (p *PslAssertion) Line() uint32 { return p.LineNum }

// PslAssumption represents `psl assume always property;`
// Emits verif.assume in BMC mode (constrains solver input space) and gates
// __verif_bad with the assumption conjunction in IC3 mode.
type PslAssumption struct {
	Property Expression
	LineNum  uint32
}

func (p *PslAssumption) isStatement()   {}
func (p *PslAssumption) Line() uint32 { return p.LineNum }

// PslCover represents `psl cover property;`
type PslCover struct {
	Property Expression
	LineNum  uint32
}

func (p *PslCover) isStatement()   {}
func (p *PslCover) Line() uint32 { return p.LineNum }

// PslAssertNever represents `psl assert never property;`
type PslAssertNever struct {
	Property Expression
	LineNum  uint32
}

func (p *PslAssertNever) isStatement()   {}
func (p *PslAssertNever) Line() uint32 { return p.LineNum }

// PslAfterResetAssertion represents `psl assert always after_reset(rst) property;`
// The property is checked only after the reset signal has de-asserted at least once.
// Before that, the assertion is vacuously true.
// Implemented via a sticky `past_reset` register: once rst goes low it latches high,
// and the effective assertion is `NOT(past_reset) OR property`.
type PslAfterResetAssertion struct {
	Reset    Expression // active-high reset signal
	Property Expression
	LineNum  uint32
}

func (p *PslAfterResetAssertion) isStatement()   {}
func (p *PslAfterResetAssertion) Line() uint32 { return p.LineNum }

type PortMapEntry struct {
	Formal string
	Actual Expression
}

// Expression is the interface for math/logic operations.
type Expression interface {
	isExpression()
}

type IdentifierExpr struct {
	Name string
}

func (IdentifierExpr) isExpression() {}

type LiteralExpr struct {
	Value string
}

func (LiteralExpr) isExpression() {}

type BinaryExpr struct {
	Op    string
	Left  Expression
	Right Expression
}

func (BinaryExpr) isExpression() {}

type UnaryExpr struct {
	Op    string
	Right Expression
}

func (UnaryExpr) isExpression() {}

type ParenExpr struct {
	Expr Expression
}

func (ParenExpr) isExpression() {}

type FunctionCallExpr struct {
	Name string
	Args []Expression
}

func (FunctionCallExpr) isExpression() {}

type IndexedNameExpr struct {
	Prefix string
	Index  Expression
}

func (IndexedNameExpr) isExpression() {}

type SelectedNameExpr struct {
	Prefix string
	Suffix string
}

func (SelectedNameExpr) isExpression() {}

type PslImplicationExpr struct {
	Op    string // "|=>" or "|->"
	Left  Expression
	Right Expression
}

func (PslImplicationExpr) isExpression() {}

type PslSequenceExpr struct {
	Elements []PslSequenceElement
}

func (PslSequenceExpr) isExpression() {}

type PslSequenceElement struct {
	Expr       Expression
	Repetition *PslRepetition
}

type PslRepetition struct {
	Count int // if >= 0, it's [*count]
	Start int // if Count < 0, it's [*start to end]
	End   int
}

type PslEventuallyExpr struct {
	Left  Expression
	Right Expression
}

func (PslEventuallyExpr) isExpression() {}

type PslStableExpr struct {
	Signal Expression
}

func (PslStableExpr) isExpression() {}

// PslNextExpr represents next[N](expr)
type PslNextExpr struct {
	Delay int
	Arg   Expression
}

func (PslNextExpr) isExpression() {}

// PslNextRangeExpr represents next_a[M to N](expr)
type PslNextRangeExpr struct {
	Start int
	End   int
	Arg   Expression
}

func (PslNextRangeExpr) isExpression() {}
