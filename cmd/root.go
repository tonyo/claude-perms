package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tonyo/claude-perms/internal/diff"
	"github.com/tonyo/claude-perms/internal/expand"
	"github.com/tonyo/claude-perms/internal/macros"
	"github.com/tonyo/claude-perms/internal/settings"
	"github.com/tonyo/claude-perms/internal/yamlconf"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "claude-perms",
		Short: "Claude Code permissions YAML compiler",
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
			return runCompile(cmd.InOrStdin(), cmd.OutOrStdout(), args[0], scope, output, dryRun, force)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "project", "Settings scope: project, user, local")
	cmd.Flags().StringVar(&output, "output", "", "Explicit output path (overrides --scope)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print result to stdout without writing")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}

type preparedPerms struct {
	Allow []string
	Deny  []string
}

func runCompile(in io.Reader, out io.Writer, yamlPath, scope, outputFlag string, dryRun, force bool) error {
	perms, err := loadPermissions(yamlPath)
	if err != nil {
		return err
	}

	allowRules, err := expandPatterns(perms.Allow)
	if err != nil {
		return fmt.Errorf("expand allow rules: %w", err)
	}
	denyRules, err := expandPatterns(perms.Deny)
	if err != nil {
		return fmt.Errorf("expand deny rules: %w", err)
	}

	compiled := settings.CompiledPermissions{
		Allow: wrapBash(allowRules),
		Deny:  wrapBash(denyRules),
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

	if err := settings.MergePermissions(raw, compiled); err != nil {
		return fmt.Errorf("merge permissions: %w", err)
	}

	newJSON := settings.CurrentPermissionsJSON(raw)

	fmt.Fprintf(out, "\nChanges to permissions in %s:\n\n", targetPath)
	diff.Display(oldJSON, newJSON, out)
	fmt.Fprintln(out)

	if !force {
		ok, err := diff.Prompt(in, out)
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		if !ok {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	if err := settings.Write(targetPath, raw); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	fmt.Fprintf(out, "Written to %s\n", targetPath)
	return nil
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

	fmt.Fprintln(out, "Allow rules:")
	if len(perms.Allow) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, p := range perms.Allow {
		expanded, err := expand.Expand(p)
		if err != nil {
			fmt.Fprintf(errOut, "  ERROR: %q: %v\n", p, err)
			hasErr = true
			continue
		}
		for _, e := range expanded {
			fmt.Fprintf(out, "  Bash(%s)\n", e)
		}
	}

	fmt.Fprintln(out, "\nDeny rules:")
	if len(perms.Deny) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, p := range perms.Deny {
		expanded, err := expand.Expand(p)
		if err != nil {
			fmt.Fprintf(errOut, "  ERROR: %q: %v\n", p, err)
			hasErr = true
			continue
		}
		for _, e := range expanded {
			fmt.Fprintf(out, "  Bash(%s)\n", e)
		}
	}

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
	allow, err := macros.InterpolateAll(pf.Permissions.Allow.Bash, pf.Macros)
	if err != nil {
		return nil, fmt.Errorf("interpolate allow rules: %w", err)
	}
	deny, err := macros.InterpolateAll(pf.Permissions.Deny.Bash, pf.Macros)
	if err != nil {
		return nil, fmt.Errorf("interpolate deny rules: %w", err)
	}
	return &preparedPerms{Allow: allow, Deny: deny}, nil
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

func wrapBash(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]string, len(patterns))
	for i, p := range patterns {
		out[i] = "Bash(" + p + ")"
	}
	return out
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
