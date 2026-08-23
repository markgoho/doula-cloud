# Camille Boyd — test plan

- **Journey**: [returning-postpartum-client.md](../journeys/returning-postpartum-client.md)
- **Persona**: [returning-postpartum-client.md](../personas/returning-postpartum-client.md)
- **A pass means**: two Engagements at one Practice — one closed and still
  readable, one live and postpartum — reachable from one portal account. **All
  three clauses fail**, in three different places for three different reasons.

Her persona file says the schema supports her, and it does: `clients` carries no
`practice_id`. The API and the portal do not. This plan is the proof, and it is
walkable to the end — every refusal she meets is a refusal she can observe.

## Preconditions

- A Practice with an Owner and Priya holding `doula`, set with
  `PATCH /api/practices/{id}/staff/{staffId}/roles`; no screen does it (RA-G2).
- **Her first Engagement, created and left alone.** It cannot be closed
  (**MO-G4**), so create it fresh and treat it as the 2024 one. That two years of
  finished work is indistinguishable from an Engagement made this morning is stage
  1's finding, not a defect in the fixture.
- **At least two Client credits.** Stage 3 spends a second one on a person the
  Practice has already paid for (**MO-G9**).
- **Two email addresses for one person.** Stage 5.3 needs a second portal account.
  This is not a fixture bypass — it is the workaround the product forces on her,
  and walking it is the point of the stage.

## Steps

### Stage 1 — Two years ago, the first Engagement ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Mark her first Engagement finished | No handler writes `UPDATE engagements`. The record of her first birth still reads `intake`, and nothing distinguishes it from one created this morning | `missing-feature (MO-G4)` |

### Stage 2 — She calls Priya

No step: the product is not involved, and **that is the finding**. Every fact she
is relying on — who she is, who her doula was, how her first birth went — lives in
a message thread and in Priya's memory, not in a field. Nothing to walk, nothing to
mark; the consequences land in stages 3 and 8.

### Stage 3 — The Practice types her in again

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/clients` and find Camille's existing record | The row is there. **It is a dead end** — the list is a read surface and nothing on it opens a second Engagement | `manual` |
| 3.1-a | Look her up by email, or add an Engagement to the Client she already is | `POST /api/practices/{id}/clients` **always inserts a new `clients` row** (`engagement/create.go`). No lookup, no client search, no add-an-Engagement-to-this-Client endpoint. Client and Engagement are created in one indivisible request, by design | `missing-feature (CB-G1)` |
| 3.2 | Create a new Client with the same name and email | Succeeds. Two `clients` rows now exist for one person, and a second Client credit is consumed for someone the Practice already paid for (**MO-G9**) | `manual` |
| 3.2-a | Carry anything from her first Engagement across by hand | The form takes name and email only (**MO-G3**). There is nothing to carry it into | `missing-feature (MO-G3)` |

### Stage 4 — Declaring it postpartum-only

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Record that this Engagement is postpartum work, not a birth | `engagements` has no type or kind column, only `status`, and the create handler names `intake` as the constant with no create-time alternative. `CONTEXT.md` calls Engagement "deliberately generic so it fits both birth-doula and postpartum-doula work"; **generic turns out to mean silent** | `missing-feature (CB-G2)` |
| 4.1-a | Approximate it by moving the status to `postpartum` | Unavailable anyway (**MO-G4**) — and it would say she has given birth under this Engagement, which she has not | `missing-feature (MO-G4)` |

### Stage 5 — The second invite refuses her — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Priya sends the portal invite on the new Engagement | `POST .../portal-invite` succeeds for a non-owner; the link goes by hand (**RA-G1**) | `manual` |
| 5.2 | Open the link, choose "I already have an account", sign in as herself | **409, and the page prints the string**: "a portal account already exists for this identity". `client_portal_users.identity_uid` is `UNIQUE` across the table (`00006_client_portal_users.sql`), so `UPDATE client_portal_users SET identity_uid = …` collides with her first row. She is refused for being a returning customer (CB-G3) | `manual` |
| 5.3 | Create a second account under a different email address | Succeeds. It is the only way forward, and it makes the duplication permanent | `manual` |

### Stage 6 — Two accounts, one person

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Sign in as account A | Lands on her 2024 birth Engagement, still `intake` | `automated (client-portal-login.e2e.ts)` |
| 6.2 | Sign out, sign in as account B | Her postpartum Engagement. Each account resolves to exactly one `clients` row, so each shows one Engagement and neither can see the other | `manual` |
| 6.2-a | Move between the two without signing out | No switcher exists. The chooser appears only on the login and accept-invite screens, and the authenticated layout's entire chrome is a sign-out button | `missing-feature (CB-G4)` |
| 6.2-b | With both under one identity, tell them apart in the chooser | The chooser labels each Engagement by `practiceName` alone (`login/+page.svelte`), so two at Rooted Birth Collective would render as two identical links. Closing CB-G1 and CB-G3 without this leaves her choosing blind | `missing-feature (CB-G4)` |

### Stage 7 — Offered a Birth Plan she does not need

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open the portal home on the postpartum Engagement | **Birth Plan** and **Contract** links, both rendered unconditionally | `manual` |
| 7.2 | Open the Birth Plan link | "No Birth Plan has been created for this Engagement yet" — which promises one is coming rather than saying it does not apply | `manual` |
| 7.2-a | Mark that no Birth Plan applies to this Engagement | There is no way to mark an Engagement as anything (CB-G2). Priya may fill one in just to clear the empty state, which puts a labour-preferences document on a postpartum Engagement | `missing-feature (CB-G5)` |

### Stage 8 — Nothing came with her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Read her new message thread | Empty. Messages are one thread per Engagement and Plan Instances are per Engagement by ADR-0001's snapshot rule — correct scoping, and it means her history does not travel | `manual` |
| 8.1-a | See a person's Engagements over time, from her side or Priya's | No such view exists on either side. "They know me" is true of Priya and false of the product | `missing-feature (CB-G6)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 1 |
| `manual` | 9 |
| `missing-feature` | 9 (MO-G4 ×2, CB-G1, MO-G3, CB-G2, CB-G4 ×2, CB-G5, CB-G6) |

No step is `blocked`. Nothing on her path touches Stripe.

CB-G3, **MO-G9**, **RA-G1** and NH-G4 are observed inside walkable steps (5.2, 3.2,
5.1, 6.1) rather than given steps of their own — CB-G3 most of all: her moment of
truth is a step that **can** be performed, and the 409 it returns is the finding.

Her single automated step is her first login, which passes. Every spec in the suite
provisions a Client who has never been seen before, so nothing in it can fail the
way she does.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 6.1 | `client-portal-login.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-08-23 — manual and missing-feature steps ([#241](https://github.com/markgoho/doula-cloud/issues/241))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Rooted Birth
Collective", Owner+Admin+Doula in one Staff row — Priya's shape, since her
journey carries this plan's staff-side stages) with two `clients` rows for
one person (Camille Boyd), her two Engagements, and two portal accounts
under two email addresses. Walked in Chrome via playwriter, one browser
profile, re-authenticating whichever side was about to act — staff and
Client-portal share one `__session` cookie per origin on `localhost:5173`,
as prior walks found; not a new finding.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `missing-feature (MO-G4)` | confirmed | No status-change control anywhere on the Engagement page — only `Status`/`Created` in a description list, no way to mark it finished |
| 3.1 | `manual` | as expected | The Clients list row is a plain link back to the same Engagement; nothing on it opens a second one |
| 3.1-a | `missing-feature (CB-G1)` | confirmed | Add a Client (name + email) is the only entry point; `POST .../clients` inserted a fresh `clients` row both times — no lookup, no search, no add-an-Engagement action anywhere |
| 3.2 | `manual` | as expected | A second `clients` row for Camille was created without complaint; credit balance went `3 -> 1` across the two creations (**MO-G9**) |
| 3.2-a | `missing-feature (MO-G3)` | confirmed | The Add a Client form takes name and email only, both times |
| 4.1 | `missing-feature (CB-G2)` | confirmed | Neither Engagement page shows anything beyond `Status`/`Created` — no kind field, no create-time alternative to `intake` |
| 4.1-a | `missing-feature (MO-G4)` | confirmed | Same absence — no status control exists on either Engagement to approximate with |
| 5.1 | `manual` | as expected | `POST .../portal-invite` returned `201`; the link was printed in plain text for hand delivery (**RA-G1**) |
| 5.2 | `manual` | as expected | `POST /api/portal/accept-invite` returned `409 CONFLICT`; the page showed the exact string "a portal account already exists for this identity" |
| 5.3 | `manual` | as expected | A second account under a different email address succeeded and landed straight on the postpartum Engagement |
| 6.2 | `manual` | as expected | Account A resolves only to the 2024 Engagement (still `intake`); account B resolves only to the postpartum one; no route connects them beyond signing out and back in |
| 6.2-a | `missing-feature (CB-G4)` | confirmed | The authenticated layout's entire chrome is one Sign-out button, on both accounts — no switcher anywhere |
| 6.2-b | `missing-feature (CB-G4)` | confirmed by inspection, not by walking | CB-G1 and CB-G3 keep any one identity from ever holding two Engagements in this stack, so the two-Engagement chooser this step asks about cannot be produced live; the claim rests on `login/+page.svelte` labeling an Engagement by `practiceName` alone, as the map already argued |
| 7.1 | `manual` | as expected | **Birth Plan** and **Contract** links both render unconditionally on the postpartum Engagement |
| 7.2 | `manual` | as expected | "No Birth Plan has been created for this Engagement yet." |
| 7.2-a | `missing-feature (CB-G5)` | confirmed | Same absence as 4.1 — no way to mark an Engagement as not needing a Birth Plan |
| 8.1 | `manual` | as expected | The postpartum Engagement's message thread is empty, isolated from the 2024 thread |
| 8.1-a | `missing-feature (CB-G6)` | confirmed | No view, on either side, shows a person's Engagements over time |

**18 steps walked: 9 `manual`, 9 `missing-feature`. Every mark holds.** No
`blocked` step exists on this plan, confirmed — nothing on her path touches
Stripe. No `journey-gap` issue filed from this ticket — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209), still blocked on
this ticket alone now that it is the last of the nine.

**Verdict**: this plan cannot pass, as written, and does not, on all three
clauses. The 2024 Engagement stays `intake` forever; the postpartum one
cannot declare what it is; and reaching both from one portal account is
refused at the exact step the map names as her moment of truth.
