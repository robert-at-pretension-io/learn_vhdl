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
//                       Example: seq('if', $.condition, 'then')
//
//   choice(a, b, c)     Match a OR b OR c (first one that works)
//                       Example: choice('and', 'or', 'xor')
//
//   repeat(rule)        Match rule zero or more times (like regex *)
//                       Example: repeat($.statement) -- 0+ statements
//
//   repeat1(rule)       Match rule one or more times (like regex +)
//                       Example: repeat1($.identifier) -- 1+ identifiers
//
//   optional(rule)      Match rule zero or one time (like regex ?)
//                       Example: optional('entity') -- 'entity' or nothing
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
//                       Example: 'entity', 'begin', ';'
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
  // WORD - DISABLED - causes issues with the grammar
  // ===========================================================================
  // word: $ => $.identifier,

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
    $.comment,    // VHDL comments can appear anywhere
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
  // Package vs package body can conflict since both start with 'package'
  // ===========================================================================
  conflicts: $ => [
    [$.package_declaration, $._package_declarative_item],
    [$._block_declarative_item, $._concurrent_statement],
    [$._conditional_signal_assignment, $._force_release_assignment, $._name],
    [$.indexed_name, $._procedure_argument],  // identifier inside parens ambiguity
    [$._block_configuration, $._component_configuration]  // for identifier...
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
    identifier: $ => token(seq(/[_a-zA-Z]+/, /[a-zA-Z0-9_]*/)),
    selector_clause: $=> prec.left(3, repeat1(seq('.', $.identifier))),
    library_clause: $=> seq('library', $.identifier, ';'),
    use_clause: $ => seq('use', $.identifier, optional($.selector_clause), ';'),

    // -------------------------------------------------------------------------
    // package_declaration
    // -------------------------------------------------------------------------
    // package identifier is
    //   [generic (...);]  -- VHDL-2008
    //   declarations...
    // end [package] [identifier];
    package_declaration: $ => seq(
      'package',
      field('name', $.identifier),
      'is',
      optional($._package_generic_clause),  // VHDL-2008 package generics
      repeat($._package_declarative_item),
      'end',
      optional('package'),
      optional($.identifier),
      ';'
    ),

    // VHDL-2008: Package generic clause
    _package_generic_clause: $ => seq(
      'generic',
      '(',
      $.generic_item,
      repeat(seq(';', $.generic_item)),
      ')',
      ';'
    ),

    // Generic item can be type, constant, or function
    generic_item: $ => choice(
      seq('type', $.identifier),  // Generic type parameter
      seq('function', $.identifier, $._parameter_list, 'return', $.identifier),  // Generic function
      $.parameter  // Generic constant (like normal parameter)
    ),

    _package_declarative_item: $ => choice(
      $.comment,
      $.constant_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.alias_declaration,
      $.subprogram_declaration,
      $.component_declaration,
      $._package_generic_clause  // Also allowed as declarative item for flexibility
    ),

    // -------------------------------------------------------------------------
    // constant_declaration
    // -------------------------------------------------------------------------
    // constant identifier : type := expression;
    constant_declaration: $ => seq(
      'constant',
      field('name', $.identifier),
      ':',
      field('type', $._type_mark),
      ':=',
      field('value', $._constant_value),
      ';'
    ),

    // Constant values can be various literals and expressions
    _constant_value: $ => choice(
      $.number,
      $.identifier,
      $._string_literal,
      seq($.number, $.identifier),  // e.g., "10 ns" for time
      seq($.identifier, $._string_literal),  // e.g., "x"AA"" - but actually parsed as single literal
      seq('(', /[^)]+/, ')')  // Aggregate or expression in parens
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
      seq('access', $.identifier),        // Access type: access integer
      seq(
        $.identifier,
        optional(choice(
          seq('(', /[^)]+/, ')'),         // Constraint in parens
          seq('range', /[^;]+/),          // Range constraint
          seq('of', $.identifier)         // For "file of X" types
        ))
      )
    ),

    // Type mark - a type name, possibly with constraints like std_logic_vector(7 downto 0)
    // Includes closing parens for the constraints
    // Type mark - structured to stop at keywords like 'is'
    _type_mark: $ => prec(-1, seq(
      $.identifier,
      optional(choice(
        seq('(', /[^)]+/, ')'),  // Parenthesized constraint
        seq('range', $._expression)  // Range constraint: range 0 to 100
      ))
    )),

    // -------------------------------------------------------------------------
    // type_declaration
    // -------------------------------------------------------------------------
    // type identifier is type_definition;
    type_declaration: $ => seq(
      'type',
      field('name', $.identifier),
      'is',
      field('definition', choice(
        prec(10, $.record_type_definition),
        prec(10, $.enumeration_type_definition),
        prec(10, $.array_type_definition),
        prec(10, $.protected_type_declaration),
        prec(10, $.protected_type_body),
        prec(-10, $._type_expression)    // Fallback for simple/constrained types
      )),
      optional(';')
    ),

    // VHDL-2008: Protected type declaration (interface)
    // type name is protected
    //   procedure/function declarations...
    // end protected [name];
    protected_type_declaration: $ => seq(
      'protected',
      repeat($._protected_type_declarative_item),
      'end',
      'protected',
      optional($.identifier)
    ),

    // VHDL-2008: Protected type body (implementation)
    // type name is protected body
    //   variable declarations, procedure/function bodies...
    // end protected body [name];
    protected_type_body: $ => seq(
      'protected',
      'body',
      repeat($._protected_type_body_item),
      'end',
      'protected',
      'body',
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
    // (ID1, ID2, ID3, ...)
    enumeration_type_definition: $ => seq(
      '(',
      $.identifier,
      repeat(seq(',', $.identifier)),
      ')'
    ),

    // -------------------------------------------------------------------------
    // array_type_definition
    // -------------------------------------------------------------------------
    // array (range1, range2, ...) of element_type
    // array (index_type range <>) of element_type  -- unconstrained
    array_type_definition: $ => seq(
      'array',
      '(',
      $._simple_expression,  // index constraint(s)
      ')',
      'of',
      $._type_mark  // element type
    ),

    // -------------------------------------------------------------------------
    // record_type_definition
    // -------------------------------------------------------------------------
    // record
    //   field : type;
    //   ...
    // end record [identifier];
    record_type_definition: $ => seq(
      'record',
      repeat($.element_declaration),
      'end',
      'record',
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
    // subtype identifier is subtype_indication;
    subtype_declaration: $ => seq(
      'subtype',
      field('name', $.identifier),
      'is',
      field('indication', $._type_mark),
      ';'
    ),

    // -------------------------------------------------------------------------
    // alias_declaration
    // -------------------------------------------------------------------------
    // alias identifier [: type] is name;
    alias_declaration: $ => seq(
      'alias',
      field('name', $.identifier),
      optional(seq(':', $._type_mark)),  // Optional type constraint
      'is',
      field('aliased_name', $._type_mark),  // Use _type_mark to allow foo(7 downto 4)
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
      optional(choice('pure', 'impure')),  // VHDL-2008
      'function',
      field('name', choice($.identifier, $._operator_symbol)),
      optional($._parameter_list),
      'return',
      field('return_type', $.identifier),
      choice(
        ';',  // Declaration only
        seq(  // Body
          'is',
          repeat($._subprogram_declarative_item),
          'begin',
          repeat($._sequential_statement),
          'end',
          optional('function'),
          optional(choice($.identifier, $._operator_symbol)),  // Can be operator symbol too
          ';'
        )
      )
    ),

    // Alias for compatibility
    function_body: $ => $.function_declaration,

    // Procedure: unified rule that handles both declaration and body
    procedure_declaration: $ => seq(
      'procedure',
      field('name', $.identifier),
      optional($._parameter_list),
      choice(
        ';',  // Declaration only
        seq(  // Body
          'is',
          repeat($._subprogram_declarative_item),
          'begin',
          repeat($._sequential_statement),
          'end',
          optional('procedure'),
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
      optional(choice('signal', 'variable', 'constant', 'file')),
      $.identifier,
      repeat(seq(',', $.identifier)),
      ':',
      optional(choice('in', 'out', 'inout', 'buffer', 'linkage')),
      $._parameter_type,
      optional(seq(':=', $._default_value))  // default value
    ),

    // Default values can be identifiers, numbers, literals, or expressions
    _default_value: $ => choice(
      $.number,
      $.identifier,
      $._string_literal,
      seq('(', /[^)]+/, ')')  // Expression in parens
    ),

    number: _ => /[0-9]+(\.[0-9]+)?/,  // Integer or floating point

    // String literals including VHDL-specific formats
    _string_literal: _ => choice(
      /"[^"]*"/,              // Regular string "text"
      /x"[0-9a-fA-F_]*"/,     // Hex string x"1A"
      /o"[0-7_]*"/,           // Octal string o"17"
      /b"[01_]*"/,            // Binary string b"1010"
      /'[^']*'/               // Character literal 'a'
    ),

    // Type in parameter context - allow identifiers with optional constraints
    // Examples: "integer", "std_logic", "std_logic_vector(7 downto 0)"
    _parameter_type: $ => seq(
      $.identifier,
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
      'component',
      field('name', $.identifier),
      optional('is'),  // VHDL-93+: optional 'is'
      optional(seq('generic', $._parameter_list, ';')),
      optional(seq('port', $._parameter_list, ';')),
      'end',
      'component',
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
      'package',
      'body',
      field('name', $.identifier),
      'is',
      repeat($._package_body_declarative_item),
      'end',
      optional(seq('package', 'body')),
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
      'entity',
      field('name', $.identifier),
      'is',
      optional(seq('generic', $._parameter_list, ';')),
      optional(seq('port', $._parameter_list, ';')),
      repeat($._entity_declarative_item),
      optional(seq(
        'begin',
        repeat($._entity_statement)
      )),
      'end',
      optional('entity'),
      optional($.identifier),
      ';'
    ),

    _entity_statement: $ => choice(
      $.comment,
      $.assert_statement,
      $.process_statement
    ),

    assert_statement: $ => seq(
      'assert',
      $._expression,
      optional(seq('report', $._report_expression)),
      optional(seq('severity', $.identifier)),
      ';'
    ),

    // Report expression - strings and concatenation, but stops at 'severity' keyword
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
      $.subprogram_declaration
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
      'architecture',
      field('name', $.identifier),
      'of',
      field('entity', $.identifier),
      'is',
      repeat($._block_declarative_item),
      'begin',
      repeat($._concurrent_statement),
      'end',
      optional('architecture'),
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
      'configuration',
      field('name', $.identifier),
      'of',
      field('entity', $.identifier),
      'is',
      repeat($._configuration_item),
      'end',
      optional('configuration'),
      optional($.identifier),
      ';'
    ),

    _configuration_item: $ => choice(
      $.comment,
      $._block_configuration
    ),

    _block_configuration: $ => seq(
      'for',
      $.identifier,  // architecture or generate label
      repeat($._configuration_item_or_component),
      'end',
      'for',
      ';'
    ),

    _configuration_item_or_component: $ => choice(
      $.comment,
      $._component_configuration,
      $._block_configuration
    ),

    _component_configuration: $ => seq(
      'for',
      choice(
        seq($.identifier, ':', $.identifier),  // instance : component
        seq('all', ':', $.identifier),         // all : component
        seq('others', ':', $.identifier),      // others : component
        $.identifier                           // Just generate label
      ),
      optional(seq(
        'use',
        'entity',
        $.identifier,
        '.',
        $.identifier,
        optional(seq('(', $.identifier, ')')),  // Optional architecture
        ';'  // Semicolon after binding indication
      )),
      'end',
      'for',
      ';'
    ),

    // -------------------------------------------------------------------------
    // context_declaration (VHDL-2008)
    // -------------------------------------------------------------------------
    // context name is
    //   library ...; use ...;
    // end [context] [name];
    context_declaration: $ => seq(
      'context',
      field('name', $.identifier),
      'is',
      repeat(choice($.library_clause, $.use_clause, $.comment)),
      'end',
      optional('context'),
      optional($.identifier),
      ';'
    ),

    _block_declarative_item: $ => choice(
      $.comment,
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
      $.file_declaration
    ),

    // VHDL-2008: Shared variable (requires protected type)
    shared_variable_declaration: $ => seq(
      'shared',
      'variable',
      field('name', $.identifier),
      ':',
      field('type', $._type_mark),
      ';'
    ),

    // File declaration
    file_declaration: $ => seq(
      'file',
      field('name', $.identifier),
      ':',
      field('type', $.identifier),
      optional(seq('open', $.identifier, 'is', $._expression)),  // Optional file open
      ';'
    ),

    // attribute identifier : type;
    attribute_declaration: $ => seq(
      'attribute',
      field('name', $.identifier),
      ':',
      field('type', $.identifier),
      ';'
    ),

    // attribute identifier of entity_list : entity_class is expression;
    attribute_specification: $ => seq(
      'attribute',
      field('name', $.identifier),
      'of',
      field('target', $.identifier),
      ':',
      field('class', $.identifier),
      'is',
      field('value', choice($._string_literal, $.identifier)),
      ';'
    ),

    signal_declaration: $ => seq(
      'signal',
      field('name', $.identifier),
      repeat(seq(',', $.identifier)),
      ':',
      field('type', $._type_mark),
      optional(seq(':=', $._constant_value)),
      ';'
    ),

    // Concurrent statements (architecture body)
    _concurrent_statement: $ => choice(
      $.comment,
      $.generate_statement,
      $.block_statement,
      $.signal_assignment,
      $.process_statement,
      $.component_instantiation,
      $.assert_statement  // Concurrent assertion
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
      'for', $.identifier, 'in', $._range_or_expression, 'generate',
      choice(
        // VHDL-2008: declarations + begin + statements
        seq(
          repeat($._block_declarative_item),
          'begin',
          repeat($._concurrent_statement)
        ),
        // Pre-2008 or simple: just statements (no declarations)
        repeat($._concurrent_statement)
      ),
      'end', 'generate', optional($.identifier), ';'
    ),

    _if_generate: $ => seq(
      'if', $._expression, 'generate',
      choice(
        // VHDL-2008: declarations + begin + statements
        seq(
          repeat($._block_declarative_item),
          'begin',
          repeat($._concurrent_statement)
        ),
        // Pre-2008 or simple: just statements
        repeat($._concurrent_statement)
      ),
      optional($._generate_else),  // VHDL-2008
      'end', 'generate', optional($.identifier), ';'
    ),

    _generate_else: $ => seq(
      'else', 'generate',
      repeat($._concurrent_statement)
    ),

    _case_generate: $ => seq(
      'case', $._expression, 'generate',
      repeat($._case_generate_alternative),
      'end', 'generate', optional($.identifier), ';'
    ),

    _case_generate_alternative: $ => seq(
      'when', $._expression, '=>',
      repeat($._concurrent_statement)  // Label handled by process_statement
    ),

    // Range expression: 0 to 10, vec'range, etc.
    _range_or_expression: $ => choice(
      seq($._expression, choice('to', 'downto'), $._expression),  // Explicit range
      $._expression  // Attribute like vec'range
    ),

    // Block statement
    block_statement: $ => seq(
      field('label', $.identifier),
      ':',
      'block',
      optional(seq('(', $._expression, ')')),  // Guard condition
      optional('is'),
      repeat($._block_declarative_item),
      'begin',
      repeat($._concurrent_statement),
      'end',
      'block',
      optional($.identifier),
      ';'
    ),

    // Signal assignment - simple, conditional, or selected
    signal_assignment: $ => choice(
      $._simple_signal_assignment,
      $._conditional_signal_assignment,
      $._selected_signal_assignment,
      $._force_release_assignment
    ),

    _simple_signal_assignment: $ => seq(
      $._name,  // Can be indexed: data(i) <= '0'
      '<=',
      optional('guarded'),  // For guarded signal assignments in blocks
      $._expression,
      ';'
    ),

    // target <= expr when cond else expr when cond else expr;
    _conditional_signal_assignment: $ => seq(
      $.identifier,
      '<=',
      $._expression,
      'when',
      $._expression,
      repeat(seq('else', $._expression, optional(seq('when', $._expression)))),
      ';'
    ),

    // with selector select target <= value when choice, value when others;
    _selected_signal_assignment: $ => seq(
      'with',
      $._expression,
      'select',
      $.identifier,
      '<=',
      $._expression,
      'when',
      $._expression,
      repeat(seq(',', $._expression, 'when', $._expression)),
      ';'
    ),

    // VHDL-2008 force/release
    _force_release_assignment: $ => seq(
      $.identifier,
      '<=',
      choice('force', 'release'),
      optional($._expression),
      optional(seq('when', $._expression)),
      ';'
    ),

    process_statement: $ => seq(
      optional(seq($.identifier, ':')),  // Optional label
      'process',
      optional(seq('(', $._sensitivity_list, ')')),  // Sensitivity list
      optional('is'),
      repeat($._process_declarative_item),
      'begin',
      repeat($._sequential_statement),
      'end',
      'process',
      optional($.identifier),
      ';'
    ),

    // Sensitivity list: 'all' (VHDL-2008) or list of signal names
    _sensitivity_list: $ => choice(
      'all',  // VHDL-2008: sensitive to all signals read in process
      seq($.identifier, repeat(seq(',', $.identifier)))
    ),

    // Process declarative items - what can appear before 'begin' in a process
    _process_declarative_item: $ => choice(
      $.comment,
      $.variable_declaration,
      $.type_declaration,
      $.subtype_declaration,
      $.constant_declaration
    ),

    component_instantiation: $ => seq(
      field('label', $.identifier),
      ':',
      choice(
        // Direct entity instantiation: entity lib.entity(arch)
        seq(
          'entity',
          $.identifier,  // library
          '.',
          $.identifier,  // entity
          optional(seq('(', $.identifier, ')'))  // Optional architecture
        ),
        // Component instantiation: component_name
        field('component', $.identifier)
      ),
      optional(seq('generic', 'map', '(', $._association_list, ')')),
      optional(seq('port', 'map', '(', $._association_list, ')')),
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

    // Variable declarations inside subprograms
    _subprogram_declarative_item: $ => choice(
      $.comment,
      $.variable_declaration
    ),

    variable_declaration: $ => seq(
      'variable',
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
      'report',
      $._report_expression,  // Message string, can include concatenation
      optional(seq('severity', $.identifier)),
      ';'
    ),

    // Sequential signal assignment (inside process/function)
    // Use prec.dynamic for runtime conflict resolution with procedure_call_statement
    sequential_signal_assignment: $ => prec.dynamic(10, seq(
      $._name,
      '<=',
      $._expression,
      ';'
    )),

    exit_statement: $ => seq(
      'exit',
      optional($.identifier),  // Optional loop label
      optional(seq('when', $._expression)),
      ';'
    ),

    next_statement: $ => seq(
      'next',
      optional($.identifier),  // Optional loop label
      optional(seq('when', $._expression)),
      ';'
    ),

    // Case statement
    case_statement: $ => seq(
      'case', $._expression, 'is',
      repeat1($._case_alternative),
      'end', 'case', ';'
    ),

    _case_alternative: $ => seq(
      'when', $._expression, '=>',
      repeat($._sequential_statement)
    ),

    wait_statement: $ => seq(
      'wait',
      optional(choice(
        seq('until', $._expression),
        seq('for', $._expression),
        seq('on', $.identifier, repeat(seq(',', $.identifier)))
      )),
      ';'
    ),

    return_statement: $ => seq(
      'return',
      optional($._expression),
      ';'
    ),

    assignment_statement: $ => seq(
      $._name,  // Can be identifier or indexed/sliced name
      ':=',
      $._expression,
      ';'
    ),

    // Name can be simple identifier or indexed/sliced
    // For assignments: identifier or identifier(index_expression)
    _name: $ => choice(
      $.indexed_name,   // Indexed name: foo(i)
      $.external_name,  // VHDL-2008: << signal .path : type >>
      $.identifier      // Simple name
    ),

    // Indexed name for array access: foo(i), shift_reg(j+1)
    // Higher precedence to prefer this when followed by <= or :=
    indexed_name: $ => prec.dynamic(5, seq(
      $.identifier,
      '(',
      $._index_expression,
      ')'
    )),

    // Simple index expression - identifiers and operators, no nested parens for now
    _index_expression: $ => repeat1(choice(
      $.identifier,
      $.number,
      /[+\-*/<>=]+/
    )),

    // Simplified if statement
    if_statement: $ => seq(
      'if',
      $._expression,  // condition
      'then',
      repeat($._sequential_statement),
      repeat(seq(  // elsif clauses
        'elsif',
        $._expression,
        'then',
        repeat($._sequential_statement)
      )),
      optional(seq(
        'else',
        repeat($._sequential_statement)
      )),
      'end',
      'if',
      ';'
    ),

    // Simplified loop (while, for, or infinite)
    loop_statement: $ => seq(
      optional(choice(
        seq('while', $._expression),  // while loop
        seq('for', $.identifier, 'in', $._range_or_expression)  // for loop
      )),
      'loop',
      repeat($._sequential_statement),
      'end',
      'loop',
      ';'
    ),

    // Expression - structured to avoid consuming keywords
    // Uses repeat of terms to stop at keywords properly
    _expression: $ => prec(-1, repeat1($._expression_term)),

    // Single term in an expression - identifier, number, operator, or grouped expression
    _expression_term: $ => choice(
      $._literal,  // Must be early to catch '0' before it's seen as quote
      $.qualified_expression,  // type'(expr) - must be before _name_or_attribute
      $._name_or_attribute,  // Identifier optionally followed by attribute
      $.external_name,  // VHDL-2008: << signal .path : type >>
      $.number,
      /[+\-*/<>=&|?]+/,  // Symbolic operators (including ?= ?/= ?< etc.)
      choice('and', 'or', 'xor', 'nand', 'nor', 'xnor', 'not',  // Logical operators
             'mod', 'rem', 'abs', 'sll', 'srl', 'sla', 'sra', 'rol', 'ror'),  // Other operators
      $._parenthesized_expression  // Grouped or aggregate
    ),

    // Qualified expression: type'(expression)
    // e.g., string'("hello"), integer'(x + 1)
    qualified_expression: $ => seq(
      $.identifier,
      "'",
      '(',
      optional(seq($._aggregate_element, repeat(seq(',', $._aggregate_element)))),
      ')'
    ),

    // VHDL-2008: External name (hierarchical reference)
    // << signal .testbench.dut.sig : std_logic >>
    external_name: $ => seq(
      '<<',
      choice('signal', 'variable', 'constant'),
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
      seq('others', '=>', repeat1($._expression_term)),  // others => value
      seq($.identifier, '=>', repeat1($._expression_term)),  // named: x => 1
      seq($.number, '=>', repeat1($._expression_term)),  // positional with index
      repeat1($._expression_term)  // Just a value
    ),

    // Literals (character, string)
    _literal: _ => choice(
      /'[^']+'/,   // Character literal: '0', 'a', etc. (at least one char)
      /"[^"]*"/    // String literal: "text", "" (can be empty)
    ),

    // Name with optional attribute: identifier['attribute] or identifier'attr(arg)
    // Using token to ensure proper matching
    _name_or_attribute: $ => choice(
      token(seq(/[_a-zA-Z]+/, /[a-zA-Z0-9_]*/, "'", /[a-zA-Z]+/, '(', /[^)]*/, ')')),  // attr with arg: time'image(now)
      token(seq(/[_a-zA-Z]+/, /[a-zA-Z0-9_]*/, "'", /[a-zA-Z]+/)),  // identifier'attribute
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
      seq($._selected_name, ';'),                // Selected: obj.method;
      seq($._selected_name, '(', ')', ';'),      // Selected with parens: obj.method();
      seq($._selected_name, '(', $._procedure_argument, repeat(seq(',', $._procedure_argument)), ')', ';')
    )),

    // Selected name: a.b or a.b.c (for method calls, package references)
    _selected_name: $ => seq(
      $.identifier,
      repeat1(seq('.', $.identifier))
    ),

    // Procedure call with arguments - separate from indexed_name
    _procedure_call_with_args: $ => seq(
      $.identifier,
      '(',
      $._procedure_argument,
      repeat(seq(',', $._procedure_argument)),
      ')'
    ),

    // Procedure argument - simpler than expression to avoid conflicts
    _procedure_argument: $ => choice(
      $.qualified_expression,  // string'("hello")
      $.identifier,
      $.number,
      $._literal
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
