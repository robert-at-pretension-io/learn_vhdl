### 2. The `grammar.js` File

This is where you will live. A basic grammar file looks like this:

```javascript
module.exports = grammar({
  name: 'vhdl',

  // 1. EXTRAS: Ignore these everywhere (whitespace, comments)
  extras: $ => [
    /\s/,
    /--.*/, 
  ],

  // 2. RULES: The definition of the language
  rules: {
    // The entry point (must be the first rule)
    source_file: $ => repeat($.design_unit),

    design_unit: $ => choice(
      $.entity_declaration,
      $.architecture_body
      // ... others
    ),

    // ... more rules ...
  }
});

```

### 3. The Vocabulary (How to write rules)

Tree-sitter uses a specific set of functions to describe structure. The `$` argument represents "all the rules in this grammar."

#### A. `seq` (Sequence)

"A follows B follows C."

* **Logic:** `A B C`
* **VHDL Example:** `entity my_name is ...`

```javascript
// Rule: "entity" + identifier + "is"
entity_head: $ => seq(
  'entity',
  $.identifier,
  'is'
)

```

#### B. `choice` (Alternatives)

"It can be A OR B OR C."

* **Logic:** `A | B | C`
* **VHDL Example:** A type can be `integer` OR `std_logic`.

```javascript
type_mark: $ => choice(
  'integer',
  'std_logic',
  'bit'
)

```

#### C. `repeat` and `repeat1` (Repetition)

* `repeat($.rule)`: Zero or more times (Like `*` in regex).
* `repeat1($.rule)`: One or more times (Like `+` in regex).

```javascript
// A list of signals inside a port
port_list: $ => repeat1($.signal_declaration)

```

#### D. `optional` (Maybe)

"This might exist, or not."

* **Logic:** `A?`
* **VHDL Example:** `variable x : integer := 0;` (The `:= 0` is optional).

```javascript
variable_decl: $ => seq(
  'variable',
  $.identifier,
  ':',
  $.type_mark,
  optional(seq(':=', $.expression)), // <--- Optional Init
  ';'
)

```

#### E. `field` (Naming Children)

**This is critical for your Rust Lowering phase.**
By default, children are just a list `[0, 1, 2]`. If you wrap a rule in `field('name', ...)`, you can access it in Rust via `child_by_field_name("name")`.

```javascript
entity_declaration: $ => seq(
  'entity',
  field('name', $.identifier), // <--- Named "name"
  'is',
  // ...
)

```

#### F. `token` (Regex)

If a rule is a "leaf" (raw text like an identifier or number), use regex inside `token()`.

```javascript
// Regex for a basic identifier (start with letter, then alpha-numeric)
identifier: $ => token(/[a-zA-Z][a-zA-Z0-9_]*/)

```

---

### 4. A Real VHDL Example: The `Entity`

Let's combine these to write the rule for a VHDL Entity.

**Target VHDL:**

```vhdl
entity MyChip is
    port (
        clk : in std_logic;
        rst : in std_logic
    );
end MyChip;

```

**The Tree-sitter Rule:**

```javascript
entity_declaration: $ => seq(
    'entity',
    field('name', $.identifier), // We want to extract this name later
    'is',
    
    // Ports are optional!
    optional(seq(
        'port',
        '(',
        $.port_list,
        ')',
        ';'
    )),

    'end',
    optional($.identifier), // Optional repetition of name
    ';'
),

port_list: $ => seq(
    $.interface_declaration,
    // Repeat: "; " + declaration
    repeat(seq(';', $.interface_declaration)) 
),

interface_declaration: $ => seq(
    field('ports', $.identifier_list),
    ':',
    field('direction', choice('in', 'out', 'inout')),
    field('type', $.type_mark)
)

```

### 5. Handling Precedence (The Math Problem)

As discussed before, you handle operator order here using `prec.left` (Left Associative) or `prec.right`.

```javascript
// Define levels first
const P = {
    ADD: 1,
    MUL: 2
}

// ... inside rules ...
expression: $ => choice(
    $.binary_expression,
    $.number,
    $.identifier
),

binary_expression: $ => choice(
    // 1 + 2
    prec.left(P.ADD, seq($.expression, '+', $.expression)),
    // 1 * 2
    prec.left(P.MUL, seq($.expression, '*', $.expression))
)

```

### 6. Your Workflow

This is how you verify your rules are working.

**Step 1: Install**

```bash
npm install tree-sitter-cli

```

**Step 2: Create `grammar.js**`
Paste your rules into the file.

**Step 3: Generate**

```bash
npx tree-sitter generate

```

**Step 4: Test (The Loop)**
Create a file `example.vhd` with some code. Run:

```bash
npx tree-sitter parse example.vhd

```

It will print the Lisp-like tree structure to your console.

* If it matches structure: **Success.**
* If you see `(ERROR ...)` nodes: **Debug.**

### Summary Cheat Sheet

| Syntax | Meaning | Example |
| --- | --- | --- |
| `seq(A, B)` | A then B | `seq('begin', $.statements)` |
| `choice(A, B)` | A OR B | `choice('integer', 'real')` |
| `repeat(A)` | 0 or more A | `repeat($.statement)` |
| `optional(A)` | A or nothing | `optional(':= 0')` |
| `field('x', A)` | Name this node 'x' | `field('lhs', $.expr)` |
| `token(/regex/)` | Raw text match | `token(/[0-9]+/)` |

Would you like to try writing the rule for a **Signal Assignment** (`x <= y + 1;`) to test your understanding?


there is no single file that contains every VHDL construct. VHDL is too massive (standards from 1987, 1993, 2000, 2002, 2008, and 2019) and context-dependent for one file to cover it all meaningfully.

However, you can get the next best thing: Compliance Test Suites and Major Open Source Libraries.

Here is where to find the "Stress Tests" you are looking for.

1. The GHDL Test Suite (The Gold Standard)
GHDL is the open-source VHDL simulator. They have spent years collecting weird edge cases.

Why use it: It contains thousands of small files, each testing a specific feature (e.g., test_block_statement.vhd, test_aggregates.vhd).

Strategy: Clone the GHDL repo. Point your tree-sitter parse loop at their testsuite/ folder. If you can parse all of GHDL's tests, your grammar is production-ready.

2. OSVVM and UVVM (The "Real World" Check)
These are the two industry-standard verification libraries. They use heavy advanced VHDL (generics, access types, massive overloading, weird configurations).

OSVVM (Open Source VHDL Verification Methodology): Uses a lot of VHDL-2008 features.

UVVM (Universal VHDL Verification Methodology): Massive codebase, very structured.

Strategy: If you can parse the osvvm package body without errors, your parser is better than 50% of commercial tools.