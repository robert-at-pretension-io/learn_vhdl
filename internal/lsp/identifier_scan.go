package lsp

import "strings"

type utf16Range struct {
	start int
	end   int
}

func findIdentifierRangesUTF16(lineText, target string) []utf16Range {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	targetRunes := []rune(target)
	if len(targetRunes) == 0 {
		return nil
	}
	lineRunes := []rune(lineText)
	var out []utf16Range
	for i := 0; i+len(targetRunes) <= len(lineRunes); i++ {
		if i > 0 && isVHDLIdentChar(lineRunes[i-1]) {
			continue
		}
		end := i + len(targetRunes)
		if end < len(lineRunes) && isVHDLIdentChar(lineRunes[end]) {
			continue
		}
		if !strings.EqualFold(string(lineRunes[i:end]), target) {
			continue
		}
		startCol := runeIndexToUTF16Column(lineRunes, i)
		endCol := runeIndexToUTF16Column(lineRunes, end)
		out = append(out, utf16Range{start: startCol, end: endCol})
	}
	return out
}
