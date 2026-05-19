package expand

import "fmt"

// Expand takes a pattern string with alternation (a|b) and optional (x)?
// syntax and returns all expanded variants.
//
// Grammar:
//
//	pattern     := segment*
//	segment     := (literal | group) modifier?
//	modifier    := '?'
//	literal     := any char except ( ) | ?
//	group       := '(' branch ('|' branch)* ')'
//	branch      := pattern
//
// Examples:
//
//	"git (status|log) *"      → ["git status *", "git log *"]
//	"git commit (--amend)? *" → ["git commit  *", "git commit --amend *"]
//	"(a|b)(c|d)"              → ["ac", "ad", "bc", "bd"]
func Expand(pattern string) ([]string, error) {
	results, pos, err := expandAt(pattern, 0)
	if err != nil {
		return nil, err
	}
	if pos != len(pattern) {
		return nil, fmt.Errorf("unexpected ')' at position %d in pattern %q", pos, pattern)
	}
	return results, nil
}

// expandAt parses from pos until end-of-string, ')', or '|'.
// Returns the set of expanded strings and the position after parsing.
func expandAt(s string, pos int) ([]string, int, error) {
	results := []string{""}

	for pos < len(s) {
		ch := s[pos]

		switch ch {
		case '(':
			pos++ // consume '('
			branches, newPos, err := parseBranches(s, pos)
			if err != nil {
				return nil, 0, err
			}
			pos = newPos

			// Check for optional modifier '?'
			suffixes := branches
			if pos < len(s) && s[pos] == '?' {
				pos++ // consume '?'
				suffixes = append([]string{""}, branches...)
			}

			results = crossProduct(results, suffixes)

		case ')', '|':
			// Stop; caller handles these
			return results, pos, nil

		case '?':
			// Bare '?' not following a group — treat as literal
			results = appendLiteral(results, "?")
			pos++

		default:
			// Consume a run of literal characters (stop at special chars)
			start := pos
			for pos < len(s) {
				c := s[pos]
				if c == '(' || c == ')' || c == '|' || c == '?' {
					break
				}
				pos++
			}
			results = appendLiteral(results, s[start:pos])
		}
	}

	return results, pos, nil
}

// parseBranches parses '|'-separated branches inside a group, consuming the closing ')'.
// pos should point to the char after the opening '('.
func parseBranches(s string, pos int) ([]string, int, error) {
	var all []string

	for {
		branch, newPos, err := expandAt(s, pos)
		if err != nil {
			return nil, 0, err
		}
		pos = newPos
		all = append(all, branch...)

		if pos >= len(s) {
			return nil, 0, fmt.Errorf("unclosed '(' in pattern %q", s)
		}

		switch s[pos] {
		case ')':
			return all, pos + 1, nil // consume ')'
		case '|':
			pos++ // consume '|', parse next branch
		}
	}
}

func crossProduct(prefixes, suffixes []string) []string {
	out := make([]string, 0, len(prefixes)*len(suffixes))
	for _, p := range prefixes {
		for _, s := range suffixes {
			out = append(out, p+s)
		}
	}
	return out
}

func appendLiteral(current []string, lit string) []string {
	out := make([]string, len(current))
	for i, c := range current {
		out[i] = c + lit
	}
	return out
}
