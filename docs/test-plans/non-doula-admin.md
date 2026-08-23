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
| 8.1 | `POST .../contract/invoices` | `connectRequired`, and the endpoint is **not** owner-gated — Stripe is the blocker, not the role. `IsOwner` carries `omitempty` (`api/internal/payments/invoice.go:51`), so for a non-owner the wire body is `{"connectRequired":true}` with the field absent, which the client defaults (`+page.svelte:311`) | `blocked` |
| 8.2 | Read the message the UI shows | "Ask a Practice Owner to connect Stripe" — an infrastructure gap wearing a permission error's costume (DW-G2) | `manual` |

**8.1 re-checked three times on 2026-08-22**, and it is still `blocked`. The
reason has changed each time, and the third one is the durable one.

First pass: the Sandbox existed ([#242](https://github.com/markgoho/doula-cloud/issues/242))
and Credits cleared, but Stripe refused `POST /v1/accounts` for new integrations
while every merged Connect path was Accounts v1.

Second pass: [#247](https://github.com/markgoho/doula-cloud/issues/247) moved the
Connect leg to Accounts v2 and created both Sandbox event destinations, so that
refusal is gone. `card_payments` replaced `charges_enabled` as the thing a Practice
must reach. What was left was that no Practice had completed the hosted onboarding.

Third pass, this walk ([#236](https://github.com/markgoho/doula-cloud/issues/236)):
that onboarding was attempted for Dee's Practice and **it cannot be completed by
the walk**. `POST .../payments/connect` succeeded and created the connected account
(`acct_1U7RdD1rKocBawcv`); the Payments screen went `Not connected` ->
`Onboarding incomplete` / `Stripe still needs some details before Clients can pay
you.` / `Stripe needs 15 more details from you.` and offered **Continue Stripe
onboarding**. That link opens Stripe's hosted form, which serves a **CAPTCHA** at
the first step. Our own code got as far as it can; what remains is an anti-
automation control on a third party's system, which a walk must not work around.

So Dee's 8.1 is `blocked` in the strongest sense the
[README](README.md) defines: the code path is complete, no product decision is
missing, and it needs a person at a browser. It clears when someone completes
Renata's onboarding by hand — the account and the link both already exist.

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
| `manual` | 20 |
| `blocked` | 1 (8.1, Connect — [#247](https://github.com/markgoho/doula-cloud/issues/247)) |
| `missing-feature` | 4 (RA-G2, RA-G4, DW-G3, DW-G5) |

Stages 8 and 9 sit either side of the `blocked` / `missing-feature` line and are
the pair that fixes it: connecting a live Stripe account would clear 8.1 and would
not touch 9.1, because recording a bank transfer has no code path at all.

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
| 8.1 | `blocked` | attempted, still `blocked` — **for a better reason** | Pre-connect: `200 {"connectRequired":true}`. Then the Owner's Connect leg was driven to see whether the mark could be cleared: `POST .../payments/connect` -> `200`, account `acct_1U7RdD1rKocBawcv` created, Payments moved to `Onboarding incomplete` / `Stripe needs 15 more details from you.` with a **Continue Stripe onboarding** link. That link's hosted form serves a **CAPTCHA**. Our code reaches the end of its own leg; what is left is an anti-automation control on Stripe's side, so the walk stopped there rather than working around it |
| 8.2 | `manual` | as expected | `Ask a Practice Owner to connect Stripe.` — and the same sentence is on the **Payments** screen for a non-owner. DW-G2 is not confined to the Invoice |
| 9.1 | `missing-feature (DW-G3)` | as expected | Confirmed unwalkable. No control on the Engagement page matches pay / paid / mark / record, and `api/main.go` has no route that writes a Payment. The only writer is the `invoice.paid` webhook |
| 10.1 | `manual` | as expected — **and correct** | Dee opened the Engagement and read both filled plans in full: the Care Plan's support people, pain management and backup-doula checkbox, and the Birth Plan's setting, people to notify and atmosphere. Per ADR-0006 an Admin reading both is right, not a leak |

**20 `manual` steps walked; 4 `missing-feature` steps confirmed unwalkable; the 1
`blocked` step attempted and its real response recorded.** Three expected results
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
