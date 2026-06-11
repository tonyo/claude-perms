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
	if len(pf.Permissions.Allow["bash"]) != 2 {
		t.Errorf("allow bash: got %d rules, want 2", len(pf.Permissions.Allow["bash"]))
	}
	if pf.Permissions.Allow["bash"][0] != "git (status|log) *" {
		t.Errorf("allow[0] = %q", pf.Permissions.Allow["bash"][0])
	}
	if len(pf.Permissions.Deny["bash"]) != 1 {
		t.Errorf("deny bash: got %d rules, want 1", len(pf.Permissions.Deny["bash"]))
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
	if len(pf.Permissions.Allow["bash"]) != 0 {
		t.Errorf("expected empty allow list")
	}
	if len(pf.Permissions.Deny["bash"]) != 0 {
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
	if len(pf.Permissions.Allow["bash"]) != 1 {
		t.Errorf("allow bash: got %d rules, want 1", len(pf.Permissions.Allow["bash"]))
	}
	if len(pf.Permissions.Deny["bash"]) != 0 {
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
	if len(pf.Permissions.Allow["bash"]) != 0 {
		t.Errorf("allow bash should be empty")
	}
	if len(pf.Permissions.Deny["bash"]) != 1 {
		t.Errorf("deny bash: got %d rules, want 1", len(pf.Permissions.Deny["bash"]))
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	path := writeTemp(t, "")
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 0 || len(pf.Permissions.Deny["bash"]) != 0 {
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
	if len(pf.Permissions.Allow["bash"]) != 2 {
		t.Errorf("allow bash: got %d rules, want 2", len(pf.Permissions.Allow["bash"]))
	}
}

func TestLoad_DictItem(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          log:
            - "*"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 1 || pf.Permissions.Allow["bash"][0] != "git log *" {
		t.Errorf("got %v, want [git log *]", pf.Permissions.Allow["bash"])
	}
}

func TestLoad_DictItemMultipleKeys(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          status:
            - "*"
          log:
            - "*"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 2 {
		t.Fatalf("got %d rules, want 2: %v", len(pf.Permissions.Allow["bash"]), pf.Permissions.Allow["bash"])
	}
}

func TestLoad_DictItemDeepNesting(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          submodule:
            update:
              - "--init *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 1 || pf.Permissions.Allow["bash"][0] != "git submodule update --init *" {
		t.Errorf("got %v, want [git submodule update --init *]", pf.Permissions.Allow["bash"])
	}
}

func TestLoad_MixedStringAndDict(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - "npm run *"
      - git:
          status:
            - "*"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 2 {
		t.Fatalf("got %d rules, want 2: %v", len(pf.Permissions.Allow["bash"]), pf.Permissions.Allow["bash"])
	}
	if pf.Permissions.Allow["bash"][0] != "npm run *" {
		t.Errorf("got[0] = %q, want \"npm run *\"", pf.Permissions.Allow["bash"][0])
	}
	if pf.Permissions.Allow["bash"][1] != "git status *" {
		t.Errorf("got[1] = %q, want \"git status *\"", pf.Permissions.Allow["bash"][1])
	}
}

func TestLoad_DictItemEmptyLeaf(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          status: []
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 0 {
		t.Errorf("expected empty rules for empty leaf, got %v", pf.Permissions.Allow["bash"])
	}
}

func TestLoad_DictItemAlternationInLeaf(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          log:
            - "(--oneline|--stat) *"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 1 || pf.Permissions.Allow["bash"][0] != "git log (--oneline|--stat) *" {
		t.Errorf("got %v", pf.Permissions.Allow["bash"])
	}
}

func TestLoad_MultipleDictItems(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - git:
          log:
            - "*"
      - docker:
          compose:
            up:
              - "*"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Permissions.Allow["bash"]) != 2 {
		t.Fatalf("got %d rules, want 2: %v", len(pf.Permissions.Allow["bash"]), pf.Permissions.Allow["bash"])
	}
	if pf.Permissions.Allow["bash"][0] != "git log *" {
		t.Errorf("got[0] = %q", pf.Permissions.Allow["bash"][0])
	}
	if pf.Permissions.Allow["bash"][1] != "docker compose up *" {
		t.Errorf("got[1] = %q", pf.Permissions.Allow["bash"][1])
	}
}

func TestLoad_MultipleTools(t *testing.T) {
	path := writeTemp(t, `
permissions:
  allow:
    bash:
      - "git status *"
    read:
      - "./src/**"
    mcp__puppeteer:
      - "*"
`)
	pf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pf.Permissions.Allow["bash"]; len(got) != 1 || got[0] != "git status *" {
		t.Errorf("bash: got %v", got)
	}
	if got := pf.Permissions.Allow["read"]; len(got) != 1 || got[0] != "./src/**" {
		t.Errorf("read: got %v", got)
	}
	if got := pf.Permissions.Allow["mcp__puppeteer"]; len(got) != 1 || got[0] != "*" {
		t.Errorf("mcp__puppeteer: got %v", got)
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
	if len(pf.Permissions.Allow["bash"]) != 1 {
		t.Errorf("allow bash: got %d rules, want 1", len(pf.Permissions.Allow["bash"]))
	}
}
