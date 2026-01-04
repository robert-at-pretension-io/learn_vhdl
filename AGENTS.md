Here is the definitive blueprint for your **Coverage-Driven VHDL Compiler**. This system is designed to be resilient, incremental, and mostly declarative.

It turns the overwhelming task of "building a compiler" into a manageable game of "filling in the blanks."

### The Core Philosophy: "Panic is Failure"

Your compiler never crashes. It ingests code, understands what it can, and explicitly reports what it cannot.

* **User Errors:** "You wrote bad VHDL (Latch inferred)."
* **System Edges:** "I don't know how to handle `generate` statements yet."

---

### The Architecture: The "Rust-Powered Declarative Stack"

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
* Topological Sort  The Build Order.
* **The Edge:** If a file requests a library that doesn't exist in the graph, report a "Missing Dependency" edge.



### Phase 3: The Bridge "Lowering" (Rust)

* **The Goal:** Translate syntax (Tree) into logic (S-Expressions & Facts).
* **The Tech:** **Hand-written Rust**.
* **The Task:** This is the glue. You write match arms to convert Tree-sitter nodes.
* *Normalization:* `rising_edge(clk)`  `(And (Event clk) (Eq clk '1'))`.
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

### The "Why" Matrix (Your Defense)

| Problem Domain | Chosen Tech | Why this specifically? |
| --- | --- | --- |
| **Syntax** | **Tree-sitter** | Handles precedence and recovery automatically. Pure config. |
| **Dependencies** | **Petgraph** | Standard graph algo library. Fast. |
| **Type/Math** | **Egglog** | Solves "Ambiguity" (Overloading) better than any manual algorithm. |
| **Control Flow** | **Crepe** | Datalog is  faster than Egglog for "Reachability" problems. |
| **Architecture** | **Rust** | Safety, speed, and the ecosystem glue that holds it all together. |

---

### The Developer's "Game Loop"

1. **Ingest:** Add a new VHDL feature to your test corpus (e.g., `case` statements).
2. **Phase 1 Check:** `tree-sitter test`.
* *Result:* `ERROR` nodes found.
* *Action:* Edit `grammar.js` to define `case_statement`.


3. **Phase 3 Check:** `cargo run` (Lowering).
* *Result:* "Edge: Missing Lowering for `case_statement`".
* *Action:* Edit `lowering.rs`. Map `case` branches to Datalog `Edge` facts (for Crepe) and Egglog `Switch` terms (for Semantics).


4. **Phase 4/5 Check:** `cargo run` (Analysis).
* *Result:* "Latch Detected" (User Error) OR "Stuck Term" (System Edge).
* *Action:* Add the semantic rules to `semantics.egg` or flow rules to `checks.dl`.


5. **Done:** The feature is now fully supported.

### Final "Gotcha" Cheat Sheet

* **Aggregates** (`others => '0'`): Use **Egglog**. They need "Context" (looking up at the parent) to be resolved.
* **Latches** (`if` without `else`): Use **Crepe**. This is a "Path Finding" problem, not a math problem.
* **Attributes** (`x'range`): Use **Rust Lowering**. You must match on the string name to know if it's a value, a type, or a range *before* sending it to the engines.

You now have the complete map. Go forth and build!