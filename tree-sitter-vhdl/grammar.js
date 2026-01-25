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
// ERROR NODES ARE POISONOUS - FIX THEM HERE!
// ============================================================================
//
// When Tree-sitter can't parse a construct, it creates an ERROR node.
// ERROR nodes are TOXIC to the entire downstream pipeline:
//
// 1. Raw text inside ERROR nodes gets misidentified
//    - Keywords like "downto", "when", "others" appear as signal names
//    - Operators get confused with identifiers
//
// 2. The extractor sees fake "signals" that don't exist
//    - Signals list includes VHDL keywords
//    - Read/write analysis is corrupted
//
// 3. OPA rules fire false positives
//    - "Signal 'downto' is unused" - obviously wrong!
//    - Users lose trust in the tool
//
// 4. Adding skip lists downstream is a LOSING BATTLE
//    - Every new ERROR node needs new workarounds
//    - Workarounds multiply and become unmaintainable
//
// THE SOLUTION: Fix the grammar so ERROR nodes don't occur.
//
// When you see false positives:
// 1. Check: npx tree-sitter parse file.vhd 2>&1 | grep ERROR
// 2. Find what construct the grammar can't handle
// 3. FIX IT HERE IN grammar.js
// 4. Regenerate: npx tree-sitter generate
// 5. ERROR disappears, false positives vanish
//
// Invest time here, save pain everywhere else!
//
// See: AGENTS.md "The Grammar Improvement Cycle"
// ============================================================================

module.exports = grammar({
  name: 'vhdl',

  // ===========================================================================
  // EXTERNAL SCANNER
  // ===========================================================================
  // Some tokens can't be handled cleanly by grammar rules because the lexer
  // tokenizes greedily. For example, X"DEADBEEF" would be tokenized as:
  //   identifier("X") + string_literal("DEADBEEF")
  // instead of a single bit_string_literal.
  //
  // External scanners (written in C) run BEFORE the normal lexer, giving us
  // first crack at recognizing these tokens. See src/scanner.c for details.
  // ===========================================================================
  externals: $ => [
    $.bit_string_literal,  // X"...", B"...", O"..." - handled by scanner.c
  ],

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
    $.protect_directive,  // VHDL-2008: `protect ... (encrypted wrappers)
  ],

  // ===========================================================================
  // CONFLICTS
  // ===========================================================================
  // Sometimes a grammar is genuinely ambiguous - the same text could be
  // parsed multiple ways. If Tree-sitter detects this during generation,
  // it will error unless you list the conflicting rules here.
  //
  // Example: In VHDL, `foo(x)` could be a function call OR an array index.
  // Unified names reduce many of these conflicts.
  //
  // Package vs package body can conflict since both start with $._kw_package
  // ===========================================================================
  conflicts: $ => [
    [$.package_declaration, $._package_declarative_item],
    [$._type_name, $._simple_name],
    [$._type_name, $._function_call_in_expr, $._simple_name],
    [$._primary_expression, $._expression_term],
    [$._index_spec, $._primary_expression],
    [$.association_element, $._index_expression_item],
    [$.association_list, $._index_expression_unified],
    [$._index_spec, $._simple_name],
    [$._array_index_constraint, $._simple_name],
    [$._type_name, $._array_index_constraint, $._simple_name],
    [$._index_spec, $._primary_expression, $._expression_term],
    [$._function_call_in_expr, $._simple_name],
    [$._index_spec, $._index_expression_item],
    [$._constant_value, $._simple_name],
    [$._entity_aspect, $._simple_name],
    [$.alias_declaration, $._simple_name],
    [$._type_mark, $._name, $._expression_term],
    [$._type_mark, $._primary_expression, $._expression_term],
    [$._type_mark, $.report_statement, $._name],
    [$.physical_literal, $._expression_term],
    [$.physical_literal, $._primary_expression, $._expression_term],
    [$.physical_literal, $.arithmetic_operator],
    [$._protected_type_private_item, $.private_variable_declaration],
    [$._generic_type_indication, $._type_name, $._simple_name],
    [$.generic_item, $.parameter],
    [$.generic_procedure_declaration, $.parameter],
    [$.generic_function_declaration, $.parameter],
    [$._generate_body],
    [$._generate_body, $._generate_block],
    [$.psl_next_expression, $._aggregate_element],
    [$.psl_parenthesized_expression, $._expression_term],
    [$.psl_next_event_expression, $.association_element, $._index_expression_item],
    [$._waveform_element, $._waveform_element_no_when],
    [$.component_instantiation, $._simple_name],
    [$._conditional_signal_assignment, $._simple_signal_assignment],
    [$._waveform_element_no_when, $._expression],
    [$.subprogram_declaration, $.subprogram_body],
    [$.concurrent_procedure_call, $.component_instantiation],
    [$._force_release_assignment, $._assignment_target],
    [$._block_configuration, $._component_configuration],
    [$._configuration_item_or_component, $._component_configuration],
    [$.record_type_definition],
    [$.protected_type_declaration],
    [$.protected_type_body],
    [$.physical_type_definition],
    [$.psl_sequence],
    [$._psl_sequence_item],
    [$._psl_assert_expression, $._expression_term],
    [$._block_declarative_item, $._concurrent_statement],
    [$._simple_name, $._aggregate_element],
    [$._primary_expression, $._expression_term, $._aggregate_element],
    [$._aggregate_choice_expression, $._aggregate_element],
    [$.generic_type_declaration],  // VHDL-2019 generic type vs type_expression
    [$._generic_type_indication, $._type_expression],  // For access/file type generics
    [$._generic_array_index, $._array_index_constraint],  // For array type generics
    [$._generic_array_index, $._type_name, $._array_index_constraint, $._simple_name],
    [$._generic_array_index, $.anonymous_type_indication],  // For inline type is (<>)
    [$.allocator_expression],  // For new Type vs new Type(constraint)
    [$._generate_label, $.assert_statement, $.psl_cover_statement, $.psl_assume_statement, $.psl_restrict_statement, $.concurrent_procedure_call, $.block_statement, $._simple_signal_assignment, $._conditional_signal_assignment, $._selected_signal_assignment, $._force_release_assignment, $.process_statement, $.component_instantiation],
    [$._end_entity_clause],
    [$._end_architecture_clause],
    [$.type_declaration],
    [$.allocator_expression, $._type_mark, $._name],
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
    _kw_parameter: _ => /[pP][aA][rR][aA][mM][eE][tT][eE][rR]/,
    _kw_pure: _ => /[pP][uU][rR][eE]/,
    _kw_impure: _ => /[iI][mM][pP][uU][rR][eE]/,
    _kw_private: _ => /[pP][rR][iI][vV][aA][tT][eE]/,
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
    _kw_assume: _ => /[aA][sS][sS][uU][mM][eE]/,
    _kw_cover: _ => /[cC][oO][vV][eE][rR]/,
    _kw_property: _ => /[pP][rR][oO][pP][eE][rR][tT][yY]/,
    _kw_sequence: _ => /[sS][eE][qQ][uU][eE][nN][cC][eE]/,
    _kw_restrict: _ => /[rR][eE][sS][tT][rR][iI][cC][tT]/,
    _kw_eventually: _ => /[eE][vV][eE][nN][tT][uU][aA][lL][lL][yY]/,
    _kw_never: _ => /[nN][eE][vV][eE][rR]/,
    _kw_report: _ => /[rR][eE][pP][oO][rR][tT]/,
    _kw_severity: _ => /[sS][eE][vV][eE][rR][iI][tT][yY]/,
    _kw_postponed: _ => /[pP][oO][sS][tT][pP][oO][nN][eE][dD]/,
    _kw_abort: _ => /[aA][bB][oO][rR][tT]/,
    _kw_sync_abort: _ => /[sS][yY][nN][cC]_[aA][bB][oO][rR][tT]/,
    _kw_async_abort: _ => /[aA][sS][yY][nN][cC]_[aA][bB][oO][rR][tT]/,
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
    _kw_view: _ => /[vV][iI][eE][wW]/,

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
    comment: $ => token(seq('--', /[^\r\n]*/)),
    // VHDL-2008: Block comments /* ... */ (can span multiple lines)
    block_comment: $ => token(seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/')),
    // VHDL-2008: `protect directives (treat as skippable preproc lines)
    protect_directive: $ => token(seq('`protect', /[^\r\n]*/)),
    identifier: $ => token(choice(
      /[_a-zA-Z][a-zA-Z0-9_]*/,
      /\\([^\\]|\\\\)+\\/
    )),
    selector_clause: $=> prec.left(3, repeat1(seq('.', choice($.identifier, $._operator_symbol, $._kw_all)))),
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
      $._name,  // The uninstantiated package (lib.pkg)
      optional(seq($._kw_generic, $._kw_map, '(', optional($.association_list), ')')),
      ';'
    ),

    // VHDL-2008: Package generic clause
    _package_generic_clause: $ => seq(
      $._kw_generic,
      '(',
      $.generic_item,
      repeat(seq(';', $.generic_item)),
      optional(';'),
      ')',
      ';'
    ),

    // Generic item can be type, constant, function, or package (VHDL-2008)
    generic_item: $ => choice(
      $.generic_type_declaration,
      $.generic_function_declaration,
      $.generic_procedure_declaration,
      seq($._kw_package, $.identifier, $._kw_is, $._kw_new, $._name,  // Generic package instantiation
          optional(seq($._kw_generic, $._kw_map, '(', optional($.association_list), ')'))),
      $.parameter  // Generic constant (like normal parameter)
    ),

    // Generic subprograms (VHDL-2008) with optional default body: is <>
    generic_function_declaration: $ => seq(
      optional(choice($._kw_pure, $._kw_impure)),
      $._kw_function,
      field('name', choice($.identifier, $._operator_symbol)),
      optional(choice(
        seq($._kw_parameter, $._parameter_list),
        $._parameter_list
      )),
      $._kw_return,
      $._return_type,
      optional(seq($._kw_is, choice('<>', $._name)))
    ),

    generic_procedure_declaration: $ => seq(
      optional(choice($._kw_pure, $._kw_impure)),
      $._kw_procedure,
      field('name', choice($.identifier, $._operator_symbol)),
      optional(choice(
        seq($._kw_parameter, $._parameter_list),
        $._parameter_list
      )),
      optional(seq($._kw_is, choice('<>', $._name)))
    ),

    // VHDL-2019: generic anonymous type declarations
    generic_type_declaration: $ => seq(
      $._kw_type,
      $.identifier,
      optional(seq(
        $._kw_is,
        choice(
          $._kw_private,
          seq('(', '<>', ')'),  // Discrete type: type T is (<>)
          '<>',
          seq($._kw_access, $._generic_type_indication),  // Access type generic
          seq($._kw_file, $._kw_of, $._generic_type_indication),  // File type generic
          seq($._kw_array, '(', $._generic_array_index, ')', $._kw_of, $._generic_type_indication),  // Array type generic
          $.enumeration_type_definition,
          $.array_type_definition,
          $.record_type_definition,
          prec(-10, $._type_expression)
        )
      ))
    ),

    // Type indication for generic type declarations
    _generic_type_indication: $ => choice(
      seq($._kw_type, $._kw_is, $._kw_private),  // Anonymous private type
      seq($._kw_type, $._kw_is, '(', '<>', ')'),  // Anonymous discrete type
      $.identifier  // Named type
    ),

    // Generic array index for VHDL-2019 array type generics
    _generic_array_index: $ => choice(
      seq($.identifier, $._kw_range, '<>'),  // index_type range <>
      seq($._kw_type, $._kw_is, '(', '<>', ')')  // type is (<>)
    ),

    _package_declarative_item: $ => choice(
      $.use_clause,  // VHDL-2008: use clause in packages (for generic package parameters)
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.subprogram_instantiation,
      $.component_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.signal_declaration,
      $.file_declaration,  // File declarations in packages
      $.shared_variable_declaration,
      $.group_template_declaration,  // Group template declarations
      $.group_declaration,  // Group declarations
      $.view_declaration,  // VHDL-2019: mode view declaration
      $.package_declaration,
      $.package_body,
      $.package_instantiation,  // Nested package instantiation
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
    _constant_value: $ => $._expression,

    // Constant argument - allow full expressions for nested function calls
    _constant_arg: $ => $._expression,

    // Allocator expression: new type, new type(constraint), or new type'(value)
    // Handles multi-dimensional via _name's recursive indexing: new bv (0 to 7)(15 downto 0)
    allocator_expression: $ => seq(
      $._kw_new,
      choice(
        $.qualified_expression,
        $._name
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
      seq($._kw_access, $._type_mark),        // Access type: access integer, access string(1 to 80)
      seq($._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression),  // Integer/float range: range 1 to 32
      seq($._kw_range, $._expression),  // Attribute range: range t'range or t'reverse_range
      seq(
        $.identifier,
        optional(choice(
          $._index_constraint,             // Constraint in parens
          seq($._kw_range, $._range_or_expression),  // Range constraint
          seq($._kw_of, $.identifier)         // For "file of X" types
        ))
      )
    ),

    // Type mark - a type name with optional constraints, or attribute-based subtype
    _type_mark: $ => prec(-1, seq(
      choice($._type_name, $._name),
      optional(choice(
        $._index_constraint,  // Parenthesized constraint: (7 downto 0), (st_ind1, st_ind2)
        seq($._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression),  // Range constraint
        seq($._kw_range, $._expression)  // Attribute range: range vec'range
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
      $._index_subtype_indication,  // integer range 1 to 3, enum range M1 to M5
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // 0 to 7, 7 downto 0
      $._name,  // type'Range, signal'Range
      $.identifier  // Just a type/subtype name
    ),

    // Subtype indication used in index/range contexts (e.g., integer range 1 to 3)
    _index_subtype_indication: $ => seq(
      choice($._type_name, $._name),
      $._kw_range,
      $._expression,
      optional(seq(choice($._kw_to, $._kw_downto), $._expression))
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
          prec(10, seq($._kw_new, $._type_mark, optional($.generic_map_aspect))),
          prec(-10, $._type_expression)    // Fallback for simple/constrained types
        )),
        optional(';'),
        optional(seq($._kw_end, optional($.identifier), optional(';')))
      )
    ),

    // Physical type definition with units (e.g., time, resistance)
    // type T is range X to Y units ... end units;
    physical_type_definition: $ => seq(
      $._kw_range, $._range_or_expression,
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
      optional(seq($._kw_generic, $._entity_generic_list, optional(';'))),
      repeat($._protected_type_declarative_item),
      optional(seq(
        $._kw_private,
        repeat($._protected_type_private_item)
      )),
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
      $.subprogram_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.constant_declaration,
      $.alias_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.private_variable_declaration,
      $.use_clause
    ),

    _protected_type_private_item: $ => choice(
      $.variable_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.attribute_specification
    ),

    _protected_type_body_item: $ => choice(
      $.type_declaration,
      $.subtype_declaration,
      $.constant_declaration,
      $.variable_declaration,
      $.file_declaration,
      $.shared_variable_declaration,
      $.alias_declaration,
      $.attribute_specification,
      $.subprogram_body,
      $.use_clause
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
      choice($._type_mark, $.anonymous_type_indication)  // element type
    ),

    // Index constraint for array type: "type range <>" or "1 to 10" or "integer range 1 to 8"
    _array_index_constraint: $ => choice(
      $.anonymous_type_indication,
      seq($.identifier, $._kw_range, '<>'),  // Unconstrained: integer range <>
      seq($.identifier, $._kw_range, $._expression, choice($._kw_to, $._kw_downto), $._expression),  // Type with range: integer range 1 to 8
      seq($.identifier, $._kw_range, $._expression),  // Attribute range: integer range t'range
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Constrained: 1 to 10
      $._name  // Type or attribute name: integer, t'range(1)
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
    // Resolution can be: identifier, (identifier), or element resolution
    subtype_declaration: $ => seq(
      $._kw_subtype,
      field('name', $.identifier),
      $._kw_is,
      optional(field('resolution', choice(
        seq('(', $.identifier, ')'),  // Resolution function in parens: (resolved)
        $.identifier                    // Resolution function as identifier
      ))),
      field('indication', $._type_mark),
      ';'
    ),

    // -------------------------------------------------------------------------
    // alias_declaration
    // -------------------------------------------------------------------------
    // alias identifier [: type] is name [signature];
    // signature = [type_mark {, type_mark} [return type_mark]]
    alias_declaration: $ => seq(
      $._kw_alias,
      field('name', choice($.identifier, $._operator_symbol)),  // Can alias operators too
      optional(seq(':', $._type_mark)),  // Optional type constraint
      $._kw_is,
      field('aliased_name', choice($._name, $._type_mark, $._operator_symbol)),
      optional($._signature),  // Optional subprogram signature
      ';'
    ),

    // Subprogram signature for alias declarations
    _signature: $ => seq(
      '[',
      optional(choice(
        seq($._kw_return, $._type_mark),
        seq(
          $._type_mark,
          repeat(seq(',', $._type_mark)),
          optional(seq($._kw_return, $._type_mark))
        )
      )),
      ']'
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
      optional(seq($._kw_generic, $._entity_generic_list)),
      optional(choice(
        seq($._kw_parameter, $._parameter_list),
        $._parameter_list
      )),
      $._kw_return,
      field('return_type', $._return_type),
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
      optional(seq($._kw_generic, $._entity_generic_list)),
      optional(choice(
        seq($._kw_parameter, $._parameter_list),
        $._parameter_list
      )),
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

    // VHDL-2008: Subprogram instantiation
    // procedure name is new subprogram_name [generic map (...)] ;
    // function name is new subprogram_name [generic map (...)] ;
    subprogram_instantiation: $ => seq(
      choice($._kw_procedure, $._kw_function),
      field('name', choice($.identifier, $._operator_symbol)),
      $._kw_is,
      $._kw_new,
      field('target', $._name),
      optional($.generic_map_aspect),
      ';'
    ),

    _operator_symbol: _ => /"[^"]+"/,  // String literals for operator overloading

    _parameter_list: $ => seq(
      '(',
      optional(seq(
        $.parameter,
        repeat(seq(';', $.parameter)),
        optional(';')  // VHDL-2019: optional trailing semicolon
      )),
      ')'
    ),

    // parameter: [signal|variable|constant] name[, name...] : [in|out|inout] type [:= default]
    parameter: $ => choice(
      seq(
        $._kw_procedure,
        field('name', $.identifier),
        optional($._parameter_list)
      ),
      seq(
        $._kw_function,
        field('name', $.identifier),
        optional($._parameter_list),
        $._kw_return,
        field('return_type', $._return_type)
      ),
      seq(
        optional(field('class', $.parameter_class)),
        $.identifier,
        repeat(seq(',', $.identifier)),
        ':',
        optional(field('direction', $.port_direction)),
        choice(
          $._parameter_type,
          $.anonymous_type_indication
        ),
        optional(seq(':=', field('default', $.default_value)))  // default value
      )
    ),

    // VHDL-2019: anonymous type indication in port list
    // Example: A : type is private; B : type is <>
    anonymous_type_indication: $ => seq(
      $._kw_type,
      $._kw_is,
      choice($._kw_private, '<>', seq('(', '<>', ')'))
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
    default_value: $ => $._expression,

    number: _ => /[0-9][0-9_]*(\.[0-9][0-9_]*)?/,  // Integer or floating point (underscores allowed)
    based_literal: _ => /(?:[0-9][0-9_]*#[0-9a-fA-F_]+(\.[0-9a-fA-F_]+)?#([eE][+-]?[0-9_]+)?|[0-9][0-9_]*:[0-9a-fA-F_]+(\.[0-9a-fA-F_]+)?:([eE][+-]?[0-9_]+)?)/,
    physical_literal: $ => seq(
      $.number,
      $.identifier
    ),

    // String literals including VHDL-specific formats
    // Bit string literals (X"...", B"...", O"...") are handled by external scanner
    // (see src/scanner.c) because Tree-sitter's lexer would otherwise grab "X" as identifier
    _string_literal: $ => choice(
      /"([^"]|"")*"/,             // Regular string "text" with doubled quotes
      /%([^%]|%%)*%/,             // Percent-delimited string: %% => %
      $.bit_string_literal,       // X"1A", B"1010", O"17" (from external scanner)
    ),

    // Invalid prefixed strings like bx"00" (invalid base prefix)
    // Tokenized to allow explicit error reporting in the driver.
    invalid_prefixed_string_literal: _ => token(prec(1, choice(
      /[A-Za-z][A-Za-z0-9_]+\"([^"]|"")*\"/,
      /[0-9][0-9_]*[sSuU]?[A-Za-z_][A-Za-z0-9_]+\"([^"]|"")*\"/
    ))),

    // bit_string_literal is declared in externals and handled by src/scanner.c
    // This gives it priority over the identifier rule, solving the X"..." tokenization issue

    // Type in parameter context - allow identifiers with optional constraints
    // Examples: "integer", "std_logic", "std_logic_vector(7 downto 0)"
    // VHDL-2008: Also supports selected names like mux_g.mux_data_array(0 to 7)
    _parameter_type: $ => choice(
      prec.dynamic(1, seq(
        field('resolution', $._name),
        $._type_mark,
        optional(choice($._kw_bus, $._kw_register))
      )),
      prec.dynamic(-1, seq(
        $._type_mark,
        optional(choice($._kw_bus, $._kw_register))
      ))
    ),

    _return_type: $ => choice(
      $._type_mark,
      seq($._name, $._kw_of, $._type_mark)
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
      optional(seq($._kw_generic, $._entity_generic_list, optional(';'))),
      optional(seq($._kw_port, $._parameter_list, ';')),
      $._kw_end,
      optional($._kw_component),
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
      $.subprogram_body,
      $.type_declaration,  // For protected type bodies
      $.constant_declaration,
      $.shared_variable_declaration,  // Shared variables in package body
      $.alias_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.use_clause,
      $.file_declaration,
      $.subtype_declaration,
      $.subprogram_instantiation,
      $.package_instantiation,
      $.package_declaration,
      $.package_body
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
      optional(seq($._kw_generic, $._entity_generic_list, optional(';'))),  // VHDL-2008: supports type generics
      optional(seq($._kw_port, $._parameter_list, ';')),
      repeat($._entity_declarative_item),
      optional(seq(
        $._kw_begin,
        repeat($._entity_statement)
      )),
      $._end_entity_clause
    ),

    // VHDL-2008: Entity generic list can include type, function, and constant generics
    _entity_generic_list: $ => seq(
      '(',
      optional(seq(
        $.generic_item,
        repeat(seq(';', $.generic_item)),
        optional(';')
      )),
      ')'
    ),

    _entity_statement: $ => choice(
      $.assert_statement,
      $.process_statement,
      $.subprogram_declaration,  // Procedure/function in entity
      $.concurrent_procedure_call  // Passive procedure call
    ),

    _end_entity_clause: $ => seq(
      $._kw_end,
      optional(choice(
        seq($._kw_entity, optional($.identifier)),
        seq($.identifier, optional($.identifier))
      )),
      optional(';')
    ),

    assert_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label for concurrent assertions
      optional($._kw_postponed),
      $._kw_assert,
      choice(
        prec.dynamic(1, $._expression),
        $._psl_assert_expression
      ),
      optional(seq(
        choice($._kw_abort, $._kw_sync_abort, $._kw_async_abort),
        $._expression
      )),
      optional(seq('@', $._expression)),
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    // Report expression - strings and concatenation, but stops at $._kw_severity keyword
    _report_expression: $ => repeat1(choice(
      seq('(', $._expression, ')'),
      $._string_literal,
      $.character_literal,
      $.qualified_expression,
      $._name,
      seq($._function_call_in_expr, '.', choice($.identifier, $._kw_all)),
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

    // PSL (Property Specification Language) - minimal support for 2019 tests
    psl_property_declaration: $ => seq(
      $._kw_property,
      $.identifier,
      $._kw_is,
      $._psl_expression,
      ';'
    ),

    psl_sequence_declaration: $ => seq(
      $._kw_sequence,
      $.identifier,
      $._kw_is,
      $.psl_sequence,
      ';'
    ),

    psl_assert_statement: $ => seq(
      optional(seq($.identifier, ':')),
      optional($._kw_postponed),
      $._kw_assert,
      $._psl_expression,
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_cover_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_cover,
      choice($._expression, $.psl_sequence),
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_assume_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_assume,
      choice($._expression, $.psl_sequence),
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_restrict_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_restrict,
      $.psl_sequence,
      optional(seq($._kw_report, $._report_expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_sequence: $ => seq(
      '{',
      $._psl_sequence_item,
      repeat(seq(';', $._psl_sequence_item)),
      '}',
      optional($.psl_repetition)
    ),

    _psl_sequence_item: $ => seq(
      $._psl_expression,
      optional($.psl_repetition)
    ),

    psl_repetition: _ => token(seq(
      '[',
      /[*+=-]/,
      /[^\]]*/,
      ']'
    )),

    psl_next_event_expression: $ => seq(
      field('name', $._name),
      '(',
      field('event', $._expression),
      ')',
      '[',
      field('range', $._range_or_expression),
      ']',
      '(',
      field('condition', $._expression),
      ')'
    ),

    psl_bracketed_call: $ => seq(
      field('name', $._name),
      '[',
      field('range', $._range_or_expression),
      ']',
      seq('(', field('condition', $._expression), ')')
    ),

    _psl_expression: $ => repeat1(choice(
      $.psl_sequence,
      $.psl_bracketed_call,
      $.psl_next_event_expression,
      $.psl_next_expression,
      $.psl_parenthesized_expression,
      $._name,
      $.number,
      $._literal,
      $.psl_repetition,
      $.logical_operator,
      $.relational_operator,
      $._kw_not,
      $._kw_eventually,
      '!',
      '|=>',
      '|->',
      '->',
      '=>',
      '+',
      '-',
      '*',
      '/',
      '&',
      '|',
      '.'
    )),

    psl_always_expression: $ => prec.right(seq(
      $._kw_always,
      $._psl_expression
    )),

    psl_never_expression: $ => prec.right(seq(
      $._kw_never,
      $._psl_expression
    )),

    psl_parenthesized_expression: $ => seq(
      '(',
      $._psl_expression,
      ')'
    ),

    _psl_next_operand: $ => choice(
      seq($._kw_not, $._name),
      $._name,
      $.number,
      $._literal
    ),

    _psl_property_operand: $ => choice(
      $.psl_always_expression,
      $.psl_never_expression,
      $.psl_next_expression,
      $.psl_sequence
    ),

    _psl_property_expression: $ => choice(
      prec.dynamic(1, seq(
        $._psl_property_operand,
        repeat1(seq(
          choice('|=>', '|->', '=>', '->'),
          $._psl_property_operand
        ))
      )),
      prec.dynamic(-1, $._psl_property_operand)
    ),

    _psl_assert_expression: $ => $._psl_property_expression,

    _entity_declarative_item: $ => choice(
      $.use_clause,
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.signal_declaration,
      $.disconnect_specification,
      $.shared_variable_declaration,
      $.group_declaration,
      $.group_template_declaration,
      $.package_declaration,
      $.package_body,
      $.package_instantiation
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
      $._end_architecture_clause
    ),

    _end_architecture_clause: $ => seq(
      $._kw_end,
      optional(choice(
        seq($._kw_architecture, optional($.identifier)),
        seq($.identifier, optional($.identifier))
      )),
      optional(';')
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
      field('entity', $._name),
      $._kw_is,
      repeat($._configuration_item),
      $._kw_end,
      optional($._kw_configuration),
      optional($.identifier),
      ';'
    ),

    _configuration_item: $ => choice(
      $.use_clause,
      $._block_configuration
    ),

    _block_configuration: $ => seq(
      $._kw_for,
      $.identifier,  // architecture or generate label
      optional(seq('(', $._range_or_expression, ')')),  // Generate index/range
      repeat(choice($.use_clause, $._configuration_item_or_component)),
      $._kw_end,
      $._kw_for,
      ';'
    ),

    _configuration_item_or_component: $ => choice(
      $._component_configuration,
      $._block_configuration
    ),

    _component_configuration: $ => seq(
      $._kw_for,
      choice(
        seq($.identifier, repeat(seq(',', $.identifier)), ':', $.identifier),  // inst1,inst2 : component
        seq($._kw_all, ':', $.identifier),         // all : component
        seq($._kw_others, ':', $.identifier),      // others : component
        $.identifier                           // Just generate label
      ),
      optional(choice(
        // Full binding indication with use entity
        seq(
          $._kw_use,
          choice(
            seq(
              $._kw_entity,
              field('entity', choice(
                seq($.identifier, '.', $.identifier),
                $.identifier
              )),
              optional(seq('(', $.identifier, ')'))  // Optional architecture
            ),
            seq(
              $._kw_configuration,
              field('configuration', $._name)
            )
          ),
          optional(seq($._kw_generic, $._kw_map, '(', /[^)]+/, ')')),  // Generic map
          optional(seq($._kw_port, $._kw_map, '(', /[^)]+/, ')')),     // Port map
          ';'  // Semicolon after binding indication
        ),
        seq(
          $._kw_use,
          $._kw_open,
          ';'
        ),
        // Incremental binding indication (generic map and/or port map without use)
        seq(
          optional(seq($._kw_generic, $._kw_map, '(', /[^)]+/, ')')),  // Generic map
          optional(seq($._kw_port, $._kw_map, '(', /[^)]+/, ')')),     // Port map
          ';'  // Semicolon after incremental binding
        )
      )),
      repeat($._block_configuration),
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
      repeat(choice($.library_clause, $.use_clause, $.context_reference)),
      $._kw_end,
      optional($._kw_context),
      optional($.identifier),
      ';'
    ),

    // VHDL-2008: context reference - importing a context
    // context lib.ctx;
    context_reference: $ => seq(
      $._kw_context,
      $._name,  // lib.context_name
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
      $.subprogram_instantiation,
      $.signal_declaration,
      $.component_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.shared_variable_declaration,
      $.file_declaration,
      $.configuration_specification,
      $.package_declaration,
      $.package_body,
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

    _group_constituent: $ => $._name,

    view_declaration: $ => seq(
      $._kw_view,
      $.identifier,
      $._kw_of,
      $._name,
      $._kw_is,
      repeat1($.view_element),
      $._kw_end,
      $._kw_view,
      ';'
    ),

    view_element: $ => seq(
      $.identifier,
      ':',
      $.port_direction,
      ';'
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
      choice(
        seq($.identifier, repeat(seq(',', $.identifier))),  // Component label list
        $._kw_all,
        $._kw_others
      ),
      ':',
      $.identifier,  // Component name
      $._kw_use,
      choice(
        seq($._kw_entity, $._entity_aspect),  // use entity work.foo(bar)
        seq($._kw_configuration, $._name),  // use configuration work.cfg
        $._kw_open  // use open
      ),
      optional(seq($._kw_generic, $._kw_map, '(', /[^)]*/, ')')),
      optional(seq($._kw_port, $._kw_map, '(', /[^)]*/, ')')),
      ';'
    ),

    // Entity aspect: library.entity(architecture)
    _entity_aspect: $ => seq(
      $._name,
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
      optional(seq(':=', field('value', $._constant_value))),
      ';'
    ),

    // File declaration
    // File declaration: file name : type [open mode] [is "filename"];
    file_declaration: $ => seq(
      $._kw_file,
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),  // Multiple file names
      ':',
      field('type', $._type_mark),
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
      field('target', choice(
        seq(choice($.identifier, $._operator_symbol), repeat(seq(',', choice($.identifier, $._operator_symbol)))),
        $._kw_others,
        $._kw_all
      )),
      optional($._signature),
      ':',
      field('class', $._entity_class),
      $._kw_is,
      field('value', $._expression),  // Can be any expression: string, number, identifier, aggregate
      ';'
    ),

    // Entity class names (some are keywords)
    _entity_class: $ => choice(
      $._kw_entity,
      $._kw_architecture,
      $._kw_configuration,
      $._kw_procedure,
      $._kw_function,
      $._kw_package,
      $._kw_type,
      $._kw_subtype,
      $._kw_constant,
      $._kw_signal,
      $._kw_variable,
      $._kw_component,
      $._kw_file,
      $.identifier  // For label, literal, units, group
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
    _signal_type_indication: $ => choice(
      prec.dynamic(1, seq(
        field('resolution', $._name),
        $._type_mark,
        optional(choice($._kw_bus, $._kw_register))
      )),
      prec.dynamic(-1, seq(
        $._type_mark,
        optional(choice($._kw_bus, $._kw_register))
      ))
    ),

    _kw_bus: _ => token(prec(1, /[bB][uU][sS]/)),
    _kw_register: _ => token(prec(1, /[rR][eE][gG][iI][sS][tT][eE][rR]/)),

    // Concurrent statements (architecture body)
    _concurrent_statement: $ => choice(
      $.generate_statement,
      $.block_statement,
      $.signal_assignment,
      $.process_statement,
      $.psl_default_clock,
      $.psl_property_declaration,
      $.psl_sequence_declaration,
      $.psl_cover_statement,
      $.psl_assume_statement,
      $.psl_restrict_statement,
      $.component_instantiation,
      $.assert_statement,  // Concurrent assertion
      $.concurrent_procedure_call  // Concurrent procedure call: proc_name;
    ),

    // Concurrent procedure call: procedure_name [(args)];
    concurrent_procedure_call: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      $._name,
      ';'
    ),

    // Generate statements: for generate, if generate, case generate (VHDL-2008)
    // IMPORTANT: for_generate, if_generate, case_generate are VISIBLE nodes (no underscore)
    // This allows the extractor to easily detect generate type
    generate_statement: $ => seq(
      field('label', $._generate_label),
      ':',
      choice(
        $.for_generate,
        $.if_generate,
        $.case_generate  // VHDL-2008
      )
    ),

    _generate_label: $ => choice(
      $.identifier,
      $._kw_default
    ),

    for_generate: $ => seq(
      $._kw_for,
      field('loop_var', $.identifier),
      $._kw_in,
      field('range', $._discrete_range),
      $._kw_generate,
      optional($._generate_body),
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    if_generate: $ => seq(
      $._kw_if,
      optional(seq($.identifier, ':')),
      field('condition', $._expression),
      $._kw_generate,
      optional($._generate_body_or_block),
      repeat($._generate_elsif),
      optional($._generate_else),  // VHDL-2008
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    _generate_elsif: $ => seq(
      $._kw_elsif,
      optional(seq($.identifier, ':')),
      $._expression,
      $._kw_generate,
      optional($._generate_body_or_block)
    ),

    _generate_else: $ => seq(
      $._kw_else,
      optional(seq($.identifier, ':')),
      $._kw_generate,
      optional($._generate_body_or_block)
    ),

    case_generate: $ => seq(
      $._kw_case,
      field('expression', $._expression),
      $._kw_generate,
      repeat($._case_generate_alternative),
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    _case_generate_alternative: $ => seq(
      $._kw_when,
      optional(seq($.identifier, ':')),
      $._choice_list,
      '=>',
      optional($._generate_body_or_block)
    ),

    _generate_body: $ => choice(
      prec.dynamic(1, seq(
        repeat1($._block_declarative_item),
        $._kw_begin,
        repeat($._concurrent_statement)
      )),
      prec.dynamic(0, seq(
        $._kw_begin,
        repeat($._concurrent_statement)
      )),
      prec.dynamic(-1, seq(
        optional($._kw_begin),
        repeat1($._concurrent_statement)
      ))
    ),

    _generate_block: $ => seq(
      repeat($._block_declarative_item),
      $._kw_begin,
      repeat($._concurrent_statement),
      $._kw_end,
      optional($.identifier),
      ';'
    ),

    _generate_body_or_block: $ => choice(
      $._generate_body,
      $._generate_block
    ),

    // Range expression: 0 to 10, vec'range, etc.
    _range_or_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Explicit range
      $._expression  // Attribute like vec'range
    ),

    // Discrete range: range or subtype indication (used in loops/slices)
    _discrete_range: $ => choice(
      $._index_subtype_indication,
      $._range_or_expression
    ),

    // Block statement with optional generic/port interface
    block_statement: $ => seq(
      field('label', $.identifier),
      ':',
      $._kw_block,
      optional(seq('(', $._expression, ')')),  // Guard condition
      optional($._kw_is),
      optional(seq($._kw_generic, $._parameter_list, ';')),  // Generic interface
      optional(seq($._kw_generic, $._kw_map, '(', $.association_list, ')', ';')),  // Generic map
      optional(seq($._kw_port, $._parameter_list, ';')),  // Port interface
      optional(seq($._kw_port, $._kw_map, '(', $.association_list, ')', ';')),  // Port map
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
      prec.dynamic(1, $._conditional_signal_assignment),
      prec.dynamic(-1, $._simple_signal_assignment),
      $._selected_signal_assignment,
      $._force_release_assignment
    ),

    // Simple signal assignment with waveform: signal <= [transport] value [after time] {, value [after time]};
    _simple_signal_assignment: $ => seq(
      optional(seq($.identifier, ':')),
      $._assignment_target,  // Can be indexed or aggregate
      '<=',
      optional($._kw_guarded),  // For guarded signal assignments in blocks
      optional($._delay_mechanism),  // Optional delay mechanism
      $._waveform_element_no_when,
      repeat(seq(',', $._waveform_element_no_when)),
      ';'
    ),

    _kw_transport: _ => /[tT][rR][aA][nN][sS][pP][oO][rR][tT]/,
    _kw_inertial: _ => /[iI][nN][eE][rR][tT][iI][aA][lL]/,
    _delay_mechanism: $ => choice($._kw_transport, $._kw_inertial),

    // target <= expr [after time] when cond else expr [after time] when cond else expr [after time];
    _conditional_signal_assignment: $ => prec.dynamic(5, seq(
      optional(seq($.identifier, ':')),
      $._assignment_target,  // Can be indexed or aggregate
      '<=',
      optional($._kw_guarded),
      optional($._delay_mechanism),
      $._waveform,
      $._kw_when,
      $._expression,
      repeat(seq($._kw_else, optional($._waveform), optional(seq($._kw_when, $._expression)))),
      ';'
    )),

    // with selector select target <= value [after time] when choice, value when others;
    _selected_signal_assignment: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_with,
      $._expression,
      $._kw_select,
      $._assignment_target,  // Can be indexed or aggregate
      '<=',
      optional($._delay_mechanism),
      $._waveform,
      $._kw_when,
      $._choice_list,  // Can be range like 1 to 19
      repeat(seq(',', $._waveform, $._kw_when, $._choice_list)),
      ';'
    ),

    // Choice in case/select - can be expression, range, or others
    _choice_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Range: 1 to 19
      $._kw_others,  // others
      $._expression  // Simple expression
    ),

    _choice_list: $ => seq(
      $._choice_expression,
      repeat(seq(choice('|', '!'), $._choice_expression))
    ),

    // VHDL-2008 force/release
    _force_release_assignment: $ => prec.dynamic(1, seq(
      optional(seq($.identifier, ':')),
      $._name,  // Can be indexed: data(i) <= force ...
      '<=',
      choice($._kw_force, $._kw_release),
      optional($._expression),
      optional(seq($._kw_when, $._expression)),
      ';'
    )),

    process_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      optional($._kw_postponed),
      $._kw_process,
      optional(seq('(', $.sensitivity_list, ')')),  // Sensitivity list
      optional($._kw_is),
      repeat($._process_declarative_item),
      $._kw_begin,
      repeat($._sequential_statement),
      $._kw_end,
      optional($._kw_postponed),
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
      $.variable_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.constant_declaration,
      $.file_declaration,  // file x : file_type [open mode is "name"];
      $.alias_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.package_instantiation,
      $.subprogram_instantiation,
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
          optional(seq(field('library', $.identifier), '.')),
          field('entity', $.identifier),
          optional(seq('(', field('architecture', $.identifier), ')'))
        ),
        // Direct configuration instantiation
        seq(
          $._kw_configuration,
          field('configuration', $._name)
        ),
        // Explicit component keyword
        seq(
          $._kw_component,
          field('component', $._type_name),
          optional(seq('(', field('architecture', $.identifier), ')'))
        ),
        // Component instantiation: component_name
        prec.dynamic(2, seq(
          field('component', $._type_name),
          optional(seq('(', field('architecture', $.identifier), ')'))
        ))
      ),
      optional($.generic_map_aspect),
      optional($.port_map_aspect),
      ';'
    ),

    // Visible wrapper nodes for map aspects - enables extraction
    generic_map_aspect: $ => seq(
      $._kw_generic, $._kw_map, '(', $.association_list, ')'
    ),

    port_map_aspect: $ => seq(
      $._kw_port, $._kw_map, '(', $.association_list, ')'
    ),

    // Association list for port/generic maps
    association_list: $ => seq(
      $.association_element,
      repeat(seq(choice(',', ';'), $.association_element)),
      optional(choice(',', ';'))
    ),

    // Association element in port/generic maps - visible for extraction
    association_element: $ => choice(
      // Named association: port_name => signal_name
      seq(
        field('formal', $._name),
        '=>',
        field('actual', choice($._expression, $._kw_open))
      ),
      // Positional association: just the signal/expression
      field('actual', $._expression)
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
      $.variable_declaration,
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.attribute_declaration,
      $.attribute_specification,
      $.alias_declaration,
      $.file_declaration,
      $.shared_variable_declaration,
      $.use_clause,
      $.subprogram_instantiation,
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

    private_variable_declaration: $ => seq(
      $._kw_private,
      $.variable_declaration
    ),

    // Statements (simplified - just match anything until semicolon for now)
    // Order matters for ambiguous cases - signal assignment before procedure call
    _sequential_statement: $ => choice(
      prec(3, $.sequential_block_statement),
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
      prec(3, $._conditional_signal_assignment),
      prec(3, $._selected_signal_assignment),
      prec(3, $.selected_variable_assignment),
      prec(2, $.assignment_statement),
      prec(1, $.procedure_call_statement)       // Procedure call - lowest for identifier(...)
    ),

    // VHDL-2019: sequential block statement inside subprograms/processes
    sequential_block_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_block,
      optional($._kw_is),
      repeat($._subprogram_declarative_item),
      $._kw_begin,
      repeat($._sequential_statement),
      $._kw_end,
      $._kw_block,
      optional($.identifier),
      ';'
    ),

    // Report statement (standalone, without assertion)
    report_statement: $ => seq(
      $._kw_report,
      $._report_expression,  // Message string, can include concatenation
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    // Sequential signal assignment (inside process/function)
    // Use prec.dynamic for runtime conflict resolution with procedure_call_statement
    // Supports waveform: signal <= value [after time] {, value [after time]};
    sequential_signal_assignment: $ => prec.dynamic(10, seq(
      optional(seq($.identifier, ':')),
      $._assignment_target,  // Includes unified names and aggregates
      '<=',
      optional($._delay_mechanism),
      $._waveform_element,
      repeat(seq(',', $._waveform_element)),
      ';'
    )),

    // Selected variable assignment (sequential)
    // with selector select target := value when choice, value when others;
    selected_variable_assignment: $ => seq(
      $._kw_with,
      $._expression,
      $._kw_select,
      $._assignment_target,
      ':=',
      $._expression,
      $._kw_when,
      $._choice_list,
      repeat(seq(',', $._expression, $._kw_when, $._choice_list)),
      ';'
    ),

    // Waveform element: value [after time_expression]
    _waveform_element: $ => seq(
      $._expression_no_conditional,
      optional(seq($._kw_after, $._expression))
    ),

    _waveform_element_no_when: $ => seq(
      $._expression_no_conditional,
      optional(seq($._kw_after, $._expression))
    ),

    _waveform: $ => seq(
      $._waveform_element_no_when,
      repeat(seq(',', $._waveform_element_no_when))
    ),

    exit_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_exit,
      optional($.identifier),  // Optional loop label
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    next_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_next,
      optional($.identifier),  // Optional loop label
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    // Case statement (regular and VHDL-2008 matching case?)
    // [label :] case[?] expression is ... end case[?] [label];
    case_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),  // VHDL-2008: optional label
      choice($._kw_case, seq($._kw_case, '?')),  // case or case?
      field('expression', $._expression),
      $._kw_is,
      repeat1($.case_alternative),
      $._kw_end,
      choice($._kw_case, seq($._kw_case, '?')),  // end case or end case?
      optional($.identifier),  // VHDL-2008: optional end label
      ';'
    ),

    // Visible case alternative for extraction (enables latch detection)
    case_alternative: $ => seq(
      $._kw_when,
      $.case_choice,
      repeat(seq('|', $.case_choice)),  // Multiple choices: when A | B | C =>
      '=>',
      repeat($._sequential_statement)
    ),

    // Case choice - visible to detect 'others' for latch analysis
    case_choice: $ => choice(
      $.others_choice,  // others keyword
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Range
      $._expression  // Simple value
    ),

    // Explicit 'others' node for easy detection
    others_choice: _ => /[oO][tT][hH][eE][rR][sS]/,

    // Wait statement: wait [on signal_list] [until condition] [for time_expression];
    wait_statement: $ => seq(
      $._kw_wait,
      optional(seq($._kw_on, $._signal_name, repeat(seq(',', $._signal_name)))),  // sensitivity clause
      optional(seq($._kw_until, $._expression)),  // condition clause
      optional(seq($._kw_for, $._expression)),    // timeout clause
      ';'
    ),

    // Signal name - used in sensitivity lists (wait on X, process(X))
    _signal_name: $ => $._name,

    return_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_return,
      optional($._expression),
      ';'
    ),

    assignment_statement: $ => seq(
      optional(seq($.identifier, ':')),
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
      $._name,
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

    // =========================================================================
    // UNIFIED NAME SYSTEM
    // Fixes "indexed_name" errors by deferring semantic checks.
    // =========================================================================

    // The atomic leaf of a name
    _simple_name: $ => choice(
      $.identifier,
      $.external_name,
      $._operator_symbol
    ),

    // The recursive master rule
    _name: $ => choice(
      $._simple_name,

      // Dot notation (Record access, package selection)
      // "my_pkg.constant" or "record.field" or "pkg.'X'" (character literal selection)
      prec.left(8, seq(
        field('prefix', $._name),
        '.',
        field('suffix', choice(
          $._simple_name,
          $._kw_all,
          $._kw_others,
          $.character_literal,  // For selecting character literals from packages
          $._operator_symbol    // For selecting operator symbols from packages
        ))
      )),

      // Generic map suffix for subprogram calls (VHDL-2019)
      prec.left(8, seq(
        field('prefix', $._name),
        $._kw_generic,
        $._kw_map,
        '(',
        field('generic_map', $.association_list),
        ')'
      )),

      // Parentheses (Array indexing, Slicing, Function calls, Type constraints)
      // "arr(1)", "func(x)", "std_logic_vector(7 downto 0)"
      prec.left(9, seq(
        field('prefix', $._name),
        '(',
        field('content', choice(
          $.association_list,
          $._index_expression_unified
        )),
        ')'
      )),

      // Attributes (Tick notation)
      // "clk'event", "arr'range"
      prec.left(10, seq(
        field('prefix', $._name),
        optional($._signature),
        "'",
        field('attribute', $.identifier),
        optional(seq('(', $._expression, ')'))
      ))
    ),

    _name_with_signature: $ => seq(
      $._name,
      $._signature
    ),

    // Index expression - supports identifiers, numbers, ranges, operators
    _index_expression_unified: $ => seq(
      $._index_expression_item,
      repeat(seq(',', $._index_expression_item))
    ),

    _index_expression_item: $ => choice(
      $._index_subtype_indication,
      prec.right(1, seq($._expression, choice($._kw_to, $._kw_downto), $._name)),
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),
      $._expression
    ),

    // Simplified if statement (VHDL-2008: supports optional label)
    // [label :] if condition then ... end if [label];
    if_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),  // VHDL-2008: optional label
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
      optional($.identifier),  // VHDL-2008: optional end label
      ';'
    ),

    // Simplified loop (while, for, or infinite) with optional label
    // label : [while cond | for i in range] loop ... end loop [label];
    loop_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      optional(choice(
        seq($._kw_while, $._expression),  // while loop
        seq($._kw_for, $.identifier, $._kw_in, $._discrete_range)  // for loop
      )),
      $._kw_loop,
      repeat($._sequential_statement),
      $._kw_end,
      $._kw_loop,
      optional($.identifier),  // Optional label at end
      ';'
    ),

    // =========================================================================
    // EXPRESSION HIERARCHY
    // =========================================================================
    // VHDL expressions have precedence (highest to lowest):
    //   1. Primary (literals, names, function calls, aggregates)
    //   2. Exponential (**)
    //   3. Multiplicative (*, /, mod, rem)
    //   4. Sign (+, - unary)
    //   5. Additive (+, -, &)
    //   6. Shift (sll, srl, sla, sra, rol, ror)
    //   7. Relational (=, /=, <, <=, >, >=)
    //   8. Logical (and, or, xor, nand, nor, xnor)
    //
    // We create VISIBLE nodes for expressions we want to analyze:
    //   - relational_expression: for trojan/trigger detection
    //   - multiplicative_expression: for power analysis
    //   - exponential_expression: for power analysis
    //
    // This structured approach lets the extractor simply walk these nodes
    // instead of trying to reconstruct expression structure from flat tokens.
    // =========================================================================

    // Expression entry point - try structured expressions first, fall back to flat
    _expression: $ => choice(
      $.conditional_expression,  // VHDL-2008/2019: expr when cond else expr
      $._logical_expression,
      prec(-1, repeat1($._expression_term))  // Fallback for edge cases
    ),

    // Expression variant that disallows top-level conditional expressions.
    // Used in waveform contexts to avoid eating "when" from signal assignments.
    _expression_no_conditional: $ => choice(
      $._logical_expression,
      prec(-1, repeat1($._expression_term))
    ),

    // Conditional expression (right-associative)
    conditional_expression: $ => prec.right(1, seq(
      field('consequence', $._logical_expression),
      $._kw_when,
      field('condition', $._expression),
      $._kw_else,
      field('alternative', $._expression)
    )),

    // Logical expression (lowest precedence) - can contain relational
    _logical_expression: $ => choice(
      prec.left(1, seq($._logical_expression, $.logical_operator, $._relational_expression)),
      $._relational_expression
    ),

    // Relational expression - VISIBLE NODE for security analysis (trojan detection)
    // Captures: left_operand COMPARE right_operand
    _relational_expression: $ => choice(
      $.relational_expression,  // Use the visible node
      $._shift_expression
    ),

    // Visible relational expression node for extraction
    relational_expression: $ => prec.left(2, seq(
      field('left', $._shift_expression),
      field('operator', $.relational_operator),
      field('right', $._shift_expression)
    )),

    // Shift expression
    _shift_expression: $ => choice(
      prec.left(3, seq($._shift_expression, $.shift_operator, $._additive_expression)),
      $._additive_expression
    ),

    // Additive expression (+, -, &)
    _additive_expression: $ => choice(
      prec.left(4, seq($._additive_expression, $.additive_operator, $._multiplicative_expression)),
      $._multiplicative_expression
    ),

    // Multiplicative expression - VISIBLE NODE for power analysis
    // Captures: left_operand * right_operand (expensive in hardware!)
    _multiplicative_expression: $ => choice(
      $.multiplicative_expression,  // Use the visible node
      $._exponential_expression
    ),

    // Visible multiplicative expression node for extraction (*, /, mod, rem)
    multiplicative_expression: $ => prec.left(5, seq(
      field('left', $._exponential_expression),
      field('operator', $.multiplicative_operator),
      field('right', $._exponential_expression)
    )),

    // Exponential expression - VISIBLE NODE for power analysis
    // Captures: base ** exponent (even more expensive!)
    _exponential_expression: $ => choice(
      $.exponential_expression,  // Use the visible node
      $._unary_expression
    ),

    // Visible exponential expression node for extraction
    exponential_expression: $ => prec.right(6, seq(
      field('base', $._unary_expression),
      '**',
      field('exponent', $._unary_expression)
    )),

    // Unary expression (not, abs, +, -)
    _unary_expression: $ => choice(
      prec(7, seq($.condition_operator, $._unary_expression)),
      prec(7, seq($._kw_not, $._unary_expression)),
      prec(7, seq($._kw_abs, $._unary_expression)),
      prec(7, seq(choice('+', '-'), $._unary_expression)),
      $._primary_expression
    ),

    // Primary expressions (highest precedence)
    _primary_expression: $ => choice(
      prec(10, $._literal),
      $._name_with_signature,
      $._name,
      $.allocator_expression,
      $.qualified_expression,
      $.physical_literal,
      $.based_literal,
      $.number,
      $._parenthesized_expression
    ),

    // Keep _expression_term for backward compatibility (some rules use it directly)
    _expression_term: $ => choice(
      prec(10, $._literal),  // Must be early and high precedence to catch char literals
      $.allocator_expression,  // new Type or new Type'(value)
      $.qualified_expression,  // type'(expr) - must be before _name
      $._name_with_signature,
      $._name,
      $.physical_literal,
      $.based_literal,
      $.number,
      $.relational_operator,  // =, /=, <, >, <=, >= (visible for semantic analysis)
      $.logical_operator,     // and, or, xor, etc. (visible for semantic analysis)
      $.arithmetic_operator,  // +, -, *, /, etc. (visible for semantic analysis)
      $.shift_operator,       // sll, srl, sla, sra, rol, ror
      $._kw_not,              // Unary not
      $._kw_abs,              // Unary abs
      $._parenthesized_expression  // Grouped or aggregate
    ),

    // Visible operator nodes for semantic analysis
    relational_operator: _ => choice('=', '/=', '<', '>', '<=', '>=', '?=', '?/=', '?<', '?>', '?<=', '?>='),
    logical_operator: _ => choice(/[aA][nN][dD]/, /[oO][rR]/, /[xX][oO][rR]/, /[nN][aA][nN][dD]/, /[nN][oO][rR]/, /[xX][nN][oO][rR]/),
    // Split arithmetic into additive and multiplicative for proper precedence
    additive_operator: _ => choice('+', '-', '&'),
    multiplicative_operator: _ => choice('*', '/', /[mM][oO][dD]/, /[rR][eE][mM]/),
    // Keep combined arithmetic_operator for backward compatibility
    arithmetic_operator: _ => choice('+', '-', '*', '/', '**', '&', /[mM][oO][dD]/, /[rR][eE][mM]/),
    shift_operator: _ => choice(/[sS][lL][lL]/, /[sS][rR][lL]/, /[sS][lL][aA]/, /[sS][rR][aA]/, /[rR][oO][lL]/, /[rR][oO][rR]/),
    condition_operator: _ => '??',

    // PSL: next[count](expression) - temporal operator
    psl_next_expression: $ => seq(
      $._kw_next,
      optional(seq('[', $._range_or_expression, ']')),  // Optional count/range
      choice(
        $.psl_parenthesized_expression,
        $._psl_next_operand
      )
    ),


    // Qualified expression: type'(expression)
    // e.g., string'("hello"), integer'(x + 1), character'('a')
    // Make identifier'( a single token so lexer won't see '( as character literal
    // Note: This doesn't allow whitespace between ' and ( (e.g., type' (x))
    qualified_expression: $ => choice(
      seq(
        token(prec(2, seq(/[_a-zA-Z][a-zA-Z0-9_.]*/, "'("))),  // identifier'( as single token
        optional(seq($._aggregate_element, repeat(seq(',', $._aggregate_element)))),
        ')'
      ),
      prec(1, seq(
        $._type_mark,
        "'",
        '(',
        optional(seq($._aggregate_element, repeat(seq(',', $._aggregate_element)))),
        ')'
      ))
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
      '@',
      $._external_path_element
    )),

    _external_path_element: $ => seq(
      $.identifier,
      optional(seq('(', $._expression, ')'))
    ),

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
    // Uses structured _expression for proper unary operator handling inside parens
    _aggregate_element: $ => choice(
      seq($._kw_others, '=>', $._expression),  // others => value
      seq($._aggregate_choice_list, '=>', $._expression),  // choices: 0 | 1 => '1'
      prec(5, $.character_literal),  // Single character literal with high precedence
      $._expression  // Just a value - uses structured expression hierarchy
    ),

    _aggregate_choice_list: $ => seq(
      $._aggregate_choice_expression,
      repeat(seq(choice('|', '!'), $._aggregate_choice_expression))
    ),

    _aggregate_choice_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Range: 1 to 7
      $.character_literal,
      $.number,
      $._name
    ),

    // Character literal as explicit rule
    character_literal: _ => /'[^']'/,

    // Literals (character, string, bit string)
    // Character literal: 'x' (single char) or ''' (apostrophe)
    // Bit string literals handled by external scanner (src/scanner.c)
    _literal: $ => prec(10, choice(
      $.character_literal,    // Single character: 'a', '0', ' ', etc.
      /'''/,                  // Apostrophe character: '''
      $._string_literal,      // Includes quoted, percent-delimited, and bit strings
      $.invalid_prefixed_string_literal
    )),

    // Procedure call statement - lower precedence than assignments
    procedure_call_statement: $ => prec.dynamic(-1, seq(
      optional(seq($.identifier, ':')),  // Optional label
      $._name,
      ';'
    )),

    // Procedure argument - allows function calls, named associations, aggregates
    _procedure_argument: $ => choice(
      seq($._name, '=>', $._procedure_arg_value),  // Named
      $._procedure_arg_value  // Positional
    ),

    // Value in a procedure argument (without the named association part)
    _procedure_arg_value: $ => $._expression,

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
