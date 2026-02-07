# Static Analysis & Dead Code Cleanup

Repeatable process for finding and removing unused code in the ashlet codebase.

## Prerequisites

```bash
# Go tools
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/tools/cmd/deadcode@latest

# Shell tools (macOS)
brew install shellcheck
```

Ensure `~/go/bin` is in your `$PATH`, or invoke with full paths.

## Step 1: Run Go static analysis

```bash
go vet ./...
staticcheck ./...
deadcode ./...
```

- `go vet` — catches common mistakes (shadowed vars, printf misuse, etc.)
- `staticcheck` — broader lint: unused code (SA), simplifications (S), style (ST)
- `deadcode` — reports functions unreachable from any `main` entrypoint

**Note:** `deadcode` flags functions only used in tests as unreachable. Before removing, decide whether the function is a public API meant for external consumers or just dead weight.

## Step 2: Fix Go findings

For each `deadcode` finding:

1. Grep for all references: `grep -rn 'FunctionName' --include='*.go'`
2. If only referenced in its own definition and tests, it's dead — remove the function and its tests.
3. If tests call it via a convenience wrapper (e.g. `CheckModels()` wrapping `CheckModelsFromConfig(nil)`), update tests to call the underlying function directly, then remove the wrapper.
4. Run `go build ./...` after edits to catch unused imports.

For `staticcheck` findings, follow the suggestion in each diagnostic (e.g. SA4006 = unused assignment, S1000 = simplifiable code).

## Step 3: Run shellcheck on shell scripts

```bash
# Full support — .sh files
shellcheck _run.sh shell/_run.sh

# Approximate — .zsh files (many false positives from zsh syntax)
shellcheck --shell=bash --exclude=SC1071,SC1091,SC2034,SC2154 shell/**/*.zsh
```

Fix findings in `.sh` files. For `.zsh` files, review manually — most errors about `${(@)...}`, `${0:A:h}`, `[#10]` arithmetic, and `local` outside functions are false positives from zsh-specific syntax that shellcheck doesn't support.

**Common real fixes in .sh files:**
- SC2181: Replace `cmd; if [ $? -ne 0 ]` with `if ! cmd`
- SC2155: Split `local x="$(cmd)"` into `local x; x="$(cmd)"`

## Step 4: Find dead shell functions

```bash
# List all function definitions
grep -rn '^\.ashlet:[a-z-]*()' shell/ --include='*.zsh'

# For each function, check if it's called anywhere outside its definition
grep -rn 'function-name' shell/ --include='*.zsh' --include='*.bats'
```

If a function only appears at its definition (and optionally in test files but never in production code), it's dead. Remove the function and its associated tests.

## Step 5: Verify

```bash
go build ./...        # Compilation
go test ./...         # Go tests
make test             # Go + bats tests
make lint             # go vet + staticcheck + shellcheck
deadcode ./...        # Should print nothing
```

All commands should exit cleanly with no errors or findings.

## Makefile integration

The `make lint` target runs all Go and shell linters:

```makefile
lint:
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not found, skipping"; \
	fi
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck _run.sh shell/_run.sh; \
	else \
		echo "shellcheck not found, skipping shell lint"; \
	fi
```

`deadcode` is not in the Makefile because it flags test-only functions, which requires human judgement. Run it manually during cleanup sessions.
