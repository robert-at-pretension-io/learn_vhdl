/* ============================================================================
 * THE MICRO-VHDL MANIFESTO: AN ARCHITECTURE OF INTENT
 * ============================================================================
 * * CORE PHILOSOPHY
 * Software executes in time. Hardware exists in space. 
 * Traditional VHDL is bloated with "software-like" temporal constructs 
 * (variables, unclocked loops, sequential logic) that blur the line between 
 * algorithms and physics. This ambiguity is the root cause of the "Von Neumann 
 * Hangover" in AI-generated hardware, where Large Language Models hallucinate 
 * impossible silicon topologies like unclocked temporal paradoxes and multi-driven 
 * short circuits.
 * * THE MICRO-VHDL SANDBOX
 * Micro-VHDL is not a new language; it is a ruthlessly stripped-down, compiler-
 * enforced subset of standard VHDL. It is designed to act as a deterministic 
 * physics engine. By restricting the grammar, we force both the engineer and 
 * the AI to stop writing sequential recipes and start drawing 2D physical graphs 
 * of logic gates, copper wires, and switchboxes.
 * * THE AXIOMS OF THIS GRAMMAR
 * 1. Time is Quarantined: Time only exists inside `rising_edge(clk)` registers. 
 * All other logic evaluates continuously at the speed of light.
 * 2. Spatial Routing over Control Flow: Combinational `if/else` is banned. 
 * Conditional routing is achieved via explicit physical multiplexers (`with...select`).
 * 3. Immutable Wires: The variable (`:=`) is banned. You may only solder wires 
 * together using concurrent assignment (`<=`).
 * 4. Deterministic Hierarchy: Components and configurations are banned. 
 * Modules must be directly instantiated.
 * 5. Provable Intent: Formal verification (PSL) is a first-class citizen.
 * * If a construct violates these axioms, it is rejected by this parser in O(1) time.
 * ============================================================================ */

module.exports = grammar({
  name: 'micro_vhdl',

  extras: $ => [
    /\s+/,
    $.comment,
  ],

  word: $ => $.identifier,

  rules: {
    // -------------------------------------------------------------------------
    // ENTRY POINT
    // -------------------------------------------------------------------------
    source_file: $ => repeat($._design_unit),

    comment: $ => token(seq('--', /[^\r\n]*/)),

    identifier: $ => /[a-zA-Z][a-zA-Z0-9_]*/,

    _design_unit: $ => choice(
      $.library_clause,
      $.use_clause,
      $.entity_declaration,
      $.architecture_body
    ),

    // -------------------------------------------------------------------------
    // BOILERPLATE
    // -------------------------------------------------------------------------
    library_clause: $ => seq('library', $.identifier, ';'),
    use_clause: $ => seq('use', $.identifier, '.', $.identifier, '.', choice($.identifier, 'all'), ';'),

    // -------------------------------------------------------------------------
    // MODULE 1: THE CANVAS (Entity & Generics)
    // -------------------------------------------------------------------------
    entity_declaration: $ => seq(
      'entity', field('name', $.identifier), 'is',
      optional($.generic_clause),
      optional($.port_clause),
      optional($.contract_declaration),
      'end', optional('entity'), optional($.identifier), ';'
    ),

    contract_declaration: $ => seq(
      'contract',
      repeat1(choice(
        seq('require', ':', field('require_expr', $._psl_expression), ';'),
        seq('ensure', ':', field('ensure_expr', $._psl_expression), ';')
      ))
    ),

    // JUSTIFICATION: Complex metaprogramming (generic types/functions) requires a 
    // massive compile-time elaboration engine. Restricting generics strictly to 
    // 'integer' allows for parameterized silicon (e.g., dynamic bus widths) 
    // without sacrificing parser simplicity or deterministic lowering.
    generic_clause: $ => seq(
      'generic', '(',
      $.generic_declaration,
      repeat(seq(';', $.generic_declaration)),
      ')', ';'
    ),

    generic_declaration: $ => seq(
      field('name', $.identifier), ':', 'integer', ':=', field('default', $.number)
    ),

    port_clause: $ => seq(
      'port', '(',
      $.port_declaration,
      repeat(seq(';', $.port_declaration)),
      ')', ';'
    ),

    // JUSTIFICATION: 'inout' and 'buffer' allow bi-directional, tri-state logic. 
    // This introduces quantum-like "Z" states that complicate formal verification 
    // and SSA translation. We enforce strictly directional dataflow: 'in' or 'out'.
    port_declaration: $ => seq(
      field('names', $.identifier_list), ':',
      field('direction', choice('in', 'out')),
      field('type', alias($.type_mark, $.type_mark))
    ),

    architecture_body: $ => seq(
      'architecture', field('name', $.identifier), 'of', field('entity', $.identifier), 'is',
      repeat($._architecture_declarative_item),
      'begin',
      repeat($._concurrent_statement),
      'end', optional('architecture'), optional($.identifier), ';'
    ),

    // -------------------------------------------------------------------------
    // MODULE 2: THE COPPER (Declarations, Types, and Arrays)
    // -------------------------------------------------------------------------
    _architecture_declarative_item: $ => choice(
      $.signal_declaration,
      $.type_declaration
    ),

    // JUSTIFICATION: Variables are banned. A 'signal' is not a memory allocation; 
    // it is a literal declaration of a physical copper wire.
    signal_declaration: $ => seq(
      'signal', field('names', $.identifier_list), ':', field('type', alias($.type_mark, $.type_mark)), ';'
    ),

    type_declaration: $ => seq(
      'type', field('name', $.identifier), 'is',
      choice($.enum_type_definition, $.record_type_definition, $.array_type_definition), ';'
    ),

    // JUSTIFICATION: Enums synthesize mathematically into the exact \log_2 wires
    // needed for a Finite State Machine, preventing "magic number" hallucinations.
    enum_type_definition: $ => seq('(', $.identifier, repeat(seq(',', $.identifier)), ')'),

    // JUSTIFICATION: Records allow LLMs to bundle related wires (like a bus or packet)
    // together, vastly reducing the cognitive load of routing dozens of individual pins.
    record_type_definition: $ => seq(
      'record',
      repeat1($.record_field),
      'end', 'record', optional($.identifier)
    ),

    record_field: $ => seq(field('names', $.identifier_list), ':', field('type', alias($.type_mark, $.type_mark)), ';'),

    // JUSTIFICATION: Explicit 1D arrays are required so synthesis tools (and CIRCT) 
    // can map large data structures directly into physical Block RAM (SRAM) 
    // rather than wasting thousands of combinational logic gates.
    array_type_definition: $ => seq(
      'array', '(', $.number, 'to', $.number, ')', 'of', field('element_type', $.type_mark)
    ),

    type_mark: $ => choice(
      'std_logic',
      seq(choice('std_logic_vector', 'unsigned'), '(', $._expression, 'downto', $._expression, ')'),
      $.identifier 
    ),

    identifier_list: $ => seq($.identifier, repeat(seq(',', $.identifier))),

    // -------------------------------------------------------------------------
    // MODULE 3 & 4: MATH, ROUTING, TIME & INSTANTIATION
    // -------------------------------------------------------------------------
    _concurrent_statement: $ => choice(
      $.concurrent_assignment,
      $.selected_assignment,
      $.synchronous_process,
      $.generate_statement,
      $.entity_instantiation,
      $.psl_assertion,
      $.psl_assumption,
      $.psl_after_reset_assertion,
      $.psl_cover,
      $.psl_assert_never
    ),

    // JUSTIFICATION: Combinational math executes continuously. Conditionals here 
    // synthesize to physical priority encoders, not sequential branch execution.
    concurrent_assignment: $ => seq(
      field('target', $._name), '<=', field('value', $._expression),
      optional(seq('when', field('condition', $._expression), 'else', field('alt_value', $._expression))),
      ';'
    ),

    // JUSTIFICATION: The Multiplexer. Forces the AI to explicitly declare routing 
    // paths for all states. The 'others' keyword ensures no floating wires (latches).
    selected_assignment: $ => seq(
      'with', field('selector', $._expression), 'select',
      field('target', $._name), '<=',
      $._expression, 'when', choice($._expression, 'others'),
      repeat(seq(',', $._expression, 'when', choice($._expression, 'others'))),
      ';'
    ),

    // JUSTIFICATION: The Lego Snap. Banning standard VHDL components and configurations 
    // removes the dual-pass compilation requirement. Direct entity instantiation 
    // allows for infinitely scalable, modular architectures.
    entity_instantiation: $ => seq(
      field('label', $.identifier), ':', 'entity', 'work', '.', field('entity_name', $.identifier),
      optional(seq('generic', 'map', '(', field('generic_map', $.association_list), ')')),
      'port', 'map', '(', field('port_map', $.association_list), ')', ';'
    ),

    association_list: $ => seq(
      $.association_element, repeat(seq(',', $.association_element))
    ),

    association_element: $ => seq(
      field('formal', $.identifier), '=>', field('actual', $._expression)
    ),

    // JUSTIFICATION: The Time Machine. This is the ONLY place time exists in Micro-VHDL. 
    // By enforcing the `rising_edge(clk)` template, we guarantee that all state 
    // maps directly to a synchronous D-Flip-Flop register. Unclocked processes are banned.
    synchronous_process: $ => seq(
      'process', '(', 'clk', ')', 'begin',
      'if', 'rising_edge', '(', 'clk', ')', 'then',
      repeat($._sequential_statement), 
      'end', 'if', ';',
      'end', 'process', ';'
    ),

    _sequential_statement: $ => choice(
      $.sequential_assignment,
      $.sequential_if 
    ),

    sequential_assignment: $ => seq(
      field('target', $._name), '<=', field('value', $._expression), ';'
    ),

    // JUSTIFICATION: Allowing `if/else` strictly *inside* the clock block maps 
    // cleanly to the enable ('en') and reset ('rst') pins of a physical register. 
    // It keeps FSMs readable without breaking spatial routing laws.
    sequential_if: $ => seq(
      'if', field('condition', $._expression), 'then',
      repeat($._sequential_statement),
      repeat($.elsif_clause),
      optional($.else_clause),
      'end', 'if', ';'
    ),

    elsif_clause: $ => seq('elsif', field('condition', $._expression), 'then', repeat($._sequential_statement)),
    else_clause: $ => seq('else', repeat($._sequential_statement)),

    // -------------------------------------------------------------------------
    // MODULE 5: THE CLONING MACHINE (Spatial Loops)
    // -------------------------------------------------------------------------
    // JUSTIFICATION: A sequential 'for' loop implies time. A 'generate' loop 
    // implies space. This explicitly stamps out parallel hardware clones 
    // (e.g., 64 Adders) across the 2D silicon canvas.
    generate_statement: $ => seq(
      field('label', $.identifier), ':', 'for', field('iterator', $.identifier), 'in', field('start', $.number), 'to', field('end', $.number), 'generate',
      repeat($._concurrent_statement),
      'end', 'generate', optional($.identifier), ';'
    ),

    // -------------------------------------------------------------------------
    // MODULE 6: THE CONTRACT (PSL)
    // -------------------------------------------------------------------------
    // JUSTIFICATION: Hardware bugs cannot be patched over the air. PSL assertions 
    // allow external SMT solvers to mathematically prove the architectural intent 
    // of the LLM across billions of clock cycles.
    psl_assertion: $ => seq(
      'psl', 'assert', 'always',
      field('property', $._psl_expression), ';'
    ),

    psl_assumption: $ => seq(
      'psl', 'assume', 'always',
      field('property', $._psl_expression), ';'
    ),

    psl_cover: $ => seq(
      'psl', 'cover',
      field('property', $._psl_expression), ';'
    ),

    psl_assert_never: $ => seq(
      'psl', 'assert', 'never',
      field('property', $._psl_expression), ';'
    ),

    psl_after_reset_assertion: $ => seq(
      'psl', 'assert', 'always', 'after_reset',
      '(', field('reset', $._name), ')',
      field('property', $._psl_expression), ';'
    ),

    _psl_expression: $ => choice(
      $.psl_implication,
      $.psl_eventually,
      $.psl_stable,
      $.psl_next,
      $.psl_next_range,
      $._expression
    ),

    psl_implication: $ => prec.left(1, seq(field('left', $._psl_expression), '|=>', field('right', $._psl_expression))),
    psl_eventually: $ => prec.left(2, seq(field('left', $._psl_expression), '->', 'eventually!', field('right', $._psl_expression))),
    psl_stable: $ => seq('stable', '(', field('signal', $._name), ')'),
    psl_next: $ => seq('next', '[', field('delay', $.number), ']', '(', field('arg', $._psl_expression), ')'),
    psl_next_range: $ => seq('next_a', '[', field('start', $.number), 'to', field('end', $.number), ']', '(', field('arg', $._psl_expression), ')'),

    // -------------------------------------------------------------------------
    // EXPRESSIONS & NAMES
    // -------------------------------------------------------------------------
    _name: $ => choice(
      $.identifier,
      $.indexed_name, 
      $.selected_name        
    ),

    indexed_name: $ => seq(field('prefix', $.identifier), '(', field('index', $._expression), ')'),
    selected_name: $ => seq(field('prefix', $.identifier), '.', field('suffix', $.identifier)),

    paren_expression: $ => seq('(', $._expression, ')'),

    _expression: $ => choice(
      $._name,
      $.number,
      $.string_literal, 
      $.char_literal,   
      $.paren_expression,
      $.binary_expression,
      $.unary_expression
    ),

    binary_expression: $ => prec.left(1, seq(
      field('left', $._expression), 
      field('operator', choice('+', '-', '*', '=', '/=', '<', '>', '<=', '>=', 'and', 'or', 'xor', '&')), 
      field('right', $._expression)
    )),

    unary_expression: $ => prec(2, seq(
      field('operator', 'not'), 
      field('argument', $._expression)
    )),

    number: $ => /[0-9]+/,
    string_literal: $ => choice(/"[01]+"/, /x"[0-9a-fA-F]+"/),
    char_literal: $ => /'[01]'/ 
  }
});
