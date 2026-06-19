package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tonyo/claude-perms/internal/diff"
	"github.com/tonyo/claude-perms/internal/editor"
	"github.com/tonyo/claude-perms/internal/settings"
)

const emptyYAMLTemplate = `# claude-perms permissions file
# Run: claude-perms compile [this-file] to apply to Claude Code settings.json
#
# Pattern syntax:
#   (a|b|c)   alternation — expands to one rule per branch
#   (foo)?    optional group — expands with and without the group
#   *         glob wildcard
#
# Tool keys: bash, read, edit, write, webfetch, agent, cd, mcp__<server>

permissions:
  allow:
    bash: []
  deny:
    bash: []
`

func newEditCmd() *cobra.Command {
	var scope string
	var output string
	var force bool

	cmd := &cobra.Command{
		Use:   "edit [perms.yaml]",
		Short: "Open YAML in editor, validate, and compile to settings.json",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yamlPath := ""
			if len(args) == 1 {
				yamlPath = args[0]
			} else {
				targetPath, err := resolveSettingsPath(scope, output)
				if err != nil {
					return err
				}
				yamlPath = filepath.Join(filepath.Dir(targetPath), "perms.yaml")
			}
			return runEdit(cmd.InOrStdin(), cmd.OutOrStdout(), yamlPath, scope, output, force, editor.Open)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "Settings scope: project, user, local")
	cmd.Flags().StringVar(&output, "output", "", "Explicit output path (overrides --scope)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

func runEdit(in io.Reader, out io.Writer, yamlPath, scope, outputFlag string, force bool, openEditor func(string) error) error {
	_, statErr := os.Stat(yamlPath)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	isNew := errors.Is(statErr, os.ErrNotExist)

	tmpPath, err := makeTempCopy(yamlPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	targetPath, err := resolveSettingsPath(scope, outputFlag)
	if err != nil {
		return err
	}

	raw, err := settings.ReadRaw(targetPath)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	oldJSON := settings.CurrentPermissionsJSON(raw)

	// One scanner shared across all prompt reads to avoid double-buffering.
	scanner := bufio.NewScanner(in)

	for {
		if err := openEditor(tmpPath); err != nil {
			return err
		}

		allowRules, denyRules, err := validateAndBuild(tmpPath)
		if err != nil {
			fmt.Fprintf(out, "Validation error: %v\n", err)
			reopen, promptErr := promptReopen(scanner, out)
			if promptErr != nil {
				return promptErr
			}
			if reopen {
				continue
			}
			return err
		}

		compiled := settings.CompiledPermissions{
			Allow: allowRules,
			Deny:  denyRules,
		}

		rawCopy := copyRaw(raw)
		if err := settings.MergePermissions(rawCopy, compiled); err != nil {
			return fmt.Errorf("merge permissions: %w", err)
		}
		newJSON := settings.CurrentPermissionsJSON(rawCopy)

		if oldJSON == newJSON {
			fmt.Fprintln(out, "No changes.")
			return nil
		}

		fmt.Fprintf(out, "\nChanges to permissions in %s:\n\n", targetPath)
		diff.Display(oldJSON, newJSON, out)
		fmt.Fprintln(out)

		if !force {
			ok, err := promptOverwrite(scanner, out)
			if err != nil {
				return fmt.Errorf("prompt: %w", err)
			}
			if !ok {
				fmt.Fprintln(out, "Aborted.")
				return nil
			}
		}

		if err := copyFile(tmpPath, yamlPath); err != nil {
			return fmt.Errorf("write %s: %w", yamlPath, err)
		}
		if isNew {
			fmt.Fprintf(out, "Created %s\n", yamlPath)
		}
		if err := settings.Write(targetPath, rawCopy); err != nil {
			return fmt.Errorf("write settings: %w", err)
		}
		fmt.Fprintf(out, "Written to %s\n", targetPath)
		return nil
	}
}

// promptReopen asks whether to re-open the editor after a validation error.
// Defaults to yes on empty input (opposite of promptOverwrite).
func promptReopen(s *bufio.Scanner, w io.Writer) (bool, error) {
	fmt.Fprint(w, "Re-open editor? [Y/n] ")
	if s.Scan() {
		answer := strings.TrimSpace(s.Text())
		return answer == "" || strings.ToLower(answer) == "y", nil
	}
	if err := s.Err(); err != nil {
		return false, err
	}
	return false, nil // EOF → don't reopen
}

// promptOverwrite asks whether to overwrite the settings file.
// Defaults to no on empty input or EOF.
func promptOverwrite(s *bufio.Scanner, w io.Writer) (bool, error) {
	fmt.Fprint(w, "Overwrite? [y/N] ")
	if s.Scan() {
		answer := strings.TrimSpace(s.Text())
		return strings.ToLower(answer) == "y", nil
	}
	if err := s.Err(); err != nil {
		return false, err
	}
	return false, nil // EOF → default No
}

func makeTempCopy(src string) (string, error) {
	var content []byte
	if data, err := os.ReadFile(src); err == nil {
		content = data
	} else if errors.Is(err, os.ErrNotExist) {
		content = []byte(emptyYAMLTemplate)
	} else {
		return "", fmt.Errorf("read %s: %w", src, err)
	}
	f, err := os.CreateTemp("", "claude-perms-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return f.Name(), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(dst); err == nil {
		mode = fi.Mode()
	}
	return os.WriteFile(dst, data, mode)
}

func validateAndBuild(yamlPath string) (allowRules, denyRules []string, err error) {
	perms, err := loadPermissions(yamlPath)
	if err != nil {
		return nil, nil, err
	}
	allowRules, err = buildRules(perms.Allow)
	if err != nil {
		return nil, nil, fmt.Errorf("expand allow rules: %w", err)
	}
	denyRules, err = buildRules(perms.Deny)
	if err != nil {
		return nil, nil, fmt.Errorf("expand deny rules: %w", err)
	}
	return allowRules, denyRules, nil
}

func copyRaw(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
