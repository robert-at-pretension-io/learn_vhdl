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
    INVALID_BIT_STRING_LITERAL,
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

static bool is_base_specifier(int32_t c) {
    return c == 'B' || c == 'b' ||
           c == 'O' || c == 'o' ||
           c == 'X' || c == 'x' ||
           c == 'D' || c == 'd';
}

static bool is_signedness(int32_t c) {
    return c == 'S' || c == 's' ||
           c == 'U' || c == 'u';
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
    // Only try to match if a bit-string token is valid at this position
    if (!valid_symbols[BIT_STRING_LITERAL] && !valid_symbols[INVALID_BIT_STRING_LITERAL]) {
        return false;
    }

    // Skip whitespace (Tree-sitter extras handle this, but be safe)
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t' ||
           lexer->lookahead == '\n' || lexer->lookahead == '\r') {
        lexer->advance(lexer, true);  // true = skip
    }

    // Check for bit string prefix with optional size and signedness
    int32_t prefix = lexer->lookahead;
    bool (*digit_check)(int32_t) = NULL;
    bool saw_size = false;
    bool saw_sign = false;

    if (prefix >= '0' && prefix <= '9') {
        // Sized bit string literal: <size>[s|u]<base>"..."
        while (is_decimal_digit(lexer->lookahead)) {
            lexer->advance(lexer, false);
        }
        saw_size = true;
        if (lexer->lookahead == 's' || lexer->lookahead == 'S' ||
            lexer->lookahead == 'u' || lexer->lookahead == 'U') {
            lexer->advance(lexer, false);
            saw_sign = true;
        }
        prefix = lexer->lookahead;
    } else if (lexer->lookahead == 's' || lexer->lookahead == 'S' ||
               lexer->lookahead == 'u' || lexer->lookahead == 'U') {
        // Unsized signedness prefix: sX"..." or uB"..."
        lexer->advance(lexer, false);
        saw_sign = true;
        prefix = lexer->lookahead;
    }

    if (!iswalpha(prefix)) {
        return false;
    }

    // Consume the first prefix letter
    lexer->advance(lexer, false);

    int32_t prefix2 = 0;
    if (iswalpha(lexer->lookahead)) {
        prefix2 = lexer->lookahead;
        lexer->advance(lexer, false);
    }

    // Must be followed by opening quote or percent delimiter
    int32_t delimiter = lexer->lookahead;
    if (delimiter != '"' && delimiter != '%') {
        return false;  // Not a bit string literal, let normal lexer handle
    }

    bool valid = false;
    int32_t base = 0;

    if (saw_sign) {
        // Signedness already consumed: next must be a single base letter
        if (is_base_specifier(prefix) && prefix2 == 0) {
            valid = true;
            base = prefix;
        }
    } else {
        if (prefix2 == 0) {
            if (is_base_specifier(prefix)) {
                valid = true;
                base = prefix;
            }
        } else if (!saw_size && is_signedness(prefix) && is_base_specifier(prefix2)) {
            // Unsized signedness prefix: sX"..." or uB"..."
            valid = true;
            base = prefix2;
        }
    }

    if (valid) {
        if (!valid_symbols[BIT_STRING_LITERAL]) {
            return false;
        }
        if (base == 'X' || base == 'x') {
            digit_check = is_hex_digit;
        } else if (base == 'B' || base == 'b') {
            digit_check = is_binary_digit;
        } else if (base == 'O' || base == 'o') {
            digit_check = is_octal_digit;
        } else if (base == 'D' || base == 'd') {
            digit_check = is_decimal_digit;
        }
    } else if (!valid_symbols[INVALID_BIT_STRING_LITERAL]) {
        return false;
    }

    // Mark the end of the token so far (the prefix)
    // This is important for error recovery
    lexer->mark_end(lexer);

    // Consume the opening delimiter
    lexer->advance(lexer, false);

    // Consume digits until closing delimiter
    while (lexer->lookahead != delimiter && lexer->lookahead != 0) {
        if (digit_check != NULL && !digit_check(lexer->lookahead)) {
            // Invalid digit for this base - still consume to closing quote
            // This allows for better error recovery
        }
        lexer->advance(lexer, false);
    }

    // Must end with closing delimiter
    if (lexer->lookahead != delimiter) {
        return false;  // Unterminated string
    }

    // Consume the closing quote
    lexer->advance(lexer, false);

    // Mark final position
    lexer->mark_end(lexer);

    // Set the token type
    lexer->result_symbol = valid ? BIT_STRING_LITERAL : INVALID_BIT_STRING_LITERAL;

    return true;  // Successfully matched!
}
