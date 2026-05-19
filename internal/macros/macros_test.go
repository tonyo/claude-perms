package macros

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		macros  map[string]string
		wantErr string
	}{
		{
			name:    "valid macro",
			macros:  map[string]string{"git_read": "status|log|diff"},
			wantErr: "",
		},
		{
			name:    "empty map",
			macros:  map[string]string{},
			wantErr: "",
		},
		{
			name:    "nil map",
			macros:  nil,
			wantErr: "",
		},
		{
			name:    "valid macro with parens",
			macros:  map[string]string{"foo": "(a|b)"},
			wantErr: "",
		},
		{
			name:    "unclosed paren in value",
			macros:  map[string]string{"broken": "(unclosed"},
			wantErr: `macro "broken"`,
		},
		{
			name:    "stray closing paren in value",
			macros:  map[string]string{"broken": "status)"},
			wantErr: `macro "broken"`,
		},
		{
			name:    "recursive reference rejected",
			macros:  map[string]string{"foo": "{{bar}}"},
			wantErr: `macro "foo"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.macros)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		macros  map[string]string
		want    string
		wantErr string
	}{
		{
			name:    "simple substitution",
			pattern: "git ({{git_read}}) *",
			macros:  map[string]string{"git_read": "status|log"},
			want:    "git (status|log) *",
		},
		{
			name:    "multiple occurrences of same macro",
			pattern: "{{cmd}} --help; {{cmd}} *",
			macros:  map[string]string{"cmd": "git"},
			want:    "git --help; git *",
		},
		{
			name:    "two different macros",
			pattern: "({{a}}|{{b}}) *",
			macros:  map[string]string{"a": "ls", "b": "cat"},
			want:    "(ls|cat) *",
		},
		{
			name:    "no macros in pattern",
			pattern: "git status *",
			macros:  map[string]string{"git_read": "status|log"},
			want:    "git status *",
		},
		{
			name:    "empty macros map no references",
			pattern: "ls *",
			macros:  map[string]string{},
			want:    "ls *",
		},
		{
			name:    "undefined macro reference",
			pattern: "git ({{missing}}) *",
			macros:  map[string]string{},
			wantErr: `undefined macro "missing"`,
		},
		{
			name:    "unclosed braces",
			pattern: "git ({{unclosed) *",
			macros:  map[string]string{},
			wantErr: "unclosed '{{'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Interpolate(tt.pattern, tt.macros)
			if tt.wantErr != "" {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInterpolateAll(t *testing.T) {
	t.Run("all valid", func(t *testing.T) {
		macros := map[string]string{"read": "status|log"}
		patterns := []string{"git ({{read}}) *", "git ({{read}}) --cached *"}
		got, err := InterpolateAll(patterns, macros)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 results, got %d", len(got))
		}
		if got[0] != "git (status|log) *" {
			t.Errorf("got[0] = %q", got[0])
		}
		if got[1] != "git (status|log) --cached *" {
			t.Errorf("got[1] = %q", got[1])
		}
	})

	t.Run("undefined macro stops early", func(t *testing.T) {
		patterns := []string{"git status *", "git ({{missing}}) *"}
		_, err := InterpolateAll(patterns, map[string]string{})
		if err == nil {
			t.Error("expected error for undefined macro")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("error should mention the macro name, got: %v", err)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		got, err := InterpolateAll(nil, map[string]string{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})
}
