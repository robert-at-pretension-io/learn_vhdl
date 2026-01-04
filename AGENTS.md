# VHDL Compiler Project

## Project Purpose

This is a **learning project** with two goals:

1. **Learn VHDL** by building a compiler for it
2. **Learn compiler construction** using modern, declarative tools

The philosophy is "learn by doing" - instead of just reading about VHDL, we build a tool that understands it. Every grammar rule we write teaches us something about the language.

## The Core Philosophy: "Panic is Failure"

The compiler never crashes. It ingests code, understands what it can, and explicitly reports what it cannot.

* **User Errors:** "You wrote bad VHDL (Latch inferred)."
* **System Edges:** "I don't know how to handle `generate` statements yet."

---

## The Architecture: The "Rust-Powered Declarative Stack"

### Phase 1: The Resilient Parser (Tree-sitter)

* **The Goal:** Turn text into a tree without crashing.
* **The Tech:** **Tree-sitter** (C-lib wrapped in Rust).
* **The Configuration:** `grammar.js`.
* **How it works:**
  * It uses a GLR algorithm that tolerates syntax errors.
  * It handles **Precedence** automatically (you define `*` > `+` in JS, not Rust).
* **The Edge:** If it sees syntax it doesn't know, it inserts an `(ERROR)` node and resynchronizes. Your Rust code scans for these nodes to report "Parsing Edges."

### Phase 2: The Librarian (Petgraph)

* **The Goal:** Figure out the compile order.
* **The Tech:** **Petgraph** + **Walkdir**.
* **The Logic:**
  * Scan the Tree-sitter tree for `library` and `use` nodes.
  * Build a DAG (Directed Acyclic Graph) of files.
  * Topological Sort → The Build Order.
* **The Edge:** If a file requests a library that doesn't exist in the graph, report a "Missing Dependency" edge.

### Phase 3: The Bridge "Lowering" (Rust)

* **The Goal:** Translate syntax (Tree) into logic (S-Expressions & Facts).
* **The Tech:** **Hand-written Rust**.
* **The Task:** This is the glue. You write match arms to convert Tree-sitter nodes.
  * *Normalization:* `rising_edge(clk)` → `(And (Event clk) (Eq clk '1'))`.
  * *Attribute Handling:* You match on the attribute string to decide if `x'range` becomes a **Value**, a **Type**, or a **Range**.
* **The Edge:** You use a catch-all match arm (`_ => ...`). If you hit a syntax node you haven't implemented yet, return `(Error "Missing Lowering")` instead of panicking.

### Phase 4: The Semantic Brain (Egglog)

* **The Goal:** Solve Equations (Types, Overloading, Constants).
* **The Tech:** **Egglog** (Equality Saturation).
* **The Configuration:** `.egg` files (Rewrite rules).
* **The Logic:**
  * **Type Inference:** `(Add Int Int)` becomes `Int`.
  * **Overloading:** It filters the list of possible functions for `"+"` until only one matches the argument types.
  * **Context:** It solves **Aggregates** (`others => '0'`) by looking at the target signal's type.
* **The Edge:** After running, query for "Stuck Terms." If `(Abs (Int 5))` didn't reduce, report "Missing Semantic Rule for Abs."

### Phase 5: The Flow Analyst (Crepe)

* **The Goal:** Analyze Paths (Latches, Dead Code, Uninitialized Variables).
* **The Tech:** **Crepe** (Datalog).
* **The Configuration:** Datalog Rules (embedded in Rust macro).
* **The Logic:**
  * You feed it **Facts**: `Edge(Block1, Block2)`, `Assigns(Block1, "signal_x")`.
  * You write **Rules**: "A Latch exists if there is a path from Start to End that never touches `signal_x`."
  * Crepe efficiently calculates the "Fixed Point" (all reachable states).
* **The Edge:** If Crepe returns an empty set for `Reachable(Start, End)`, your graph building logic (Phase 3) is likely broken.

### Phase 6: The Reporter (Miette)

* **The Goal:** Talk to the human.
* **The Tech:** **Miette**.
* **The Task:**
  * Take the "Edges" from Phases 1, 3, 4.
  * Take the "User Errors" from Phases 4, 5.
  * Map them back to the original source code spans.
  * Print beautiful, helpful diagnostics.

---

## The "Why" Matrix

| Problem Domain | Chosen Tech | Why this specifically? |
| --- | --- | --- |
| **Syntax** | **Tree-sitter** | Handles precedence and recovery automatically. Pure config. |
| **Dependencies** | **Petgraph** | Standard graph algo library. Fast. |
| **Type/Math** | **Egglog** | Solves "Ambiguity" (Overloading) better than any manual algorithm. |
| **Control Flow** | **Crepe** | Datalog is faster than Egglog for "Reachability" problems. |
| **Architecture** | **Rust** | Safety, speed, and the ecosystem glue that holds it all together. |

---

## The Developer's "Game Loop"

This is the tight feedback loop for learning:

1. **Look at test.vhdl** - Find a construct you don't understand yet
2. **Run `./dev.sh`** - See ERROR nodes for unparsed constructs
3. **Add grammar rules** - Edit `grammar.js` to recognize the construct
4. **Run `./dev.sh` again** - Errors should decrease
5. **Repeat** - Each cycle teaches you more VHDL

### Example Cycle

