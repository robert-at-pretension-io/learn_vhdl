// ============================================================================
// Tree-sitter Grammar for VHDL
// ============================================================================
//
// ============================================================================
//
// WHAT IS THIS FILE?
// ------------------
// This file defines the grammar for VHDL using Tree-sitter's JavaScript DSL.
// When you run `tree-sitter generate`, it reads this file and produces a
// parser (in C) that can convert VHDL source code into a syntax tree.
//
// HOW TREE-SITTER WORKS:
// ----------------------
// 1. You define grammar rules in JavaScript
// 2. Tree-sitter generates a C parser from these rules
// 3. Your Rust code uses the parser to get a syntax tree
// 4. You walk the tree to analyze/compile the code
//
// The beauty: Tree-sitter handles error recovery automatically. If there's
// a syntax error, it inserts an ERROR node and keeps parsing. No crashes!
//
// ============================================================================
// TREE-SITTER DSL REFERENCE
// ============================================================================
//
// RULE COMBINATORS (how to build rules):
//
//   seq(a, b, c)        Match a, then b, then c in sequence
//                       Example: seq($._kw_if, $.condition, $._kw_then)
//
//   choice(a, b, c)     Match a OR b OR c (first one that works)
//                       Example: choice($._kw_and, $._kw_or, $._kw_xor)
//
//   repeat(rule)        Match rule zero or more times (like regex *)
//                       Example: repeat($.statement) -- 0+ statements
//
//   repeat1(rule)       Match rule one or more times (like regex +)
//                       Example: repeat1($.identifier) -- 1+ identifiers
//
//   optional(rule)      Match rule zero or one time (like regex ?)
//                       Example: optional($._kw_entity) -- $._kw_entity or nothing
//
//   token(rule)         Combine into single token (no whitespace allowed inside)
//                       Example: token(seq('--', /.*/)) -- comment as one token
//
//   token.immediate(r)  Like token(), AND no whitespace allowed BEFORE it
//                       Example: For things that must touch previous token
//
// REFERENCING OTHER RULES:
//
//   $.rule_name         Reference a rule defined in this grammar
//                       Example: $.identifier, $.expression
//
//   $._rule_name        "Hidden" rule (underscore prefix)
//                       These don't create named nodes in the tree.
//                       Useful for grouping without cluttering output.
//
// TERMINALS (leaf nodes that match actual text):
//
//   'keyword'           Literal string match (case-sensitive by default)
//                       Example: $._kw_entity, $._kw_begin, ';'
//
//   /regex/             Regular expression match
//                       Example: /[0-9]+/ for integers
//
//   /regex/i            Case-insensitive regex
//                       Example: /entity/i matches ENTITY, Entity, etc.
//
// NAMING & ALIASING:
//
//   field('name', rule) Give a name to part of a rule for easy access
//                       Example: field('condition', $.expression)
//                       In Rust: node.child_by_field_name("condition")
//
//   alias(rule, 'name') Rename a node in the output tree
//                       Example: alias($.identifier, 'type_name')
//
// PRECEDENCE (for resolving ambiguity):
//
//   prec(n, rule)       Set precedence level (higher n = binds tighter)
//                       Example: prec(2, seq($.expr, '*', $.expr))
//
//   prec.left(n, rule)  Left-associative: a op b op c = (a op b) op c
//                       Example: prec.left(1, seq($.expr, '+', $.expr))
//
//   prec.right(n, rule) Right-associative: a op b op c = a op (b op c)
//                       Example: prec.right(3, seq($.expr, '**', $.expr))
//
// ============================================================================
// GRAMMAR STRUCTURE
// ============================================================================
//
// module.exports = grammar({
//   name: 'language_name',
//
//   extras: $ => [...],      // Tokens that can appear anywhere (whitespace, comments)
//   conflicts: $ => [...],   // Known ambiguities that are OK
//   word: $ => $.identifier, // What counts as a "word" (helps error recovery)
//
//   rules: {
//     // First rule is the entry point (what a whole file looks like)
//     source_file: $ => ...,
//
//     // Other rules...
//   }
// });
//
// ============================================================================

module.exports = grammar({
  name: 'vhdl',

  // ===========================================================================
  // WORD - helps tree-sitter handle keyword boundaries
  // ===========================================================================
  word: $ => $.identifier,

  // ===========================================================================
  // EXTRAS
  // ===========================================================================
  // Tokens that can appear ANYWHERE between other tokens.
  // Tree-sitter automatically allows these between any two tokens.
  // Typically: whitespace and comments.
  //
  // Without this, you'd have to explicitly allow whitespace everywhere!
  // ===========================================================================
  extras: $ => [
    /\s+/,        // Whitespace: spaces, tabs, newlines
    $.comment,    // VHDL line comments can appear anywhere
    $.block_comment,  // VHDL-2008: block comments /* ... */
  ],

  // ===========================================================================
  // CONFLICTS
  // ===========================================================================
  // Sometimes a grammar is genuinely ambiguous - the same text could be
  // parsed multiple ways. If Tree-sitter detects this during generation,
  // it will error unless you list the conflicting rules here.
  //
  // Example: In VHDL, `foo(x)` could be a function call OR an array index.
  // You'd add: conflicts: $ => [[$.function_call, $.indexed_name]]
  //
  // Package vs package body can conflict since both start with $._kw_package
  // ===========================================================================
  conflicts: $ => [
    [$.package_declaration, $._package_declarative_item],
    [$._block_declarative_item, $._concurrent_statement],
    [$._conditional_signal_assignment, $._force_release_assignment, $._name],
    [$._block_configuration, $._component_configuration],  // for identifier...
    [$.subprogram_declaration, $.subprogram_body],  // both start with procedure/function
    [$.indexed_name, $._name_or_attribute],  // identifier followed by ( - is it index or part of attr?
    [$.indexed_name, $._expression_term],  // selected_name followed by ( in expression
    [$.indexed_name, $._type_name],  // identifier( in alias: indexed_name vs type constraint
    [$.concurrent_procedure_call, $.component_instantiation],  // label: identifier; ambiguity
    [$._index_primary, $._procedure_arg_value],  // identifier in index vs procedure arg
    [$._index_primary, $._literal],  // character literal in index
    [$._index_primary, $.character_literal],  // explicit character literal in index
    [$._index_primary, $._name_or_attribute],  // identifier in index
    [$._index_primary, $._expression_term]  // number/identifier in index
  ],

  // ===========================================================================
  // RULES
  // ===========================================================================
  rules: {
    // -------------------------------------------------------------------------
    // ENTRY POINT: source_file
    // -------------------------------------------------------------------------
    // The first rule is always the entry point - what a complete file looks like.
    //
    // A VHDL file contains zero or more "design units". For now, we only
    // recognize comments. Everything else will become ERROR nodes.
    //
    // repeat($._definition) means: match _definition zero or more times.
    // An empty file is valid (repeat allows zero matches).
    // -------------------------------------------------------------------------
    source_file: $ => repeat($._definition),

    // Case-insensitive VHDL keywords
    _kw_library: _ => /[lL][iI][bB][rR][aA][rR][yY]/,
    _kw_use: _ => /[uU][sS][eE]/,
    _kw_package: _ => /[pP][aA][cC][kK][aA][gG][eE]/,
    _kw_body: _ => /[bB][oO][dD][yY]/,
    _kw_is: _ => /[iI][sS]/,
    _kw_end: _ => /[eE][nN][dD]/,
    _kw_units: _ => /[uU][nN][iI][tT][sS]/,
    _kw_generic: _ => /[gG][eE][nN][eE][rR][iI][cC]/,
    _kw_type: _ => /[tT][yY][pP][eE]/,
    _kw_function: _ => /[fF][uU][nN][cC][tT][iI][oO][nN]/,
    _kw_procedure: _ => /[pP][rR][oO][cC][eE][dD][uU][rR][eE]/,
    _kw_return: _ => /[rR][eE][tT][uU][rR][nN]/,
    _kw_pure: _ => /[pP][uU][rR][eE]/,
    _kw_impure: _ => /[iI][mM][pP][uU][rR][eE]/,
    _kw_signal: _ => /[sS][iI][gG][nN][aA][lL]/,
    _kw_variable: _ => /[vV][aA][rR][iI][aA][bB][lL][eE]/,
    _kw_constant: _ => /[cC][oO][nN][sS][tT][aA][nN][tT]/,
    _kw_file: _ => /[fF][iI][lL][eE]/,
    _kw_in: _ => /[iI][nN]/,
    _kw_out: _ => /[oO][uU][tT]/,
    _kw_inout: _ => /[iI][nN][oO][uU][tT]/,
    _kw_buffer: _ => /[bB][uU][fF][fF][eE][rR]/,
    _kw_linkage: _ => /[lL][iI][nN][kK][aA][gG][eE]/,
    _kw_subtype: _ => /[sS][uU][bB][tT][yY][pP][eE]/,
    _kw_alias: _ => /[aA][lL][iI][aA][sS]/,
    _kw_component: _ => /[cC][oO][mM][pP][oO][nN][eE][nN][tT]/,
    _kw_entity: _ => /[eE][nN][tT][iI][tT][yY]/,
    _kw_architecture: _ => /[aA][rR][cC][hH][iI][tT][eE][cC][tT][uU][rR][eE]/,
    _kw_configuration: _ => /[cC][oO][nN][fF][iI][gG][uU][rR][aA][tT][iI][oO][nN]/,
    _kw_open: _ => /[oO][pP][eE][nN]/,
    _kw_context: _ => /[cC][oO][nN][tT][eE][xX][tT]/,
    _kw_port: _ => /[pP][oO][rR][tT]/,
    _kw_map: _ => /[mM][aA][pP]/,
    _kw_process: _ => /[pP][rR][oO][cC][eE][sS][sS]/,
    _kw_begin: _ => /[bB][eE][gG][iI][nN]/,
    _kw_wait: _ => /[wW][aA][iI][tT]/,
    _kw_until: _ => /[uU][nN][tT][iI][lL]/,
    _kw_for: _ => /[fF][oO][rR]/,
    _kw_on: _ => /[oO][nN]/,
    _kw_if: _ => /[iI][fF]/,
    _kw_then: _ => /[tT][hH][eE][nN]/,
    _kw_elsif: _ => /[eE][lL][sS][iI][fF]/,
    _kw_else: _ => /[eE][lL][sS][eE]/,
    _kw_case: _ => /[cC][aA][sS][eE]/,
    _kw_when: _ => /[wW][hH][eE][nN]/,
    _kw_others: _ => /[oO][tT][hH][eE][rR][sS]/,
    _kw_loop: _ => /[lL][oO][oO][pP]/,
    _kw_while: _ => /[wW][hH][iI][lL][eE]/,
    _kw_exit: _ => /[eE][xX][iI][tT]/,
    _kw_next: _ => /[nN][eE][xX][tT]/,
    _kw_assert: _ => /[aA][sS][sS][eE][rR][tT]/,
    _kw_report: _ => /[rR][eE][pP][oO][rR][tT]/,
    _kw_severity: _ => /[sS][eE][vV][eE][rR][iI][tT][yY]/,
    _kw_and: _ => /[aA][nN][dD]/,
    _kw_or: _ => /[oO][rR]/,
    _kw_xor: _ => /[xX][oO][rR]/,
    _kw_nand: _ => /[nN][aA][nN][dD]/,
    _kw_nor: _ => /[nN][oO][rR]/,
    _kw_xnor: _ => /[xX][nN][oO][rR]/,
    _kw_not: _ => /[nN][oO][tT]/,
    _kw_mod: _ => /[mM][oO][dD]/,
    _kw_rem: _ => /[rR][eE][mM]/,
    _kw_abs: _ => /[aA][bB][sS]/,
    _kw_sll: _ => /[sS][lL][lL]/,
    _kw_srl: _ => /[sS][rR][lL]/,
    _kw_sla: _ => /[sS][lL][aA]/,
    _kw_sra: _ => /[sS][rR][aA]/,
    _kw_rol: _ => /[rR][oO][lL]/,
    _kw_ror: _ => /[rR][oO][rR]/,
    _kw_force: _ => /[fF][oO][rR][cC][eE]/,
    _kw_release: _ => /[rR][eE][lL][eE][aA][sS][eE]/,
    _kw_guarded: _ => /[gG][uU][aA][rR][dD][eE][dD]/,
    _kw_select: _ => /[sS][eE][lL][eE][cC][tT]/,
    _kw_with: _ => /[wW][iI][tT][hH]/,
    _kw_record: _ => /[rR][eE][cC][oO][rR][dD]/,
    _kw_array: _ => /[aA][rR][rR][aA][yY]/,
    _kw_access: _ => /[aA][cC][cC][eE][sS][sS]/,
    _kw_protected: _ => /[pP][rR][oO][tT][eE][cC][tT][eE][dD]/,
    _kw_shared: _ => /[sS][hH][aA][rR][eE][dD]/,
    _kw_attribute: _ => /[aA][tT][tT][rR][iI][bB][uU][tT][eE]/,
    _kw_of: _ => /[oO][fF]/,
    _kw_range: _ => /[rR][aA][nN][gG][eE]/,
    _kw_to: _ => /[tT][oO]/,
    _kw_downto: _ => /[dD][oO][wW][nN][tT][oO]/,
    _kw_after: _ => /[aA][fF][tT][eE][rR]/,
    _kw_generate: _ => /[gG][eE][nN][eE][rR][aA][tT][eE]/,
    _kw_block: _ => /[bB][lL][oO][cC][kK]/,
    _kw_all: _ => /[aA][lL][lL]/,
    _kw_open: _ => /[oO][pP][eE][nN]/,
    _kw_new: _ => /[nN][eE][wW]/,
    // PSL keywords (Property Specification Language)
    _kw_default: _ => /[dD][eE][fF][aA][uU][lL][tT]/,
    _kw_clock: _ => /[cC][lL][oO][cC][kK]/,
    _kw_always: _ => /[aA][lL][wW][aA][yY][sS]/,
    _kw_disconnect: _ => /[dD][iI][sS][cC][oO][nN][nN][eE][cC][tT]/,
    _kw_group: _ => /[gG][rR][oO][uU][pP]/,

    // -------------------------------------------------------------------------
    // HIDDEN RULE: _definition
    // -------------------------------------------------------------------------
    // Rules starting with underscore (_) are "hidden" - they don't create
    // named nodes in the syntax tree. Only their children appear.
    //
    // This is useful for grouping alternatives. Instead of seeing:
    //   (source_file (_definition (comment)))
    // You see:
    //   (source_file (comment))
    //
    // choice() means: match any ONE of these alternatives.
    // The parser tries them in order and uses the first that matches.
    // -------------------------------------------------------------------------
    _definition: $ => choice(
      $.comment,
      $.library_clause,
      $.use_clause,
      $.context_reference,  // VHDL-2008: context lib.ctx;
      prec.dynamic(3, $.package_instantiation),  // VHDL-2008: package X is new Y;
      prec.dynamic(2, $.package_body),
      prec.dynamic(1, $.package_declaration),
      $.function_declaration,
      $.procedure_declaration,
      $.entity_declaration,
      $.architecture_body,
      $.configuration_declaration,
      $.context_declaration
    ),

    // -------------------------------------------------------------------------
    // TERMINAL RULE: comment
    // -------------------------------------------------------------------------
    // VHDL comments start with -- and continue to end of line.
    //
    // token() is crucial here! It combines the sequence into a SINGLE token.
    // Without token(), Tree-sitter would allow whitespace between '--' and
    // the comment text, which is wrong.
    //
    // seq('--', /.*/) means: match '--' followed by any characters until newline.
    // The /.*/ regex matches everything except newline (. doesn't match \n).
    // -------------------------------------------------------------------------
    comment: $ => token(seq('--', /.*/)),
    // VHDL-2008: Block comments /* ... */ (can span multiple lines)
    block_comment: $ => token(seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/')),
    identifier: $ => /[_a-zA-Z][a-zA-Z0-9_]*/,
    selector_clause: $=> prec.left(3, repeat1(seq('.', $.identifier))),
    library_clause: $=> seq($._kw_library, $.identifier, repeat(seq(',', $.identifier)), ';'),
    use_clause: $ => seq(
      $._kw_use,
      $.identifier,
      optional($.selector_clause),
      repeat(seq(',', $.identifier, optional($.selector_clause))),  // Multiple items: use a.b, c.d;
      ';'
    ),

    // -------------------------------------------------------------------------
    // package_declaration
    // -------------------------------------------------------------------------
    // package identifier is
    //   [generic (...);]  -- VHDL-2008
    //   declarations...
    // end [package] [identifier];
    package_declaration: $ => seq(
      $._kw_package,
      field('name', $.identifier),
      $._kw_is,
      optional($._package_generic_clause),  // VHDL-2008 package generics
      repeat($._package_declarative_item),
      $._kw_end,
      optional($._kw_package),
      optional($.identifier),
      ';'
    ),

    // VHDL-2008: Package instantiation
    // package name is new lib.pkg [generic map (...)];
    package_instantiation: $ => seq(
      $._kw_package,
      field('name', $.identifier),
      $._kw_is,
      $._kw_new,
      $._selected_name,  // The uninstantiated package (lib.pkg)
      optional(seq($._kw_generic, $._kw_map, '(', optional($._association_list), ')')),
      ';'
    ),

    // VHDL-2008: Package generic clause
    _package_generic_clause: $ => seq(
      $._kw_generic,
      '(',
      $.generic_item,
      repeat(seq(';', $.generic_item)),
      ')',
      ';'
    ),

    // Generic item can be type, constant, function, or package (VHDL-2008)
    generic_item: $ => choice(
      seq($._kw_type, $.identifier),  // Generic type parameter
      seq($._kw_function, $.identifier, $._parameter_list, $._kw_return, $.identifier),  // Generic function
      seq($._kw_package, $.identifier, $._kw_is, $._kw_new, $._selected_name,  // Generic package instantiation
          optional(seq($._kw_generic, $._kw_map, '(', optional($._association_list), ')'))),
      $.parameter  // Generic constant (like normal parameter)
    ),

    _package_declarative_item: $ => choice(
      $.comment,
      $.use_clause,  // VHDL-2008: use clause in packages (for generic package parameters)
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.component_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.signal_declaration,
      $.file_declaration,  // File declarations in packages
      $.group_template_declaration,  // Group template declarations
      $.group_declaration,  // Group declarations
      $._package_generic_clause  // Also allowed as declarative item for flexibility
    ),

    // -------------------------------------------------------------------------
    // constant_declaration
    // -------------------------------------------------------------------------
    // constant identifier : type [:= expression];
    // Without := it's a "deferred constant" (value in package body)
    constant_declaration: $ => seq(
      $._kw_constant,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple names: a, b : integer
      ':',
      field('type', $._type_mark),
      optional(seq(':=', field('value', $._constant_value))),
      ';'
    ),

    // Constant values can be various literals and expressions
    _constant_value: $ => choice(
      $.allocator_expression,  // new Type'(value)
      seq(optional(choice('+', '-')), $.number, optional($.identifier), repeat(seq(/[+\-*/]+/, optional(choice('+', '-')), choice($.number, $._parenthesized_expression), optional($.identifier)))),  // Arithmetic: 3.0 + 2.0, -10 ns, 2 ** 5, -9.0 * (-3.0)
      seq(choice('+', '-'), $.identifier),  // Unary: - p2
      seq($._kw_not, $.identifier),  // Logical not: not C1
      seq($.attribute_name, repeat(seq(/[+\-*/]+/, choice($.number, $.identifier)))),  // attr expr: c'LENGTH - 1
      seq($._parenthesized_expression, repeat1(seq(/[+\-*/]+/, $._parenthesized_expression))),  // (expr) / (expr), (1-4)/(1-4)
      seq($.identifier, repeat1(seq(/[+\-*/&]+/, $.identifier))),  // identifier op identifier: a / b, x * y, a & b
      seq($.identifier, '(', optional(seq($._procedure_argument, repeat(seq(',', $._procedure_argument)))), ')'),  // Function call: F(x, y)
      $.identifier,
      $._string_literal,
      seq($.identifier, $._string_literal),  // e.g., x"AA" based literal
      $._parenthesized_expression  // Aggregate with nested parens supported
    ),

    // Allocator expression: new type or new type'(value)
    allocator_expression: $ => seq(
      $._kw_new,
      choice(
        seq($.identifier, "'", '(', optional(seq($._aggregate_element, repeat(seq(',', $._aggregate_element)))), ')'),  // new Type'(values)
        $.identifier  // new Type (for unconstrained)
      )
    ),

    // Simplified expression - placeholder for proper expression parsing
    // Match word characters, spaces, and some punctuation, but stop at delimiters
    // Note: includes '(' but not ')' to avoid eating closing parens of param lists
    // NOT using token() to allow word boundaries to work properly
    // _simple_expression - fallback for value/expression contexts
    // Does NOT include '(' or ')'
    _simple_expression: _ => prec(-1, /[a-zA-Z0-9_., \t"'+\-*/<>=]+/),

    // _type_expression - for type definition contexts
    // Matches: identifier optionally followed by constraint
    // e.g., "integer", "std_logic_vector(7 downto 0)", "file of string"
    _type_expression: $ => choice(
      seq($._kw_access, $.identifier),        // Access type: access integer
      seq($._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression),  // Integer/float range: range 1 to 32
      seq(
        $.identifier,
        optional(choice(
          seq('(', /[^)]+/, ')'),         // Constraint in parens
          seq($._kw_range, /[^;]+/),          // Range constraint
          seq($._kw_of, $.identifier)         // For "file of X" types
        ))
      )
    ),

    // Type mark - a type name, possibly with constraints like std_logic_vector(7 downto 0)
    // Type name can be simple (integer) or selected (work.pkg.Type)
    _type_mark: $ => prec(-1, seq(
      $._type_name,
      optional(choice(
        $._index_constraint,  // Parenthesized constraint: (7 downto 0), (st_ind1, st_ind2)
        seq($._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression)  // Range constraint
      ))
    )),

    // Index constraint in parentheses - handles multi-dimensional and comments
    _index_constraint: $ => seq(
      '(',
      $._index_spec,
      repeat(seq(',', $._index_spec)),
      ')'
    ),

    // Single index specification
    _index_spec: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // 0 to 7, 7 downto 0
      $.identifier  // Just a type/subtype name
    ),

    // Type name - simple identifier or selected name (work.pkg.Type)
    _type_name: $ => seq(
      $.identifier,
      repeat(seq('.', $.identifier))
    ),

    // -------------------------------------------------------------------------
    // type_declaration
    // -------------------------------------------------------------------------
    // type identifier is type_definition;
    // or: type identifier; (incomplete/forward declaration)
    type_declaration: $ => choice(
      // Incomplete type declaration (forward declaration): type name;
      seq($._kw_type, field('name', $.identifier), ';'),
      // Full type declaration
      seq(
        $._kw_type,
        field('name', $.identifier),
        $._kw_is,
        field('definition', choice(
          prec(10, $.record_type_definition),
          prec(10, $.enumeration_type_definition),
          prec(10, $.array_type_definition),
          prec(10, $.physical_type_definition),
          prec(10, $.protected_type_declaration),
          prec(10, $.protected_type_body),
          prec(-10, $._type_expression)    // Fallback for simple/constrained types
        )),
        optional(';')
      )
    ),

    // Physical type definition with units (e.g., time, resistance)
    // type T is range X to Y units ... end units;
    physical_type_definition: $ => seq(
      $._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression,
      $._kw_units,
      $.identifier, ';',  // Base unit
      repeat(seq($.identifier, '=', $._expression, ';')),  // Secondary units
      $._kw_end, $._kw_units, optional($.identifier)
    ),

    // VHDL-2008: Protected type declaration (interface)
    // type name is protected
    //   procedure/function declarations...
    // end protected [name];
    protected_type_declaration: $ => seq(
      $._kw_protected,
      repeat($._protected_type_declarative_item),
      $._kw_end,
      $._kw_protected,
      optional($.identifier)
    ),

    // VHDL-2008: Protected type body (implementation)
    // type name is protected body
    //   variable declarations, procedure/function bodies...
    // end protected body [name];
    protected_type_body: $ => seq(
      $._kw_protected,
      $._kw_body,
      repeat($._protected_type_body_item),
      $._kw_end,
      $._kw_protected,
      $._kw_body,
      optional($.identifier)
    ),

    _protected_type_declarative_item: $ => choice(
      $.comment,
      $.subprogram_declaration
    ),

    _protected_type_body_item: $ => choice(
      $.comment,
      $.variable_declaration,
      $.subprogram_body
    ),

    // -------------------------------------------------------------------------
    // enumeration_type_definition
    // -------------------------------------------------------------------------
    // (ID1, ID2, ID3, ...) or ('Z', '0', '1', 'X') for character enums
    enumeration_type_definition: $ => seq(
      '(',
      $._enumeration_literal,
      repeat(seq(',', $._enumeration_literal)),
      ')'
    ),

    _enumeration_literal: $ => choice(
      $.identifier,
      /'[^']'/,  // Character literal: 'a', '0', etc.
      /'''/      // Apostrophe character: '''
    ),

    // -------------------------------------------------------------------------
    // array_type_definition
    // -------------------------------------------------------------------------
    // array (range1, range2, ...) of element_type
    // array (index_type range <>) of element_type  -- unconstrained
    array_type_definition: $ => seq(
      $._kw_array,
      '(',
      $._array_index_constraint,
      repeat(seq(',', $._array_index_constraint)),
      ')',
      $._kw_of,
      $._type_mark  // element type
    ),

    // Index constraint for array type: "type range <>" or "1 to 10" or "integer range 1 to 8"
    _array_index_constraint: $ => choice(
      seq($.identifier, $._kw_range, '<>'),  // Unconstrained: integer range <>
      seq($.identifier, $._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression),  // Type with range: integer range 1 to 8
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Constrained: 1 to 10
      $.identifier  // Just a type: integer
    ),

    // -------------------------------------------------------------------------
    // record_type_definition
    // -------------------------------------------------------------------------
    // record
    //   field : type;
    //   ...
    // end record [identifier];
    record_type_definition: $ => seq(
      $._kw_record,
      repeat($.element_declaration),
      $._kw_end,
      $._kw_record,
      optional($.identifier)
    ),

    element_declaration: $ => seq(
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple names: x, y : integer
      ':',
      field('type', $._type_mark),
      ';'
    ),

    // -------------------------------------------------------------------------
    // subtype_declaration
    // -------------------------------------------------------------------------
    // subtype identifier is [resolution_function] subtype_indication;
    subtype_declaration: $ => seq(
      $._kw_subtype,
      field('name', $.identifier),
      $._kw_is,
      optional(field('resolution', $.identifier)),  // Optional resolution function
      field('indication', $._type_mark),
      ';'
    ),

    // -------------------------------------------------------------------------
    // alias_declaration
    // -------------------------------------------------------------------------
    // alias identifier [: type] is name;
    alias_declaration: $ => seq(
      $._kw_alias,
      field('name', $.identifier),
      optional(seq(':', $._type_mark)),  // Optional type constraint
      $._kw_is,
      field('aliased_name', choice(
        $.indexed_name,   // C2(2), arr(7 downto 4)
        $.selected_name,  // pkg.sig
        $._type_mark      // type(7 downto 4) or just identifier
      )),
      ';'
    ),

    // -------------------------------------------------------------------------
    // subprogram_declaration
    // -------------------------------------------------------------------------
    // function identifier (...) return type;
    // procedure identifier (...);
    subprogram_declaration: $ => choice(
      $.function_declaration,
      $.procedure_declaration
    ),

    // Function: unified rule that handles both declaration and body
    function_declaration: $ => seq(
      optional(choice($._kw_pure, $._kw_impure)),  // VHDL-2008
      $._kw_function,
      field('name', choice($.identifier, $._operator_symbol)),
      optional($._parameter_list),
      $._kw_return,
      field('return_type', $.identifier),
      choice(
        ';',  // Declaration only
        seq(  // Body
          $._kw_is,
          repeat($._subprogram_declarative_item),
          $._kw_begin,
          repeat($._sequential_statement),
          $._kw_end,
          optional($._kw_function),
          optional(choice($.identifier, $._operator_symbol)),  // Can be operator symbol too
          ';'
        )
      )
    ),

    // Alias for compatibility
    function_body: $ => $.function_declaration,

    // Procedure: unified rule that handles both declaration and body
    procedure_declaration: $ => seq(
      $._kw_procedure,
      field('name', $.identifier),
      optional($._parameter_list),
      choice(
        ';',  // Declaration only
        seq(  // Body
          $._kw_is,
          repeat($._subprogram_declarative_item),
          $._kw_begin,
          repeat($._sequential_statement),
          $._kw_end,
          optional($._kw_procedure),
          optional($.identifier),
          ';'
        )
      )
    ),

    // Alias for compatibility
    procedure_body: $ => $.procedure_declaration,

    _operator_symbol: _ => /"[^"]+"/,  // String literals for operator overloading

    _parameter_list: $ => seq(
      '(',
      optional(seq(
        $.parameter,
        repeat(seq(';', $.parameter))
      )),
      ')'
    ),

    // parameter: [signal|variable|constant] name[, name...] : [in|out|inout] type [:= default]
    parameter: $ => seq(
      optional(field('class', $.parameter_class)),
      $.identifier,
      repeat(seq(',', $.identifier)),
      ':',
      optional(field('direction', $.port_direction)),
      $._parameter_type,
      optional(seq(':=', field('default', $.default_value)))  // default value
    ),

    // Port/parameter direction - visible for extraction
    port_direction: _ => choice(
      /[iI][nN]/,
      /[oO][uU][tT]/,
      /[iI][nN][oO][uU][tT]/,
      /[bB][uU][fF][fF][eE][rR]/,
      /[lL][iI][nN][kK][aA][gG][eE]/
    ),

    // Parameter class - visible for extraction
    parameter_class: _ => choice(
      /[sS][iI][gG][nN][aA][lL]/,
      /[vV][aA][rR][iI][aA][bB][lL][eE]/,
      /[cC][oO][nN][sS][tT][aA][nN][tT]/,
      /[fF][iI][lL][eE]/
    ),

    // Default values can be identifiers, numbers, literals, or expressions
    // Visible for extraction
    default_value: $ => choice(
      seq(optional(choice('+', '-')), $.number, optional($.identifier)),  // Numbers: 1, -1, 1.0, 1 fs, -1.0 ns
      seq($.identifier, $._string_literal),  // Based string like B"10010110"
      $._string_literal,  // String literal: "1011"
      $.identifier,  // Simple identifier
      seq('(', /[^)]+/, ')'),  // Expression in parens
      prec(-1, seq($.identifier, repeat1(seq(/[+\-*/<>=]+/, choice($.identifier, $.number)))))  // Expression: a**b+c
    ),

    number: _ => /[0-9][0-9_]*(\.[0-9][0-9_]*)?/,  // Integer or floating point (underscores allowed)

    // String literals including VHDL-specific formats
    _string_literal: _ => choice(
      /"[^"]*"/,              // Regular string "text"
      /x"[0-9a-fA-F_]*"/,     // Hex string x"1A"
      /o"[0-7_]*"/,           // Octal string o"17"
      /b"[01_]*"/,            // Binary string b"1010"
      /'[^']'/,               // Character literal: 'a', '0', etc.
      /'''/                   // Apostrophe character: '''
    ),

    // Type in parameter context - allow identifiers with optional constraints
    // Examples: "integer", "std_logic", "std_logic_vector(7 downto 0)"
    // VHDL-2008: Also supports selected names like mux_g.mux_data_array(0 to 7)
    _parameter_type: $ => seq(
      $.identifier,
      repeat(seq('.', $.identifier)),  // Optional selected name: pkg.type
      optional(seq('(', $._simple_expression, ')'))
    ),

    // -------------------------------------------------------------------------
    // component_declaration
    // -------------------------------------------------------------------------
    // component identifier
    //   [generic (...);]
    //   [port (...);]
    // end component [identifier];
    component_declaration: $ => seq(
      $._kw_component,
      field('name', $.identifier),
      optional($._kw_is),  // VHDL-93+: optional $._kw_is
      optional(seq($._kw_generic, $._parameter_list, ';')),
      optional(seq($._kw_port, $._parameter_list, ';')),
      $._kw_end,
      $._kw_component,
      optional($.identifier),
      ';'
    ),

    // -------------------------------------------------------------------------
    // package_body
    // -------------------------------------------------------------------------
    // package body identifier is
    //   declarations and subprogram bodies...
    // end [package body] [identifier];
    package_body: $ => seq(
      $._kw_package,
      $._kw_body,
      field('name', $.identifier),
      $._kw_is,
      repeat($._package_body_declarative_item),
      $._kw_end,
      optional(seq($._kw_package, $._kw_body)),
      optional($.identifier),
      ';'
    ),

    _package_body_declarative_item: $ => choice(
      $.comment,
      $.subprogram_body,
      $.type_declaration,  // For protected type bodies
      $.constant_declaration
    ),

    // -------------------------------------------------------------------------
    // entity_declaration
    // -------------------------------------------------------------------------
    // entity identifier is
    //   [generic (...);]
    //   [port (...);]
    //   [declarative items]
    // end [entity] [identifier];
    entity_declaration: $ => seq(
      $._kw_entity,
      field('name', $.identifier),
      $._kw_is,
      optional(seq($._kw_generic, $._entity_generic_list, ';')),  // VHDL-2008: supports type generics
      optional(seq($._kw_port, $._parameter_list, ';')),
      repeat($._entity_declarative_item),
      optional(seq(
        $._kw_begin,
        repeat($._entity_statement)
      )),
      $._kw_end,
      optional($._kw_entity),
      optional($.identifier),
      ';'
    ),

    // VHDL-2008: Entity generic list can include type, function, and constant generics
    _entity_generic_list: $ => seq(
      '(',
      optional(seq(
        $.generic_item,
        repeat(seq(';', $.generic_item))
      )),
      ')'
    ),

    _entity_statement: $ => choice(
      $.comment,
      $.assert_statement,
      $.process_statement,
      $.subprogram_declaration,  // Procedure/function in entity
      $.concurrent_procedure_call  // Passive procedure call
    ),

    assert_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label for concurrent assertions
      $._kw_assert,
      $._expression,
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $.identifier)),
      ';'
    ),

    // Report expression - strings and concatenation, but stops at $._kw_severity keyword
    _report_expression: $ => repeat1(choice(
      $._string_literal,
      $._name_or_attribute,
      $._function_call_in_expr,  // Function calls like to_hstring(x)
      '&'  // String concatenation operator
    )),

    // Function call in expression context: func(arg) or func(a, b)
    _function_call_in_expr: $ => seq(
      $.identifier,
      '(',
      optional(seq($._expression, repeat(seq(',', $._expression)))),
      ')'
    ),

    _entity_declarative_item: $ => choice(
      $.comment,
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.signal_declaration
    ),

    // -------------------------------------------------------------------------
    // architecture_body
    // -------------------------------------------------------------------------
    // architecture identifier of entity_name is
    //   [declarative items]
    // begin
    //   [statements]
    // end [architecture] [identifier];
    architecture_body: $ => seq(
      $._kw_architecture,
      field('name', $.identifier),
      $._kw_of,
      field('entity', $.identifier),
      $._kw_is,
      repeat($._block_declarative_item),
      $._kw_begin,
      repeat($._concurrent_statement),
      $._kw_end,
      optional($._kw_architecture),
      optional($.identifier),
      ';'
    ),

    // -------------------------------------------------------------------------
    // configuration_declaration
    // -------------------------------------------------------------------------
    // configuration name of entity is
    //   for architecture_name
    //     for component_spec : component_name
    //       use entity lib.entity(arch);
    //     end for;
    //   end for;
    // end [configuration] [name];
    configuration_declaration: $ => seq(
      $._kw_configuration,
      field('name', $.identifier),
      $._kw_of,
      field('entity', $.identifier),
      $._kw_is,
      repeat($._configuration_item),
      $._kw_end,
      optional($._kw_configuration),
      optional($.identifier),
      ';'
    ),

    _configuration_item: $ => choice(
      $.comment,
      $._block_configuration
    ),

    _block_configuration: $ => seq(
      $._kw_for,
      $.identifier,  // architecture or generate label
      repeat($._configuration_item_or_component),
      $._kw_end,
      $._kw_for,
      ';'
    ),

    _configuration_item_or_component: $ => choice(
      $.comment,
      $._component_configuration,
      $._block_configuration
    ),

    _component_configuration: $ => seq(
      $._kw_for,
      choice(
        seq($.identifier, ':', $.identifier),  // instance : component
        seq($._kw_all, ':', $.identifier),         // all : component
        seq($._kw_others, ':', $.identifier),      // others : component
        $.identifier                           // Just generate label
      ),
      optional(choice(
        // Full binding indication with use entity
        seq(
          $._kw_use,
          $._kw_entity,
          $.identifier,
          '.',
          $.identifier,
          optional(seq('(', $.identifier, ')')),  // Optional architecture
          optional(seq($._kw_generic, $._kw_map, '(', /[^)]+/, ')')),  // Generic map
          optional(seq($._kw_port, $._kw_map, '(', /[^)]+/, ')')),     // Port map
          ';'  // Semicolon after binding indication
        ),
        // Incremental binding indication (generic map and/or port map without use)
        seq(
          optional(seq($._kw_generic, $._kw_map, '(', /[^)]+/, ')')),  // Generic map
          optional(seq($._kw_port, $._kw_map, '(', /[^)]+/, ')')),     // Port map
          ';'  // Semicolon after incremental binding
        )
      )),
      $._kw_end,
      $._kw_for,
      ';'
    ),

    // -------------------------------------------------------------------------
    // context_declaration (VHDL-2008)
    // -------------------------------------------------------------------------
    // context name is
    //   library ...; use ...;
    // end [context] [name];
    context_declaration: $ => seq(
      $._kw_context,
      field('name', $.identifier),
      $._kw_is,
      repeat(choice($.library_clause, $.use_clause, $.context_reference, $.comment)),
      $._kw_end,
      optional($._kw_context),
      optional($.identifier),
      ';'
    ),

    // VHDL-2008: context reference - importing a context
    // context lib.ctx;
    context_reference: $ => seq(
      $._kw_context,
      $._selected_name,  // lib.context_name
      ';'
    ),

    // Note: $.comment removed since comments are in extras and handled automatically
    _block_declarative_item: $ => choice(
      $.use_clause,  // Use clauses allowed in architecture declarative region
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.signal_declaration,
      $.component_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.shared_variable_declaration,
      $.file_declaration,
      $.configuration_specification,
      $.package_instantiation,  // VHDL-2008: local package instantiation
      $.psl_default_clock,  // PSL: default clock declaration
      $.disconnect_specification,  // Disconnection specification for guarded signals
      $.group_declaration,  // Group declaration
      $.group_template_declaration  // Group template declaration
    ),

    // Disconnection specification: disconnect signal : type after time;
    disconnect_specification: $ => seq(
      $._kw_disconnect,
      choice($.identifier, $._kw_all, $._kw_others),
      ':',
      $._type_mark,
      $._kw_after,
      $._expression,
      ';'
    ),

    // Group template declaration: group identifier is (entity_class {, entity_class});
    group_template_declaration: $ => seq(
      $._kw_group,
      $.identifier,
      $._kw_is,
      '(',
      $.identifier,
      repeat(seq(',', $.identifier)),
      optional(seq('<', '>')),  // For variadic groups: (signal, signal <>)
      ')',
      ';'
    ),

    // Group declaration: group identifier : template_name (items);
    // Items can be identifiers or selected names (PROJECT.GLOBALS.CK)
    group_declaration: $ => seq(
      $._kw_group,
      $.identifier,
      ':',
      $.identifier,
      '(',
      $._group_constituent,
      repeat(seq(',', $._group_constituent)),
      ')',
      ';'
    ),

    _group_constituent: $ => choice(
      $.selected_name,  // PROJECT.GLOBALS.CK
      $.identifier
    ),

    // PSL: default clock is expression;
    psl_default_clock: $ => seq(
      $._kw_default,
      $._kw_clock,
      $._kw_is,
      $._expression,
      ';'
    ),

    // Configuration specification: for label : component use entity lib.entity(arch);
    configuration_specification: $ => seq(
      $._kw_for,
      choice($.identifier, $._kw_all, $._kw_others),  // Component label(s)
      ':',
      $.identifier,  // Component name
      $._kw_use,
      choice(
        seq($._kw_entity, $._entity_aspect),  // use entity work.foo(bar)
        seq($._kw_configuration, $._selected_name),  // use configuration work.cfg
        $._kw_open  // use open
      ),
      optional(seq($._kw_generic, $._kw_map, '(', /[^)]*/, ')')),
      optional(seq($._kw_port, $._kw_map, '(', /[^)]*/, ')')),
      ';'
    ),

    // Entity aspect: library.entity(architecture)
    _entity_aspect: $ => seq(
      $._selected_name,
      optional(seq('(', $.identifier, ')'))  // Optional architecture name
    ),

    // VHDL-2008: Shared variable (requires protected type)
    shared_variable_declaration: $ => seq(
      $._kw_shared,
      $._kw_variable,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple variable names
      ':',
      field('type', $._type_mark),
      ';'
    ),

    // File declaration
    // File declaration: file name : type [open mode] [is "filename"];
    file_declaration: $ => seq(
      $._kw_file,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple file names
      ':',
      field('type', $.identifier),
      optional(seq($._kw_open, $.identifier)),  // Optional open mode
      optional(seq($._kw_is, $._expression)),   // Optional filename
      ';'
    ),

    // attribute identifier : type;
    attribute_declaration: $ => seq(
      $._kw_attribute,
      field('name', $.identifier),
      ':',
      field('type', $.identifier),
      ';'
    ),

    // attribute identifier of entity_list : entity_class is expression;
    attribute_specification: $ => seq(
      $._kw_attribute,
      field('name', $.identifier),
      $._kw_of,
      field('target', $.identifier),
      ':',
      field('class', $.identifier),
      $._kw_is,
      field('value', $._expression),  // Can be any expression: string, number, identifier, aggregate
      ';'
    ),

    // Signal declaration: signal name : [resolution_func] type [signal_kind] [:= value];
    // Resolution function comes before type name if present
    signal_declaration: $ => seq(
      $._kw_signal,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),
      ':',
      $._signal_type_indication,
      optional(seq(':=', $._constant_value)),
      ';'
    ),

    // Signal type indication - handles resolution function and signal kind
    _signal_type_indication: $ => seq(
      repeat($.identifier),  // 0+ resolution functions/type parts before final type
      $._type_mark,
      optional(choice($._kw_bus, $._kw_register))  // Optional signal kind
    ),

    _kw_bus: _ => /[bB][uU][sS]/,
    _kw_register: _ => /[rR][eE][gG][iI][sS][tT][eE][rR]/,

    // Concurrent statements (architecture body)
    _concurrent_statement: $ => choice(
      $.comment,
      $.generate_statement,
      $.block_statement,
      $.signal_assignment,
      $.process_statement,
      $.component_instantiation,
      $.assert_statement,  // Concurrent assertion
      $.concurrent_procedure_call  // Concurrent procedure call: proc_name;
    ),

    // Concurrent procedure call: procedure_name [(args)];
    concurrent_procedure_call: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      choice(
        seq($.identifier, ';'),
        seq($.identifier, '(', optional(seq($._procedure_argument, repeat(seq(',', $._procedure_argument)))), ')', ';'),
        seq($.selected_name, ';'),
        seq($.selected_name, '(', optional(seq($._procedure_argument, repeat(seq(',', $._procedure_argument)))), ')', ';')
      )
    ),

    // Generate statements: for generate, if generate, case generate (VHDL-2008)
    generate_statement: $ => seq(
      field('label', $.identifier),
      ':',
      choice(
        $._for_generate,
        $._if_generate,
        $._case_generate  // VHDL-2008
      )
    ),

    _for_generate: $ => seq(
      $._kw_for, $.identifier, $._kw_in, $._range_or_expression, $._kw_generate,
      choice(
        // VHDL-2008: declarations + begin + statements
        seq(
          repeat($._block_declarative_item),
          $._kw_begin,
          repeat($._concurrent_statement)
        ),
        // Pre-2008 or simple: just statements (no declarations)
        repeat($._concurrent_statement)
      ),
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    _if_generate: $ => seq(
      $._kw_if, $._expression, $._kw_generate,
      choice(
        // VHDL-2008: declarations + begin + statements
        seq(
          repeat($._block_declarative_item),
          $._kw_begin,
          repeat($._concurrent_statement)
        ),
        // Pre-2008 or simple: just statements
        repeat($._concurrent_statement)
      ),
      optional($._generate_else),  // VHDL-2008
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    _generate_else: $ => seq(
      $._kw_else, $._kw_generate,
      repeat($._concurrent_statement)
    ),

    _case_generate: $ => seq(
      $._kw_case, $._expression, $._kw_generate,
      repeat($._case_generate_alternative),
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    _case_generate_alternative: $ => seq(
      $._kw_when, $._expression, '=>',
      repeat($._concurrent_statement)  // Label handled by process_statement
    ),

    // Range expression: 0 to 10, vec'range, etc.
    _range_or_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Explicit range
      $._expression  // Attribute like vec'range
    ),

    // Block statement with optional generic/port interface
    block_statement: $ => seq(
      field('label', $.identifier),
      ':',
      $._kw_block,
      optional(seq('(', $._expression, ')')),  // Guard condition
      optional($._kw_is),
      optional(seq($._kw_generic, $._parameter_list, ';')),  // Generic interface
      optional(seq($._kw_generic, $._kw_map, '(', $._association_list, ')', ';')),  // Generic map
      optional(seq($._kw_port, $._parameter_list, ';')),  // Port interface
      optional(seq($._kw_port, $._kw_map, '(', $._association_list, ')', ';')),  // Port map
      repeat($._block_declarative_item),
      $._kw_begin,
      repeat($._concurrent_statement),
      $._kw_end,
      $._kw_block,
      optional($.identifier),
      ';'
    ),

    // Signal assignment - simple, conditional, or selected (with optional label)
    signal_assignment: $ => choice(
      $._labeled_signal_assignment,  // A: (I(1), I(2)) <= value;
      $._simple_signal_assignment,
      $._conditional_signal_assignment,
      $._selected_signal_assignment,
      $._force_release_assignment
    ),

    // Labeled signal assignment (concurrent): label: target <= waveform;
    _labeled_signal_assignment: $ => seq(
      $.identifier,
      ':',
      $._assignment_target,
      '<=',
      optional($._kw_guarded),
      optional($._kw_transport),
      $._waveform_element,
      repeat(seq(',', $._waveform_element)),
      ';'
    ),

    // Simple signal assignment with waveform: signal <= [transport] value [after time] {, value [after time]};
    _simple_signal_assignment: $ => seq(
      $._name,  // Can be indexed: data(i) <= '0'
      '<=',
      optional($._kw_guarded),  // For guarded signal assignments in blocks
      optional($._kw_transport),  // Optional transport delay mechanism
      $._waveform_element,
      repeat(seq(',', $._waveform_element)),
      ';'
    ),

    _kw_transport: _ => /[tT][rR][aA][nN][sS][pP][oO][rR][tT]/,

    // target <= expr [after time] when cond else expr [after time] when cond else expr [after time];
    _conditional_signal_assignment: $ => seq(
      $.identifier,
      '<=',
      optional($._kw_transport),
      $._waveform_element,
      $._kw_when,
      $._expression,
      repeat(seq($._kw_else, $._waveform_element, optional(seq($._kw_when, $._expression)))),
      ';'
    ),

    // with selector select target <= value [after time] when choice, value when others;
    _selected_signal_assignment: $ => seq(
      $._kw_with,
      $._expression,
      $._kw_select,
      $.identifier,
      '<=',
      optional($._kw_transport),
      $._waveform_element,
      $._kw_when,
      $._choice_expression,  // Can be range like 1 to 19
      repeat(seq(',', $._waveform_element, $._kw_when, $._choice_expression)),
      ';'
    ),

    // Choice in case/select - can be expression, range, or others
    _choice_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Range: 1 to 19
      $._kw_others,  // others
      $._expression  // Simple expression
    ),

    // VHDL-2008 force/release
    _force_release_assignment: $ => seq(
      $.identifier,
      '<=',
      choice($._kw_force, $._kw_release),
      optional($._expression),
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    process_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      $._kw_process,
      optional(seq('(', $.sensitivity_list, ')')),  // Sensitivity list
      optional($._kw_is),
      repeat($._process_declarative_item),
      $._kw_begin,
      repeat($._sequential_statement),
      $._kw_end,
      $._kw_process,
      optional($.identifier),
      ';'
    ),

    // Sensitivity list: all (VHDL-2008) or list of signal names
    // Visible for extraction
    sensitivity_list: $ => choice(
      $._kw_all,  // VHDL-2008: sensitive to all signals read in process
      seq($._signal_name, repeat(seq(',', $._signal_name)))  // Signal names including selected/indexed
    ),

    // Process declarative items - what can appear before $._kw_begin in a process
    _process_declarative_item: $ => choice(
      $.comment,
      $.variable_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.constant_declaration,
      $.file_declaration,  // file x : file_type [open mode is "name"];
      $.alias_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.subprogram_declaration,
      $.subprogram_body,
      $.use_clause
    ),

    component_instantiation: $ => seq(
      field('label', $.identifier),
      ':',
      choice(
        // Direct entity instantiation: entity lib.entity(arch)
        seq(
          $._kw_entity,
          $.identifier,  // library
          '.',
          $.identifier,  // entity
          optional(seq('(', $.identifier, ')'))  // Optional architecture
        ),
        // Component instantiation: component_name
        field('component', $.identifier)
      ),
      optional(seq($._kw_generic, $._kw_map, '(', $._association_list, ')')),
      optional(seq($._kw_port, $._kw_map, '(', $._association_list, ')')),
      ';'
    ),

    _association_list: $ => seq(
      $._association_element,
      repeat(seq(',', $._association_element))
    ),

    _association_element: $ => choice(
      seq($.identifier, '=>', $._expression),  // Named association
      $._expression  // Positional association
    ),


    // -------------------------------------------------------------------------
    // subprogram_body (function/procedure implementations)
    // -------------------------------------------------------------------------
    // Subprogram body - now just uses the unified declaration rules
    subprogram_body: $ => choice(
      $.function_declaration,
      $.procedure_declaration
    ),

    // Declarative items inside subprograms (functions/procedures)
    _subprogram_declarative_item: $ => choice(
      $.comment,
      $.variable_declaration,
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.alias_declaration,
      $.use_clause,
      $.subprogram_body  // Nested functions/procedures
    ),

    variable_declaration: $ => seq(
      $._kw_variable,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple names: seed1, seed2
      ':',
      field('type', $._type_mark),
      optional(seq(':=', field('value', $._constant_value))),
      ';'
    ),

    // Statements (simplified - just match anything until semicolon for now)
    // Order matters for ambiguous cases - signal assignment before procedure call
    _sequential_statement: $ => choice(
      $.comment,
      prec(3, $.return_statement),
      prec(3, $.if_statement),
      prec(3, $.loop_statement),
      prec(3, $.wait_statement),
      prec(3, $.case_statement),
      prec(3, $.exit_statement),
      prec(3, $.next_statement),
      prec(3, $.assert_statement),              // Sequential assertion
      prec(3, $.report_statement),              // Report without assertion
      prec(3, $.sequential_signal_assignment),  // Signal assignment - try before proc call
      prec(2, $.assignment_statement),
      prec(1, $.procedure_call_statement)       // Procedure call - lowest for identifier(...)
    ),

    // Report statement (standalone, without assertion)
    report_statement: $ => seq(
      $._kw_report,
      $._report_expression,  // Message string, can include concatenation
      optional(seq($._kw_severity, $.identifier)),
      ';'
    ),

    // Sequential signal assignment (inside process/function)
    // Use prec.dynamic for runtime conflict resolution with procedure_call_statement
    // Supports waveform: signal <= value [after time] {, value [after time]};
    sequential_signal_assignment: $ => prec.dynamic(10, seq(
      $._assignment_target,  // Includes identifier, selected_name, indexed_name
      '<=',
      $._waveform_element,
      repeat(seq(',', $._waveform_element)),
      ';'
    )),

    // Waveform element: value [after time_expression]
    _waveform_element: $ => seq(
      $._expression,
      optional(seq($._kw_after, $._expression))
    ),

    exit_statement: $ => seq(
      $._kw_exit,
      optional($.identifier),  // Optional loop label
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    next_statement: $ => seq(
      $._kw_next,
      optional($.identifier),  // Optional loop label
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    // Case statement (regular and VHDL-2008 matching case?)
    case_statement: $ => seq(
      choice($._kw_case, seq($._kw_case, '?')),  // case or case?
      $._expression,
      $._kw_is,
      repeat1($._case_alternative),
      $._kw_end,
      choice($._kw_case, seq($._kw_case, '?')),  // end case or end case?
      ';'
    ),

    _case_alternative: $ => seq(
      $._kw_when, $._expression, '=>',
      repeat($._sequential_statement)
    ),

    // Wait statement: wait [on signal_list] [until condition] [for time_expression];
    wait_statement: $ => seq(
      $._kw_wait,
      optional(seq($._kw_on, $._signal_name, repeat(seq(',', $._signal_name)))),  // sensitivity clause
      optional(seq($._kw_until, $._expression)),  // condition clause
      optional(seq($._kw_for, $._expression)),    // timeout clause
      ';'
    ),

    // Signal name - used in sensitivity lists (wait on X, process(X))
    _signal_name: $ => choice(
      $.indexed_name,   // A(1), arr(idx)
      $.selected_name,  // record.field
      $.identifier      // simple signal
    ),

    return_statement: $ => seq(
      $._kw_return,
      optional($._expression),
      ';'
    ),

    assignment_statement: $ => seq(
      $._assignment_target,  // Can be identifier, selected name, or indexed/sliced name
      ':=',
      $._expression,
      optional(seq(  // Conditional assignment (VHDL-2008): x := expr when cond else expr
        $._kw_when, $._expression,
        repeat(seq($._kw_else, $._expression, optional(seq($._kw_when, $._expression))))
      )),
      ';'
    ),

    // Assignment target - left side of := or <= assignment
    // Includes selected names, indexed names, and aggregates
    _assignment_target: $ => choice(
      $.indexed_name,   // arr(i), foo.bar(i)
      $.selected_name,  // record.field, V1.S11.S12
      $.identifier,     // Simple variable
      $._aggregate_target  // (S, T) for aggregate assignment
    ),

    // Aggregate target for signal/variable assignment: (name, name, ...) or (a=>name, b=>name)
    _aggregate_target: $ => seq(
      '(',
      $._aggregate_target_element,
      repeat(seq(',', $._aggregate_target_element)),
      ')'
    ),

    // Element in aggregate target - can be positional or named
    _aggregate_target_element: $ => choice(
      seq($.identifier, '=>', $._signal_name),  // Named: a => rec.field
      $._signal_name  // Positional
    ),

    // Name can be simple identifier, indexed/sliced, or attribute
    // Supports chaining: arr(1)(2), arr(1 to 8)(7), V(2 to 4)'LOW
    // Note: selected_name (pkg.name) is handled separately in procedure_call_statement
    _name: $ => choice(
      $.indexed_name,   // Indexed/sliced name: foo(i), foo(1 to 8)
      $.attribute_name, // Attribute: foo'attr
      $.external_name,  // VHDL-2008: << signal .path : type >>
      $.identifier      // Simple name
    ),

    // Simple name (cannot be recursive) - used as base for chained names
    _simple_name: $ => choice(
      $.identifier,
      $.external_name
    ),

    // Selected name: prefix.suffix
    // Prefix can be identifier, selected_name, or indexed_name (for v1(1).x)
    selected_name: $ => prec.left(6, seq(
      choice($.identifier, $.selected_name, $.indexed_name),
      '.',
      $.identifier
    )),

    // Indexed/sliced name for array access: foo(i), foo(1 to 8), foo(1)(2)
    // Prefix can be identifier, selected_name, or another indexed_name for chaining
    indexed_name: $ => prec.dynamic(5, seq(
      choice(
        $.identifier,
        $.selected_name,
        $.indexed_name    // Allow chaining: foo(1)(2)
      ),
      '(',
      $._index_expression,
      ')'
    )),

    // Attribute name: prefix'attribute or prefix'attribute(arg)
    attribute_name: $ => prec.left(4, seq(
      choice($.identifier, $.indexed_name, $.selected_name),
      "'",
      $.identifier,
      optional(seq('(', $._attribute_argument, ')'))  // Optional argument: 'Left(1), 'Image(x)
    )),

    // Simpler argument for attribute - can be identifier, indexed_name, or expression
    _attribute_argument: $ => choice(
      $.indexed_name,  // p32(2) inside character'succ(p32(2))
      seq($.identifier, repeat(seq(/[+\-*/<>=]+/, choice($.identifier, $.number)))),  // expr: x + 1
      $.number
    ),

    // Index expression - supports identifiers, numbers, ranges, operators
    _index_expression: $ => seq(
      $._index_element,
      repeat(seq(choice(',', $._kw_to, $._kw_downto), $._index_element))
    ),

    // Each index element is a single value with optional operators
    _index_element: $ => seq(
      $._index_primary,
      repeat(seq(/[+\-*/<>=]+/, $._index_primary))  // Optional operators between primaries
    ),

    // Primary value in an index - attribute must have higher precedence
    _index_primary: $ => choice(
      prec(3, $.attribute_name),  // type'Left(1), arr'Length - highest priority
      $.identifier,
      $.number,
      /'[^']'/,  // Character literal: 'a', '0', etc.
      /'''/      // Apostrophe character: '''
    ),

    // Simplified if statement
    if_statement: $ => seq(
      $._kw_if,
      $._expression,  // condition
      $._kw_then,
      repeat($._sequential_statement),
      repeat(seq(  // elsif clauses
        $._kw_elsif,
        $._expression,
        $._kw_then,
        repeat($._sequential_statement)
      )),
      optional(seq(
        $._kw_else,
        repeat($._sequential_statement)
      )),
      $._kw_end,
      $._kw_if,
      ';'
    ),

    // Simplified loop (while, for, or infinite) with optional label
    // label : [while cond | for i in range] loop ... end loop [label];
    loop_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      optional(choice(
        seq($._kw_while, $._expression),  // while loop
        seq($._kw_for, $.identifier, $._kw_in, $._range_or_expression)  // for loop
      )),
      $._kw_loop,
      repeat($._sequential_statement),
      $._kw_end,
      $._kw_loop,
      optional($.identifier),  // Optional label at end
      ';'
    ),

    // Expression - structured to avoid consuming keywords
    // Uses repeat of terms to stop at keywords properly
    _expression: $ => prec(-1, repeat1($._expression_term)),

    // Single term in an expression - identifier, number, operator, or grouped expression
    _expression_term: $ => choice(
      prec(10, $._literal),  // Must be early and high precedence to catch char literals
      $.allocator_expression,  // new Type or new Type'(value)
      $.qualified_expression,  // type'(expr) - must be before _name_or_attribute
      prec(5, $.attribute_name),  // arr'succ(x) - higher precedence than bare identifier
      $.indexed_name,    // arr(i), arr(1 to 8) - indexed/sliced names in expressions
      $.selected_name,   // pkg.const - selected names in expressions
      $._name_or_attribute,  // Identifier optionally followed by attribute
      $.external_name,  // VHDL-2008: << signal .path : type >>
      $.number,
      $.relational_operator,  // =, /=, <, >, <=, >= (visible for semantic analysis)
      $.logical_operator,     // and, or, xor, etc. (visible for semantic analysis)
      $.arithmetic_operator,  // +, -, *, /, etc. (visible for semantic analysis)
      $.shift_operator,       // sll, srl, sla, sra, rol, ror
      $._kw_not,              // Unary not
      $._kw_abs,              // Unary abs
      $._kw_always,  // PSL: always temporal operator
      $.psl_next_expression,  // PSL: next[n](expr)
      $._parenthesized_expression  // Grouped or aggregate
    ),

    // Visible operator nodes for semantic analysis
    relational_operator: _ => choice('=', '/=', '<', '>', '<=', '>=', '?=', '?/=', '?<', '?>', '?<=', '?>='),
    logical_operator: _ => choice(/[aA][nN][dD]/, /[oO][rR]/, /[xX][oO][rR]/, /[nN][aA][nN][dD]/, /[nN][oO][rR]/, /[xX][nN][oO][rR]/),
    arithmetic_operator: _ => choice('+', '-', '*', '/', '**', '&', /[mM][oO][dD]/, /[rR][eE][mM]/),
    shift_operator: _ => choice(/[sS][lL][lL]/, /[sS][rR][lL]/, /[sS][lL][aA]/, /[sS][rR][aA]/, /[rR][oO][lL]/, /[rR][oO][rR]/),

    // PSL: next[count](expression) - temporal operator
    psl_next_expression: $ => seq(
      $._kw_next,
      '[',
      $.number,  // Count
      ']',
      '(',
      $._expression,
      ')'
    ),

    // Qualified expression: type'(expression)
    // e.g., string'("hello"), integer'(x + 1), character'('a')
    // Make identifier'( a single token so lexer won't see '( as character literal
    // Note: This doesn't allow whitespace between ' and ( (e.g., type' (x))
    qualified_expression: $ => seq(
      token(prec(2, seq(/[_a-zA-Z][a-zA-Z0-9_]*/, "'("))),  // identifier'( as single token
      optional(seq($._aggregate_element, repeat(seq(',', $._aggregate_element)))),
      ')'
    ),

    // VHDL-2008: External name (hierarchical reference)
    // << signal .testbench.dut.sig : std_logic >>
    external_name: $ => seq(
      '<<',
      choice($._kw_signal, $._kw_variable, $._kw_constant),
      $._external_pathname,
      ':',
      $._type_mark,
      '>>'
    ),

    // Pathname for external names: .absolute.path or ^.relative.path
    _external_pathname: $ => repeat1(choice(
      '.',
      '^',
      $.identifier
    )),

    // Parenthesized expression - can be grouping or aggregate
    // Handles: (expr), (a, b, c), (x => 1, y => 2), (others => '0')
    _parenthesized_expression: $ => seq(
      '(',
      optional(seq(
        $._aggregate_element,
        repeat(seq(',', $._aggregate_element))
      )),
      ')'
    ),

    // Element in an aggregate or parenthesized expression
    _aggregate_element: $ => choice(
      seq($._kw_others, '=>', repeat1($._expression_term)),  // others => value
      seq($.identifier, '=>', repeat1($._expression_term)),  // named: x => 1
      seq($.number, '=>', repeat1($._expression_term)),  // positional with index
      prec(5, $.character_literal),  // Single character literal with high precedence
      repeat1($._expression_term)  // Just a value (includes _literal)
    ),

    // Character literal as explicit rule
    character_literal: _ => /'[^']'/,

    // Literals (character, string)
    // Character literal: 'x' (single char) or ''' (apostrophe)
    _literal: $ => prec(10, choice(
      $.character_literal,    // Single character: 'a', '0', ' ', etc.
      /'''/,       // Apostrophe character: '''
      /"[^"]*"/    // String literal: "text", "" (can be empty)
    )),

    // Name with optional attribute: identifier['attribute]
    // Note: attr with argument like time'image(now) handled by attribute_name rule
    _name_or_attribute: $ => choice(
      token(seq(/[_a-zA-Z]+/, /[a-zA-Z0-9_]*/, "'", /[a-zA-Z]+/)),  // identifier'attribute (without arg)
      $.identifier  // Just identifier
    ),

    // Procedure call statement - higher precedence than _simple_statement
    // Using direct patterns to avoid complex nesting issues
    // Procedure call statement
    // Shares prefix with signal assignment via $._name and indexed_name
    procedure_call_statement: $ => prec.dynamic(-1, choice(
      seq($.identifier, ';'),                    // No args: bar;
      seq($.identifier, '(', ')', ';'),          // Empty parens: bar();
      seq($._procedure_call_with_args, ';'),     // With args: foo(a, b, c);
      seq($.selected_name, ';'),                 // Selected: obj.method, pkg.func.call;
      seq($.selected_name, '(', ')', ';'),       // Selected with parens: obj.method();
      seq($.selected_name, '(', $._procedure_argument, repeat(seq(',', $._procedure_argument)), ')', ';'),
      seq($.indexed_name, ';')                   // Indexed: pkg.func(arg);
    )),

    // Selected name: a.b or a.b.c (for method calls, package references)
    _selected_name: $ => prec.left(5, seq(
      $.identifier,
      repeat1(seq('.', $.identifier))
    )),

    // Procedure call with arguments - separate from indexed_name
    _procedure_call_with_args: $ => seq(
      $.identifier,
      '(',
      $._procedure_argument,
      repeat(seq(',', $._procedure_argument)),
      ')'
    ),

    // Procedure argument - allows function calls, named associations, aggregates
    _procedure_argument: $ => choice(
      seq($.indexed_name, '=>', $._procedure_arg_value),   // Named: P(1) => V
      seq($.selected_name, '=>', $._procedure_arg_value),  // Named: P.a => V.b
      seq($.identifier, '=>', $._procedure_arg_value),     // Named: x => value
      $._procedure_arg_value  // Positional
    ),

    // Value in a procedure argument (without the named association part)
    _procedure_arg_value: $ => choice(
      $.qualified_expression,  // string'("hello")
      $._function_call_in_expr,  // ROUND(var2), func(a, b)
      seq($._literal, repeat1(seq('&', $._literal))),  // String concat: "1" & "2"
      seq($._literal, /[+\-*/<>=&|?]+/, $._literal),  // Binary op: '1' ?= '1', a < b
      seq('??', $.qualified_expression),  // VHDL-2008 condition operator: ?? std_logic'('1')
      seq('??', $._literal),  // VHDL-2008 condition operator: ?? 'H'
      seq($.number, $.identifier),  // Physical literal: 3 ns, 10 ms
      seq($.identifier, /[+\-*/<>=&|?]+/, choice($._literal, $.identifier, $.number)),  // Binary: x + 1
      seq($.identifier, $._kw_mod, $.number),  // Specific: x mod 16
      seq($._parenthesized_expression, $._kw_mod, $.number),  // (a + 1) mod 16
      seq($._parenthesized_expression, /[+\-*/<>=&|?]+/, choice($.number, $.identifier)),  // (x) + 1
      $.indexed_name,  // arr(i), slice(0 to 3)
      $.selected_name, // record.field
      $.identifier,
      $.number,
      $._literal,
      $._parenthesized_expression  // Aggregates: (1, 2), (x => 1)
    ),

    // Catch-all for other statements (very low priority)
    _simple_statement: $ => prec(-10, seq(
      $.identifier,
      optional(/[^;]*/),
      ';'
    )),

    // =========================================================================
    // Add more VHDL rules here as you learn
    // =========================================================================
  }
});
