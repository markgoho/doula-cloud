#!/bin/bash
# PreToolUse gate on the two git commands that go wrong in a shared
# working tree. This repo is worked on by several Claude Code sessions at
# once, against ONE checkout: the working tree, the index and HEAD are all
# visible to and mutable by every other session, in real time.
#
# Two commands are refused here, each for a failure that has actually
# happened rather than a hypothetical one:
#
#   git add -A / --all / -u / . / (no pathspec)
#     Stages whatever is in the tree at that instant, which includes files
#     another session is midway through writing. On #512 this swept three
#     of another session's brand-new files into a commit; unstaging and
#     re-committing swept them in again, because more had been staged in
#     the seconds between. The index is the shared thing -- so never
#     compose a commit through it.
#
#   git commit --amend
#     Rewrites whatever HEAD points at NOW, which is not necessarily the
#     commit you made. On #516 another session ran its own reset between
#     this session's `reset --soft HEAD~1` and its `--amend`, so the amend
#     consumed that session's just-landed commit and erased it from the
#     log. HEAD is not stable between two of your own commands.
#
# `git add <explicit path>` is left alone: it names what it stages, so it
# cannot pick up a file it was not pointed at.
#
# Fails open (allows) on anything that is not clearly one of those two
# calls, or if jq/git are unavailable -- this hook must never block an
# unrelated Bash command.
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

# One segment per line, so a `git add` written inside a commit message is
# not mistaken for a `git add` being run. Only a segment whose own first
# words are `git add` / `git commit` is ever examined.
segments="$(printf '%s' "$cmd" | tr '\n;|&' '\n\n\n\n')"

while IFS= read -r segment; do
	# Strip leading whitespace and any `(`/`{` left by a subshell split.
	segment="$(printf '%s' "$segment" | sed -E 's/^[[:space:](){]*//')"

	case "$segment" in
	'git add'*)
		arguments="$(printf '%s' "$segment" | sed -E 's/^git add[[:space:]]*//')"
		# Anything that is not a flag is a pathspec, which is the form
		# this gate wants people to use.
		pathspec="$(printf '%s' "$arguments" | tr ' \t' '\n\n' | grep -vE '^-' | grep -vE '^\.$|^\*$|^:/$|^$' | head -1)"
		global="$(printf '%s' "$arguments" | grep -oE '(^| )(-A|--all|-u|--update)( |$)' | head -1)"
		if [ -n "$global" ] || [ -z "$pathspec" ]; then
			deny "This repo is worked on by several sessions against one shared index, so \`git add\` with no pathspec (or with -A/--all/-u/.) stages whatever another session happens to have in the tree right now -- it has swept other sessions' files into commits before. Build the commit from explicit paths instead, without touching the index at all: git commit -F <message-file> -- path/one path/two. Then confirm with: git show --stat HEAD."
		fi
		;;
	'git commit'*)
		if printf '%s' "$segment" | grep -qE '(^| )--amend( |$)'; then
			deny "\`git commit --amend\` rewrites whatever HEAD points at now, and in this shared checkout HEAD can move between two of your own commands -- an amend here has already swallowed another session's commit. To fix a bad commit, anchor on the ref another session cannot move: git reset --soft origin/<branch> (never HEAD~1), then re-commit with explicit paths via git commit -F <message-file> -- <paths>. Re-read git log --oneline -3 before and after."
		fi
		;;
	esac
done <<EOF
$segments
EOF

allow
