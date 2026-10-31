# Dee Whitlock — first call to signed and billed

- **Persona**: [non-doula-admin.md](../personas/non-doula-admin.md)
- **Goal**: do the paperwork end of an Engagement without waiting on a doula who is at a birth
- **Entry point**: an invitation from Renata, accepted at `/accept-invite`
- **Done looks like**: an Engagement with a Doula on it, a signed Contract, and an Invoice with a recorded Payment — all without Dee ever opening a Care Plan or logging a Visit.

## Moment of truth

**Stage 9 — recording the Payment.** Their journey is named "first call to signed and **billed**", and their "Done looks like" requires an Invoice with a recorded Payment. Payments are written only by the Stripe webhook, so a check, a bank transfer, or a cash deposit — the normal case for a small practice — cannot be recorded at all. There is no workaround.

This is **not** the out-of-scope Stripe gap. A live Stripe account would not fix it. The missing capability is manual Payment recording (DW-G3).

The competing candidate was Stage 5, putting a Doula on the Engagement (RA-G4).

**The walk broke the original argument and it is re-stated here.** It read: stage 5 loses because Dee can create a Visit naming the doula instead. They cannot — `POST .../visits` is Doula-gated and refuses them (**DW-G6**). The *product* workaround never existed.

The verdict still holds, on the ground that is actually left. Stage 5's workaround is out of band and always was: Dee tells the doula, the same way they do today, and the work happens. Nothing about the business stops. Stage 9 has no equivalent — a payment that arrived cannot be written down anywhere, in or out of the product, and the book stays open. Friction that a phone call absorbs is not the same as a record that cannot exist.

## Words

Dee is a domain expert on the business half and a stranger to the care half.

| Domain term | What Dee says | Note |
| --- | --- | --- |
| Admin | "office manager" | Dee's own word for themself is the word `GLOSSARY.md` ruled out. The screen now agrees with `GLOSSARY.md`, not with Dee: `rolesLabel` renders `Admin` (RA-G3, closed) |
| Engagement | "the file", "a booking" | |
| Contract | "the agreement" | `GLOSSARY.md` avoids "agreement" |
| Invoice | "the bill" | `GLOSSARY.md` avoids "bill" |
| Care Plan / Birth Plan | "the doula's stuff" | They do not want to read either, which is itself the answer to a live question — see Stage 10 |

## Stages

### Stage 1 — Accept the invitation

**Thinking**: "Renata said she'd send this." **Pain points**: no email arrives (RA-G1); Renata pastes a link into a text. The accept screen is one screen since [#2238](https://github.com/markgoho/doula-cloud/issues/2238): it reads the Invitation by its token first, so it names the Practice, the roles and the invited address, and asks no account question and no email. A person new here still gives four answers only she can give, each on a founder record: her first name and last name ([#1537](https://github.com/markgoho/doula-cloud/issues/1537), decided on [#1493](https://github.com/markgoho/doula-cloud/issues/1493)), the state she works from ([#415](https://github.com/markgoho/doula-cloud/issues/415), decided on [#389](https://github.com/markgoho/doula-cloud/issues/389)) and a password (ADR-0026), and then presses **Accept invite**. Two screens of three would make three screens; one screen of five keeps the two, so the Decisions budget is 5.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **1.1** — Open the invite link at `/accept-invite`.
- **1.2** — On the one accept screen, headed "Join *Practice name*", give first name, last name, the state she works from and a new password; press **Accept invite** (`POST /api/staff/accept-invite`). The Membership carries the roles and employment type the Invitation named, and the Overview she lands on says "You've joined *Practice name*. You're {roles}, as {an employee}." once, on arrival ([#2239](https://github.com/markgoho/doula-cloud/issues/2239)).
- **1.3** — ~~Choose the Practice from the membership list.~~ **There is nothing to choose**: acceptance leaves one membership, so `decideLanding` redirects straight to `/practices/{id}` (`app/src/lib/landing.ts:24-26`) and the picker never renders. Same root as Renata's 1.2 — the picker needs two memberships, which only Lena Vasquez's journey gives a Persona.

### Stage 2 — Receive the Admin role

**Thinking**: "Am I set up?" **Pain points**: this stage cannot be walked — there is no role UI (RA-G2). And even once the role is set, nothing reads it.

- **2.1** — Renata sets `admin` on Dee's membership. No screen does this.

### Stage 3 — Discover what the Admin role grants

**Thinking**: "What am I allowed to do here?" **Pain points**: the permission model is binary owner / non-owner. `owner` is the only role ever checked, at `payments/connect.go`, `plans/template.go`, `contracts/template.go`, and `staffauth/roles.go` (plus the invite and Staff-list handlers). Neither `office_manager` nor `doula` is read anywhere. **Dee is indistinguishable from any other non-owner Staff member.** The Admin role grants nothing and withholds nothing.

> **Partly overtaken by the code.** The pain point above records the walk as it happened. `admin` is read on trunk now — ADR-0008's read table is enforced at the `staffauth.GatedRouter` mount seam, and [#267](https://github.com/markgoho/doula-cloud/issues/267) moved `GET .../payments/connect` onto `staffauth.OwnerAndAdmin`, so Dee sees this Practice's Stripe Connect state. Only the write in `payments/connect.go` stays Owner-only.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **3.1** — Land on `/practices/[practiceId]` and see the non-owner tiles: Clients, Billing **and Payments**. Payments sits outside `{#if roles.includes('owner')}` (**RA-G9**, found by [#235](https://github.com/markgoho/doula-cloud/issues/235) and confirmed here on a zero-role membership).
- **3.2** — Open Billing and read the Practice's credit balance and purchase ledger. `billing/balance.go` takes any Staff member, so a non-owner sees what the Practice spends. Buying credits is correctly refused (`billing/purchase.go` requires Owner).

### Stage 4 — Take the call and create the Client

**Thinking**: "She is due in March, wants a birth doula, heard about us from her midwife." **Pain points**: they take a page of notes on the call, and the form accepted two fields when this map was drawn — everything else went back into a notebook, which is the problem the product was meant to solve. **That is closed** ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252), ADR-0017): the notes have somewhere to go. The Start work form asks the kind of work, the due date and who the Doula is in one act, by decision ([#1515](https://github.com/markgoho/doula-cloud/issues/1515)), so its six decisions are the stage's largest screen ([TB-G11](evaluator-doula.md#gaps-found) holds the reason).

**Budget**:

* **Screens**: 8 (Hick's Law, Flow)
* **Decisions**: 6 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **4.1** — **Find or add a Client** on the Clients list, search, then **Add a new Client**: intake is one question from there, the name, and **Save and continue** opens the Start work form. The date of birth, email, phone, address and the Practice's own Client Field Template sections are added from the Client's record.
- **4.2** — Press **Save and continue**. The save is **free** — a Client, no Engagement, no credit — and it is not owner-gated, so it **passes** for Dee. It opens the Start work form: asking for paid work with the Client is the separate act at `POST .../engagement-requests`, and its approval is what creates the Engagement and locks the Credit. **Go to {name}'s record without starting work** leaves the Client saved and nothing started.

### Stage 5 — Put a Doula on the Engagement

**Thinking**: "Priya has room in March." **Pain points**: an Engagement has no Doula on it, and nothing puts one there. See RA-G4. On a new Engagement the Doula is now named on the Start work form, by decision ([#1515](https://github.com/markgoho/doula-cloud/issues/1515)), which rejected naming her afterward *in place of* asking at the start, so the stage's screen is that form and its six decisions ([TB-G11](evaluator-doula.md#gaps-found) holds the reason). Putting a Doula on an Engagement that already exists is [RA-G13](practice-owner.md#gaps-found)'s, on Renata Alvarez's stage 4, under its own budget of 2: the Doula row's **Change**, "Who is the Doula?" and **Save the Doula** ([#2230](https://github.com/markgoho/doula-cloud/issues/2230)).

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 6 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **5.1** — Open the Engagement and look for a control that puts a Doula on it. There is none.
- **5.2** — ~~The nearest act is to create a Visit naming a Staff member.~~ **There is no nearest act.** `POST .../visits` requires the Doula role (`api/internal/visit/roles.go:41`), which Dee does not hold, so the button is on her screen and answers `403 only a Staff member with the Doula role can do that`. Stage 5 has no in-product act for her at all (**DW-G6**).

### Stage 6 — Send the Contract

**Thinking**: "Get it out today." **Pain points**: none identified. The Contract *template* is owner-gated (`contracts/template.go:75`), but building and sending a per-Engagement Contract is not, so Dee can do the whole thing.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **6.1** — Send the portal invite (`POST /api/practices/{id}/engagements/{id}/portal-invite`) so the Client can sign in.
- **6.2** — Build the Contract (`POST .../contract`), status `draft`.
- **6.3** — Send it (`POST .../contract/send`), status `sent`.

### Stage 7 — Track the signature

**Thinking**: "Has she signed yet?" **Pain points**: they must open each Engagement to check. There is no unsigned- contract list (RA-G6).

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **7.1** — Open the Engagement and read the Contract status (`draft` / `sent` / `signed` / `voided`).

> **Nadia crossing.** `POST .../contract/void` exists and no stage in any practice-side map exercises it. Voiding a Contract is the practice-side surface of an Engagement that ends early — which is Nadia Haddad's path. The method standard says to walk her journey first where it overlaps another. Stages 7–9 here, and Priya's Stages 7–8, are the crossing points. [Her map](loss-client.md) now exists and **these stages are unchanged**: what she adds is what the void looks like from the Client's side — the bare word `voided` (**NH-G5**) — and the absence of any Invoice surface in the portal (**NH-G6**), both hers to own. The Staff steps here stand.

### Stage 8 — Raise the Invoice

**Thinking**: "Bill the deposit." **Pain points**: the endpoint is **not** owner-gated, so the role is not the blocker — Stripe Connect is. Without a connected account the handler returns `connectRequired`. `IsOwner` carries `omitempty` (`api/internal/payments/invoice.go:51`), so for a non-owner the field is *absent* from the body rather than `false` — the wire response is `{"connectRequired":true}` and the client defaults it (`+page.svelte:311`, `result.isOwner ?? false`). Either way the UI shows "Ask a Practice Owner to connect Stripe." Dee is stopped by an infrastructure gap wearing the costume of a permission error.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **8.1** — `POST /api/practices/{id}/engagements/{id}/contract/invoices`.
- **8.2** — Read the "ask an Owner" message.

> The persona file lists `payments/invoice.go` among the owner-gated places. That is imprecise: the file computes `isOwner` only to choose which message to show. The persona note should be corrected.

### Stage 9 — Record the Payment — moment of truth

**Thinking**: "She paid by bank transfer. Mark it paid." **Pain points**: on the first run there was no way to: a check, a transfer, or a cash deposit — the normal case for a small practice — could not be recorded at all. [DW-G3](https://github.com/markgoho/doula-cloud/issues/271) built **Record payment** on the Invoice's row. Its shape is by decision, and the budget follows it (DW-G11). [#271](https://github.com/markgoho/doula-cloud/issues/271) asks for a check step before the Payment is written, and the founder's ruling on [#1923](https://github.com/markgoho/doula-cloud/issues/1923) opens Date received on today because that step shows the date again, so the stage crosses three screens from the Invoice: the Invoice, the form, and its check step. #271 records the Invoice's whole amount, with no partial Payments, so the form asks no amount; it asks the date, how it was paid, and an optional note, which `other` requires, then the save or the cancel: five decisions. The **Invoices** list lands on the Invoice itself in its Engagement's Invoices section ([#2240](https://github.com/markgoho/doula-cloud/issues/2240)).

**Budget**:

* **Screens**: 3 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **9.1** — Open the Invoice from **Invoices**, then **Record payment** on its row, the form, and its check step.

### Stage 10 — Read a filled Care Plan or Birth Plan

**Thinking**: "I do not want to read this, but sometimes I have to check a date in it." **Pain points**: the question is genuinely open. `GET .../plans/{planType}` has no role check, so **Dee can read every filled Care Plan and Birth Plan today**. Whether they should is undecided and this journey cannot decide it alone — the Care Plan is defined in `GLOSSARY.md` as staff-only internal notes, which does not by itself exclude an Admin.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **10.1** — Open an Engagement and read both plan sections.

Record it as an open decision, not as a pass or a failure.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| DW-G1 | 3 | Both | **Closed.** The Admin role granted nothing when this map was drawn; `admin` is read now for the Practice's Stripe Connect state ([#267](https://github.com/markgoho/doula-cloud/issues/267)) — see the Stage 3 note above. | [#269](https://github.com/markgoho/doula-cloud/issues/269) |
| DW-G2 | 8 | Experience | Missing Stripe infrastructure surfaces to a non-owner as "ask a Practice Owner" — an infrastructure gap that reads as a permission error. | [#270](https://github.com/markgoho/doula-cloud/issues/270) |
| DW-G3 | 9 | Interaction | No manual Payment recording. Payments are written only by the Stripe webhook, so a check or bank transfer cannot be recorded. | [#271](https://github.com/markgoho/doula-cloud/issues/271) |
| DW-G4 | 3 | Interaction | The Billing balance and ledger are not owner-gated in the UI or the API (`billing/balance.go:86` takes any Staff member), so any non-owner sees the Practice's spending. Buying credits is correctly owner-gated (`billing/purchase.go:33`). | [#272](https://github.com/markgoho/doula-cloud/issues/272) |
| DW-G5 | 7 | Both | No unsigned-contract or outstanding-work list. Chasing signatures means opening every Engagement in turn. | [#273](https://github.com/markgoho/doula-cloud/issues/273) |
| DW-G6 | 5 | Interaction | The Admin cannot record anything about care, not even as a proxy. `owner` and `doula` are the only roles the codebase reads, and Dee holds neither, so the one assignment-shaped act in the product — `POST .../visits` — refuses them (`api/internal/visit/roles.go:41`). **Add a Visit** renders on their screen and 403s. This is the other half of DW-G1: the Admin role grants nothing, *and* the two roles that do grant something both exclude them. | [#274](https://github.com/markgoho/doula-cloud/issues/274) |
| DW-G7 | 7, 8 | Interaction | **A voided Contract can still be billed, and with Stripe connected it really is.** `POST .../contract/invoices` never reads the Contract's status, and **Create Invoice** keeps rendering after the void. Walked twice: before Connect it reached the gate (`200 {"connectRequired":true}`); after Connect the same call answered `201` and Stripe finalized a **$500 invoice with a hosted payment URL** against an agreement the Practice had already voided. `POST .../contract/send` and `POST .../contract/void` both 409 correctly on a voided Contract; the Invoice route is the one that does not. | [#275](https://github.com/markgoho/doula-cloud/issues/275) |
| DW-G8 | every stage | Both | **No screen in the product has a title.** `document.title` is `""` on `/login`, the practice landing, Clients, Billing and the Engagement page, so every tab is blank and SvelteKit's own live region announces `untitled page` to a screen reader on each client-side navigation. Found on Dee's walk because it is on every screen Dee opens; it is app-wide, not theirs. | [#276](https://github.com/markgoho/doula-cloud/issues/276) |
| DW-G9 | 1 | Interaction | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)); Decisions closed by a budget change.** **Accepting a Staff invite crosses three screens of four decisions each.** `**Screens**: 2 (Hick's Law, Flow)` — measured 3: **Accept your Staff invite** is two screens (the account question with email and password, then name and work state), then the Overview. `**Decisions**: 3 (Hick's Law, Miller's Law)` — measured 4 on each accept screen; the Overview's 9 is RA-G11's. Second walk, 2026-10-28. Built on #2238: the screen reads the Invitation before she types (`POST /api/staff/accept-invite/lookup`), so the account question and the email field are gone and the stage is the accept screen and the Overview, 2 screens. A person new here answers first name, last name, work state and a new password and presses **Accept invite**: 5 decisions, each answer on a founder record (#1537, #415, ADR-0026), so the Decisions budget is 5, with the reason in Pain points. Someone who already has an account types her password only: 2. Measured on the fix's own stack at 1440px and 320px on 2026-10-31, nothing wider than 320px: a new Admin crossed 2 screens with 5 decisions; a contractor already at Rooted, joining a second Practice, crossed 2 screens with 2 decisions and landed on the Practice she joined. | [#2238](https://github.com/markgoho/doula-cloud/issues/2238) |
| DW-G10 | 1 | Experience | **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** **The screen after accepting never says she joined, or as what.** `**Confirmation**: yes (Peak-End Rule)` — measured no. The Overview names the Practice in the top bar and nothing else; "Joined as Admin (Employee)" is only on her own Staff page. The invitee's version of TB-G9. Second walk, 2026-10-28. Built on #2239: accepting lands on the Overview of the Practice she joined, which opens, on arrival only, with "You've joined Rooted Birth Collective. You're an Admin, as an employee." as a focused status before its heading, the carrier TB-G9 uses. Measured on the fix's own stack at 320px on 2026-10-31: the line was focused on arrival and gone after a reload. | [#2239](https://github.com/markgoho/doula-cloud/issues/2239) |
| DW-G11 | 9 | Interaction | **Closed by a budget change.** Recording a bank transfer failed `**Screens**: 2 (Hick's Law, Flow). The Invoice, then the form that records the Payment` — measured 4: Invoices, the Engagement's Invoices section, **Record payment**, its check step — and `**Decisions**: 4 (Hick's Law, Miller's Law). The amount, the date it arrived, how it was paid, and the save` — measured 5: Method, Note, Date received, **Continue**, **Cancel**; no amount field. Second walk, 2026-10-28. Both lines were set before [#271](https://github.com/markgoho/doula-cloud/issues/271) chose the form's shape: its check step, which the founder's ruling on [#1923](https://github.com/markgoho/doula-cloud/issues/1923) relies on, and the Invoice's whole amount with no amount to ask. The budget is now 3 and 5, with the reason in stage 9's Pain points, and an **Invoices** row now lands on its Invoice, not the top of the Engagement. Measured again 2026-10-31: 3 and 5. | [#2240](https://github.com/markgoho/doula-cloud/issues/2240) |
| DW-G12 | 9 | Both | **Closed.** A recorded Payment never said how it was paid, and nothing confirmed it. `**Confirmation**: yes (Peak-End Rule). The Invoice reads paid, says by what and when, and the book is closed` — measured no. The row reads Paid and the date; the method and note are on no screen and not in the Invoice read, no message says it was recorded, and focus falls to the page. Second walk, 2026-10-28. Now **Confirm and record** leaves a focused status line, "Payment of $950.00 recorded: bank transfer, received Oct 26, 2026.", and the Invoice reads **Paid by** and the **Note** for an Owner, an Admin and an employee Doula, never a contractor. Measured again 2026-10-31: yes. | [#2241](https://github.com/markgoho/doula-cloud/issues/2241) |
| DW-G13 | 10 | Interaction | **Reaching a Client's plans from the Clients list crosses three screens.** `**Screens**: 1 (Hick's Law, Flow)` — measured 3: Clients, the Client's record, the Engagement. RA-G13 counts the same path on Renata's stage 4. Second walk, 2026-10-28. | [#2242](https://github.com/markgoho/doula-cloud/issues/2242) |
| DW-G14 | 10 | Experience | **A Care Plan's answers are read only through its editor.** Each plan section shows progress and who last edited it; reading an answer means **Change the Care Plan**, a form with **Save Care Plan**. The Birth Plan has **Print for the hospital**; the Care Plan has no read view on the Staff side. Second walk, 2026-10-28. | [#2243](https://github.com/markgoho/doula-cloud/issues/2243) |

Also hit here, filed on their owning maps: **RA-G1** (no invite email), **RA-G2** (no role UI), **RA-G3** (raw enum on screen, closed), **RA-G4** (no Doula on an Engagement), **MO-G3** (Client takes name and email only).

## Open decisions

Not gaps, and not `journey-gap` issues. This journey exposes them; it cannot settle them alone.

- ~~**May an Admin read a filled Care Plan or Birth Plan?**~~ **Settled: yes, both.** [ADR-0006](../adr/0006-read-follows-the-role.md) reads "staff-only" as *not the Client*, which does not exclude any Staff role. Today's ungated behavior is correct for Dee — but not for a Doula, who under the same ADR loses the Contract's money and the credit ledger. See DW-G4.
