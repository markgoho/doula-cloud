# Testing: `api/` (Go)

## `api/`: lint with golangci-lint, matching CI exactly

CI runs `golangci-lint` (config: `api/.golangci.yml`) as its own gating step, separate from `go vet`/`go build` -- a change can compile and pass `go test` while still failing CI on `golangci-lint` alone (goconst, noctx, unparam, wrapcheck, and the rest of the curated set in that config). `go vet` is not a substitute for it. The local binary must also be the version CI pins; see "Toolchain versions: local must match CI exactly" below. Before considering `api/` work done, run the same command CI runs:

```sh
cd api
GOLANGCI_LINT_CACHE="$(git rev-parse --show-toplevel)/api/.golangci-cache" golangci-lint run
```

**Always set `GOLANGCI_LINT_CACHE` to a path under `--show-toplevel`, never bare `golangci-lint run`.** Without it, golangci-lint's results cache defaults to one location shared by every worktree on the machine (`~/.cache/golangci-lint` / `~/Library/Caches/golangci-lint`), keyed in a way that does not account for a worktree's path being reused or removed -- a session linting after another worktree was pruned can see findings that point at files that no longer exist on disk, or, worse, a stale "clean" entry that masks a real issue in the current worktree's own changed file (#587). `--show-toplevel` resolves to the current worktree's own root, so the cache lives and dies with that worktree and never leaks into another one; `.golangci-cache` is gitignored, and `.claude/hooks/gate-golangci-lint-cache.sh` blocks a bare `golangci-lint run` in a Claude Code session. If you ever see findings in files that don't exist in your working tree, that's this problem: run `golangci-lint cache clean` with the same `GOLANGCI_LINT_CACHE` set, then rerun.

Two linters in this set are package-wide, not per-file, so a change to one file can newly flag lines you didn't touch in other files in the same package -- `goconst` (a literal crosses its repetition threshold once new call sites are added elsewhere) and `unparam` (a return value becomes "never used" once it's whole-package, not just per-caller). Don't skip fixing those on the grounds that "that file isn't part of this change" -- if `golangci-lint run` at the repo's current state reports it, CI will too.

## Toolchain versions: local must match CI exactly

A local run is evidence of what CI will do only when both run the same Go and the same golangci-lint. They drifted once without anyone noticing: Homebrew moved this machine to Go 1.27.1 and golangci-lint 2.13.2 while CI ran 1.26.5 and v2.12.2. A local covcheck then reported 279 lines that CI passed, and local lint reported findings that CI did not ([#1409](https://github.com/markgoho/doula-cloud/issues/1409)).

- **Go**: `api/go.mod`'s `go` line is the only place the version is named. CI reads it through setup-go's `go-version-file`. `api/Dockerfile`'s `FROM golang:` tag must be the same exact version, not a floating minor. `api/toolchain_guardrail_test.go` fails when the Go running `go test` is not the version go.mod declares, or when the Dockerfile tag differs. A local Go that is *older* fixes itself, because `GOTOOLCHAIN=auto` downloads the declared one. A *newer* one just runs, and that is the case this test catches. To keep the newer Go, move go.mod and the Dockerfile to it in one commit, and CI follows. To stay on the declared Go, run with `GOTOOLCHAIN=go<version>`.
- **golangci-lint**: the `version:` under `golangci/golangci-lint-action` in `.github/workflows/ci.yml` is the pin. `.claude/hooks/gate-golangci-lint-cache.sh` refuses a `golangci-lint run` from a Claude Code session when `golangci-lint version --short` differs from it. To keep the newer version, change the pin and fix what it reports in the same PR. To stay on the pinned version, install it. If either version cannot be read, the hook lets the run through: no binary on `PATH` fails on its own, and a moved ci.yml step is the repo's problem to fix, not a reason to block every lint.

Moving either one usually means fixing what the new version reports in the same PR. A new Go language version turns on new `modernize` analyzers. `api/.golangci.yml` sets both of golangci-lint's per-linter output caps to 0, so CI and a local run show the whole list, not the first page of it. `golangci-lint run --fix` applies the findings that have an automatic fix.

## `api/`: real Postgres for tests, container-engine-agnostic

`api/internal/testdb` uses testcontainers-go to start **one** real, disposable Postgres container per test *process* (`go test` forks one process per package), applies the goose migrations (`api/db/migrations`) once into a template database, and then hands each call to `testdb.New(t)` a fresh database cloned from that template — a file copy, not a migration replay. It hands back a `*testdb.DB` with two connections: `Admin` (the superuser the migrations ran as, for fixture setup) and `App` (a low-privilege `app_runtime`-derived role, the one the running application actually connects as). Postgres superusers and table owners always bypass Row-Level Security, so tests that need to observe RLS in effect — not just assume it — must query through `App`, not `Admin`.

Every package that calls `testdb.New` must define a `TestMain` that hands off to `testdb.Main`, so the shared container is terminated once at process exit rather than leaked or torn down mid-run:

```go
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
```

CI runs this against Docker (preinstalled on the runner, no setup needed). Locally, testcontainers-go reads `DOCKER_HOST` from the environment, so pointing that at a Podman socket runs the same tests against Podman instead, with no code change:

```sh
# macOS: podman machine start; the socket path comes from step 1 of
# walk-a-screen-locally.md. Linux: unix:///run/user/$(id -u)/podman/podman.sock.
# Ryuk is unreliable under rootless Podman, hence TESTCONTAINERS_RYUK_DISABLED.
DOCKER_HOST="unix://<socket>" TESTCONTAINERS_RYUK_DISABLED=true go -C api test ./...
```

Why the variables go on the command rather than through `export`, and which socket value to use, is in [`walk-a-screen-locally.md`](walk-a-screen-locally.md), steps 1 and 2.

Ryuk being disabled locally is why `testdb.Main` exists: without an explicit `container.Terminate` at process exit, a full local `go test ./...` would leave one Postgres container running per package that calls `testdb.New`. CI leaves Ryuk enabled as a backstop, but relies on `testdb.Main` too, since Ryuk only reaps containers after they're already orphaned.

Every package that calls `testdb.New` must wire up its own `TestMain` -- four didn't (`internal/mfarecoverymail`, `internal/outbox`, `internal/sessionmint`, `internal/sessionnotice`), which meant those packages leaked their container on every run, clean or not, until #889 gave each one the same three-line `TestMain` every other package already had.

### A due-time fixture must not compare two clocks

The host and the database run on different clocks whenever the container engine is VM-backed — Podman on macOS, Docker Desktop on macOS. The VM keeps its own time, drifts against the host's, and does not resync when the Mac wakes. CI never sees this: on `ubuntu-latest` the container shares the runner's kernel clock, so the skew is structurally zero.

That makes a coin flip out of any fixture that inserts `next_attempt_at` from a host-side `time.Now()` and then claims the row back through a predicate like `WHERE next_attempt_at <= now()` — a coin flip on the sign of the drift. With the VM's clock even milliseconds behind the host's, the row is not yet due, the worker claims nothing, and the assertion reads the row back exactly as inserted — which looks like the worker silently did nothing rather than like a clock problem. [#987](https://github.com/markgoho/doula-cloud/issues/987) was eight `internal/outbox` tests failing this way.

Seed a due-time from the database's own clock, never the host's. `insertTestRow` in `api/internal/outbox/outbox_test.go` is the pattern: it takes an offset (`0` for "due now", `-time.Minute` for "overdue", `time.Hour` for "not yet due") and computes the timestamp in SQL as `now() + $3 * interval '1 microsecond'`, so one clock decides. Where a fixture must pass a host-side `time.Time`, give it a margin far larger than any plausible skew, and say in a comment that the margin is what absorbs it.

## Reaping orphaned testcontainers

`testdb.Main`'s teardown above only runs on a clean process exit. A killed test process -- an interrupted agent, a timeout, a cut-short TDD loop -- never reaches it, and Ryuk being disabled locally ("`api/`: real Postgres for tests" above) means nothing else reaps that container either. With several parallel Claude Code agent sessions on one machine, these orphans accumulate without bound: 116 running `postgres:16-alpine` containers, 146 total, 133MB free of a 4GB Podman machine, observed live (#889). That starvation made `podman compose up` for `dev:full` time out with `ETIMEDOUT` and made unrelated `api/internal/visit` tests flake locally while staying green in CI.

**Why Ryuk stays disabled, and what does its job.** Ryuk is testcontainers' own reaper: a sidecar container that removes a test's containers when the test process's connection to it drops. It is unreliable under rootless Podman, so local runs set `TESTCONTAINERS_RYUK_DISABLED=true`, and CI, on Docker, keeps it as a backstop. Locally its job is done by an owner label instead ([#2096](https://github.com/markgoho/doula-cloud/issues/2096)). `testdb.OwnerLabels()` (`api/internal/testdb/labels.go`) writes two labels on every container a Go test starts — `testdb.New`'s and `internal/simclock`'s own `TestMain`, the only two `postgres.Run` calls in `api/`; a new testcontainers start passes it too:

- `cloud.doula.test-container=true`, the marker: this container is a Go test's.
- `cloud.doula.test-container.pid=<pid>`, the pid of the test binary that started it (`os.Getpid()`, for example the `simclock.test` process `go test` runs).

`.claude/hooks/testdb-reap.ts` is a `SessionStart` hook (registered in `.claude/settings.json`, beside `e2e-stack-reap.ts`) that removes a container **when, and only when, it carries the marker and its pid is gone.** It reads liveness with `isProcessAlive` from `e2e-stack-reap.ts` (the pidfile sweep's own test: `kill(pid, 0)`, where only `ESRCH` means gone). There is no age rule: a live run keeps its container for as long as it takes, however loaded the machine, and an orphan is removable the moment its process is gone. Everything else is kept: a container with no marker (which includes every container from before #2096, though each carries `org.testcontainers=true`), a marker with a missing or malformed pid, a container with a `com.docker.compose.project` label (an e2e stack, which `e2e-stack-reap.ts` owns), and a container whose pid the OS has handed to another process since — that one reads as alive and waits until the pid is free. The pid is a pid on the host that ran `go test`, which is the Mac the reaper runs on; CI's containers live on its own runner and never meet this hook.

`bun .claude/hooks/testdb-reap.ts --dry-run` prints what a run would remove and removes nothing.

The hook points the engine at `DOCKER_HOST` when the variable is set, the same way `testdb.go` does ("`api/`: real Postgres for tests" above) — but **an unset `DOCKER_HOST` is not a reason to skip the run**, and treating it as one made this hook inert for months. The variable is set by hand on the command that runs the tests, as the block above shows; the repo never sets it in a login profile, and a `SessionStart` hook inherits the login environment rather than that shell's. So the hook never saw it and always returned early: 55 containers, the oldest 38 hours old, on a machine that had started dozens of sessions since ([#1066](https://github.com/markgoho/doula-cloud/issues/1066)). `podman ps` with no `--url` reaches the same engine through its default `podman machine` connection, so `.claude/hooks/container-engine.ts` — the seam both reapers share — adds `--url` only when there is a socket to name, and otherwise just invokes the engine. A set `DOCKER_HOST` can be just as wrong: a login profile can export one for another engine (on the founder's Mac, a Colima Docker socket), `podman --url` at it fails with `ping response was 404`, and the reaper failed open on that for weeks — 91 `org.testcontainers=true` containers, the oldest 23 hours old, on 2026-10-26 ([#1687](https://github.com/markgoho/doula-cloud/issues/1687)). So `testdb-reap.ts` calls the engine through `execEngine`, in the same file, which tries the `--url` form first and, if it fails, the bare form once more. `e2e-stack-reap.ts` still uses the single `--url` form; #1687 tracks moving it. If the engine isn't reachable at all, the hook does nothing and exits 0. It **fails open on every error path** -- unlike `gate-worktree-edit.ts`/`gate-bash-write.ts`, which fail closed because they're `PreToolUse` gates deciding whether to allow a tool call. This hook runs on `SessionStart`, makes no such decision, and exists purely to tidy up; a reaper that errors must never block or slow a session from starting. `gate-shared-index.sh` is the existing fail-open precedent, for the same class of reason. It's quiet when there's nothing to reap; when it does reap, it logs the count and the reason.

Containers from before #2096 carry no marker, so no reaper can tell a live one from an orphan, and none removes them. Remove them by hand at a quiet moment — the command also stops a live run from a worktree still on code older than #2096:

```sh
podman rm -f -t 2 $(podman ps -aq --filter 'label=org.testcontainers=true' --filter 'label!=cloud.doula.test-container')
```

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb` applies them programmatically (see above). At deploy time, `scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a blocking pre-deploy step — it must exit non-zero, and stop the deploy, if migration fails. It's not yet wired to a real instance (none is provisioned in the `doula-cloud` GCP project); see the script's header for the required env vars.

**The row-safety guardrail.** `migrate`, `deploy-api` and `deploy-app` run only on a push to trunk, and every pull request builds an empty Postgres, so a statement that only *existing rows* can refuse is green on the PR and red on the first trunk push — with both deploys stuck behind it. That is [#1021](https://github.com/markgoho/doula-cloud/issues/1021). `api/db/migrations/rowsafety.go` classifies every statement in a migration's Up section against the family of shapes that can fail that way — `ADD COLUMN … NOT NULL` with no `DEFAULT`, `ADD COLUMN … DEFAULT` carrying an inline `REFERENCES`/`CHECK`/`UNIQUE`, `ALTER COLUMN … SET NOT NULL`, `ALTER COLUMN … TYPE`, `ADD CONSTRAINT … UNIQUE`/`PRIMARY KEY`/`EXCLUDE`, `ADD CONSTRAINT … CHECK`, `ADD CONSTRAINT … FOREIGN KEY`, `VALIDATE CONSTRAINT`, `CREATE UNIQUE INDEX`, a generated column, DML, and a `DO` block — and `guardrail_test.go`, a required PR check, fails any it finds. Each class is proved against a real populated Postgres in `rowsafety_pg_test.go`, not asserted from the documentation: the statement must succeed on an empty database and fail on one holding a row. A statement that is genuinely safe gets a note in `api/db/migrations/safety/` saying why, headed by the class it answers; `safety/README.md` describes the shape. Two things are never reported: a statement on a table the same migration creates, which has no rows yet, and an `ALTER TABLE` action carrying its own `DEFAULT` or `NOT VALID` — the classifier reads each action of a multi-action `ALTER TABLE` on its own, so one action's marker never covers the action beside it.
