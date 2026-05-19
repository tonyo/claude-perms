package yamlconf

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_Valid(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - "git (status|log) *"
      - "npm run *"
  deny:
    bash:
      - "(rm|sudo) *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 2 {
		t.Errorf("allow bash: got %d rules, want 2", len(pf.Permissions.Allow.Bash))
	}
	if pf.Permissions.Allow.Bash[0] != "git (status|log) *" {
		t.Errorf("allow[0] = %q", pf.Permissions.Allow.Bash[0])
	}
	if len(pf.Permissions.Deny.Bash) != 1 {
		t.Errorf("deny bash: got %d rules, want 1", len(pf.Permissions.Deny.Bash))
	}
}

func TestLoad_EmptyLists(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash: []
  deny:
    bash: []
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 0 {
		t.Errorf("expected empty allow list")
	}
	if len(pf.Permissions.Deny.Bash) != 0 {
		t.Errorf("expected empty deny list")
	}
}

func TestLoad_OnlyAllow(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - "ls *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 1 {
		t.Errorf("allow bash: got %d rules, want 1", len(pf.Permissions.Allow.Bash))
	}
	if len(pf.Permissions.Deny.Bash) != 0 {
		t.Errorf("deny bash should be empty")
	}
}

func TestLoad_OnlyDeny(t *testing.T) {
	path := writeTemp(t, `
permissions:
  deny:
    bash:
      - "sudo *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 0 {
		t.Errorf("allow bash should be empty")
	}
	if len(pf.Permissions.Deny.Bash) != 1 {
		t.Errorf("deny bash: got %d rules, want 1", len(pf.Permissions.Deny.Bash))
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeTemp(t, "")
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 0 || len(pf.Permissions.Deny.Bash) != 0 {
		t.Errorf("expected all empty for empty file")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, `{invalid yaml: [}`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_WithOptionalSyntax(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - "git commit (--amend)? *"
      - "(sudo)? apt install *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 2 {
		t.Errorf("allow bash: got %d rules, want 2", len(pf.Permissions.Allow.Bash))
	}
}

func TestLoad_CommentsIgnored(t *testing.T) {
	path := writeTemp(t, `
# this is a comment
permissions:
  allow:
    bash:
      - "ls *"  # inline comment
  # deny is omitted
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow.Bash) != 1 {
		t.Errorf("allow bash: got %d rules, want 1", len(pf.Permissions.Allow.Bash))
	}
}
