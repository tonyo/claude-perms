package expand

import (
	"reflect"
	"sort"
	"testing"
)

func sorted(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

func TestExpand(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    []string
		wantErr bool
	}{
		// Flat / no special syntax
		{
			name:    "plain string",
			pattern: "ls *",
			want:    []string{"ls *"},
		},
		{
			name:    "plain no wildcard",
			pattern: "git status",
			want:    []string{"git status"},
		},
		{
			name:    "empty string",
			pattern: "",
			want:    []string{""},
		},
		{
			name:    "no parens",
			pattern: "npm run build",
			want:    []string{"npm run build"},
		},

		// Alternation
		{
			name:    "simple alternation",
			pattern: "git (status|log) *",
			want:    []string{"git status *", "git log *"},
		},
		{
			name:    "three-way alternation prefix",
			pattern: "(ls|cat|head) *",
			want:    []string{"ls *", "cat *", "head *"},
		},
		{
			name:    "four-way alternation",
			pattern: "git (status|log|diff|show) *",
			want:    []string{"git status *", "git log *", "git diff *", "git show *"},
		},
		{
			name:    "adjacent alternation groups",
			pattern: "(a|b)(c|d)",
			want:    []string{"ac", "ad", "bc", "bd"},
		},
		{
			name:    "single-branch group (no alternation)",
			pattern: "git (status) *",
			want:    []string{"git status *"},
		},
		{
			name:    "alternation at end",
			pattern: "git (push|pull)",
			want:    []string{"git push", "git pull"},
		},
		{
			name:    "alternation at start",
			pattern: "(npm|yarn) install",
			want:    []string{"npm install", "yarn install"},
		},
		{
			name:    "many alternation options",
			pattern: "(ls|cat|head|tail|wc) *",
			want:    []string{"ls *", "cat *", "head *", "tail *", "wc *"},
		},

		// Optional modifier
		{
			name:    "optional group at end",
			pattern: "cmd (--verbose)?",
			want:    []string{"cmd ", "cmd --verbose"},
		},
		{
			name:    "optional group in middle",
			pattern: "git commit (--amend)? *",
			want:    []string{"git commit  *", "git commit --amend *"},
		},
		{
			name:    "two optional groups",
			pattern: "(foo)?(bar)?",
			want:    []string{"", "bar", "foo", "foobar"},
		},
		{
			name:    "optional at start",
			pattern: "(sudo)? apt install",
			want:    []string{" apt install", "sudo apt install"},
		},
		{
			name:    "optional only",
			pattern: "(--flag)?",
			want:    []string{"", "--flag"},
		},

		// Nested groups
		{
			name:    "nested alternation",
			pattern: "(git (push|pull)|npm) *",
			want:    []string{"git push *", "git pull *", "npm *"},
		},
		{
			name:    "nested inside branch",
			pattern: "(a|(b|c)) *",
			want:    []string{"a *", "b *", "c *"},
		},
		{
			name:    "deeply nested",
			pattern: "(a(b(c|d)e|f)g|h)",
			want:    []string{"abceg", "abdeg", "afg", "h"},
		},
		{
			name:    "triple nesting",
			pattern: "(a(b(c|d)|e)|f)",
			want:    []string{"abc", "abd", "ae", "f"},
		},

		// Optional + alternation combined
		{
			name:    "optional alternation group",
			pattern: "git (status|log)? *",
			want:    []string{"git  *", "git status *", "git log *"},
		},
		{
			name:    "nested optional",
			pattern: "(git (push|pull))? install",
			want:    []string{" install", "git push install", "git pull install"},
		},
		{
			name:    "alternation then optional",
			pattern: "(npm|yarn) (install|add) (--save-dev)?",
			want: []string{
				"npm install ", "npm install --save-dev",
				"npm add ", "npm add --save-dev",
				"yarn install ", "yarn install --save-dev",
				"yarn add ", "yarn add --save-dev",
			},
		},

		// Edge cases
		{
			name:    "empty group",
			pattern: "()",
			want:    []string{""},
		},
		{
			name:    "empty first branch",
			pattern: "(|b)",
			want:    []string{"", "b"},
		},
		{
			name:    "empty last branch",
			pattern: "(a|)",
			want:    []string{"a", ""},
		},
		{
			name:    "bare question mark is literal",
			pattern: "foo? bar",
			want:    []string{"foo? bar"},
		},
		{
			name:    "question mark in middle of literal",
			pattern: "a?b",
			want:    []string{"a?b"},
		},
		{
			name:    "glob star preserved",
			pattern: "find * -name *.go",
			want:    []string{"find * -name *.go"},
		},
		{
			name:    "multiple glob stars",
			pattern: "git (status|diff) -- *.go",
			want:    []string{"git status -- *.go", "git diff -- *.go"},
		},
		{
			name:    "real world: npm scripts",
			pattern: "npm (run|exec) *",
			want:    []string{"npm run *", "npm exec *"},
		},
		{
			name:    "real world: deny dangerous",
			pattern: "(rm|sudo|curl|wget) *",
			want:    []string{"rm *", "sudo *", "curl *", "wget *"},
		},
		{
			name:    "real world: git safe ops",
			pattern: "git (status|log|diff|show|branch|stash) *",
			want:    []string{"git status *", "git log *", "git diff *", "git show *", "git branch *", "git stash *"},
		},

		// Error cases
		{
			name:    "unclosed paren",
			pattern: "git (status",
			wantErr: true,
		},
		{
			name:    "unexpected closing paren",
			pattern: "git status)",
			wantErr: true,
		},
		{
			name:    "close before open",
			pattern: "git )status(",
			wantErr: true,
		},
		{
			name:    "double unclosed",
			pattern: "((a|b)",
			wantErr: true,
		},
		{
			name:    "stray close after valid group",
			pattern: "(a|b))",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Expand(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(sorted(got), sorted(tt.want)) {
				t.Errorf("Expand(%q)\n  got  %v\n  want %v", tt.pattern, got, tt.want)
			}
			// Also verify count matches (catches duplicates)
			if len(got) != len(tt.want) {
				t.Errorf("Expand(%q) len = %d, want %d", tt.pattern, len(got), len(tt.want))
			}
		})
	}
}

func TestExpandOrder(t *testing.T) {
	// Verify that output order matches left-to-right, depth-first expansion
	// (not just sorted equality)
	got, err := Expand("(a|b)(c|d)")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ac", "ad", "bc", "bd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order: got %v, want %v", got, want)
	}
}

func TestExpandOptionalOrder(t *testing.T) {
	// Optional: empty string comes first (the "not present" case)
	got, err := Expand("(foo)?")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("optional order: got %v, want %v", got, want)
	}
}
