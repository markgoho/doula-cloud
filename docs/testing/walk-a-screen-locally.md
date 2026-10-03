# Walk a screen locally

The recipe for running one spec, the local stack and a seeded Practice from a worktree. Each command is one plain command, in the form the worktree isolation accepts ([`docs/agents/worktree-flow.md`](../agents/worktree-flow.md)): variables go in front of the command they are for, never through `export`.

1. **The Podman socket.** Print its path; `DOCKER_HOST` is `unix://` followed by it:

   ```sh
   podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}'
   ```

   Use this value even when your shell profile already exports a `DOCKER_HOST`: a profile value can name another engine's socket.

2. **Container-backed commands** (`go test` against `api/internal/testdb`, `test:e2e`, `dev:full`) take both variables inline:

   ```sh
   DOCKER_HOST=unix://<socket> TESTCONTAINERS_RYUK_DISABLED=true go -C api test ./internal/outbox/
   ```

3. **One `app/` spec.** Run it through `app/`'s own `test:unit` script, so the vitest in `app/node_modules` runs, with `TZ=UTC` because CI runs in UTC and the local gate otherwise runs in your own zone, where a date spec can fail only locally. `bunx vitest` from the worktree root fetches another vitest and writes a lockfile. Use `--project server` for a `*.spec.ts`, `--project client` for a `*.svelte.spec.ts`. From the worktree root, or the same arguments to `bunx vitest` when standing in `app/`:

   ```sh
   TZ=UTC bun run --cwd app test:unit --run --project server src/lib/indent.usage.spec.ts
   ```

4. **Commit with `TZ=UTC` too**, for the same reason: the pre-commit gate runs the whole unit suite. `TZ=UTC git commit -F <message file> -- <paths>`.

5. **The local stack and the dev origin.** Start the stack in the background with step 2's variables in front of `bun run --cwd app dev:full`. The dev server listens on `5173 + 100 × offset`, where the offset is the number in the worktree's `.port-offset` (no file, as in the main checkout, is offset 0): offset 3 is `http://localhost:5473`. `app/e2e/ports.ts` holds every port and the formula.

6. **A signed-in Staff member, a Client and an Engagement.** With the stack up:

   ```sh
   SEED_CLIENT=1 bun run --cwd app seed:staff-session
   ```

   It prints JSON: the `__session` cookie value, the dev origin, and the Practice, Client and Engagement URLs. Without `SEED_CLIENT=1` it seeds the founding Owner and the Practice only. Put the cookie into a browser with `context.addCookies([{ name: '__session', value: cookieValue, url: origin, httpOnly: true, secure: false, sameSite: 'Lax' }])` and `page.goto` the URL you need. What this path does and does not exercise, and the founder's `/feedback` variant, are in [`e2e.md`](e2e.md), "Logging in as Staff locally".

7. **Stop the stack** by stopping the background command; `dev:full` takes the stack down on exit. A stack a killed session left behind is collected by the `SessionStart` reaper ([`e2e.md`](e2e.md), "Reaping orphaned e2e stacks").
