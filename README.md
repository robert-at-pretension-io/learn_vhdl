# VHDL Compiler

A learning-focused VHDL compiler built with Rust and Tree-sitter.

## What Is This?

This project has two goals:

1. **Learn VHDL** by building a compiler for it
2. **Learn compiler construction** using modern, declarative tools

The philosophy is "learn by doing" — instead of just reading about VHDL, we build a tool that understands it. Every grammar rule teaches us something about the language.

## Why Build a Compiler to Learn a Language?

When you write a compiler, you must understand the language at a deeper level than when you just use it:

- **Syntax**: What sequences of tokens are valid? What's optional? What's required?
- **Semantics**: What does each construct actually mean?
- **Edge cases**: What happens with unusual but legal code?

The tight feedback loop of "write grammar rule → see what parses → fix errors" is an incredibly effective way to learn.

## Quick Start

```bash
# Install dependencies
cd tree-sitter-vhdl && npm install && cd ..

# Run the development loop (watches for changes)
./dev.sh

# Or run once
./dev.sh --once
```

Edit `tree-sitter-vhdl/grammar.js` to add grammar rules. Edit `test.vhdl` to test different VHDL constructs. The dev script will automatically rebuild and show you parse errors.

## Project Structure

```
.
├── tree-sitter-vhdl/
│   ├── grammar.js          # The grammar definition (your main workspace)
│   ├── package.json        # Node.js config for tree-sitter CLI
│   ├── Cargo.toml          # Rust crate config
│   └── bindings/rust/      # Rust bindings to the generated parser
├── src/
│   └── main.rs             # Rust CLI that runs the parser
├── test.vhdl               # Kitchen sink test file with VHDL examples
├── dev.sh                  # Build & watch script
├── AGENTS.md               # Detailed architecture documentation
└── grammar_dev.md          # Tree-sitter grammar writing guide
```

## The Development Loop

1. **Look at test.vhdl** — Find a construct you don't understand yet
2. **Run `./dev.sh`** — See ERROR nodes for unparsed constructs
3. **Add grammar rules** — Edit `grammar.js` to recognize the construct
4. **Watch the errors decrease** — Each cycle teaches you more

Example:
```bash
# See what's broken
./dev.sh --once
# Output: ERROR at 9:1 "library ieee;"

# Add the library_clause rule to grammar.js
# Run again...
# Output: ERROR at 10:1 "use ieee.std_logic_1164.all;"

# Add the use_clause rule... and so on
```

## Architecture Overview

The compiler uses a "Rust-Powered Declarative Stack":

| Phase | Tool | Purpose |
|-------|------|---------|
| **Parsing** | Tree-sitter | Turn text into syntax tree (error-tolerant) |
| **Dependencies** | Petgraph | Build dependency graph, topological sort |
| **Lowering** | Hand-written Rust | Convert syntax tree to semantic representation |
| **Type Checking** | Egglog | Equality saturation for type inference & overloading |
| **Flow Analysis** | Crepe | Datalog for latch detection, dead code, etc. |
| **Reporting** | Miette | Beautiful error messages with source spans |

See [AGENTS.md](AGENTS.md) for detailed architecture documentation.

## Core Philosophy: "Panic is Failure"

The compiler never crashes. It ingests code, understands what it can, and explicitly reports what it cannot:

- **User Errors**: "You wrote bad VHDL (latch inferred)."
- **System Edges**: "I don't know how to handle `generate` statements yet."

---

# Adapting This Project for Other Languages

This project structure can be copied to build compilers for any language. Here's how:

## Step 1: Fork the Structure

```bash
# Copy the project
cp -r vhdl_compiler my_language_compiler
cd my_language_compiler

# Rename the tree-sitter directory
mv tree-sitter-vhdl tree-sitter-mylang

# Update names in configuration files
```

## Step 2: Update Configuration Files

### `tree-sitter-mylang/grammar.js`

Replace the grammar with your language's syntax:

```javascript
module.exports = grammar({
  name: 'mylang',  // Change this

  extras: $ => [
    /\s+/,
    $.comment,
  ],

  rules: {
    // Start fresh with your language's entry point
    source_file: $ => repeat($._definition),

    _definition: $ => choice(
      $.comment,
      // Add your language's top-level constructs
    ),

    // Define comment syntax for your language
    comment: $ => token(seq('//', /.*/)),  // C-style
    // comment: $ => token(seq('#', /.*/)),   // Python-style
    // comment: $ => token(seq('--', /.*/)),  // SQL/VHDL-style
  }
});
```

### `tree-sitter-mylang/package.json`

```json
{
  "name": "tree-sitter-mylang",
  ...
}
```

### `tree-sitter-mylang/Cargo.toml`

```toml
[package]
name = "tree-sitter-mylang"
...
```

### `tree-sitter-mylang/bindings/rust/lib.rs`

```rust
extern "C" {
    fn tree_sitter_mylang() -> tree_sitter::Language;
}

pub fn language() -> tree_sitter::Language {
    unsafe { tree_sitter_mylang() }
}
```

### `tree-sitter-mylang/bindings/rust/build.rs`

```rust
fn main() {
    // ... same structure, just update the library name
    c_config.compile("tree-sitter-mylang");
}
```

### Root `Cargo.toml`

```toml
[package]
name = "mylang-compiler"
...

[dependencies]
tree-sitter = "0.22"
tree-sitter-mylang = { path = "./tree-sitter-mylang" }
```

### `src/main.rs`

```rust
// Change the import
use tree_sitter_mylang;

// Change the language loading
parser
    .set_language(&tree_sitter_mylang::language())
    .expect("Error loading MyLang grammar");
```

## Step 3: Create Your Test File

Create a "kitchen sink" test file with examples of every construct in your language:

```
test.mylang
```

Include comments explaining what's language-defined vs. convention (like we did in `test.vhdl`).

## Step 4: Start the Learning Loop

```bash
./dev.sh
```

Now iterate:
1. See an ERROR node
2. Look up that construct in your language's specification
3. Add a grammar rule
4. Repeat

## Tips for Different Language Types

### For C-like Languages (C, Java, Go, Rust)

- Expressions need careful precedence handling
- Statement terminators (`;`) vs. separators
- Block structure with `{` `}`

```javascript
// Precedence example
const PREC = {
  ASSIGN: 1,
  OR: 2,
  AND: 3,
  COMPARE: 4,
  ADD: 5,
  MUL: 6,
  UNARY: 7,
  CALL: 8,
};

binary_expression: $ => choice(
  prec.left(PREC.ADD, seq($.expression, '+', $.expression)),
  prec.left(PREC.MUL, seq($.expression, '*', $.expression)),
  // ...
),
```

### For Indentation-Sensitive Languages (Python, Haskell, YAML)

Tree-sitter can handle these with an external scanner:

```javascript
module.exports = grammar({
  // ...
  externals: $ => [
    $._indent,
    $._dedent,
    $._newline,
  ],
  // ...
});
```

You'll need to write a C file (`src/scanner.c`) to track indentation.

### For Lisp-like Languages (Scheme, Clojure)

These are actually the easiest — mostly just parentheses and atoms:

```javascript
rules: {
  source_file: $ => repeat($._form),
  
  _form: $ => choice(
    $.list,
    $.vector,
    $.atom,
  ),
  
  list: $ => seq('(', repeat($._form), ')'),
  vector: $ => seq('[', repeat($._form), ']'),
  atom: $ => choice($.symbol, $.number, $.string),
  
  symbol: $ => /[a-zA-Z_+\-*\/<>=!?][a-zA-Z0-9_+\-*\/<>=!?]*/,
  number: $ => /\d+/,
  string: $ => /"[^"]*"/,
}
```

### For Markup Languages (HTML, XML, Markdown)

Watch out for:
- Nested structures
- Mixed content (text + elements)
- Special characters and escaping

## Resources

### Tree-sitter Documentation
- [Official Docs](https://tree-sitter.github.io/tree-sitter/)
- [Creating Parsers](https://tree-sitter.github.io/tree-sitter/creating-parsers)

### Example Grammars to Study
- [tree-sitter-python](https://github.com/tree-sitter/tree-sitter-python)
- [tree-sitter-rust](https://github.com/tree-sitter/tree-sitter-rust)
- [tree-sitter-javascript](https://github.com/tree-sitter/tree-sitter-javascript)

### Testing Your Grammar
- **GHDL test suite** (for VHDL): Thousands of edge-case tests
- **Language compliance suites**: Most languages have official test suites
- **Real-world codebases**: Try parsing popular open-source projects

## License

MIT

## Contributing

This is a learning project. If you're using it to learn a language, consider contributing:
- Grammar improvements
- Better error messages
- Documentation of language constructs
- Test cases for edge cases
