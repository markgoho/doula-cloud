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
| 1.3 | Choose the Practice from the membership list | Lands on `/practices/[practiceId]` | `manual` |

### Stage 2 — Receive the Admin role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | As the Owner, set `office_manager` on Dee's membership from a screen | No screen does this | `missing-feature (RA-G2)` |
| 2.1-a | Set it with `PATCH .../staff/{staffId}/roles` instead | Succeeds for an Owner; the Staff screen then prints `office_manager`, not "Admin" (RA-G3) | `manual` |

### Stage 3 — Discover what the Admin role grants

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Land on `/practices/[practiceId]` | Two tiles only: Clients and Billing. The five owner tiles are hidden | `manual` |
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
| 5.2 | Create a Visit naming the Doula instead | Succeeds — a dateless record that does not express coverage | `manual` |

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
| 8.1 | `POST .../contract/invoices` | `connectRequired` with `isOwner: false`. The endpoint is **not** owner-gated — Stripe is the blocker, not the role | `blocked` |
| 8.2 | Read the message the UI shows | "Ask a Practice Owner to connect Stripe" — an infrastructure gap wearing a permission error's costume (DW-G2) | `manual` |

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
| `manual` | 19 |
| `blocked` | 1 (8.1, Stripe) |
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

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**.
That is [#236](https://github.com/markgoho/doula-cloud/issues/236).
