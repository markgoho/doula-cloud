# Testing: the pre-commit hook and the CI gates

## Pre-commit hook: `gofmt`, `app/` typecheck/lint, and Prettier plus `tsc` over the tooling trees (enabled repo-wide, enforced in CI)

`core.hooksPath` is set to an absolute path (`scripts/hooks`) in this repo's shared `.git/config`, so the hook below is already active for the main checkout and every `.claude/worktrees/*` worktree — there is nothing to opt into per clone. Because the path is absolute, every worktree runs the *main* checkout's `scripts/hooks/pre-commit`, not its own branch's copy; a worktree mid-refactor of the hook script itself won't see its own changes take effect until they land on the branch checked out in main. Re-run `git config core.hooksPath scripts/hooks` only if setting up a fresh clone.

`scripts/hooks/pre-commit` runs:
1. **`api/` (Go)**: blocks any commit that stages an unformatted `.go` file, prompting to run `gofmt -w <file>` on it.
2. **`app/` (SvelteKit)**: if any `app/*` files are staged, runs `bun run --cwd app check` (`svelte-check`) and `bun run --cwd app lint` (`eslint`), blocking commits with broken imports, type errors, or lint failures -- including `@stylistic/quotes`, #1232's single-quote rule, since app/'s formatting gate rides this same step rather than a step of its own. See "Formatting: which trees are gated, and by what" below.
3. **`app/` unit suite and coverage gate**: still only when `app/*` files are staged, runs `bun run --cwd app test:unit:coverage`. This is where the design brief's smoothness gates live (see [`app-unit.md`](app-unit.md)) and, since #1232, where `indent.usage.spec.ts` checks app/'s tab-indentation rule -- the brief's own argument is that a commitment nobody measures decays — so the cheapest place to measure is before the commit exists. Measured on an idle 14-CPU machine, this step is ~16s and peaks around 4.9 GB for 2689 tests at 100% coverage, on top of the ~7s for steps 1-2. The Playwright e2e suite deliberately stays out: it builds the app and starts Postgres, the object store, the BFF and the Auth emulator — it is costed in [`e2e.md`](e2e.md)'s "What the e2e suite costs, and why its workers are capped", which is where to look before running it beside anything else. See "The memory this gate costs, and why the browser pool is capped" below for where that 4.9 GB goes, and "Only one session runs this step at a time" for the lock that keeps two sessions from paying it simultaneously.

4. **`.claude/hooks/` and `scripts/` (the repo's own tooling), formatting**: if any `.ts` file in either tree is staged, runs Prettier's `--check` over exactly those staged files, blocking a commit that stages one which is not formatted. See "Formatting: which trees are gated, and by what" below for what the rule is and why these two trees needed a gate of their own.
5. **`.claude/hooks/` and `scripts/`, typecheck**: still only when a `.ts` file in either tree is staged, runs `tsc --noEmit -p tsconfig.tooling.json` over the whole tooling program, blocking a commit that leaves a type error anywhere in either tree. Unlike step 4, this cannot be scoped to the staged files alone — a type error can live in a file the staged one only imports — so it always typechecks both trees in full rather than a per-file diff. See "Typecheck: which trees are gated, and by what" below for the gate itself and why it is scoped narrower than the repo.

The CI jobs are the actual enforcement backstop regardless of whether the local hook is enabled — required PR status checks reject a push that would have failed it (see [`docs/agents/worktree-flow-reference.md`](../agents/worktree-flow-reference.md), "Enforcement").

## Formatting: which trees are gated, and by what

Formatting is gated per tree, not repo-wide, and each tree's gate is the tool that tree is written with.

| Tree | Rule | Gate |
| --- | --- | --- |
| `api/` | `gofmt`, and `.editorconfig`'s `[*.go]` override (#1232) says so | `gofmt -l .` as a CI step, plus step 1 of the pre-commit hook |
| `.claude/hooks/`, `scripts/` | `.editorconfig` (2-space indent) + `.prettierrc` (single quotes) | `bun run format:check` as the CI `format` job, plus step 4 of the pre-commit hook |
| `app/` | `.editorconfig`'s `[app/**]` override (tab indent) + `@stylistic/quotes` in `app/eslint.config.js` (single quotes) | `bun run --cwd app lint` (quotes) + `indent.usage.spec.ts` (indentation) — both already required by the `app` CI job and pre-commit steps 2-3 |

The two tooling trees are the decision #1120 asked for, so it is recorded here: **they follow `.editorconfig` and `.prettierrc` rather than being an exception to them.** They had drifted to 22 tab-indented files out of 29, and quoting was split inside a single feature, because nothing measured either rule — `gofmt` is Go-only, the pre-commit hook's `app/` steps are scoped to staged `app/*` paths, and Prettier appeared in no `package.json` and no workflow in this repository at all. Conforming rather than excepting was chosen because it needs no config change (7 of the 29 files already matched) and because `.editorconfig` is what a contributor's editor applies on save, so conforming makes the editor and the file agree instead of making the file win an argument with the editor.

`bun run format` at the repo root rewrites the two trees; `bun run format:check` is the same pass as an assertion. Both are scoped to `.claude/hooks/**/*.ts` and `scripts/**/*.ts` and reach nothing else. Prettier reads `.editorconfig` for indentation by default, so a single `--check` enforces both of the configs above and there is no second copy of the indent rule to drift.

`app/` is the decision #1232 asked for, and it went the other way from the tooling trees: `.editorconfig` gained an `[app/**]` override (tab indent) rather than app/ conforming to the bare `[*]` block, because every file in the tree already matched tabs and none matched the 2-space rule — the opposite of the tooling trees' 7-of-29 split that made conforming the cheap option there. `api/` got the same shape of override, `[*.go]`, with `gofmt` named as the reason beside it, so `.editorconfig` finally says what `gofmt -l .` already enforces.

Enforcement is two separate mechanisms rather than one Prettier pass, because app/ has no Prettier of its own. `@stylistic/eslint-plugin`'s `quotes` rule in `app/eslint.config.js` covers single-quoting — reached through `bun run --cwd app lint`, already a required step everywhere formatting needed a check — and a narrow `app/src/lib/indent.usage.spec.ts` covers the tab requirement, riding the same `bun run --cwd app test:unit:coverage` gate the design brief's smoothness rules already live behind. `@stylistic/eslint-plugin`'s own `indent` rule was tried first and rejected: it disagreed with several already-tab-indented lines that hand-align a wrapped union type or object literal under an opening brace, rather than finding any real space-indented file, so adopting it would have meant reformatting deliberate code to match an opinion nobody asked for. The usage spec instead encodes exactly `.editorconfig`'s own rule — a line's indentation may not start with a space — and exempts the one legitimate case already in the tree: a continuation line inside an open `/* */` or `<!-- -->` block comment, the same wrinkle #1120's own reformat commit found and hand-fixed in two tooling-tree files. `src/lib/components/atoms/Icon/generated/` is excepted from the quotes rule for a different reason: `scripts/sync-icons.ts` writes it from raw SVG markup and says not to edit it by hand, so its double-quoted strings (escaping the SVG's own double-quoted attributes) are the generator's choice, not a drift to fix.

No app/ file needed a reformat for indentation — it was already uniformly tab-indented. A small number of files were still double-quoted; those were fixed with an ordinary `eslint --fix`, landed as its own pull request ahead of this gate (#1232's history has both).

## Typecheck: which trees are gated, and by what

`.claude/hooks/` and `scripts/` had no typecheck gate at all until #1335 — `app/`'s `svelte-check` and `api/`'s `go build`/`go vet` each typecheck their own tree, but nothing ran `tsc` over these two. The root `tsconfig.json` has no `include`, so `tsc -p tsconfig.json` walks the whole repo: it typechecks `app/` and `gcp-dashboard/` a second time under settings neither tree's own `tsconfig.json` uses, and it fails on unrelated `app/e2e/*.ts` Playwright/DOM-lib errors that are someone else's tree's problem. `tsconfig.tooling.json` at the repo root extends the root config's strict settings (including `noUncheckedIndexedAccess`, which is what caught #1335's type errors — sixteen of them across four files once the scope was narrowed and actually run, not only the three the issue named) but replaces `include` with exactly `.claude/hooks/**/*.ts` and `scripts/**/*.ts` — the two trees with no gate of their own, plus whatever files they import (`tsc` follows imports, so a shared helper like `app/e2e/ports.ts` enters the program too).

`bun run typecheck` runs it (`tsc --noEmit -p tsconfig.tooling.json`); the CI `format` job runs the same command as its `Typecheck` step, and step 5 of the pre-commit hook above runs it locally whenever a file in either tree is staged.

## `docs/`: the specs that read it run in their own CI job

A PR that changes only `docs/` or a root `*.md` file skips CI's `api` and `app` jobs ([#1466](https://github.com/markgoho/doula-cloud/issues/1466)), so the specs that read `docs/` cannot live only inside `app`'s suite. CI's `docs` job runs them on every PR: `app/src/lib/prose.usage.spec.ts`, `adrNumbers.usage.spec.ts`, `testPlans.usage.spec.ts` and `spelling.usage.spec.ts`. To run the same set locally, from `app/`: `bunx vitest --run --project server src/lib/prose.usage.spec.ts src/lib/adrNumbers.usage.spec.ts src/lib/testPlans.usage.spec.ts src/lib/spelling.usage.spec.ts`. A new spec that reads `docs/` goes on that job's list in `.github/workflows/ci.yml`; the rules for the skip itself are in [`docs/agents/worktree-flow-reference.md`](../agents/worktree-flow-reference.md), "Enforcement".

## The memory this gate costs, and why the browser pool is capped

The gate above is the heaviest thing this repo runs locally, and the reason is not obvious from its name: `app/vite.config.ts` runs its `client` project in **browser mode**, so `test:unit:coverage` starts headless Chromium. The browsers belong to the pre-commit gate, not to the e2e suite the gate deliberately excludes.

Vitest sizes that pool as `Math.min(12, ncpu - 1)` — a guard against the main thread choking, with nothing in it that asks what memory is free, and nothing that knows how many other sessions are doing the same thing. Several Claude sessions run against this repo at once ([`docs/agents/worktree-flow.md`](../agents/worktree-flow.md)), each free to commit whenever it likes. On a 24 GB machine whose non-repo residents (a browser, the `claude` processes, the Podman VM) already come to ~10 GB, two uncapped gates at ~7 GB each do not fit, and the harness kills the commit with "system is running low on memory". That was [#935](https://github.com/markgoho/doula-cloud/issues/935).

So the `client` project pins `maxWorkers` to `Math.min(6, availableParallelism() - 1)`. Measured on an idle 14-CPU machine:

| Renderers | Peak (browsers + Vitest) | Wall time |
| --- | --- | --- |
| 12 (Vitest's default here) | ~7.0 GB | 17.4s |
| **6 (what we pin)** | **4.9 GB** | **16.3s** |
| 4 | 4.5 GB | 18.0s |

Compare warm runs only. The first run of a session pays a cold Vite transform (`transform 38s, import 87s` against ~4s and ~22s once warm) and takes about 20s at any worker count; a cold run measured against a warm one will credit the cap with roughly 4s it did not earn.

Memory is the reason for the cap, and it is the column that moves: browser RSS falls from ~6.0 GB to ~4.0 GB. Wall time is roughly a wash — about a second, plus a real drop in CPU time (111s to 78s) from no longer oversubscribing 14 cores. Six is chosen because it costs nothing in speed, not because it buys any.

It is clamped rather than a bare constant so CI's worker count is untouched: a 4-vCPU `ubuntu-latest` runner already resolves to 3, and a constant 6 would have raised the parallelism there. The `groupOrder` split below *does* apply in CI, where the two projects previously overlapped — measured at no cost, with the `app` job at 4m20s against 4m11s and 4m39s for the two preceding trunk runs.

Two things to know before you change it:

- **`--maxWorkers` on the command line will not override this.** The browser pool reads the project's own `maxWorkers`, not the root config's. A run with `--maxWorkers=4` still spawns 12 renderers. Edit the `client` project in `app/vite.config.ts`.
- **The `server` project carries `sequence: { groupOrder: 1 }` because of this cap.** Vitest refuses two projects that share a `groupOrder` but disagree on `maxWorkers`. Splitting them is a second memory win, not a formality: the Node forks no longer overlap the Chromium renderers, so the run has one peak instead of two stacked together.

### Only one session runs this step at a time

Capping one run does not coordinate several. Two concurrent gates fit (~10 GB of headroom used); three do not, because roughly 3.5 GB per gate is fixed cost that no worker count removes — so lowering the cap further buys no third run, and only admission control does. Up to 9 worktrees run at once ([`docs/agents/worktree-flow-reference.md`](../agents/worktree-flow-reference.md)), each free to commit whenever it likes, so three at once is the ordinary case rather than the edge one. That was [#936](https://github.com/markgoho/doula-cloud/issues/936).

`scripts/gate-lock.ts` wraps the `test:unit:coverage` step in a machine-wide exclusive lock: `bun scripts/gate-lock.ts -- <command>`. A second session waits and says so — one line naming the holding worktree, its pid, and how long it has been running, repeated every 15 seconds so the commit reads as queued rather than hung. `gofmt`, `check` and `lint` are cheap and stay unlocked.

The lock is a directory created with a non-recursive `mkdir`, which is the atomic test-and-set; macOS ships no `flock` binary, which is why this is a bun script and not a shell one-liner. It lives in the **git common directory** (`$(git rev-parse --git-common-dir)/pre-commit-gate.lock`), which every worktree of this checkout resolves to the same path — a per-worktree `.git` would give each session a private lock and coordinate nothing.

**Every path through it fails open**, because a gate that wedges every commit is worse than the memory pressure it prevents:

- **The owner is gone.** The holder's pid is checked with signal 0 on every poll; a killed session's lock is reclaimed at once, with no waiting on any threshold. `EPERM` counts as alive — only `ESRCH` proves a process dead.
- **The lock has gone quiet.** A holder touches the lock directory every 5 seconds for as long as its command runs, so `STALE_LOCK_MS` (5 minutes) measures *abandonment*, not duration — 60 consecutive missed beats. Without the heartbeat this would be a run-time limit, and a gate queued behind two others on a pressured machine would have its lock taken while it was demonstrably still working. It only ever fires on a lock this script did not finish writing; a holder that was killed is caught by the liveness check above, which needs no threshold.
- **Reclaiming is single-winner.** A stale lock is taken by `rename`, not `rm -rf`. Two waiters that both judge the same lock stale would otherwise both end up running — the first removes it and takes a fresh one, the second deletes *that* and takes one too. Only one rename can succeed; the loser gets `ENOENT` and goes round the loop. For the same reason a holder releases its lock only on positive proof that the lock is still the one it took (an owner file naming its own pid), never on the absence of proof. The directory's inode is the obvious cheaper receipt and does not work: APFS reuses the inode a removed directory just gave up.
- **The wrapper itself is optional.** `scripts/hooks/pre-commit` resolves it from `$0` (the hook's absolute path in the main checkout, since `core.hooksPath` is absolute) rather than the committing worktree's cwd, and runs the step unwrapped if the file is not there. A worktree branched before this landed still commits.
- **Anything unexpected.** No git directory, an unwritable parent, a lock that cannot be reasoned about: the wrapper prints `gate-lock: running without the lock (…)` and runs the command.

**Landing this is not the same as switching it on.** `core.hooksPath` is absolute into the main checkout, so every worktree runs *main's* `scripts/hooks/pre-commit` and therefore *main's* `scripts/gate-lock.ts`. Until main's own tree carries both, the hook's `-f` fallback runs the step unwrapped everywhere — by design, but it means the lock starts working only once main's `trunk` has fast-forwarded past the merge. `.claude/hooks/sync-trunk.ts` does that on `SessionStart`, so in practice it is live from each session's next start.

Two escape hatches, for when you do not want to wait:

```sh
# Run this commit's gate with no lock at all.
SKIP_GATE_LOCK=1 git commit -m "…"

# Clear a lock by hand. Almost never needed -- a dead owner is reclaimed
# on the next poll and a stale one after 5 minutes -- but this is the
# command if you want it gone now.
rm -rf "$(git rev-parse --git-common-dir)/pre-commit-gate.lock"
```

Measured on the same 14-CPU / 24 GB machine as the table above, warm, sampling every 500 ms. The full record, including how the run was driven, is on [#936](https://github.com/markgoho/doula-cloud/issues/936).

| | Peak whole gate | Peak Chromium | Renderer processes | Free-memory floor |
| --- | --- | --- | --- | --- |
| One gate | 6.00 GB | 4.78 GB | 12 | 34% |
| **Three concurrent gates, locked** | **7.33 GB** | **6.22 GB** | **12** | **34%** |
| Three concurrent gates, unlocked | ~18 GB (projected) | ~14 GB | 36 | — |

Three sessions committing at once cost about a fifth more than one, not three times — they run one after another, finishing at +21s, +43s and +78s, each waiting on the previous one's lock. The overshoot above a single gate is the tail of one run's browsers exiting while the next starts, and the number that matters is the free-memory floor: identical at one gate and three.

The unlocked row was deliberately **not** reproduced. Doing so means intentionally exhausting memory on a machine with other live agent sessions mid-commit, and the figure is already known from the per-gate one: ~18 GB against the ~10.5 GB of non-repo residents #936 measured is well past 24 GB, the same arithmetic that killed a commit at two *uncapped* gates in #935.

**Do not read the renderer count as a worker count.** A single capped gate was observed at both 6 and 12 `--type=renderer` processes across runs of the same command, so the number does not map one-to-one onto the `maxWorkers` cap of 6 and the reason for the spread was not chased down. Compare it against the one-gate row above, never against the cap. [#937](https://github.com/markgoho/doula-cloud/issues/937) sized Playwright's workers without it for exactly this reason — it took the worker count from Playwright's own `Running N tests using M workers` line and the cost from sampled RSS, and [`e2e.md`](e2e.md)'s "What the e2e suite costs, and why its workers are capped" is what that measured.

### When a commit is killed for memory

The failure looks like a hung or killed commit, not a test failure, so check the machine rather than the diff:

```sh
# What the test browsers are costing right now. `grep -i chrome` is no use
# here -- it also matches your own browser. ms-playwright is the giveaway.
ps -Ao pid,rss,command | grep ms-playwright | grep -v grep

# How many renderers are live -- this is the number the cap controls.
ps -Ao command= | grep ms-playwright | grep -v grep | grep -c -- --type=renderer

# Current pressure. Prefer this to `sysctl vm.swapusage`: macOS never
# shrinks its swap-used counter, so a high figure there is a record of the
# worst moment since boot, not a reading of now.
memory_pressure | tail -3
```

If renderers from another session are live, wait rather than kill them — they exit on their own and the memory comes back. A fix that races another session is worse than the wait.

## Coverage: 100% line coverage, with justified exceptions

Both `api/` and `app/` are gated at 100% line coverage in CI. A line that genuinely can't be exercised by a test (e.g. `log.Fatal` on listener startup failure) needs an inline comment justifying the exception — it is not left to ad-hoc PR discussion.

**`api/` (Go):** mark the line, or the `if` guarding it, with a comment containing `coverage:ignore`:

```go
// coverage:ignore reason: listener startup, not exercised by unit tests
if err := http.ListenAndServe(":"+port, nil); err != nil {
	log.Fatal(err)
}
```

`api/tools/covcheck` parses the `go test -coverprofile` output and fails the build on any zero-coverage line that has no `coverage:ignore` comment directly above it or within the uncovered block. Run it locally:

```sh
cd api
go test ./... -coverprofile=coverage.out
go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/
```

`tools/covcheck` itself is still tested (`go test ./...` runs its unit tests same as any other package) but excluded from the coverage *requirement* via `-skip` — it's dev tooling, not shipped application code.

"Directly above" means above the block as Go 1.26 reported it, which is how every marker in the repo is placed. Go 1.27 reports the same code in different shapes: an `if` body's block starts at its first statement rather than on the brace line, and a straight-line block is cut into pieces at each comment line. covcheck maps a 1.27 profile back onto the 1.26 blocks before judging it, so the marker above the `if` keeps working and nothing moves ([#1409](https://github.com/markgoho/doula-cloud/issues/1409)). One real difference remains: 1.27 counts some lines on their own that 1.26 folded into a neighboring covered block, so a genuinely unreachable line can appear for the first time. That one needs its own marker.

**`app/` (SvelteKit + Vitest):** Vitest's `v8` coverage provider has this built in — use `/* v8 ignore next */` (or `/* v8 ignore start */` / `/* v8 ignore stop */` for a range), with a trailing reason comment. The 100% threshold is set in `app/vite.config.ts` under `test.coverage`, scoped to `src/lib/**` — the code Vitest unit-tests. `src/routes/**` is exercised by the Playwright e2e suite instead, but that suite does not currently collect coverage, so route code has no coverage gate yet. As route code accumulates real logic (beyond markup), instrumenting the e2e run's coverage and merging it into the same threshold is the follow-up; until then the 100% gate is honest about covering `src/lib/**` only, not all of `app/`.

## Reading a failed CI run

Find the failed job, then read its log through the API. `gh run view --log-failed` serves the latest attempt of each job, so after a re-run it can show a log that is not the one that failed; the job's own log by id does not have that problem. The log holds terminal escape sequences, and `gh api` refuses to print it until you pass `--allow-escape-sequences`:

```sh
gh api repos/markgoho/doula-cloud/actions/runs/<run id>/jobs --jq '.jobs[] | select(.conclusion == "failure") | .id'
gh api repos/markgoho/doula-cloud/actions/jobs/<job id> --jq '.steps[] | select(.conclusion == "failure") | .name'
gh api repos/markgoho/doula-cloud/actions/jobs/<job id>/logs --allow-escape-sequences
```

A run that is red once and green on a re-run of the same commit is a flake, and the fix is to the test or the job's resilience, not another re-run. Whether a trunk run proves a merge's `migrate` and deploys is in [`docs/agents/worktree-flow-reference.md`](../agents/worktree-flow-reference.md), "What a green PR does not prove".
