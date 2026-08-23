# Dee Whitlock — test plan

- **Journey**: [non-doula-admin.md](../journeys/non-doula-admin.md)
- **Persona**: [non-doula-admin.md](../personas/non-doula-admin.md)
- **A pass means**: an Engagement with a Doula assigned, a signed Contract, and an
  Invoice with a recorded Payment — reached without Dee opening a Care Plan or
  logging a Visit.

## Preconditions

- A Practice with an Owner (Renata) and at least one other Staff member holding
  `doula`, so there is somebody for Dee to assign.
- An invitation issued by the Owner, and its link (printed on the invite screen —
  no email is sent).
- Dee's role must be set with `PATCH /api/practices/{id}/staff/{staffId}/roles`;
  no screen does it (RA-G2). **Run the plan twice** — once with `roles = '{}'` as
  acceptance leaves it, once with `office_manager` — because the two runs are
  expected to be identical (DW-G1).

## Steps

### Stage 1 — Accept the invitation

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link at `/accept-invite` | The accept form renders | `manual` |
| 1.2 | Set email and password, press **Accept invite** | `POST /api/staff/accept-invite` creates the membership with zero roles | `manual` |
| 1.3 | Choose the Practice from the membership list | **Nothing to choose** — one membership, so `decideLanding` redirects straight to `/practices/[practiceId]` (`app/src/lib/landing.ts:24-26`) and the picker never renders. Unwalkable for anyone until LV-G2 | `manual` |

### Stage 2 — Receive the Admin role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | As the Owner, set `office_manager` on Dee's membership from a screen | No screen does this | `missing-feature (RA-G2)` |
| 2.1-a | Set it with `PATCH .../staff/{staffId}/roles` instead | Succeeds for an Owner; the Staff screen then prints `office_manager`, not "Admin" (RA-G3) | `manual` |

### Stage 3 — Discover what the Admin role grants

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Land on `/practices/[practiceId]` | **Three** tiles: Clients, Billing and **Payments**. Four tiles are owner-gated; Payments sits outside the block (RA-G9) | `manual` |
| 3.1-a | Compare this run against the zero-role run | **Identical.** `office_manager` is read nowhere in the codebase; the role grants nothing and withholds nothing | `manual` |
| 3.2 | Open Billing | The Practice's credit balance and purchase ledger render for a non-owner — `billing/balance.go` takes any Staff member (DW-G4) | `manual` |
| 3.2-a | Attempt to buy credits | Refused: `billing/purchase.go` requires Owner. **This refusal is correct** | `manual` |

The Billing screen is automated for an *Owner* (`billing.e2e.ts`); no spec reads it
as a non-owner, which is the case that matters here.

### Stage 4 — Take the call and create the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients/new` | Two fields, Name and Email — against a page of notes from the call (MO-G3) | `manual` |
| 4.2 | Press **Add Client** | `POST .../clients` is not owner-gated, so it **passes** for Dee; Client and Engagement are created at `intake` | `manual` |

### Stage 5 — Assign a Doula

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open the Engagement and look for an assignment control | None exists | `missing-feature (RA-G4)` |
| 5.2 | Create a Visit naming the Doula instead | **Refused.** `POST .../visits` requires the Doula role (`api/internal/visit/roles.go:41`); **Add a Visit** renders for Dee and answers `403 only a Staff member with the Doula role can do that` (DW-G6) | `manual` |

### Stage 6 — Send the Contract

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Send the portal invite | `POST .../portal-invite` succeeds for a non-owner | `manual` |
| 6.2 | Build the Contract | `POST .../contract` creates it at `draft` — not owner-gated | `manual` |
| 6.3 | Send it | Status `sent` | `manual` |
| 6.3-a | Try to edit the Practice's **Contract Template** | Refused: `contracts/template.go:75` requires Owner. The page still renders and shows the real template first (PR-G4) | `manual` |

### Stage 7 — Track the signature

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open the Engagement and read the Contract status | One of `draft` / `sent` / `signed` / `voided` | `manual` |
| 7.1-a | Void a **signed** Contract from the Engagement page | The **Void** button renders only on a `signed` Contract (`ContractStatus.svelte`) and `POST .../contract/void` succeeds for a non-owner — `contracts/void.go:30` has no role check, only `staffauth.Middleware`. `signed` is the only status it accepts; anything else 409s. The Contract becomes `voided`, terminal, and still renders in full | `manual` |
| 7.2 | Find every Engagement whose Contract is unsigned | No such list; every Engagement must be opened in turn | `missing-feature (DW-G5)` |
| 7.2-a | Raise an Invoice against the Contract just voided | **It is not refused.** `POST .../contract/invoices` does not read the Contract's status and goes straight to the Connect gate, while **Create Invoice** keeps rendering on a `voided` Contract (DW-G7) | `manual` |

**Nadia crossing, settled.** `POST .../contract/void` had no step here.
[Her plan](loss-client.md) needs a voided Contract for its stage 6, and the only
control that produces one is on this screen — so **7.1-a is added** and the void
path is walked once, on the Staff side that owns it. Stages 8 and 9 are
**unchanged**: what she adds there is the Client's side of the same moment — the
bare word `voided` (**NH-G5**) and the absence of any Invoice surface in the portal
(**NH-G6**) — and both gaps are hers to own. The step ids here are otherwise
untouched.

### Stage 8 — Raise the Invoice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | `POST .../contract/invoices` | **Creates the Invoice.** `201 {"connectRequired":false,"invoice":{…,"status":"open"}}` on a Practice whose Stripe account is connected, for Dee, who is not an Owner — the endpoint never was owner-gated. On a Practice that has *not* connected it answers `200 {"connectRequired":true}` instead; `IsOwner` carries `omitempty` (`api/internal/payments/invoice.go:51`), so for a non-owner the field is absent rather than `false`, and the client defaults it (`+page.svelte:311`) | `manual` |
| 8.2 | Read the message the UI shows | On an unconnected Practice: "Ask a Practice Owner to connect Stripe" — an infrastructure gap wearing a permission error's costume (DW-G2). The same sentence is on the **Payments** screen for a non-owner, so DW-G2 is not confined to this stage | `manual` |

**8.1 was `blocked` for most of 2026-08-22, and cleared on Dee's own walk.** The
reason changed four times in a day, which is worth keeping because each change was
real work rather than a re-reading.

1. The Sandbox did not exist. [#242](https://github.com/markgoho/doula-cloud/issues/242) created it, and Credits cleared.
2. Stripe refused `POST /v1/accounts` for new integrations while every merged Connect path was Accounts v1. [#247](https://github.com/markgoho/doula-cloud/issues/247) moved the leg to Accounts v2.
3. No Practice had been through the hosted onboarding. A first attempt on this walk was made from a Playwright-launched browser and hit a **CAPTCHA** at the email step.
4. It was then driven through a real Chrome, which Stripe does not challenge the same way, and **it completed**. `card_payments` and `payouts` both went `active`, written by the `capability_status_updated` thin event rather than by a poll.

**The CAPTCHA is not the boundary; the browser was.** Stripe challenged an
automation-launched Chromium and did not challenge a human's own Chrome, so the
honest statement is that this step needs **an attended session**, not that Stripe
has closed the door. See [connect-onboarding.md](connect-onboarding.md) for the
walked-through recipe, so no later walk re-derives it.

### Stage 9 — Record the Payment (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Mark an Invoice paid after a bank transfer | No screen, no endpoint. Payments are written only by the Stripe webhook, so the normal case for a small practice cannot be recorded at all | `missing-feature (DW-G3)` |

A live Stripe account would not fix this step. It is not the out-of-scope Stripe
gap; the missing capability is manual Payment recording.

### Stage 10 — Read a filled Care Plan or Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 10.1 | Open an Engagement and read both plan sections | Both render. `GET .../plans/{planType}` has no role check, and per [ADR-0006](../adr/0006-read-follows-the-role.md) an Admin reading both is **correct**, not a leak | `manual` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 0 |
| `manual` | 21 |
| `blocked` | 0 (8.1 cleared on the walk — Connect completed) |
| `missing-feature` | 4 (RA-G2, RA-G4, DW-G3, DW-G5) |

Stages 8 and 9 sat either side of the `blocked` / `missing-feature` line, and the
walk proved the line was drawn in the right place: connecting a live Stripe account
**did** clear 8.1 and did **not** touch 9.1, because recording a bank transfer has
no code path at all. This plan now has no `blocked` step.

Dee's is the only practice-side plan with **no automated step**. Every spec in the
suite runs as an Owner who signed up, and the Admin exists only past the invite
route, which no spec walks.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

This plan has **no** `automated` step, so the suite says nothing about it.
Every step below stage 1 waits on the walk.

### 2026-08-22 — manual walk ([#236](https://github.com/markgoho/doula-cloud/issues/236))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x720 as Dee
Whitlock, with separate contexts for Renata (Owner) and for the Client who has to
sign. Preconditions built as the plan allows: `POST /api/staff/signup` for
`Rooted Birth Collective`, then `POST .../invitations` for Priya Raman and for Dee,
and `PATCH .../staff/{staffId}/roles` giving Priya `doula` so stage 5 has a target.
This plan has no `automated` step, so nothing was skipped as already-run.

**On "run the plan twice".** Walking all 24 steps twice would prove DW-G1 no
better than walking the *comparison* twice, so the second run is a fixed battery
rather than the whole plan: the same 10 endpoints and the same 7 screens, captured
once at `roles = '{}'` and again at `office_manager`, and diffed. The session's
call, recorded here rather than asked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `Accept your Staff invite`, an email box, a password box, and an **Account mode** radio pair — `I'm new here -- create an account` / `I already have an account -- log in`. One button, **Accept invite** |
| 1.2 | `manual` | as expected | `200 POST /api/staff/accept-invite`, and `GET .../staff` then showed `{"name":"Dee Whitlock","roles":[]}`. Zero roles, as claimed |
| 1.3 | `manual` | **falsified** | There is nothing to choose. One membership, so the browser went straight to `/practices/{id}` and `Welcome to Rooted Birth Collective`; no picker rendered. Identical to Renata's 1.2 and unwalkable for anyone until LV-G2 |
| 2.1 | `missing-feature (RA-G2)` | as expected | Confirmed unwalkable. The Staff screen has no editable control; the roster is read-only |
| 2.1-a | `manual` | as expected | `PATCH .../staff/{staffId}/roles` with `["office_manager"]` -> `200`, and the roster then read `office_manager` — the raw enum, not **Admin** (RA-G3). The staff id came from `GET .../staff`, which no screen prints |
| 3.1 | `manual` | **falsified** | **Three** tiles, not two: `Clients Billing Payments`. Payments sits outside `{#if roles.includes('owner')}` — RA-G9, which #235 found from Jo Mercer's zero-role membership and this walk confirms from Dee's |
| 3.1-a | `manual` | as expected — **and this is the strongest result on the plan** | The battery was run at `roles = '{}'` and again at `["office_manager"]` and the two are **byte-identical**: 10 endpoint responses the same status and the same body, 7 screens the same headings, links, buttons, inputs and text. `office_manager` changed nothing anywhere. DW-G1 is not an inference from reading the code; it is a measured null result |
| 3.2 | `manual` | as expected | `Credit balance: 3`, and a Date / Origin / Quantity ledger showing `signup_bonus +3`. A non-owner reads what the Practice spends (DW-G4) |
| 3.2-a | `manual` | as expected — **correct refusal** | `POST .../billing/purchases` -> `403 only a Practice Owner can do that`, though **Buy credits** and its Quantity box render for her first |
| 4.1 | `manual` | as expected | Two fields, `Their name` and `Their email`, and **Add Client**. Nothing about a due date, a referral, or the call (MO-G3) |
| 4.2 | `manual` | as expected, plus one fact | `201`, and it lands **straight on the Engagement page** — not back on the list — reading `Marisol Vega / Status intake / Created 8/22/2026`. Same landing Tasha's walk found |
| 5.1 | `missing-feature (RA-G4)` | as expected | Confirmed unwalkable. Every control on the page: `Send portal invite`, `Add a Visit`, `Create Care Plan`, `Create Birth Plan`, `Create Draft Contract`, `Send`. Nothing names a Doula |
| 5.2 | `manual` | **falsified** | The workaround does not exist. **Add a Visit** renders for Dee and answers `403 only a Staff member with the Doula role can do that`, in the page and on a direct `POST` alike — `visit/roles.go:41` requires `doula`, which she does not hold. New gap **DW-G6** on the journey map, and the map's moment-of-truth argument is re-stated, because it rested on this step working |
| 6.1 | `manual` | as expected | `201` for a non-owner, and the screen prints `Invited. There is no email sending yet, so share this link with them directly:` with a raw `/portal/accept-invite?token=<uuid>` URL |
| 6.2 | `manual` | as expected | `201`, `Status: draft`, and six blank merge-field inputs — `Practice name`, `Client name`, `Scope of service`, two dates and `Price` (**MO-G10**, Maya's) |
| 6.3 | `manual` | as expected | `PUT` then `POST .../contract/send` -> `200`, `Status: sent`. With the fields filled by hand the Client's portal read the prose resolved: `This agreement is between Rooted Birth Collective and Marisol Vega for doula services.` |
| 6.3-a | `manual` | as expected | The page renders the Practice's real template prose and the whole merge-field legend to Dee first; **Save** -> `403 only a Practice Owner can do that`. Read open, write shut — PR-G4's shape, and under ADR-0006 the read is correct |
| 7.1 | `manual` | as expected | After the Client signed (two-step: an electronic-signature disclosure, then full legal name plus a checkbox) the Engagement page read `Status: signed` and offered **Void Contract** |
| 7.1-a | `manual` | as expected | **Dee, a non-owner, voided a signed Contract**: `200`, `Status: voided`, `Voided — this Contract is no longer active.` Terminal as claimed — a second void `409`s and so does `send`. The prose and all six fields still render |
| 7.2 | `missing-feature (DW-G5)` | as expected | Confirmed unwalkable. The Clients screen is two columns, `Name` and `Status`, and the Status is the *Engagement's* (`intake`), not the Contract's. No cross-Engagement view of signature state exists |
| 7.2-a | `manual` | **new** | The void does not close the till. **Create Invoice** and its Amount box keep rendering on the `voided` Contract, and `POST .../contract/invoices` answers `200 {"connectRequired":true}` — the Connect gate, not a refusal. The handler never reads the Contract's status, so with a connected account the next thing it meets is Stripe. New gap **DW-G7** |
| 8.1 | `manual` (was `blocked`) | **re-marked — it clears** | Pre-connect: `200 {"connectRequired":true}`. The Owner then completed Stripe's hosted onboarding (see the addendum below), and the same call as Dee answered `201 {"connectRequired":false,"invoice":{…,"status":"open","amountCents":90000}}`. A non-owner Admin raised a $900 Invoice on a connected account |
| 8.2 | `manual` | as expected | `Ask a Practice Owner to connect Stripe.` — and the same sentence is on the **Payments** screen for a non-owner. DW-G2 is not confined to the Invoice |
| 9.1 | `missing-feature (DW-G3)` | as expected | Confirmed unwalkable. No control on the Engagement page matches pay / paid / mark / record, and `api/main.go` has no route that writes a Payment. The only writer is the `invoice.paid` webhook |
| 10.1 | `manual` | as expected — **and correct** | Dee opened the Engagement and read both filled plans in full: the Care Plan's support people, pain management and backup-doula checkbox, and the Birth Plan's setting, people to notify and atmosphere. Per ADR-0006 an Admin reading both is right, not a leak |

**21 `manual` steps walked (8.1 among them, after the addendum below); 4
`missing-feature` steps confirmed unwalkable; no `blocked` step left on this
plan.** Three expected results
were falsified — 1.3, 3.1 and 5.2 — and one new step, 7.2-a, was added by the
walk. Two gaps minted on the journey map (**DW-G6**, **DW-G7**), plus **DW-G8**
below. No `journey-gap` issue was filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**One finding is not Dee's and is not any stage's: no screen in the product has a
title.** `document.title` is `""` on `/login`, the practice landing, Clients,
Billing and the Engagement page, so every browser tab is blank and SvelteKit's own
live region announces `untitled page` to a screen reader at every client-side
navigation. It surfaced here because it is on every screen this walk opened. Filed
as **DW-G8** on Dee's map for want of a better owner; it belongs to all nine.

**Verdict against "a pass means": it does not pass, and it fails at both ends.**
The middle holds and holds well — Dee creates the Client, sends the portal invite,
builds, sends and voids the Contract, and reads both plans, all as a non-owner,
refused only where the refusal is correct. What fails is the paperwork the journey
is named for. There is **no Doula assigned**, and now no proxy for one either: the
one act the map offered as a workaround is Doula-gated and refuses them (DW-G6).
There is **no recorded Payment**, and no route that could write one (DW-G3).

Their moment of truth landed where the map put it, and the walk made the argument
for it honest rather than merely plausible. Stage 5 lost the comparison for the
wrong reason — the map credited Dee with a product workaround they do not have.
Stage 9 still wins, on the ground that survives: a doula can be told by phone, and
the work happens anyway; money that arrived by bank transfer cannot be written
down anywhere, and the book stays open.

The role, meanwhile, is a null. Two runs of the same battery, one with no roles at
all and one holding `office_manager`, came back identical in every byte. Whatever
Dee is allowed to do, they are allowed to do it because they are Staff.

### 2026-08-22, later — Connect completed, and 8.1 cleared ([#236](https://github.com/markgoho/doula-cloud/issues/236))

The walk above left 8.1 `blocked` because Stripe's hosted onboarding served a
CAPTCHA to the Playwright-launched browser. It was then driven again through the
**user's own Chrome**, which Stripe did not challenge the same way, and it
completed. Recipe kept at [connect-onboarding.md](connect-onboarding.md).

`acct_1U7Rwv1rKod8tdZe`, an unregistered US business, industry *Other personal
services*, Stripe test bank, Radar Pro, Climate and Tax declined. On submission
Stripe redirected to `?connect=return`; `status` went `onboarding_incomplete` ->
`pending` -> **`active`**, with `cardPaymentsStatus` and `payoutsStatus` both
`active` and no requirements outstanding. **The webhook wrote it**: the
`capability_status_updated` thin event landed on `/api/stripe/account-webhook`
and every delivery answered `200`.

Four things fell out of it.

**8.1 clears, and the practice side has no `blocked` step left.** As Dee — not an
Owner — `POST .../contract/invoices` answered `201 {"connectRequired":false,…}`
and created a `$900.00` Invoice at `open`. The endpoint never was owner-gated and
the walk now proves it end to end rather than at a gate.

**DW-G7 is worse than the pre-connect walk could show.** The Contract was voided
and the *same* call was made again: `201`, a second Invoice, `$500.00`, `open`.
Stripe holds a real, finalized, payable invoice with a hosted payment URL against
an agreement the Practice has already voided. Before Connect this only reached the
`connectRequired` gate, so the gap read as a missing status check; it is in fact a
bill a Client can pay for a Contract that no longer exists.

**An Invoice can be raised before the Practice can take payment.** The $900 one
was created while `cardPaymentsStatus` was still `restricted` and the account was
`pending`. The product's gate is *has a connected account*, not *can actually
accept a card payment*, so a Client can be sent a bill during Stripe's review
window. Recorded here rather than minted: whether that is wrong is a decision
about what the gate should mean, and [#209](https://github.com/markgoho/doula-cloud/issues/209)
is where it gets argued.

**The Practice name reaches the Client, and the Payments screen does not reach the
Owner.** Both Stripe invoices carry `account_name: "Rooted Birth Collective"`, so
the `display_name` fix from `7261a59` holds on a second, independent Practice —
the `DOULA.CLOU` regression has not come back. The Payments screen is the opposite
story: seconds after a successful submission it still read `Onboarding incomplete`
/ "Stripe still needs some details before Clients can pay you" and offered
**Continue Stripe onboarding**, a dead end, while the API already said `pending`
with zero requirements. It fetches once in `onMount` and never again
(`settings/payments/+page.svelte:21`), so its own banner — "Status updates once
Stripe confirms your account is active" — describes something the page never does.
A manual reload showed `Awaiting Stripe review` / "Nothing is needed from you."
New gap **MO-G11** on [Maya's map](../journeys/solo-birth-doula.md), which owns
the Connect step.
