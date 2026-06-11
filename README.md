# claude-perms

A CLI tool that compiles a YAML permissions file into the `permissions` block of a [Claude Code](https://claude.ai/code) `settings.json`. Supports richer pattern syntax than the raw JSON format — alternation, optional groups, macros, and comments — across all Claude Code tool types.

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

**Pre-built binaries** — download the latest release from [GitHub Releases](https://github.com/tonyo/claude-perms/releases), then make it executable:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/tonyo/claude-perms/releases/latest/download/claude-perms-darwin-arm64 -o claude-perms
chmod +x claude-perms && sudo mv claude-perms /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/tonyo/claude-perms/releases/latest/download/claude-perms-darwin-amd64 -o claude-perms
chmod +x claude-perms && sudo mv claude-perms /usr/local/bin/

# Linux
curl -L https://github.com/tonyo/claude-perms/releases/latest/download/claude-perms-linux-amd64 -o claude-perms
chmod +x claude-perms && sudo mv claude-perms /usr/local/bin/
```

**Go install** (requires Go 1.21+):

```bash
go install github.com/tonyo/claude-perms@latest
```

**Build from source:**

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

## Supported tool types

Each key under `allow`/`deny` maps to a Claude Code tool name:

| YAML key | Claude Code tool | Example rule |
|---|---|---|
| `bash` | `Bash` | `Bash(git status *)` |
| `read` | `Read` | `Read(./src/**)` |
| `edit` | `Edit` | `Edit(./src/**)` |
| `write` | `Write` | `Write(./output/**)` |
| `webfetch` | `WebFetch` | `WebFetch(domain:example.com)` |
| `agent` | `Agent` | `Agent(Explore)` |
| `cd` | `Cd` | `Cd(./src/**)` |
| `powershell` | `PowerShell` | `PowerShell(Get-Location)` |
| `mcp__<server>` | `mcp__<server>` | `mcp__puppeteer(*)` |
| `mcp__<server>__<tool>` | `mcp__<server>__<tool>` | `mcp__puppeteer__puppeteer_navigate(*)` |

## Pattern syntax

| Syntax | Meaning | Expands to |
|---|---|---|
| `(a\|b\|c)` | Alternation | One rule per branch |
| `(foo)?` | Optional group | Rules with and without the group |
| `*` | Glob wildcard | Passed through as-is |
| `{{name}}` | Macro reference | Replaced with the macro's value before expansion |

Groups can be nested: `(git (push|pull)|npm) *`

A bare `?` not following a group is treated as a literal character.

Multiple consecutive spaces in an expanded pattern are collapsed to one, so optional groups that resolve to empty never leave double spaces.

## Macros

Macros let you define a pattern fragment once and reuse it across multiple rules. Define them under a top-level `macros:` key, then reference with `{{name}}`:

```yaml
macros:
  git_read: "status|log|diff|show"
  git_write: "push|reset|clean"

permissions:
  allow:
    bash:
      - "git ({{git_read}}) *"
      - "git ({{git_read}}) --cached *"
  deny:
    bash:
      - "git ({{git_write}}) *"
```

Macro values are validated at compile time — unclosed parentheses or other syntax errors are caught before any rules are written. Referencing an undefined macro is also an error.

Macros cannot reference other macros.

## Example

`perms.yaml`:

```yaml
macros:
  git_read: "status|log|diff|show|branch|stash"
  git_danger: "push|reset|clean|checkout"

permissions:
  allow:
    bash:
      - "git ({{git_read}}) *"
      - "git commit (--amend)? *"
      - "npm (run|exec) *"
      - "(ls|cat|head|tail|grep|wc) *"
    read:
      - "./src/**"
      - "./tests/**"
    webfetch:
      - "domain:docs.example.com"
    mcp__puppeteer:
      - "*"

  deny:
    bash:
      - "git ({{git_danger}}) *"
      - "(rm|sudo|curl|wget) *"
    write:
      - "/etc/**"
      - "~/**"
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
  Bash(git commit *)
  Bash(git commit --amend *)
  Bash(npm run *)
  Bash(npm exec *)
  Bash(ls *)
  ...
  Read(./src/**)
  Read(./tests/**)
  WebFetch(domain:docs.example.com)
  mcp__puppeteer(*)

Deny rules:
  Bash(git push *)
  Bash(git reset *)
  ...
  Write(/etc/**)
  Write(~/**)
```

`claude-perms compile --force perms.yaml` writes the expanded rules into `.claude/settings.json`, leaving all other keys (model, theme, etc.) untouched.

## How it works

The tool:
1. Parses the YAML input
2. Validates macro definitions and resolves `{{name}}` references
3. Expands each pattern using a recursive descent parser (handles nesting)
4. Normalizes spaces in expanded results (collapses runs from empty optional groups)
5. Wraps each result as `ToolName(pattern)` using the tool type from the YAML key
6. Reads the target `settings.json` (if it exists) using `map[string]json.RawMessage` to preserve unknown keys
7. Replaces only the `permissions` block
8. Shows a line-level diff and prompts before writing
9. Writes atomically (temp file + rename)
