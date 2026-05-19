package diff

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

// Display prints a line-level unified diff of oldText vs newText to w.
// ANSI colors are used only when w is a terminal.
func Display(oldText, newText string, w io.Writer) {
	useColor := isTerminal(w)

	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	ops := lcs(oldLines, newLines)

	for _, op := range ops {
		switch op.kind {
		case opEqual:
			fmt.Fprintf(w, "  %s\n", op.line)
		case opRemove:
			if useColor {
				fmt.Fprintf(w, "%s- %s%s\n", colorRed, op.line, colorReset)
			} else {
				fmt.Fprintf(w, "- %s\n", op.line)
			}
		case opAdd:
			if useColor {
				fmt.Fprintf(w, "%s+ %s%s\n", colorGreen, op.line, colorReset)
			} else {
				fmt.Fprintf(w, "+ %s\n", op.line)
			}
		}
	}
}

// Prompt writes "Overwrite? [y/N] " to w, reads a line from r,
// and returns true if the user answered "y" or "Y".
func Prompt(r io.Reader, w io.Writer) (bool, error) {
	fmt.Fprint(w, "Overwrite? [y/N] ")
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		answer := strings.TrimSpace(scanner.Text())
		return strings.ToLower(answer) == "y", nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil // EOF → default No
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err == nil {
			return fi.Mode()&os.ModeCharDevice != 0
		}
	}
	return false
}

type opKind int

const (
	opEqual opKind = iota
	opAdd
	opRemove
)

type diffOp struct {
	kind opKind
	line string
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty string from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lcs computes a line-level diff using the longest common subsequence algorithm.
func lcs(old, new []string) []diffOp {
	m, n := len(old), len(new)

	// dp[i][j] = length of LCS of old[:i] and new[:j]
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if old[i-1] == new[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff ops
	var ops []diffOp
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && old[i-1] == new[j-1]:
			ops = append(ops, diffOp{opEqual, old[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			ops = append(ops, diffOp{opAdd, new[j-1]})
			j--
		default:
			ops = append(ops, diffOp{opRemove, old[i-1]})
			i--
		}
	}

	// Reverse (backtracking produces ops in reverse order)
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}
