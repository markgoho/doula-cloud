#!/bin/bash
# PreToolUse gate on `golangci-lint run`. It refuses two things: a local
# golangci-lint that is not the version ci.yml pins (#1409, see the check
# below), and a run without a scoped GOLANGCI_LINT_CACHE (#587, this
# header). Tested by scripts/gate-golangci-lint-cache.test.ts.
#
# golangci-lint's results cache defaults to one location shared by every
# worktree on the machine (~/.cache/golangci-lint / ~/Library/Caches/
# golangci-lint), keyed in a way that does not account for a worktree's
# path being reused or removed. A session linting after another worktree
# was pruned can get findings that point at files under a worktree that no
# longer exists on disk -- or, worse, a stale "clean" result that masks a
# real issue in the current worktree's own changed file (#587).
#
# The fix (docs/testing.md) is to always set GOLANGCI_LINT_CACHE to a path
# under `git rev-parse --show-toplevel`, so the cache lives inside the
# current worktree and is never shared with another one. This gate makes
# that the only way to invoke `golangci-lint run` from a session: Bash
# tool calls don't retain shell state between invocations (no persisted
# `export`), so a command that doesn't mention GOLANGCI_LINT_CACHE truly
# ran without it.
#
# Only the `run` subcommand is gated -- `cache clean`/`cache status`/
# `config verify`/etc. don't read or write the results cache in the way
# that causes this hazard.
#
# Fails open (allows) on anything that doesn't look like `golangci-lint
# run`, or if jq is unavailable -- this hook must never block an unrelated
# Bash command.
set -u

deny() {
	reason="$1"
	jq -n --arg reason "$reason" '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"deny",permissionDecisionReason:$reason}}'
	exit 0
}

allow() {
	printf '{}'
	exit 0
}

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null)" || allow
[ -n "$cmd" ] || allow

printf '%s' "$cmd" | grep -qE '(^|[[:space:];&|(])golangci-lint([[:space:]]+[^;&|]*)?[[:space:]]+run([[:space:]]|$)' || allow

# The local golangci-lint must be the version CI pins (#1409). Homebrew
# upgrades it on its own schedule, and a newer one turns on analyzers CI
# does not run, so a local result stops being evidence of what CI will
# say -- on 2026-09-20 this machine ran 2.13.2 against CI's v2.12.2. The
# pin is read from this checkout's own ci.yml: the first `version:` after
# the golangci-lint-action step. Either side unreadable (no golangci-lint
# on PATH, no ci.yml, the step moved) falls through to the cache check
# rather than blocking: a missing binary fails on its own, and a moved
# step is the repo's to notice, not a reason to stop every lint.
root="$(git rev-parse --show-toplevel 2>/dev/null)"
pinned="$(awk '/golangci\/golangci-lint-action@/ {seen=1} seen && /^[[:space:]]*version:/ {sub(/^[[:space:]]*version:[[:space:]]*v?/, ""); print; exit}' "$root/.github/workflows/ci.yml" 2>/dev/null)"
local_version="$(golangci-lint version --short 2>/dev/null)"
if [ -n "$pinned" ] && [ -n "$local_version" ] && [ "$local_version" != "$pinned" ]; then
	deny "The local golangci-lint is $local_version, but .github/workflows/ci.yml pins v$pinned, so this run cannot tell you what CI will say (#1409). If the newer version is the one to keep, set ci.yml's golangci-lint-action \`version:\` to v$local_version and fix what it reports in the same PR. If not, install v$pinned locally."
fi

printf '%s' "$cmd" | grep -q 'GOLANGCI_LINT_CACHE' && allow

deny "\`golangci-lint run\` without GOLANGCI_LINT_CACHE set shares its results cache with every worktree on the machine, including pruned ones -- see docs/testing.md. Run: GOLANGCI_LINT_CACHE=\"\$(git rev-parse --show-toplevel)/api/.golangci-cache\" golangci-lint run"
