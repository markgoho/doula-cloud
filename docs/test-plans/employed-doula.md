# Priya Raman — test plan

- **Journey**: [employed-doula.md](../journeys/employed-doula.md)
- **Persona**: [employed-doula.md](../personas/employed-doula.md)
- **A pass means**: an active membership holding the Doula role, an Engagement
  assigned to her opened, a Visit logged, messages exchanged — **and she never saw
  a screen she had no right to see.**

She is the negative-permission Persona, so half this plan is the
[Permission boundary](#permission-boundary) matrix, which is tester work rather
than anything Priya would do.

## Preconditions

- A Practice with an Owner, **two or more** Clients, at least one of them created
  by a different Staff member — otherwise stage 4 cannot fail the way it should.
- One Engagement carrying a filled Birth Plan and a Contract with an amount on it.
- Priya invited as a `doula` and accepted. The role rides the Invitation now
  (#316), so there is no follow-up call to make.
- A phone, or a phone-sized viewport, for stage 6. It is the moment of truth and it
  is device-specific.

## Steps

### Stage 1 — Accept the invitation on the phone

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link on the device the text arrived on | The accept form renders on a small viewport | `manual` |
| 1.2 | Set email and password, press **Accept invite** | Membership created with `roles = '{}'` | `manual` |

### Stage 2 — Receive the Doula role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Have the Owner set `doula` from a screen | **Edit membership** on Priya's row of `/practices/[practiceId]/staff` sets roles and employment type together and saves (#316) | `manual` |
| 2.1-b | Compare every later step with and without the role | **Exactly one observable difference.** A 21-endpoint battery run at `roles = '{}'` and again at `['doula']` differs only at `POST .../visits` (`403` -> `201`), plus `GET .../session` echoing the value. `visit/roles.go:40` gates the caller and `:57` gates a reassignment target; nothing else in Go and nothing in the front end reads it. The role is not decorative, but it gates only the Visit ([PR-G3](https://github.com/markgoho/doula-cloud/issues/278), narrowed) | `manual` |

### Stage 3 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Sign in at `/login` | `POST /api/session` succeeds and lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 3.2 | Choose Rooted Birth Collective | **There is nothing to choose.** One membership redirects straight to `/practices/[practiceId]`; no picker renders. Same as Renata's 1.2 and Dee's 1.3, and unwalkable for anyone until [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) | `manual` |
| 3.3 | Read the tiles | Invite, Staff, Plan Templates and Contract Template are hidden by `{#if roles.includes('owner')}`. **Clients, Billing and Payments remain** — Payments is outside the owner block ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)), as #235's walk found by landing a zero-role member on it | `manual` |

### Stage 4 — Find her Clients

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Every Client in the Practice**, including other doulas'. The handler is Practice-scoped by design: "v1 has no restricted-visibility model" ([PR-G1](https://github.com/markgoho/doula-cloud/issues/225)) | `manual` |
| 4.2-a | Look for a column saying which rows are hers | None — Engagements carry no Doula | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) |

### Stage 5 — Open the Engagement

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open an Engagement that is **not** hers | It opens. No read path is role-checked | `manual` |
| 5.2 | Read the Contract and Invoices sections | The merge values **including the amount** and the Invoice history render; the **prose does not** — the staff Engagement page shows `Status:` and the six filled merge-field inputs only, and the prose lives on the Contract Template screen and the Client's portal view. Per [ADR-0006](../adr/0006-read-follows-the-role.md) an employed Doula reads a Contract's scope but **not** its money ([PR-G2](https://github.com/markgoho/doula-cloud/issues/277)) | `manual` |

| 5.2-a | Set the Price on a draft Contract, send it, and raise an Invoice | **All three succeed.** Only Contract *status* refuses (`409` on a non-draft), never the role; the Invoice reaches the Stripe gate (`connectRequired`), not a refusal ([PR-G8](https://github.com/markgoho/doula-cloud/issues/282)) | `manual` |

### Stage 6 — Read the Birth Plan (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | On the phone, reach this Client's Birth Plan from a cold start, timed | It is a section partway down one long Engagement page holding Visits, Care Plan, Contract, Invoices and Messages | `manual` |
| 6.2 | Read the filled values | `GET .../plans/birth_plan` renders the Plan Instance's snapshot | `manual` |
| 6.2-a | Deep-link straight to it, collapse what she does not need, or print it for hospital staff from her side | None of the three exist; the print stylesheet lives on the Client's portal view | `missing-feature (PR-G5)` [#280](https://github.com/markgoho/doula-cloud/issues/280) |

### Stage 7 — Log a Visit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Add a Visit | A row of her name and a creation timestamp | `manual` |
| 7.1-a | Record when the Visit was | No date or time anywhere | `missing-feature (MO-G1)` [#250](https://github.com/markgoho/doula-cloud/issues/250) |
| 7.1-b | Record which of the three kinds it was — prenatal, birth, postpartum | A Visit carries no type | `missing-feature (PR-G6)` [#281](https://github.com/markgoho/doula-cloud/issues/281) |
| 7.1-c | Record what was covered, for "what was I told last time" | A Visit carries no notes | `missing-feature (MO-G2)` [#251](https://github.com/markgoho/doula-cloud/issues/251) |

### Stage 8 — Message the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Send a message | `POST .../messages` appends to the one Engagement thread | `manual` |
| 8.2 | Have the Client reply | One continuous thread, in order, immutable | `manual` |
| 8.2-a | With the Client's thread open, deliver a push event | The tab refetches and shows the message; the push itself carries no content (ADR-0002) | `automated (push-notification.e2e.ts)` |

**Nadia crossing, settled.** Stages 7 and 8 are where an Engagement ending in loss
lands on the Doula's side — a bereavement Visit, and a thread that must not carry
on unchanged. [Her plan](loss-client.md) now exists and **neither stage changes**:
her 7.1 walks the bereavement Visit against **PR-G6**, **MO-G1** and **MO-G2**,
already given steps here, and the thread with no way to mark that its subject has
changed is hers to own (**NH-G7**). No step id here moves.

## Permission boundary

**Not stages.** Priya would never type these URLs. Each is walked as Priya, signed
in, holding `doula` and not `owner`. No owner-only route has a `+page.ts` load
guard, so the page always renders; what differs is whether the API hands over data
(PR-G4).

| Step | Route | Expected result | Mark |
| --- | --- | --- | --- |
| PR-B1 | `/practices/[practiceId]/staff` | **Holds.** The heading renders, then an error instead of the roster — the `GET` requires Owner | `manual` |
| PR-B2 | `/practices/[practiceId]/invite` | **Holds on submit.** The form renders; `POST .../invitations` refuses | `manual` |
| PR-B3 | `/practices/[practiceId]/settings/plan-templates` | **Reads through.** `GET` is ungated, `PUT` is Owner. Under ADR-0006 Templates are open to every Staff role, so this is a correct read and a refusal only at **Save** | `manual` |
| PR-B4 | `/practices/[practiceId]/settings/contract-template` | Same shape, same verdict as PR-B3 | `manual` |
| PR-B5 | `/practices/[practiceId]/settings/payments` | **Holds, and not at the endpoint.** The screen prints the Practice's Stripe status to her (`Not connected` — [RA-G9](https://github.com/markgoho/doula-cloud/issues/267)), then renders **no Connect button at all**, only `Ask a Practice Owner to connect Stripe.` `POST .../connect` also refuses on a direct call | `manual` |
| PR-B6 | `/practices/[practiceId]/billing` | **Fails.** The balance and ledger take any Staff member, so she reads the Practice's spending; buying is correctly refused ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272)) — **Buy credits** renders `disabled` for a non-owner (`billing/+page.svelte:84`) and the endpoint `403`s on a direct call | `manual` |

The pattern under test: protection sits on the write endpoint, never on the read.
No spec in the suite asserts a refusal by role at all, so every row here is new
ground.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 2 |
| `manual` | 22 (six of them the permission boundary; 5.2-a added by the walk) |
| `missing-feature` | 6 ([RA-G2](https://github.com/markgoho/doula-cloud/issues/261), [RA-G4](https://github.com/markgoho/doula-cloud/issues/225), [PR-G5](https://github.com/markgoho/doula-cloud/issues/280), [PR-G6](https://github.com/markgoho/doula-cloud/issues/281), [MO-G1](https://github.com/markgoho/doula-cloud/issues/250), [MO-G2](https://github.com/markgoho/doula-cloud/issues/251)) |

PR-G1, PR-G2, PR-G3, PR-G4 and PR-G8 are observed inside walkable steps (4.2, 5.2,
5.2-a, 2.1-b, PR-B3 to PR-B6) rather than given steps of their own: the step can be
performed, and what it hands back is the finding. **PR-G9** is observed at 6.1, in
what happens *before* the step rather than in the step itself.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 3.1 | `staff-login.e2e.ts` | pass |
| 8.2-a | `push-notification.e2e.ts` | pass |

**2 automated steps: all pass.**

### 2026-08-23 — manual walk ([#237](https://github.com/markgoho/doula-cloud/issues/237))

`bun run dev:full` in `app/`, walked as Priya Raman: stages 1 and 6 on an emulated
Pixel 7 (412x839), everything else at 1280x900, with separate contexts for Renata
(Owner) and for Marisol Vega (the Client who replies). This plan's two `automated`
steps were **not** re-run.

Preconditions built as the plan allows, and no further: `POST /api/staff/signup` for
`Rooted Birth Collective` (Renata, whom signup grants all three roles), the Client
**Marisol Vega** created by Renata and the Client **Tabitha Nunes** created by a
second Staff member, **Jo Mercer**, so stage 4 can fail the way it should; a filled
Care Plan and Birth Plan and a `sent` Contract priced `$1,800` on Marisol's
Engagement; and her portal invite accepted, so stage 8 has a reply side. Priya's own
invitation was only *sent* — accepting it **is** steps 1.1 and 1.2, and the
`PATCH .../staff/{staffId}/roles` **is** step 2.1-a, so neither was provisioned away.

**On "compare every later step with and without the role" (2.1-b).** Walking all 28
steps twice would say nothing 2.1-b does not, so the second pass is a fixed battery
rather than the whole plan: **21 endpoints**, captured once at `roles = '{}'` — the
state acceptance actually leaves her in — and again at `['doula']`, then diffed with
ids and timestamps normalised away. The session's call, recorded here rather than
asked, following Dee's precedent.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | On a 412px viewport: `Accept your Staff invite`, an Email box, a Password box, an **Account mode** radio pair, and one **Accept invite** button. `scrollWidth` equals `clientWidth`, so nothing overflows sideways. The tab title is empty ([DW-G8](https://github.com/markgoho/doula-cloud/issues/276)) |
| 1.2 | `manual` | as expected, plus one fact | `200 POST /api/staff/accept-invite`, and `GET .../staff` then read `{"name":"Priya Raman","roles":[]}` — zero roles, as claimed. It also lands her **straight on** `/practices/{id}`, greeting her `Welcome to Rooted Birth Collective` with three tiles, which pre-empts 3.2 below |
| 2.1 | `missing-feature (RA-G2)` [#261](https://github.com/markgoho/doula-cloud/issues/261) | as expected | Confirmed unwalkable from the Owner's side: the Staff screen prints a **Roles** column (`no roles yet`, `doula`, `owner, office_manager, doula` — the raw enum, [RA-G3](https://github.com/markgoho/doula-cloud/issues/262)) and the only control under **Actions** is **End sessions everywhere**. Nothing sets a role |
| 2.1-a | `manual` | as expected | `PATCH .../staff/{staffId}/roles` with `["doula"]` -> `200 {"roles":["doula"]}`. The staff id came from `GET .../staff`, which no screen prints — the same out-of-band start Dee's 2.1-a and Renata's 3.3-a have |
| 2.1-b | `manual` | **falsified** | The battery diff is **two lines, one of them behaviour**: `GET .../session` echoes `roles:["doula"]`, and `POST .../visits` goes `403 only a Staff member with the Doula role can do that` -> `201`. Everything else — the Clients list, the Engagement, both plans, the Contract, Invoices, Billing, both Templates, Payments, and every refusal — is byte-identical. The role is read in **one package**: `visit/roles.go:40` for the caller and `:57` for a reassignment target (a target without `doula` is refused `400 staff member does not hold the Doula role at this practice`). No front-end file reads it. So `doula` is **not** decorative — it gates the Visit, the one act this journey is named for — and it gates nothing else. [PR-G3](https://github.com/markgoho/doula-cloud/issues/278) narrowed on the map, not deleted |
| 3.1 | `automated (staff-login.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| 3.2 | `manual` | **falsified** | There is nothing to choose. Signing in at `/login` went straight to `/practices/{id}`; no picker rendered. One membership, and [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) makes a second unreachable, so this step is unwalkable for anyone. Third plan to carry the same false claim, after Renata's 1.2 and Dee's 1.3 |
| 3.3 | `manual` | as expected | Exactly three links: `Clients`, `Billing`, `Payments`. Invite, Staff, Plan Templates and Contract Template are gone. Payments is outside the owner block — [RA-G9](https://github.com/markgoho/doula-cloud/issues/267), now confirmed from a third membership |
| 4.1 | `manual` | as expected | The list renders for a non-owner, `200 GET .../clients` |
| 4.2 | `manual` | as expected | **Two rows: `Marisol Vega` and `Tabitha Nunes`** — she reads the Client Jo Mercer created, which is not hers and which she has no Engagement with. [PR-G1](https://github.com/markgoho/doula-cloud/issues/225) confirmed against a Practice built to test it |
| 4.2-a | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. The table is two columns, `Name` and `Status`. Nothing names a Doula, because no Engagement carries one |
| 5.1 | `manual` | as expected | Marisol's Engagement — Renata's Client, not Priya's — opened on the first try. Seven `GET`s fired, all `200`: the Engagement, Visits, Messages, both plans, the Contract and the Invoices. No read path is role-checked |
| 5.2 | `manual` | **falsified in part** | The money is all there: `Price` reads `$1,800`, and `Scope of service`, both dates and both names are filled; the Invoices section reads `No Invoices yet.` beside an Amount box and **Create Invoice**. [PR-G2](https://github.com/markgoho/doula-cloud/issues/277) confirmed. What does **not** render is the **prose** — the Engagement page shows `Status: sent` and the six merge-field inputs only. The cell claimed prose; the prose is on the Contract Template screen and in the Client's portal |
| 5.2-a | `manual` | **new — and the worst result on this plan** | Priya, holding `doula` and nothing else, opened Tabitha's draft Contract, set `price` to `$99` and `scope_of_service` to `Priya wrote this` (`PUT .../contract` -> `200`), then **sent it to the Client** (`POST .../contract/send` -> `200 {"status":"sent"}`). `POST .../contract/invoices` answered `200 {"connectRequired":true}` — the Stripe gate, not a role refusal — so on a connected Practice she raises the Invoice too. The only refusals a Contract makes are about its own status (`409 contract is no longer a draft`, `409 contract is not signed`). New gap **[PR-G8](https://github.com/markgoho/doula-cloud/issues/282)** |
| 6.1 | `manual` | as expected in kind, and sharper in fact | **From a genuine cold start she reaches nothing**: the app's `/` is the unmodified SvelteKit scaffold — `Welcome to SvelteKit` and a link to the framework's docs (`app/src/routes/+page.svelte`) — and the Engagement page renders **zero** `<a>` elements, so there is no way in and no way back. New gap **[PR-G9](https://github.com/markgoho/doula-cloud/issues/283)**. Timed from the Practice landing instead, the URL she would have kept: **2 clicks and ~1.7 s** to the Birth Plan loaded and in view. The page is **1755 CSS px** tall at 412px wide and the **Birth Plan** heading sits at y=692, **0.83 screens** down, behind `Marisol Vega`, `Visits` and `Care Plan`. Milder burial than [PR-G5](https://github.com/markgoho/doula-cloud/issues/280) claimed, and it degrades as Visits accumulate, since Visits is the one section above it that grows |
| 6.2 | `manual` | as expected | `GET .../plans/birth_plan` (not `.../plans/birth`; cell corrected) renders the snapshot: `Hospital`, `Partner`/`Doula`/`Midwife` ticked, `Dim lights, my own playlist. No epidural unless I ask twice.`, photos consented. The one thing she came for is right there and correct |
| 6.2-a | `missing-feature (PR-G5)` [#280](https://github.com/markgoho/doula-cloud/issues/280) | as expected | Confirmed unwalkable, all three ways. The only `id` on the page is `svelte-announcer`, so there is nothing to deep-link to; there are **0** `<details>` elements, so nothing collapses; and no stylesheet loaded on that page carries an `@media print` rule, with no print, export or share control anywhere |
| 7.1 | `manual` | as expected | **Add a Visit** -> `201`, and the Visits table gained a row reading `Priya Raman`, `8/23/2026`. She could only do this because of 2.1-a |
| 7.1-a | `missing-feature (MO-G1)` [#250](https://github.com/markgoho/doula-cloud/issues/250) | as expected, and sharper | Confirmed unwalkable. There **is** a `Date` column and it prints `8/23/2026` — `created_at`, not when the Visit was, exactly as Tasha's walk found on the Visits screen. Nothing on the page takes a date |
| 7.1-b | `missing-feature (PR-G6)` [#281](https://github.com/markgoho/doula-cloud/issues/281) | as expected | Confirmed unwalkable. The Visits section's only input is `Reassign to Staff id`. No type, no prenatal/birth/postpartum anywhere |
| 7.1-c | `missing-feature (MO-G2)` [#251](https://github.com/markgoho/doula-cloud/issues/251) | as expected | Confirmed unwalkable. No notes field, so "what was I told last time" has nowhere to live |
| 8.1 | `manual` | as expected | `POST .../messages` -> `201`, appended to the one Engagement thread with her name, `(staff)`, and a timestamp |
| 8.2 | `manual` | as expected | Marisol replied from the portal and the message landed in the same thread, in order, labelled `(client)`. Immutable as claimed: no edit or delete control renders, and `api/main.go` mounts no `PUT`, `PATCH` or `DELETE` on messages |
| 8.2-a | `automated (push-notification.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| PR-B1 | `manual` | as expected | **Holds.** The heading `Staff` renders, then `only a Practice Owner can do that` where the roster would be. `403 GET .../staff` |
| PR-B2 | `manual` | as expected | **Holds on submit.** The form renders with **no API call at all** on load — name, email, **Send invite** — and the press answers `403 only a Practice Owner can do that`, printed on the screen |
| PR-B3 | `manual` | as expected | **Reads through.** `200 GET .../plan-templates/care_plan` hands her the Practice's five real Care Plan fields with every editing control — **Move up**, **Move down**, **Remove**, **Add field**, **Save**. `403 PUT` at **Save**. Under ADR-0006 the read is correct and only the refusal at Save matters |
| PR-B4 | `manual` | as expected | Same shape, same verdict. The Practice's real template prose and the whole six-token merge-field legend render; `403 PUT` at **Save** |
| PR-B5 | `manual` | **falsified in shape** | It holds, but not where the cell said. The screen hands her the Practice's Stripe status first — `Stripe Connect status: Not connected` ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) — and then renders **no Connect button at all**, only `Ask a Practice Owner to connect Stripe.` The direct `POST .../connect` also `403`s. So the refusal she meets is a hidden control, not a rejected press |
| PR-B6 | `manual` | as expected, with one correction | **Fails on read**: `Credit balance: 1` and the whole ledger, three rows, `signup_bonus +3` and two `consumption -1`. She reads what the Practice spends ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272)). Buying is refused twice over — **Buy credits** renders `disabled` for a non-owner (`billing/+page.svelte:84`), and the endpoint answers `403 only a Practice Owner can do that` on a direct call |

**22 `manual` steps walked (5.2-a added by the walk); 6 `missing-feature` steps
confirmed unwalkable; no `blocked` step on this plan.** Four expected results were
falsified — 2.1-b, 3.2, 5.2 and PR-B5 — and none was re-marked, because every one of
them is a performable step whose claim was simply wrong ([#235](https://github.com/markgoho/doula-cloud/issues/235)'s
precedent). Two gaps minted on the journey map, **PR-G8** and **PR-G9**, and **PR-G3**
narrowed rather than deleted. No `journey-gap` issue was filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and the walk made the failure
worse than the map argued.** Three of the four clauses hold: she has an active
membership holding the Doula role, she logged a Visit — and only the role let her —
and she exchanged messages with the Client in one clean thread. The Engagement she
opened is **not assigned to her**, because no Engagement is assigned to anyone
(RA-G4). The last clause fails hardest. The map's case was that she *sees* screens she
has no right to: the whole Practice's Client list, another Doula's Contract price, the
Practice's credit ledger, both Templates, the Stripe status. All of that is confirmed.
What the map did not claim, and the walk found, is that she can **act** on what she
should not even see — set the price on a Contract and send it to a Client
(**PR-G8**). Reading follows no role; writing follows only the Contract's own status.

**Her moment of truth stands where the map put it, and fails for a different
reason.** Once she is on the Engagement page the Birth Plan is 0.83 screens away and
reads correctly in under two seconds, which is not the corridor disaster the map
feared. Getting there is: the product's front door is the framework's demo page, the
Engagement page has no links on it, and there is no deep link, no collapse and no
print. The corridor problem is real; it is a problem of arrival, not of scrolling.

