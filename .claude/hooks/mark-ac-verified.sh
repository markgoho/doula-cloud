#!/bin/bash
# PostToolUse companion to gate-issue-close.sh: when a `gh issue edit`
# command carrying --body or --body-file runs, record that the issue's
# acceptance-criteria checkboxes were (presumably) just re-verified and
# edited, so the matching `gh issue close` is allowed to proceed once.
#
# Fails open on any unexpected condition (no jq, not a git repo, no
# match) -- this script must never itself break an unrelated Bash call.
set -u

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null)" || exit 0
[ -n "$cmd" ] || exit 0

printf '%s' "$cmd" | grep -qE '(^|&&|;)[[:space:]]*gh issue edit[[:space:]]' || exit 0
printf '%s' "$cmd" | grep -qE -- '--body(-file)?([[:space:]=]|$)' || exit 0

issue="$(printf '%s' "$cmd" | grep -oE 'gh issue edit[[:space:]]+[0-9]+' | grep -oE '[0-9]+' | head -1)"
[ -n "$issue" ] || exit 0

common_dir="$(git rev-parse --git-common-dir 2>/dev/null)" || exit 0
sentinel_dir="$common_dir/claude-ac-verified"
mkdir -p "$sentinel_dir" 2>/dev/null && touch "$sentinel_dir/$issue" 2>/dev/null

exit 0
