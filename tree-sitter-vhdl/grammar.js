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
function kw(word) {
  return new RegExp(word.split('').map(c => `[${c.toLowerCase()}${c.toUpperCase()}]`).join(''));
}

module.exports = grammar({
  name: 'vhdl',

  externals: $ => [
    $.bit_string_literal,
    $.invalid_bit_string_literal,
  ],

  word: $ => $.identifier,

  extras: $ => [
    /\s+/,
    $.comment,
    $.block_comment,
    $.protect_directive,
  ],

  conflicts: $ => [
    [$.package_declaration, $._package_declarative_item],
    [$._end_entity_clause],
    [$.association_element, $._index_expression_item],
    [$.concurrent_procedure_call, $.component_instantiation],
    [$.component_instantiation, $._simple_name],
    [$.generic_procedure_declaration, $.parameter],
    [$.generic_function_declaration, $.parameter],
    [$._index_spec, $._simple_name],
    [$._array_index_constraint, $._simple_name],
    [$._generic_type_indication, $._simple_name],
    [$._generic_array_index, $._array_index_constraint, $._simple_name],
    [$._generic_array_index, $._array_index_constraint],
    [$._index_spec, $._index_expression_item],
    [$._index_spec, $._primary_expression],
    [$.type_declaration],
    [$._configuration_item_or_component, $.component_configuration],
    [$.block_configuration, $.component_configuration],
    [$.alias_declaration, $._simple_name],
    [$._protected_type_private_item, $.private_variable_declaration],
    [$._waveform_element, $._waveform_element_no_when],
    [$._end_architecture_clause],
    [$.psl_next_event_expression, $.association_element, $._index_expression_item],
    [
      $.assert_statement,
      $.psl_cover_statement,
      $.psl_assume_statement,
      $.psl_restrict_statement,
      $.concurrent_procedure_call,
      $._generate_label,
      $.block_statement,
      $.simple_signal_assignment,
      $.conditional_signal_assignment,
      $.selected_signal_assignment,
      $.force_release_assignment,
      $.process_statement,
      $.component_instantiation,
    ],
    [$.force_release_assignment, $._assignment_target],
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
    _kw_library: _ => kw('library'),
    _kw_use: _ => kw('use'),
    _kw_package: _ => kw('package'),
    _kw_body: _ => kw('body'),
    _kw_is: _ => kw('is'),
    _kw_end: _ => kw('end'),
    _kw_units: _ => kw('units'),
    _kw_generic: _ => kw('generic'),
    _kw_type: _ => kw('type'),
    _kw_function: _ => kw('function'),
    _kw_procedure: _ => kw('procedure'),
    _kw_return: _ => kw('return'),
    _kw_parameter: _ => kw('parameter'),
    _kw_pure: _ => kw('pure'),
    _kw_impure: _ => kw('impure'),
    _kw_private: _ => kw('private'),
    _kw_signal: _ => kw('signal'),
    _kw_variable: _ => kw('variable'),
    _kw_constant: _ => kw('constant'),
    _kw_file: _ => kw('file'),
    _kw_in: _ => kw('in'),
    _kw_out: _ => kw('out'),
    _kw_inout: _ => kw('inout'),
    _kw_buffer: _ => kw('buffer'),
    _kw_linkage: _ => kw('linkage'),
    _kw_subtype: _ => kw('subtype'),
    _kw_alias: _ => kw('alias'),
    _kw_component: _ => kw('component'),
    _kw_entity: _ => kw('entity'),
    _kw_architecture: _ => kw('architecture'),
    _kw_configuration: _ => kw('configuration'),
    _kw_open: _ => kw('open'),
    _kw_context: _ => kw('context'),
    _kw_port: _ => kw('port'),
    _kw_map: _ => kw('map'),
    _kw_process: _ => kw('process'),
    _kw_begin: _ => kw('begin'),
    _kw_wait: _ => kw('wait'),
    _kw_null: _ => kw('null'),
    _kw_until: _ => kw('until'),
    _kw_for: _ => kw('for'),
    _kw_on: _ => kw('on'),
    _kw_if: _ => kw('if'),
    _kw_then: _ => kw('then'),
    _kw_elsif: _ => kw('elsif'),
    _kw_else: _ => kw('else'),
    _kw_case: _ => kw('case'),
    _kw_when: _ => kw('when'),
    _kw_others: _ => kw('others'),
    _kw_loop: _ => kw('loop'),
    _kw_while: _ => kw('while'),
    _kw_exit: _ => kw('exit'),
    _kw_next: _ => kw('next'),
    _kw_assert: _ => kw('assert'),
    _kw_assume: _ => kw('assume'),
    _kw_cover: _ => kw('cover'),
    _kw_property: _ => kw('property'),
    _kw_sequence: _ => kw('sequence'),
    _kw_restrict: _ => kw('restrict'),
    _kw_eventually: _ => kw('eventually'),
    _kw_never: _ => kw('never'),
    _kw_report: _ => kw('report'),
    _kw_severity: _ => kw('severity'),
    _kw_postponed: _ => kw('postponed'),
    _kw_abort: _ => kw('abort'),
    _kw_sync_abort: _ => kw('sync_abort'),
    _kw_async_abort: _ => kw('async_abort'),
    _kw_and: _ => kw('and'),
    _kw_or: _ => kw('or'),
    _kw_xor: _ => kw('xor'),
    _kw_nand: _ => kw('nand'),
    _kw_nor: _ => kw('nor'),
    _kw_xnor: _ => kw('xnor'),
    _kw_not: _ => kw('not'),
    _kw_mod: _ => kw('mod'),
    _kw_rem: _ => kw('rem'),
    _kw_abs: _ => kw('abs'),
    _kw_sll: _ => kw('sll'),
    _kw_srl: _ => kw('srl'),
    _kw_sla: _ => kw('sla'),
    _kw_sra: _ => kw('sra'),
    _kw_rol: _ => kw('rol'),
    _kw_ror: _ => kw('ror'),
    _kw_force: _ => kw('force'),
    _kw_release: _ => kw('release'),
    _kw_guarded: _ => kw('guarded'),
    _kw_unaffected: _ => kw('unaffected'),
    _kw_reject: _ => kw('reject'),
    _kw_select: _ => kw('select'),
    _kw_with: _ => kw('with'),
    _kw_record: _ => kw('record'),
    _kw_array: _ => kw('array'),
    _kw_access: _ => kw('access'),
    _kw_protected: _ => kw('protected'),
    _kw_shared: _ => kw('shared'),
    _kw_attribute: _ => kw('attribute'),
    _kw_of: _ => kw('of'),
    _kw_range: _ => kw('range'),
    _kw_to: _ => kw('to'),
    _kw_downto: _ => kw('downto'),
    _kw_after: _ => kw('after'),
    _kw_generate: _ => kw('generate'),
    _kw_block: _ => kw('block'),
    _kw_all: _ => kw('all'),
    _kw_open: _ => kw('open'),
    _kw_new: _ => kw('new'),
    // PSL keywords (Property Specification Language)
    _kw_default: _ => kw('default'),
    _kw_clock: _ => kw('clock'),
    _kw_always: _ => kw('always'),
    _kw_disconnect: _ => kw('disconnect'),
    _kw_group: _ => kw('group'),
    _kw_view: _ => kw('view'),

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
      /[a-zA-Z](_?[a-zA-Z0-9])*/,
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
      field('names', $.identifier_list),
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

    // Type mark - unified name with optional constraints, or attribute-based subtype
    _type_mark: $ => prec(-1, seq(
      $._name,
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
      $.range_expression,  // 0 to 7, 7 downto 0
      $._name,  // type'Range, signal'Range
      $.identifier  // Just a type/subtype name
    ),

    // Subtype indication used in index/range contexts (e.g., integer range 1 to 3)
    _index_subtype_indication: $ => seq(
      $._name,
      $._kw_range,
      $._expression,
      optional(seq(choice($._kw_to, $._kw_downto), $._expression))
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
      $.subprogram_declaration,
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
      repeat1($.element_declaration),
      $._kw_end,
      $._kw_record,
      optional($.identifier)
    ),

    element_declaration: $ => seq(
      field('names', $.identifier_list),
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
      field('name', choice($.identifier, $.character_literal, $._operator_symbol)),  // Can alias operators/char literals too
      optional(seq(':', field('type', $._type_mark))),  // Optional type constraint
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

    // Simple comma-separated identifier list
    identifier_list: $ => seq(
      $.identifier,
      repeat(seq(',', $.identifier))
    ),

    // Port clause with visible parameter list for extraction
    port_clause: $ => seq(
      $._kw_port,
      field('ports', alias($._parameter_list, $.parameter_list)),
      ';'
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
        field('names', $.identifier_list),
        ':',
        optional(field('direction', $.port_direction)),
        field('type', alias(choice(
          $._parameter_type,
          $.anonymous_type_indication
        ), $.parameter_type)),
        optional(seq(':=', field('default', $.default_value)))  // default value
      )
    ),

    // VHDL-2019: anonymous type indication in port list
    // Example: A : type is private; B : type is <>
    anonymous_type_indication: $ => seq(
      $._kw_type,
      $._kw_is,
      choice($._kw_private, '<>')
    ),

    // Port/parameter direction - visible for extraction
    port_direction: _ => choice(
      kw('in'),
      kw('out'),
      kw('inout'),
      kw('buffer'),
      kw('linkage')
    ),

    // Parameter class - visible for extraction
    parameter_class: _ => choice(
      kw('signal'),
      kw('variable'),
      kw('constant'),
      kw('file')
    ),

    // Default values can be identifiers, numbers, literals, or expressions
    // Visible for extraction
    default_value: $ => $._expression,

    number: _ => /[0-9](_?[0-9])*(\.[0-9](_?[0-9])*)?([eE][-+]?[0-9](_?[0-9])*)?/,  // Integer/real with optional exponent
    based_literal: _ => /(?:[0-9][0-9_]*#[0-9a-fA-F_]+(\.[0-9a-fA-F_]+)?#([eE][+-]?[0-9_]+)?|[0-9][0-9_]*:[0-9a-fA-F_]+(\.[0-9a-fA-F_]+)?:([eE][+-]?[0-9_]+)?)/,
    physical_literal: $ => seq(
      $.number,
      $.identifier
    ),

    // String literals including VHDL-specific formats
    // Bit string literals (X"...", B"...", O"...") are handled by external scanner
    // (see src/scanner.c) because Tree-sitter's lexer would otherwise grab "X" as identifier
    string_literal: $ => choice(
      /"([^"\r\n]|"")*"/,         // Regular string "text" with doubled quotes, single-line only
      /%([^%\r\n]|%%)*%/,         // Percent-delimited string: %% => %, single-line only
      $.bit_string_literal,       // X"1A", B"1010", O"17" (from external scanner)
      $.invalid_bit_string_literal
    ),

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
      optional($.port_clause),
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
      $.subprogram_declaration,
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
      optional($.port_clause),
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
      optional(seq($._kw_report, $._expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
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
      optional(seq($._kw_report, $._expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_cover_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_cover,
      choice($._expression, $.psl_sequence),
      optional(seq($._kw_report, $._expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_assume_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_assume,
      choice($._expression, $.psl_sequence),
      optional(seq($._kw_report, $._expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_restrict_statement: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_restrict,
      $.psl_sequence,
      optional(seq($._kw_report, $._expression)),
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    psl_sequence: $ => seq(
      '{',
      $._psl_sequence_item,
      repeat(seq(';', $._psl_sequence_item)),
      '}'
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
      $.block_configuration
    ),

    block_configuration: $ => seq(
      $._kw_for,
      field('label', $.identifier),  // architecture or generate label
      optional(seq('(', field('index', $._range_or_expression), ')')),  // Generate index/range
      repeat(choice($.use_clause, $._configuration_item_or_component)),
      $._kw_end,
      $._kw_for,
      ';'
    ),

    _configuration_item_or_component: $ => choice(
      $.component_configuration,
      $.block_configuration
    ),

    component_configuration: $ => seq(
      $._kw_for,
      choice(
        seq($.identifier, repeat(seq(',', $.identifier)), ':', field('component', $.identifier)),  // inst1,inst2 : component
        seq($._kw_all, ':', field('component', $.identifier)),         // all : component
        seq($._kw_others, ':', field('component', $.identifier)),      // others : component
        field('label', $.identifier)                           // Just generate label
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
              optional(seq('(', field('architecture', $.identifier), ')'))  // Optional architecture
            ),
            seq(
              $._kw_configuration,
              field('configuration', $._name)
            )
          ),
          optional($.generic_map_aspect),
          optional($.port_map_aspect),
          ';'  // Semicolon after binding indication
        ),
        seq(
          $._kw_use,
          $._kw_open,
          ';'
        ),
        // Incremental binding indication (generic map and/or port map without use)
        seq(
          optional($.generic_map_aspect),
          optional($.port_map_aspect),
          ';'  // Semicolon after incremental binding
        )
      )),
      repeat($.block_configuration),
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
      $.package_instantiation,  // VHDL-2008: local package instantiation  // PSL: default clock declaration
      $.disconnect_specification,  // Disconnection specification for guarded signals
      $.group_declaration,  // Group declaration
      $.group_template_declaration  // Group template declaration
    ),

    // Disconnection specification: disconnect signal : type after time;
    disconnect_specification: $ => seq(
      $._kw_disconnect,
      field('target', choice($.identifier, $._kw_all, $._kw_others)),
      ':',
      field('type', $._type_mark),
      $._kw_after,
      field('time', $._expression),
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
      optional($.generic_map_aspect),
      optional($.port_map_aspect),
      ';'
    ),

    // Entity aspect: library.entity(architecture)
    _entity_aspect: $ => seq(
      field('entity', $._entity_name),
      optional(seq('(', field('architecture', $.identifier), ')'))
    ),

    _entity_name: $ => choice(
      $._simple_name,
      prec.left(8, seq(
        field('prefix', $._entity_name),
        '.',
        field('suffix', choice(
          $._simple_name,
          $.character_literal,
          $._operator_symbol
        ))
      ))
    ),

    // VHDL-2008: Shared variable (requires protected type)
    shared_variable_declaration: $ => seq(
      $._kw_shared,
      $._kw_variable,
      field('names', $.identifier_list),
      ':',
      field('type', $._type_mark),
      optional(seq(':=', field('value', $._constant_value))),
      ';'
    ),

    // File declaration
    // File declaration: file name : type [open mode] [is "filename"];
    file_declaration: $ => seq(
      $._kw_file,
      field('names', $.identifier_list),
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
      field('names', $.identifier_list),
      ':',
      field('type', alias($._signal_type_indication, $.signal_type_indication)),
      optional(seq(':=', field('value', $._constant_value))),
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

    _kw_bus: _ => token(prec(1, kw('bus'))),
    _kw_register: _ => token(prec(1, kw('register'))),

    // Concurrent statements (architecture body)
    _concurrent_statement: $ => choice(
      $.generate_statement,
      $.block_statement,
      $.signal_assignment,
      $.process_statement,
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
      field('range', alias($._discrete_range, $.discrete_range)),  // Visible wrapper for ChildByFieldName
      $._kw_generate,
      optional($._generate_body),
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    if_generate: $ => seq(
      $._kw_if,
      optional(seq($.identifier, ':')),
      field('condition', $._expression),
      $._kw_generate,
      optional($._generate_body),
      repeat($.elsif_generate_clause),
      optional($.else_generate_clause),  // VHDL-2008
      $._kw_end, $._kw_generate, optional($.identifier), ';'
    ),

    elsif_generate_clause: $ => seq(
      $._kw_elsif,
      optional(seq($.identifier, ':')),
      $._expression,
      $._kw_generate,
      optional($._generate_body)
    ),

    else_generate_clause: $ => seq(
      $._kw_else,
      optional(seq($.identifier, ':')),
      $._kw_generate,
      optional($._generate_body)
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
      optional($._generate_body)
    ),

    _generate_body: $ => choice(
      prec.dynamic(1, seq(
        repeat1($._block_declarative_item),
        $._kw_begin,
        repeat($._concurrent_statement)
      )),
      prec.dynamic(0, repeat1($._concurrent_statement))
    ),

    // Range expression: 0 to 10, vec'range, etc.
    _range_or_expression: $ => choice(
      seq($._expression, choice($._kw_to, $._kw_downto), $._expression),  // Explicit range
      $._expression  // Attribute like vec'range
    ),

    // Visible range expression with named fields for low/high/direction
    // Used for easier extraction of for-generate ranges
    range_expression: $ => seq(
      field('low', $._expression),
      field('direction', alias(choice($._kw_to, $._kw_downto), $.direction)),  // Visible wrapper for direction
      field('high', $._expression)
    ),

    // Discrete range: range or subtype indication (used in loops/slices)
    _discrete_range: $ => choice(
      $._index_subtype_indication,
      $.range_expression,  // Use visible range_expression for explicit ranges
      $._expression  // Attribute like vec'range (fallback)
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
      optional($.port_clause),  // Port interface
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
      prec.dynamic(2, $.force_release_assignment),
      prec.dynamic(1, $.conditional_signal_assignment),
      prec.dynamic(-1, $.simple_signal_assignment),
      $.selected_signal_assignment
    ),

    // Simple signal assignment with waveform: signal <= [transport] value [after time] {, value [after time]};
    simple_signal_assignment: $ => seq(
      optional(seq($.identifier, ':')),
      field('target', alias($._assignment_target, $.assignment_target)),  // Visible wrapper for ChildByFieldName
      '<=',
      optional($._kw_guarded),  // For guarded signal assignments in blocks
      optional($._delay_mechanism),  // Optional delay mechanism
      $._waveform_element_no_when,
      repeat(seq(',', $._waveform_element_no_when)),
      ';'
    ),

    _kw_transport: _ => kw('transport'),
    _kw_inertial: _ => kw('inertial'),
    _delay_mechanism: $ => choice(
      $._kw_transport,
      seq(optional(seq($._kw_reject, $._expression)), $._kw_inertial)
    ),

    // target <= expr [after time] when cond else expr [after time] when cond else expr [after time];
    conditional_signal_assignment: $ => prec.dynamic(5, seq(
      optional(seq($.identifier, ':')),
      field('target', alias($._assignment_target, $.assignment_target)),  // Visible wrapper for ChildByFieldName
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
    selected_signal_assignment: $ => seq(
      optional(seq($.identifier, ':')),
      $._kw_with,
      $._expression,
      $._kw_select,
      field('target', alias($._assignment_target, $.assignment_target)),  // Visible wrapper for ChildByFieldName
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
    force_release_assignment: $ => prec.dynamic(10, seq(
      optional(seq($.identifier, ':')),
      field('target', alias($._name, $.assignment_target)),  // Visible wrapper for ChildByFieldName
      '<=',
      choice($._kw_force, $._kw_release),
      optional($._logical_expression),  // Use non-conditional to avoid consuming 'when'
      optional(seq($._kw_when, $._expression)),
      ';'
    )),

    process_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),  // Optional label
      optional($._kw_postponed),
      $._kw_process,
      optional(seq('(', field('sensitivity', $.sensitivity_list), ')')),  // Sensitivity list
      optional($._kw_is),
      repeat($._process_declarative_item),
      $._kw_begin,
      repeat($._sequential_statement),
      $._kw_end,
      optional($._kw_postponed),
      $._kw_process,
      optional(field('end_label', $.identifier)),
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
          field('component', $._name),
          optional(seq('(', field('architecture', $.identifier), ')'))
        ),
        // Component instantiation: component_name
        prec.dynamic(2, seq(
          field('component', $._name),
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
      $.subprogram_declaration  // Nested functions/procedures
    ),

    variable_declaration: $ => seq(
      $._kw_variable,
      field('names', $.identifier_list),
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
      prec(3, $.null_statement),
      prec(3, $.assert_statement),              // Sequential assertion
      prec(3, $.report_statement),              // Report without assertion
      prec(3, $.sequential_signal_assignment),  // Signal assignment - try before proc call
      prec(3, $.conditional_signal_assignment),
      prec(3, $.selected_signal_assignment),
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
      $._expression,  // Message expression
      optional(seq($._kw_severity, $._expression)),
      ';'
    ),

    null_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
      $._kw_null,
      ';'
    ),

    // Sequential signal assignment (inside process/function)
    // Use prec.dynamic for runtime conflict resolution with procedure_call_statement
    // Supports waveform: signal <= value [after time] {, value [after time]};
    sequential_signal_assignment: $ => prec.dynamic(10, seq(
      optional(seq($.identifier, ':')),
      field('target', alias($._assignment_target, $.assignment_target)),  // Visible wrapper for ChildByFieldName
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

    _waveform_element_no_when: $ => choice(
      $._kw_unaffected,
      seq(
        $._expression_no_conditional,
        optional(seq($._kw_after, $._expression))
      )
    ),

    _waveform: $ => seq(
      $._waveform_element_no_when,
      repeat(seq(',', $._waveform_element_no_when))
    ),

    exit_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
      $._kw_exit,
      optional($.identifier),  // Optional loop label
      optional(seq($._kw_when, $._expression)),
      ';'
    ),

    next_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
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
    others_choice: _ => kw('others'),

    // Wait statement: wait [on signal_list] [until condition] [for time_expression];
    wait_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
      $._kw_wait,
      optional(seq($._kw_on, field('sensitivity', $.sensitivity_list))),  // sensitivity clause
      optional(seq($._kw_until, field('condition', $._expression))),  // condition clause
      optional(seq($._kw_for, field('timeout', $._expression))),    // timeout clause
      ';'
    ),

    // Signal name - used in sensitivity lists (wait on X, process(X))
    _signal_name: $ => $._name,

    return_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
      $._kw_return,
      optional($._expression),
      ';'
    ),

    assignment_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),
      $._assignment_target,  // Can be identifier, selected name, or indexed/sliced name
      ':=',
      $._expression,
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
      $.selected_name,
      $.name_with_generic_map,
      $.indexed_name,
      $.attribute_name
    ),

    selected_name: $ => prec.left(8, seq(
      field('prefix', $._name),
      '.',
      field('suffix', choice(
        $._simple_name,
        $._kw_all,
        $._kw_others,
        $.character_literal,
        $._operator_symbol
      ))
    )),

    // Generic map suffix for subprogram calls (VHDL-2019)
    name_with_generic_map: $ => prec.left(8, seq(
      field('prefix', $._name),
      $._kw_generic,
      $._kw_map,
      '(',
      field('generic_map', $.association_list),
      ')'
    )),

    // Parentheses (array indexing, slicing, function calls, type constraints)
    indexed_name: $ => prec.left(9, seq(
      field('prefix', $._name),
      '(',
      field('content', choice(
        $.association_list,
        $.index_expression_list
      )),
      ')'
    )),

    // Attributes (tick notation)
    attribute_name: $ => prec.left(10, seq(
      field('prefix', $._name),
      optional($._signature),
      token.immediate("'"),
      field('attribute', $.identifier),
      optional(seq('(', $._expression, ')'))
    )),
    _name_with_signature: $ => seq(
      $._name,
      $._signature
    ),

    // Index expression - supports identifiers, numbers, ranges, operators
    index_expression_list: $ => seq(
      $._index_expression_item,
      repeat(seq(',', $._index_expression_item))
    ),

    _index_expression_item: $ => choice(
      $._index_subtype_indication,
      $.range_expression, // <--- Replaces the manual seq(...) for to/downto
      $._expression
    ),

    // Simplified if statement (VHDL-2008: supports optional label)
    // [label :] if condition then ... end if [label];
    if_statement: $ => seq(
      optional(seq(field('label', $.identifier), ':')),  // VHDL-2008: optional label
      $._kw_if,
      field('condition', alias($._expression, $.condition)),  // Wrapper node for ChildByFieldName
      $._kw_then,
      repeat($._sequential_statement),
      repeat($.elsif_clause),
      optional($.else_clause),
      $._kw_end,
      $._kw_if,
      optional($.identifier),  // VHDL-2008: optional end label
      ';'
    ),

    elsif_clause: $ => seq(
      $._kw_elsif,
      field('condition', alias($._expression, $.condition)),
      $._kw_then,
      repeat($._sequential_statement)
    ),

    else_clause: $ => seq(
      $._kw_else,
      repeat($._sequential_statement)
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

    // Expression entry point
    _expression: $ => choice(
      $.conditional_expression,  // VHDL-2008/2019: expr when cond else expr
      $._logical_expression
    ),

    // Expression variant that disallows top-level conditional expressions.
    // Used in waveform contexts to avoid eating "when" from signal assignments.
    _expression_no_conditional: $ => $._logical_expression,

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
      $.logical_expression,
      $.non_associative_logical_expression,
      $._relational_expression
    ),

    logical_expression: $ => prec.left(1, seq(
      field('left', choice($.logical_expression, $._relational_expression)),
      field('operator', $.associative_logical_operator),
      field('right', $._relational_expression)
    )),

    non_associative_logical_expression: $ => prec(1, seq(
      field('left', $._relational_expression),
      field('operator', $.non_associative_logical_operator),
      field('right', $._relational_expression)
    )),

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
      $.additive_expression,
      $._sign_expression
    ),

    additive_expression: $ => prec.left(4, seq(
      field('left', $._additive_expression),
      field('operator', $.additive_operator),
      field('right', $._sign_expression)
    )),

    _sign_expression: $ => choice(
      seq(choice('+', '-'), $._multiplicative_expression),
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

    // Unary expression (not/abs/reduction operators)
    _unary_expression: $ => choice(
      prec(7, seq($.condition_operator, $._unary_expression)),
      prec(7, seq($._kw_not, $._unary_expression)),
      prec(7, seq($._kw_abs, $._unary_expression)),
      prec(7, seq($.logical_operator, $._unary_expression)),
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

    // _expression_term removed — all expression parsing now uses the structured
    // hierarchy (_logical_expression → _relational_expression → ... → _primary).

    // Visible operator nodes for semantic analysis
    relational_operator: _ => choice('=', '/=', '<', '>', '<=', '>=', '?=', '?/=', '?<', '?>', '?<=', '?>='),
    associative_logical_operator: $ => choice($._kw_and, $._kw_or, $._kw_xor),
    non_associative_logical_operator: $ => choice($._kw_nand, $._kw_nor, $._kw_xnor),
    logical_operator: $ => choice(
      $.associative_logical_operator,
      $.non_associative_logical_operator
    ),
    // Split arithmetic into additive and multiplicative for proper precedence
    additive_operator: _ => choice('+', '-', '&'),
    multiplicative_operator: $ => choice('*', '/', $._kw_mod, $._kw_rem),
    // Keep combined arithmetic_operator for backward compatibility
    arithmetic_operator: $ => choice('+', '-', '*', '/', '**', '&', $._kw_mod, $._kw_rem),
    shift_operator: $ => choice($._kw_sll, $._kw_srl, $._kw_sla, $._kw_sra, $._kw_rol, $._kw_ror),
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
    qualified_expression: $ => seq(
      field('type', $._type_mark),
      token.immediate("'"),
      '(',
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
      $.string_literal       // Includes quoted, percent-delimited, and bit strings
    )),

    // Procedure call statement - lower precedence than assignments
    procedure_call_statement: $ => prec.dynamic(-1, seq(
      optional(seq($.identifier, ':')),  // Optional label
      $._name,
      ';'
    )),

    // =========================================================================
    // Add more VHDL rules here as you learn
    // =========================================================================
  }
});
