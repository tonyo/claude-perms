# claude-perms

CLI tool that compiles a YAML permissions file into the `permissions` block of a Claude Code `settings.json`.

## Commands

```bash
go build -o claude-perms .       # build binary
go test ./...                    # run all tests
go test ./... -coverprofile=...  # with coverage
```

## Architecture

```
main.go                          # cobra root → Execute()
cmd/root.go                      # all cobra commands + core logic
internal/expand/                 # alternation/optional pattern expander
internal/yamlconf/               # YAML input loader
internal/settings/               # settings.json read/merge/write
internal/diff/                   # line diff display + y/N prompt
```

### Package responsibilities

- **`internal/expand`** — pure function `Expand(pattern) []string`. Implements a recursive descent parser for `(a|b|c)` alternation and `(x)?` optional syntax. No I/O.
- **`internal/yamlconf`** — `Load(path)` reads and unmarshals the input YAML into `PermissionsFile`.
- **`internal/settings`** — `ReadRaw`/`MergePermissions`/`Write` operate on `map[string]json.RawMessage` to preserve unknown keys in the settings file. Writes are atomic (tmp + rename).
- **`internal/diff`** — `Display` computes LCS-based line diff with optional ANSI color. `Prompt` reads y/N from an `io.Reader`.
- **`cmd/root.go`** — `runCompile` and `runCheck` accept `io.Reader`/`io.Writer` for testability; cobra commands wire in `cmd.InOrStdin()` etc.

## Pattern syntax

| Syntax | Meaning | Example |
|---|---|---|
| `(a\|b\|c)` | Alternation — one rule per branch | `git (status\|log) *` → 2 rules |
| `(foo)?` | Optional — rules with and without the group | `git commit (--amend)? *` → 2 rules |
| `*` | Glob wildcard, passed through as-is | `npm run *` |

Bare `?` not following a group is treated as a literal character.

## Input YAML shape

```yaml
permissions:
  allow:
    bash:
      - "git (status|log|diff) *"
      - "git commit (--amend)? *"
  deny:
    bash:
      - "(rm|sudo|curl) *"
```

## Test conventions

- Table-driven tests with `t.Run` in all internal packages
- `cmd/` tests call `runCompile`/`runCheck` directly with `strings.NewReader`/`strings.Builder` — no process spawning
- Regression tests in `cmd/regression_test.go` document specific bugs with the triggering scenario in a comment
- `cmd/testhelpers_test.go` provides `writeTempYAML` shared across cmd tests
