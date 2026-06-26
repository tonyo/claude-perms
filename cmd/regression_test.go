package cmd

import (
	"os"
	"path/filepath"
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

// Regression: compile prompted for overwrite even when the compiled output
// matched what was already in settings.json (no-change case was missing).
func TestCompile_NoChanges_ShortCircuits(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	yaml := writeTempYAML(t, validYAML)

	// First compile populates settings.json.
	if err := runCompile(strings.NewReader("y\n"), &strings.Builder{}, yaml, "project", settingsPath, false, false, nil); err != nil {
		t.Fatalf("setup: %v", err)
	}
	statBefore, _ := os.Stat(settingsPath)

	// Second compile with the same YAML must not prompt.
	var out strings.Builder
	if err := runCompile(strings.NewReader(""), &out, yaml, "project", settingsPath, false, false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.' in output, got: %s", out.String())
	}
	if s, _ := os.Stat(settingsPath); s.ModTime() != statBefore.ModTime() {
		t.Error("settings.json should not have been rewritten")
	}
}
