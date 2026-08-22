# Renata Alvarez — grow and run the roster

- **Persona**: [practice-owner.md](../personas/practice-owner.md)
- **Goal**: see the whole Practice at once — who is assigned to whom, which
  Contracts are unsigned, which Invoices are unpaid — without asking four people
- **Entry point**: already has a Practice; signs in at `/login`
- **Done looks like**: a new Doula has accepted an invitation, holds the Doula
  role, and appears as the assigned Doula on a live Engagement. Renata can see, in
  one place, every Engagement in the Practice and its Contract and Invoice state.

> **This persona is contradicted by the schema.** Her file states that she assigns
> a Doula to an Engagement and that the Doula "appears as the assigned Doula on a
> live Engagement". The `engagements` table has no staff column
> (`00005_client_engagement.sql`); only `visits.staff_id` exists. The need is real
> and stays in this map as Stage 4. The persona sentence asserting the capability
> exists should be revised. See RA-G4.

## Moment of truth

**Stage 8 — two Clients go into labour the same night, and she needs to know
within a minute who is free.** Coverage is her stated anxiety, and it is the one
thing that cannot wait until morning. Today no screen in the product answers it,
and the underlying data to answer it does not exist either.

## Words

Renata is a domain expert. Her language and `CONTEXT.md` mostly agree, which is
itself worth recording: the divergences below are the exceptions, not the rule.

| Domain term | What Renata says | Note |
| --- | --- | --- |
| Staff | "my doulas", "my team" | She says Staff only about Dee |
| Engagement | "a client", "a birth" | She counts her year in births, not Engagements |
| Admin | "the office" | |
| Visit | "a prenatal", "the birth" | She distinguishes types the model does not |

## Stages

### Stage 1 — Sign in and choose the Practice

**Thinking**: routine. **Pain points**: none.

- **1.1** — `/login`, sign in (`POST /api/session`).
- **1.2** — Choose Rooted Birth Collective from her memberships.
- **1.3** — Land on `/practices/[practiceId]`. Because she holds `owner`, the
  page shows the Invite, Staff, Plan Templates, Contract Template, and Payments
  tiles, all gated by `{#if roles.includes('owner')}`.

### Stage 2 — Invite a new Doula

**Thinking**: "She starts in two weeks. Get her set up."
**Pain points**: no email is sent. The screen says so and prints a link that
Renata must deliver herself, by text or by her own email client. Her new hire's
first impression of the Practice's software is a pasted URL.

- **2.1** — Open `/practices/[practiceId]/invite`.
- **2.2** — Enter the new Doula's name and email; press **Send invite**
  (`POST /api/practices/{id}/invitations`, owner-gated).
- **2.3** — Copy the printed link and send it out of band.
- **2.4** — The invitee accepts at `/accept-invite`. The membership is created
  holding **zero** roles (`invite.go` inserts `'{}'`).

### Stage 3 — Set the new Doula's roles

**Thinking**: "Now make her a doula, not an owner."
**Pain points**: this stage cannot be walked in the product. The API exists —
`PATCH /api/practices/{id}/staff/{staffId}/roles`, owner-gated — but the Staff
screen has no role control. Its only row action is **End sessions everywhere**.

- **3.1** — Open `/practices/[practiceId]/staff` (owner-gated at
  `staffauth/staff.go:25`).
- **3.2** — Read the Roles column, which renders raw enum strings
  (`member.roles.join(', ')`) — so Dee appears as `office_manager`, a word
  `CONTEXT.md` has ruled out.
- **3.3** — Find no way to change them.

Until this is built, the whole roster is unbuildable through the UI, and every
Persona reachable only through the invite route (Priya, Dee) starts with no roles
at all.

### Stage 4 — Assign the Doula to Engagements

**Thinking**: "Priya takes the two October clients."
**Pain points**: an Engagement has no Doula. There is no field, no endpoint, and
no screen. The assignment she thinks in terms of does not exist in the model.

- **4.1** — Open an Engagement.
- **4.2** — Look for an assignment control. There is none.
- **4.3** — The nearest available act is to add a Visit and pick the Staff member
  on it (`POST .../visits`, which takes a `staffId`). Assignment exists at Visit
  level only, and a Visit has no date, so this cannot express "Priya covers this
  birth".

### Stage 5 — Reassign when someone is sick

**Thinking**: "Priya is ill. Move her Thursday to Jo."
**Pain points**: reassignment works, but only over the Visit-level assignment from
Stage 4, so it moves a dateless record rather than a booking.

- **5.1** — `PATCH /api/practices/{id}/engagements/{id}/visits/{visitId}` with a
  new `staffId`.

### Stage 6 — See the whole Practice

**Thinking**: "Show me everyone."
**Pain points**: the list is Practice-wide, which is what she needs — but it has
two columns, Name and Status, and Status is `intake` for every row forever
(MO-G4). Nothing about Contracts, Invoices, or who is covering whom.

- **6.1** — Open `/practices/[practiceId]/clients`
  (`GET /api/practices/{id}/clients`). The handler is explicitly
  Practice-scoped: "every Client with an Engagement at the current Practice,
  regardless of which Staff member created it". **This half of her requirement
  passes.**
- **6.2** — Open each Engagement one at a time to learn its Contract and Invoice
  state. There is no roll-up.

### Stage 7 — See the money across all Staff

**Thinking**: "Which invoices are unpaid?"
**Pain points**: Invoices exist only inside one Engagement's Contract. There is no
Practice-wide invoice list and no unpaid view. The screen named **Billing** is
about credits she buys from Doula Cloud, not money her Clients owe her.

- **7.1** — Open `/practices/[practiceId]/billing` and find a credit balance and
  ledger.
- **7.2** — Look for unpaid Client invoices. Find none, at any level above a
  single Engagement.

### Stage 8 — Coverage, at 2 a.m. — moment of truth

**Thinking**: "Who is already at a birth?"
**Pain points**: no screen shows availability, on-call state, or who is where.
Because Visits carry no dates (MO-G1), the data a coverage view would read does
not exist. This is the highest-value gap in the whole practice-side set, and it
came from the experience layer — a click-by-click map would never have surfaced
it, because there is no click to record.

- **8.1** — Sign in on a phone.
- **8.2** — Look for who is free. There is nowhere to look.

### Stage 9 — Edit Plan Templates for the whole Practice

**Thinking**: "Add the hospital-transfer question to every birth plan."
**Pain points**: none identified.

- **9.1** — `/practices/[practiceId]/settings/plan-templates`,
  `PUT /api/practices/{id}/plan-templates/{planType}` (owner-gated).
- **9.2** — Confirm an already-filled Plan Instance is unchanged. It snapshots the
  field definitions at creation (`00012_plan_instances.sql`), so this **passes**.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| RA-G1 | 2 | Both | Invitations send no email. The Owner must copy a link and deliver it out of band. |
| RA-G2 | 3 | Interaction | No role-assignment UI. The `PATCH .../roles` endpoint exists but nothing calls it, so the roster cannot be built in the product. |
| RA-G3 | 3 | Both | The Staff list renders raw enum values, so an Admin shows on screen as `office_manager` — the word `CONTEXT.md` ruled out. |
| RA-G4 | 4 | Interaction | An Engagement has no assigned Doula. No column, no endpoint, no screen. Assignment exists only per Visit. |
| RA-G5 | 8 | Experience | No coverage or availability view, and no dated Visits to build one from. Her stated anxiety has no surface at all. |
| RA-G6 | 6 | Both | The Clients list shows Name and Status only. No Contract state, Invoice state, or Doula — so "see the whole Practice" needs one Engagement page per Client. |
| RA-G7 | 7 | Interaction | No Practice-wide Invoice or unpaid list. Invoices are reachable only inside a single Engagement's Contract. |

Also hit here, filed on their owning maps: **MO-G4** (an Engagement's status never
changes, so her one status column is dead), **MO-G1** (dateless Visits, which is
why RA-G5 has no data to read).
