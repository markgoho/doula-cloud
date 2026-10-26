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

6. **The demo Practice.** `dev:full` seeds Rooted Birth Collective, the simulation's World ([`docs/simulation/worlds/rooted-birth-collective.md`](../simulation/worlds/rooted-birth-collective.md)), each time it starts, once the stack is up and before the dev server starts ([#1994](https://github.com/markgoho/doula-cloud/issues/1994)). Put `SEED_DEMO=0` in front of the command to skip it. The stack starts empty each time, so the seed runs on each start. This is true also after a `dev:full` that was killed without its teardown: that run leaves the database volume, and the Auth emulator keeps nothing, so `dev:full` first removes what such a run left ([#2010](https://github.com/markgoho/doula-cloud/issues/2010)). A `seed:demo` against a stack in that state stops and tells you to start `dev:full` again; it does not make a second Practice. It goes through the BFF and the Auth emulator, as the e2e fixtures do, and `app/scripts/demo-practice.ts` says what it makes and why. The e2e suite starts its own stack from `global-setup.ts` and never seeds it.

   Every account signs in with the password `demo-password-local-only`. It is a dev-only password that exists only in the local emulator, and it is kept in `app/scripts/demo-practice.ts` (`DEMO_PASSWORD`). Staff sign in at the real `/login` form on the dev origin (step 5), in any browser:

   | Account | Email |
   | --- | --- |
   | Owner (also Admin and Doula) | `renata.alvarez@sim.doula.cloud` |
   | Admin, not a Doula | `dee.whitlock@sim.doula.cloud` |
   | Employee Doula | `priya.raman@sim.doula.cloud` |
   | Contractor Doula | `lena.vasquez@sim.doula.cloud` |

   The other eleven Rooted Staff members sign in the same way, at `given.family@sim.doula.cloud` (`joss.adeyemi@sim.doula.cloud`, for example). Two Clients sign in to the portal:

   | Client | Email | What the portal shows |
   | --- | --- | --- |
   | Camille Boyd | `camille.boyd@sim.doula.cloud` | Care in progress: a signed Contract, an open Invoice, Visits and Messages |
   | Mira Castellanos | `mira.castellanos@sim.doula.cloud` | After a loss: Priya recorded "The pregnancy ended without a living baby" six days ago ([#2007](https://github.com/markgoho/doula-cloud/issues/2007)) |

   A Client has no password: she signs in with a magic link. The seed prints a link for each Client. A link lasts 15 minutes and works once. For a new one, type her email at `/portal/login` on the dev origin, or run `bun run --cwd app seed:demo`. That command seeds the demo Practice against a running stack, or finds it already there and changes nothing, and prints the accounts and new links.

   **Local mail.** While `dev:full` runs, it drains every mail outbox each 3 seconds through `POST /api/internal/outboxes/drain`, the route the deployed `process-outbox-drain` job calls each 5 minutes ([#2010](https://github.com/markgoho/doula-cloud/issues/2010)). So the mail an act in the browser sends (a magic link, a Staff Invitation, a password reset) arrives in the local mail catcher, the sandbox mailbox, at `http://127.0.0.1:<19080 + 100 × offset>/`. Offset 3 is `http://127.0.0.1:19380/`. The catcher lists each address; one address's mail is at `/inbox/<address>`, for example `/inbox/camille.boyd%40sim.doula.cloud`. `dev:full` prints the catcher's URL with the demo accounts. No mail goes to a real address: the stack sends all of it to the catcher.

   What the Practice holds: the fifteen Staff members, each joined through a real Invitation, with the founding grant of Credits; two thirds of the World's anonymous book (29 Clients across the eight employee Doulas, with due dates that start from the current day) in intake, active and completed care; Priya's four Clients, with a signed Contract and an open Invoice for Camille, a paid Invoice and an overdue Invoice for two of the other three, and a Contract that waits for a signature; Mira, a fifth Client of Priya's that the World does not have, with a signed Contract and a recorded loss; Visits logged and scheduled, Message threads, an Offer that Lena accepted and one that Fern has not answered, the on-call days that come from those due dates, and the Activity all of it records.

7. **A signed-in Staff member, a Client and an Engagement.** With the stack up:

   ```sh
   SEED_CLIENT=1 bun run --cwd app seed:staff-session
   ```

   It prints JSON: the `__session` cookie value, the dev origin, and the Practice, Client and Engagement URLs. Without `SEED_CLIENT=1` it seeds the founding Owner and the Practice only. The Owner has no second factor, the same as after a real signup: since [#1531](https://github.com/markgoho/doula-cloud/issues/1531) she enters her Practice without one, and the second-factor recommendation and the notices on the five acts that need one are shown only to her. To walk one of those five acts (`SecondFactorActs` in `api/internal/staffauth/secondfactor.go`) or the Practice's MFA switch, put `SEED_SECOND_FACTOR=1` in front of the command too: the script then enrolls the emulator's phone factor for her and signs her in with it. Put the cookie into a browser with `context.addCookies([{ name: '__session', value: cookieValue, url: origin, httpOnly: true, secure: false, sameSite: 'Lax' }])` and `page.goto` the URL you need. What this path does and does not exercise, and the founder's `/feedback` variant, are in [`e2e.md`](e2e.md), "Logging in as Staff locally".

8. **Stop the stack** by stopping the background command; `dev:full` takes the stack down on exit. A stack a killed session left behind is collected by the `SessionStart` reaper ([`e2e.md`](e2e.md), "Reaping orphaned e2e stacks").
