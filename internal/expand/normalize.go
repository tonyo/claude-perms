package expand

import "strings"

// NormalizeSpaces collapses consecutive spaces outside quoted string literals
// to a single space and trims leading/trailing whitespace.
func NormalizeSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inDouble := false
	inSingle := false
	prevSpace := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		switch {
		case inDouble:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) && s[i+1] == '"' {
				i++
				b.WriteByte(s[i])
			} else if c == '"' {
				inDouble = false
			}
			prevSpace = false

		case inSingle:
			b.WriteByte(c)
			if c == '\'' {
				inSingle = false
			}
			prevSpace = false

		case c == '"':
			inDouble = true
			prevSpace = false
			b.WriteByte(c)

		case c == '\'':
			inSingle = true
			prevSpace = false
			b.WriteByte(c)

		case c == ' ':
			if !prevSpace {
				b.WriteByte(c)
			}
			prevSpace = true

		default:
			prevSpace = false
			b.WriteByte(c)
		}
	}

	return strings.TrimSpace(b.String())
}
