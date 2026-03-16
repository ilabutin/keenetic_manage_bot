package router

import (
	"fmt"
	"os"
	"strings"
)

// AddToRoutingRule appends entry to the "domain" array of the routing rule
// identified by outboundTag in the JSONC routing file. Comments are preserved.
func AddToRoutingRule(filePath, outboundTag, entry string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}
	updated, err := insertDomainEntry(string(data), outboundTag, entry)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(updated), 0644)
}

// RemoveFromRoutingRule removes entry from the "domain" array of the rule
// identified by outboundTag. Comments are preserved.
func RemoveFromRoutingRule(filePath, outboundTag, entry string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}
	updated, err := removeDomainEntry(string(data), outboundTag, entry)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, []byte(updated), 0644)
}

// ReadDomainEntries returns all entries in the "domain" array of the rule
// identified by outboundTag.
func ReadDomainEntries(filePath, outboundTag string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("чтение файла: %w", err)
	}
	content := string(data)
	arrOpen, arrClose, err := findDomainArray(content, outboundTag)
	if err != nil {
		return nil, err
	}
	return extractStringEntries(content, arrOpen, arrClose), nil
}

func insertDomainEntry(content, outboundTag, entry string) (string, error) {
	arrOpen, arrClose, err := findDomainArray(content, outboundTag)
	if err != nil {
		return "", err
	}

	if strings.Contains(content[arrOpen+1:arrClose], `"`+entry+`"`) {
		return "", fmt.Errorf("%q уже есть в правиле %q", entry, outboundTag)
	}

	indent := extractArrayIndent(content, arrOpen, arrClose)
	beforeClose := strings.TrimRight(content[arrOpen+1:arrClose], " \t\n\r")
	insertAt := arrOpen + 1 + len(beforeClose)

	return content[:insertAt] + ",\n" + indent + `"` + entry + `"` + content[insertAt:], nil
}

func removeDomainEntry(content, outboundTag, entry string) (string, error) {
	arrOpen, arrClose, err := findDomainArray(content, outboundTag)
	if err != nil {
		return "", err
	}

	entries := extractStringEntries(content, arrOpen, arrClose)
	newEntries := make([]string, 0, len(entries))
	found := false
	for _, e := range entries {
		if e == entry {
			found = true
		} else {
			newEntries = append(newEntries, e)
		}
	}
	if !found {
		return "", fmt.Errorf("%q не найдено в правиле %q", entry, outboundTag)
	}

	indent := extractArrayIndent(content, arrOpen, arrClose)
	closingIndent := extractClosingIndent(content, arrClose)
	newInside := reconstructArray(newEntries, indent, closingIndent)
	return content[:arrOpen+1] + newInside + content[arrClose:], nil
}

// findDomainArray finds the '[' and matching ']' of the "domain" array in the
// rule identified by outboundTag. Returns their positions in content.
func findDomainArray(content, outboundTag string) (arrOpen, arrClose int, err error) {
	searchStr := `"outboundTag": "` + outboundTag + `"`
	pos := 0
	for {
		tagPos := findNextOutside(content, searchStr, pos)
		if tagPos < 0 {
			break
		}
		ruleStart := findEnclosingBrace(content, tagPos)
		if ruleStart < 0 {
			pos = tagPos + 1
			continue
		}
		ruleEnd := findMatchingClose(content, ruleStart)
		if ruleEnd < 0 {
			pos = tagPos + 1
			continue
		}
		domainPos := findNextOutside(content, `"domain"`, ruleStart)
		if domainPos < 0 || domainPos >= ruleEnd {
			pos = tagPos + 1
			continue // rule exists but has no domain array; try next occurrence
		}
		ao := findNextOutside(content, "[", domainPos+len(`"domain"`))
		if ao < 0 || ao >= ruleEnd {
			pos = tagPos + 1
			continue
		}
		ac := findMatchingClose(content, ao)
		if ac < 0 {
			return 0, 0, fmt.Errorf("нарушена структура массива domain в правиле %q", outboundTag)
		}
		return ao, ac, nil
	}
	return 0, 0, fmt.Errorf("правило outboundTag=%q с массивом domain не найдено", outboundTag)
}

// extractStringEntries extracts all quoted string values from content[arrOpen+1:arrClose].
func extractStringEntries(content string, arrOpen, arrClose int) []string {
	var entries []string
	i := arrOpen + 1
	for i < arrClose {
		if content[i] == '"' {
			end := skipString(content, i)
			entries = append(entries, content[i+1:end-1])
			i = end
		} else {
			i++
		}
	}
	return entries
}

// extractArrayIndent returns the leading whitespace used for entries inside the array.
func extractArrayIndent(content string, arrOpen, arrClose int) string {
	inside := content[arrOpen+1 : arrClose]
	nl := strings.Index(inside, "\n")
	if nl < 0 {
		return "        "
	}
	line := inside[nl+1:]
	indent := ""
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			break
		}
	}
	if indent == "" {
		return "        "
	}
	return indent
}

// extractClosingIndent returns the whitespace between the last newline and ']'.
func extractClosingIndent(content string, closePos int) string {
	i := closePos - 1
	for i >= 0 && content[i] != '\n' {
		i--
	}
	if i < 0 {
		return ""
	}
	return content[i+1 : closePos]
}

// reconstructArray rebuilds array content (between '[' and ']') from entries.
func reconstructArray(entries []string, entryIndent, closingIndent string) string {
	if len(entries) == 0 {
		return "\n" + closingIndent
	}
	var sb strings.Builder
	for i, e := range entries {
		sb.WriteString("\n" + entryIndent + `"` + e + `"`)
		if i < len(entries)-1 {
			sb.WriteString(",")
		}
	}
	sb.WriteString("\n" + closingIndent)
	return sb.String()
}

// findNextOutside returns the index of substr in content[from:] that is outside
// string literals and JSONC comments, or -1 if not found.
func findNextOutside(content, substr string, from int) int {
	n := len(substr)
	for i := from; i+n <= len(content); {
		// Check for match before deciding to skip — search strings often start with '"'.
		if content[i:i+n] == substr {
			return i
		}
		switch {
		case content[i] == '"':
			i = skipString(content, i)
		case i+1 < len(content) && content[i] == '/' && content[i+1] == '/':
			i = skipLineComment(content, i)
		case i+1 < len(content) && content[i] == '/' && content[i+1] == '*':
			i = skipBlockComment(content, i)
		default:
			i++
		}
	}
	return -1
}

// findMatchingClose finds the closing ']' or '}' for the open bracket at content[openPos].
func findMatchingClose(content string, openPos int) int {
	open := content[openPos]
	var close byte
	switch open {
	case '[':
		close = ']'
	case '{':
		close = '}'
	default:
		return -1
	}
	depth := 0
	i := openPos
	for i < len(content) {
		switch {
		case content[i] == '"':
			i = skipString(content, i)
		case i+1 < len(content) && content[i] == '/' && content[i+1] == '/':
			i = skipLineComment(content, i)
		case i+1 < len(content) && content[i] == '/' && content[i+1] == '*':
			i = skipBlockComment(content, i)
		case content[i] == open:
			depth++
			i++
		case content[i] == close:
			depth--
			if depth == 0 {
				return i
			}
			i++
		default:
			i++
		}
	}
	return -1
}

// findEnclosingBrace scans backwards from pos to find the '{' that directly
// encloses it. Simplified: assumes routing.json rule values don't contain braces.
func findEnclosingBrace(content string, pos int) int {
	depth := 0
	for i := pos - 1; i >= 0; i-- {
		switch content[i] {
		case '}':
			depth++
		case '{':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

func skipString(content string, from int) int {
	i := from + 1
	for i < len(content) {
		if content[i] == '\\' {
			i += 2
			continue
		}
		if content[i] == '"' {
			return i + 1
		}
		i++
	}
	return len(content)
}

func skipLineComment(content string, from int) int {
	i := from + 2
	for i < len(content) && content[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(content string, from int) int {
	i := from + 2
	for i+1 < len(content) {
		if content[i] == '*' && content[i+1] == '/' {
			return i + 2
		}
		i++
	}
	return len(content)
}
