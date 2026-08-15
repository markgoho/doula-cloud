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

`api/internal/testdb` uses testcontainers-go to start a real, disposable
Postgres container for Go HTTP tests, applies the goose migrations
(`api/db/migrations`) against it, and hands back a `*testdb.DB` with two
connections: `Admin` (the superuser the migrations ran as, for fixture
setup) and `App` (a low-privilege `app_runtime`-derived role, the one the
running application actually connects as). Postgres superusers and table
owners always bypass Row-Level Security, so tests that need to observe RLS
in effect — not just assume it — must query through `App`, not `Admin`.

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

## `api/`: migrations via goose

Migrations live in `api/db/migrations`. In dev/CI, `internal/testdb`
applies them programmatically (see above). At deploy time,
`scripts/migrate.sh` applies them through the Cloud SQL Auth Proxy as a
blocking pre-deploy step — it must exit non-zero, and stop the deploy, if
migration fails. It's not yet wired to a real instance (none is
provisioned in the `doula-cloud` GCP project); see the script's header for
the required env vars.

## `app/`: self-contained compose stack for e2e

`app/compose.e2e.yaml` defines the backing services (Postgres, the goose
migration step, and the Go BFF itself) that Playwright e2e tests run
against — pinned/locally-built images, no calls to external/real services.
`app/playwright.config.ts` brings the stack up and down automatically via
`globalSetup`/`globalTeardown` (`app/e2e/global-setup.ts`,
`app/e2e/global-teardown.ts`), through `app/e2e/stack.ts`, which shells out
to `$CONTAINER_ENGINE compose` (`docker compose` and `podman compose` share
the same v2 CLI syntax). CI sets `CONTAINER_ENGINE=docker` — Docker ships
preinstalled and needs no setup on `ubuntu-latest`, unlike Podman's
rootless-socket dance and its `docker-compose`-as-external-provider
translation layer, which silently broke container-to-host networking
(`host.containers.internal`) under CI in practice. `CONTAINER_ENGINE`
defaults to `podman` when unset, so local dev is unaffected and needs
`podman-compose` on `PATH` (`brew install podman-compose`) as before.
`compose.e2e.yaml`'s `api` service resolves the right host-gateway
hostname (`host.containers.internal` for Podman, `host.docker.internal`
for Docker, the latter needing an explicit `extra_hosts: host-gateway`
entry) via the `E2E_HOST_GATEWAY` env var `stack.ts` sets from
`CONTAINER_ENGINE`.
