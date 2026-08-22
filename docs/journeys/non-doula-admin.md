# Dee Whitlock — first call to signed and billed

- **Persona**: [non-doula-admin.md](../personas/non-doula-admin.md)
- **Goal**: do the paperwork end of an Engagement without waiting on a doula who
  is at a birth
- **Entry point**: an invitation from Renata, accepted at `/accept-invite`
- **Done looks like**: an Engagement with a Doula assigned, a signed Contract, and
  an Invoice with a recorded Payment — all without Dee ever opening a Care Plan or
  logging a Visit.

## Moment of truth

**Stage 9 — recording the Payment.** Their journey is named "first call to signed
and **billed**", and their "Done looks like" requires an Invoice with a recorded
Payment. Payments are written only by the Stripe webhook, so a cheque, a bank
transfer, or a cash deposit — the normal case for a small practice — cannot be
recorded at all. There is no workaround.

This is **not** the out-of-scope Stripe gap. A live Stripe account would not fix
it. The missing capability is manual Payment recording (DW-G3).

The competing candidate was Stage 5, assigning the Doula (RA-G4). It loses
because Dee has a human workaround there — they tell the doula out of band, the
same way they do today. A missing capability with a workaround is friction. A book
they cannot close is a break.

## Words

Dee is a domain expert on the business half and a stranger to the care half.

| Domain term | What Dee says | Note |
| --- | --- | --- |
| Admin | "office manager" | Dee's own word for themself is the word `CONTEXT.md` ruled out — **and the screen agrees with Dee, not with `CONTEXT.md`** (RA-G3) |
| Engagement | "the file", "a booking" | |
| Contract | "the agreement" | `CONTEXT.md` avoids "agreement" |
| Invoice | "the bill" | `CONTEXT.md` avoids "bill" |
| Care Plan / Birth Plan | "the doula's stuff" | They do not want to read either, which is itself the answer to a live question — see Stage 10 |

## Stages

### Stage 1 — Accept the invitation

**Thinking**: "Renata said she'd send this."
**Pain points**: no email arrives (RA-G1); Renata pastes a link into a text.

- **1.1** — Open the invite link at `/accept-invite`.
- **1.2** — Set email and password; press **Accept invite**
  (`POST /api/staff/accept-invite`). The membership is created with zero roles.
- **1.3** — Choose the Practice from the membership list.

### Stage 2 — Receive the Admin role

**Thinking**: "Am I set up?"
**Pain points**: this stage cannot be walked — there is no role UI (RA-G2). And
even once the role is set, nothing reads it.

- **2.1** — Renata sets `office_manager` on Dee's membership. No screen does this.

### Stage 3 — Discover what the Admin role grants

**Thinking**: "What am I allowed to do here?"
**Pain points**: the permission model is binary owner / non-owner. `owner` is the
only role ever checked, at `payments/connect.go`, `plans/template.go`,
`contracts/template.go`, and `staffauth/roles.go` (plus the invite and Staff-list
handlers). Neither `office_manager` nor `doula` is read anywhere. **Dee is
indistinguishable from any other non-owner Staff member.** The Admin role grants
nothing and withholds nothing.

- **3.1** — Land on `/practices/[practiceId]` and see the non-owner tiles only:
  Clients and Billing.
- **3.2** — Open Billing and read the Practice's credit balance and purchase
  ledger. `billing/balance.go` takes any Staff member, so a non-owner sees what
  the Practice spends. Buying credits is correctly refused
  (`billing/purchase.go` requires Owner).

### Stage 4 — Take the call and create the Client

**Thinking**: "She is due in March, wants a birth doula, heard about us from her
midwife."
**Pain points**: they take a page of notes on the call and the form accepts two
fields. Everything else goes back into a notebook, which is the problem the
product was meant to solve.

- **4.1** — `/practices/[practiceId]/clients/new`, enter name and email.
- **4.2** — Press **Add Client** (`POST /api/practices/{id}/clients`). Client and
  Engagement are created together at status `intake`. Not owner-gated, so this
  **passes** for Dee.

### Stage 5 — Assign a Doula

**Thinking**: "Priya has room in March."
**Pain points**: there is no assignment. See RA-G4.

- **5.1** — Open the Engagement and look for an assignment control. There is none.
- **5.2** — The nearest act is to create a Visit naming a Staff member — a
  dateless record that does not express coverage.

### Stage 6 — Send the Contract

**Thinking**: "Get it out today."
**Pain points**: none identified. The Contract *template* is owner-gated
(`contracts/template.go:75`), but building and sending a per-Engagement Contract
is not, so Dee can do the whole thing.

- **6.1** — Send the portal invite
  (`POST /api/practices/{id}/engagements/{id}/portal-invite`) so the Client can
  sign in.
- **6.2** — Build the Contract (`POST .../contract`), status `draft`.
- **6.3** — Send it (`POST .../contract/send`), status `sent`.

### Stage 7 — Track the signature

**Thinking**: "Has she signed yet?"
**Pain points**: they must open each Engagement to check. There is no unsigned-
contract list (RA-G6).

- **7.1** — Open the Engagement and read the Contract status
  (`draft` / `sent` / `signed` / `voided`).

> **Nadia crossing.** `POST .../contract/void` exists and no stage in any
> practice-side map exercises it. Voiding a Contract is the practice-side surface
> of an Engagement that ends early — which is Nadia Haddad's path. The method
> standard says to walk her journey first where it overlaps another. Stages 7–9
> here, and Priya's Stages 7–8, are the crossing points and may need revision once
> her map exists (blocked behind #210 and #206).

### Stage 8 — Raise the Invoice

**Thinking**: "Bill the deposit."
**Pain points**: the endpoint is **not** owner-gated, so the role is not the
blocker — Stripe Connect is. Without a connected account the handler returns
`connectRequired`, and because Dee is not an Owner it returns `isOwner: false`,
which the UI turns into "Ask a Practice Owner to connect Stripe." Dee is stopped
by an infrastructure gap wearing the costume of a permission error.

- **8.1** — `POST /api/practices/{id}/engagements/{id}/contract/invoices`.
- **8.2** — Read the "ask an Owner" message.

> The persona file lists `payments/invoice.go` among the owner-gated places. That
> is imprecise: the file computes `isOwner` only to choose which message to show.
> The persona note should be corrected.

### Stage 9 — Record the Payment — moment of truth

**Thinking**: "She paid by bank transfer. Mark it paid."
**Pain points**: there is no way to. Payments are written only by the Stripe
webhook. A cheque, a transfer, or a cash deposit — the normal case for a small
practice — cannot be recorded at all, and no Stripe account exists either.

- **9.1** — Look for a way to mark an Invoice paid. There is none.

### Stage 10 — Read a filled Care Plan or Birth Plan

**Thinking**: "I do not want to read this, but sometimes I have to check a date in
it."
**Pain points**: the question is genuinely open. `GET .../plans/{planType}` has no
role check, so **Dee can read every filled Care Plan and Birth Plan today**.
Whether they should is undecided and this journey cannot decide it alone — the
Care Plan is defined in `CONTEXT.md` as staff-only internal notes, which does not
by itself exclude an Admin.

- **10.1** — Open an Engagement and read both plan sections.

Record it as an open decision, not as a pass or a failure.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| DW-G1 | 3 | Both | The Admin role grants nothing. The permission model is binary owner / non-owner, and `office_manager` is never read. Dee is indistinguishable from any other non-owner. |
| DW-G2 | 8 | Experience | Missing Stripe infrastructure surfaces to a non-owner as "ask a Practice Owner" — an infrastructure gap that reads as a permission error. |
| DW-G3 | 9 | Interaction | No manual Payment recording. Payments are written only by the Stripe webhook, so a cheque or bank transfer cannot be recorded. |
| DW-G4 | 3 | Interaction | The Billing balance and ledger are not owner-gated in the UI or the API (`billing/balance.go:86` takes any Staff member), so any non-owner sees the Practice's spending. Buying credits is correctly owner-gated (`billing/purchase.go:33`). |
| DW-G5 | 7 | Both | No unsigned-contract or outstanding-work list. Chasing signatures means opening every Engagement in turn. |

Also hit here, filed on their owning maps: **RA-G1** (no invite email),
**RA-G2** (no role UI), **RA-G3** (`office_manager` on screen), **RA-G4** (no
Doula on an Engagement), **MO-G3** (Client takes name and email only).

## Open decisions

Not gaps, and not `journey-gap` issues. This journey exposes them; it cannot
settle them alone.

- ~~**May an Admin read a filled Care Plan or Birth Plan?**~~ **Settled: yes, both.**
  [ADR-0006](../adr/0006-read-follows-the-role.md) reads "staff-only" as *not the
  Client*, which does not exclude any Staff role. Today's ungated behaviour is
  correct for Dee — but not for a Doula, who under the same ADR loses the Contract's
  money and the credit ledger. See DW-G4.
