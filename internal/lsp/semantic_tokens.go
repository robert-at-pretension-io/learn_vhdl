package lsp

import (
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

var semanticTokenTypes = []string{
	string(protocol.SemanticTokenTypeKeyword),
	string(protocol.SemanticTokenTypeVariable),
	string(protocol.SemanticTokenTypeFunction),
	string(protocol.SemanticTokenTypeClass),
	string(protocol.SemanticTokenTypeInterface),
	string(protocol.SemanticTokenTypeType),
	string(protocol.SemanticTokenTypeNumber),
	string(protocol.SemanticTokenTypeString),
	string(protocol.SemanticTokenTypeComment),
	string(protocol.SemanticTokenTypeOperator),
}

var semanticTokenTypeIndex = map[string]uint32{
	"keyword":   0,
	"variable":  1,
	"function":  2,
	"class":     3,
	"interface": 4,
	"type":      5,
	"number":    6,
	"string":    7,
	"comment":   8,
	"operator":  9,
}

func semanticTokensLegend() protocol.SemanticTokensLegend {
	return protocol.SemanticTokensLegend{
		TokenTypes:     semanticTokenTypes,
		TokenModifiers: []string{},
	}
}

func (s *Server) textDocumentSemanticTokensFull(_ *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	uri := params.TextDocument.URI
	text, ok := s.documentStore.Get(uri)
	if !ok {
		b, err := os.ReadFile(uriToFile(uri))
		if err != nil {
			return &protocol.SemanticTokens{Data: []protocol.UInteger{}}, nil
		}
		text = string(b)
	}

	kindByName := buildPrimarySymbolKindMap(s.symbolStore.AllEntries())
	tokens := lexSemanticTokens(text, kindByName)
	data := encodeSemanticTokens(tokens)
	return &protocol.SemanticTokens{Data: data}, nil
}

type semanticToken struct {
	line     int
	startCol int
	length   int
	typ      uint32
}

func lexSemanticTokens(text string, kindByName map[string]string) []semanticToken {
	lines := strings.Split(text, "\n")
	tokens := make([]semanticToken, 0, 256)
	for lineNo, lineText := range lines {
		runes := []rune(lineText)
		i := 0
		for i < len(runes) {
			r := runes[i]
			if unicode.IsSpace(r) {
				i++
				continue
			}
			if r == '-' && i+1 < len(runes) && runes[i+1] == '-' {
				tokens = append(tokens, newSemanticToken(lineNo, runes, i, len(runes)-i, "comment"))
				break
			}
			if r == '"' {
				start := i
				i++
				for i < len(runes) {
					if runes[i] == '"' {
						i++
						break
					}
					i++
				}
				tokens = append(tokens, newSemanticToken(lineNo, runes, start, i-start, "string"))
				continue
			}
			if unicode.IsDigit(r) {
				start := i
				i++
				for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '_' || runes[i] == '.') {
					i++
				}
				tokens = append(tokens, newSemanticToken(lineNo, runes, start, i-start, "number"))
				continue
			}
			if unicode.IsLetter(r) || r == '_' {
				start := i
				i++
				for i < len(runes) && isVHDLIdentChar(runes[i]) {
					i++
				}
				word := string(runes[start:i])
				typ := semanticTypeForWord(word, kindByName)
				tokens = append(tokens, newSemanticToken(lineNo, runes, start, i-start, typ))
				continue
			}
			if isOperatorRune(r) {
				tokens = append(tokens, newSemanticToken(lineNo, runes, i, 1, "operator"))
			}
			i++
		}
	}
	return tokens
}

func newSemanticToken(line int, lineRunes []rune, startRune int, lengthRunes int, typ string) semanticToken {
	startCol := runeIndexToUTF16Column(lineRunes, startRune)
	length := runeIndexToUTF16Column(lineRunes[startRune:startRune+lengthRunes], lengthRunes)
	return semanticToken{
		line:     line,
		startCol: startCol,
		length:   length,
		typ:      semanticTokenTypeIndex[typ],
	}
}

func semanticTypeForWord(word string, kindByName map[string]string) string {
	lower := strings.ToLower(word)
	if isVHDLKeyword(lower) {
		return "keyword"
	}
	switch kindByName[lower] {
	case "function", "procedure":
		return "function"
	case "entity":
		return "class"
	case "component":
		return "interface"
	case "type":
		return "type"
	case "package", "architecture":
		return "class"
	default:
		return "variable"
	}
}

func buildPrimarySymbolKindMap(entries []symbolEntry) map[string]string {
	priority := map[string]int{
		"entity": 1, "architecture": 2, "package": 3, "component": 4,
		"type": 5, "function": 6, "procedure": 7, "constant": 8, "signal": 9, "port": 10, "instance": 11,
	}
	kinds := make(map[string]string)
	for _, e := range entries {
		key := strings.ToLower(e.name)
		cur, ok := kinds[key]
		if !ok || priority[e.kind] < priority[cur] {
			kinds[key] = e.kind
		}
	}
	return kinds
}

func encodeSemanticTokens(tokens []semanticToken) []protocol.UInteger {
	if len(tokens) == 0 {
		return []protocol.UInteger{}
	}
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		return tokens[i].startCol < tokens[j].startCol
	})
	data := make([]protocol.UInteger, 0, len(tokens)*5)
	prevLine := 0
	prevStart := 0
	for i, tok := range tokens {
		deltaLine := tok.line - prevLine
		deltaStart := tok.startCol
		if i > 0 && deltaLine == 0 {
			deltaStart = tok.startCol - prevStart
		}
		data = append(data,
			protocol.UInteger(deltaLine),
			protocol.UInteger(deltaStart),
			protocol.UInteger(tok.length),
			protocol.UInteger(tok.typ),
			0, // modifiers bitset
		)
		prevLine = tok.line
		prevStart = tok.startCol
	}
	return data
}

func isVHDLKeyword(word string) bool {
	for _, kw := range vhdlKeywords {
		if kw == word {
			return true
		}
	}
	return false
}

func isOperatorRune(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '=', '<', '>', '&', ':':
		return true
	default:
		return false
	}
}
