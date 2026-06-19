package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveEditor_Visual(t *testing.T) {
	t.Setenv("VISUAL", "/usr/bin/true")
	t.Setenv("EDITOR", "")

	got, err := resolveEditor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/usr/bin/true" {
		t.Errorf("expected /usr/bin/true, got %q", got)
	}
}

func TestResolveEditor_EditorFallback(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "/usr/bin/true")

	got, err := resolveEditor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/usr/bin/true" {
		t.Errorf("expected /usr/bin/true, got %q", got)
	}
}

func TestResolveEditor_VisualTakesPrecedence(t *testing.T) {
	t.Setenv("VISUAL", "/usr/bin/true")
	t.Setenv("EDITOR", "/usr/bin/false")

	got, err := resolveEditor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/usr/bin/true" {
		t.Errorf("expected VISUAL to win, got %q", got)
	}
}

func TestResolveEditor_NoneFound(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("PATH", "")

	_, err := resolveEditor()
	if err == nil {
		t.Fatal("expected error when no editor found")
	}
}

func TestResolveEditor_UnixFallback(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	// Find the first fallback that exists on this system.
	var want string
	for _, name := range unixFallbacks {
		if path, err := exec.LookPath(name); err == nil {
			want = path
			break
		}
	}
	if want == "" {
		t.Skip("no unix fallback editor available on this system")
	}

	got, err := resolveEditor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestOpen_RunsEditorOnFile(t *testing.T) {
	// Use a dummy editor that simply exits 0 without touching the file.
	truebin, err := exec.LookPath("true")
	if err != nil {
		t.Skip("'true' not available")
	}
	t.Setenv("VISUAL", truebin)

	tmp := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(tmp, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Open(tmp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpen_PropagatesEditorError(t *testing.T) {
	falsebin, err := exec.LookPath("false")
	if err != nil {
		t.Skip("'false' not available")
	}
	t.Setenv("VISUAL", falsebin)

	tmp := filepath.Join(t.TempDir(), "test.yaml")
	if err := os.WriteFile(tmp, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Open(tmp); err == nil {
		t.Fatal("expected error from editor exit 1")
	}
}
