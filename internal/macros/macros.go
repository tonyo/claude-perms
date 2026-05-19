package macros

import (
	"fmt"
	"strings"

	"github.com/tonyo/claude-perms/internal/expand"
)

// Validate checks every macro value for syntax errors and rejects recursive references.
func Validate(macros map[string]string) error {
	for name, value := range macros {
		if strings.Contains(value, "{{") {
			return fmt.Errorf("macro %q: recursive macro references are not supported", name)
		}
		if _, err := expand.Expand("(" + value + ")"); err != nil {
			return fmt.Errorf("macro %q: %w", name, err)
		}
	}
	return nil
}

// Interpolate replaces all {{name}} occurrences in pattern with their macro values.
func Interpolate(pattern string, macros map[string]string) (string, error) {
	result := pattern
	start := 0
	for {
		open := strings.Index(result[start:], "{{")
		if open == -1 {
			break
		}
		open += start
		close := strings.Index(result[open:], "}}")
		if close == -1 {
			return "", fmt.Errorf("pattern has unclosed '{{' at position %d", open)
		}
		close += open
		name := result[open+2 : close]
		val, ok := macros[name]
		if !ok {
			return "", fmt.Errorf("pattern references undefined macro %q", name)
		}
		result = result[:open] + val + result[close+2:]
		start = open + len(val)
	}
	return result, nil
}

// InterpolateAll applies Interpolate to each pattern in the slice.
func InterpolateAll(patterns []string, macros map[string]string) ([]string, error) {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		interpolated, err := Interpolate(p, macros)
		if err != nil {
			return nil, err
		}
		out[i] = interpolated
	}
	return out, nil
}
