# Test plans

One test plan per [Journey](../journeys/). Each file slug matches its journey file
slug, which matches its [persona](../personas/) file slug, one-to-one.

A test plan is the **runnable form of a journey map's interaction layer**. The
experience layer does not become steps — it becomes `journey-gap` issues on the
map itself. A plan therefore proves or disproves the map: every step below is a
claim about the product read out of the code.

**Stripe status (2026-08-22, second update):** the Sandbox exists and **Buy
credits is walked and passing** — see Maya's 3.4-a. Connect's code leg is now
built too: [#247](https://github.com/markgoho/doula-cloud/issues/247) moved it
to Accounts v2 and both Sandbox event destinations are created, so
`POST .../payments/connect` no longer 401s.

**Connect and Invoices are now walked too** (2026-08-22). A Practice onboarded
through the v2 Account Link to an active account, raised a $1,800 Invoice, and a
Client paid it with a test card; `invoice.paid` created the `payments` row.

**The practice side now has no `blocked` step at all.** Dee's 8.1 was the last one,
and [#236](https://github.com/markgoho/doula-cloud/issues/236) cleared it by
onboarding a second Practice end to end: `card_payments` and `payouts` both
`active`, written by the `capability_status_updated` thin event, after which a
**non-owner Admin** raised a `$900` Invoice. The recipe is kept at
[connect-onboarding.md](connect-onboarding.md) so no later walk re-derives it.

**On the CAPTCHA.** That walk first hit one and recorded the step as closed to
automation. That was wrong twice over, and the corrected reading is the useful
one. The variable is **whose browser**, not headless-vs-headed and not whether a
person is watching:

| Browser | Result |
| --- | --- |
| Playwright-launched Chromium, headless | CAPTCHA at the email step |
| Playwright-launched Chromium, headed | CAPTCHA at the password step |
| `playwriter` driving the user's own Chrome | **no CAPTCHA on any of the nine screens** |

Stripe is fingerprinting the automation-launched browser, not the absence of a
human. The run that completed was driven end to end by `playwriter` and was never
challenged, so **a Connect step is walkable unattended provided it is driven
through the user's real Chrome** — which makes it `manual`, not `blocked`. A walk
still never works around an anti-automation control; it uses a browser that does
not trip one, and hands over to the human if that fails.

The walk earned its keep: it found four defects that reading could not. Two were
config that reported itself healthy while delivering nothing (`events_from` on
the event destination, and `--forward-thin-connect-to` in `stripe-listen.sh`),
and two were only visible to the Client (an invoice that said `From DOULA.CLOU`,
and `payments` rows with no Stripe reference). One of them passed its unit tests
because the fixture supplied a field production never sends.

**Run status (2026-08-22):** the automated steps of all nine plans are run and
**all pass** — `bun run test:e2e`, 16 passed, 0 failed. One walk ticket per plan
carries the rest ([#233](https://github.com/markgoho/doula-cloud/issues/233)–[#241](https://github.com/markgoho/doula-cloud/issues/241)),
and each plan's **Run log** names its own. **The six practice-side plans are walked** — Tasha
Bell's, Maya Okonkwo's, Renata Alvarez's, Dee Whitlock's, Priya Raman's and Lena
Vasquez's
([#233](https://github.com/markgoho/doula-cloud/issues/233),
[#234](https://github.com/markgoho/doula-cloud/issues/234),
[#235](https://github.com/markgoho/doula-cloud/issues/235),
[#236](https://github.com/markgoho/doula-cloud/issues/236),
[#237](https://github.com/markgoho/doula-cloud/issues/237),
[#238](https://github.com/markgoho/doula-cloud/issues/238)). **Two of the three
client-side plans are walked** — Nadia Haddad's and Hannah Sorensen's
([#239](https://github.com/markgoho/doula-cloud/issues/239),
[#240](https://github.com/markgoho/doula-cloud/issues/240)); Camille Boyd's is
not. Filing the `journey-gap` issues stays
[#209](https://github.com/markgoho/doula-cloud/issues/209), which waits on all
nine walks.

A walk may also **falsify an expected result** without moving its mark. Renata's
1.2, 1.3 and 4.3 each stayed `manual` — the step is performable, and what the
product did when it was performed is simply not what the plan claimed. The cell is
corrected in place and the finding goes to the owning map (RA-G9, RA-G10), the
same treatment Maya's 5.2 got.

A walk may also **add a step**. Dee's 7.2-a came out of voiding a Contract and then
noticing **Create Invoice** still rendered on it — a check no cell had named. A new
`-a` id is the same move as a plan appending a check the map's step does not name;
the map still owns the gap it mints (DW-G7). Priya's 5.2-a is the same move from the
other direction: her 5.2 tested only whether she could *read* a Contract's money, so
the walk appended the write, and she set a price and sent the Contract (PR-G8).

A walk may **narrow a gap rather than confirm or delete it**. Priya's PR-G3 claimed
the `doula` role "is never read anywhere in the codebase". A 21-endpoint battery run
at `roles = '{}'` and again at `['doula']` found it read in exactly one place —
`visit/roles.go` — gating the one act her journey is named for. The gap survives in
narrowed form on the map that owns it, the same treatment TB-G7 got.

**One claim ran through four plans before a walk could settle it.** Renata's 1.2,
Dee's 1.3 and Priya's 3.2 each expected a Practice picker and each found none, and
each walk recorded it as unwalkable rather than missing. Lena's walk rendered it:
signing in with two memberships lists both Practices under `Choose a Practice`, on
`/login` itself. The picker was never absent — every other Persona holds exactly one
membership, and LV-G2 makes a second unreachable through the product, so only the
fixture bypass in Lena's Preconditions can produce the state it needs. A claim no
single plan could test is worth carrying across plans rather than deleting.

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
4. **Permission boundary** — where the plan carries one. Only Priya's does
   (PR-B1 to PR-B6). Tester-only steps, never persona steps.
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
  have is absent.

  **The Stripe account now exists** ([#242](https://github.com/markgoho/doula-cloud/issues/242)),
  and Credits went `blocked` -> `manual` on 2026-08-22: Maya's 3.4-a buys two
  credits through Stripe Checkout and the ledger is credited by the
  `checkout.session.completed` webhook. Connect and Invoices did **not** clear,
  and the reason has changed in a way the mark does not fit cleanly.

  **The rule needs restating, because reality moved.** The old test — *would
  connecting the third-party infrastructure clear it* — assumed the only way a
  complete code path can fail is that the infrastructure is missing. Connecting
  Stripe did not clear Connect: Stripe refused `POST /v1/accounts` for new
  integrations while every merged Connect path was Accounts v1. The
  infrastructure was present and our code could not use it.

  [#247](https://github.com/markgoho/doula-cloud/issues/247) has since fixed the
  code, so that particular gap is closed — but the restated rule below is what
  caught it, and it holds for the third reason Connect is still `blocked`: the
  walk itself has not been done.

  So `blocked` now means: **the step cannot finish for a reason outside the
  screen it is walked from, and no product decision is missing.** A step whose
  code targets a withdrawn third-party API is `blocked`, not
  `missing-feature` — the feature was specified and built, and what it needs is
  a migration, not a decision. A walk that meets one records *which* of the two
  reasons applies, because they clear on completely different work.

  It was briefly two. Maya's walk found Contract signing answering a bare 500,
  because signing writes the PDF to the object store before it writes the status
  (`api/internal/contracts/sign.go:85-89`) and the stack pointed the SDK at
  `storage-emulator-disabled.invalid:1`. That was the harness, not the product —
  the deployed service has the real bucket — so `compose.e2e.yaml` gained a
  `fake-gcs-server` and the steps went back to `manual`
  ([#234](https://github.com/markgoho/doula-cloud/issues/234)). **Prefer that fix
  to the mark**: where the missing infrastructure can be stood up locally, stand
  it up, and keep `blocked` for the one thing that cannot be (a Stripe account is
  a business relationship, not a container). Dee's walk
  ([#236](https://github.com/markgoho/doula-cloud/issues/236)) drew the second
  half of that line and then erased it: Stripe's hosted onboarding CAPTCHA'd a
  Playwright-launched browser in both headless and headed mode, and let the user's
  own Chrome through `playwriter` straight past, unchallenged, on every screen. So
  what looked like a closed door was **the wrong browser**. A walk never works
  around an anti-automation control; it uses one that does not trip it.

  The expected result is still the real as-built response, so the step is walked
  and observed like a `manual` one — the mark exists so the first run's numbers do
  not read a bill we have not paid as a hole in the product. **That response is
  not always graceful**: the Invoice leg answers `connectRequired` on a Practice
  that has not connected. The bare `internal error` (HTTP 500) that Buy credits
  and Connect Stripe both used to answer was the missing API key, and is gone —
  both legs were walked end to end on 2026-08-22.
- **`missing-feature (<gap id>)`** — the step cannot be performed at all: no
  screen, no endpoint, no column. It cites the gap ID **owned by a journey map**.
  A test plan never mints a gap ID. If a run exposes a gap no map owns, it goes
  back to the map that owns the stage.

The line between `manual` and `missing-feature` is *can the step be attempted*,
not *does it give a good answer*. A step whose honest as-built result is a refusal
(`402 no credits remaining`, a role refusal, a raw enum on screen) is `manual`.

The line between `blocked` and `missing-feature` is **is a product decision
missing** — if not, it is `blocked`, whether what it waits on is infrastructure
we have not stood up or a third-party API we have not migrated to. Dee's stage 9
is the case that fixes the rule in mind:
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
| `signup-form.e2e.ts` | The `/signup` form itself: fill, submit, land on `/practices/[practiceId]`, then a fresh `/login` with the same credentials |
| `staff-invite-role.e2e.ts` | The Staff invite flow end to end (send through `/invite`, accept through `/accept-invite`) for a Doula who is not the Owner, then a `403` on an Owner-only action from her session |
| `contract-lifecycle.e2e.ts` | A Contract's full lifecycle: build and send on the Practice side, the Client signing it in the portal, and the Signed PDF coming back |
| `add-client-visits.e2e.ts` | The Add Client intake form (#497) and an Engagement's Visits section: add a Visit, read it back |

Nothing in the suite exercises: Invoices, or the Staff roster screen (`/practices/[practiceId]/staff`).

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

## The run, and the gap issues

Every plan has been executed once ([#209](https://github.com/markgoho/doula-cloud/issues/209)): the automated specs in one suite run, then nine manual walks, one per plan ([#233](https://github.com/markgoho/doula-cloud/issues/233)–[#241](https://github.com/markgoho/doula-cloud/issues/241)). Each plan carries its own dated **Run log**.

| Plan | Persona | `automated` | `manual` | `blocked` | `missing-feature` |
| --- | --- | --- | --- | --- | --- |
| [evaluator-doula.md](evaluator-doula.md) | Tasha Bell | 4 | 10 | 0 | 5 |
| [solo-birth-doula.md](solo-birth-doula.md) | Maya Okonkwo | 12 | 16 | 0 | 4 |
| [practice-owner.md](practice-owner.md) | Renata Alvarez | 5 | 16 | 0 | 7 |
| [non-doula-admin.md](non-doula-admin.md) | Dee Whitlock | 0 | 21 | 0 | 4 |
| [employed-doula.md](employed-doula.md) | Priya Raman | 3 | 21 | 0 | 6 |
| [contractor-doula.md](contractor-doula.md) | Lena Vasquez | 1 | 17 | 0 | 9 |
| [loss-client.md](loss-client.md) | Nadia Haddad | 5 | 13 | 0 | 9 |
| [first-time-client.md](first-time-client.md) | Hannah Sorensen | 6 | 18 | 0 | 8 |
| [returning-postpartum-client.md](returning-postpartum-client.md) | Camille Boyd | 1 | 9 | 0 | 9 |
| **Total** | | **37** | **141** | **0** | **61** |

Every `automated` step passed. **No plan carries a `blocked` step any more** — Stripe was the last holdout and [#242](https://github.com/markgoho/doula-cloud/issues/242) opened the Sandbox, after which Connect, Checkout and Invoices were all walked for real.

**#318 added four specs** (`signup-form.e2e.ts`, `staff-invite-role.e2e.ts`,
`contract-lifecycle.e2e.ts`, `add-client-visits.e2e.ts`) closing the seams the
first journey run's [Spec inventory](#spec-inventory) named. The `automated`
counts above move for evaluator-doula.md, solo-birth-doula.md and
employed-doula.md, whose Steps tables named an exact step
(`/signup`'s three steps; PR-B2's role refusal) that a new spec now drives
identically. `contract-lifecycle.e2e.ts` and `add-client-visits.e2e.ts` cover
real steps on several other plans too, but this pass did not re-mark them:
those plans describe the Add Client form and the Contract lifecycle in
shapes ADR-0017 and #234 have already moved past (a combined
Client-and-Engagement create with an immediate credit spend; Contract
signing as untestable), so flipping their marks without rewriting their own
stale narrative would trade one inaccuracy for another. That reconciliation
is left for whoever next walks those plans.

### Gap issues

One `journey-gap` issue per missing or broken capability, deduplicated across the nine plans — a capability that fails on several journeys has one issue, not one per plan. None carries a severity or a priority: the project is pre-launch and everything here is fixed before launch.


**One entry point**: [#328](https://github.com/markgoho/doula-cloud/issues/328) holds the whole backlog and carries the dependency reading — which fixes close several tickets at once, and which decisions gate the rest. Behind all of it, [#329](https://github.com/markgoho/doula-cloud/issues/329) walks these nine plans **again** once every gap is closed, because a closed gap issue says a capability exists, not that the Persona can get through.

**Grouped by Persona.** Each gap issue is a GitHub sub-issue of the parent for the journey that found it, so the tracker shows one progress bar per Persona:

| Parent | Persona | Gaps here |
| --- | --- | --- |
| [#319](https://github.com/markgoho/doula-cloud/issues/319) | [Tasha Bell (the evaluator doula)](evaluator-doula.md) | 7 |
| [#320](https://github.com/markgoho/doula-cloud/issues/320) | [Maya Okonkwo (the solo birth doula)](solo-birth-doula.md) | 10 |
| [#321](https://github.com/markgoho/doula-cloud/issues/321) | [Renata Alvarez (the multi-doula practice owner)](practice-owner.md) | 9 |
| [#322](https://github.com/markgoho/doula-cloud/issues/322) | [Dee Whitlock (the non-doula Admin)](non-doula-admin.md) | 8 |
| [#323](https://github.com/markgoho/doula-cloud/issues/323) | [Priya Raman (the employed doula)](employed-doula.md) | 7 |
| [#324](https://github.com/markgoho/doula-cloud/issues/324) | [Lena Vasquez (the contractor doula)](contractor-doula.md) | 2 |
| [#325](https://github.com/markgoho/doula-cloud/issues/325) | [Nadia Haddad (the client whose pregnancy ends in loss)](loss-client.md) | 7 |
| [#326](https://github.com/markgoho/doula-cloud/issues/326) | [Hannah Sorensen (the first-time pregnant client)](first-time-client.md) | 7 |
| [#327](https://github.com/markgoho/doula-cloud/issues/327) | [Camille Boyd (the returning postpartum-only client)](returning-postpartum-client.md) | 6 |

| Gap | Issue | Parent |
| --- | --- | --- |
| MO-G1 | [#250](https://github.com/markgoho/doula-cloud/issues/250) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G2 | [#251](https://github.com/markgoho/doula-cloud/issues/251) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G3 | [#252](https://github.com/markgoho/doula-cloud/issues/252) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G4 | [#253](https://github.com/markgoho/doula-cloud/issues/253) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G5 | [#254](https://github.com/markgoho/doula-cloud/issues/254) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G6 | [#255](https://github.com/markgoho/doula-cloud/issues/255) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G7 | [#256](https://github.com/markgoho/doula-cloud/issues/256) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G9 | [#257](https://github.com/markgoho/doula-cloud/issues/257) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G10 | [#258](https://github.com/markgoho/doula-cloud/issues/258) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| MO-G11 | [#259](https://github.com/markgoho/doula-cloud/issues/259) | [Maya Okonkwo (the solo birth doula)](https://github.com/markgoho/doula-cloud/issues/320) |
| RA-G1 | [#260](https://github.com/markgoho/doula-cloud/issues/260) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G2 | [#261](https://github.com/markgoho/doula-cloud/issues/261) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G3 | [#262](https://github.com/markgoho/doula-cloud/issues/262) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G5 | [#263](https://github.com/markgoho/doula-cloud/issues/263) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G6 | [#264](https://github.com/markgoho/doula-cloud/issues/264) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G7 | [#265](https://github.com/markgoho/doula-cloud/issues/265) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G8 | [#266](https://github.com/markgoho/doula-cloud/issues/266) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G9 | [#267](https://github.com/markgoho/doula-cloud/issues/267) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| RA-G10 | [#268](https://github.com/markgoho/doula-cloud/issues/268) | [Renata Alvarez (the multi-doula practice owner)](https://github.com/markgoho/doula-cloud/issues/321) |
| DW-G1 | [#269](https://github.com/markgoho/doula-cloud/issues/269) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G2 | [#270](https://github.com/markgoho/doula-cloud/issues/270) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G3 | [#271](https://github.com/markgoho/doula-cloud/issues/271) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G4 | [#272](https://github.com/markgoho/doula-cloud/issues/272) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G5 | [#273](https://github.com/markgoho/doula-cloud/issues/273) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G6 | [#274](https://github.com/markgoho/doula-cloud/issues/274) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G7 | [#275](https://github.com/markgoho/doula-cloud/issues/275) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| DW-G8 | [#276](https://github.com/markgoho/doula-cloud/issues/276) | [Dee Whitlock (the non-doula Admin)](https://github.com/markgoho/doula-cloud/issues/322) |
| PR-G2 | [#277](https://github.com/markgoho/doula-cloud/issues/277) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G3 | [#278](https://github.com/markgoho/doula-cloud/issues/278) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G4 | [#279](https://github.com/markgoho/doula-cloud/issues/279) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G5 | [#280](https://github.com/markgoho/doula-cloud/issues/280) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G6 | [#281](https://github.com/markgoho/doula-cloud/issues/281) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G8 | [#282](https://github.com/markgoho/doula-cloud/issues/282) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| PR-G9 | [#283](https://github.com/markgoho/doula-cloud/issues/283) | [Priya Raman (the employed doula)](https://github.com/markgoho/doula-cloud/issues/323) |
| TB-G1 | [#284](https://github.com/markgoho/doula-cloud/issues/284) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G2 | [#285](https://github.com/markgoho/doula-cloud/issues/285) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G3 | [#286](https://github.com/markgoho/doula-cloud/issues/286) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G4 | [#287](https://github.com/markgoho/doula-cloud/issues/287) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G5 | [#288](https://github.com/markgoho/doula-cloud/issues/288) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G6 | [#289](https://github.com/markgoho/doula-cloud/issues/289) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| TB-G7 | [#290](https://github.com/markgoho/doula-cloud/issues/290) | [Tasha Bell (the evaluator doula)](https://github.com/markgoho/doula-cloud/issues/319) |
| LV-G8 | [#291](https://github.com/markgoho/doula-cloud/issues/291) | [Lena Vasquez (the contractor doula)](https://github.com/markgoho/doula-cloud/issues/324) |
| LV-G9 | [#292](https://github.com/markgoho/doula-cloud/issues/292) | [Lena Vasquez (the contractor doula)](https://github.com/markgoho/doula-cloud/issues/324) |
| NH-G1 | [#293](https://github.com/markgoho/doula-cloud/issues/293) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G2 | [#294](https://github.com/markgoho/doula-cloud/issues/294) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G3 | [#295](https://github.com/markgoho/doula-cloud/issues/295) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G4 | [#296](https://github.com/markgoho/doula-cloud/issues/296) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G6 | [#297](https://github.com/markgoho/doula-cloud/issues/297) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G7 | [#298](https://github.com/markgoho/doula-cloud/issues/298) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| NH-G8 | [#299](https://github.com/markgoho/doula-cloud/issues/299) | [Nadia Haddad (the client whose pregnancy ends in loss)](https://github.com/markgoho/doula-cloud/issues/325) |
| HS-G1 | [#300](https://github.com/markgoho/doula-cloud/issues/300) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G2 | [#301](https://github.com/markgoho/doula-cloud/issues/301) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G3 | [#302](https://github.com/markgoho/doula-cloud/issues/302) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G4 | [#303](https://github.com/markgoho/doula-cloud/issues/303) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G5 | [#304](https://github.com/markgoho/doula-cloud/issues/304) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G6 | [#305](https://github.com/markgoho/doula-cloud/issues/305) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| HS-G7 | [#306](https://github.com/markgoho/doula-cloud/issues/306) | [Hannah Sorensen (the first-time pregnant client)](https://github.com/markgoho/doula-cloud/issues/326) |
| CB-G1 | [#307](https://github.com/markgoho/doula-cloud/issues/307) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |
| CB-G2 | [#308](https://github.com/markgoho/doula-cloud/issues/308) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |
| CB-G3 | [#309](https://github.com/markgoho/doula-cloud/issues/309) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |
| CB-G4 | [#310](https://github.com/markgoho/doula-cloud/issues/310) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |
| CB-G5 | [#311](https://github.com/markgoho/doula-cloud/issues/311) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |
| CB-G6 | [#312](https://github.com/markgoho/doula-cloud/issues/312) | [Camille Boyd (the returning postpartum-only client)](https://github.com/markgoho/doula-cloud/issues/327) |

**Not filed here, and why:**

| Gap | Where it lives instead |
| --- | --- |
| LV-G1, LV-G2, LV-G3, LV-G4, LV-G5, LV-G6, LV-G7, RA-G4, PR-G1 | [Who a Staff member is to a Practice, and which work is theirs](https://github.com/markgoho/doula-cloud/issues/225) owns these outright and carries execution; its tickets are their issues |
| NH-G5 | Folded into [#212](https://github.com/markgoho/doula-cloud/issues/212), which already owns the Client register, with two acceptance criteria added for it |
| MO-G8 | Not a product capability. A Stripe Sandbox account exists ([#242](https://github.com/markgoho/doula-cloud/issues/242)) and a Practice was onboarded to `card_payments: active` on Dee's walk |

**PR-G7** is folded into **RA-G1** ([#260](https://github.com/markgoho/doula-cloud/issues/260)) — the experience half of the same missing email, as Priya's own map directs. The sending capability is being charted separately at [#213](https://github.com/markgoho/doula-cloud/issues/213); #260 is the journey record that both invitations need it.
