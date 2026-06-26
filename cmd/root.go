package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tonyo/claude-perms/internal/diff"
	"github.com/tonyo/claude-perms/internal/editor"
	"github.com/tonyo/claude-perms/internal/expand"
	"github.com/tonyo/claude-perms/internal/macros"
	"github.com/tonyo/claude-perms/internal/settings"
	"github.com/tonyo/claude-perms/internal/yamlconf"
)

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "claude-perms",
		Version: version,
		Short:   "Claude Code permissions YAML compiler",
		Long: `Compiles a YAML permissions file into the permissions block
of a Claude Code settings.json file.

Pattern syntax:
  (a|b|c)   Alternation — expands to one rule per branch
  (foo)?    Optional   — expands to rules with and without the group
  *         Glob wildcard (passed through to Claude Code as-is)`,
		SilenceUsage: true,
	}
	root.AddCommand(newCompileCmd())
	root.AddCommand(newCheckCmd())
	root.AddCommand(newEditCmd(editor.Open))
	return root
}

func newCompileCmd() *cobra.Command {
	var scope string
	var output string
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "compile <perms.yaml>",
		Short: "Compile YAML into Claude Code settings.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompile(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], scope, output, dryRun, force, editor.Open)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "Settings scope: project, user, local")
	cmd.Flags().StringVar(&output, "output", "", "Explicit output path (overrides --scope)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print result to stdout without writing")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

type taggedPattern struct {
	Tool    string
	Pattern string
}

type preparedPerms struct {
	Allow []taggedPattern
	Deny  []taggedPattern
}

func runCompile(in io.Reader, out io.Writer, yamlPath, scope, outputFlag string, dryRun, force bool, openEditor func(string) error) error {
	compiled, err := compileYAML(yamlPath)
	if err != nil {
		return err
	}

	if dryRun {
		raw := map[string]json.RawMessage{}
		if err := settings.MergePermissions(raw, compiled); err != nil {
			return err
		}
		fmt.Fprintln(out, settings.CurrentPermissionsJSON(raw))
		return nil
	}

	targetPath, err := resolveSettingsPath(scope, outputFlag)
	if err != nil {
		return err
	}

	raw, err := settings.ReadRaw(targetPath)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	oldJSON := settings.CurrentPermissionsJSON(raw)
	scanner := diff.NewScanner(in)

	var tmpYAMLPath string
	defer func() {
		if tmpYAMLPath != "" {
			os.Remove(tmpYAMLPath)
		}
	}()

	for {
		rawCopy := copyRaw(raw)
		if err := settings.MergePermissions(rawCopy, compiled); err != nil {
			return fmt.Errorf("merge permissions: %w", err)
		}
		newJSON := settings.CurrentPermissionsJSON(rawCopy)

		if force {
			if err := settings.Write(targetPath, rawCopy); err != nil {
				return fmt.Errorf("write settings: %w", err)
			}
			fmt.Fprintf(out, "Written to %s\n", targetPath)
			return nil
		}

		fmt.Fprintf(out, "\nChanges to permissions in %s:\n\n", targetPath)
		diff.Display(oldJSON, newJSON, out)
		fmt.Fprintln(out)

		action, err := diff.Prompt(scanner, out, oldJSON, newJSON)
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		switch action {
		case diff.ActionYes:
			if tmpYAMLPath != "" {
				if err := copyFile(tmpYAMLPath, yamlPath); err != nil {
					return fmt.Errorf("write %s: %w", yamlPath, err)
				}
			}
			if err := settings.Write(targetPath, rawCopy); err != nil {
				return fmt.Errorf("write settings: %w", err)
			}
			fmt.Fprintf(out, "Written to %s\n", targetPath)
			return nil
		case diff.ActionNo:
			fmt.Fprintln(out, "Aborted.")
			return nil
		case diff.ActionEdit:
			if tmpYAMLPath == "" {
				p, err := makeTempCopy(yamlPath)
				if err != nil {
					return err
				}
				tmpYAMLPath = p
			}
			for {
				if err := openEditor(tmpYAMLPath); err != nil {
					return err
				}
				newCompiled, err := compileYAML(tmpYAMLPath)
				if err != nil {
					fmt.Fprintf(out, "Validation error: %v\n", err)
					reopen, promptErr := promptReopen(scanner, out)
					if promptErr != nil {
						return promptErr
					}
					if reopen {
						continue
					}
					return fmt.Errorf("compile after edit: %w", err)
				}
				compiled = newCompiled
				break
			}
		}
	}
}

func compileYAML(yamlPath string) (settings.CompiledPermissions, error) {
	perms, err := loadPermissions(yamlPath)
	if err != nil {
		return settings.CompiledPermissions{}, err
	}
	allow, err := buildRules(perms.Allow)
	if err != nil {
		return settings.CompiledPermissions{}, fmt.Errorf("expand allow rules: %w", err)
	}
	deny, err := buildRules(perms.Deny)
	if err != nil {
		return settings.CompiledPermissions{}, fmt.Errorf("expand deny rules: %w", err)
	}
	return settings.CompiledPermissions{Allow: allow, Deny: deny}, nil
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <perms.yaml>",
		Short: "Validate and preview expanded rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd.OutOrStdout(), cmd.ErrOrStderr(), args[0])
		},
	}
}

func runCheck(out, errOut io.Writer, yamlPath string) error {
	perms, err := loadPermissions(yamlPath)
	if err != nil {
		return err
	}

	hasErr := false

	printRules := func(label string, patterns []taggedPattern) {
		fmt.Fprintln(out, label)
		if len(patterns) == 0 {
			fmt.Fprintln(out, "  (none)")
			return
		}
		for _, tp := range patterns {
			expanded, err := expandNormalized(tp.Pattern)
			if err != nil {
				fmt.Fprintf(errOut, "  ERROR: %q: %v\n", tp.Pattern, err)
				hasErr = true
				continue
			}
			for _, e := range expanded {
				fmt.Fprintf(out, "  %s(%s)\n", tp.Tool, e)
			}
		}
	}

	printRules("Allow rules:", perms.Allow)
	fmt.Fprintln(out)
	printRules("Deny rules:", perms.Deny)

	if hasErr {
		return fmt.Errorf("one or more patterns failed to expand")
	}
	return nil
}

func loadPermissions(yamlPath string) (*preparedPerms, error) {
	pf, err := yamlconf.Load(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("load yaml: %w", err)
	}
	if err := macros.Validate(pf.Macros); err != nil {
		return nil, err
	}
	allow, err := collectTagged(pf.Permissions.Allow, pf.Macros)
	if err != nil {
		return nil, fmt.Errorf("interpolate allow rules: %w", err)
	}
	deny, err := collectTagged(pf.Permissions.Deny, pf.Macros)
	if err != nil {
		return nil, fmt.Errorf("interpolate deny rules: %w", err)
	}
	return &preparedPerms{Allow: allow, Deny: deny}, nil
}

func collectTagged(rules yamlconf.ToolRules, macroMap map[string]string) ([]taggedPattern, error) {
	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []taggedPattern
	for _, key := range keys {
		toolName, err := resolveToolName(key)
		if err != nil {
			return nil, err
		}
		interpolated, err := macros.InterpolateAll(rules[key], macroMap)
		if err != nil {
			return nil, err
		}
		for _, p := range interpolated {
			out = append(out, taggedPattern{Tool: toolName, Pattern: p})
		}
	}
	return out, nil
}

func resolveToolName(key string) (string, error) {
	switch key {
	case "bash":
		return "Bash", nil
	case "powershell":
		return "PowerShell", nil
	case "read":
		return "Read", nil
	case "edit":
		return "Edit", nil
	case "write":
		return "Write", nil
	case "webfetch":
		return "WebFetch", nil
	case "agent":
		return "Agent", nil
	case "cd":
		return "Cd", nil
	}
	if strings.HasPrefix(key, "mcp__") {
		return key, nil
	}
	return "", fmt.Errorf("unknown tool %q (valid: bash, powershell, read, edit, write, webfetch, agent, cd, mcp__<server>)", key)
}

func expandPatterns(patterns []string) ([]string, error) {
	var out []string
	for _, p := range patterns {
		expanded, err := expand.Expand(p)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func wrapTool(tool string, patterns []string) []string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		out[i] = tool + "(" + p + ")"
	}
	return out
}

func expandNormalized(pattern string) ([]string, error) {
	out, err := expand.Expand(pattern)
	if err != nil {
		return nil, err
	}
	for i, e := range out {
		out[i] = expand.NormalizeSpaces(e)
	}
	return out, nil
}

func buildRules(tagged []taggedPattern) ([]string, error) {
	var out []string
	for _, tp := range tagged {
		expanded, err := expandNormalized(tp.Pattern)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", tp.Pattern, err)
		}
		out = append(out, wrapTool(tp.Tool, expanded)...)
	}
	return out, nil
}

func resolveSettingsPath(scope, outputFlag string) (string, error) {
	if outputFlag != "" {
		return outputFlag, nil
	}
	switch scope {
	case "project":
		return ".claude/settings.json", nil
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	case "local":
		return ".claude/settings.local.json", nil
	default:
		return "", fmt.Errorf("unknown scope %q (valid: project, user, local)", scope)
	}
}
