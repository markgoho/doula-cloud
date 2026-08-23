# Lena Vasquez — from an offered job to a finished, paid one

- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **Goal**: carry one Client for a Practice she does not belong to, see the money on
  that job, and see nothing else of the Practice
- **Entry point**: an emailed invitation to Rooted Birth Collective, accepted on an
  account Lena already holds at another agency — after which jobs are offered to her
  one at a time, and she takes them or refuses them
- **Done looks like**: the Engagement she took is finished, she can point to what she
  agreed and what she was paid, and she never saw a Client who was not hers

She is the second negative-permission Persona and the tighter one. Priya Raman is
refused the money; Lena is refused the Practice.

> **Assign or invite?** [#211](https://github.com/markgoho/doula-cloud/issues/211)
> left it to this map. It is decided here as **offered — she accepts or declines** —
> see [Decided here](#decided-here-offered-not-assigned), below the stages. The
> stages are written to that decision.

## Moment of truth

**Stage 3 — the offer, before she has said yes.** This is the screen that decides
whether Doula Cloud is worth anything to a contractor, and it is the hardest screen
in her journey to get right: it must tell her enough to take or refuse the job — who
the Client is, when, and for how much — while she is still an outsider with no claim
on any of it. Too little and she goes back to the phone and the product is a
notification service. Too much and the agency's book is open to someone who has not
agreed to anything.

Her second-hardest moment is stage 5, reading back the fee months later; it is the
durable-record half of the same need. Both are design findings, not priority signals.

## Words

| Domain term | What Lena says | Note |
| --- | --- | --- |
| Engagement | "a job", "the February birth" | Priya says "my client"; Lena says "a job". The same noun, two relationships to it |
| Offer | "what Renata sent me", "the February one" | New to the model as of this map. She has no special word for it and does not need one — but she does need to tell an offer she has not answered from one she has taken |
| Practice | "the agency", "Renata's" | She belongs to none of them |
| Staff | — she rejects the word | "I'm not staff, I just work with them." `CONTEXT.md` calls every member Staff, and the roster screen will show her among the employees. The divergence is the finding, not the wording: the model has no way to say what she is |
| Employment type | "I'm a contractor", "I'm 1099" | A term she uses about herself daily and the schema cannot hold at all |
| Contract | "the terms", "my rate" | The Contract is between the Practice and the Client. Lena reads it as the description of *her* job — a second, unmodelled reading of the same row |
| Visit | "a prenatal", "the birth" | Same three kinds Priya names; the model still has one and it carries no type (MO-G1, MO-G2) |
| Client | "her", "the mom" | As Priya. `CONTEXT.md` avoids "mom" deliberately |

## Stages

### Stage 1 — Join the agency, on an account she already has

Joining is not taking a job. She becomes a member of Rooted Birth Collective once;
jobs are offered to her afterwards, one at a time, in stage 3. Renata phones her
first, but nothing said in that call has to reach the product.

**Thinking**: "I already sign in to this thing for the other agency."
**Pain points**: **this stage is expected to fail outright.** `InviteHandler` always
inserts a new `staff` row (`invite.go:58`), and acceptance writes her identity onto
that new row (`accept.go:101`) — but `staff.identity_uid` is `UNIQUE`
(`00002_practice_staff_tenancy.sql:21`), and hers is already on the row from the
other agency. The person the schema's own comment describes — one who "can work at
more than one Practice via separate `practice_memberships` rows" (lines 16–18) —
cannot be created through the only route that reaches her (LV-G2). No email carries
the link either (RA-G1), so Renata pastes a URL into a text. The refusal is clean and
what it leaves behind is not: the invite writes the pending `staff` row and its
`roles = '{}'` membership before she ever presses accept (`invite.go:56`, `:66`), in a
different request from the one that fails, so the roster gains a member who can never
sign in and whom no route removes (LV-G8, found on the walk).

- **1.1** — Open the invite link at `/accept-invite`.
- **1.2** — Sign in as herself and submit
  (`POST /api/staff/accept-invite`). Expected: a **409**, "a staff account already
  exists for this identity" — `accept.go:104` catches the unique violation
  deliberately (`isUniqueViolation`) rather than letting it surface as a 500. The
  refusal is clean; what is missing is the second membership behind it, not error
  handling.
- **1.3** — The membership, if it were created, would carry `roles = '{}'`
  (`invite.go:67`) and no employment type, because there is no column for one
  (RA-G8, LV-G1).

### Stage 2 — Sign in and choose the agency

**Thinking**: "Which one is this — Renata's or the other one?"
**Pain points**: she is the only Persona who meets the Practice picker as a real
decision rather than a formality, and she meets it every time. Nothing on it says
what she is at each Practice. The walk confirmed the picker renders — it is the only
place in the effort where it ever has, since every other Persona holds one membership.

- **2.1** — `/login` (`POST /api/session`).
- **2.2** — Choose Rooted Birth Collective from the list.
- **2.3** — Land on `/practices/[practiceId]`. Owner-only tiles are hidden by
  `{#if roles.includes('owner')}`; **Clients, Billing and Payments remain**, exactly as
  they do for Priya — Payments sits outside the owner block (RA-G9). Billing is the
  Practice's own credit spending and Payments its Stripe state, neither of which is her
  business at all — and she is not even the employee they were already wrong for
  (DW-G4). What the walk added is that the leak runs the other way too: nothing stops
  her *adding* a Client and spending one of the agency's credits (LV-G9), or pricing
  and sending a Contract on a Client who is not hers (PR-G8).

### Stage 3 — The offer — moment of truth

**Thinking**: "Who is she, when is it, and what does it pay? Do I want it?"
**Pain points**: **none of this stage exists.** An Engagement cannot be offered to
anybody: it carries no Doula at all (RA-G4), so there is no attachment for an offer
to lead to, and no Offer to lead there with (LV-G6). Nor is there a rule for what she
may read while she decides — [ADR-0006](../adr/0006-read-follows-the-role.md)'s read
table has four columns and none of them is *offered, not yet accepted*, which this
map has just made a real state of the model (LV-G7). This is the stage her whole
relationship with the product is built on, and it is a hole.

The tension is the design problem, and it is not resolvable by leaving it to the
phone call: an offer must say enough to be taken or refused — Client, dates, on-call
terms, fee — while she is still an outsider with no claim on any of it. If she has to
ring Renata to find out what she is being offered, the offer screen is a notification
and the decision never left the phone.

- **3.1** — Receive the offer of the February Engagement.
- **3.2** — Read enough to decide.
- **3.3** — Accept, which is what attaches her — or decline, which must be recorded,
  so Renata can see the refusal and offer the work on.

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
  remembering the name from the offer.

### Stage 5 — Check the terms and the fee

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
membership entirely is the only conceivable lever, it is wrong — it erases her from the
Visits she worked — and **the product does not have it**: `api/main.go` mounts no route
that deletes a membership or a `staff` row. What an Owner can actually press is **End
sessions everywhere**, which ends a sign-in and not a read (confirmed on the walk:
`204`, and Lena signed straight back in and read the book). This is the mirror of RA-G4 rather than a restatement of it —
RA-G4 is that attachment cannot be *made*; LV-G4 is that it cannot be *ended*
(LV-G4).

- **8.1** — No product step.

## Decided here: offered, not assigned

**Not a stage.** [#211](https://github.com/markgoho/doula-cloud/issues/211) settled
that a contractor Doula reads *the Engagements she is attached to* and deliberately
left open how she becomes attached: an Owner **assigns** her, taking effect at once,
or the Practice **offers** her the job and she accepts or declines. Either way the
read set once she is attached is the same. The difference is whether *offered* is a
state the model holds.

**Decided: the Practice offers, and Lena accepts or declines. Her acceptance is what
attaches her.**

The reason is what she is. She is outside the business, and a person outside the
business is not someone work happens to — the difference between a contractor and an
employee is precisely that she may refuse. A record that shows her carrying a job she
never agreed to is not a record of a contract; it is the agency writing down its own
version of an agreement it has not yet got. Assignment would have modelled her as a
smaller Priya Raman, and she is not one.

Assignment was the cheaper option and it was argued for: the negotiation happens on
the phone, so attachment would only record a yes already given; the product already
has exactly one assignment concept (a Visit's `staff_id`) with no handshake; and
declining stays real either way, because she can always say no before anything is
recorded. It loses because it makes the phone call load-bearing. A product that can
only record agreements reached elsewhere cannot be the place two businesses agree.

**What it obliges — the price, and it is not small.**

1. **An Offer is a new concept in the model** (LV-G6), with its own terminal states:
   accepted, declined, and probably withdrawn. Declined has to be durable, or Renata
   cannot tell "she said no" from "she has not looked yet". Settled in
   [#229](https://github.com/markgoho/doula-cloud/issues/229) as six states — the two
   guessed at here plus `withdrawn`, `superseded` (another Doula was faster) and
   `expired`.
2. **A fifth read state.** ADR-0006's table describes what each role reads about a
   Practice's Engagements. It now needs a column for a Doula who has been *offered*
   an Engagement and has not accepted: enough to decide on, no more (LV-G7). That
   column is the hardest part of this decision and the reason it is the moment of
   truth.
3. **Detachment is still separate.** Acceptance is not its own undo — a job that ends
   in June must stop granting reads, and nothing expresses that (LV-G4). Offering
   solves how attachment begins; it says nothing about how it ends.

**Not reopened by**: an Owner wanting to skip the handshake for her own employees.
Employed Doulas read every Engagement at the Practice already ([ADR-0006](../adr/0006-read-follows-the-role.md)),
so nothing here becomes a step everyone must take.

**Amended by [#229](https://github.com/markgoho/doula-cloud/issues/229)**: offering is
no longer the contractor's route *only*. An employee may be offered work too, and the
sentence above is why it costs her nothing — the Offer grants her no reach she lacks,
it settles the **claim** that she is on this birth. Offering stays **mandatory** for a
contractor and **optional** for an employee, who may still simply be assigned.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| LV-G1 | 1 | Both | A membership carries no **employment type**. `practice_memberships` is `(practice_id, staff_id, roles, created_at)` (`00002_practice_staff_tenancy.sql:29`), and nothing anywhere distinguishes an employed Doula from a contracted one, so neither half of the contractor read rule in [ADR-0006](../adr/0006-read-follows-the-role.md) — the money she may see, the Practice she may not — is expressible. Distinct root from RA-G8: putting roles on the invitation does not add an attribute that is not a role. On the experience layer it is why she appears on the Staff roster among employees, under a word she does not use about herself. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G2 | 1 | Interaction | A person cannot be Staff at two Practices. `InviteHandler` always inserts a fresh `staff` row (`invite.go:58`) and acceptance writes the caller's identity onto it (`accept.go:101`), but `staff.identity_uid` is `UNIQUE` (`00002_practice_staff_tenancy.sql:21`) — so inviting someone who already has an account cannot produce a second membership. `00002` promises multi-Practice membership in its own comment (lines 16–18). The contractor is the first Persona for whom this is the normal case rather than an oddity. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G3 | 5, 7 | Both | Nothing records what a Practice owes a doula. `invoices` is `(practice_id, contract_id, amount_cents, …)` (`00024_invoices.sql:16`) — the Practice billing the Client. A contractor's own rate is not on the Contract, which prices the Client's care, and there is no other place for it, so half of her "done looks like" cannot be reached and the two sides keep separate books. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G4 | 8 | Interaction | An attachment cannot end. Nothing expresses a Doula ceasing to be attached to an Engagement, so a contractor's read never lapses when the job does — and the only available lever, removing her membership, erases her from the Visits she worked. The mirror of RA-G4, not a restatement: RA-G4 is that attachment cannot be **made**. Both are settled together or neither is. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G5 | 4 | Experience | An outside contractor reads the agency's entire client list. Same root as PR-G1 on the interaction layer, but a different finding: for Priya it is a scope failure inside one team; for Lena it is one business reading another's book. It is the reason "restricted visibility" cannot stay a v1 deferral — it is filed here as the experience-layer consequence, and fixing PR-G1 fixes it. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G6 | 3 | Interaction | An Engagement cannot be **offered**. This map decides that a contractor is attached by accepting an offer, and the model holds no such thing: no Offer, no acceptance, and no durable record of a decline — so Renata cannot tell a refusal from silence, and cannot offer the work on. Distinct root from RA-G4, which is that attachment cannot be recorded at all: an Offer is the transition RA-G4's attachment would be the result of, and building one without the other is impossible in either order. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G7 | 3 | Both | There is no read rule for a Doula who has been offered an Engagement and has not accepted. [ADR-0006](../adr/0006-read-follows-the-role.md)'s table has four columns — Owner, Admin, employee Doula, contractor Doula — and this map adds a fifth state to the model that none of them describes. The rule must let her decide (Client, dates, on-call terms, fee) without opening the Practice to someone who has agreed to nothing, and it is what makes the offer screen a decision rather than a notification. Amends ADR-0006; distinct from LV-G6, which is the concept, not the permission. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G8 | 1 | Interaction | A failed acceptance leaves a member behind. `InviteHandler` writes the pending `staff` row and its `roles = '{}'` membership at **invite** time (`invite.go:56`, `:66`), so when acceptance 409s the two rows survive — they were committed by a different request and the rollback cannot reach them. The Practice keeps a member who can never sign in, printed on the Staff screen under the invitee's real name and email and indistinguishable from someone who has accepted and not been given roles yet, since acceptance also leaves `roles = []`. Nothing removes it: `api/main.go` mounts no route that deletes a membership or a `staff` row. Distinct root from LV-G2 — LV-G2 is that the second membership is unreachable, LV-G8 is that the failed attempt is not swept up. Found on the walk ([#238](https://github.com/markgoho/doula-cloud/issues/238)). | [#291](https://github.com/markgoho/doula-cloud/issues/291) |
| LV-G9 | 2, 4 | Interaction | Any Staff member spends the Practice's credits. `POST .../clients` is mounted behind `staffauth.Middleware` with no role check (`api/main.go:169`) and `engagement/create.go:97` consumes a credit in the same transaction, so a Doula who holds no other role — a contractor who does not belong to the business — adds a Client and takes the balance down. Filed here because the contractor is who makes it visible, but it is true of every non-owner: the read half of it is DW-G4 (a non-owner reads the ledger) and this is the write half, where an outsider draws on it. Distinct from PR-G8, which is the Contract write on a Client who is not hers; this one costs the Practice money. Found on the walk ([#238](https://github.com/markgoho/doula-cloud/issues/238)). | [#292](https://github.com/markgoho/doula-cloud/issues/292) |

Also hit here, filed on their owning maps: **RA-G1** (no invite email), **RA-G4** (no
Doula on an Engagement — which is the whole of her read scope), **RA-G8** (an
invitation carries no roles), **PR-G1** (the Client list is Practice-wide),
**PR-G2** (the Contract read cannot separate scope from money — Lena is its other
half), **PR-G5** (no phone-first path to the Birth Plan), **PR-G6**, **MO-G1** and
**MO-G2** (dateless, type-less, note-less Visits), **DW-G4** (Billing readable by any
Staff member), **TB-G3** (what a Credit buys is unsettled). The walk added four more:
**RA-G9** (Payments is outside the owner gate), **RA-G10** (a Visit cannot name
anyone), **PR-G8** (a Doula-only member prices and sends a Contract) and **PR-G9** (the
Engagement page renders no links) — each confirmed from outside the business, which is
a sharper reading of the same defect but not a second one.
