# Test plans

One test plan per [Journey](../journeys/). Each file slug matches its journey file
slug, which matches its [persona](../personas/) file slug, one-to-one.

A test plan is the **runnable form of a journey map's interaction layer**. The
experience layer does not become steps — it becomes `journey-gap` issues on the
map itself. A plan therefore proves or disproves the map: every step below is a
claim about the product read out of the code, and **no plan has been executed
yet**. The first run is [#209](https://github.com/markgoho/doula-cloud/issues/209).

## Fixed structure

1. **Header** — journey link, persona link, what a pass means.
2. **Preconditions** — the state the run needs before step 1, and how to reach it
   when the product cannot build it.
3. **Steps** — one small table per stage, mirroring the journey map's stages.
4. **Permission boundary** — where the map carries one (Priya, Lena). Tester-only
   steps, never persona steps.
5. **Marks** — the count, and the run log.

### Step ids

A step keeps the id it has on the journey map (`3.2` is Maya 3.2), so a plan and a
map can be read side by side. Where a test needs a check the map's step does not
name, the check is appended as `3.2-a`. A plan never renumbers a map.

## The three marks

Every step carries exactly one mark.

- **`automated (<spec>)`** — an existing Playwright spec drives this step and
  asserts its result. It counts only when the spec exercises the step the way the
  Persona would, through the UI, or asserts that behaviour directly.
  **Fixture setup does not count.** `birth-plan.e2e.ts` creates its Client with
  `POST /api/practices/{id}/clients`; that automates nothing about the **Add
  Client** form, which stays `manual`. Every spec in the suite provisions its
  Practice through `POST /api/staff/signup`, so **no spec covers the `/signup`
  form** either.
- **`manual`** — a person can walk the step today against the running stack. The
  expected result is what the product does **as built**, which includes a refusal
  or an error where that is the honest answer (`connectRequired`, `402 no credits
  remaining`, a raw enum on screen). A step is `manual` when it can be performed
  and the result observed, whatever the result is.
- **`missing-feature (<gap id>)`** — the step cannot be performed at all: no
  screen, no endpoint, no column. It cites the gap ID **owned by a journey map**.
  A test plan never mints a gap ID. If a run exposes a gap no map owns, it goes
  back to the map that owns the stage.

The line between `manual` and `missing-feature` is *can the step be attempted*,
not *does it give a good answer*. Stripe-gated steps are `manual`: the endpoint
answers, and the answer (`connectRequired`) is the thing under test.

## Running

- **Automated steps**: `bun run test:e2e` in `app/`. The stack — Postgres in
  compose, migrate, the BFF, and the Firebase Auth emulator — is started and
  stopped by `app/e2e/stack.ts`; see [docs/testing.md](../testing.md).
- **Manual steps**: `bun run dev:full` in `app/`, walked in a browser as the
  Persona, on the device the journey names (Priya's stage 6 is a phone).

## Spec inventory

What the suite covers today, as the marks below were assigned from:

| Spec | Covers, through the UI |
| --- | --- |
| `staff-login.e2e.ts` | `/login`, landing on `/practices/[practiceId]` |
| `sign-out.e2e.ts` | Sign out, and a stale second tab losing access |
| `session-cookie.e2e.ts` | The `__session` cookie being set and cleared |
| `plan-templates.e2e.ts` | Plan Templates screen: read seeded, add field, save, per-plan-type isolation |
| `contract-template.e2e.ts` | Contract Template screen: read seeded, edit prose, save, reload |
| `billing.e2e.ts` | Billing screen: credit balance `3` and the `signup_bonus` ledger row |
| `birth-plan.e2e.ts` | Clients list → Engagement page → create, fill and save a Birth Plan → the Client's read-only portal view |
| `portal-invite-accept.e2e.ts` | Client accepting a portal invite and landing on their Engagement |
| `client-portal-login.e2e.ts` | Client portal login |
| `portal-link-landing.e2e.ts` | A signed-in Client following a portal link from another site |
| `portal-sign-out.e2e.ts` | Client sign-out, including a second tab |
| `push-notification.e2e.ts` | A push event waking an open thread tab, which refetches the message |

Nothing in the suite exercises: signup through the form, the Add Client form,
Visits, Contracts per Engagement (build, send, sign), Invoices, the Staff screen,
the invite flow for Staff, or any refusal by role.

## Plans

| Plan | Persona | Journey |
| --- | --- | --- |
| [evaluator-doula.md](evaluator-doula.md) | Tasha Bell | [journey](../journeys/evaluator-doula.md) |
| [solo-birth-doula.md](solo-birth-doula.md) | Maya Okonkwo | [journey](../journeys/solo-birth-doula.md) |
| [practice-owner.md](practice-owner.md) | Renata Alvarez | [journey](../journeys/practice-owner.md) |
| [non-doula-admin.md](non-doula-admin.md) | Dee Whitlock | [journey](../journeys/non-doula-admin.md) |
| [employed-doula.md](employed-doula.md) | Priya Raman | [journey](../journeys/employed-doula.md) |
| [contractor-doula.md](contractor-doula.md) | Lena Vasquez | [journey](../journeys/contractor-doula.md) |

Client-side plans follow the client-side journey maps
([#208](https://github.com/markgoho/doula-cloud/issues/208)).
