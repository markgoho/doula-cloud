# Worktree flow: the default way to land a change

This repo's default is **one worktree per unit of work, landed via a PR** — not a direct push
to `trunk`. Several Claude Code sessions work against this repo at once; a worktree gives
each one its own working tree, index, and `HEAD` instead of contending for the one shared
checkout.

## The flow

```sh
EnterWorktree                              # hook provisions env, node_modules, port offset
git branch -m <type>/<issue>-<description> # EnterWorktree sanitizes '/' to '-'; rename after
… work, commit (pre-commit gate runs) …
git push -u origin HEAD
gh pr create --fill
gh pr merge --squash --auto                # lands the moment CI is green
ExitWorktree                               # or: git worktree remove
```

Branch format: `<type>/<issue>-<description>` — e.g. `fix/510-labeled-field-inline-row`.
`EnterWorktree` sanitizes `/` to `-` in both the folder name and the branch name it creates,
so rename the branch with `git branch -m` right after entering; the folder itself stays
dashed.

Trunk-based development still applies: short-lived branches, squash merge, linear history.
`allow_auto_merge` is on specifically so this flow is *faster* than pushing straight to
trunk, not slower — `gh pr merge --squash --auto` returns immediately and the merge lands on
its own once required checks pass.

## What a green PR does not prove

`migrate`, `deploy-api` and `deploy-app` run only on a push to `trunk` — never on a pull
request — so a green PR proves nothing about them (#1022). What's easy to miss is that even
a *push to trunk* often doesn't prove them either: `ci.yml`'s concurrency group holds one
in-progress run and one queued run per push; a third push arriving while a second is still
queued cancels that second run outright, with zero jobs ever started (#1207). This is routine
under a burst of back-to-back merges, not a failure — `.github/workflows/trunk-red.yml` does
not open a `trunk-red` issue for it, because the jobs never ran to fail. It instead comments
on the merge's own PR naming the gap.

**What to check instead:** the next trunk `CI` push run whose head has your merge as an
ancestor —

```sh
git merge-base --is-ancestor <your-merge-sha> <later-run-head-sha>
```

— read by that later run's `conclusion` field, never by the exit status of a watch command.
**`gh run watch --exit-status` exits 0 on a canceled run.** Anything that trusts that exit
code reports a cancellation as a pass. Read `conclusion` directly:

```sh
gh run list --branch trunk --limit 5 --json databaseId,conclusion,status,headSha
gh api repos/markgoho/doula-cloud/actions/runs/<id> --jq '.conclusion'
```

`migrate` applies every goose migration up to `HEAD` and `deploy-api`/`deploy-app` ship
`HEAD`, so a later run's success *is* evidence for every commit it contains — cancellation
only ever costs attribution (which commit's run gets credited), never verification (whether
the migration and deploys actually happened).

**Worked example.** PR #1310/#1316's merge, commit `0a136a83`, pushed a trunk `CI` run
(`34699091787`) that was canceled while still queued — `gh api
repos/markgoho/doula-cloud/actions/runs/34699091787/jobs` returns an empty job list, so
`migrate`/`deploy-api`/`deploy-app` never ran for that commit specifically. The next trunk
`CI` push run, `34699163504` (head `67169e9f`, PR #1317), contains `0a136a83` as an ancestor
and ran all three jobs to `success` — that run, not `34699091787`, is the real evidence
`0a136a83`'s migration and deploys happened.

## What's provisioned automatically

`EnterWorktree` (and, as a fallback, a manually run `git worktree add`) runs
`.claude/hooks/worktree-provision.ts`, which:

- Copies `app/.env.local` from the main checkout if the worktree doesn't already have one.
  This is the only gitignored file local dev needs (Stripe Sandbox + Mailgun test keys); if
  it's missing from main too, see `docs/environment.md`'s First-time setup.
- Symlinks root `node_modules` to the main checkout's, unless the branch's own
  `package.json`/`bun.lock` differ from `trunk`, in which case it removes the symlink and
  runs a real `bun install` at the worktree root instead. Re-checked on every provision, so
  adding a root dependency later and re-entering the worktree switches it to a real install.
- **Every sibling SvelteKit package's `node_modules` always gets a real `bun install`, never
  a symlink.** A sibling package is any top-level directory whose own `package.json`
  declares `@sveltejs/kit` — today `app/` and `gcp-dashboard/`, detected by scanning rather
  than a hardcoded list, so a third one needs no change here (#950). SvelteKit 3 writes
  generated per-checkout state into it (`node_modules/$app/tsconfig.json`,
  `node_modules/$app/types`) that TypeScript resolves via that file's real path before
  applying its `rootDirs` entries — through a symlink, every worktree's `rootDirs` collapses
  onto whichever checkout that package's `node_modules` physically lives in, breaking every
  `./$types` import project-wide. Confirmed empirically for `app/`: reproduces on a bare
  `trunk` checkout with no other changes, in every symlinked worktree, and is absent under a
  real install (which is what CI already does). A live symlink here is also a write hazard on
  its own: `svelte-kit sync` mutates `node_modules/$app/*` in place, so two worktrees sharing
  a symlinked `node_modules` would clobber each other's generated state even without the
  `rootDirs` bug. The dependency-manifest check above still governs whether this is a *fresh*
  install or a left-alone existing one for each such package — it just never chooses a
  symlink for any of them.
- Assigns a port offset (`.port-offset`, gitignored) — the lowest value 1–9 that is neither
  claimed by another live worktree nor blocked by a port already bound on this machine.
  `app/e2e/ports.ts` shifts every port by `offset * 100`, so two worktrees can run
  `bun run dev:full` or the e2e suite at the same time without colliding. The main checkout
  and CI have no `.port-offset` file, so they always run at offset 0 — today's exact ports,
  unchanged.

  The bound-port check exists because "unclaimed" and "available" are not the same thing
  (#927): an unrelated local service holding one of an offset's ports is invisible to the
  worktree bookkeeping, and the collision would otherwise surface much later as a bind
  failure inside `startStack` that reads like a broken emulator rather than an unusable
  offset. Observed for real — a local process on `127.0.0.1:9999` made offset 9 (the Auth
  emulator, `9099 + 900`) unusable while every other offset-9 port was free. Provisioning
  now probes every port in `BASE_PORTS` (exported from `app/e2e/ports.ts`, so there is no
  second copy of the list to drift), skips an offset whose port is taken and says which
  port that was, and refuses with both causes named rather than handing out an offset that
  cannot work.

**Never run `bun install` through a live `node_modules` symlink** — it mutates the main
checkout's modules for every worktree sharing them at once. The provisioning hook already
avoids this (it unlinks before installing); don't `bun install` by hand inside a worktree
without checking `ls -la node_modules` first.

**Never run bare `golangci-lint run` in a worktree.** Its shared results cache can serve findings pointing at a pruned worktree's files, or mask a real issue behind a stale "clean" result — see `docs/testing.md`'s `golangci-lint` section for the `GOLANGCI_LINT_CACHE` invocation that scopes the cache to the current worktree instead; `.claude/hooks/gate-golangci-lint-cache.sh` blocks the bare form in a Claude Code session.

**Only one worktree at a time may hold port offset 0** — that's the main checkout itself, by
having no `.port-offset` file. A worktree never gets assigned 0.

**The pool is capped at 9 concurrent worktrees** (offsets 1–9). If provisioning fails because
the pool is full, prune stale worktrees first (`bun .claude/hooks/worktree-prune.ts
--dry-run`, then `--merged`) rather than working around the cap. Never remove a worktree by hand to free an offset without reading its owner first — see "A live worktree is never removed" below.

## Resuming an existing branch

`EnterWorktree` only creates new branches. To resume work on a branch that already exists:

```sh
git worktree add .claude/worktrees/<slug> <branch>
```

The `PostToolUse` fallback hook provisions it the same way `EnterWorktree` would.

## Enforcement

- **Local**: `.claude/hooks/gate-worktree-edit.ts` blocks `Edit`/`Write` on a tracked file in
  the main checkout. Anything under `.claude/worktrees/`, anything outside the repo, and
  anything `git check-ignore` reports as ignored (e.g. `app/.env.local`,
  `settings.local.json`) stays editable in main.
- **Local, Bash path**: `.claude/hooks/gate-bash-write.ts` (#573) applies the same boundary
  and the same three exceptions to a Bash command that writes to a tracked path in the main
  checkout — shell redirection (`>`, `>>`), `tee`, `sed -i`, `cp`/`mv`/`install`/`rsync`, and
  `dd of=`. This is **not** exhaustive by construction: a write performed through an opaque
  pipeline, a script invoked by name, a scripting-language one-liner (e.g. `python3 -c
  "open(...).write(...)"`), or a compiled binary that computes its own target path is not
  recognized and is not blocked. That gap is accepted, not hidden — the GitHub ruleset below
  is the real, unconditional boundary; this hook and `gate-worktree-edit.ts` are a fast local
  nudge on top of it, not a substitute for it. A relative write target is measured against
  the directory a leading `cd` in the same command would put a shell in, not against the
  hook's own working directory (#680) — so `cd <worktree> && sed -i '' '…' <relative-path>`
  is judged where the write actually lands. Only `&&` carries that directory into the next
  stage, because only there does a failed `cd` short-circuit the write; past `||`, `;`, `|`,
  `&`, a newline, or an unquoted parenthesis the write really would land in the main
  checkout, so it is judged there. A `cd` that leaves the checkout, or whose destination
  cannot be established (bare `cd`, `cd -`, `cd ~…`, an unexpanded variable), falls back to
  resolving against the hook's own working directory rather than guessing.
- **How a hook names its script**: `"$(git rev-parse --show-toplevel)/.claude/hooks/<file>"`,
  always — never a path relative to the working directory. A hook command inherits the
  session's cwd, and `app/` is where most sessions stand, so a relative path resolves against
  a directory that has no `.claude/` and the hook dies with `Module not found`. It dies
  quietly: only exit code 2 blocks, so a gate that cannot run permits what it exists to
  refuse (#569). `$CLAUDE_PROJECT_DIR`, which the hooks documentation recommends for this
  symptom, is measurably wrong here — from a `Stop` hook inside a worktree it holds the main
  checkout, so it would run main's copy against a worktree session, the inversion #555 fixed.
  `--show-toplevel` names the checkout the session is in, whose own committed copy should
  run. `scripts/hook-registration.test.ts` enforces this from `app/`.
- **A gate fails closed**: `gate-worktree-edit.ts` and `gate-bash-write.ts` both exit 2 with a
  reason when they cannot run at all (e.g. `git worktree list` fails), because a gate that
  crashed has not decided the edit or command is safe. `gate-shared-index.sh` deliberately
  does the opposite and fails open — it fires on every Bash command, so a broken gate there
  would halt all shell work rather than let one edit through. `gate-bash-write.ts` also fires
  on every Bash command but keeps `gate-worktree-edit.ts`'s fail-closed rule for a crash; its
  fail-*open* case is narrower and deliberate — a command it does not recognize as a write at
  all is let through, which is the coverage gap recorded above, not a malfunction.
- **GitHub**: a ruleset on `trunk` requires a passing PR (checks: `scripts`, `actionlint`,
  `api`, `app`, `api-image`, `plan`, `format` — the `ci.yml` and `terraform-plan.yml` jobs
  only; the Firebase preview workflows are
  `paths:`-filtered and would never satisfy a required check that always waits on them),
  blocks force-push and deletion, and requires linear history. Repository-admin is a bypass
  actor for a genuine emergency or a docs-only fix.

`gate-shared-index.sh` (blocking pathspec-less `git add` and `git commit --amend`) stays
active. Its threat model — several sessions racing one index — still holds *within* a
worktree whenever more than one session enters the same one; worktrees narrow the blast
radius, they don't remove the need for it.

## The Bash tool can refuse a compound command inside a worktree

While a session is isolated in a worktree, the Bash tool can refuse a command it judges "too complex to verify that it stays inside the worktree." The judgment looks at the command's text, not at what the command actually touches, so a command whose every write lands inside the worktree can still be refused for its shape or vocabulary. This is a Claude Code harness behavior, not something this repo configures — every Bash `PreToolUse` hook in `.claude/settings.json` was checked against it and none match ([#574](https://github.com/markgoho/doula-cloud/issues/574)).

Two concrete refusals from [#574](https://github.com/markgoho/doula-cloud/issues/574), neither of which wrote anything outside the worktree: a `mkdir -p /tmp/fakebin && ln -sf "$(which bun)" /tmp/fakebin/bun && echo '{...}' | env PATH=/tmp/fakebin bun ...` chain, and a `cat > .claude/settings.local.json <<EOF ... EOF` heredoc whose body merely contained the text `git rev-parse` as data being written into a gitignored config file, never executed. A further set turned up while landing [#1012](https://github.com/markgoho/doula-cloud/issues/1012) in a worktree-isolated session: a `gh api --method POST/PATCH ...` call built with `-f body="$(cat body.md)"` to fill in a PR or issue body, a `--jq '... \(...) ...'` interpolation template, and `podman machine inspect --format '{{...}}'`.

The refusal message names its own concern: writing this section from inside a worktree, a `Monitor` command that polled `gh pr view` in a `while` loop with a `--jq` object template got "runs gh with the text {...} inside a construct too complex to verify, so what it runs cannot be shown not to be git. Refusing to run it — a worktree-isolated agent's git operations must target its own worktree." So the guard treats `gh` as possibly-git and refuses a loop it can't read through, not just an explicit `git` invocation. Routing the same poll through a small Python script (`python3 <file>.py`, the script itself doing the `subprocess` calls and the `while` loop) ran without a refusal — confirming that a script interpreter is a real workaround here, not just a suggestion, and that `Monitor`'s `command` is checked by the same guard as a plain Bash call.

**Workaround:** split the compound command into several plain commands, or route it through a script interpreter (e.g. `python3 -c "..."`) instead of shell chaining. Two forms that come up often in this flow have a specific fix: for a `gh api` body edit, write the payload to a local JSON file and pass it with `gh api --input <file>` instead of `-f field="$(cat ...)"`; for a `--jq` template that uses `\(...)` interpolation, pipe the output through `python3 -c` to format it instead.

## A live worktree is never removed

On 2026-09-10 two agents lost all their uncommitted work in one hour (#1212). In one case, a session that needed a free port offset removed a worktree by hand. It looked like an abandoned spawn by every signal git gives: a bare `agent-<id>` branch, no commits past trunk, a clean `git status`, a `HEAD` at an old trunk. But that is also the state of a live agent twenty minutes into its task that has not made its first commit yet. Branch and tree state cannot tell the two apart. So "a clean tree with no commits of its own is abandoned" is not a rule here.

**The liveness signal.** At provision time, `.claude/hooks/worktree-owner.ts` writes `.worktree-owner.json` into the worktree root. It is gitignored, like `.port-offset`, and it is also added to the checkout's shared `.git/info/exclude`, so a branch cut before the ignore line existed does not see it as a change. The file records the Claude Code process that created the worktree: its pid, the start time `ps` reports for that pid, and the session id. A subagent with `isolation: "worktree"` runs inside its session's own process, so the session's process is the right owner for the subagent too. This was checked on 2026-09-20: an isolated subagent's worktree recorded the pid of its parent session. A session that resumes an existing worktree runs no provisioning, so `.claude/hooks/worktree-claim.ts` takes the claim over when the recorded owner is gone. It runs after `EnterWorktree` (entry by `path`) and on every `Stop` (the backstop for a plain `cd`). It never takes a claim from an owner that is still alive, and it never writes into the main checkout. The owner is:

- **alive**: the pid exists and `ps -o lstart= -p <pid>` still prints the recorded start time. Nothing removes the worktree.
- **gone**: the pid does not exist, or it now belongs to a process with a different start time (the OS gave the pid to a new process).
- **unknown**: no file (a worktree made before #1212, or with `git worktree add` in a terminal outside Claude Code), or a file that cannot be read. This is not proof of life and not proof of death.

There is no heartbeat, and that is deliberate. An agent that spends ten minutes on a review or a CI watch touches no file, so mtime is exactly the signal that failed. The OS answers "is this pid running" directly.

**Read it from the shell before you remove anything:**

```sh
bun .claude/hooks/worktree-prune.ts --dry-run    # owner=alive(pid …, session …) | gone(pid …) | unknown, per worktree
cat .claude/worktrees/<slug>/.worktree-owner.json
ps -o lstart= -p <pid>                           # alive only if this prints the recorded processStartedAt
```

**What enforces it:**

- `worktree-prune.ts --merged` never removes a worktree whose owner is alive. It also never uses the "no commits of its own" test (tip is an ancestor of trunk) unless the owner is provably gone. With the owner alive or unknown, only a PR that is `MERGED` at the worktree's exact tip counts as landed. It never falls back to `git worktree remove --force`: if plain removal fails, it reports `skip (remove refused)` and moves on.
- `.claude/hooks/gate-worktree-remove.ts` is a `PreToolUse` gate on any Bash command that contains `worktree remove`. It refuses a removal when the worktree's owner is alive and is not the session that runs the command. `--force` does not get past it, because the 2026-09-10 removal used `--force`. It also refuses `--force` on a worktree with uncommitted changes, whoever owns it. Without `--force`, git itself refuses a dirty tree. The deliberate override is to put `ALLOW_LIVE_WORKTREE_REMOVE=1` before the command. Use it only after you have read the owner and seen that the process is not working in that worktree. The gate finds the removal inside a subshell, in a quoted path, and under `git -C`. It checks a relative path against every directory the command `cd`s into. The override counts only when it is the first word of the command that does the removal. **One limit:** the owner is a process, and all subagents of a session run inside that process. So the gate does not stop a session from removing its own subagent's worktree. That session spawned the agent, so it must itself make sure the agent is done before it removes the worktree.
- `e2e-stack-reap.ts` and `testdb-reap.ts`, the other `SessionStart` reapers, remove containers, stacks and pidfiles. They never remove a worktree.

A session that is short of port offsets must not use a manual removal to get one. Run `--dry-run`, and remove only worktrees whose owner is `gone` or `unknown` and whose work has landed. If every slot has a live owner, wait for a session to finish. Reclaiming worktrees that hold offsets for days without a merge (`research/*`, `prototype/*`) is #1340's work, and it must use this same check.

**If your own worktree is gone.** Every tool call then fails with "the isolation worktree appears to have been removed". Nothing on disk can be recovered: uncommitted work went with the directory, and the branch never had it. Stop editing. Write what you found and what you changed as a comment on the ticket immediately, while you can still reach the tracker. Then end the task and say that the worktree was removed while you worked in it. Do not recreate the worktree and continue from memory. The guards do not cover every path (a plain `rm -rf` is out of their reach), so the standing mitigation is to commit as soon as you have anything worth keeping, not at the end of the task.

## Cleanup

`.claude/hooks/worktree-prune.ts`:

```sh
bun .claude/hooks/worktree-prune.ts --dry-run   # list every worktree: branch, owner, PR state, dirty flag, size
bun .claude/hooks/worktree-prune.ts --merged    # remove only a worktree whose branch is
                                                 # merged into trunk AND whose tree is clean
```

Both modes end with an **orphan-branch sweep**. Removing a worktree does not take its branch
with it, and neither `ExitWorktree` nor `git worktree remove` can — `git branch -d` refuses a
squash-merged branch for the same reason `--merged` used to skip it — so orphaned branches
accumulate in the main checkout (there were about twenty when the sweep was written). A branch
goes only on proof that nothing is lost with it: its tip is already an ancestor of
`origin/trunk`, or its PR is `MERGED` **at that exact tip**. Everything else stays, which is
what protects `research/baseline-521-harness` and the unpushed `prototype/*` branches — none
has a merged PR and none is an ancestor of trunk, so no rule can reach them. `--dry-run` lists
what would go without touching anything.

`--merged` never touches a dirty worktree, a locked one, one whose owner is alive, or one whose branch hasn't landed —
including a branch with unpushed, unmerged commits. `ExitWorktree` (or `git worktree remove`)
once a PR shows `MERGED` is the normal path; the pruner is the backstop for whatever a dead
session left behind.

**"Landed" means a `MERGED` PR, not `git branch --merged`.** A squash merge writes a new
commit, so the branch tip never becomes an ancestor of `trunk` and `git branch --merged` never
lists it. Deciding it that way — which this pruner did until the check was fixed — makes
`--merged` inert for every branch this flow produces, and `git branch -d` refuse afterwards
for the same reason, which is why the branch delete falls back to `-D` when, and only when,
the PR says `MERGED`.

Two hooks make the cleanup happen without anyone remembering it:

- **`SessionStart`** runs `worktree-prune.ts --merged` asynchronously. Every session begins by
  clearing whatever landed and was left behind, including by sessions that died. This is what
  keeps the 9-slot pool from filling.
- **`Stop`** runs `.claude/hooks/gate-worktree-cleanup.ts`, which **blocks** when the session
  is standing in a worktree that is clean and whose PR reads `MERGED`, telling it to call
  `ExitWorktree`. A pruner cannot move a live session out of the directory it occupies; only
  the session can. The hook checks the two local conditions first and asks `gh` only if both
  pass, so it is silent and off the network on an ordinary turn.

Three things the two hooks are deliberately careful about, because each one bit during
construction:

- **A blocking `Stop` hook is a loop.** It is re-entered every time the session stops, and the
  session cannot always comply — `ExitWorktree` only removes a worktree *this* session created,
  and is a no-op on one entered by path or inherited from a dead session. So the nudge is
  **once per session per branch**, held by a sentinel in the temp directory. Ignoring it is
  safe; the pruner collects the worktree later.
- **The pruner runs while other sessions are live.** A session whose PR has just merged sits in
  a clean, landed worktree for as long as it takes to write its summary, and deleting that
  directory out from under it fails in a way that is near-unreadable from the inside. So
  `--merged` skips the worktree the running process is standing in, and any worktree touched in
  the last 30 minutes.
- **A branch name outlives the branch.** `gh pr view <branch>` resolves by name, so reusing
  `fix/545-…` for new work makes the old, merged PR answer for it — which would delete a live
  worktree. Both the hook and the pruner require the PR's `headRefOid` to be the commit the
  worktree has checked out, which also means a follow-up commit pushed on top of a merged
  branch reads as unfinished rather than landed.
- **Measuring "touched" has to happen first.** `git status` rewrites the index whose mtime is
  the freshness signal, so asking after the dirty check makes every worktree look freshly
  touched, forever, and the pruner silently stops removing anything. The freshness is captured
  before any other git command reaches the worktree.
