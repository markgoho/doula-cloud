# Lena Vasquez — from an offered job to a finished, paid one

- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **Goal**: carry one Client for a Practice she does not belong to, see the money on that job, and see nothing else of the Practice
- **Entry point**: an emailed invitation to Rooted Birth Collective, accepted on an account Lena already holds at another agency — after which jobs are offered to her one at a time, and she takes them or refuses them
- **Done looks like**: the Engagement she took is finished, she can point to what she agreed and what she was paid, and she never saw a Client who was not hers

She is the second negative-permission Persona and the tighter one. Priya Raman is refused the money; Lena is refused the Practice.

> **Assign or invite?** [#211](https://github.com/markgoho/doula-cloud/issues/211) left it to this map. It is decided here as **offered — she accepts or declines** — see [Decided here](#decided-here-offered-not-assigned), below the stages. The stages are written to that decision.

## Moment of truth

**Stage 3 — the offer, before she has said yes.** This is the screen that decides whether DoulaCloud is worth anything to a contractor, and it is the hardest screen in her journey to get right: it must tell her enough to take or refuse the job — who the Client is, when, and for how much — while she is still an outsider with no claim on any of it. Too little and she goes back to the phone and the product is a notification service. Too much and the agency's book is open to someone who has not agreed to anything.

Her second-hardest moment is stage 5, reading back the fee months later; it is the durable-record half of the same need. Both are design findings, not priority signals.

## Words

| Domain term | What Lena says | Note |
| --- | --- | --- |
| Engagement | "a job", "the February birth" | Priya says "my client"; Lena says "a job". The same noun, two relationships to it |
| Offer | "what Renata sent me", "the February one" | New to the model as of this map. She has no special word for it and does not need one — but she does need to tell an offer she has not answered from one she has taken |
| Practice | "the agency", "Renata's" | She belongs to none of them |
| Staff | — she rejects the word | "I'm not staff, I just work with them." `GLOSSARY.md` calls every member Staff, and the roster screen will show her among the employees. The divergence is the finding, not the wording: the model has no way to say what she is |
| Employment type | "I'm a contractor", "I'm 1099" | A term she uses about herself daily and the schema cannot hold at all |
| Contract | "the terms", "my rate" | The Contract is between the Practice and the Client. Lena reads it as the description of *her* job — a second, unmodeled reading of the same row |
| Visit | "a prenatal", "the birth" | Same three kinds Priya names; the model still has one and it carries no type (MO-G1, MO-G2) |
| Client | "her", "the mom" | As Priya. `GLOSSARY.md` avoids "mom" deliberately |

## Stages

### Stage 1 — Join the agency, on an account she already has

Joining is not taking a job. She becomes a member of Rooted Birth Collective once; jobs are offered to her afterwards, one at a time, in stage 3. Renata phones her first, but nothing said in that call has to reach the product.

**Thinking**: "I already sign in to this thing for the other agency." **Pain points**: none from the account collision this stage was named for — that refusal is gone. `staffauth`'s accept path resolves her existing `staff` row by `identity_uid` (`resolveStaff`, `accept.go:306`) instead of inserting a second one, and inserts a `practice_memberships` row for Rooted Birth Collective carrying the roles Renata's invitation set (`accept.go:226`) — the person the schema's own comment always described, one who "can work at more than one Practice via separate `practice_memberships` rows" (`00002_practice_staff_tenancy.sql`, lines 16–18), is exactly who this route now creates. `InviteHandler` no longer writes a `staff` row or a membership at invite time either (`invite.go:54`), so a failed acceptance leaves nothing behind to sweep up (LV-G8, closed); and where one does need sweeping, a membership can now be removed outright, through `DELETE .../staff/{staffId}/membership`, which the roster's own **Remove from practice** control reaches. What remains: no email carries the invite link (RA-G1), so Renata still pastes a URL into a text.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **1.1** — Open the invite link at `/accept-invite`.
- **1.2** — Sign in as herself and submit (`POST /api/staff/accept-invite`). Expected: a **200**, holding the same `staffId` her other agency already gave her and Rooted Birth Collective's `practiceId` — the accept path found her existing row rather than colliding with it.
- **1.3** — The new membership carries the roles Renata's invitation set, not the empty roles a pending row used to carry before she ever pressed accept.

### Stage 2 — Sign in and choose the agency

**Thinking**: "Which one is this — Renata's or the other one?" **Pain points**: she is the only Persona who meets the Practice picker as a real decision rather than a formality, and she meets it every time. Nothing on it says what she is at each Practice. The walk confirmed the picker renders — it is the only place in the effort where it ever has, since every other Persona holds one membership.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **2.1** — `/login` (`POST /api/session`).
- **2.2** — Choose Rooted Birth Collective from the list.
- **2.3** — Land on `/practices/[practiceId]`. Owner-only tiles are hidden by `{#if roles.includes('owner')}`; **Clients, Billing and Payments remain**, exactly as they do for Priya — Payments sits outside the owner block (RA-G9). Billing is the Practice's own credit spending and Payments its Stripe state, neither of which is her business at all — and she is not even the employee they were already wrong for (DW-G4). What the walk added is that she cannot spend the agency's credits by adding a Client — she is refused outright before she reaches that far (LV-G9, closed) — but she can still price and send a Contract on a Client who is not hers (PR-G8).

### Stage 3 — The offer — moment of truth

**Thinking**: "Who is she, when is it, and what does it pay? Do I want it?" **Pain points**: **none of this stage exists.** An Engagement cannot be offered to anybody: it carries no Doula at all (RA-G4), so there is no attachment for an offer to lead to, and no Offer to lead there with (LV-G6). Nor is there a rule for what she may read while she decides — [ADR-0006](../adr/0006-read-follows-the-role.md)'s read table has four columns and none of them is *offered, not yet accepted*, which this map has just made a real state of the model (LV-G7). This is the stage her whole relationship with the product is built on, and it is a hole.

The tension is the design problem, and it is not resolvable by leaving it to the phone call: an offer must say enough to be taken or refused — Client, dates, on-call terms, fee — while she is still an outsider with no claim on any of it. If she has to ring Renata to find out what she is being offered, the offer screen is a notification and the decision never left the phone.

Once she has a Client at the Practice, she meets an Offer on the Overview, which holds two decisions at once, each headed group one under the hub rule: her Offers, and Recent activity ([#486](https://github.com/markgoho/doula-cloud/issues/486)). [#2228](https://github.com/markgoho/doula-cloud/issues/2228) raised the budget to 3 for the Quick links; the founder dropped them on 2026-10-31 ([#2398](https://github.com/markgoho/doula-cloud/issues/2398)), so it is 2 again.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **3.1** — Receive the offer of the February Engagement.
- **3.2** — Read enough to decide.
- **3.3** — Accept, which is what attaches her — or decline, which must be recorded, so Renata can see the refusal and offer the work on.

### Stage 4 — Find the one job she took

**Thinking**: "Just the February one." **Pain points**: **she sees the whole agency.** `GET /api/practices/{id}/clients` is Practice-scoped by design and says so in the handler ("v1 has no restricted-visibility model"), and there is no column marking which Engagement is hers, because Engagements carry no Doula (RA-G4). For Priya this is a scope failure. For Lena it is a confidentiality failure between two businesses: an outside contractor reading the agency's entire client list.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **4.1** — Open `/practices/[practiceId]/clients`.
- **4.2** — Read every Client at Rooted Birth Collective, and pick hers out by remembering the name from the offer.

### Stage 5 — Check the terms and the fee

**Thinking**: "Three prenatals, on call from the 8th, and the number we said." **Pain points**: the Contract read hands back prose, merge fields and values in one object with no role check (`contract.go:136`), so today she gets the money — by accident, on every Engagement in the Practice, not only on hers. [ADR-0006](../adr/0006-read-follows-the-role.md) says she *should* read the money on her own work and Priya should not, which means the Contract read must be able to return scope without money and money with it. That single missing split is **PR-G2**, minted on Priya's map; Lena is the other half of it and does not re-mint. What she cannot get at all is the part that is hers: the Contract is between the Practice and the Client, and it records the Client's fee, not Lena's. Her rate lives in the phone call.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Memory**: yes (Working Memory)
* **320px**: yes (ADR-0024)

- **5.1** — Open `/practices/[practiceId]/engagements/[engagementId]`.
- **5.2** — Read the Contract section (`GET .../contract`): prose, merge fields, values.
- **5.3** — Look for what *she* is owed. Nothing on the page holds it (LV-G3).

### Stage 6 — Do the work

**Thinking**: "Same as any of my own Clients." **Pain points**: identical to Priya's stages 6–8, and filed there. The Birth Plan is a section partway down a long page with no deep link (PR-G5); a Visit is `(engagement_id, staff_id, created_at)`, so it carries no date, no type and no note (MO-G1, MO-G2, PR-G6). Nothing here is different for a contractor, which is itself worth recording: the care work is the half of her journey the product already treats correctly.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **6.1** — Read the Birth Plan (`GET .../plans/birth`).
- **6.2** — Log Visits (`POST .../visits`).
- **6.3** — Message the Client (`POST .../messages`).

### Stage 7 — Get paid

**Thinking**: "Invoice Renata, and check it against what we said in November." **Pain points**: she still invoices Renata from her own books, since DoulaCloud moves no money to a Doula ([#391](https://github.com/markgoho/doula-cloud/issues/391)). Once Renata pays her, Renata records it on the Engagement (**Doula fees**, **Mark fee paid**), and Lena points to it on **Your offers**: the day she accepted, her fee, and "Paid $450.00 on …, by bank transfer". The Practice's record and hers are one record now (LV-G3, closed by [#2271](https://github.com/markgoho/doula-cloud/issues/2271)). An accepted Offer no longer names the Client, so she tells two jobs apart by the day she accepted and the fee.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **320px**: yes (ADR-0024)

- **7.1** — She invoices Renata by email, from her own books. When Renata has recorded paying her, she opens **Your offers** and reads what she was paid on the accepted Offer.

### Stage 8 — The job ends

**Thinking**: "That one's done. Next." **Pain points**: nothing expresses an attachment ending. Whatever eventually grants her a read of "the Engagements she is attached to" has no modeled way to stop granting it, so a contractor who worked one birth in February still reads that Client in December — including, once the money split exists, the money. Removing her membership entirely is the only conceivable lever, it is wrong — it erases her from the Visits she worked — and **the product does not have it**: `api/main.go` mounts no route that deletes a membership or a `staff` row. What an Owner can actually press is **End sessions everywhere**, which ends a sign-in and not a read (confirmed on the walk: `204`, and Lena signed straight back in and read the book). This is the mirror of RA-G4 rather than a restatement of it — RA-G4 is that attachment cannot be *made*; LV-G4 is that it cannot be *ended* (LV-G4).

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **8.1** — No product step.

## Decided here: offered, not assigned

**Not a stage.** [#211](https://github.com/markgoho/doula-cloud/issues/211) settled that a contractor Doula reads *the Engagements she is attached to* and deliberately left open how she becomes attached: an Owner **assigns** her, taking effect at once, or the Practice **offers** her the job and she accepts or declines. Either way the read set once she is attached is the same. The difference is whether *offered* is a state the model holds.

**Decided: the Practice offers, and Lena accepts or declines. Her acceptance is what attaches her.**

The reason is what she is. She is outside the business, and a person outside the business is not someone work happens to — the difference between a contractor and an employee is precisely that she may refuse. A record that shows her carrying a job she never agreed to is not a record of a contract; it is the agency writing down its own version of an agreement it has not yet got. Assignment would have modeled her as a smaller Priya Raman, and she is not one.

Assignment was the cheaper option and it was argued for: the negotiation happens on the phone, so attachment would only record a yes already given; the product already has exactly one assignment concept (a Visit's `staff_id`) with no handshake; and declining stays real either way, because she can always say no before anything is recorded. It loses because it makes the phone call load-bearing. A product that can only record agreements reached elsewhere cannot be the place two businesses agree.

**What it obliges — the price, and it is not small.**

1. **An Offer is a new concept in the model** (LV-G6), with its own terminal states: accepted, declined, and probably withdrawn. Declined has to be durable, or Renata cannot tell "she said no" from "she has not looked yet". Settled in [#229](https://github.com/markgoho/doula-cloud/issues/229) as six states — the two guessed at here plus `withdrawn`, `superseded` (another Doula was faster) and `expired`.
2. **A fifth read state.** ADR-0006's table describes what each role reads about a Practice's Engagements. It now needs a column for a Doula who has been *offered* an Engagement and has not accepted: enough to decide on, no more (LV-G7). That column is the hardest part of this decision and the reason it is the moment of truth.
3. **Detachment is still separate.** Acceptance is not its own undo — a job that ends in June must stop granting reads, and nothing expresses that (LV-G4). Offering solves how attachment begins; it says nothing about how it ends.

**Not reopened by**: an Owner wanting to skip the handshake for her own employees. Employed Doulas read every Engagement at the Practice already ([ADR-0006](../adr/0006-read-follows-the-role.md)), so nothing here becomes a step everyone must take.

**Amended by [#229](https://github.com/markgoho/doula-cloud/issues/229)**: offering is no longer the contractor's route *only*. An employee may be offered work too, and the sentence above is why it costs her nothing — the Offer grants her no reach she lacks, it settles the **claim** that she is on this birth. Offering stays **mandatory** for a contractor and **optional** for an employee, who may still simply be assigned.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| LV-G1 | 1 | Both | A membership carries no **employment type**. `practice_memberships` is `(practice_id, staff_id, roles, created_at)` (`00002_practice_staff_tenancy.sql:29`), and nothing anywhere distinguishes an employed Doula from a contracted one, so neither half of the contractor read rule in [ADR-0006](../adr/0006-read-follows-the-role.md) — the money she may see, the Practice she may not — is expressible. Distinct root from RA-G8: putting roles on the invitation does not add an attribute that is not a role. On the experience layer it is why she appears on the Staff roster among employees, under a word she does not use about herself. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G2 | 1 | Interaction | **Closed 2026-08-25, [#225](https://github.com/markgoho/doula-cloud/issues/225).** `staffauth`'s accept path (`resolveStaff`, `accept.go:306`) reads the caller's existing `staff` row by `identity_uid` before deciding whether to insert one, and inserts a `practice_memberships` row on the found row for a second Practice — `staff.identity_uid` stays `UNIQUE` (`00002_practice_staff_tenancy.sql:21`), but as the one-row-per-identity mechanism `00002`'s own comment (lines 16–18) always described, not as the blocker. Nothing of the original finding survives: a person invited to a second Practice on an account she already holds joins it. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G3 | 5, 7 | Both | **Closed 2026-10-31, [#2271](https://github.com/markgoho/doula-cloud/issues/2271).** The paid half is a **Fee Payment** (GLOSSARY.md): a record, never a payment. An Owner or Admin presses **Mark fee paid** in the Engagement's **Doula fees** section, for an ended Attachment too, and records the method, the day and a note against her whole accepted fee (`attachment_fee_payments`, `00136`); a mistake is removed by a reversal beside it. Lena reads "Paid" and "$450.00 on …, by bank transfer" on **Your offers**, which she keeps after the job ends. The story before: nothing records what a Practice owes a doula. `invoices` is `(practice_id, contract_id, amount_cents, …)` (`00024_invoices.sql:16`) — the Practice billing the Client. A contractor's own rate is not on the Contract, which prices the Client's care, and there is no other place for it, so half of her "done looks like" cannot be reached and the two sides keep separate books. **The owed half is answered (second walk, [#2177](https://github.com/markgoho/doula-cloud/issues/2177)):** the fee she accepted is on the Offer, the attachment and her Engagement ("Your fee $450.00, the fee you accepted on …"). **The paid half is open:** [#225](https://github.com/markgoho/doula-cloud/issues/225) closed without it, [#391](https://github.com/markgoho/doula-cloud/issues/391) ruled the payout out, and whether DoulaCloud records one is [#2271](https://github.com/markgoho/doula-cloud/issues/2271). | [#2271](https://github.com/markgoho/doula-cloud/issues/2271) |
| LV-G4 | 8 | Interaction | An attachment cannot end. Nothing expresses a Doula ceasing to be attached to an Engagement, so a contractor's read never lapses when the job does — and the only available lever, removing her membership, erases her from the Visits she worked. The mirror of RA-G4, not a restatement: RA-G4 is that attachment cannot be **made**. Both are settled together or neither is. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G5 | 4 | Experience | An outside contractor reads the agency's entire client list. Same root as PR-G1 on the interaction layer, but a different finding: for Priya it is a scope failure inside one team; for Lena it is one business reading another's book. It is the reason "restricted visibility" cannot stay a v1 deferral — it is filed here as the experience-layer consequence, and fixing PR-G1 fixes it. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G6 | 3 | Interaction | An Engagement cannot be **offered**. This map decides that a contractor is attached by accepting an offer, and the model holds no such thing: no Offer, no acceptance, and no durable record of a decline — so Renata cannot tell a refusal from silence, and cannot offer the work on. Distinct root from RA-G4, which is that attachment cannot be recorded at all: an Offer is the transition RA-G4's attachment would be the result of, and building one without the other is impossible in either order. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G7 | 3 | Both | There is no read rule for a Doula who has been offered an Engagement and has not accepted. [ADR-0006](../adr/0006-read-follows-the-role.md)'s table has four columns — Owner, Admin, employee Doula, contractor Doula — and this map adds a fifth state to the model that none of them describes. The rule must let her decide (Client, dates, on-call terms, fee) without opening the Practice to someone who has agreed to nothing, and it is what makes the offer screen a decision rather than a notification. Amends ADR-0006; distinct from LV-G6, which is the concept, not the permission. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| LV-G8 | 1 | Interaction | **Closed 2026-08-25, [#291](https://github.com/markgoho/doula-cloud/issues/291).** `InviteHandler` inserts a `practice_invitations` row and no `staff` row or membership (`invite.go:54`) — no person exists until she proves control of the invited address by accepting — so a failed acceptance has nothing to leave behind. A membership can also now be removed outright, through `DELETE /api/practices/{practiceId}/staff/{staffId}/membership`, which the roster's **Remove from practice** control reaches, and the roster itself separates a pending invitation from a member who holds no roles. Found on the walk ([#238](https://github.com/markgoho/doula-cloud/issues/238)). | [#291](https://github.com/markgoho/doula-cloud/issues/291) |
| LV-G9 | 2, 4 | Interaction | **Closed 2026-09-20, [#1236](https://github.com/markgoho/doula-cloud/issues/1236).** Saving a Client spends no Credit at all: `client.CreateHandler`'s own test asserts zero `credit_ledger` rows and zero `engagements` rows after a save, and a Credit locks only when an Engagement Request is approved (ADR-0017), an act `engagementrequest.ApproveHandler` restricts to an Owner or Admin (`staffauth.RequireOwnerOrAdmin`). Nor can a contractor Doula reach either act at all: `CreateHandler` refuses her with *"You contract for this Practice, so you cannot add a Client here. Work reaches you as an Offer."*, and `engagementrequest.RequestHandler` refuses her with *"You contract for this Practice, so you cannot request an Engagement here. Work reaches you as an Offer."* — each backed independently by an RLS policy (`clients_insert`, `00042_client_intake_schema.sql`; `engagement_requests_insert`, `00047_engagement_request_endpoints.sql`). Nothing of the original finding survives: not the spend at Add Client, and not an outsider reaching it. | [#292](https://github.com/markgoho/doula-cloud/issues/292) |
| LV-G10 | 1 | Both | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The accept screen lands on the Practice the Invitation was for, from the accept response's `practiceId`, with "You've joined *Practice*. You're a Doula, as a contractor. Your other Practices are unchanged." focused once on its Overview ([#2466](https://github.com/markgoho/doula-cloud/pull/2466)). That Overview's first read stamps it as her last-used Practice (`staffauth.Middleware`'s `stampActivity`), so her next sign-in goes there; `app/e2e/accept-second-practice.e2e.ts` drives both ([#2265](https://github.com/markgoho/doula-cloud/issues/2265)). Was: accepting a second Practice's invitation on an account she already holds answers `200` with the new Practice's id. Then `land()` (`accept-invite/+page.svelte`) ignores that id and calls `decideLanding`, which goes to her last-used Practice. So she lands on the *other* agency's Overview, and nothing says she joined anything. **Confirmation**: yes (Peak-End Rule) — measured no. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2265](https://github.com/markgoho/doula-cloud/issues/2265) |
| LV-G11 | 2 | Experience | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The switcher and the Practice picker now say what she is at each Practice, the roles and then the employment type: "Doula, as a contractor" under an agency she contracts to, "Doula, as an employee" under one she works at. The words are the roster's and the invite form's Employee and Contractor, set apart with "as" because a bare comma read as a second role to the three personas asked ([#2266](https://github.com/markgoho/doula-cloud/issues/2266)). Was: the header's Practice switcher shows her role at each Practice ("Doula") and never her employment type, though the Membership records it and the roster shows it. Doula is the word she would see under an agency she works at and under one she contracts to, alike. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2266](https://github.com/markgoho/doula-cloud/issues/2266) |
| LV-G12 | 3 | Interaction | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** An Offer to someone already at the Practice now queues a Platform Notification in the same transaction (`staff_notice_outbox`, one table for every notice to one Staff member about another person's act), nudged as the email path is. It reads "You've been offered work on DoulaCloud", names neither the Practice nor the Client, links to `/practices/<the sending Practice>/offers`, and says "Answer by (the expiry, in the Practice's zone, zone named)". An Offer answered, withdrawn or lapsed before the send, or a recipient no longer at the Practice, mails nobody. A decline or an expiry mails nobody either; GLOSSARY.md's Offer entry records why. Measured on the fix's own stack on 2026-11-01: the seeded open Offer to Fern Okada arrived in the catcher, and its link, opened signed out, went to sign-in and then to Rooted's **Your offers** with the Offer on it. Was: an Offer to someone already at the Practice sends no mail and no other notice: `offer.CreateHandler` queues the outbox only on the email path. She learns of it only by signing in to that Practice, and an Offer expires. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2267](https://github.com/markgoho/doula-cloud/issues/2267) |
| LV-G13 | 3 | Experience | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** Both answers, on **Your offers** and on the Overview's "Offers awaiting your answer", now end in a status that is announced and focused, because the card it answered is gone. **Accept** reads "You accepted. You're on (Client's full name)'s birth." ("postpartum care" for a postpartum Engagement), with a link "Open (Client)'s birth" to the Engagement. The accept response carries the Engagement, its kind and the Client's name, which her new Attachment lets her read. **Decline** first asks "Decline this offer?" with "You can't undo this. (Practice) can send you a new offer later.", **Keep the offer** and **Decline offer**. This is the house's hard confirm (`ConfirmDialog`; block-over-warn in `docs/design/govuk-alignment.md`'s modal Dialog paragraph), not an undo, because a decline is final for that Offer. Then: "You declined. (Practice) can see that on the Client's page." A decline mails nobody (GLOSSARY.md's Offer entry), so the line says where the Practice sees it. Wording from three persona consults on #2268. Measured on the fix's own stack at 320px on 2026-11-01: Fern Okada declined on the Overview and accepted a re-offer on **Your offers**. Both statuses took focus, the link opened Reese Calloway's Engagement, and nothing was wider than 320px. Was: neither answer to an Offer confirmed what it did. **Decline** removes the card with no message and no word that the Practice was told, and with no check first, though declining is final. **Accept** turns the card into "Accepted" with no Client, no Engagement and no link. **Confirmation**: yes (Peak-End Rule) — measured no. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2268](https://github.com/markgoho/doula-cloud/issues/2268) |
| LV-G14 | 5 | Experience | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The Engagement's Contract section now reads "Terms" with the terms she accepted under "Your fee", from the Attachment's own copy of the Offer's terms (`fee_terms`), so only she reads it. An accepted Offer on **Your offers** links to its Engagement, "Open the birth due Feb 2027", while she still reaches it; an employee keeps the link, a contractor loses it when her Attachment ends. The Practice-side Offers list is unchanged. Labels from three persona consults on #2269. Was: her Engagement shows the fee she accepted and not the terms. **Your offers** shows the terms, without the Client or a link. **Memory**: yes (Working Memory) — measured no. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2269](https://github.com/markgoho/doula-cloud/issues/2269) |
| LV-G15 | 8 | Experience | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The Engagement's read now answers a contractor whose own Attachment on it ended with `410 ATTACHMENT_ENDED` and one sentence, and the page reads "Your work has ended", "Your work on the birth due Feb 2027 ended on Oct 31, 2026." and "Your fee, the terms and what you were paid stay on Your offers.", with a **Your offers** link. Nothing else about the Engagement is in the answer, and a contractor never on it still gets `404`. **Your offers** reads "Ended" with the day beside the accepted Offer. Completing re-reads On call, which then reads "Care has ended." with no Doula listed. What she keeps is recorded in GLOSSARY.md's Attachment entry: her Offer, never her Visits, Birth Plan answers or Messages (ADR-0008's open axis). Was: when the Engagement completes, her attachment ends, as LV-G4's fix intends. But what she meets is "Page not found" on the address she had, and nothing anywhere says the job ended or when. The completed Engagement's On call section still lists her. **Confirmation**: yes (Peak-End Rule) — measured no. Found on the second walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)). | [#2270](https://github.com/markgoho/doula-cloud/issues/2270) |

Also hit here, filed on their owning maps: **RA-G1** (no invite email), **RA-G4** (no Doula on an Engagement — which is the whole of her read scope), **RA-G8** (an invitation carries no roles), **PR-G1** (the Client list is Practice-wide), **PR-G2** (the Contract read cannot separate scope from money — Lena is its other half), **PR-G5** (no phone-first path to the Birth Plan), **PR-G6**, **MO-G1** and **MO-G2** (dateless, type-less, note-less Visits), **DW-G4** (Billing readable by any Staff member), **TB-G3** (what a Credit buys is unsettled), **DW-G14** (a plan's answers read only through its editor or the print page; built, waiting on a walk, [#2264](https://github.com/markgoho/doula-cloud/issues/2264)). The walk added four more: **RA-G9** (Payments is outside the owner gate), **RA-G10** (a Visit cannot name anyone), **PR-G8** (a Doula-only member prices and sends a Contract) and **PR-G9** (the Engagement page renders no links) — each confirmed from outside the business, which is a sharper reading of the same defect but not a second one.
