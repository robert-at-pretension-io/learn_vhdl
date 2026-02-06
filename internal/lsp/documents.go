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

	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}

	lineText := lines[line]
	if character < 0 || character >= len(lineText) {
		// If character is at end of line, back up one
		if character > 0 && character == len(lineText) {
			character--
		} else {
			return ""
		}
	}

	// Find word boundaries — VHDL identifiers are [a-zA-Z_][a-zA-Z0-9_]*
	start := character
	for start > 0 && isVHDLIdentChar(rune(lineText[start-1])) {
		start--
	}
	end := character
	for end < len(lineText) && isVHDLIdentChar(rune(lineText[end])) {
		end++
	}

	if start == end {
		return ""
	}

	word := lineText[start:end]
	// Verify it starts with a letter or underscore
	if len(word) > 0 && !unicode.IsLetter(rune(word[0])) && word[0] != '_' {
		return ""
	}
	return word
}

func isVHDLIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
