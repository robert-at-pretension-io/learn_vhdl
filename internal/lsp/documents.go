package lsp

import (
	"strings"
	"sync"
	"unicode"
)

// DocumentStore tracks the contents of open text documents.
type DocumentStore struct {
	mu   sync.RWMutex
	docs map[string]string // URI -> content
}

// NewDocumentStore creates an empty document store.
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		docs: make(map[string]string),
	}
}

// Set stores the content for the given URI.
func (ds *DocumentStore) Set(uri, content string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.docs[uri] = content
}

// Get returns the content for the given URI, or empty string if not tracked.
func (ds *DocumentStore) Get(uri string) (string, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	content, ok := ds.docs[uri]
	return content, ok
}

// Delete removes the document from tracking.
func (ds *DocumentStore) Delete(uri string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.docs, uri)
}

// WordAtPosition extracts the VHDL identifier at the given line and character.
// Returns empty string if no word is found.
func (ds *DocumentStore) WordAtPosition(uri string, line, character int) string {
	ds.mu.RLock()
	content, ok := ds.docs[uri]
	ds.mu.RUnlock()
	if !ok {
		return ""
	}
	word, _, _, ok := wordRangeAtPosition(content, line, character)
	if !ok {
		return ""
	}
	return word
}

// WordRangeAtPosition returns the identifier and UTF-16 start/end columns.
func (ds *DocumentStore) WordRangeAtPosition(uri string, line, character int) (string, int, int, bool) {
	ds.mu.RLock()
	content, ok := ds.docs[uri]
	ds.mu.RUnlock()
	if !ok {
		return "", 0, 0, false
	}
	return wordRangeAtPosition(content, line, character)
}

// PrefixAtPosition returns identifier prefix immediately before the cursor.
func (ds *DocumentStore) PrefixAtPosition(uri string, line, character int) string {
	ds.mu.RLock()
	content, ok := ds.docs[uri]
	ds.mu.RUnlock()
	if !ok {
		return ""
	}
	return prefixAtPosition(content, line, character)
}

func wordRangeAtPosition(content string, line, character int) (string, int, int, bool) {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return "", 0, 0, false
	}

	lineText := lines[line]
	runes := []rune(lineText)
	if character < 0 {
		return "", 0, 0, false
	}
	runeIndex, ok := utf16ColumnToRuneIndex(runes, character)
	if !ok {
		return "", 0, 0, false
	}
	// If character is at end of line, back up one
	if runeIndex == len(runes) {
		if runeIndex > 0 {
			runeIndex--
		} else {
			return "", 0, 0, false
		}
	}
	// If on non-identifier character, back up to preceding character if it is an identifier.
	if !isVHDLIdentChar(runes[runeIndex]) {
		if runeIndex > 0 && isVHDLIdentChar(runes[runeIndex-1]) {
			runeIndex--
		} else {
			return "", 0, 0, false
		}
	}

	// Find word boundaries — VHDL identifiers are [a-zA-Z_][a-zA-Z0-9_]*
	start := runeIndex
	for start > 0 && isVHDLIdentChar(runes[start-1]) {
		start--
	}
	end := runeIndex
	for end < len(runes) && isVHDLIdentChar(runes[end]) {
		end++
	}

	if start == end {
		return "", 0, 0, false
	}

	wordRunes := runes[start:end]
	// Verify it starts with a letter or underscore
	if len(wordRunes) > 0 && !unicode.IsLetter(wordRunes[0]) && wordRunes[0] != '_' {
		return "", 0, 0, false
	}
	startCol := runeIndexToUTF16Column(runes, start)
	endCol := runeIndexToUTF16Column(runes, end)
	return string(wordRunes), startCol, endCol, true
}

func prefixAtPosition(content string, line, character int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) || character < 0 {
		return ""
	}
	runes := []rune(lines[line])
	runeIndex, ok := utf16ColumnToRuneIndex(runes, character)
	if !ok {
		return ""
	}
	start := runeIndex
	for start > 0 && isVHDLIdentChar(runes[start-1]) {
		start--
	}
	if start == runeIndex {
		return ""
	}
	wordRunes := runes[start:runeIndex]
	if len(wordRunes) > 0 && !unicode.IsLetter(wordRunes[0]) && wordRunes[0] != '_' {
		return ""
	}
	return string(wordRunes)
}

func isVHDLIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func utf16ColumnToRuneIndex(runes []rune, column int) (int, bool) {
	if column < 0 {
		return 0, false
	}
	units := 0
	for i, r := range runes {
		if units == column {
			return i, true
		}
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		// If the client points into a surrogate pair, snap to this rune.
		if column < units+width {
			return i, true
		}
		units += width
	}
	if units == column {
		return len(runes), true
	}
	return 0, false
}

func runeIndexToUTF16Column(runes []rune, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}
	if runeIndex > len(runes) {
		runeIndex = len(runes)
	}
	units := 0
	for i := 0; i < runeIndex; i++ {
		if runes[i] > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return units
}
