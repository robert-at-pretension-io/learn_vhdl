# VHDL Linter Rule Reference

This document describes every lint rule implemented in the VHDL linter.
Rules are organized by category. Each entry explains **what** the rule checks,
**why** it matters, and at what **severity** it fires.

> **Optional rules** are disabled by default. Enable them in `vhdl_lint.json`
> under `lint.rules` by setting the rule ID to `"warning"`, `"error"`, or
> `"info"`. Any rule can also be turned off with `"off"`.

---

## Table of Contents

1. [Project Structure](#1-project-structure)
2. [Dependencies & Imports](#2-dependencies--imports)
3. [Cross-File Semantics](#3-cross-file-semantics)
4. [Subprogram Resolution](#4-subprogram-resolution)
5. [Signals & Ports](#5-signals--ports)
6. [Processes](#6-processes)
7. [Sensitivity Lists](#7-sensitivity-lists)
8. [Sequential Logic](#8-sequential-logic)
9. [Combinational Logic](#9-combinational-logic)
10. [Latches & Incomplete Assignments](#10-latches--incomplete-assignments)
11. [Finite State Machines](#11-finite-state-machines)
12. [Instances & Port Maps](#12-instances--port-maps)
13. [Clocks & Resets](#13-clocks--resets)
14. [Clock Domain Crossing (CDC)](#14-clock-domain-crossing-cdc)
15. [Reset Domain Crossing (RDC)](#15-reset-domain-crossing-rdc)
16. [Synthesis](#16-synthesis)
17. [Power](#17-power)
18. [Security (Trojan Detection)](#18-security-trojan-detection)
19. [Naming & Style](#19-naming--style)
20. [Quality & Hierarchy](#20-quality--hierarchy)
21. [Types](#21-types)
22. [Configurations](#22-configurations)
23. [Testbenches](#23-testbenches)
24. [Verification Contracts](#24-verification-contracts)
25. [Dead Code](#25-dead-code)
26. [Variables](#26-variables)
27. [Extended Dead Code](#27-extended-dead-code)
28. [Extended Quality](#28-extended-quality)

---

## 1. Project Structure

In VHDL, a design is split into **entities** (the interface — like a class's
public API), **architectures** (the implementation behind an entity), and
**packages** (shared type/function libraries, similar to modules). These design
units live in files and are grouped into **libraries** (namespaces). Getting
this organizational structure wrong means the design won't compile or will
silently use the wrong implementation, so these rules catch file-level
structural problems early.

### `entity_has_ports` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

Flags any entity that declares zero ports. In synthesizable hardware, an entity
without ports cannot communicate with the outside world, which almost always
indicates a mistake — like writing a class with no public methods and no side
effects. Testbench entities (names ending in `_tb`, `_test`, etc.)
are automatically excluded since they legitimately have no ports.

### `entity_without_arch` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

Fires when an entity is declared but no architecture is ever defined for it.
An entity without an architecture is an incomplete design unit — it has an
interface but no implementation. Think of it as declaring a function prototype
but never writing the body. This usually means a file is missing from
the project or a declaration was left behind after refactoring.

### `architecture_has_entity` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

Fires when an architecture names an entity that does not exist anywhere in the
project. For example, `architecture rtl of my_core is` would trigger this rule
if no entity called `my_core` is found. This is always a hard error at the
language level (the design cannot elaborate), though the linter marks it
optional because incomplete projects may intentionally have dangling references.

### `duplicate_entity_in_library`

| Severity | Default |
|----------|---------|
| error    | on      |

Two files in the **same** library both declare an entity with the same name.
VHDL tools vary in how they handle this — some silently pick one, others error.
The linter treats it as an error because it leads to unpredictable elaboration
(the linker-like phase where the tool assembles the full design hierarchy).
The message tells you where the first declaration was seen and suggests either
splitting the variants into separate libraries or excluding one file in config.

### `duplicate_package_in_library`

| Severity | Default |
|----------|---------|
| error    | on      |

Same as above but for packages. Two package declarations with the same name
in the same library create ambiguity for any `use` clause that references
that package.

### `duplicate_architecture_in_library` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

Two architectures for the **same** entity with the **same** architecture name
in the same library. While VHDL allows multiple architectures per entity
(somewhat like having multiple implementations of an interface), having
two with identical names in one library is always a conflict.

### `duplicate_entity_in_file`

| Severity | Default |
|----------|---------|
| error    | on      |

A single file declares the same entity name more than once. This is never valid
VHDL and typically happens from a copy-paste error.

### `multiple_entities_per_file` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A single file contains more than one entity declaration. While legal VHDL, many
teams prefer a one-entity-per-file convention for clarity and tool compatibility
— similar to Java's one-public-class-per-file convention.

### `file_entity_mismatch` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The primary entity name does not match the filename. For example, a file called
`my_core.vhd` that defines entity `some_other_core`. Many teams and tools
expect these to match, much like Python expects a module's filename to match
its import name.

### `very_long_file` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The file contains an unusually large number of design units, suggesting it
should be split into smaller files for maintainability.

### `package_body_without_declaration`

| Severity | Default |
|----------|---------|
| error    | on      |

A `package body` exists but no matching `package` declaration was found in the
same library. In VHDL, a package body provides implementations for subprograms
declared in the package header — like a `.c` file without a corresponding `.h`.
The body is useless without its declaration and will fail during analysis.

---

## 2. Dependencies & Imports

VHDL designs compose by importing packages and referencing entities across
**libraries** (namespace containers configured in the project). A `library`
clause makes a namespace visible; a `use` clause imports specific packages from
it. These rules check that every library, package, and context clause resolves
to something real in the project, catching the VHDL equivalent of broken
imports before elaboration.

### `unresolved_library` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A `library` clause names a library that is not mapped in the project config.
For example, `library my_lib;` when `my_lib` does not appear in
`vhdl_lint.json`. The message lists known libraries and suggests adding the
missing one to config.

### `unresolved_package` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A `use` clause references a package that cannot be found in the expected
library. For example, `use work.my_pkg.all;` when no file defines `my_pkg` in
the `work` library. This is the VHDL equivalent of `import foo.bar` when `bar`
doesn't exist. The message may suggest candidate packages found in other
libraries.

### `unresolved_dependency` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

An entity instantiation references a target that cannot be resolved. This is
the general form of "cannot find the entity you are trying to instantiate" —
like calling a constructor for a class that was never defined. The message
includes candidates if the entity name exists in a different library.

### `component_resolved` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A component instance references an entity that does not exist. This is similar
to `unresolved_dependency` but specifically for component-based instantiation
(as opposed to direct entity instantiation). Components are an older VHDL
indirection mechanism where you first declare a local "component" interface,
then bind it to an actual entity — similar to forward-declaring a class.

---

## 3. Cross-File Semantics

These rules require analyzing relationships **between** files — type
resolution, port matching across instantiation boundaries, subprogram
visibility, and more. They catch problems that only surface when the full
project is considered together, analogous to linker errors in compiled
languages. A design might compile file-by-file but fail when the tool tries
to connect everything during elaboration.

### `unresolved_type_mark`

| Severity | Default |
|----------|---------|
| error    | on      |

A type name used in a signal/port/variable declaration cannot be resolved
through the visible packages. For example, declaring `signal data : my_type`
when `my_type` is not available via any `use` clause. This means the design
will fail to compile — like using a type that was never imported.

### `unresolved_instance_binding`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance (component or direct entity instantiation) refers to a target
entity that cannot be found in the project. Unlike `unresolved_dependency`,
this rule uses the full cross-file binding analysis to resolve through
library mappings.

### `ambiguous_instance_binding`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance target resolves to **multiple** entities (e.g., the same entity
name exists in two libraries and the instance does not specify which one).
The design cannot elaborate because the tool does not know which entity
to use — similar to an ambiguous overload in C++ that the compiler cannot
resolve.

### `extra_port_connection`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance's port map connects a port name that does not exist on the target
entity. For example, `port map (clk => clock, bogus => sig)` when the entity
has no port called `bogus`. Think of it as passing a named argument that the
function doesn't accept.

### `missing_port_connection`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance's port map omits a required port — one that has no default value
and is not explicitly left `open`. Every non-defaulted port must appear in the
port map, just like a required function parameter cannot be omitted.

### `port_direction_mismatch`

| Severity | Default |
|----------|---------|
| error    | on      |

The direction of a signal connected to a port does not match the port's
declared direction. For example, connecting an `out` signal to an `in` port.
In hardware, ports have physical direction — `in` means the wire carries data
*into* the block, `out` means it drives data *out*. Connecting them backwards
would mean two drivers fighting over the same wire or an input left floating.

### `port_type_mismatch`

| Severity | Default |
|----------|---------|
| error    | on      |

The type of the actual signal connected in a port map does not match the
formal port's declared type. For example, connecting a `std_logic_vector` to
an `unsigned` port. Even when both are bit arrays of the same width, VHDL's
strong type system treats them as incompatible.

### `extra_generic`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance's generic map specifies a generic that does not exist on the
target entity. Generics are VHDL's compile-time parameters (like template
arguments or constructor parameters that configure a module's width, depth,
etc.).

### `missing_generic`

| Severity | Default |
|----------|---------|
| error    | on      |

An instance's generic map omits a generic that has no default value. Generics
without defaults must be explicitly mapped, just like required template
parameters.

### `generic_type_mismatch`

| Severity | Default |
|----------|---------|
| error    | on      |

The value provided for a generic does not match the generic's declared type.

### `cross_file_assignment_type_mismatch`

| Severity | Default |
|----------|---------|
| error    | on      |

A signal assignment crosses a type boundary that the type resolver can detect.
For example, assigning an `unsigned` expression to a `signed` signal. While
both are bit vectors, VHDL treats signed and unsigned as distinct types with
different arithmetic semantics, and an implicit conversion would silently
reinterpret the bits.

### `unresolved_unqualified_call`

| Severity | Default |
|----------|---------|
| error    | on      |

A procedure or function is called by its short name (not prefixed with a
package name), and the linter cannot find any matching declaration in the
visible packages. For example, calling `DoSomething(x)` when no visible
package declares a procedure called `DoSomething`. The message lists which
packages are visible and may suggest candidates found in other libraries.

### `ambiguous_unqualified_call`

| Severity | Default |
|----------|---------|
| warning  | on      |

A procedure or function call by short name matches **multiple** declarations
in different visible packages, making it ambiguous which one will be called.
This is like having two wildcard imports that both provide the same function
name. The message lists the candidates. Resolution options: qualify the call
(`pkg.DoSomething(x)`) or remove a conflicting `use` clause.

### `unresolved_import` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

Umbrella configuration ID that controls both `unresolved_library` and
`unresolved_package` rules together. Setting `unresolved_import` to `"off"`
disables both; setting it to `"warning"` downgrades both. Useful when you want
a single toggle for all import-resolution diagnostics.

### `unresolved_subprogram` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

An unqualified subprogram call (function or procedure called by short name)
cannot be resolved through any visible package. This is the configuration alias
that controls `unresolved_unqualified_call` diagnostics. Use this ID in config
to adjust severity for all unresolved-call findings at once.

### `ambiguous_subprogram` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An unqualified subprogram call matches declarations in multiple visible
packages, making it ambiguous. This is the configuration alias that controls
`ambiguous_unqualified_call` diagnostics.

### `rule_skipped` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Emitted when a rule wanted to fire but lacked enough type information to make
a definitive judgment. This is an informational diagnostic letting you know the
analysis was inconclusive rather than definitively safe. Common cases include
ambiguous calls where overloads differ only by subtype constraints.

---

## 4. Subprogram Resolution

VHDL **subprograms** are functions and procedures — the equivalent of methods
or free functions. Functions return values and (when pure) must not have side
effects; procedures can modify their arguments via `out` and `inout` modes.
These rules enforce parameter-mode legality and catch calls to subprograms
that don't exist where expected.

### `function_param_invalid_mode` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A **pure** function has a parameter with a mode other than `in`. VHDL requires
that all parameters of a pure function be mode `in` — they cannot have `out`,
`inout`, or `buffer`. This is the language enforcing referential transparency:
a pure function must not modify its arguments. (Impure functions are exempt
from this check.)

### `procedure_param_invalid_mode` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A procedure has a parameter with mode `buffer` or `linkage`. These modes are
not valid for procedure parameters (only `in`, `out`, and `inout` are allowed).
`buffer` and `linkage` are port-level concepts that have no meaning in a
subprogram context.

### `unresolved_qualified_function_call`

| Severity | Default |
|----------|---------|
| error    | on      |

A fully qualified function call like `pkg.my_func(x)` refers to a function
that does not exist in the named package. The package exists but the function
name was not found among its declarations.

### `unresolved_qualified_procedure_call`

| Severity | Default |
|----------|---------|
| error    | on      |

Same as above but for procedure calls. `pkg.my_proc(x)` when `my_proc` is
not declared in `pkg`.

---

## 5. Signals & Ports

In hardware, **signals** are physical wires — they carry values between
components and persist across time. **Ports** are the external pins of a
component: `in` ports receive data, `out` ports drive data, and `inout` ports
are bidirectional (think I2C data lines). Unlike software variables, every
signal consumes real routing resources on the chip, and every undriven wire
or unused port is wasted silicon. These rules catch wiring mistakes that would
cause undefined behavior, resource waste, or compilation failures.

### `unused_signal` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal is declared but never read anywhere. It occupies hardware resources
without contributing to the design's behavior. Often left behind after
refactoring — like a declared-but-unused variable, except it also costs
physical chip area.

### `undriven_signal` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A signal is read but never assigned a value. In simulation it stays at its
initial value (usually `'U'` for "uninitialized"); in synthesis the tool may
optimize it away or tie it to a constant, causing the circuit to behave
differently from simulation.

### `multi_driven_signal` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal is assigned in multiple places (e.g., two different processes or a
process and a concurrent assignment). Unless the signal type has a resolution
function (like `std_logic`, which resolves conflicts using signal-strength
rules), this creates a driver conflict — two circuits trying to force a wire
to different voltages, which in real hardware can damage the chip.

### `undeclared_signal_usage` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal name is referenced but no declaration for it can be found in the
current scope. This may indicate a typo or a missing declaration.

### `input_port_driven` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

An input port is assigned a value inside the architecture. Input ports are
read-only — driving them is illegal in VHDL. In hardware terms, an input pin
is connected to an external driver; trying to drive it internally creates a
short circuit.

### `undriven_output_port`

| Severity | Default |
|----------|---------|
| error    | on      |

An output port is never assigned a value. The output will be undefined, which
means anything connected to this pin in the larger system receives garbage.
This is almost certainly a bug.

### `unused_input_port` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An input port is declared but never read inside the architecture. It is wired
in but unused — the parent module routes a signal to this pin but the
implementation ignores it.

### `output_port_read`

| Severity | Default |
|----------|---------|
| info     | on      |

An output port is read inside the architecture. While allowed in VHDL-2008,
this was illegal in earlier standards (where you had to use an intermediate
signal and assign it to the port). Code using this feature may not compile
with older tool versions.

### `inout_as_input` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A bidirectional (`inout`) port is only ever read, never written. It could be
declared as `in` instead. Using `inout` unnecessarily forces the synthesis tool
to infer tri-state logic, which is more expensive than a simple input buffer.

### `inout_as_output` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A bidirectional (`inout`) port is only ever written, never read. It could be
declared as `out` instead.

### `wide_signal` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal is declared with an unusually large width (e.g., 128+ bits). This is
informational — wide signals are not wrong but may deserve scrutiny for
resource usage, since every bit becomes a physical wire that must be routed
across the chip.

### `duplicate_signal_name` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The same signal name appears in multiple different entities. While legal
(each entity has its own scope), it can cause confusion during debugging
when viewing waveforms in simulation.

### `duplicate_signal_in_entity`

| Severity | Default |
|----------|---------|
| error    | on      |

A signal is declared twice in the same entity/architecture scope. This is
always a compile error in VHDL.

### `duplicate_port_in_entity`

| Severity | Default |
|----------|---------|
| error    | on      |

A port name appears twice in the same entity declaration. Also always a
compile error.

### `port_width_mismatch` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A signal connected to a port in a port map has a different width than the port
expects. For example, connecting a 16-bit signal to an 8-bit port. In
hardware, width mismatches mean the physical wiring simply doesn't fit —
there's no implicit truncation or zero-extension like in some software
languages.

---

## 6. Processes

A **process** is VHDL's fundamental behavioral construct — a block of
sequential code (if/case/loop statements) that describes hardware behavior.
Unlike software functions, all processes in a design execute **concurrently**
with each other, like threads that all run in parallel. Each process typically
describes either a piece of combinational logic (output changes whenever input
changes) or sequential logic (output changes only on a clock edge). These
general process-level rules flag structural issues that make processes harder
to analyze or more error-prone.

### `complex_process` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A process assigns a large number of signals. Very complex processes are harder
to verify and maintain, and synthesis tools may produce suboptimal results when
too much logic is packed into one block. Consider splitting into multiple
processes, each responsible for fewer signals.

### `comb_process_no_default` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A combinational process contains a case statement that may not cover all
branches, but doesn't initialize signals with default values at the top. This
pattern is the classic cause of inferred latches (see
[Latches & Incomplete Assignments](#10-latches--incomplete-assignments) for why
latches are problematic). Assigning defaults before the case statement
guarantees every signal gets a value on every path.

### `process_label_missing` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A process has no label. Labels make waveform debugging and log messages much
easier to understand — without them, simulation tools show processes as
anonymous blocks, making it hard to trace which hardware generated a signal
change. Example: `p_decode : process(all)` instead of just `process(all)`.

---

## 7. Sensitivity Lists

A process's **sensitivity list** declares which signals cause it to wake up
and re-execute, similar to a dependency list in a reactive framework. When any
signal in the list changes, the process runs; if a signal the process reads is
*not* in the list, the process won't react to that signal's changes. Getting
this wrong is one of the most common sources of simulation/synthesis mismatch:
simulation respects the sensitivity list literally, but synthesis tools infer
the intent from the code and may produce hardware that ignores the list entirely.

### `sensitivity_list_incomplete` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A combinational process reads a signal that is **not** in its sensitivity list.
This means the process will not re-evaluate when that signal changes, causing
a simulation/synthesis mismatch: the simulator follows the sensitivity list
(process sleeps through the change), but the synthesized hardware reacts
instantly. This is one of the most common VHDL bugs.

### `sensitivity_list_superfluous` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal appears in the sensitivity list but is never read inside the process.
Not harmful but adds noise and suggests the process was modified without
updating its sensitivity list — like an unused import.

### `empty_sensitivity_combinational` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A combinational process has an empty sensitivity list. It will never
re-evaluate after initialization, effectively making it dead code in
simulation. The synthesized hardware may still work (synthesis ignores the
list), but the simulation/synthesis mismatch makes verification useless.

### `vhdl2008_sensitivity_all` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The process uses `process(all)`, a VHDL-2008 feature that automatically
includes all read signals in the sensitivity list. Purely informational —
this is the recommended modern approach because it eliminates sensitivity-list
bugs entirely.

### `long_sensitivity_list` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A combinational process has a very long explicit sensitivity list. Consider
using `process(all)` (VHDL-2008) instead — it's impossible to get wrong and
stays correct as the process body evolves.

---

## 8. Sequential Logic

**Sequential logic** is hardware that remembers state — its outputs depend not
only on current inputs but on what happened on previous clock edges. A clock
signal is a periodic square wave that acts as the heartbeat of the circuit;
on each rising (or falling) edge, flip-flops capture their input values and
hold them until the next edge. This is how hardware implements registers,
counters, and state machines. A **reset** signal forces registers to a known
starting state (analogous to initializing variables). These rules catch
problems in clocked process structure that lead to broken timing, missing
resets, or driver conflicts.

### `missing_clock_sensitivity` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A sequential process (one that uses `rising_edge`/`falling_edge`) does not have
the clock signal in its sensitivity list. The process will not trigger on clock
edges in simulation, even though the synthesized hardware will — another
simulation/synthesis mismatch.

### `signal_in_seq_and_comb` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A signal is assigned in both a sequential process (clocked) and a combinational
process or concurrent assignment. This creates multiple drivers — two pieces
of hardware fighting over the same wire — and is almost always a design error.

### `missing_reset_sensitivity` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A sequential process tests a reset signal but the reset is not in the
sensitivity list. For asynchronous resets (resets that take effect immediately,
not waiting for a clock edge), this means the reset will be ignored in
simulation because the process never wakes up when reset changes.

### `very_wide_register` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A sequential process assigns a large number of signals on the clock edge. Each
assigned signal becomes a hardware register (flip-flop), so this may indicate
the process is consuming a lot of chip resources in a single block. Consider
splitting the logic or evaluating whether all those registers are necessary.

### `mixed_edge_clocking` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

Different processes in the same architecture use both rising and falling edges
of the same clock. Dual-edge designs (DDR-style) are unusual and require
careful timing analysis. More commonly, this indicates a bug where one process
was accidentally written with the wrong edge.

### `async_reset_naming` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A reset signal used asynchronously does not follow a naming convention that
indicates it is an asynchronous reset (e.g., `arst`, `areset_n`). Clear naming
helps reviewers quickly distinguish between synchronous resets (sampled on the
clock edge) and asynchronous resets (take effect immediately).

---

## 9. Combinational Logic

**Combinational logic** is hardware with no memory — outputs are a pure
function of current inputs, like a math expression evaluated continuously.
Examples: an adder, a multiplexer, a decoder. Because combinational circuits
have no clock, any feedback loop (output feeding back to input) creates an
unstable or oscillating circuit rather than useful storage. These rules
detect feedback loops and structural issues in combinational processes that
lead to broken hardware or unintended latches.

### `combinational_feedback` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A combinational process reads and writes the same signal. This creates a
feedback loop — the output feeds back into the input with no clock edge to
break the cycle. In hardware, this can oscillate or settle at an
unpredictable value. It may also cause synthesis to infer a latch.

### `direct_combinational_loop` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A signal's value depends on itself directly within the same combinational
process (e.g., `a <= a + 1` outside a clocked process). This creates an
infinite loop — in hardware, the signal tries to increment itself
continuously with no clock to pace the updates.

### `two_stage_combinational_loop` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A combinational feedback loop through two signals: A depends on B and B
depends on A, both in combinational processes. Even though the loop goes
through two signals, there's no register (clock edge) to break the cycle,
so it's still unstable.

### `three_stage_combinational_loop` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

Same as above but with three stages: A -> B -> C -> A through combinational
logic.

### `cross_process_combinational_loop` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A combinational loop exists between two separate combinational processes.
Process P1 writes signal A and reads signal B; process P2 writes B and reads A.
Since both processes are combinational (no clock), this creates a feedback loop
across process boundaries.

### `potential_combinational_loop` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

The linter detected a pattern that **might** be a combinational loop but cannot
confirm it definitively. Manual review recommended.

### `large_combinational_process` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A combinational process reads and writes a large number of signals, making it
hard to verify there are no latches or feedback loops hiding in the logic.

---

## 10. Latches & Incomplete Assignments

A **latch** is a level-sensitive storage element — it holds a value whenever
its enable signal is inactive, and passes through its input when the enable is
active. Unlike a flip-flop (which captures data only on a clock *edge*), a
latch is transparent whenever its enable is high, making it sensitive to
glitches and very difficult to time correctly. In FPGA and most ASIC design,
latches are almost never wanted. They get **inferred accidentally** when a
combinational process doesn't assign a signal on every possible code path:
the synthesis tool sees "this signal needs to keep its old value sometimes"
and creates a latch. These rules detect the patterns that cause this.

### `potential_latch` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A `case` statement inside a **combinational** process does not have a
`when others =>` clause. If the case expression takes a value not covered by
the explicit choices, the signal retains its old value — this infers a latch
in synthesis. This is one of the most important rules for synthesizable code.

### `incomplete_case_latch`

| Severity | Default |
|----------|---------|
| warning  | on      |

Similar to `potential_latch` but fires for any case statement (not just those
in combinational processes) that lacks `when others`. Even in a sequential
context, an incomplete case can cause unexpected value retention.

### `enum_case_incomplete` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A case statement over an enumeration type is missing one or more literal values
AND has no `when others`. This is a stronger version of `incomplete_case_latch`
that uses type information to check exact coverage — the linter knows all
possible enum values and can tell you exactly which ones are missing.

### `combinational_incomplete_assignment` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal is both read and written in a combinational process but may not be
assigned on all paths. Informational flag for manual review.

### `conditional_assignment_review` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A conditional signal assignment (`when ... else`) may not cover all conditions,
potentially inferring a latch. This is the concurrent (outside a process)
equivalent of an if-chain that's missing its final else.

### `selected_assignment_review` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A selected signal assignment (`with ... select`) may not cover all selection
values. This is the concurrent equivalent of an incomplete case statement.

### `combinational_default_values` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A combinational process does not initialize its output signals with default
values before the logic. Initializing outputs at the top of the process
prevents latches even if a branch is missed — the default ensures every signal
has a value on every path.

### `fsm_no_reset_state` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A state signal (used in an FSM pattern) has no reset value. Without a defined
reset state, the FSM starts in an unknown state after power-on, and the
hardware may behave unpredictably until it accidentally falls into a valid
state.

---

## 11. Finite State Machines

A **finite state machine** (FSM) is one of the most fundamental hardware
patterns — a circuit that moves between a finite set of named states based on
inputs and the current state. Think of a traffic light controller or a
protocol handler. In VHDL, FSMs are typically implemented with an enumerated
type for the states and a case statement that describes transitions. Getting
FSM structure wrong can mean the machine gets stuck, enters an illegal state,
or has dead states that waste silicon. These rules analyze FSM patterns for
common structural defects.

### `state_signal_not_enum` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal that behaves like an FSM state variable uses a vector type
(`std_logic_vector`, `unsigned`, etc.) instead of an enumerated type.
Enumerated types make FSMs more readable (states are named like `IDLE`,
`RUNNING` rather than `"01"`, `"10"`) and enable synthesis tools to choose
optimal state encodings (one-hot, binary, Gray code) automatically.

### `single_state_signal` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An FSM pattern uses a single `state` signal without a separate `next_state`
signal. Two-signal FSM style (`state`/`next_state`) is a common best practice
that cleanly separates the registered output (current state, updated on clock
edge) from the combinational next-state logic (pure function of inputs and
current state). The single-signal style can work but makes the boundary less
explicit.

### `fsm_missing_default_state` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

An FSM's case statement over the state signal does not have a `when others`
default branch. If the state signal takes an unexpected value (from a bug,
metastability, or a radiation-induced bit flip in aerospace designs), the FSM
has no recovery path and may stay stuck in an illegal state forever.

### `fsm_unhandled_state` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A specific enum literal (state) defined in the state type is not explicitly
handled by any `when` branch. It falls into `when others` silently, which
may mask an unintentional omission.

### `fsm_unreachable_state` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A state in the enumeration is never the target of an assignment. No transition
leads to this state, making it dead code — the state exists in the type
definition but the machine can never enter it.

---

## 12. Instances & Port Maps

An **instance** is a copy of another entity placed inside your design —
similar to creating an object from a class. The **port map** is how you wire
the instance's ports to signals in the current architecture, and the
**generic map** sets compile-time parameters (width, depth, etc.). Incorrect
wiring here means broken connections between modules that may be very hard to
debug in hardware, since you cannot set breakpoints on a physical wire. These
rules catch miswired, incomplete, or suspicious instantiation patterns.

### `sparse_port_map` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An instance has very few port connections relative to the target entity's port
count. This may indicate an incomplete instantiation where the designer forgot
to connect most of the ports.

### `empty_port_map` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An instance has no named port map entries at all. Either it uses positional
association (fragile) or was left incomplete.

### `instance_name_matches_component` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The instance label is the same as the component name (e.g., `my_fifo :
my_fifo`). While legal, unique instance labels improve readability —
especially when the same component is instantiated multiple times and you need
to distinguish them in simulation waveforms.

### `repeated_component_instantiation` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The same component is instantiated many times in one architecture. This may
warrant using a `generate` statement instead — VHDL's equivalent of a loop
that creates hardware instances parametrically.

### `many_instances` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture contains a large number of instances, suggesting it may be
doing too much structural work in a single file. Consider splitting into
sub-hierarchies.

### `hardcoded_port_value` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An instance port map connects a port directly to a literal value like `'0'`
or `"00000000"` instead of a named signal or constant. Hardcoded values make
the design less configurable and harder to change later.

### `open_port_connection` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An instance explicitly leaves a port `open`. For output ports this is fine
(the output is simply not used); for input ports it means the port uses its
default value (if any) and has no external connection.

### `floating_instance_input` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

An instance has an input port that is not connected to anything and has no
default value. The input will be undefined, and the sub-circuit will receive
random or uninitialized values.

### `hardcoded_generic` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A generic is mapped to a literal value instead of a constant or parameter.
Using named constants makes the design more maintainable and self-documenting.

### `positional_mapping` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An instance uses positional port/generic association instead of named
association. Positional mapping (connecting by order rather than by name)
breaks silently if the entity's port order changes — similar to how positional
function arguments are fragile when parameters get reordered.

### `instance_naming_convention` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The instance label does not follow a common naming convention (e.g., `u_`
prefix or `_inst` suffix). These prefixes help distinguish instance labels
from signal names when reading the code or simulation traces.

---

## 13. Clocks & Resets

The **clock** is the heartbeat of synchronous digital design — a periodic
signal whose edges tell flip-flops when to capture new data. The **reset**
is what forces all registers to a known starting state at power-on or during
error recovery. Getting either wrong can corrupt the entire design: a
glitchy clock causes timing violations (data captured at the wrong moment),
and a missing or improperly synchronized reset means registers start with
garbage values. These rules enforce clock and reset signal hygiene.

### `clock_not_std_logic` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A clock port is not declared as `std_logic`. Clocks should always be single-bit
`std_logic` signals to ensure clean edge detection. Using a wider type or
a non-standard type for a clock is almost certainly a mistake.

### `reset_not_std_logic` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A reset port is not declared as `std_logic`. Same rationale as clocks —
resets are inherently single-bit control signals.

### `multiple_clocks_in_process` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A single sequential process references multiple clock signals (e.g., both
`rising_edge(clk_a)` and `rising_edge(clk_b)`). A process should be clocked by
exactly one clock — using two clocks in one process confuses synthesis tools
and makes timing analysis impossible.

### `async_reset_active_high` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An asynchronous reset appears to be active-high (the reset takes effect when
the signal is `'1'`). Many design standards require active-low resets (reset
when `'0'`, written as `rst_n`) because active-low is safer against accidental
assertion from floating or uninitialized inputs. Informational only.

### `missing_reset` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A sequential process has no reset logic at all. Without a reset, registers
start with undefined values after power-on and the design relies on the
application to eventually drive them to valid states — which may never happen
for control signals.

---

## 14. Clock Domain Crossing (CDC)

A **clock domain** is a group of flip-flops all driven by the same clock
signal. When a signal produced in one clock domain is consumed in another,
it's called a **clock domain crossing** (CDC). This is one of the trickiest
problems in digital design: the receiving flip-flop may sample the signal
while it's changing, entering a **metastable** state — an electrical
"undecided" condition that can propagate as a random 0 or 1, corrupting
downstream logic. Metastability bugs are intermittent and nearly impossible
to reproduce, making them the hardware equivalent of a race condition.
Proper synchronization circuits (typically a chain of 2+ flip-flops) allow
the metastable state to resolve before the value is used.

### `cdc_unsync_single_bit` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A single-bit signal written in one clock domain is read in another without
a synchronizer. Single-bit CDC crossings can be safely synchronized with a
two-flip-flop synchronizer — the simplest and most common CDC solution.

### `cdc_unsync_multi_bit` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A multi-bit signal (bus) crosses clock domains without synchronization.
Multi-bit crossings are much harder than single-bit because individual bits
may arrive at different clock cycles, creating a garbled intermediate value
(e.g., a bus transitioning from `0111` to `1000` might momentarily read as
`1111`). These require special techniques: gray-code encoding, handshake
protocols, or asynchronous FIFOs.

### `cdc_insufficient_sync`

| Severity | Default |
|----------|---------|
| warning  | on      |

A CDC synchronizer chain has fewer than the recommended number of flip-flop
stages (typically 2). A single stage is not sufficient to reliably resolve
metastability — the probability of failure decreases exponentially with each
additional stage.

### `signal_crosses_clock_domain` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

General flag for any signal that the analysis determines crosses a clock domain
boundary. Use this to get a full inventory of all CDC crossings in the design,
which is the starting point for a thorough CDC review.

---

## 15. Reset Domain Crossing (RDC)

Just as signals crossing clock domains need synchronization, **reset signals**
that span multiple clock domains need special care. If a reset is released
(de-asserted) at an arbitrary time relative to a clock domain's clock, the
flip-flops in that domain may see the reset release at different clock edges,
causing some to leave reset before others. This produces an inconsistent state
that can hang the design. These rules flag reset signals that are used across
domains without proper synchronization or that are generated by unreliable
combinational logic.

### `reset_crosses_domains`

| Severity | Default |
|----------|---------|
| error    | on      |

A reset signal is used in processes clocked by different clock domains. Each
clock domain should have its own reset synchronizer to ensure clean,
synchronized de-assertion in that domain.

### `combinational_reset_gen` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A reset signal is generated by combinational logic (not registered). Glitches
on combinational logic — momentary wrong values caused by different input
signals arriving at slightly different times — can cause spurious resets that
corrupt the design state.

### `async_reset_unsynchronized` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An asynchronous reset is not passed through a synchronizer before use. While
assertion can be asynchronous (the reset takes effect immediately), de-assertion
should be synchronous to the clock domain to prevent different flip-flops from
leaving reset on different clock edges.

### `partial_reset_domain` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

Some processes in a clock domain have reset logic while others do not. This
inconsistency may mean some registers are left in unknown states after reset,
while others are cleanly initialized — leading to a partially valid state.

### `short_reset_sync` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A reset synchronizer uses only a single flip-flop stage. Best practice
requires at least two stages, just like CDC synchronizers, to reliably
resolve metastability on the reset release edge.

---

## 16. Synthesis

**Synthesis** is the process of converting VHDL code into actual hardware
gates, flip-flops, and wires — analogous to compilation in software. Not
all valid VHDL is synthesizable (e.g., `wait for 10 ns` works in simulation
but has no hardware equivalent), and even synthesizable code can produce poor
results if written carelessly. These rules catch patterns that cause synthesis
problems: gated clocks that create timing hazards, missing resets on critical
paths, unregistered outputs that make timing closure harder, and other issues
that affect the quality of the generated hardware.

### `gated_clock_detection` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A clock signal is assigned inside logic (e.g., `clk_out <= clk_in and enable`).
Gated clocks — where combinational logic sits between the clock source and
flip-flop clock inputs — can produce glitches that cause spurious clock edges.
Use clock enable logic inside the sequential process instead (where the clock
stays clean and the enable gates the data path), or use dedicated clock gating
cells provided by the technology library.

### `very_wide_bus` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal is declared very wide. Extremely wide buses consume significant routing
and register resources on the chip, and long buses may cause timing issues
because the signal must physically travel across more chip area.

### `critical_signal_no_reset` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal that appears to be on a critical path (e.g., an output port) has no
reset value. It will start with an unknown value after power-on, which means
the module's outputs could drive garbage into downstream logic until the
first valid assignment arrives.

### `combinational_reset` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A reset signal is generated by combinational logic rather than being a primary
input or registered output. Informational — see `combinational_reset_gen` for
the error-level version.

### `potential_memory_inference` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal pattern may cause the synthesis tool to infer a memory block (RAM/ROM)
instead of individual flip-flops. This is informational — it may be intentional
(arrays often map to block RAM), but unintentional memory inference can consume
scarce RAM resources.

### `unregistered_output` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An output port is driven directly by combinational logic rather than a
register. Unregistered outputs change as soon as their inputs change (with
propagation delay), which makes timing unpredictable for downstream modules
that sample this output on their clock edge.

### `multiple_clock_domains` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An architecture contains processes clocked by different clock signals. This is
informational — multi-clock designs are common but require careful CDC
handling. If you're seeing this, make sure every signal crossing between
domains is properly synchronized (see [CDC](#14-clock-domain-crossing-cdc)).

---

## 17. Power

In hardware, every wire that toggles (switches between 0 and 1) consumes
**dynamic power** — the dominant power cost in modern chips. Unlike software
where an unused computation just wastes CPU cycles, an unguarded arithmetic
unit in hardware burns power on every clock cycle whether its result is needed
or not, because the transistors physically switch. **Operand isolation**
(gating an operation's inputs with an enable signal so they don't toggle when
the result is unused) and **clock gating** (stopping the clock to idle
flip-flops) are the primary techniques for reducing power. These rules flag
expensive operations that lack power-saving guards.

### `unguarded_multiplication` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A multiplication operation's operands are not gated by an enable signal. When
the result is not needed, the multiplier still toggles, wasting power. Gating
inputs with an enable signal ("operand isolation") can significantly reduce
dynamic power — a large multiplier can draw substantial current even when its
output is discarded.

### `unguarded_division` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

Same as above but for division, which is even more expensive and often
synthesized as an iterative circuit that runs for many clock cycles.

### `unguarded_exponent` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

Exponentiation without operand isolation. Exponential operations are rare in
hardware and very expensive when present.

### `power_hotspot` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A process contains many expensive arithmetic operations. It may be a power
hotspot that deserves clock gating or operand isolation to prevent unnecessary
switching.

### `combinational_multiplier` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A multiplier is in a combinational process (not registered). Combinational
multipliers are always active — their inputs ripple through the logic
whenever any source signal changes — while registered multipliers only
compute on clock edges.

### `weak_guard` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An expensive operation is guarded by an enable, but the guard appears to be
weak (e.g., the enable is not directly controlling the operation inputs, so
the operands may still toggle).

### `dsp_candidate_no_control` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A wide signal multiplication could benefit from being mapped to a **DSP block**
(dedicated multiplier hardware on FPGAs) but has no clock enable signal. DSP
blocks typically have enable inputs, and without them the tool may use generic
fabric logic instead, which is slower and less power-efficient.

### `clock_gating_opportunity` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A process is a candidate for clock gating — it has an enable condition that
could be used to gate the clock instead of using conditional logic. Clock
gating shuts off the clock to idle flip-flops, saving power from both the
flip-flops and all their downstream combinational logic.

---

## 18. Security (Trojan Detection)

**Hardware trojans** are malicious circuits secretly inserted into a design,
often by a compromised third-party IP block or supply chain attack. A typical
trojan has a **trigger** (a condition that activates the payload, such as
comparing a counter or input against a "magic number") and a **payload** (the
malicious action, such as leaking data or disabling a function). Because
hardware is burned into silicon, a trojan cannot be patched after fabrication.
These rules look for trigger-like patterns — comparisons against large,
seemingly arbitrary literal values — that are a well-known indicator of
hardware trojans in academic research and government certification processes.

### `magic_number_comparison` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A comparison against a large, seemingly arbitrary literal value (a "magic
number"). Hardware trojans often compare a counter or input against a specific
trigger value. The linter flags comparisons against literals wider than a
configurable threshold.

### `trigger_drives_output` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

The result of a comparison against a large literal directly drives an output
signal. This is the classic trojan payload pattern: a hidden condition that
activates when a specific input is seen, then changes an output to cause damage
or leak information.

### `multi_trigger_process` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A single process contains multiple comparisons against large literals. A
multi-condition trigger is more suspicious than a single comparison — it
suggests a deliberately crafted activation sequence.

### `large_literal_comparison` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A single comparison against a large literal. Less severe than
`magic_number_comparison` — it flags even when the comparison does not
obviously drive an output.

### `counter_trigger` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A counter-like signal is compared against a large literal. Trojans sometimes
wait for a counter to reach a specific value (a time bomb) before activating,
since counters are common in hardware and the trigger only fires after a long
period of normal operation.

### `inverted_trigger` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A comparison against a large literal uses an inverted operator (`/=`). Trojans
may use `signal /= trigger_value` to activate for all values **except** the
normal case, making the trigger harder to spot in testing since the malicious
path is the common one and the "normal" path is rare.

---

## 19. Naming & Style

Cosmetic and convention rules. In hardware teams, naming conventions matter
more than in typical software because signal names show up in waveform viewers,
timing reports, synthesis logs, and place-and-route tools — all outside the
source code. Consistent naming helps reviewers and downstream tools quickly
identify what a signal does. None of these rules affect correctness, but they
improve the readability and maintainability of the design.

### `naming_convention` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Entity names should use lowercase with underscores. Mixed case or all-caps
names trigger this rule. VHDL is case-insensitive, so naming conventions are
purely for human readability.

### `entity_naming` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Entity name does not follow the project's naming convention.

### `signal_input_naming` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Input ports should end with a suffix like `_i` to indicate direction at the
point of use. When you see `data_i` in the code, you immediately know it's an
input without looking at the port declaration.

### `signal_output_naming` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Output ports should end with `_o`.

### `active_low_naming` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An active-low signal (reset, enable, etc.) should end with `_n` to make its
polarity clear. Active-low means the signal's "active" state is `'0'`, which
is counterintuitive unless the name advertises it.

### `legacy_packages`

| Severity | Default |
|----------|---------|
| warning  | on      |

The design uses non-standard Synopsys packages like `std_logic_arith`,
`std_logic_unsigned`, or `std_logic_signed`. These were vendor-specific
packages from the 1990s that became widespread by convention but were never
part of the IEEE standard. They should be replaced with the IEEE-standard
`numeric_std` package, which provides the same functionality with clearer
semantics and no vendor lock-in.

### `large_entity` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An entity has a very large number of ports, suggesting the interface may be
too broad and should be refactored into smaller sub-modules or use record
types to group related signals.

### `architecture_naming_convention` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Architecture name is not one of the conventional names (`rtl`, `behavioral`,
`structural`, `sim`, etc.). By convention, `rtl` means register-transfer-level
synthesizable code, `behavioral` means simulation-only, and `structural` means
pure wiring of instances.

### `empty_architecture` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An architecture has no signals, processes, instances, or concurrent
assignments — it is completely empty. An empty implementation is usually a
placeholder that was never filled in.

### `entity_name_with_numbers` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Entity name contains digits (e.g., `fifo_v2`). Some teams prohibit version
numbers in entity names, preferring version control to track revisions.

---

## 20. Quality & Hierarchy

Rules about design organization and readability. In hardware, the design
**hierarchy** (how modules are nested and connected) maps directly to the
physical layout of the chip. A well-organized hierarchy makes synthesis, timing
analysis, and debugging easier; a flat or tangled hierarchy produces longer
build times and harder-to-close timing. These rules flag structural code smells
that indicate the design could benefit from reorganization.

### `trivial_architecture` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An architecture is essentially empty — it has no meaningful content. Unlike
`empty_architecture`, this also catches architectures with only trivial content
like a single constant assignment.

### `buffer_port` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A port uses the `buffer` direction mode. This is a deprecated VHDL feature
that was largely superseded by VHDL-2008 readable output ports. `buffer` was
originally needed to read back an output value, but it caused cascading
interface constraints because the mode had to propagate through the hierarchy.

### `unlabeled_generate` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A `generate` block has no label. Generate blocks create hardware
parametrically (like a compile-time for-loop that stamps out N copies of a
circuit). Labels are required for generate blocks in many VHDL standards and
make the design hierarchy easier to navigate in simulation and synthesis tools.

### `large_package` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A package is very large. Consider splitting it into smaller, focused packages
to reduce recompilation scope and improve readability.

### `short_signal_name` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal has a very short name (e.g., `a`, `x`). Short names reduce
readability, especially in waveform viewers where you're reading hundreds
of signal names.

### `long_signal_name` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal name is extremely long, hurting readability in a different way.

### `short_port_name` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A port has a very short name.

### `mixed_port_directions` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An entity has ports in mixed directions (some in, some out, some inout)
without clear grouping. Grouping inputs together, then outputs, makes the
interface easier to read. Informational only.

### `bidirectional_port` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A port is declared as `inout` (bidirectional). Bidirectional ports require
tri-state logic (a wire that can be driven, high-impedance, or received) and
are often restricted in modern FPGA designs because internal tri-state buses
are not supported — the synthesis tool must convert them to multiplexers.

### `many_signals` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An entity declares a very large number of internal signals, suggesting the
implementation is doing too much in one place.

### `deep_generate_nesting` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Generate blocks are nested several levels deep, making the design hierarchy
hard to follow. Deeply nested generates are the hardware equivalent of deeply
nested loops — technically correct but very hard to reason about.

### `magic_width_number` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A signal uses a non-standard width (not 1, 8, 16, 32, 64, etc.). While not
wrong, unusual widths may deserve a comment explaining why — they sometimes
indicate a magic constant that should be derived from a generic.

---

## 21. Types

VHDL has a strong, static type system. The language distinguishes between
`signed` (two's complement, can be negative) and `unsigned` (non-negative)
numeric types, requiring explicit conversions between them. This prevents
subtle arithmetic bugs — accidentally treating a signed value as unsigned
changes the meaning of the high bit — but mixing them carelessly leads to
verbose conversion code and potential errors.

### `mixed_signedness` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture uses both `signed` and `unsigned` types. Mixing signedness
requires explicit conversion and can be a source of subtle bugs — for example,
comparing a `signed` value to an `unsigned` value without conversion may
silently produce wrong results, similar to comparing `int` and `unsigned int`
in C.

---

## 22. Configurations

A **configuration** in VHDL selects which architecture to use for each entity
in the design hierarchy, and can also bind specific instances to specific
entity/architecture pairs. Think of it as a dependency-injection manifest that
lets you swap implementations (e.g., use a behavioral model for simulation
but an RTL model for synthesis) without changing the structural code. These
rules catch configurations that reference non-existent entities, architectures,
or instances.

### `configuration_missing_entity` — optional

| Severity | Default |
|----------|---------|
| error    | off     |

A configuration declaration names an entity that does not exist in the project.

### `configuration_missing_arch`

| Severity | Default |
|----------|---------|
| error    | on      |

A configuration references an architecture that does not exist for the named
entity. The entity exists but the specified implementation does not.

### `configuration_binding_missing_instance`

| Severity | Default |
|----------|---------|
| error    | on      |

A binding specification inside a configuration refers to an instance label that
does not exist in the target architecture. The configuration is trying to
override the implementation for an instance that was never created.

### `configuration_binding_mismatch`

| Severity | Default |
|----------|---------|
| error    | on      |

A configuration binding specifies an architecture that does not match the
bound entity. The named architecture exists but belongs to a different entity.

---

## 23. Testbenches

A **testbench** is a VHDL entity that exists only in simulation — it
instantiates the design under test, drives stimulus signals, and checks
outputs. Testbenches are the hardware equivalent of unit tests: they exercise
the design and verify its behavior. By convention, testbench entities have no
ports (they're self-contained simulation environments), and their architecture
names indicate simulation rather than synthesis (`sim`, `behavioral`, `test`).
These rules flag testbench entities that deviate from these conventions.

### `testbench_with_ports` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An entity whose name looks like a testbench (`_tb`, `_test`) has ports.
Testbenches should typically be port-less top-level entities, since they
generate their own stimulus internally.

### `entity_no_ports_not_tb` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

An entity has no ports but its name does not look like a testbench. If it is
meant to be synthesizable, it needs ports — otherwise it's a black box with
no way to interact with the rest of the design.

### `mismatched_tb_architecture` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A testbench entity has an architecture name that looks like a synthesis
architecture (e.g., `rtl`) rather than a simulation name (e.g., `sim`,
`behavioral`, `test`). While harmless, it's misleading — readers expect
`rtl` to mean synthesizable code.

### `tb_with_synth_arch` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

Similar to above — informational flag that a testbench uses a synthesis-style
architecture name.

---

## 24. Verification Contracts

The linter supports a **verification contract system** where designers annotate
their architectures with structured comments (`--@check`, `--@waive`) that
declare what **formal properties** should hold for recognized design patterns.
Formal verification mathematically proves that a property holds for all
possible inputs, unlike simulation which only tests specific scenarios. This
contract system bridges the gap: the linter detects known hardware constructs
(FSMs, handshake protocols, FIFOs, etc.) and checks that the designer has
declared the expected formal properties for each one. Think of it as a
type-system-like layer for design intent — the annotations say "I claim this
is a FIFO and it should never be read when empty," and the linter verifies
the annotation is present and correctly bound to the right signals.

### Recognized Constructs

The contract system automatically detects these hardware constructs:

| Construct | Bindings Required | Description |
|-----------|------------------|-------------|
| **FSM** | `state` (enum signal) | Finite state machine with enumerated state |
| **Counter** | `counter` (numeric signal) | Up/down counter |
| **Ready/Valid** | `valid`, `ready` (bit signals/ports) | AXI-style handshake interface — a standard protocol where the sender asserts `valid` when data is available and the receiver asserts `ready` when it can accept; the transfer occurs when both are high simultaneously |
| **FIFO** | `rd_en`, `empty` and/or `wr_en`, `full` | First-in-first-out buffer control signals |
| **Pulse** | `pulse` (bit signal/port) | Single-cycle pulse signal — asserted for exactly one clock cycle |
| **Arbiter** | `grants` and optionally `reqs` | One-hot arbiter grant signals — a circuit that decides which of several requestors gets access to a shared resource |
| **Reset Hygiene** | `signals` (signal/port list) | Reset behavior verification |

### Contract Rules

### `missing_verification_block`

| Severity | Default |
|----------|---------|
| warning  | on      |

An architecture contains one or more recognized constructs (FSM, ready/valid,
etc.) but has no `-- verification` block. Adding a verification block with
appropriate `--@check` tags ensures the design's properties are formally
tracked.

### `missing_verification_check`

| Severity | Default |
|----------|---------|
| warning  | on      |

A recognized construct exists in the architecture and a verification block
is present, but the required `--@check` tag for that construct is missing.
The message includes which check IDs are needed and suggested bindings.

### `invalid_verification_tag`

| Severity | Default |
|----------|---------|
| error    | on      |

A `--@check` comment line is malformed — the syntax could not be parsed.
Check the tag format: `--@check <id> scope=<arch> <binding>=<signal>`.

### `stray_verification_tag`

| Severity | Default |
|----------|---------|
| warning  | on      |

A `--@check` tag appears outside of any `-- verification` block. Tags must
be inside a labeled verification block to be associated with the architecture.

### `stale_verification_tag_binding`

| Severity | Default |
|----------|---------|
| error    | on      |

A `--@check` tag references a signal or construct that no longer exists or
does not match the current architecture. The tag is stale and needs updating —
similar to a stale test that references a deleted function.

### `scope_mismatch`

| Severity | Default |
|----------|---------|
| warning  | on      |

A `--@check` tag declares `scope=<arch_name>` but is located in a different
architecture. The tag will not be matched to the correct construct.

### `missing_liveness_bound`

| Severity | Default |
|----------|---------|
| error    | on      |

A verification tag for a liveness property (e.g., `rv.eventual_progress_bounded`)
requires an explicit bound parameter but none was provided. Liveness properties
assert that "something good eventually happens" — without a bound, the formal
tool cannot check whether "eventually" means 10 cycles or 10 billion.

### `missing_cover_companion`

| Severity | Default |
|----------|---------|
| warning  | on      |

A `--@check` tag asserts a property that should have a corresponding `cover`
companion tag. For example, `fsm.legal_state` should be accompanied by
`cover.fsm.transition_taken` to ensure the FSM is actually exercised. Without
cover properties, a formally "correct" design might vacuously pass because the
construct is never activated.

### `invalid_verification_waiver`

| Severity | Default |
|----------|---------|
| error    | on      |

A `--@waive` comment line is malformed.

### `ambiguous_construct`

| Severity | Default |
|----------|---------|
| warning  | on      |

The automatic construct detection found a pattern that could match multiple
construct types. Manual annotation via `--@check` tags is needed to resolve
the ambiguity.

### Check IDs

Each construct has specific check IDs:

| Check ID | Construct | Description |
|----------|-----------|-------------|
| `fsm.legal_state` | FSM | State signal always holds a valid enum value |
| `fsm.reset_known` | FSM | Reset drives state to a known value |
| `cover.fsm.transition_taken` | FSM | Every state transition is exercised |
| `rv.stable_while_stalled` | Ready/Valid | Data is stable while valid=1 and ready=0 |
| `rv.eventual_progress_bounded` | Ready/Valid | Transaction completes within bounded time |
| `cover.rv.handshake` | Ready/Valid | A handshake (valid=1, ready=1) actually occurs |
| `fifo.no_read_empty` | FIFO | Never read from an empty FIFO |
| `fifo.no_write_full` | FIFO | Never write to a full FIFO |
| `cover.fifo.activity` | FIFO | FIFO read and write activity observed |
| `ctr.range` | Counter | Counter stays within declared bounds |
| `ctr.step_rule` | Counter | Counter increments/decrements by expected step |
| `cover.ctr.moved` | Counter | Counter value actually changes |
| `reset.no_unknown_after_grace` | Reset | No unknown values after reset de-asserts |
| `reset.asserted_implies_known_defaults` | Reset | Signals have known values during reset |
| `pulse.width_one_cycle` | Pulse | Pulse is exactly one clock cycle wide |
| `cover.pulse.fired` | Pulse | Pulse actually fires at least once |
| `arb.onehot0` | Arbiter | Grant vector is one-hot or zero |
| `arb.no_grant_without_req` | Arbiter | No grant asserted without corresponding request |
| `cover.arb.grant_seen` | Arbiter | At least one grant is observed |

---

## 25. Dead Code

Dead code in hardware is not just untidy — it consumes FPGA resources (LUTs,
flip-flops, routing) and makes the design harder to understand and maintain.
Unlike software where an optimizer can eliminate dead code, synthesis tools may
or may not trim unused logic, and the presence of dead code can mask real
problems like misspelled signal names or forgotten connections. These rules
detect common patterns of dead code: signals that are written but never read,
constants/types/components/subprograms that are declared but never used.

All dead code rules are **optional** (off by default) because some projects
intentionally leave declarations for future use or documentation purposes.

### `write_only_signal` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal is assigned (driven) but never read by any process, concurrent
assignment, or port map. This means the signal's value is computed but
discarded — the flip-flops or combinational logic driving it serve no purpose.
Common causes include leftover debug signals, copy-paste errors, or signals
that were meant to connect to an instance port but never did. Output ports
are excluded (they are consumed externally). Testbench files are excluded.

### `unused_constant` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture-local constant is declared but never referenced in any
expression, comparison, port map, or generic map. Package-level constants
are excluded because they may be used by other files. This often indicates
a leftover from refactoring or a configuration constant that was superseded.

### `unused_component` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A component is declared in the architecture but no instance targets it.
In VHDL, a `component` declaration is a local "forward declaration" of an
entity's interface — it only matters if an instance references it. An unused
component declaration is dead weight that can confuse readers into thinking
the module is instantiated somewhere in the design.

### `unused_type` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture-local type (enum, record, array, etc.) is declared but no
signal, port, variable, constant, subtype, or subprogram parameter uses it.
Package-level types are excluded because they may be used cross-file. This
typically happens when a type was defined for an FSM or data structure that
was later removed or refactored.

### `unused_subprogram` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture-local function or procedure is declared but never called from
any process. Package-level subprograms are excluded because they may be called
from other files. Unused subprograms add clutter and may indicate incomplete
refactoring — the function was written but the calling code was removed.

---

## 26. Variables

Variable analysis catches common mistakes with process-local variables.
Variables in VHDL behave like software variables (immediate assignment) rather
than signals (scheduled update), so they have their own class of issues.

### `unused_variable` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A process-local variable is declared but never read or assigned in the process
body. This is dead code — the variable was likely left over from refactoring or
was declared for future use that never materialized. Unlike signals, variables
have no hardware effect, so an unused one is pure clutter.

### `variable_shadows_signal` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A process declares a variable with the same name as an architecture signal or
entity port. Inside the process, the variable takes precedence over the signal,
which can lead to subtle bugs: assignments that look like they drive the signal
actually modify the local variable. Renaming the variable eliminates the
ambiguity.

### `uninitialized_variable_read` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A process-local variable is read but never assigned within the process. In VHDL,
uninitialized variables of type `std_logic` default to `'U'` (uninitialized),
which propagates through simulation as unknown values. This almost always
indicates a missing assignment or copy-paste error.

---

## 27. Extended Dead Code

Additional dead code patterns beyond the base set in section 25.

### `unused_subtype` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An architecture-local subtype is declared but no signal, port, variable,
constant, or subprogram parameter references it. Package-level subtypes are
excluded because they may be used cross-file.

### `unused_generic` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A generic parameter on an entity is never referenced in any architecture
body — not in constant initializers, process expressions, comparisons, or
instance generic maps. This typically means the generic was added for future
configurability but never wired in.

### `record_field_unused` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A record type has a field that is never accessed via dotted notation in any
process, concurrent assignment, or port map. The field declaration is dead
weight and may indicate a partially implemented interface.

### `dead_generate` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A for-generate statement has a statically resolvable range that yields zero
iterations. The generate block produces no hardware, which is almost certainly
a configuration error (e.g., `for i in 0 to -1 generate`).

### `generate_range_mismatch` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A for-generate's iteration count doesn't match the width of a signal declared
inside it. For example, a generate that iterates 10 times over a signal that
is only 8 bits wide. This often indicates an off-by-one error or a
misconfigured generic.

### `procedure_body_missing` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A procedure is declared without a body outside of a package specification.
In package specs, bodyless declarations are normal (the body goes in the
package body). In architectures, a bodyless procedure means the implementation
is missing.

### `potential_overflow` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

An arithmetic operation (`+` or `-`) produces a result that is the same width
or narrower than its widest operand. Addition of two N-bit values can produce
an (N+1)-bit result; truncating it silently discards the carry. Only fires
when both operand and result widths are statically known.

---

## 28. Extended Quality

Additional code quality and design hygiene rules.

### `duplicate_use_clause` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

The same `use` clause appears more than once in the same file. Duplicate
imports are harmless but indicate sloppy copy-paste. They can also confuse
readers about whether different library versions are intended.

### `use_all_abuse` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A `use ... .all` import targets a non-standard package (not `ieee.std_logic_1164`,
`ieee.numeric_std`, or similar). Wildcard imports from user packages pollute the
namespace and can cause ambiguous name resolution. Prefer explicit imports for
non-IEEE packages.

### `signal_fanout` — optional

| Severity | Default |
|----------|---------|
| warning  | off     |

A signal is read by more than 10 distinct processes. High fanout signals can
cause timing closure problems in synthesis because the tool must replicate
drivers or insert buffers. Consider restructuring the design to reduce the
number of readers, or use explicit buffer trees.

### `unused_library_clause` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A `library` clause declares a library name (other than `ieee`, `std`, or
`work`) that is never referenced by any `use` clause, direct entity
instantiation, or context clause in the same file. Unused library clauses
are harmless but indicate dead imports that should be cleaned up.

### `unused_use_clause` — optional

| Severity | Default |
|----------|---------|
| info     | off     |

A `use` clause imports a package whose exported names (types, functions,
procedures, constants) are never referenced in the same file. The check
covers standard IEEE/STD packages with hardcoded export lists and user
packages whose contents are visible in the project. Unknown packages are
skipped to avoid false positives. Testbench files are excluded.
