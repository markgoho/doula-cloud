# Lena Vasquez — test plan

- **Journey**: [contractor-doula.md](../journeys/contractor-doula.md)
- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **A pass means**: the Engagement she took is finished, she can point to what she
  agreed and what she was paid, and she never saw a Client who was not hers.

This plan is expected to fail at step 1.2 and never reach stage 3 unmodified. It is
written to be run anyway: the failures are the evidence, and the fixture bypass
below is what lets the later stages be walked at all.

## Preconditions

- **Two** Practices: the other agency where Lena already works, and Rooted Birth
  Collective. She holds a Staff account at the first one already — that is the
  point of her.
- Rooted Birth Collective with an Owner and **at least two** Clients, only one of
  which is meant to be hers.
- **Fixture bypass for stages 2–8.** Stage 1 cannot produce her second membership
  (LV-G2), so insert a `practice_memberships` row for her **existing** `staff` row
  against the second Practice, directly in Postgres. Whether the schema accepts
  that is itself unproven — `00002` promises multi-Practice membership in its own
  comment — so the run's first job is to find out, and to record the answer.
- No fixture can create an **Offer** or an employment type. Stage 3 has nothing to
  provision.

## Steps

### Stage 1 — Join the agency, on an account she already has

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link at `/accept-invite` | The accept form renders | `manual` |
| 1.2 | Sign in as herself and submit | **Expected failure**: `InviteHandler` inserted a fresh `staff` row and acceptance writes her identity onto it, but `staff.identity_uid` is `UNIQUE` and hers is on the other agency's row. A unique-constraint violation, caught at `accept.go:104` and surfacing as a **`409`**, "a staff account already exists for this identity" | `manual` |
| 1.2-a | Check for a second membership | None exists. A person cannot be Staff at two Practices through the only route that reaches her | `missing-feature (LV-G2)` |
| 1.3 | Record that she is a contractor, not an employee | `practice_memberships` is `(practice_id, staff_id, roles, created_at)` — no column holds it | `missing-feature (LV-G1)` |

### Stage 2 — Sign in and choose the agency

Reachable only after the fixture bypass.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Sign in at `/login` | `POST /api/session` succeeds | `automated (staff-login.e2e.ts)` |
| 2.2 | Choose Rooted Birth Collective from **two** memberships | The picker lists both. She is the only Persona for whom this is a real decision, and she meets it every time | `manual` |
| 2.2-a | Read what she is at each Practice from the picker | Nothing on it distinguishes the agency she contracts to from the one she works at | `missing-feature (LV-G1)` |
| 2.3 | Read the tiles | Owner tiles hidden; **Clients and Billing remain**. Billing is the agency's own credit spending — not her business at all, and she is not even the employee it was already wrong for (DW-G4) | `manual` |

### Stage 3 — The offer (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Receive the offer of the February Engagement | Nothing offers an Engagement to anybody. There is no Offer, and no attachment for one to lead to | `missing-feature (LV-G6)` |
| 3.2 | Read enough to take or refuse it — Client, dates, on-call terms, fee — while still an outsider | No read rule covers *offered, not yet accepted*; ADR-0006's table has four columns and none is hers | `missing-feature (LV-G7)` |
| 3.3 | Decline, and have Renata see the refusal | A decline is not recorded anywhere, so silence and "no" are the same thing to the Practice | `missing-feature (LV-G6)` |

Every step of the stage her whole relationship with the product is built on is a
hole. Nothing here degrades to a manual walk-through: there is no screen to open.

### Stage 4 — Find the one job she took

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Every Client at Rooted Birth Collective.** For Priya this is a scope failure inside one team; here it is one business reading another's book (LV-G5, same root as PR-G1) | `manual` |
| 4.2-a | Pick hers out | Only by remembering the name from the offer — no column marks it (RA-G4) | `manual` |

### Stage 5 — Check the terms and the fee

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open her Engagement | The single-page view renders | `manual` |
| 5.2 | Read the Contract section | Prose, merge fields and values in one object with no role check — **she gets the money**, on every Engagement in the Practice, not only hers. ADR-0006 says she should read it on her own work and Priya should not, which needs a split the read cannot make (PR-G2) | `manual` |
| 5.3 | Find what *she* is owed | Nothing holds it. The Contract prices the Client's care, and her rate lives in the phone call | `missing-feature (LV-G3)` |

### Stage 6 — Do the work

Identical to Priya's stages 6–8 and marked there; walk
[employed-doula.md](employed-doula.md) steps 6.1 to 8.2 as Lena and record only
where a contractor differs. Nothing is expected to — which is the finding: the care
half is the half the product already treats correctly.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Read the Birth Plan | As Priya 6.1–6.2, including no deep link and no handoff from her side (PR-G5) | `manual` |
| 6.2 | Log a Visit | As Priya 7.1: no date, no type, no note (MO-G1, MO-G2, PR-G6) | `manual` |
| 6.3 | Message the Client | As Priya 8.1–8.2 | `manual` |

### Stage 7 — Get paid

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Point to what she was paid for the February birth | No product step exists. `invoices` rows are `(practice_id, contract_id, …)` — the Practice billing the Client. Nothing records a Practice owing a doula, so the two sides keep separate books | `missing-feature (LV-G3)` |

### Stage 8 — The job ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | End the attachment when the job is finished | Nothing expresses it. Her read never lapses; the only lever is removing her membership, which erases her from the Visits she worked | `missing-feature (LV-G4)` |
| 8.1-a | Re-run step 4.1 afterwards | She still reads the agency's Clients | `manual` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 1 |
| `manual` | 13 |
| `missing-feature` | 9 steps over 6 gaps (LV-G1, LV-G2, LV-G3, LV-G4, LV-G6, LV-G7) |

LV-G5 is observed at 4.2 rather than given a step of its own: the list opens, and
what it shows is the finding.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 2.1 | `staff-login.e2e.ts` | pass |

**1 automated steps: all pass.**

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**.
That is [#238](https://github.com/markgoho/doula-cloud/issues/238).
