package cmd

import (
	"strings"
	"testing"
)

// Regression: compile --dry-run panicked with "assignment to entry in nil map"
// because the dry-run path called ReadRaw("/dev/null") which returned a nil map,
// then passed it to MergePermissions which tried to assign into it.
func TestCompileDryRun_NoPanic(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git (status|log) *"
  deny:
    bash:
      - "(rm|sudo) *"
`)
	var out strings.Builder
	err := runCompile(strings.NewReader(""), &out, yaml, "project", "", true /*dryRun*/, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Bash(git status *)") {
		t.Errorf("expected expanded rules in output, got: %s", out.String())
	}
}

// Regression: same crash path but with empty allow/deny lists.
func TestCompileDryRun_EmptyLists_NoPanic(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash: []
  deny:
    bash: []
`)
	var out strings.Builder
	err := runCompile(strings.NewReader(""), &out, yaml, "project", "", true, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Regression: same crash path with no permissions block at all.
func TestCompileDryRun_NoPermissions_NoPanic(t *testing.T) {
	yaml := writeTempYAML(t, `{}`)
	var out strings.Builder
	err := runCompile(strings.NewReader(""), &out, yaml, "project", "", true, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
