package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type CompiledPermissions struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

// ReadRaw reads a settings.json file and returns the top-level keys as raw JSON.
// If the file does not exist, an empty map is returned with no error.
func ReadRaw(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return raw, nil
}

// MergePermissions updates the "permissions" key in raw with the compiled
// allow/deny values, preserving any other keys already present in the
// permissions object (e.g. additionalDirectories).
func MergePermissions(raw map[string]json.RawMessage, p CompiledPermissions) error {
	// Start from the existing permissions object so unknown keys are preserved.
	perms := map[string]json.RawMessage{}
	if existing, ok := raw["permissions"]; ok {
		if err := json.Unmarshal(existing, &perms); err != nil {
			return fmt.Errorf("parse existing permissions: %w", err)
		}
	}

	for _, field := range []struct {
		key  string
		vals []string
	}{
		{"allow", p.Allow},
		{"deny", p.Deny},
	} {
		if len(field.vals) == 0 {
			delete(perms, field.key)
			continue
		}
		encoded, err := json.Marshal(field.vals)
		if err != nil {
			return err
		}
		perms[field.key] = json.RawMessage(encoded)
	}

	encoded, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	raw["permissions"] = json.RawMessage(encoded)
	return nil
}

// CurrentPermissionsJSON returns the pretty-printed "permissions" block from raw,
// or "(none)" if the key is absent.
func CurrentPermissionsJSON(raw map[string]json.RawMessage) string {
	perm, ok := raw["permissions"]
	if !ok {
		return "(none)"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, perm, "", "  "); err != nil {
		return string(perm)
	}
	return buf.String()
}

// Write serializes raw to indented JSON and atomically writes it to path.
// The parent directory is created if it does not exist.
func Write(path string, raw map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
