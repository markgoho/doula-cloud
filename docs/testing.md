# Testing infrastructure

## Pre-commit hook: `gofmt` and `app/` typecheck/lint (opt-in locally, enforced in CI)

To catch formatting, typecheck, or lint issues before they reach CI, enable the repo's pre-commit hook once per clone:

```sh
git config core.hooksPath scripts/hooks
```

`scripts/hooks/pre-commit` runs:
1. **`api/` (Go)**: blocks any commit that stages an unformatted `.go` file, prompting to run `gofmt -w <file>` on it.
2. **`app/` (SvelteKit)**: if any `app/*` files are staged, runs `bun run --cwd app check` (`svelte-check`) and `bun run --cwd app lint` (`eslint`), blocking commits with broken imports, type errors, or lint failures.

This is opt-in (`core.hooksPath` is local git config, not something a clone picks up automatically) — the CI jobs are the actual enforcement backstop regardless of whether it's enabled locally.

## `api/`: lint with golangci-lint, matching CI exactly

CI runs `golangci-lint` (config: `api/.golangci.yml`) as its own gating step,
separate from `go vet`/`go build` -- a change can compile and pass `go test`
while still failing CI on `golangci-lint` alone (goconst, noctx, unparam,
wrapcheck, and the rest of the curated set in that config). `go vet` is not a
substitute for it. Before considering `api/` work done, run the same command
CI runs:

```sh
cd api
golangci-lint run
```

Two linters in this set are package-wide, not per-file, so a change to one
file can newly flag lines you didn't touch in other files in the same
package -- `goconst` (a literal crosses its repetition threshold once new
call sites are added elsewhere) and `unparam` (a return value becomes
"never used" once it's whole-package, not just per-caller). Don't skip
fixing those on the grounds that "that file isn't part of this change" --
if `golangci-lint run` at the repo's current state reports it, CI will too.

## Coverage: 100% line coverage, with justified exceptions

Both `api/` and `app/` are gated at 100% line coverage in CI. A line that
genuinely can't be exercised by a test (e.g. `log.Fatal` on listener
startup failure) needs an inline comment justifying the exception — it is
not left to ad-hoc PR discussion.

**`api/` (Go):** mark the line, or the `if` guarding it, with a comment
containing `coverage:ignore`:

```go
// coverage:ignore reason: listener startup, not exercised by unit tests
if err := http.ListenAndServe(":"+port, nil); err != nil {
	log.Fatal(err)
}
```

`api/tools/covcheck` parses the `go test -coverprofile` output and fails
the build on any zero-coverage line that has no `coverage:ignore` comment
directly above it or within the uncovered block. Run it locally:

```sh
cd api
go test ./... -coverprofile=coverage.out
go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/
```

`tools/covcheck` itself is still tested (`go test ./...` runs its unit
tests same as any other package) but excluded from the coverage
*requirement* via `-skip` — it's dev tooling, not shipped application
code.

**`app/` (SvelteKit + Vitest):** Vitest's `v8` coverage provider has this
built in — use `/* v8 ignore next */` (or `/* v8 ignore start */` /
`/* v8 ignore stop */` for a range), with a trailing reason comment. The
100% threshold is set in `app/vite.config.ts` under `test.coverage`,
scoped to `src/lib/**` — the code Vitest unit-tests. `src/routes/**` is
exercised by the Playwright e2e suite instead, but that suite does not
currently collect coverage, so route code has no coverage gate yet. As
route code accumulates real logic (beyond markup), instrumenting the e2e
run's coverage and merging it into the same threshold is the follow-up;
until then the 100% gate is honest about covering `src/lib/**` only, not
all of `app/`.

## `api/`: real Postgres for tests, container-engine-agnostic

`api/internal/testdb` uses testcontainers-go to start **one** real,
disposable Postgres container per test *process* (`go test` forks one
process per package), applies the goose migrations (`api/db/migrations`)
once into a template database, and then hands each call to `testdb.New(t)`
a fresh database cloned from that template — a file copy, not a migration
replay. It hands back a `*testdb.DB` with two connections: `Admin` (the
superuser the migrations ran as, for fixture setup) and `App` (a
low-privilege `app_runtime`-derived role, the one the running application
actually connects as). Postgres superusers and table owners always bypass
Row-Level Security, so tests that need to observe RLS in effect — not just
assume it — must query through `App`, not `Admin`.

Every package that calls `testdb.New` must define a `TestMain` that hands
off to `testdb.Main`, so the shared container is terminated once at
process exit rather than leaked or torn down mid-run:

```go
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
```

CI runs this against Docker (preinstalled on the runner, no setup needed).
Locally, testcontainers-go reads `DOCKER_HOST` from the environment, so
pointing that at a Podman socket runs the same tests against Podman
instead, with no code change:

```sh
# macOS: podman machine start, then export the socket it prints, e.g.
export DOCKER_HOST='unix:///path/to/podman-machine-default-api.sock'
# Linux: podman ships with a rootless socket, e.g.
export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"

export TESTCONTAINERS_RYUK_DISABLED=true # Ryuk is unreliable under rootless Podman
cd api
go test ./...
```

Ryuk being disabled locally is why `testdb.Main` exists: without an
explicit `container.Terminate` at process exit, a full local `go test
./...` would leave one Postgres container running per package that calls
`testdb.New`. CI leaves Ryuk enabled as a backstop, but relies on
`testdb.Main` too, since Ryuk only reaps containers after they're already
orphaned.

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb`
applies them programmatically (see above). At deploy time,
`scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a
blocking pre-deploy step — it must exit non-zero, and stop the deploy, if
migration fails. It's not yet wired to a real instance (none is
provisioned in the `doula-cloud` GCP project); see the script's header for
the required env vars.

## `app/`: e2e stack — Postgres in compose, migrate/BFF/emulator as host processes

`app/compose.e2e.yaml` now defines exactly one backing service: a pinned
`postgres:16-alpine`. Everything else Playwright e2e tests run against —
the goose migration step, the `app_e2e` login role, the Go BFF, and the
Firebase Auth emulator — runs as a plain host process, started/stopped by
`app/e2e/stack.ts` (`startStack`/`stopStack`), which `app/playwright.config.ts`
wires up via `globalSetup`/`globalTeardown`
(`app/e2e/global-setup.ts`/`app/e2e/global-teardown.ts`). `stack.ts` runs
`api/cmd/migrate` with `go run` and the BFF with `go build` + a tracked
PID (mirroring how it already managed the Firebase emulator), against
`DATABASE_URL`s pointed at the compose Postgres over `127.0.0.1:15432`.
Building the BFF and migrate binaries as compose images used to cost the
e2e run a cold `go mod download`/build on every CI run with no cache
sharing between them; running them as host processes instead lets CI's
`app` job share a single warm Go build cache (via `actions/setup-go`,
keyed on `api/go.sum`) with everything else that touches `api/`.

Postgres itself stays in `compose.e2e.yaml` (image pinning matters more
than build cost there) and is brought up/down via `$CONTAINER_ENGINE
compose` (`docker compose` and `podman compose` share the same v2 CLI
syntax) — CI sets `CONTAINER_ENGINE=docker` (preinstalled, no
rootless-socket setup needed on `ubuntu-latest`); it defaults to `podman`
for local dev, which needs `podman-compose` on `PATH` (`brew install
podman-compose`). Since every process now reaches Postgres, the emulator,
and the BFF's own listener over loopback directly, the old
`E2E_HOST_GATEWAY`/`host.containers.internal`/`host.docker.internal`
container-to-host routing machinery is gone — it was only ever needed to
get a *container* to reach a host-bound service, and nothing runs the BFF
in a container anymore.

Because that removes the only thing that proved `api/Dockerfile` still
builds and boots (the runtime image is distroless
`gcr.io/distroless/static-debian12`, where a missing CA bundle or tzdata
would break a real deploy even though `go build`/`go test` pass cleanly),
CI's `api-image` job (`.github/workflows/ci.yml`) now builds that image
with `docker/build-push-action` and runs a boot smoke test against it —
container stays running and answers on its port — in parallel with `app`,
off the critical path (see PR #108 for measured before/after timings).
