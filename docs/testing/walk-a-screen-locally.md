# Walk a screen locally

The recipe for running one spec, the local stack and a seeded Practice from a worktree. Each command is one plain command, in the form the worktree isolation accepts ([`docs/agents/worktree-flow.md`](../agents/worktree-flow.md)): variables go in front of the command they are for, never through `export`.

1. **The Podman socket.** On WSL or Linux with rootless Podman, the shell already exports `DOCKER_HOST` (`unix:///run/user/<uid>/podman/podman.sock`); use that. On a Mac, print the path; `DOCKER_HOST` is `unix://` followed by it:

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

6. **The demo Practice.** `dev:full` seeds Rooted Birth Collective, the simulation's World ([`docs/simulation/worlds/rooted-birth-collective.md`](../simulation/worlds/rooted-birth-collective.md)), each time it starts, once the stack is up and before the dev server starts ([#1994](https://github.com/markgoho/doula-cloud/issues/1994)). Put `SEED_DEMO=0` in front of the command to skip it. The stack starts empty each time, so the seed runs on each start. This is true also after a `dev:full` that was killed without its teardown: that run leaves the database volume, and the Auth emulator keeps nothing, so `dev:full` first removes what such a run left: the stack, and the dev server if it still holds the dev port ([#2010](https://github.com/markgoho/doula-cloud/issues/2010)). A `seed:demo` against a stack in that state stops and tells you to start `dev:full` again; it does not make a second Practice. It goes through the BFF and the Auth emulator, as the e2e fixtures do, and `app/scripts/demo-practice.ts` says what it makes and why. The e2e suite starts its own stack from `global-setup.ts` and never seeds it.

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

   What the Practice holds: the fifteen Staff members, each joined through a real Invitation, with the founding grant of Credits; two thirds of the World's anonymous book (29 Clients across the eight employee Doulas, with due dates that start from the current day) in intake, active and completed care, and 32 Invoices across it, which with Priya's three below make a book of 35, more than one page of the Invoices list: 29 of the 32 by hand, mostly paid by check, bank transfer, cash or other, some owed and not yet due, six overdue (Imani's among them) from 3 to 62 days, deposits and balances on one Contract, a refund, a bounced check, a write-off and a void; and three on Stripe's rail, which the by-hand rail cannot make and the seed makes by writing the database directly (`seedStripeRail` in `app/e2e/stack.ts`, with no call to Stripe): INV-0020 owed and not yet due, INV-0030 overdue, and INV-0019 with a payment on its way, which is past its due date and is not counted overdue ([#2844](https://github.com/markgoho/doula-cloud/issues/2844)). Their Stripe ids begin `demo-seed-`, so they are for reading: an act on one of them would call Stripe with an id Stripe does not have; Priya's four Clients, with a signed Contract and an open Invoice for Camille, a paid Invoice and an overdue Invoice for two of the other three, and a Contract that waits for a signature; Mira, a fifth Client of Priya's that the World does not have, with a signed Contract and a recorded loss; Visits logged and scheduled, Message threads, an Offer that Lena accepted and one that Fern has not answered, the on-call days that come from those due dates, and the Activity all of it records.

7. **A signed-in Staff member, a Client and an Engagement.** With the stack up:

   ```sh
   SEED_CLIENT=1 bun run --cwd app seed:staff-session
   ```

   It prints JSON: the `__session` cookie value, the dev origin, and the Practice, Client and Engagement URLs. Without `SEED_CLIENT=1` it seeds the founding Owner and the Practice only. The Owner has no second factor, the same as after a real signup: since [#1531](https://github.com/markgoho/doula-cloud/issues/1531) she enters her Practice without one, and the second-factor recommendation and the notices on the five acts that need one are shown only to her. To walk one of those five acts (`SecondFactorActs` in `api/internal/staffauth/secondfactor.go`) or the Practice's MFA switch, put `SEED_SECOND_FACTOR=1` in front of the command too: the script then enrolls the emulator's phone factor for her and signs her in with it. Put the cookie into a browser with `context.addCookies([{ name: '__session', value: cookieValue, url: origin, httpOnly: true, secure: false, sameSite: 'Lax' }])` and `page.goto` the URL you need. What this path does and does not exercise, and the founder's `/feedback` variant, are in [`e2e.md`](e2e.md), "Logging in as Staff locally".

8. **Sign in and land on a Staff screen, with a browser tool.** With `dev:full` up (about a minute from start to the dev server's "ready" line; the first page load after that compiles and screenshots blank for a few seconds, so `wait` 5 s first):

   1. Open `<dev origin>/login`, fill `renata.alvarez@sim.doula.cloud` and `demo-password-local-only` (the Owner; no second factor), submit. She lands on `/practices/<practiceId>`; read the id from the URL (it changes on every start).
   2. Navigate to the screen, `/practices/<practiceId>/on-call` or `/schedule`, for example. A signed-in tab keeps its session across navigations, so no cookie step is needed. The cookie route in step 7 is for Playwright, which has no form to fill.

   Walked on 2026-11-08 at offset 1 (`http://localhost:5273`): `on-call` and `schedule` both render the seeded Practice (5 births on call, 22 scheduled visits).

9. **Look at a page at 320px.** In Claude in Chrome, `resize_window` moves the OS window but does not shrink the page: `innerWidth` stayed 1920 after `resize_window` to 320x640, and Chrome will not make a window that narrow. `chrome-devtools`' `emulate` with `viewport: "320x640x2,mobile,touch"` is the real thing where that server is available ([`persona-walk.md`](../agents/persona-walk.md)). Without it, use a same-origin iframe: it is a real 320px CSS viewport, so container and viewport queries, the mobile nav and wrapping all respond as on a phone. The session cookie rides along. From a signed-in tab, `javascript_tool`:

   ```js
   document.body.innerHTML = '';
   const base = location.pathname.replace(/\/[^/]+$/, '');
   for (const [i, p] of ['on-call', 'schedule'].entries()) {
   	const f = document.createElement('iframe');
   	f.src = base + '/' + p;
   	f.style.cssText = `position:fixed;top:8px;left:${8 + i * 342}px;width:320px;height:740px;border:1px solid red`;
   	document.body.appendChild(f);
   }
   ```

   Then `wait` 6 s and `screenshot`. Check overflow with `f.contentDocument.documentElement.scrollWidth <= f.contentWindow.innerWidth`; the iframe's own scrollbar takes about 2px, so `innerWidth` reads 318. Scroll a frame with `f.contentWindow.scrollTo(0, y)`. It does not emulate touch or device pixel ratio; a check that needs them needs `emulate`. Close the tab when done.

10. **Stop the stack** by stopping the background command; `dev:full` takes the stack down on exit. A stack a killed session left behind is collected by the `SessionStart` reaper ([`e2e.md`](e2e.md), "Reaping orphaned e2e stacks").
