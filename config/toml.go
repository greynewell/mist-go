// Package config provides TOML parsing and configuration loading with
// environment variable overrides. The TOML parser is implemented from
// scratch with zero external dependencies.
package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// ParseTOML reads TOML from r and returns a nested map. It supports
// comments, bare and quoted keys, string/int/float/bool values,
// arrays, and [table] / [table.sub] sections.
func ParseTOML(r io.Reader) (map[string]any, error) {
	root := make(map[string]any)
	current := root
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || line[0] == '#' {
			continue
		}

		if line[0] == '[' {
			if line[len(line)-1] != ']' {
				return nil, fmt.Errorf("line %d: unclosed table header", lineNum)
			}
			path := line[1 : len(line)-1]
			current = ensureTable(root, strings.Split(strings.TrimSpace(path), "."))
			continue
		}

		key, val, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}
		current[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading toml: %w", err)
	}

	return root, nil
}

func ensureTable(root map[string]any, parts []string) map[string]any {
	m := root
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, ok := m[p]
		if !ok {
			child := make(map[string]any)
			m[p] = child
			m = child
			continue
		}
		child, ok := v.(map[string]any)
		if !ok {
			child = make(map[string]any)
			m[p] = child
		}
		m = child
	}
	return m
}

func parseLine(line string, lineNum int) (string, any, error) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", nil, fmt.Errorf("line %d: expected key = value", lineNum)
	}

	key := strings.TrimSpace(line[:eq])
	key = unquoteKey(key)
	if key == "" {
		return "", nil, fmt.Errorf("line %d: empty key", lineNum)
	}

	raw := strings.TrimSpace(line[eq+1:])
	val, err := parseValue(raw, lineNum)
	if err != nil {
		return "", nil, err
	}

	return key, val, nil
}

func unquoteKey(k string) string {
	if len(k) >= 2 && k[0] == '"' && k[len(k)-1] == '"' {
		k = k[1 : len(k)-1]
	}
	return k
}

func parseValue(raw string, lineNum int) (any, error) {
	if raw == "" {
		return "", nil
	}

	raw = stripInlineComment(raw)

	if raw[0] == '"' {
		return parseQuotedString(raw, lineNum)
	}
	if raw[0] == '\'' {
		return parseLiteralString(raw, lineNum)
	}
	if raw[0] == '[' {
		return parseArray(raw, lineNum)
	}
	if raw == "true" {
		return true, nil
	}
	if raw == "false" {
		return false, nil
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f, nil
	}

	return raw, nil
}

func stripInlineComment(s string) string {
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(s); i++ {
		if !inQuote && (s[i] == '"' || s[i] == '\'') {
			inQuote = true
			quoteChar = s[i]
			continue
		}
		if inQuote && s[i] == quoteChar {
			inQuote = false
			continue
		}
		if !inQuote && s[i] == '#' {
			return strings.TrimRightFunc(s[:i], unicode.IsSpace)
		}
	}
	return s
}

func parseQuotedString(raw string, lineNum int) (string, error) {
	if len(raw) < 2 || raw[len(raw)-1] != '"' {
		return "", fmt.Errorf("line %d: unterminated string", lineNum)
	}
	s := raw[1 : len(raw)-1]
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	return s, nil
}

func parseLiteralString(raw string, lineNum int) (string, error) {
	if len(raw) < 2 || raw[len(raw)-1] != '\'' {
		return "", fmt.Errorf("line %d: unterminated literal string", lineNum)
	}
	return raw[1 : len(raw)-1], nil
}

func parseArray(raw string, lineNum int) ([]any, error) {
	if raw[len(raw)-1] != ']' {
		return nil, fmt.Errorf("line %d: unterminated array", lineNum)
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return []any{}, nil
	}

	var result []any
	for _, elem := range splitArrayElements(inner) {
		elem = strings.TrimSpace(elem)
		if elem == "" {
			continue
		}
		val, err := parseValue(elem, lineNum)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func splitArrayElements(s string) []string {
	var parts []string
	depth := 0
	inQuote := false
	quoteChar := byte(0)
	start := 0

	for i := 0; i < len(s); i++ {
		if !inQuote && (s[i] == '"' || s[i] == '\'') {
			inQuote = true
			quoteChar = s[i]
			continue
		}
		if inQuote && s[i] == quoteChar {
			inQuote = false
			continue
		}
		if !inQuote {
			switch s[i] {
			case '[':
				depth++
			case ']':
				depth--
			case ',':
				if depth == 0 {
					parts = append(parts, s[start:i])
					start = i + 1
				}
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
