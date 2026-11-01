# Priya Raman — from invitation to caring for a Client of her own

- **Persona**: [employed-doula.md](../personas/employed-doula.md)
- **Goal**: know where she has to be, what this Client wants, and what she was told last time — without texting Renata
- **Entry point**: an emailed invitation from Renata, accepted at `/accept-invite`
- **Done looks like**: an active membership holding the Doula role, an Engagement she is on opened, a Visit logged, and messages exchanged with the Client — **and she never saw a screen she had no right to see.**

She is the negative-permission Persona. Half of this map is about what should be absent.

## Moment of truth

**Stage 6 — the Birth Plan, on a phone, in a hospital corridor.** Everything else she does could wait; this cannot. If she cannot get to this Client's preferences in seconds on the device in her hand, the product has failed at the only moment that matters to her.

> **Walked ([#237](https://github.com/markgoho/doula-cloud/issues/237)), and it fails somewhere else than this map expected.** Once she is on the Engagement page the Birth Plan is 0.83 screens down and reads correctly in about 1.7 s on a Pixel 7 — the burial is real but mild. What fails is *arriving*: the app's front door is the SvelteKit scaffold (**PR-G9**) and the Engagement page has no links on it, so from a cold start there is no path to this screen at all, only a URL she has kept. The moment of truth stands where this map put it; the reason it fails has moved from the scroll to the way in.

> **Walked again ([#2176](https://github.com/markgoho/doula-cloud/issues/2176)): the way in is fixed, and the corridor fails on its budgets.** From a cold phone she now reaches the Birth Plan unaided, and it has its own page she can print or download. On a Pixel 7 it is still two screens from the Engagement and five screens down it, the page she reads does not say when the plan last changed, and a saved address to it is lost through sign-in (**PR-G10**, **PR-G11**, **PR-G12**). The numbers are in the test plan's Run log.

> **Walked a third time (2026-11-01, [#2264](https://github.com/markgoho/doula-cloud/issues/2264)): the corridor holds on its budgets.** On an emulated Pixel 7, **Birth Plan** is in **Jump to** on the Engagement's first screen, the answers read in the section with no editor open, the Engagement settles in under 300 ms, the hospital copy names Camille Boyd, Priya Raman, Rooted Birth Collective and the day it last changed, and a saved address survives sign-in on a new phone (**PR-G10**, **PR-G11** and **PR-G12** are closed). What she came for was not there: the starter Birth Plan asks nothing about pain relief, so "an epidural unless she asks twice" has no place on the copy she hands a nurse ([#2580](https://github.com/markgoho/doula-cloud/issues/2580)).

## Words

| Domain term | What Priya says | Note |
| --- | --- | --- |
| Engagement | "my client", "the March one" | |
| Client | "the mom", "she" | `GLOSSARY.md` avoids "mom" deliberately. The Persona says it anyway — a real divergence between the model's language and a Doula's speech |
| Visit | "a prenatal", "the birth", "the postpartum" | She names three kinds; the model has one and it carries no type |
| Care Plan | "Renata's notes" | She reads them far more than she writes them |
| Birth Plan | "her birth plan" | Matches |

## Stages

### Stage 1 — Accept the invitation on the phone

**Thinking**: "First impression of my new job's software." **Pain points**: no email is sent (RA-G1). Renata texts a raw URL, which is a poor first impression and an unverifiable one — Priya cannot tell it is genuine. The accept screen is one screen since [#2238](https://github.com/markgoho/doula-cloud/issues/2238): it reads the Invitation by its token first, so it names the Practice, the roles and the invited address, and asks no account question and no email. A person new here still gives four answers only she can give, each on a founder record: her first name and last name ([#1537](https://github.com/markgoho/doula-cloud/issues/1537), decided on [#1493](https://github.com/markgoho/doula-cloud/issues/1493)), the state she works from ([#415](https://github.com/markgoho/doula-cloud/issues/415), decided on [#389](https://github.com/markgoho/doula-cloud/issues/389)) and a password (ADR-0026), and then presses **Accept invite**. Two screens of three would make three screens; one screen of five keeps the two, so the Decisions budget is 5.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **1.1** — Open the invite link at `/accept-invite`, on whatever device the message arrived on.
- **1.2** — On the one accept screen, headed "Join *Practice name*", give first name, last name, the state she works from and a new password; press **Accept invite** (`POST /api/staff/accept-invite`). The Membership carries the roles and employment type the Invitation named, and the Overview she lands on says "You've joined *Practice name*. You're {roles}, as {an employee}." once, on arrival ([#2239](https://github.com/markgoho/doula-cloud/issues/2239)).

### Stage 2 — Receive the Doula role

**Thinking**: none — invisible to her. **Pain points**: no role UI exists (RA-G2), and even once `doula` is set, nothing in the codebase reads it. The role is decorative.

- **2.1** — Renata sets the role. No screen does this.

### Stage 3 — Sign in and choose the Practice

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **3.1** — `/login` (`POST /api/session`).
- **3.2** — Choose Rooted Birth Collective.
- **3.3** — Land on `/practices/[practiceId]`. The owner-only tiles (Invite, Staff, Plan Templates, Contract Template) are hidden by `{#if roles.includes('owner')}`. **Clients, Billing and Payments remain** — Payments is outside the owner block (RA-G9, corrected by the walk of [#235](https://github.com/markgoho/doula-cloud/issues/235)), and Billing is the Practice's credit spending (DW-G4).

### Stage 4 — Find her Clients

**Thinking**: "Which ones are mine?" **Pain points**: **she sees all of them.** `GET /api/practices/{id}/clients` is Practice-scoped by design — the handler comment says so: "every Client with an Engagement at the current Practice, regardless of which Staff member created it — v1 has no restricted-visibility model." This is not a suspicion; it is the stated behavior. Her persona's scope requirement fails, and there is no column telling her which rows are hers, because Engagements carry no Doula (RA-G4).

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **4.1** — Open `/practices/[practiceId]/clients`.
- **4.2** — Read a list of every Client in the Practice, including other doulas'.

### Stage 5 — Open the Engagement

**Thinking**: "Right, her." **Pain points**: one long page holding Visits, Care Plan, Birth Plan, Contract, Invoices, and Messages. The Contract's amount and the Invoice history sit on the same page as the care record, for any Client — including ones that are not hers. None of the read paths are role-checked; owner checks live on write endpoints.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **5.1** — Open `/practices/[practiceId]/engagements/[engagementId]`.
- **5.2** — See the Contract section (`GET .../contract`) and the Invoices section (`GET .../contract/invoices`), neither of which she should need.

### Stage 6 — Read the Birth Plan — moment of truth

**Thinking**: "She did not want an epidural unless she asks twice." **Pain points**: the Birth Plan is a section partway down the page, on a phone, in a corridor, under time pressure. There is no deep link to it, no collapse of the sections she does not need, and no way to hand it to hospital staff from her side — the print stylesheet is on the Client's portal view. The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) confirmed all three and added the one that costs her most: there is no way to *reach* the page without a remembered URL (**PR-G9**). Each screen she reads on holds controls the founder chose before this stage was walked, so the Decisions budget is 3, not 1 ([#2257](https://github.com/markgoho/doula-cloud/issues/2257)): the Engagement's first screen has **Message the Client** ([#2146](https://github.com/markgoho/doula-cloud/issues/2146)), the **Jump to** list ([#2004](https://github.com/markgoho/doula-cloud/issues/2004)) and the summary card with its **Change** links ([#1998](https://github.com/markgoho/doula-cloud/issues/1998)); the Birth Plan section has **Change the Birth Plan** ([#2004](https://github.com/markgoho/doula-cloud/issues/2004), [#2243](https://github.com/markgoho/doula-cloud/issues/2243)) and **Print for the hospital** ([#280](https://github.com/markgoho/doula-cloud/issues/280)); the Birth Plan page has **Print** and **Download Birth Plan (PDF)** ([#306](https://github.com/markgoho/doula-cloud/issues/306)).

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **6.1** — Scroll to the Birth Plan section (`GET .../plans/birth`).
- **6.2** — Read it.

### Stage 7 — Log a Visit

**Thinking**: "Record what we covered so it is there next time." **Pain points**: this is the reason she came, and it does not work. A Visit is `(engagement_id, staff_id, created_at)`. She can record that a Visit happened, by her, now. She cannot record when it was, what kind it was, or what was said. Her stated need — "what was I told last time" — is unanswerable (MO-G1, MO-G2).

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **7.1** — Add a Visit in the Visits section (`POST /api/practices/{id}/engagements/{id}/visits`).
- **7.2** — See a row with a Staff name and a creation timestamp.

### Stage 8 — Message the Client

**Thinking**: "Confirm Thursday." **Pain points**: the message form shows four decisions at once, Message, Attachment, **Send** and **Cancel**, each a recorded choice: the Engagement page shows a section's form only when asked, behind **Write a message** (#2004), with a **Cancel** that hands focus back to it (#2103), and a Message may carry an image or PDF (GLOSSARY.md, Message). The reasoning is on Maya Okonkwo's map, [MO-G16](solo-birth-doula.md#gaps-found) ([#2197](https://github.com/markgoho/doula-cloud/issues/2197)). Push-triggered fetch wakes the Client's device (ADR-0002); Priya must not treat it as a substitute for a phone call in labor.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **8.1** — Send a message (`POST .../messages`).
- **8.2** — Receive the reply in the same continuous thread.

> **Nadia crossing.** Stages 7 and 8 are where an Engagement that ends in loss lands on the Doula's side: the Visit that becomes a bereavement visit, and the message thread that must not carry on as if nothing changed. The method standard says to walk Nadia Haddad's journey first where it overlaps another. [Her map](loss-client.md) now exists and **these stages are unchanged**: her stage 7 records the bereavement Visit against **PR-G6**, **MO-G1** and **MO-G2**, already owned here, and the thread with no way to mark that its subject has changed is hers to own (**NH-G7**). Nothing she needs contradicts a step here.

## Permission boundary

**Not a stage.** Priya would never type these URLs, so none of this belongs in the numbered interaction layer — a journey map records what the Persona does. It is a test matrix, kept on this map because this is where the finding was made, and `docs/test-plans/employed-doula.md` will derive from it directly.

No owner-only route has a `+page.ts` load guard — every one is a `+page.svelte` that renders, fetches on mount, and shows whatever the API returns. So the page always appears; what differs is whether the API hands over data.

- **PR-B1** — `/practices/[practiceId]/staff` — the `GET` requires Owner (`staffauth/staff.go:25`). **Expected to hold**: the page renders its heading, then shows an error instead of the roster.
- **PR-B2** — `/practices/[practiceId]/invite` — the form renders; only `POST .../invitations` requires Owner. **Expected to hold on submit.**
- **PR-B3** — `/practices/[practiceId]/settings/plan-templates` — **expected to fail on read**. `GET` is ungated (`plans/template.go:81`); only `PUT` checks Owner (line 130). She sees the Practice's real template definitions.
- **PR-B4** — `/practices/[practiceId]/settings/contract-template` — same shape, same expected failure (`contracts/template.go:31` ungated, `:75` gated).
- **PR-B5** — `/practices/[practiceId]/settings/payments` — `POST .../connect` requires Owner. **Expected to hold.**
- **PR-B6** — `/practices/[practiceId]/billing` — **expected to fail on read**: the balance and ledger take any Staff member (`billing/balance.go:86`). Buying credits is correctly refused (`billing/purchase.go:33`).

The pattern to test for: protection sits on the write endpoint, not the read. On three of these six she is shown data she has no right to see, then refused at the button.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| PR-G1 | 4 | Interaction | She sees every Client in the Practice. The list handler is Practice-scoped by design ("v1 has no restricted-visibility model"), so her scope requirement fails outright. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| PR-G2 | 5 | Both | She can read any Engagement's Contract amount and Invoice history, for any Client. Read paths carry no role checks. [ADR-0006](../adr/0006-read-follows-the-role.md) settles the rule: an **employed** Doula reads a Contract's scope but not its money, so the Contract read must be able to return one without the other. The write side is **PR-G8**. | [#277](https://github.com/markgoho/doula-cloud/issues/277) |
| PR-G3 | 2 | Interaction | **The `doula` role is read in exactly one package, and nowhere else.** Narrowed by the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237), which ran a 21-endpoint battery at `roles = '{}'` and again at `['doula']` and found a single behavioral difference: `POST .../visits` goes `403` -> `201`. `api/internal/visit/roles.go:40` gates the caller and `:57` gates a reassignment target; no other Go package and no front-end file reads the value. So the role is not decorative — it gates the Visit, the one act this journey is named for — but it gates nothing else, which is what **PR-G8** is the sharp edge of. | [#278](https://github.com/markgoho/doula-cloud/issues/278) |
| PR-G4 | Permission boundary | Interaction | No route has a load-time guard. The Plan Template, Contract Template, and Billing screens gate on write only (`GET` is ungated on all three), so she reaches the page and reads real Practice data. [ADR-0006](../adr/0006-read-follows-the-role.md) puts the refusal on the read endpoint, not the guard, and rules the Templates readable by any Staff role — so of these three, only Billing is a refusal for her. The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) corrected the second half: she is **not** always refused at the save button. Both Templates hand her every editing control and answer `403` on **Save**, but Billing renders **Buy credits** already `disabled` (`billing/+page.svelte:84`, `disabled={isPurchasing \|\| !isOwner}`) and Payments renders no Connect button at all, printing `Ask a Practice Owner to connect Stripe.` instead. So a client-side role gate does exist — on exactly those two controls and on nothing else in the product. | [#279](https://github.com/markgoho/doula-cloud/issues/279) |
| PR-G5 | 6 | Experience | The Birth Plan is a section partway down a single-page Engagement view, with no deep link and no phone-first path, at the exact moment she is standing in a corridor. Measured on a Pixel 7 by the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237): the page is 1755 CSS px tall and the **Birth Plan** heading sits 0.83 screens down, so the burial is milder than this gap first claimed — and everything else holds. The only `id` on the page is `svelte-announcer`, there is no `<details>` anywhere, no stylesheet on that page carries an `@media print` rule, and there is no print, export or share control. The page also renders **zero** `<a>` elements, so there is no way off it and no way back to Clients; she arrives by a remembered URL or not at all (**PR-G9**). | [#280](https://github.com/markgoho/doula-cloud/issues/280) |
| PR-G6 | 7 | Experience | A Visit carries no type, so the three kinds she distinguishes — prenatal, birth, postpartum — are one undifferentiated row. Dateless and note-less Visits are **MO-G1** and **MO-G2**. | [#281](https://github.com/markgoho/doula-cloud/issues/281) |
| PR-G7 | 1 | Experience | Her first impression is a raw URL pasted into a text message, which she cannot verify is genuine. This is the experience half of **RA-G1**; the missing email itself is filed there. The second walk ([#2176](https://github.com/markgoho/doula-cloud/issues/2176)) received the email: it says she has been invited to "a practice on DoulaCloud" and names neither the Practice nor who invited her, by decision (ADR-0009 and ADR-0011, recorded on [#339](https://github.com/markgoho/doula-cloud/issues/339)). |  |
| PR-G8 | 5 | Interaction | **A Doula can write a Contract's money, not merely read it.** Walked in [#237](https://github.com/markgoho/doula-cloud/issues/237): Priya, holding `doula` and nothing else, set `price` to `$99` on a draft Contract (`PUT .../contract`, `200`) and then **sent it to the Client** (`POST .../contract/send`, `200`); `POST .../contract/invoices` answered `200 {"connectRequired":true}` — the Stripe gate, not a role refusal — so on a connected Practice she raises the Invoice too. Every Contract route in `api/main.go:200-215` is mounted behind `staffauth.Middleware` alone; the only refusals are about Contract **status** (`409 contract is no longer a draft`, `409 contract is not signed`), never about who is asking. [ADR-0006](../adr/0006-read-follows-the-role.md) says an employed Doula may not read a Contract's money; today she can set it and bill it. **Settled by amending the rule, not the product**: [ADR-0008](../adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md) supersedes ADR-0006's table and gives an employed Doula a Contract's money and the right to raise an Invoice on every Engagement at the Practice ([#282](https://github.com/markgoho/doula-cloud/issues/282)); nobody sets the price any more, since it comes from the Practice's rate (#967). The second walk found the price refused to every caller (`400`) and the rest hers. | [#282](https://github.com/markgoho/doula-cloud/issues/282) |
| PR-G9 | 6 | Both | **The product's front door is the SvelteKit scaffold.** `app/src/routes/+page.svelte` is the unmodified template — `Welcome to SvelteKit` and a link to the framework's documentation. Her moment of truth is the one step in this whole effort that is timed *from a cold start*, and the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) found that a cold start reaches nothing: `/` is a dead end, `/login` redirects a signed-in user nowhere useful, and the Engagement page has no links, so the only route to this Client's Birth Plan is a Practice URL she has memorized or bookmarked. Measured from the Practice landing instead, the Birth Plan is 2 clicks and ~1.7 s away — the mechanics are fine; there is simply no way in. App-wide, filed here because this is the only journey that starts cold. | [#283](https://github.com/markgoho/doula-cloud/issues/283) |
| PR-G10 | 6 | Interaction | **Closed. Walked 2026-11-01 ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) on an emulated Pixel 7: the Engagement's first screen 3 (**Message the Client**, **Jump to**, the summary card), with **Birth Plan** in **Jump to** at y=437; the Birth Plan section 2 and the Birth Plan page 2; yes. At 320x640 the **Birth Plan** link sits at y=565, still on the first screen.** Decisions closed by a budget change. **On a phone the Birth Plan is five screens down, and reading it shows four decisions.** `**Decisions**: 1 (Hick's Law, Miller's Law)` — measured 4 on the Engagement page's first screen on a Pixel 7 (**Message the Client** and three **Change**), none of them the Birth Plan, and 2 on the Birth Plan page (**Print**, **Download Birth Plan (PDF)**). The page is 7955 CSS px tall, the **Birth Plan** heading 4.9 screens down, and **On this page** starts below the first screen. Second walk, 2026-10-28. Built on #2257: on a narrow page the **Jump to** list sits under the Client's name and above the summary card, at y=339 on a Pixel 7 (was 1053) and y=415 at 320px, so **Birth Plan** is on the first screen and jumps there in 14 ms. Every control on the reading screens is on a founder record, so the Decisions budget is 3, with the reason in stage 6's Pain points: the first screen 3 (**Message the Client**, **Jump to** as one list, the summary card as one group of rows), the Birth Plan section 2 (**Change the Birth Plan**, **Print for the hospital**), the Birth Plan page 2. | [#2257](https://github.com/markgoho/doula-cloud/issues/2257) |
| PR-G11 | 6 | Interaction | **Closed. Walked 2026-11-01 ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) on an emulated Pixel 7, no throttling: Camille Boyd's Engagement from the Clients list settled whole in 257, 289 and 295 ms over three warm presses, the body first changing in 77 to 100 ms; yes. The blind walk read 302 ms and 200 ms on the same phone, and 851 ms at 390x844 on the route's first visit of the session.** **On a phone the Engagement page takes up to 0.8 s to settle.** `**Time**: 400 ms, 100 ms (Doherty Threshold)` — measured 501 to 784 ms from the Clients list on an emulated Pixel 7, though the Birth Plan section is drawn in about 230 ms. Sign-in on the phone, 1014 ms, is RA-G16's. Second walk, 2026-10-28. Built on #2258: the sections read at once, not one after another, and each holds its place and says it is loading until its own read lands, as the summary card does (#2251). On the fix's own stack, warm, from the Clients list on an emulated Pixel 7 with no throttling, the whole page settled in 203 to 223 ms (was 473 to 493 ms on the same stack before) and the Birth Plan's answers drew in 106 to 117 ms. The budget is unchanged. | [#2258](https://github.com/markgoho/doula-cloud/issues/2258) |
| PR-G12 | 6 | Both | **Closed. Walked 2026-11-01 ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) on an emulated Pixel 7, in a fresh context that had never signed in: the saved Birth Plan address went to `/login?returnTo=` that address, with no "we signed you out", and **Sign in** landed on "Camille Boyd's Birth Plan"; yes. The redirect read the same at 320x640, with no sideways scroll.** **A saved Birth Plan address does not survive sign-in.** `**Cold**: yes (Paradox of the Active User)` — measured no. The Birth Plan's own address, opened signed out on a new phone, goes to `/login?sessionEnded=true`, which tells a device that never signed in "we signed you out", and **Sign in** lands on the Overview, not the plan. Second walk, 2026-10-28. A Staff address refused for want of a session now goes to `/login?returnTo=` that address, and **Sign in** (or a session she already holds) lands there, as GOV.UK returns a person to the page asked for (ADR-0021). The login screen follows only a path on this app's own origin, not the login screen itself, and otherwise lands her as before. "For your security, we signed you out." shows only when the page she was on had been talking to the service with a session; a device that never signed in, and a refusal after her own **Sign out**, say nothing. A session that ends mid-use still says so. Measured on the fix's own stack on 2026-10-31, on an emulated Pixel 7. | [#2259](https://github.com/markgoho/doula-cloud/issues/2259) |

Also hit here, filed on their owning maps: **RA-G1** (no invite email), **RA-G2** (no role UI), **RA-G4** (no Doula on an Engagement — which is why she cannot tell which Clients are hers), **MO-G1** and **MO-G2** (dateless, note-less Visits), **DW-G4** (Billing readable by any Staff member), **DW-G14** (a plan's answers read only through its editor or the print page; closed: built, and walked 2026-11-01, [#2264](https://github.com/markgoho/doula-cloud/issues/2264): Camille Boyd's Birth Plan answers read in its section on the Engagement, with no field, and Lena Vasquez's half read the same on Adaeze Nwosu's).
