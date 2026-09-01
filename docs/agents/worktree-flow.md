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

## What's provisioned automatically

`EnterWorktree` (and, as a fallback, a manually run `git worktree add`) runs
`.claude/hooks/worktree-provision.ts`, which:

- Copies `app/.env.local` from the main checkout if the worktree doesn't already have one.
  This is the only gitignored file local dev needs (Stripe Sandbox + Mailgun test keys); if
  it's missing from main too, see `docs/environment.md`'s `scripts/stripe-setup.sh`.
- Symlinks root `node_modules` to the main checkout's, unless the branch's own
  `package.json`/`bun.lock` differ from `trunk`, in which case it removes the symlink and
  runs a real `bun install` at the worktree root instead. Re-checked on every provision, so
  adding a root dependency later and re-entering the worktree switches it to a real install.
- **`app/node_modules` always gets a real `bun install`, never a symlink.** SvelteKit 3
  writes generated per-checkout state into it (`node_modules/$app/tsconfig.json`,
  `node_modules/$app/types`) that TypeScript resolves via that file's real path before
  applying its `rootDirs` entries — through a symlink, every worktree's `rootDirs` collapses
  onto whichever checkout `app/node_modules` physically lives in, breaking every `./$types`
  import project-wide. Confirmed empirically: reproduces on a bare `trunk` checkout with no
  other changes, in every symlinked worktree, and is absent under a real install (which is
  what CI already does). A live symlink here is also a write hazard on its own: `svelte-kit
  sync` mutates `node_modules/$app/*` in place, so two worktrees sharing a symlinked
  `app/node_modules` would clobber each other's generated state even without the `rootDirs`
  bug. The dependency-manifest check above still governs whether this is a *fresh* install or
  a left-alone existing one — it just never chooses a symlink for `app/`.
- Assigns a port offset (`.port-offset`, gitignored) — the lowest value 1–9 not already
  claimed by another live worktree. `app/e2e/ports.ts` shifts every port by
  `offset * 100`, so two worktrees can run `bun run dev:full` or the e2e suite at the same
  time without colliding. The main checkout and CI have no `.port-offset` file, so they
  always run at offset 0 — today's exact ports, unchanged.

**Never run `bun install` through a live `node_modules` symlink** — it mutates the main
checkout's modules for every worktree sharing them at once. The provisioning hook already
avoids this (it unlinks before installing); don't `bun install` by hand inside a worktree
without checking `ls -la node_modules` first.

**Only one worktree at a time may hold port offset 0** — that's the main checkout itself, by
having no `.port-offset` file. A worktree never gets assigned 0.

**The pool is capped at 9 concurrent worktrees** (offsets 1–9). If provisioning fails because
the pool is full, prune stale worktrees first (`bun .claude/hooks/worktree-prune.ts
--dry-run`, then `--merged`) rather than working around the cap.

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
- **GitHub**: a ruleset on `trunk` requires a passing PR (checks: `scripts`, `actionlint`,
  `api`, `app`, `api-image` — the `ci.yml` jobs only; the Firebase preview workflows are
  `paths:`-filtered and would never satisfy a required check that always waits on them),
  blocks force-push and deletion, and requires linear history. Repository-admin is a bypass
  actor for a genuine emergency or a docs-only fix.

`gate-shared-index.sh` (blocking pathspec-less `git add` and `git commit --amend`) stays
active. Its threat model — several sessions racing one index — still holds *within* a
worktree whenever more than one session enters the same one; worktrees narrow the blast
radius, they don't remove the need for it.

## Cleanup

`.claude/hooks/worktree-prune.ts`:

```sh
bun .claude/hooks/worktree-prune.ts --dry-run   # list every worktree: branch, PR state, dirty flag, size
bun .claude/hooks/worktree-prune.ts --merged    # remove only a worktree whose branch is
                                                 # merged into trunk AND whose tree is clean
```

`--merged` never touches a dirty worktree, a locked one, or one whose branch hasn't landed —
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
