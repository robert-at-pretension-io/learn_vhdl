// ============================================================================
// Tree-sitter Grammar for VHDL
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
  // For now, we have no conflicts. Add them as needed when you see errors
  // like "Unresolved conflict for symbol sequence..."
  // ===========================================================================
  // conflicts: $ => [],

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
      // As you learn VHDL, add more design units here:
      // $.library_clause,
      // $.use_clause,
      // $.entity_declaration,
      // $.architecture_body,
      // $.package_declaration,
      // $.package_body,
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

    // =========================================================================
    // YOUR LEARNING PATH
    // =========================================================================
    //
    // Start simple and build up. Each time you add a rule:
    // 1. Run `./dev.sh` to regenerate the parser and test
    // 2. Look at the errors - they tell you what's not yet recognized
    // 3. Add the next rule to handle those errors
    //
    // SUGGESTED ORDER:
    //
    // Step 1: Identifiers (names of things)
    // -------------------------------------
    // identifier: $ => /[a-zA-Z_][a-zA-Z0-9_]*/,
    //
    // Note: VHDL is case-insensitive, but we store identifiers as-is.
    // Case-insensitivity is handled later in semantic analysis.
    //
    //
    // Step 2: Basic literals (values)
    // -------------------------------
    // integer_literal: $ => /\d+/,
    // string_literal: $ => /"[^"]*"/,
    // character_literal: $ => /'[^']'/,
    //
    //
    // Step 3: Library and Use clauses
    // -------------------------------
    // Almost every VHDL file starts with these:
    //   library ieee;
    //   use ieee.std_logic_1164.all;
    //
    // library_clause: $ => seq(
    //   'library',
    //   $.identifier,
    //   ';'
    // ),
    //
    //
    // Step 4: Entity declaration
    // --------------------------
    // The "interface" of a hardware module:
    //   entity foo is
    //     port (...);
    //   end entity foo;
    //
    //
    // Step 5: Architecture body
    // -------------------------
    // The "implementation" of a hardware module:
    //   architecture rtl of foo is
    //   begin
    //     ...
    //   end architecture rtl;
    //
    //
    // Step 6: Expressions with precedence
    // -----------------------------------
    // This is where prec.left() and prec.right() become important.
    // Operators have different precedence levels:
    //   - ** (exponentiation) binds tightest
    //   - * / mod rem
    //   - + - &
    //   - = /= < <= > >=
    //   - and or nand nor xor xnor bind loosest
    //
    // =========================================================================
  }
});
