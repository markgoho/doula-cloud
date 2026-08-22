# Lena Vasquez — from an offer over the phone to a finished, paid job

- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **Goal**: carry one Client for a Practice she does not belong to, see the money on
  that job, and see nothing else of the Practice
- **Entry point**: Renata's phone call offering a February birth, then an emailed
  invitation to Rooted Birth Collective — on an account Lena already holds elsewhere
- **Done looks like**: the Engagement she took is finished, she can point to what she
  agreed and what she was paid, and she never saw a Client who was not hers

She is the second negative-permission Persona and the tighter one. Priya Raman is
refused the money; Lena is refused the Practice.

> **Assign or invite?** [#211](https://github.com/markgoho/doula-cloud/issues/211)
> left it to this map. It is decided here as **assigned** — see
> [Decided here](#decided-here-assigned-not-invited), below the stages. The stages
> are written to that decision.

## Moment of truth

**Stage 5 — the fee on the job, months after the phone call.** Everything before it
she could do by text message with Renata. This is the one thing only the product can
give her: a durable, shared record of what she agreed to, readable by both sides,
that she does not have to keep herself. If she has to maintain her own spreadsheet
of what each agency owes her, the Practice's record and hers drift apart, and she
stops trusting the one she did not write.

It is a design finding, not a priority signal.

## Words

| Domain term | What Lena says | Note |
| --- | --- | --- |
| Engagement | "a job", "the February birth" | Priya says "my client"; Lena says "a job". The same noun, two relationships to it |
| Practice | "the agency", "Renata's" | She belongs to none of them |
| Staff | — she rejects the word | "I'm not staff, I just work with them." `CONTEXT.md` calls every member Staff, and the roster screen will show her among the employees. The divergence is the finding, not the wording: the model has no way to say what she is |
| Employment type | "I'm a contractor", "I'm 1099" | A term she uses about herself daily and the schema cannot hold at all |
| Contract | "the terms", "my rate" | The Contract is between the Practice and the Client. Lena reads it as the description of *her* job — a second, unmodelled reading of the same row |
| Visit | "a prenatal", "the birth" | Same three kinds Priya names; the model still has one and it carries no type (MO-G1, MO-G2) |
| Client | "her", "the mom" | As Priya. `CONTEXT.md` avoids "mom" deliberately |

## Stages

### Stage 1 — The offer, on the phone

**Thinking**: "Do I want this, and is the money worth it?"
**Pain points**: none in the product, because the product is not here. The whole
negotiation — Client, dates, on-call window, fee — happens in a phone call, and the
app learns of it only after Lena has said yes. This is the stage that decides the
assign-or-invite question below: everything that would make an in-app offer worth
reacting to is settled before there is anything to react to.

- **1.1** — No product step. Renata calls; Lena says yes.

### Stage 2 — Accept the invitation, on an account she already has

**Thinking**: "I already sign in to this thing for the other agency."
**Pain points**: **this stage is expected to fail outright.** `InviteHandler` always
inserts a new `staff` row (`invite.go:58`), and acceptance writes her identity onto
that new row (`accept.go:101`) — but `staff.identity_uid` is `UNIQUE`
(`00002_practice_staff_tenancy.sql:21`), and hers is already on the row from the
other agency. The person the schema's own comment describes — one who "can work at
more than one Practice via separate `practice_memberships` rows" (lines 16–18) —
cannot be created through the only route that reaches her (LV-G2). No email carries
the link either (RA-G1), so Renata pastes a URL into a text.

- **2.1** — Open the invite link at `/accept-invite`.
- **2.2** — Sign in as herself and submit
  (`POST /api/staff/accept-invite`). Expected: a unique-constraint failure surfacing
  as a 500, not a second membership.
- **2.3** — The membership, if it were created, would carry `roles = '{}'`
  (`invite.go:67`) and no employment type, because there is no column for one
  (RA-G8, LV-G1).

### Stage 3 — Sign in and choose the agency

**Thinking**: "Which one is this — Renata's or the other one?"
**Pain points**: she is the only Persona who meets the Practice picker as a real
decision rather than a formality, and she meets it every time. Nothing on it says
what she is at each Practice.

- **3.1** — `/login` (`POST /api/session`).
- **3.2** — Choose Rooted Birth Collective from the list.
- **3.3** — Land on `/practices/[practiceId]`. Owner-only tiles are hidden by
  `{#if roles.includes('owner')}`; **Clients and Billing remain**, exactly as they do
  for Priya. Billing is the Practice's own credit spending, which is not her business
  at all — she is not even the employee it was already wrong for (DW-G4).

### Stage 4 — Find the one job she took

**Thinking**: "Just the February one."
**Pain points**: **she sees the whole agency.** `GET /api/practices/{id}/clients` is
Practice-scoped by design and says so in the handler ("v1 has no restricted-visibility
model"), and there is no column marking which Engagement is hers, because Engagements
carry no Doula (RA-G4). For Priya this is a scope failure. For Lena it is a
confidentiality failure between two businesses: an outside contractor reading the
agency's entire client list.

- **4.1** — Open `/practices/[practiceId]/clients`.
- **4.2** — Read every Client at Rooted Birth Collective, and pick hers out by
  remembering the name from the phone call.

### Stage 5 — Check the terms and the fee — moment of truth

**Thinking**: "Three prenatals, on call from the 8th, and the number we said."
**Pain points**: the Contract read hands back prose, merge fields and values in one
object with no role check (`contract.go:136`), so today she gets the money — by
accident, on every Engagement in the Practice, not only on hers.
[ADR-0006](../adr/0006-read-follows-the-role.md) says she *should* read the money on
her own work and Priya should not, which means the Contract read must be able to
return scope without money and money with it. That single missing split is **PR-G2**,
minted on Priya's map; Lena is the other half of it and does not re-mint. What she
cannot get at all is the part that is hers: the Contract is between the Practice and
the Client, and it records the Client's fee, not Lena's. Her rate lives in the phone
call.

- **5.1** — Open `/practices/[practiceId]/engagements/[engagementId]`.
- **5.2** — Read the Contract section (`GET .../contract`): prose, merge fields,
  values.
- **5.3** — Look for what *she* is owed. Nothing on the page holds it (LV-G3).

### Stage 6 — Do the work

**Thinking**: "Same as any of my own Clients."
**Pain points**: identical to Priya's stages 6–8, and filed there. The Birth Plan is
a section partway down a long page with no deep link (PR-G5); a Visit is
`(engagement_id, staff_id, created_at)`, so it carries no date, no type and no note
(MO-G1, MO-G2, PR-G6). Nothing here is different for a contractor, which is itself
worth recording: the care work is the half of her journey the product already treats
correctly.

- **6.1** — Read the Birth Plan (`GET .../plans/birth`).
- **6.2** — Log Visits (`POST .../visits`).
- **6.3** — Message the Client (`POST .../messages`).

### Stage 7 — Get paid

**Thinking**: "Invoice Renata, and check it against what we said in November."
**Pain points**: this stage has no product in it. `invoices` rows carry a
`practice_id` and a `contract_id` (`00024_invoices.sql:16`) — the Practice billing the
Client. There is no record anywhere of a Practice owing a doula, so the second half
of her "done looks like" — *point to what she was paid* — is unanswerable, and the
Practice's record and hers are guaranteed to be separate documents (LV-G3). Credits
do not help: they are Doula Cloud's own billing, bought only by an Owner, and what a
Credit even buys is unsettled in code (TB-G3).

- **7.1** — No product step. She invoices Renata by email, from her own books.

### Stage 8 — The job ends

**Thinking**: "That one's done. Next."
**Pain points**: nothing expresses an attachment ending. Whatever eventually grants
her a read of "the Engagements she is attached to" has no modelled way to stop
granting it, so a contractor who worked one birth in February still reads that Client
in December — including, once the money split exists, the money. Removing her
membership entirely is the only available lever, and it is wrong: it erases her from
the Visits she worked. This is the mirror of RA-G4 rather than a restatement of it —
RA-G4 is that attachment cannot be *made*; LV-G4 is that it cannot be *ended*
(LV-G4).

- **8.1** — No product step.

## Decided here: assigned, not invited

**Not a stage.** [#211](https://github.com/markgoho/doula-cloud/issues/211) settled
that a contractor Doula reads *the Engagements she is attached to* and deliberately
left open how she becomes attached: an Owner **assigns** her, taking effect at once,
or the Practice **invites** her to a job and she accepts or declines. Either way the
read set is the same. The difference is whether "offered" is a state the model holds.

**Decided: an Owner attaches her. There is no in-app offer, and no acceptance.**

Four reasons, in the order they carried weight:

1. **An offer she cannot read is not a decision.** An offer worth accepting or
   declining must show her the Client, the dates and the money. She may not read any
   of that until she is attached — that is the whole rule. So an in-app offer needs a
   *fifth* read state that ADR-0006 does not have: what a not-yet-attached Doula may
   see about an Engagement she has been offered. Building the offer means building
   that state, and it exists only to serve a conversation that has already happened.
2. **Stage 1 is where the yes happens, and it is outside the product.** Renata calls
   and names the fee. By the time anything could appear on Lena's screen, she has
   already said yes. The app's job is to record the agreement, not to collect it.
3. **The product already has exactly one assignment concept** — a Visit's `staff_id`
   — and it is a bare assignment with no handshake. A second, richer one would make
   two ways of saying who is doing the work.
4. **Declining stays real.** She says no on the phone, and no attachment is made.
   Nothing about assignment obliges her to take work; it obliges the Owner to wait
   for the yes before recording it.

**The cost, accepted.** The model cannot tell "offered" from "agreed", and the
Practice's record shows Lena attached to a job she never touched a button for. If a
Practice ever needs the app itself to carry the offer — a marketplace of open work
rather than a phone call between two people who know each other — this decision is
what gets revisited, and it is a new question, not this one reopened.

**What it obliges.** Assignment is only half a lifecycle. Because there is no
acceptance to withdraw, detachment has to be a first-class operation, not an
afterthought — see LV-G4.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| LV-G1 | 2 | Both | A membership carries no **employment type**. `practice_memberships` is `(practice_id, staff_id, roles, created_at)` (`00002_practice_staff_tenancy.sql:29`), and nothing anywhere distinguishes an employed Doula from a contracted one, so neither half of the contractor read rule in [ADR-0006](../adr/0006-read-follows-the-role.md) — the money she may see, the Practice she may not — is expressible. Distinct root from RA-G8: putting roles on the invitation does not add an attribute that is not a role. On the experience layer it is why she appears on the Staff roster among employees, under a word she does not use about herself. |
| LV-G2 | 2 | Interaction | A person cannot be Staff at two Practices. `InviteHandler` always inserts a fresh `staff` row (`invite.go:58`) and acceptance writes the caller's identity onto it (`accept.go:101`), but `staff.identity_uid` is `UNIQUE` (`00002_practice_staff_tenancy.sql:21`) — so inviting someone who already has an account cannot produce a second membership. `00002` promises multi-Practice membership in its own comment (lines 16–18). The contractor is the first Persona for whom this is the normal case rather than an oddity. |
| LV-G3 | 5, 7 | Both | Nothing records what a Practice owes a doula. `invoices` is `(practice_id, contract_id, amount_cents, …)` (`00024_invoices.sql:16`) — the Practice billing the Client. A contractor's own rate is not on the Contract, which prices the Client's care, and there is no other place for it, so half of her "done looks like" cannot be reached and the two sides keep separate books. |
| LV-G4 | 8 | Interaction | An attachment cannot end. Nothing expresses a Doula ceasing to be attached to an Engagement, so a contractor's read never lapses when the job does — and the only available lever, removing her membership, erases her from the Visits she worked. The mirror of RA-G4, not a restatement: RA-G4 is that attachment cannot be **made**. Both are settled together or neither is. |
| LV-G5 | 4 | Experience | An outside contractor reads the agency's entire client list. Same root as PR-G1 on the interaction layer, but a different finding: for Priya it is a scope failure inside one team; for Lena it is one business reading another's book. It is the reason "restricted visibility" cannot stay a v1 deferral — it is filed here as the experience-layer consequence, and fixing PR-G1 fixes it. |

Also hit here, filed on their owning maps: **RA-G1** (no invite email), **RA-G4** (no
Doula on an Engagement — which is the whole of her read scope), **RA-G8** (an
invitation carries no roles), **PR-G1** (the Client list is Practice-wide),
**PR-G2** (the Contract read cannot separate scope from money — Lena is its other
half), **PR-G5** (no phone-first path to the Birth Plan), **PR-G6**, **MO-G1** and
**MO-G2** (dateless, type-less, note-less Visits), **DW-G4** (Billing readable by any
Staff member), **TB-G3** (what a Credit buys is unsettled).
