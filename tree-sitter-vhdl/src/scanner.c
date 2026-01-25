/**
 * External Scanner for VHDL Tree-sitter Grammar
 * ==============================================
 *
 * This file handles tokenization that can't be expressed cleanly in grammar.js.
 * The main use case: bit string literals like X"DEADBEEF", B"1010", O"777"
 *
 * WHY WE NEED THIS:
 * -----------------
 * Tree-sitter's lexer tokenizes greedily. When it sees "X" it matches as identifier
 * before it can consider X"..." as a bit string literal. The grammar.js prec()
 * function doesn't help here because it affects parsing, not lexing.
 *
 * External scanners run BEFORE the normal lexer, giving us first crack at the input.
 * We use this to recognize bit string literals before "X" gets grabbed as identifier.
 *
 * HOW IT WORKS:
 * -------------
 * 1. Grammar declares external tokens: externals: $ => [$.bit_string_literal]
 * 2. Tree-sitter calls tree_sitter_vhdl_external_scanner_scan() for each token
 * 3. We check if current position starts with X", B", O" (case-insensitive)
 * 4. If yes, we consume the entire literal and return true
 * 5. If no, we return false and Tree-sitter uses normal lexer
 */

#include "tree_sitter/parser.h"
#include <wctype.h>
#include <string.h>

// Token types we handle - must match order in grammar.js externals array
enum TokenType {
    BIT_STRING_LITERAL,
};

/**
 * Create scanner state (called once per parser instance)
 */
void *tree_sitter_vhdl_external_scanner_create(void) {
    return NULL;  // No state needed
}

/**
 * Destroy scanner state
 */
void tree_sitter_vhdl_external_scanner_destroy(void *payload) {
    // Nothing to free
}

/**
 * Serialize scanner state (for incremental parsing)
 */
unsigned tree_sitter_vhdl_external_scanner_serialize(void *payload, char *buffer) {
    return 0;  // No state to serialize
}

/**
 * Deserialize scanner state
 */
void tree_sitter_vhdl_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
    // Nothing to deserialize
}

/**
 * Check if character is a valid hex digit
 */
static bool is_hex_digit(int32_t c) {
    return (c >= '0' && c <= '9') ||
           (c >= 'a' && c <= 'f') ||
           (c >= 'A' && c <= 'F') ||
           c == '_';  // VHDL allows underscores in literals
}

/**
 * Check if character is a valid binary digit
 */
static bool is_binary_digit(int32_t c) {
    return c == '0' || c == '1' || c == '_';
}

/**
 * Check if character is a valid octal digit
 */
static bool is_octal_digit(int32_t c) {
    return (c >= '0' && c <= '7') || c == '_';
}

/**
 * Check if character is a valid decimal digit
 */
static bool is_decimal_digit(int32_t c) {
    return (c >= '0' && c <= '9') || c == '_';
}

/**
 * Main scanning function - called by Tree-sitter for each token
 *
 * @param payload   Our scanner state (unused)
 * @param lexer     Tree-sitter lexer interface
 * @param valid_symbols  Which tokens are valid at current position
 * @return true if we matched a token, false to let normal lexer try
 */
bool tree_sitter_vhdl_external_scanner_scan(
    void *payload,
    TSLexer *lexer,
    const bool *valid_symbols
) {
    // Only try to match if bit_string_literal is valid at this position
    if (!valid_symbols[BIT_STRING_LITERAL]) {
        return false;
    }

    // Skip whitespace (Tree-sitter extras handle this, but be safe)
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t' ||
           lexer->lookahead == '\n' || lexer->lookahead == '\r') {
        lexer->advance(lexer, true);  // true = skip
    }

    // Check for bit string prefix: X, x, B, b, O, o
    int32_t prefix = lexer->lookahead;
    bool (*digit_check)(int32_t) = NULL;

    if (prefix >= '0' && prefix <= '9') {
        // Sized bit string literal: <size>[s|u]<base>"..."
        while (is_decimal_digit(lexer->lookahead)) {
            lexer->advance(lexer, false);
        }
        if (lexer->lookahead == 's' || lexer->lookahead == 'S' ||
            lexer->lookahead == 'u' || lexer->lookahead == 'U') {
            lexer->advance(lexer, false);
        }
        prefix = lexer->lookahead;
    }

    if (prefix == 'X' || prefix == 'x') {
        digit_check = is_hex_digit;
    } else if (prefix == 'B' || prefix == 'b') {
        digit_check = is_binary_digit;
    } else if (prefix == 'O' || prefix == 'o') {
        digit_check = is_octal_digit;
    } else if (prefix == 'D' || prefix == 'd') {
        digit_check = is_decimal_digit;
    } else {
        return false;  // Not a bit string literal
    }

    // Consume the prefix
    lexer->advance(lexer, false);

    // Must be followed by opening quote
    if (lexer->lookahead != '"') {
        return false;  // Not a bit string literal, let normal lexer handle
    }

    // Mark the end of the token so far (the prefix)
    // This is important for error recovery
    lexer->mark_end(lexer);

    // Consume the opening quote
    lexer->advance(lexer, false);

    // Consume digits until closing quote
    while (lexer->lookahead != '"' && lexer->lookahead != 0) {
        if (!digit_check(lexer->lookahead)) {
            // Invalid digit for this base - still consume to closing quote
            // This allows for better error recovery
        }
        lexer->advance(lexer, false);
    }

    // Must end with closing quote
    if (lexer->lookahead != '"') {
        return false;  // Unterminated string
    }

    // Consume the closing quote
    lexer->advance(lexer, false);

    // Mark final position
    lexer->mark_end(lexer);

    // Set the token type
    lexer->result_symbol = BIT_STRING_LITERAL;

    return true;  // Successfully matched!
}
