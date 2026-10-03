# Worktree flow: the default way to land a change

One worktree per unit of work, landed by a squash-merged PR, never a direct push to `trunk`. Several Claude Code sessions work against this repo at once, and a worktree gives each one its own working tree, index and `HEAD`. Why each step is shaped the way it is, what gets provisioned, what enforces it and how cleanup works: [`worktree-flow-reference.md`](worktree-flow-reference.md).

## The procedure

1. **Enter.** `EnterWorktree` (a hook provisions `app/.env.local`, `node_modules` and a `.port-offset`). To resume a branch that already exists, see the reference's "Resuming an existing branch". Every edit to a tracked file happens in the worktree: hooks refuse an `Edit`, `Write` or Bash write to one in the main checkout (the reference's "Enforcement").
2. **Name the branch** `<type>/<issue>-<description>`, e.g. `fix/510-labeled-field-inline-row`: `git branch -m fix/510-labeled-field-inline-row`.
3. **Work and commit** with explicit pathspecs; the pre-commit gate runs. `TZ=UTC git commit -F <message file> -- <paths>`. Running specs and the local stack: [`docs/testing/walk-a-screen-locally.md`](../testing/walk-a-screen-locally.md).
4. **Rebase and push.** `git fetch origin trunk`, then `git rebase origin/trunk`, then `git push -u origin HEAD`.
5. **Open the PR** with a body you wrote as unwrapped Markdown, with `Closes #<issue>` in it: `gh pr create --title "<title> (#<issue>)" --body-file <file>`.
6. **Queue the merge.** `gh pr merge --squash --auto`.
7. **Wait for the merge** by running this as a separate tool call about once a minute, never in a shell loop:

   ```sh
   gh pr view <PR> --json state,mergeStateStatus
   ```

   `state` becomes `MERGED`. If every required check has passed, `mergeStateStatus` is `CLEAN`, and the PR is still `OPEN` a few minutes later, auto-merge has stalled: land it with `gh pr merge <PR> --squash`.
8. **Wait for the trunk run** when the change touches what only trunk runs (`migrate`, `deploy-api`, `deploy-app`, the site deploy). Each command is its own tool call; read `conclusion`, never the exit status of `gh run watch`, which exits 0 on a canceled run:

   ```sh
   gh run list --branch trunk --limit 5 --json databaseId,conclusion,status,headSha
   gh api repos/markgoho/doula-cloud/actions/runs/<id> --jq '.conclusion'
   ```

   A canceled run proves nothing; the next run whose head contains your merge does. The reference's "What a green PR does not prove" says how to tell. A red run: [`docs/testing/gates.md`](../testing/gates.md), "Reading a failed CI run".
9. **Leave.** `ExitWorktree` once the PR reads `MERGED`. The pruner collects what a dead session left; never remove another session's worktree by hand (the reference's "A live worktree is never removed").

## Forms the worktree isolation accepts

Inside a worktree-isolated session the Bash tool refuses a command whose shape it cannot verify stays inside the worktree, whatever the command actually touches. `.claude/hooks/gate-worktree-bash-shape.ts` refuses the common shapes first, and its message names the form that passes ([#1678](https://github.com/markgoho/doula-cloud/issues/1678)); it also refuses `gh pr create --fill` in every session. The forms that pass:

- One plain command per tool call, with absolute paths. Git runs in a call of its own, never as `git -C <path>` or `cd <dir> && git …`; the Bash tool already starts in the worktree.
- Variables in front of the command they are for (`X=v cmd`), never `export`.
- File content written with the Write tool, then the command run on the file (`git commit -F <file>`, `--body-file <file>`), never a heredoc.
- A `gh api` body edit as a JSON file passed with `gh api --input <file>`, not `-f body="$(cat …)"`; `gh issue edit --body-file <file>` for an issue body.
- A `--jq` filter of plain paths, joined when you need several (`--jq '.a + " " + .b'`, `(.n | tostring)` for a number); no `\(...)` template and no `{...}` object.
- No `sleep N` followed by a command. Poll with separate repeated tool calls (steps 7 and 8); a loop that has to run unattended goes in a script run as `python3 <file>.py`.
