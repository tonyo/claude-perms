package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── runCheck ────────────────────────────────────────────────────────────────

func TestRunCheck_NormalOutput(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git (status|log) *"
      - "npm run *"
  deny:
    bash:
      - "(rm|sudo) *"
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Bash(git status *)",
		"Bash(git log *)",
		"Bash(npm run *)",
		"Bash(rm *)",
		"Bash(sudo *)",
		"Allow rules:",
		"Deny rules:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestRunCheck_EmptyLists(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash: []
  deny:
    bash: []
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both sections should show (none)
	if count := strings.Count(out.String(), "(none)"); count != 2 {
		t.Errorf("expected 2 '(none)' markers, got %d:\n%s", count, out.String())
	}
}

func TestRunCheck_InvalidPattern(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git (unclosed"
`)
	var errOut strings.Builder
	err := runCheck(&strings.Builder{}, &errOut, yaml)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
	if !strings.Contains(errOut.String(), "ERROR") {
		t.Errorf("expected ERROR in stderr, got: %s", errOut.String())
	}
}

func TestRunCheck_MissingFile(t *testing.T) {
	err := runCheck(&strings.Builder{}, &strings.Builder{}, "/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRunCheck_OnlyAllow(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "ls *"
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Bash(ls *)") {
		t.Errorf("expected allow rule in output")
	}
	if !strings.Contains(got, "(none)") {
		t.Errorf("expected '(none)' for empty deny section")
	}
}

// ── macros integration ───────────────────────────────────────────────────────

func TestRunCheck_MacrosExpanded(t *testing.T) {
	yaml := writeTempYAML(t, `
macros:
  git_read: "status|log|diff"
permissions:
  allow:
    bash:
      - "git ({{git_read}}) *"
  deny:
    bash: []
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"Bash(git status *)", "Bash(git log *)", "Bash(git diff *)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestRunCheck_InvalidMacro(t *testing.T) {
	yaml := writeTempYAML(t, `
macros:
  broken: "(unclosed"
permissions:
  allow:
    bash:
      - "git ({{broken}}) *"
`)
	err := runCheck(&strings.Builder{}, &strings.Builder{}, yaml)
	if err == nil {
		t.Error("expected error for invalid macro")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should mention the macro name, got: %v", err)
	}
}

func TestRunCheck_UndefinedMacro(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git ({{missing}}) *"
`)
	err := runCheck(&strings.Builder{}, &strings.Builder{}, yaml)
	if err == nil {
		t.Error("expected error for undefined macro")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention the macro name, got: %v", err)
	}
}

func TestRunCompile_MacrosExpanded(t *testing.T) {
	yaml := writeTempYAML(t, `
macros:
  git_read: "status|log"
permissions:
  allow:
    bash:
      - "git ({{git_read}}) *"
  deny:
    bash: []
`)
	outPath := filepath.Join(t.TempDir(), "settings.json")
	err := runCompile(strings.NewReader(""), &strings.Builder{},
		yaml, "project", outPath, false, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	body := string(data)
	if !strings.Contains(body, "Bash(git status *)") {
		t.Errorf("expected expanded macro rule in file, got:\n%s", body)
	}
	if !strings.Contains(body, "Bash(git log *)") {
		t.Errorf("expected expanded macro rule in file, got:\n%s", body)
	}
}

func TestRunCompile_InvalidMacro_ReturnsError(t *testing.T) {
	yaml := writeTempYAML(t, `
macros:
  bad: "(unclosed"
permissions:
  allow:
    bash:
      - "git ({{bad}}) *"
`)
	err := runCompile(strings.NewReader(""), &strings.Builder{},
		yaml, "project", filepath.Join(t.TempDir(), "s.json"), false, true, nil)
	if err == nil {
		t.Error("expected error for invalid macro")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("error should mention the macro name, got: %v", err)
	}
}

// ── resolveSettingsPath ──────────────────────────────────────────────────────

func TestResolveSettingsPath_ExplicitOutput(t *testing.T) {
	got, err := resolveSettingsPath("project", "/some/explicit/path.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/some/explicit/path.json" {
		t.Errorf("got %q, want explicit path", got)
	}
}

func TestResolveSettingsPath_Project(t *testing.T) {
	got, err := resolveSettingsPath("project", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != ".claude/settings.json" {
		t.Errorf("got %q", got)
	}
}

func TestResolveSettingsPath_Local(t *testing.T) {
	got, err := resolveSettingsPath("local", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != ".claude/settings.local.json" {
		t.Errorf("got %q", got)
	}
}

func TestResolveSettingsPath_User(t *testing.T) {
	got, err := resolveSettingsPath("user", "")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".claude", "settings.json")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveSettingsPath_Unknown(t *testing.T) {
	_, err := resolveSettingsPath("bogus", "")
	if err == nil {
		t.Error("expected error for unknown scope")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention the bad scope, got: %v", err)
	}
}

// ── runCompile non-dry-run paths ─────────────────────────────────────────────

func TestRunCompile_Force_WritesFile(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git (status|log) *"
  deny:
    bash:
      - "(rm|sudo) *"
`)
	outPath := filepath.Join(t.TempDir(), "settings.json")
	var out strings.Builder
	err := runCompile(strings.NewReader(""), &out,
		yaml, "project", outPath, false /*dryRun*/, true /*force*/, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := got["permissions"]; !ok {
		t.Error("missing 'permissions' key in output")
	}
	body := string(data)
	if !strings.Contains(body, "Bash(git status *)") {
		t.Errorf("expected expanded rule in file, got:\n%s", body)
	}
}

func TestRunCompile_Force_PreservesOtherKeys(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "settings.json")
	existing := `{"model":"claude-opus-4","theme":"dark","permissions":{"allow":["Bash(old *)"]}}`
	os.WriteFile(outPath, []byte(existing), 0o644)

	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "ls *"
`)
	var out strings.Builder
	err := runCompile(strings.NewReader(""), &out,
		yaml, "project", outPath, false, true /*force*/, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]json.RawMessage
	data, _ := os.ReadFile(outPath)
	json.Unmarshal(data, &got)

	if _, ok := got["model"]; !ok {
		t.Error("'model' key was lost")
	}
	if _, ok := got["theme"]; !ok {
		t.Error("'theme' key was lost")
	}
	if !strings.Contains(string(data), "Bash(ls *)") {
		t.Error("new rule not present")
	}
	if strings.Contains(string(data), "Bash(old *)") {
		t.Error("old rule should have been replaced")
	}
}

func TestRunCompile_PromptDecline_DoesNotWrite(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "settings.json")
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "ls *"
`)
	var out strings.Builder
	err := runCompile(strings.NewReader("n\n"), &out,
		yaml, "project", outPath, false, false /*force*/, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected 'Aborted' in output, got: %s", out.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Error("file should not have been written after declining prompt")
	}
}

func TestRunCompile_PromptAccept_WritesFile(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "settings.json")
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "ls *"
`)
	err := runCompile(strings.NewReader("y\n"), &strings.Builder{},
		yaml, "project", outPath, false, false /*force*/, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("file should have been written after accepting prompt: %v", err)
	}
}

func TestRunCompile_InvalidPattern_ReturnsError(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git (unclosed"
`)
	err := runCompile(strings.NewReader(""), &strings.Builder{},
		yaml, "project", filepath.Join(t.TempDir(), "s.json"), false, true, nil)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestRunCompile_EditAction_ReloadsYAML(t *testing.T) {
	// e → editor rewrites YAML with a new rule → y writes the updated result
	// and copies the edited content back to the original YAML file.
	originalContent := `
permissions:
  allow:
    bash:
      - "ls *"
`
	yamlPath := writeTempYAML(t, originalContent)
	outPath := filepath.Join(t.TempDir(), "settings.json")

	fakeEditor := func(path string) error {
		return os.WriteFile(path, []byte(`
permissions:
  allow:
    bash:
      - "git status *"
`), 0o644)
	}

	err := runCompile(strings.NewReader("e\ny\n"), &strings.Builder{},
		yamlPath, "project", outPath, false, false, fakeEditor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	if !strings.Contains(string(data), "Bash(git status *)") {
		t.Errorf("expected reloaded rule in settings, got:\n%s", data)
	}
	// the edited YAML is committed to the original file on confirm
	yamlData, _ := os.ReadFile(yamlPath)
	if !strings.Contains(string(yamlData), "git status") {
		t.Errorf("expected YAML to be updated on confirm, got:\n%s", yamlData)
	}
}

func TestRunCompile_EditAction_Decline_DoesNotModifyYAML(t *testing.T) {
	// e → editor rewrites temp copy → n → original YAML must be untouched
	originalContent := `
permissions:
  allow:
    bash:
      - "ls *"
`
	yamlPath := writeTempYAML(t, originalContent)
	outPath := filepath.Join(t.TempDir(), "settings.json")

	fakeEditor := func(path string) error {
		return os.WriteFile(path, []byte(`
permissions:
  allow:
    bash:
      - "git status *"
`), 0o644)
	}

	err := runCompile(strings.NewReader("e\nn\n"), &strings.Builder{},
		yamlPath, "project", outPath, false, false, fakeEditor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// settings must not be written
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Error("settings.json should not have been written after declining")
	}
	// original YAML must be unchanged
	yamlData, _ := os.ReadFile(yamlPath)
	if strings.Contains(string(yamlData), "git status") {
		t.Errorf("original YAML must not be modified on decline, got:\n%s", yamlData)
	}
}

// ── check default YAML path ──────────────────────────────────────────────────

func TestCheckCmd_DefaultYAMLPath(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	yamlPath := filepath.Join(dir, "perms.yaml")

	if err := os.WriteFile(yamlPath, []byte(validYAML), 0o644); err != nil {
		t.Fatalf("write perms.yaml: %v", err)
	}

	root := NewRootCmd("test")
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"check", "--output", settingsPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Bash(git status)") {
		t.Errorf("expected expanded rule in output, got: %s", out.String())
	}
}

// ── compile default YAML path ────────────────────────────────────────────────

// compile with no positional arg should resolve the YAML path from --scope /
// --output the same way edit does: <settings-dir>/perms.yaml.
func TestCompileCmd_DefaultYAMLPath(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	yamlPath := filepath.Join(dir, "perms.yaml")

	if err := os.WriteFile(yamlPath, []byte(validYAML), 0o644); err != nil {
		t.Fatalf("write perms.yaml: %v", err)
	}

	root := NewRootCmd("test")
	root.SetIn(strings.NewReader("y\n"))
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"compile", "--output", settingsPath, "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json to be written: %v", err)
	}
}

// ── expandPatterns error path ────────────────────────────────────────────────

func TestExpandPatterns_InvalidPattern(t *testing.T) {
	_, err := expandPatterns([]string{"valid *", "git (unclosed"})
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
	if !strings.Contains(err.Error(), "git (unclosed") {
		t.Errorf("error should mention the bad pattern, got: %v", err)
	}
}

func TestExpandPatterns_AllValid(t *testing.T) {
	got, err := expandPatterns([]string{"git (status|log) *", "npm run *"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // 2 from alternation + 1 plain
		t.Errorf("expected 3 expanded patterns, got %d: %v", len(got), got)
	}
}

// ── multi-tool support ───────────────────────────────────────────────────────

func TestRunCheck_MultipleTools(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git status *"
    read:
      - "./src/**"
    webfetch:
      - "domain:example.com"
  deny:
    bash: []
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Bash(git status *)",
		"Read(./src/**)",
		"WebFetch(domain:example.com)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestRunCheck_MCPTool(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    mcp__puppeteer:
      - "*"
  deny: {}
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "mcp__puppeteer(*)") {
		t.Errorf("expected mcp__puppeteer(*) in output, got:\n%s", out.String())
	}
}

func TestRunCheck_UnknownTool(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bask:
      - "git status *"
`)
	err := runCheck(&strings.Builder{}, &strings.Builder{}, yaml)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "bask") {
		t.Errorf("error should mention the bad key, got: %v", err)
	}
}

func TestRunCompile_MultipleTools(t *testing.T) {
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git status *"
    read:
      - "./src/**"
    mcp__puppeteer:
      - "*"
  deny:
    write:
      - "/etc/**"
`)
	outPath := filepath.Join(t.TempDir(), "settings.json")
	err := runCompile(strings.NewReader(""), &strings.Builder{},
		yaml, "project", outPath, false, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(outPath)
	body := string(data)
	for _, want := range []string{
		"Bash(git status *)",
		"Read(./src/**)",
		"mcp__puppeteer(*)",
		"Write(/etc/**)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in output file, got:\n%s", want, body)
		}
	}
}

func TestRunCheck_NormalizeSpaces(t *testing.T) {
	// "(--amend)?" expands to "" producing "git commit  *" (double space)
	// after normalization it should be "git commit *"
	yaml := writeTempYAML(t, `
permissions:
  allow:
    bash:
      - "git commit (--amend)? *"
`)
	var out strings.Builder
	if err := runCheck(&out, &strings.Builder{}, yaml); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "Bash(git commit  *)") {
		t.Errorf("double space not normalized in output:\n%s", got)
	}
	if !strings.Contains(got, "Bash(git commit *)") {
		t.Errorf("expected normalized rule in output, got:\n%s", got)
	}
}
