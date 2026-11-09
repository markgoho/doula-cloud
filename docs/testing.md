# Testing infrastructure

Testing is split by branch. Read the part for the tree you are changing; each section named below lives only in that part.

- **[Walk a screen locally](testing/walk-a-screen-locally.md)**: the Podman socket, the inline `DOCKER_HOST` form, one `app/` spec, `TZ=UTC`, the dev origin under a port offset, a seeded Staff member, Client and Engagement, signing in and landing on a Staff screen with a browser tool, and looking at it at 320px.
- **[The pre-commit hook and the CI gates](testing/gates.md)**: "Pre-commit hook", "Formatting: which trees are gated, and by what", "Typecheck: which trees are gated, and by what", "`docs/`: the specs that read it run in their own CI job", "The memory this gate costs, and why the browser pool is capped" (with "Only one session runs this step at a time" and "When a commit is killed for memory"), "Coverage: 100% line coverage, with justified exceptions", and "Reading a failed CI run".
- **[`api/` (Go)](testing/api.md)**: "lint with golangci-lint, matching CI exactly", "Toolchain versions: local must match CI exactly", "real Postgres for tests, container-engine-agnostic" (with "A due-time fixture must not compare two clocks"), "Reaping orphaned testcontainers", and "migrations via goose". Ryuk, testcontainers' own reaper, stays disabled locally because it is unreliable under rootless Podman; in its place every container a Go test starts carries the pid of its test process, and the `SessionStart` hook `.claude/hooks/testdb-reap.ts` removes one whose process is gone. "Reaping orphaned testcontainers" has the rule.
- **[The `app/` unit suite](testing/app-unit.md)**: "Smoothness: gated on causes, not on frame rate" and "Layout: the continuum check, and what a fixture owes it".
- **[The `app/` e2e suite](testing/e2e.md)**: "What the e2e suite costs, and why its workers are capped", "Accessibility: axe on every archetype, and a keyboard walk beside it", "e2e stack" (with "Confirming a flake fixed"), "Reaping orphaned e2e stacks", "Stripe: fakes in CI, the Sandbox by hand", and "Logging in as Staff locally".

## Measuring duplication

`bun run dupes` runs jscpd (pinned in the root `package.json`) with `.jscpd.json` and prints one table: production and test rows, and for each format the clones and duplicated lines, plus totals. It also writes the same numbers to `.jscpd-report/dupes.json` (git-ignored), by kind and format. Measure with this command and no other, so every number comes from the same scope (#2743).

The scope is `app/src` and `api/`, in `typescript`, `svelte`, `css` and `go`, at 50 tokens minimum. It leaves out `api/internal/pdffont/fonts/` (third-party license texts), `api/db/migrations/` (applied migrations are never edited, so their clones cannot be removed) and `app/src/lib/components/atoms/Icon/generated/` (written by `bun run sync-icons`). The Svelte column counts markup only; the script block of a `.svelte` file counts under `typescript`, and its `<style>` under `css`.

A clone counts as test when either side is a test file: `*_test.go`, `*.spec.ts`, `*.test.ts`, a file under an `e2e/`, `testdata/`, `testing/`, `fixtures/`, `harness/` or `vitest-examples/` directory, a fixture or test helper (`*.fixture.ts`, `*Fixture.ts`, `*.testing.ts`, `test[A-Z]*` and `Test[A-Z]*` files, `vitest.*.setup.ts`), a Go fake (`fake.go`, `*_fake.go`), or a file in the Go test-helper packages `testdb`, `authntest`, `apierrtest`, `reqtest` and `fixture`. Every other clone is production. The rules are `isTestFile` in `scripts/dupes.ts`.

### The new-code check

`bun run dupes:check` (the "Check for new duplicated code" step of CI's `format` job, pull requests only) fails a PR when its new or changed code duplicates code that is already there. There is no baseline: the clones already in the tree are not a PR's to fix, only the ones it touches (#2743).

It finds the lines the change adds or changes with `git diff --unified=0` against the merge base with `origin/trunk`, runs the same scan as `bun run dupes` (the same `.jscpd.json` scope), and fails on every clone where either fragment's line range overlaps a changed line, in production and test code alike. Each failure prints both locations and line ranges. A change that touches no scanned file skips the scan and passes. A trunk push has no PR diff, so the step does not run there. Locally, commit or stage your work first (an untracked file is not in the diff), and pass another base as the argument if `origin/trunk` is not the one: `bun run dupes:check origin/some-branch`.

To fix a failure, extract the shared piece so the second copy is a call: `fixture.Scene` for Go test setup, `reqtest.Expect` for request assertions, `pagecursor` for cursor pages, `app/src/testing/routePage.ts` for a route spec's harness, or a new helper beside them. When the two blocks must stay apart, wrap one in jscpd's own markers with the reason on the start line:

```go
// jscpd:ignore-start -- Staff and Client reply differently on purpose
...
// jscpd:ignore-end
```

In Svelte markup write `<!-- jscpd:ignore-start -- reason -->`, in CSS `/* jscpd:ignore-start -- reason */`, and in TypeScript or a Svelte script `//` as in Go. jscpd drops every token from the start line through the end line, so the fragment no longer matches anything. A start with no reason after `--`, or with no end marker after it, fails the check wherever it is in the scanned tree, changed line or not, so a marker cannot be left bare. The reason follows the same rule as `spelling:ignore`: it says why, not that. The rules are `markerProblems`, `parseDiff` and `newClones` in `scripts/dupes-check.ts`.
