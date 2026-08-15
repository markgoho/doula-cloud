# Testing infrastructure

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

## `api/`: real Postgres for tests, via Podman

`api/internal/testdb` uses testcontainers-go to start a real, disposable
Postgres container for Go HTTP tests, applies the goose migrations
(`api/db/migrations`) against it, and hands back a `*testdb.DB` with two
connections: `Admin` (the superuser the migrations ran as, for fixture
setup) and `App` (a low-privilege `app_runtime`-derived role, the one the
running application actually connects as). Postgres superusers and table
owners always bypass Row-Level Security, so tests that need to observe RLS
in effect — not just assume it — must query through `App`, not `Admin`.
It targets Podman: testcontainers-go reads `DOCKER_HOST` from the
environment, so no code here is Podman-specific.

To run locally:

```sh
# macOS: podman machine start, then export the socket it prints, e.g.
export DOCKER_HOST='unix:///path/to/podman-machine-default-api.sock'
# Linux: podman ships with a rootless socket, e.g.
export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"

export TESTCONTAINERS_RYUK_DISABLED=true # Ryuk is unreliable under rootless Podman
cd api
go test ./...
```

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb`
applies them programmatically (see above). At deploy time,
`scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a
blocking pre-deploy step — it must exit non-zero, and stop the deploy, if
migration fails. It's not yet wired to a real instance (none is
provisioned in the `doula-cloud` GCP project); see the script's header for
the required env vars.

## `app/`: self-contained podman-compose stack for e2e

`app/compose.e2e.yaml` defines the backing services (currently just
Postgres) that Playwright e2e tests run against — pinned images, no calls
to external/real services. `app/playwright.config.ts` brings the stack up
and down automatically via `globalSetup`/`globalTeardown`
(`app/e2e/global-setup.ts`, `app/e2e/global-teardown.ts`), which shell out
to `podman compose`. Locally this needs `podman-compose` on `PATH`
(`brew install podman-compose`); CI installs it via `pipx`.
