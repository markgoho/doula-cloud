# Maya Okonkwo — from empty account to one live Engagement

- **Persona**: [solo-birth-doula.md](../personas/solo-birth-doula.md)
- **Goal**: replace the paper folder — one place that holds the Client record, the
  Contract, the Birth Plan, and the messages
- **Entry point**: cold signup at `/signup`
- **Done looks like**: a Client with a signed Contract, a completed Birth Plan the
  Client can read in the portal, at least one Visit, an open message thread, and an
  Invoice she can point at — done without asking anyone for help.

## Moment of truth

**Stage 5 — the Contract comes back signed without her leaving the app.** If she
still has to print, email, and chase a signature, the folder has not been
replaced and nothing else in the product matters. Her other make-or-break moment,
the 3 a.m. retrieval of a Birth Plan, sits after this journey ends; it is the test
of whether she *stays*, not of whether she starts.

## Words

Maya is close to a domain expert but not inside the model.

| Domain term | What Maya says | Note |
| --- | --- | --- |
| Engagement | "her file", "my client" | The word Engagement is invisible in her workflow and should stay that way |
| Care Plan | "my notes" | She would not call her own notes a plan |
| Plan Instance | — | She has no word for it, and needs none. It is a modelling term |
| Plan Template | "the form I use every time" | |
| Visit | "a visit", "the birth" | Matches |
| Client | "my client", "the mom" | `CONTEXT.md` avoids "mom" deliberately |

## Stages

### Stage 1 — Sign up and create the Practice

**Thinking**: "I lost an intake form last spring. I am not doing that again."
**Pain points**: none — she has the strongest motivation of any Persona.

- **1.1** — Open `/signup`.
- **1.2** — Fill Practice name, Your name, Email, Password; press **Create
  Practice** (`POST /api/staff/signup`).
- **1.3** — Land on `/practices/[practiceId]`. She now holds Owner, Admin, and
  Doula on one membership, granted in a single statement (`signup.go:152`).

She is **not** a test of role separation. Every permission boundary rides on
Renata's invite route instead.

### Stage 2 — Judge the seeded Plan Templates

**Thinking**: "Is this form going to fit how I actually work?"
**Pain points**: she is asked to evaluate an abstract form before she has a single
Client to picture. This is the least motivating work in the journey, placed at the
point where she is least invested.

- **2.1** — Open `/practices/[practiceId]/settings/plan-templates`.
- **2.2** — Review the seeded default Care Plan and Birth Plan fields.
- **2.3** — Edit and save (`PUT /api/practices/{id}/plan-templates/{planType}`),
  owner-gated at `plans/template.go:220`. She is Owner, so this passes.
- **2.4** — Review the seeded Contract Template at
  `/practices/[practiceId]/settings/contract-template`. Signup seeds all three
  templates in one transaction — Care Plan, Birth Plan, and Contract
  (`signup.go:160`, `signup.go:168`) — so no Practice ever has to create one
  before sending its first Contract.

### Stage 3 — Add the first Client

**Thinking**: "Everything in the folder goes in here."
**Pain points**: the folder holds a due date, a phone number, an address, a
hospital, a partner's name, and a page of intake notes. The form takes two fields.

- **3.1** — Open `/practices/[practiceId]/clients/new`.
- **3.2** — Enter name and email; press **Add Client**
  (`POST /api/practices/{id}/clients`). A Client and an Engagement at status
  `intake` are created together.
- **3.3** — Open the Engagement from the Clients list.

### Stage 4 — Fill the Care Plan and the Birth Plan

**Thinking**: "This is the part that actually helps me at 3 a.m."
**Pain points**: long-form typing, on a phone, by someone who is on call. Anything
needing a laptop and twenty quiet minutes will not get done.

- **4.1** — On the Engagement page, fill the Care Plan section
  (`PUT /api/practices/{id}/engagements/{id}/plans/care`).
- **4.2** — Fill the Birth Plan section (`.../plans/birth`). Each Plan Instance
  snapshots the template's field definitions at creation, so a later template edit
  cannot alter it.

### Stage 5 — Contract and signature — moment of truth

**Thinking**: "If she can sign this on her phone tonight, I am sold."
**Pain points**: the ordering is not obvious. The Client cannot sign anything
until a portal invite has been sent, and nothing on the Contract section says so.

- **5.1** — Send the portal invite
  (`POST /api/practices/{id}/engagements/{id}/portal-invite`) so the Client has an
  account to sign in with.
- **5.2** — Build the Contract from the Practice's contract template
  (`POST /api/practices/{id}/engagements/{id}/contract`), status `draft`.
- **5.3** — Send it (`.../contract/send`), status `sent`.
- **5.4** — The Client signs in the portal
  (`POST /api/portal/engagements/{id}/contract/sign`), status `signed`.
- **5.5** — Maya sees `signed` on the Engagement page without leaving the app.

### Stage 6 — Schedule Visits

**Thinking**: "Prenatals at 32 and 36 weeks, then the birth."
**Pain points**: this stage cannot be walked. A Visit is
`(engagement_id, staff_id, created_at)` and nothing else — no date, no time, no
type, no notes. `POST .../visits` records that a Visit *happened*, now, by
somebody. It cannot schedule one, and the only edit is reassignment to another
Staff member.

- **6.1** — Open the Visits section on the Engagement page.
- **6.2** — Add a Visit (`POST /api/practices/{id}/engagements/{id}/visits`) and
  get a row with a creation timestamp.

This is the largest single gap on the practice side. It also removes any
possibility of a calendar, which is what Renata's coverage view would need.

### Stage 7 — Get paid

**Thinking**: "I am tired of chasing money by text."
**Pain points**: two money screens with names that invite the wrong click.
**Billing** is where the Practice *buys credits from Doula Cloud*. **Settings →
Payments** is where the Practice *gets paid by Clients*. Nothing on either screen
explains the difference.

- **7.1** — Open `/practices/[practiceId]/settings/payments` and start Stripe
  Connect onboarding (`POST /api/practices/{id}/payments/connect`, owner-gated).
- **7.2** — Raise an Invoice against the signed Contract
  (`POST /api/practices/{id}/engagements/{id}/contract/invoices`).

No Stripe account exists yet, so this leg is **blocked rather than broken**. The
invoice endpoint returns `connectRequired` before it creates anything.

### Stage 8 — Message the Client

**Thinking**: "Now everything about her is in one thread."
**Pain points**: none identified. Messages are Engagement-scoped, immutable, and
delivered by push-triggered fetch (ADR-0002), which she must not treat as a
substitute for a phone call in labour.

- **8.1** — Send a message from the Engagement page
  (`POST /api/practices/{id}/engagements/{id}/messages`).
- **8.2** — The Client replies from the portal; the thread is continuous.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| MO-G1 | 6 | Interaction | A Visit has no date or time. Visits can be recorded, never scheduled. No calendar is possible. |
| MO-G2 | 6 | Both | A Visit has no notes. "What was I told last time" has nowhere to live. |
| MO-G3 | 3 | Both | A Client record holds name and email only — no due date, phone, address, or intake notes. The paper folder cannot be replaced. |
| MO-G4 | 3 | Interaction | An Engagement's status never changes. There is no update path anywhere, so every Engagement stays `intake` for its whole life. |
| MO-G5 | 2 | Experience | She must judge a seeded Plan Template before she has any Client to picture, at her lowest motivation. |
| MO-G6 | 5 | Experience | The Client cannot sign until a portal invite is sent, and nothing in the Contract section says so. The ordering is implicit. |
| MO-G7 | 7 | Experience | "Billing" (credits she buys) and "Payments" (Stripe Connect, so she gets paid) are two unexplained money screens whose names invite the wrong click. |
| MO-G8 | 7 | Interaction | Stripe Connect is not configured, so getting paid is blocked. Expected — record it, do not chase it. |
