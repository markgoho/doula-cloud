# Priya Raman — from invitation to caring for an assigned Client

- **Persona**: [employed-doula.md](../personas/employed-doula.md)
- **Goal**: know where she has to be, what this Client wants, and what she was
  told last time — without texting Renata
- **Entry point**: an emailed invitation from Renata, accepted at `/accept-invite`
- **Done looks like**: an active membership holding the Doula role, an Engagement
  assigned to her opened, a Visit logged, and messages exchanged with the
  Client — **and she never saw a screen she had no right to see.**

She is the negative-permission Persona. Half of this map is about what should be
absent.

## Moment of truth

**Stage 6 — the Birth Plan, on a phone, in a hospital corridor.** Everything else
she does could wait; this cannot. If she cannot get to this Client's preferences
in seconds on the device in her hand, the product has failed at the only moment
that matters to her.

## Words

| Domain term | What Priya says | Note |
| --- | --- | --- |
| Engagement | "my client", "the March one" | |
| Client | "the mom", "she" | `CONTEXT.md` avoids "mom" deliberately. The Persona says it anyway — a real divergence between the model's language and a Doula's speech |
| Visit | "a prenatal", "the birth", "the postpartum" | She names three kinds; the model has one and it carries no type |
| Care Plan | "Renata's notes" | She reads them far more than she writes them |
| Birth Plan | "her birth plan" | Matches |

## Stages

### Stage 1 — Accept the invitation on the phone

**Thinking**: "First impression of my new job's software."
**Pain points**: no email is sent (RA-G1). Renata texts a raw URL, which is a poor
first impression and an unverifiable one — Priya cannot tell it is genuine.

- **1.1** — Open the invite link at `/accept-invite`, on whatever device the
  message arrived on.
- **1.2** — Set email and password; press **Accept invite**
  (`POST /api/staff/accept-invite`). The membership is created with zero roles
  (`invite.go` inserts `'{}'`).

### Stage 2 — Receive the Doula role

**Thinking**: none — invisible to her.
**Pain points**: no role UI exists (RA-G2), and even once `doula` is set, nothing
in the codebase reads it. The role is decorative.

- **2.1** — Renata sets the role. No screen does this.

### Stage 3 — Sign in and choose the Practice

- **3.1** — `/login` (`POST /api/session`).
- **3.2** — Choose Rooted Birth Collective.
- **3.3** — Land on `/practices/[practiceId]`. The owner-only tiles (Invite,
  Staff, Plan Templates, Contract Template, Payments) are hidden by
  `{#if roles.includes('owner')}`. **Clients and Billing remain**, and Billing is
  the Practice's credit spending (DW-G6).

### Stage 4 — Find her Clients

**Thinking**: "Which ones are mine?"
**Pain points**: **she sees all of them.** `GET /api/practices/{id}/clients` is
Practice-scoped by design — the handler comment says so: "every Client with an
Engagement at the current Practice, regardless of which Staff member created it —
v1 has no restricted-visibility model." This is not a suspicion; it is the stated
behaviour. Her persona's scope requirement fails, and there is no column telling
her which rows are hers, because Engagements carry no Doula (RA-G4).

- **4.1** — Open `/practices/[practiceId]/clients`.
- **4.2** — Read a list of every Client in the Practice, including other doulas'.

### Stage 5 — Open the Engagement

**Thinking**: "Right, her."
**Pain points**: one long page holding Visits, Care Plan, Birth Plan, Contract,
Invoices, and Messages. The Contract's amount and the Invoice history sit on the
same page as the care record, for any Client — including ones that are not hers.
None of the read paths are role-checked; owner checks live on write endpoints.

- **5.1** — Open `/practices/[practiceId]/engagements/[engagementId]`.
- **5.2** — See the Contract section (`GET .../contract`) and the Invoices section
  (`GET .../contract/invoices`), neither of which she should need.

### Stage 6 — Read the Birth Plan — moment of truth

**Thinking**: "She did not want an epidural unless she asks twice."
**Pain points**: the Birth Plan is a section partway down a long page, on a phone,
in a corridor, under time pressure. There is no deep link to it, no collapse of
the sections she does not need, and no way to hand it to hospital staff from her
side — the print stylesheet is on the Client's portal view.

- **6.1** — Scroll to the Birth Plan section
  (`GET .../plans/birth`).
- **6.2** — Read it.

### Stage 7 — Log a Visit

**Thinking**: "Record what we covered so it is there next time."
**Pain points**: this is the reason she came, and it does not work. A Visit is
`(engagement_id, staff_id, created_at)`. She can record that a Visit happened, by
her, now. She cannot record when it was, what kind it was, or what was said. Her
stated need — "what was I told last time" — is unanswerable (MO-G1, MO-G2).

- **7.1** — Add a Visit in the Visits section
  (`POST /api/practices/{id}/engagements/{id}/visits`).
- **7.2** — See a row with a Staff name and a creation timestamp.

### Stage 8 — Message the Client

**Thinking**: "Confirm Thursday."
**Pain points**: none identified. Push-triggered fetch wakes the Client's device
(ADR-0002); Priya must not treat it as a substitute for a phone call in labour.

- **8.1** — Send a message (`POST .../messages`).
- **8.2** — Receive the reply in the same continuous thread.

### Stage 9 — Confirm the walls hold

**Thinking**: nothing — she would never try. This stage exists for the test plan,
not for her.

- **9.1** — Navigate directly to `/practices/[practiceId]/staff`. The API requires
  Owner (`staffauth/staff.go:25`), so the page should show an error. **Expected to
  hold.**
- **9.2** — `/practices/[practiceId]/invite` — `POST .../invitations` requires
  Owner. **Expected to hold**, but only on submit; the form itself renders.
- **9.3** — `/practices/[practiceId]/settings/plan-templates` — `PUT` requires
  Owner (`plans/template.go:220`). **Expected to hold on write**; the `GET` is
  not owner-gated, so she can read the Practice's template definitions.
- **9.4** — `/practices/[practiceId]/settings/contract-template` — same shape
  (`contracts/template.go:75`).
- **9.5** — `/practices/[practiceId]/settings/payments` — `POST .../connect`
  requires Owner. **Expected to hold.**
- **9.6** — `/practices/[practiceId]/billing` — **expected to fail**: the balance
  and ledger are readable by any Staff member.

The pattern to test for: owner-only surfaces are hidden as links but reachable as
URLs, and their protection is on the write endpoint rather than the read. Every
one of them shows her a screen she has no right to see, then refuses the button.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| PR-G1 | 4 | Interaction | She sees every Client in the Practice. The list handler is Practice-scoped by design ("v1 has no restricted-visibility model"), so her scope requirement fails outright. |
| PR-G2 | 5 | Both | She can read any Engagement's Contract amount and Invoice history, for any Client. Read paths carry no role checks. |
| PR-G3 | 2 | Interaction | The `doula` role is never read anywhere in the codebase. Holding it changes nothing. |
| PR-G4 | 9 | Interaction | Owner-only screens are hidden as links but reachable by URL, and gated on write rather than read — so she reaches the page, sees the data, and is refused only at the button. |
| PR-G5 | 6 | Experience | The Birth Plan is a section partway down a long single-page Engagement view, with no deep link and no phone-first path, at the exact moment she is standing in a corridor. |
| PR-G6 | 7 | Both | A Visit records no date, type, or notes, so "what was I told last time" — her stated reason for using the product — cannot be answered. |
| PR-G7 | 1 | Experience | Her first impression is a raw URL pasted into a text message, which she cannot verify is genuine. |
