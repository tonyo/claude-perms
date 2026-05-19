# claude-perms

A CLI tool that compiles a YAML permissions file into the `permissions` block of a [Claude Code](https://claude.ai/code) `settings.json`. Supports richer pattern syntax than the raw JSON format — alternation, optional groups, and comments.

## Problem

Claude Code's `settings.json` permissions are flat JSON strings with limited glob syntax. Writing rules for multiple similar commands requires repetition:

```json
"allow": [
  "Bash(git status *)",
  "Bash(git log *)",
  "Bash(git diff *)",
  "Bash(git show *)"
]
```

## Solution

Write a YAML file with alternation syntax, let `claude-perms` expand it:

```yaml
permissions:
  allow:
    bash:
      - "git (status|log|diff|show) *"
```

## Installation

```bash
go install github.com/tonyo/claude-perms@latest
```

Or build from source:

```bash
git clone https://github.com/tonyo/claude-perms
cd claude-perms
go build -o claude-perms .
```

## Usage

```bash
# Preview what rules would be generated
claude-perms check perms.yaml

# Compile into .claude/settings.json (shows diff, prompts before writing)
claude-perms compile perms.yaml

# Other scopes
claude-perms compile --scope user perms.yaml   # ~/.claude/settings.json
claude-perms compile --scope local perms.yaml  # .claude/settings.local.json

# Explicit output path
claude-perms compile --output path/to/settings.json perms.yaml

# Skip confirmation prompt
claude-perms compile --force perms.yaml

# Print result to stdout without writing
claude-perms compile --dry-run perms.yaml
```

## Pattern syntax

| Syntax | Meaning | Expands to |
|---|---|---|
| `(a\|b\|c)` | Alternation | One rule per branch |
| `(foo)?` | Optional group | Rules with and without the group |
| `*` | Glob wildcard | Passed through as-is |

Groups can be nested: `(git (push|pull)|npm) *`

A bare `?` not following a group is treated as a literal character.

## Example

`perms.yaml`:

```yaml
# Safe read-only git commands
permissions:
  allow:
    bash:
      - "git (status|log|diff|show|branch|stash) *"
      - "git commit (--amend)? *"
      - "npm (run|exec) *"
      - "(ls|cat|head|tail|grep|wc) *"

  deny:
    bash:
      - "git (push|reset|clean|checkout) *"
      - "(rm|sudo|curl|wget) *"
```

`claude-perms check perms.yaml` output:

```
Allow rules:
  Bash(git status *)
  Bash(git log *)
  Bash(git diff *)
  Bash(git show *)
  Bash(git branch *)
  Bash(git stash *)
  Bash(git commit  *)
  Bash(git commit --amend *)
  Bash(npm run *)
  Bash(npm exec *)
  Bash(ls *)
  ...

Deny rules:
  Bash(git push *)
  Bash(git reset *)
  ...
```

`claude-perms compile --force perms.yaml` writes the expanded rules into `.claude/settings.json`, leaving all other keys (model, theme, etc.) untouched.

## How it works

The tool:
1. Parses the YAML input
2. Expands each pattern using a recursive descent parser (handles nesting)
3. Wraps each result in `Bash(...)`
4. Reads the target `settings.json` (if it exists) using `map[string]json.RawMessage` to preserve unknown keys
5. Replaces only the `permissions` block
6. Shows a line-level diff and prompts before writing
7. Writes atomically (temp file + rename)
