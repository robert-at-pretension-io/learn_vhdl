// Your VHDL grammar - fill this in as you learn!
// Run `npm run build` or `tree-sitter generate` after changes

module.exports = grammar({
  name: 'vhdl',

  rules: {
    // TODO: Start here! Define your first rule.
    // The first rule is the entry point.
    source_file: $ => repeat($._definition),

    _definition: $ => choice(
      $.comment,
      // Add more as you learn: $.entity_declaration, $.architecture_body, etc.
    ),

    comment: $ => token(seq('--', /.*/)),
  }
});
