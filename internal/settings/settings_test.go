package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRaw_NonexistentFile(t *testing.T) {
	raw, err := ReadRaw(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) != 0 {
		t.Errorf("expected empty map, got %v", raw)
	}
}

func TestReadRaw_InvalidJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(f, []byte(`{invalid}`), 0o644)
	_, err := ReadRaw(f)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), f) {
		t.Errorf("error should contain filename, got: %v", err)
	}
}

func TestReadRaw_ValidJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(f, []byte(`{"model":"claude-opus-4","foo":"bar"}`), 0o644)
	raw, err := ReadRaw(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) != 2 {
		t.Errorf("expected 2 keys, got %d", len(raw))
	}
	if _, ok := raw["model"]; !ok {
		t.Error("missing 'model' key")
	}
}

func TestMergePermissions_IntoEmpty(t *testing.T) {
	raw := map[string]json.RawMessage{}
	p := CompiledPermissions{
		Allow: []string{"Bash(ls *)"},
		Deny:  []string{"Bash(rm *)"},
	}
	if err := MergePermissions(raw, p); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Errorf("expected 1 key, got %d", len(raw))
	}
	if _, ok := raw["permissions"]; !ok {
		t.Error("missing 'permissions' key")
	}
}

func TestMergePermissions_PreservesOtherKeys(t *testing.T) {
	raw := map[string]json.RawMessage{
		"model":   json.RawMessage(`"claude-opus-4"`),
		"someKey": json.RawMessage(`{"nested":true}`),
	}
	p := CompiledPermissions{Allow: []string{"Bash(ls *)"}}
	if err := MergePermissions(raw, p); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 3 {
		t.Errorf("expected 3 keys, got %d", len(raw))
	}
	if _, ok := raw["model"]; !ok {
		t.Error("'model' key was removed")
	}
	if _, ok := raw["someKey"]; !ok {
		t.Error("'someKey' key was removed")
	}
}

func TestMergePermissions_ReplacesExisting(t *testing.T) {
	raw := map[string]json.RawMessage{
		"permissions": json.RawMessage(`{"allow":["Bash(old *)"],"deny":["Bash(bad *)"]}`),
	}
	p := CompiledPermissions{Allow: []string{"Bash(new *)"}}
	if err := MergePermissions(raw, p); err != nil {
		t.Fatal(err)
	}

	var got CompiledPermissions
	if err := json.Unmarshal(raw["permissions"], &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Allow) != 1 || got.Allow[0] != "Bash(new *)" {
		t.Errorf("unexpected allow: %v", got.Allow)
	}
	if len(got.Deny) != 0 {
		t.Errorf("deny should be empty, got: %v", got.Deny)
	}
}

func TestMergePermissions_EmptyLists(t *testing.T) {
	raw := map[string]json.RawMessage{}
	p := CompiledPermissions{} // both nil → omitempty
	if err := MergePermissions(raw, p); err != nil {
		t.Fatal(err)
	}
	var got CompiledPermissions
	json.Unmarshal(raw["permissions"], &got)
	if len(got.Allow) != 0 || len(got.Deny) != 0 {
		t.Errorf("expected empty compiled permissions")
	}
}

func TestCurrentPermissionsJSON_Missing(t *testing.T) {
	raw := map[string]json.RawMessage{}
	got := CurrentPermissionsJSON(raw)
	if got != "(none)" {
		t.Errorf("expected '(none)', got %q", got)
	}
}

func TestCurrentPermissionsJSON_Present(t *testing.T) {
	raw := map[string]json.RawMessage{
		"permissions": json.RawMessage(`{"allow":["Bash(ls *)"]}`),
	}
	got := CurrentPermissionsJSON(raw)
	if !strings.Contains(got, "Bash(ls *)") {
		t.Errorf("expected permissions content, got %q", got)
	}
}

func TestWrite_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", ".claude")
	path := filepath.Join(dir, "settings.json")
	raw := map[string]json.RawMessage{"k": json.RawMessage(`"v"`)}
	if err := Write(path, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWrite_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	raw := map[string]json.RawMessage{"x": json.RawMessage(`1`)}
	if err := Write(path, raw); err != nil {
		t.Fatal(err)
	}
	// No .tmp file should remain
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Error("temp file was not cleaned up")
	}
}

func TestWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := map[string]json.RawMessage{
		"model":  json.RawMessage(`"claude-opus-4"`),
		"extras": json.RawMessage(`{"a":1}`),
	}
	MergePermissions(original, CompiledPermissions{Allow: []string{"Bash(ls *)"}})

	if err := Write(path, original); err != nil {
		t.Fatal(err)
	}

	readBack, err := ReadRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(readBack) != 3 {
		t.Errorf("expected 3 keys after round-trip, got %d", len(readBack))
	}
	if _, ok := readBack["model"]; !ok {
		t.Error("'model' lost in round-trip")
	}
	if _, ok := readBack["permissions"]; !ok {
		t.Error("'permissions' lost in round-trip")
	}
}

func TestWrite_EndsWithNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	raw := map[string]json.RawMessage{"a": json.RawMessage(`1`)}
	if err := Write(path, raw); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("file does not end with newline")
	}
}
