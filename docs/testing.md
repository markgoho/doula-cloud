# Testing infrastructure

Testing is split by branch. Read the part for the tree you are changing; each section named below lives only in that part.

- **[Walk a screen locally](testing/walk-a-screen-locally.md)**: the Podman socket, the inline `DOCKER_HOST` form, one `app/` spec, `TZ=UTC`, the dev origin under a port offset, and a seeded Staff member, Client and Engagement.
- **[The pre-commit hook and the CI gates](testing/gates.md)**: "Pre-commit hook", "Formatting: which trees are gated, and by what", "Typecheck: which trees are gated, and by what", "`docs/`: the specs that read it run in their own CI job", "The memory this gate costs, and why the browser pool is capped" (with "Only one session runs this step at a time" and "When a commit is killed for memory"), "Coverage: 100% line coverage, with justified exceptions", and "Reading a failed CI run".
- **[`api/` (Go)](testing/api.md)**: "lint with golangci-lint, matching CI exactly", "Toolchain versions: local must match CI exactly", "real Postgres for tests, container-engine-agnostic" (with "A due-time fixture must not compare two clocks"), "Reaping orphaned testcontainers", and "migrations via goose".
- **[The `app/` unit suite](testing/app-unit.md)**: "Smoothness: gated on causes, not on frame rate" and "Layout: the continuum check, and what a fixture owes it".
- **[The `app/` e2e suite](testing/e2e.md)**: "What the e2e suite costs, and why its workers are capped", "Accessibility: axe on every archetype, and a keyboard walk beside it", "e2e stack" (with "Confirming a flake fixed"), "Reaping orphaned e2e stacks", "Stripe: fakes in CI, the Sandbox by hand", and "Logging in as Staff locally".
