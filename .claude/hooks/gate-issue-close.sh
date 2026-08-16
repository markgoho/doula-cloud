#!/bin/bash
# PreToolUse gate on `gh issue close`. Blocks unless mark-ac-verified.sh
# already recorded (this session or any other, in any worktree of this
# repo -- the sentinel lives under the shared .git dir) that a `gh issue
# edit ... --body*` ran for the same issue number. That's how we know the
# acceptance-criteria checkboxes were actually re-edited, not just
# narrated in a closing comment. See feedback_implement_close_issue
# memory: a closing comment that *describes* AC completion is not the
# same as checking the boxes.
#
# The sentinel is consumed (deleted) on a successful check, so closing
# the same issue again later requires a fresh --body edit first --
# re-verification can't be satisfied once and reused indefinitely.
#
# Fails open (allows) on anything that isn't clearly a `gh issue close`
# call, or if jq/git aren't available -- this hook must never block
# unrelated Bash commands.
set -u

deny() {
	reason="$1"
	jq -n --arg reason "$reason" '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"deny",permissionDecisionReason:$reason}}'
	exit 0
}

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null)" || { printf '{}'; exit 0; }
[ -n "$cmd" ] || { printf '{}'; exit 0; }

printf '%s' "$cmd" | grep -qE '(^|&&|;)[[:space:]]*gh issue close[[:space:]]' || { printf '{}'; exit 0; }

issue="$(printf '%s' "$cmd" | grep -oE 'gh issue close[[:space:]]+[0-9]+' | grep -oE '[0-9]+' | head -1)"
if [ -z "$issue" ]; then
	printf '{}'
	exit 0
fi

common_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
if [ -z "$common_dir" ]; then
	printf '{}'
	exit 0
fi
sentinel="$common_dir/claude-ac-verified/$issue"

if [ -f "$sentinel" ]; then
	rm -f "$sentinel"
	printf '{}'
	exit 0
fi

deny "Before closing issue #$issue: re-verify each acceptance-criteria checkbox against the code actually committed on HEAD (git show HEAD:<path>, rerun the relevant test/coverage command) -- not against memory of earlier work this session. Then edit the issue body so each verified '- [ ]' becomes '- [x]' via: gh issue edit $issue --body-file <path>, preserving the rest of the body verbatim. Only then retry gh issue close."
