# Test plans

One test plan per [Journey](../journeys/). Each file slug matches its journey file
slug, which matches its [persona](../personas/) file slug, one-to-one.

A test plan is the **runnable form of a journey map's interaction layer**. The
experience layer does not become steps — it becomes `journey-gap` issues on the
map itself. A plan therefore proves or disproves the map: every step below is a
claim about the product read out of the code.

**Run status (2026-08-22):** the automated steps of all nine plans are run and
**all pass** — `bun run test:e2e`, 16 passed, 0 failed. One walk ticket per plan
carries the rest ([#233](https://github.com/markgoho/doula-cloud/issues/233)–[#241](https://github.com/markgoho/doula-cloud/issues/241)),
and each plan's **Run log** names its own. **Tasha Bell's and Maya Okonkwo's plans are walked**
([#233](https://github.com/markgoho/doula-cloud/issues/233),
[#234](https://github.com/markgoho/doula-cloud/issues/234)); the other seven are
not. Filing the `journey-gap` issues stays
[#209](https://github.com/markgoho/doula-cloud/issues/209), which waits on all
nine walks.

A walk may **re-mark a step**. Tasha's 3.3-a went from `missing-feature (TB-G7)`
to `manual` once the Staff screen turned out to answer it — the rule that a mark
is a claim read out of the code cuts both ways, and the run is what settles it.
Re-marking is not minting: the gap ID stays owned by its journey map, and its
wording is corrected there.

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

## The four marks

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
- **`blocked`** — the code path is complete and the step can be attempted, but it
  cannot finish because third-party infrastructure the walking stack does not
  have is absent. Today that is **only Stripe** again: no account exists
  (`CLAUDE.md`), so Credits, Connect, and Invoices are all `blocked`.

  It was briefly two. Maya's walk found Contract signing answering a bare 500,
  because signing writes the PDF to the object store before it writes the status
  (`api/internal/contracts/sign.go:85-89`) and the stack pointed the SDK at
  `storage-emulator-disabled.invalid:1`. That was the harness, not the product —
  the deployed service has the real bucket — so `compose.e2e.yaml` gained a
  `fake-gcs-server` and the steps went back to `manual`
  ([#234](https://github.com/markgoho/doula-cloud/issues/234)). **Prefer that fix
  to the mark**: where the missing infrastructure can be stood up locally, stand
  it up, and keep `blocked` for the one thing that cannot be (a Stripe account is
  a business relationship, not a container).

  The expected result is still the real as-built response, so the step is walked
  and observed like a `manual` one — the mark exists so the first run's numbers do
  not read a bill we have not paid as a hole in the product. **That response is
  not always graceful**: the Invoice leg answers `connectRequired`, but Buy
  credits and Connect Stripe both answer a bare `internal error` (HTTP 500).
- **`missing-feature (<gap id>)`** — the step cannot be performed at all: no
  screen, no endpoint, no column. It cites the gap ID **owned by a journey map**.
  A test plan never mints a gap ID. If a run exposes a gap no map owns, it goes
  back to the map that owns the stage.

The line between `manual` and `missing-feature` is *can the step be attempted*,
not *does it give a good answer*. A step whose honest as-built result is a refusal
(`402 no credits remaining`, a role refusal, a raw enum on screen) is `manual`.

The line between `blocked` and `missing-feature` is **would connecting the
third-party infrastructure clear it** — a live Stripe account, or a reachable
object store. It clears every `blocked` step and not one
`missing-feature` step — Dee's stage 9 is the case that fixes the rule in mind:
recording a bank transfer stays impossible with Stripe connected, so it is
`missing-feature (DW-G3)`, while the Invoice she cannot raise one stage earlier is
`blocked`.

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
| [loss-client.md](loss-client.md) | Nadia Haddad | [journey](../journeys/loss-client.md) |
| [first-time-client.md](first-time-client.md) | Hannah Sorensen | [journey](../journeys/first-time-client.md) |
| [returning-postpartum-client.md](returning-postpartum-client.md) | Camille Boyd | [journey](../journeys/returning-postpartum-client.md) |

The first six are practice-side ([#207](https://github.com/markgoho/doula-cloud/issues/207)),
the last three client-side ([#208](https://github.com/markgoho/doula-cloud/issues/208)).
**No plan carries a `blocked` step on the client side**: Stripe never reaches the
Client portal, so every hole a Client meets is a hole in the product.
