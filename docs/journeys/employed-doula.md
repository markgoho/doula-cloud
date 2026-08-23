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

> **Walked ([#237](https://github.com/markgoho/doula-cloud/issues/237)), and it
> fails somewhere else than this map expected.** Once she is on the Engagement
> page the Birth Plan is 0.83 screens down and reads correctly in about 1.7 s on
> a Pixel 7 — the burial is real but mild. What fails is *arriving*: the app's
> front door is the SvelteKit scaffold (**PR-G9**) and the Engagement page has no
> links on it, so from a cold start there is no path to this screen at all, only
> a URL she has kept. The moment of truth stands where this map put it; the
> reason it fails has moved from the scroll to the way in.

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
  Staff, Plan Templates, Contract Template) are hidden by
  `{#if roles.includes('owner')}`. **Clients, Billing and Payments remain** —
  Payments is outside the owner block (RA-G9, corrected by the walk of
  [#235](https://github.com/markgoho/doula-cloud/issues/235)), and Billing is the
  Practice's credit spending (DW-G4).

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
**Pain points**: the Birth Plan is a section partway down the page, on a phone,
in a corridor, under time pressure. There is no deep link to it, no collapse of
the sections she does not need, and no way to hand it to hospital staff from her
side — the print stylesheet is on the Client's portal view. The walk of
[#237](https://github.com/markgoho/doula-cloud/issues/237) confirmed all three
and added the one that costs her most: there is no way to *reach* the page
without a remembered URL (**PR-G9**).

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

> **Nadia crossing.** Stages 7 and 8 are where an Engagement that ends in loss
> lands on the Doula's side: the Visit that becomes a bereavement visit, and the
> message thread that must not carry on as if nothing changed. The method standard
> says to walk Nadia Haddad's journey first where it overlaps another.
> [Her map](loss-client.md) now exists and **these stages are unchanged**: her
> stage 7 records the bereavement Visit against **PR-G6**, **MO-G1** and
> **MO-G2**, already owned here, and the thread with no way to mark that its
> subject has changed is hers to own (**NH-G7**). Nothing she needs contradicts a
> step here.

## Permission boundary

**Not a stage.** Priya would never type these URLs, so none of this belongs in the
numbered interaction layer — a journey map records what the Persona does. It is a
test matrix, kept on this map because this is where the finding was made, and
`docs/test-plans/employed-doula.md` will derive from it directly.

No owner-only route has a `+page.ts` load guard — every one is a `+page.svelte`
that renders, fetches on mount, and shows whatever the API returns. So the page
always appears; what differs is whether the API hands over data.

- **PR-B1** — `/practices/[practiceId]/staff` — the `GET` requires Owner
  (`staffauth/staff.go:25`). **Expected to hold**: the page renders its heading,
  then shows an error instead of the roster.
- **PR-B2** — `/practices/[practiceId]/invite` — the form renders; only
  `POST .../invitations` requires Owner. **Expected to hold on submit.**
- **PR-B3** — `/practices/[practiceId]/settings/plan-templates` — **expected to
  fail on read**. `GET` is ungated (`plans/template.go:81`); only `PUT` checks
  Owner (line 130). She sees the Practice's real template definitions.
- **PR-B4** — `/practices/[practiceId]/settings/contract-template` — same shape,
  same expected failure (`contracts/template.go:31` ungated, `:75` gated).
- **PR-B5** — `/practices/[practiceId]/settings/payments` — `POST .../connect`
  requires Owner. **Expected to hold.**
- **PR-B6** — `/practices/[practiceId]/billing` — **expected to fail on read**: the
  balance and ledger take any Staff member (`billing/balance.go:86`). Buying
  credits is correctly refused (`billing/purchase.go:33`).

The pattern to test for: protection sits on the write endpoint, not the read. On
three of these six she is shown data she has no right to see, then refused at the
button.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| PR-G1 | 4 | Interaction | She sees every Client in the Practice. The list handler is Practice-scoped by design ("v1 has no restricted-visibility model"), so her scope requirement fails outright. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| PR-G2 | 5 | Both | She can read any Engagement's Contract amount and Invoice history, for any Client. Read paths carry no role checks. [ADR-0006](../adr/0006-read-follows-the-role.md) settles the rule: an **employed** Doula reads a Contract's scope but not its money, so the Contract read must be able to return one without the other. The write side is **PR-G8**. | [#277](https://github.com/markgoho/doula-cloud/issues/277) |
| PR-G3 | 2 | Interaction | **The `doula` role is read in exactly one package, and nowhere else.** Narrowed by the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237), which ran a 21-endpoint battery at `roles = '{}'` and again at `['doula']` and found a single behavioural difference: `POST .../visits` goes `403` -> `201`. `api/internal/visit/roles.go:40` gates the caller and `:57` gates a reassignment target; no other Go package and no front-end file reads the value. So the role is not decorative — it gates the Visit, the one act this journey is named for — but it gates nothing else, which is what **PR-G8** is the sharp edge of. | [#278](https://github.com/markgoho/doula-cloud/issues/278) |
| PR-G4 | Permission boundary | Interaction | No route has a load-time guard. The Plan Template, Contract Template, and Billing screens gate on write only (`GET` is ungated on all three), so she reaches the page and reads real Practice data. [ADR-0006](../adr/0006-read-follows-the-role.md) puts the refusal on the read endpoint, not the guard, and rules the Templates readable by any Staff role — so of these three, only Billing is a refusal for her. The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) corrected the second half: she is **not** always refused at the save button. Both Templates hand her every editing control and answer `403` on **Save**, but Billing renders **Buy credits** already `disabled` (`billing/+page.svelte:84`, `disabled={isPurchasing \|\| !isOwner}`) and Payments renders no Connect button at all, printing `Ask a Practice Owner to connect Stripe.` instead. So a client-side role gate does exist — on exactly those two controls and on nothing else in the product. | [#279](https://github.com/markgoho/doula-cloud/issues/279) |
| PR-G5 | 6 | Experience | The Birth Plan is a section partway down a single-page Engagement view, with no deep link and no phone-first path, at the exact moment she is standing in a corridor. Measured on a Pixel 7 by the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237): the page is 1755 CSS px tall and the **Birth Plan** heading sits 0.83 screens down, so the burial is milder than this gap first claimed — and everything else holds. The only `id` on the page is `svelte-announcer`, there is no `<details>` anywhere, no stylesheet on that page carries an `@media print` rule, and there is no print, export or share control. The page also renders **zero** `<a>` elements, so there is no way off it and no way back to Clients; she arrives by a remembered URL or not at all (**PR-G9**). | [#280](https://github.com/markgoho/doula-cloud/issues/280) |
| PR-G6 | 7 | Experience | A Visit carries no type, so the three kinds she distinguishes — prenatal, birth, postpartum — are one undifferentiated row. Dateless and note-less Visits are **MO-G1** and **MO-G2**. | [#281](https://github.com/markgoho/doula-cloud/issues/281) |
| PR-G7 | 1 | Experience | Her first impression is a raw URL pasted into a text message, which she cannot verify is genuine. This is the experience half of **RA-G1**; the missing email itself is filed there. |  |

| PR-G8 | 5 | Interaction | **A Doula can write a Contract's money, not merely read it.** Walked in [#237](https://github.com/markgoho/doula-cloud/issues/237): Priya, holding `doula` and nothing else, set `price` to `$99` on a draft Contract (`PUT .../contract`, `200`) and then **sent it to the Client** (`POST .../contract/send`, `200`); `POST .../contract/invoices` answered `200 {"connectRequired":true}` — the Stripe gate, not a role refusal — so on a connected Practice she raises the Invoice too. Every Contract route in `api/main.go:200-215` is mounted behind `staffauth.Middleware` alone; the only refusals are about Contract **status** (`409 contract is no longer a draft`, `409 contract is not signed`), never about who is asking. [ADR-0006](../adr/0006-read-follows-the-role.md) says an employed Doula may not read a Contract's money; today she can set it and bill it. | [#282](https://github.com/markgoho/doula-cloud/issues/282) |
| PR-G9 | 6 | Both | **The product's front door is the SvelteKit scaffold.** `app/src/routes/+page.svelte` is the unmodified template — `Welcome to SvelteKit` and a link to the framework's documentation. Her moment of truth is the one step in this whole effort that is timed *from a cold start*, and the walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) found that a cold start reaches nothing: `/` is a dead end, `/login` redirects a signed-in user nowhere useful, and the Engagement page has no links, so the only route to this Client's Birth Plan is a Practice URL she has memorised or bookmarked. Measured from the Practice landing instead, the Birth Plan is 2 clicks and ~1.7 s away — the mechanics are fine; there is simply no way in. App-wide, filed here because this is the only journey that starts cold. | [#283](https://github.com/markgoho/doula-cloud/issues/283) |

Also hit here, filed on their owning maps: **RA-G1** (no invite email),
**RA-G2** (no role UI), **RA-G4** (no Doula on an Engagement — which is why she
cannot tell which Clients are hers), **MO-G1** and **MO-G2** (dateless, note-less
Visits), **DW-G4** (Billing readable by any Staff member).
