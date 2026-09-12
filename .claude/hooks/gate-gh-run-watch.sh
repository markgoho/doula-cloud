#!/bin/bash
# PreToolUse gate on `gh run watch` (#1207).
#
# `gh run watch --exit-status` exits 0 on a canceled run -- a canceled
# run is neither a pass nor a fail to the shell, but the exit code alone
# can't say which, and every close-out that trusted it has reported a
# cancellation as a pass. This happened repeatedly enough (see
# docs/agents/worktree-flow.md's "What a green PR does not prove") that
# it's now enforced rather than left to be remembered: `gh run watch` is
# refused outright, in favor of reading a run's `conclusion` field
# directly.
#
# Fails open (allows) on anything that doesn't look like `gh run watch`,
# or if jq is unavailable -- this hook must never block an unrelated
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

printf '%s' "$cmd" | grep -qE '(^|[[:space:];&|(])gh[[:space:]]+run[[:space:]]+watch([[:space:]]|$)' || allow

deny "\`gh run watch\` exits 0 on a canceled run, so it cannot tell a cancellation from a pass -- see docs/agents/worktree-flow.md. Poll and read \`conclusion\` directly instead, e.g.: gh run list --branch trunk --limit 5 --json databaseId,conclusion,status; gh api repos/<owner>/<repo>/actions/runs/<id> --jq '.conclusion'"
