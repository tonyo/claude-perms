package diff

import (
	"strings"
	"testing"
)

func TestDisplay_Identical(t *testing.T) {
	text := "{\n  \"allow\": [\"Bash(ls *)\"]\n}"
	var sb strings.Builder
	Display(text, text, &sb)
	out := sb.String()
	if strings.Contains(out, "+") || strings.Contains(out, "-") {
		t.Errorf("identical inputs should have no + or - lines, got:\n%s", out)
	}
}

func TestDisplay_CompleteReplace(t *testing.T) {
	old := "line1\nline2\nline3"
	new := "lineA\nlineB\nlineC"
	var sb strings.Builder
	Display(old, new, &sb)
	out := sb.String()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "+ ") && !strings.HasPrefix(l, "- ") {
			t.Errorf("expected all lines to be +/-, got: %q", l)
		}
	}
}

func TestDisplay_SingleLineAdded(t *testing.T) {
	old := "line1\nline2"
	new := "line1\nnew-line\nline2"
	var sb strings.Builder
	Display(old, new, &sb)
	out := sb.String()
	if !strings.Contains(out, "+ new-line") {
		t.Errorf("expected '+ new-line' in output, got:\n%s", out)
	}
	if strings.Count(out, "+") != 1 {
		t.Errorf("expected exactly 1 addition, got:\n%s", out)
	}
}

func TestDisplay_SingleLineRemoved(t *testing.T) {
	old := "line1\nremoved\nline2"
	new := "line1\nline2"
	var sb strings.Builder
	Display(old, new, &sb)
	out := sb.String()
	if !strings.Contains(out, "- removed") {
		t.Errorf("expected '- removed' in output, got:\n%s", out)
	}
	if strings.Count(out, "-") != 1 {
		t.Errorf("expected exactly 1 removal, got:\n%s", out)
	}
}

func TestDisplay_OldEmpty(t *testing.T) {
	var sb strings.Builder
	Display("(none)", "line1\nline2", &sb)
	out := sb.String()
	// All lines should be additions or the (none) removal
	if !strings.Contains(out, "+ line1") {
		t.Errorf("expected '+ line1' in output, got:\n%s", out)
	}
}

func TestDisplay_NewEmpty(t *testing.T) {
	var sb strings.Builder
	Display("line1\nline2", "", &sb)
	out := sb.String()
	if !strings.Contains(out, "- line1") {
		t.Errorf("expected '- line1' in output, got:\n%s", out)
	}
}

func TestDisplay_BothEmpty(t *testing.T) {
	var sb strings.Builder
	Display("", "", &sb)
	out := sb.String()
	if out != "" {
		t.Errorf("expected empty output for empty inputs, got: %q", out)
	}
}

func TestDisplay_NoColors(t *testing.T) {
	// strings.Builder is not a terminal, so no ANSI codes expected
	var sb strings.Builder
	Display("old", "new", &sb)
	out := sb.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("no ANSI codes expected for non-terminal output, got: %q", out)
	}
}

func TestDisplay_ContextLines(t *testing.T) {
	old := "a\nb\nc\nd"
	new := "a\nX\nc\nd"
	var sb strings.Builder
	Display(old, new, &sb)
	out := sb.String()
	// "a", "c", "d" are context (equal), "b" removed, "X" added
	if !strings.Contains(out, "  a") {
		t.Errorf("context line 'a' should appear as equal, got:\n%s", out)
	}
	if !strings.Contains(out, "- b") {
		t.Errorf("expected '- b', got:\n%s", out)
	}
	if !strings.Contains(out, "+ X") {
		t.Errorf("expected '+ X', got:\n%s", out)
	}
}

func TestPrompt_Yes(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "y \n", " Y\n"} {
		r := strings.NewReader(input)
		var sb strings.Builder
		got, err := Prompt(r, &sb)
		if err != nil {
			t.Fatalf("Prompt(%q): unexpected error: %v", input, err)
		}
		if !got {
			t.Errorf("Prompt(%q) = false, want true", input)
		}
	}
}

func TestPrompt_No(t *testing.T) {
	for _, input := range []string{"n\n", "N\n", "\n", "no\n", "nope\n"} {
		r := strings.NewReader(input)
		var sb strings.Builder
		got, err := Prompt(r, &sb)
		if err != nil {
			t.Fatalf("Prompt(%q): unexpected error: %v", input, err)
		}
		if got {
			t.Errorf("Prompt(%q) = true, want false", input)
		}
	}
}

func TestPrompt_EOF(t *testing.T) {
	r := strings.NewReader("") // immediate EOF
	var sb strings.Builder
	got, err := Prompt(r, &sb)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}
	if got {
		t.Error("EOF should return false (default No)")
	}
}

func TestPrompt_PrintsQuestion(t *testing.T) {
	r := strings.NewReader("n\n")
	var sb strings.Builder
	Prompt(r, &sb)
	if !strings.Contains(sb.String(), "Overwrite?") {
		t.Errorf("expected prompt text, got: %q", sb.String())
	}
}

func TestLCS_Simple(t *testing.T) {
	ops := lcs([]string{"a", "b", "c"}, []string{"a", "X", "c"})
	var removes, adds, equals int
	for _, op := range ops {
		switch op.kind {
		case opRemove:
			removes++
		case opAdd:
			adds++
		case opEqual:
			equals++
		}
	}
	if removes != 1 || adds != 1 || equals != 2 {
		t.Errorf("expected 1 remove, 1 add, 2 equal; got %d/%d/%d", removes, adds, equals)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"a\nb\n", []string{"a", "b"}}, // trailing newline stripped
		{"a\n\nb", []string{"a", "", "b"}},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitLines(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
