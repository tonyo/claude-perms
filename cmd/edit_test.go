package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validYAML = `
permissions:
  allow:
    bash:
      - "git status"
`

const validYAMLAlt = `
permissions:
  allow:
    bash:
      - "git log"
`

const invalidYAML = `
permissions:
  allow:
    bash:
      - "git (unclosed"
`

// fakeEditor returns a func that writes content to the file it receives.
func fakeEditor(content string) func(string) error {
	return func(path string) error {
		return os.WriteFile(path, []byte(content), 0o644)
	}
}

// multiEditor cycles through a slice of editors on each call.
func multiEditor(editors ...func(string) error) func(string) error {
	calls := 0
	return func(path string) error {
		if calls >= len(editors) {
			return errors.New("editor called too many times")
		}
		e := editors[calls]
		calls++
		return e(path)
	}
}

func TestRunEdit_FileNotExist_CreatesTemplate(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "perms.yaml")
	settingsPath := filepath.Join(dir, "settings.json")

	var out strings.Builder
	editorSawTemplate := false
	edit := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "permissions:") {
			editorSawTemplate = true
		}
		return os.WriteFile(path, []byte(validYAML), 0o644)
	}

	err := runEdit(strings.NewReader("y\n"), &out, yamlPath, "project", settingsPath, false, edit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !editorSawTemplate {
		t.Error("editor did not see template content in temp file")
	}
	// yamlPath must be created on confirmation
	if _, err := os.Stat(yamlPath); err != nil {
		t.Errorf("expected perms.yaml to be created: %v", err)
	}
	if !strings.Contains(out.String(), "Created") {
		t.Errorf("expected 'Created' in output, got: %s", out.String())
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json to be written: %v", err)
	}
}

func TestRunEdit_ValidYAML_WritesOnAccept(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	var out strings.Builder
	err := runEdit(strings.NewReader("y\n"), &out, yamlPath, "project", settingsPath, false, fakeEditor(validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json written: %v", err)
	}
	if !strings.Contains(out.String(), "Written to") {
		t.Errorf("expected 'Written to' in output, got: %s", out.String())
	}
}

func TestRunEdit_ValidYAML_AbortsOnDecline(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	originalContent, _ := os.ReadFile(yamlPath)

	var out strings.Builder
	err := runEdit(strings.NewReader("n\n"), &out, yamlPath, "project", settingsPath, false, fakeEditor(validYAMLAlt))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// settings.json must not be written
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("expected settings.json NOT to be written")
	}
	// original YAML must be unchanged
	afterContent, _ := os.ReadFile(yamlPath)
	if string(afterContent) != string(originalContent) {
		t.Errorf("original YAML was modified on abort:\nbefore: %s\nafter: %s", originalContent, afterContent)
	}
	if !strings.Contains(out.String(), "Aborted.") {
		t.Errorf("expected 'Aborted.' in output, got: %s", out.String())
	}
}

func TestRunEdit_NoChanges_ShortCircuits(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-populate settings.json with the compiled output of validYAML.
	yamlPath := writeTempYAML(t, validYAML)
	var preOut strings.Builder
	if err := runEdit(strings.NewReader("y\n"), &preOut, yamlPath, "project", settingsPath, false, fakeEditor(validYAML)); err != nil {
		t.Fatalf("setup: %v", err)
	}
	statBefore, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("setup: stat settings.json: %v", err)
	}
	yamlStatBefore, _ := os.Stat(yamlPath)

	// Edit again with the same YAML — no changes expected.
	var out strings.Builder
	if err := runEdit(strings.NewReader(""), &out, yamlPath, "project", settingsPath, false, fakeEditor(validYAML)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No changes.") {
		t.Errorf("expected 'No changes.' in output, got: %s", out.String())
	}
	// Neither settings.json nor the YAML source should be rewritten.
	if s, _ := os.Stat(settingsPath); s.ModTime() != statBefore.ModTime() {
		t.Error("settings.json should not have been rewritten")
	}
	if s, _ := os.Stat(yamlPath); s.ModTime() != yamlStatBefore.ModTime() {
		t.Error("perms.yaml should not have been rewritten on no-change")
	}
}

func TestRunEdit_InvalidYAML_ThenFixed_Loops(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, invalidYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	edit := multiEditor(fakeEditor(invalidYAML), fakeEditor(validYAML))

	// Input: "y" to reopen after error, then "y" to confirm write.
	var out strings.Builder
	err := runEdit(strings.NewReader("y\ny\n"), &out, yamlPath, "project", settingsPath, false, edit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Validation error") {
		t.Errorf("expected 'Validation error' in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Re-open editor?") {
		t.Errorf("expected re-open prompt in output, got: %s", out.String())
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json to be written after fix: %v", err)
	}
}

func TestRunEdit_InvalidYAML_Decline_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML) // start with valid content
	settingsPath := filepath.Join(dir, "settings.json")

	originalContent, _ := os.ReadFile(yamlPath)

	var out strings.Builder
	// Editor writes invalid YAML to the temp file; user declines re-open.
	err := runEdit(strings.NewReader("n\n"), &out, yamlPath, "project", settingsPath, false, fakeEditor(invalidYAML))
	if err == nil {
		t.Fatal("expected error after declining reopen")
	}
	// Original YAML must be untouched.
	afterContent, _ := os.ReadFile(yamlPath)
	if string(afterContent) != string(originalContent) {
		t.Errorf("original YAML was modified despite error:\nbefore: %s\nafter: %s", originalContent, afterContent)
	}
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		t.Error("expected settings.json NOT to be written")
	}
}

func TestRunEdit_Force_SkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	var out strings.Builder
	// Empty reader — no prompt input needed with force=true.
	err := runEdit(strings.NewReader(""), &out, yamlPath, "project", settingsPath, true, fakeEditor(validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Written to") {
		t.Errorf("expected 'Written to' in output, got: %s", out.String())
	}
}

func TestRunEdit_EditorError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	editorErr := errors.New("editor crashed")
	failEditor := func(string) error { return editorErr }

	var out strings.Builder
	err := runEdit(strings.NewReader(""), &out, yamlPath, "project", settingsPath, false, failEditor)
	if err == nil {
		t.Fatal("expected error from editor failure")
	}
	if !errors.Is(err, editorErr) {
		t.Errorf("expected editorErr, got: %v", err)
	}
}

func TestRunEdit_InvalidScope_ReturnsError(t *testing.T) {
	yamlPath := writeTempYAML(t, validYAML)
	var out strings.Builder
	err := runEdit(strings.NewReader(""), &out, yamlPath, "bogus", "", false, func(string) error { return nil })
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}

func TestRunEdit_PromptRedisplay(t *testing.T) {
	// d redisplays the diff, then y confirms
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	var out strings.Builder
	err := runEdit(strings.NewReader("d\ny\n"), &out, yamlPath, "project", settingsPath, false, fakeEditor(validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json written after d+y: %v", err)
	}
}

func TestRunEdit_PromptEdit_ReOpensEditor(t *testing.T) {
	// e re-opens the editor (edits to validYAMLAlt), then y confirms
	dir := t.TempDir()
	yamlPath := writeTempYAML(t, validYAML)
	settingsPath := filepath.Join(dir, "settings.json")

	edit := multiEditor(fakeEditor(validYAML), fakeEditor(validYAMLAlt))

	var out strings.Builder
	err := runEdit(strings.NewReader("e\ny\n"), &out, yamlPath, "project", settingsPath, false, edit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json written: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(data), "Bash(git log)") {
		t.Errorf("expected rule from second edit in settings, got:\n%s", data)
	}
}
