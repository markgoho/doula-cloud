#!/bin/bash
# PreToolUse gate on `golangci-lint run` without a scoped GOLANGCI_LINT_CACHE.
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

printf '%s' "$cmd" | grep -q 'GOLANGCI_LINT_CACHE' && allow

deny "\`golangci-lint run\` without GOLANGCI_LINT_CACHE set shares its results cache with every worktree on the machine, including pruned ones -- see docs/testing.md. Run: GOLANGCI_LINT_CACHE=\"\$(git rev-parse --show-toplevel)/api/.golangci-cache\" golangci-lint run"
