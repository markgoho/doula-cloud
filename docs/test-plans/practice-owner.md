# Renata Alvarez — test plan

- **Journey**: [practice-owner.md](../journeys/practice-owner.md)
- **Persona**: [practice-owner.md](../personas/practice-owner.md)
- **A pass means**: a new Doula has accepted an invitation, holds the Doula role,
  and appears as the assigned Doula on a live Engagement; and one screen shows
  every Engagement in the Practice with its Contract and Invoice state.

## Preconditions

- A Practice with Renata as Owner, and at least two Clients with Engagements.
  Build it through Maya's stages 1 and 3, or provision it with
  `POST /api/staff/signup` plus `POST /api/practices/{id}/clients` as the specs do.
- A second Identity Platform account for the invitee, and a way to read the
  invitation token (it is printed on the invite screen; no email is sent).

## Steps

### Stage 1 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Sign in at `/login` | `POST /api/session` sets `__session`; the browser lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 1.2 | Choose Rooted Birth Collective from her memberships | **There is no choosing.** With one membership `decideLanding` redirects straight to `/practices/{id}` (`app/src/lib/landing.ts:24-26`); the `Choose a Practice` picker renders only for two or more, which [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) makes unreachable through the product | `manual` |
| 1.3 | Read the tiles | All seven render for her — but only **four** are owner-gated (Invite, Staff, Plan Templates, Contract Template). **Payments sits outside the gate** (`app/src/routes/practices/[practiceId]/+page.svelte:76-81`), so every member sees it, roles or none ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) | `manual` |

The membership picker (1.2) is exercised by no spec — every spec's Staff member
belongs to exactly one Practice, so the multi-membership path is untested here and
is Lena's normal case.

### Stage 2 — Invite a new Doula

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/practices/[practiceId]/invite` | The form renders for an Owner | `manual` |
| 2.2 | Enter a name and email, press **Send invite** | `POST /api/practices/{id}/invitations` succeeds and the screen **prints a link**; no email leaves the system | `manual` |
| 2.2-a | Check the invitee's inbox | Nothing arrives. No invitation email is sent by anything | `missing-feature (RA-G1)` [#260](https://github.com/markgoho/doula-cloud/issues/260) |
| 2.3 | Deliver the link out of band | The invitee receives a raw URL by text or Renata's own mail client, which they cannot verify is genuine | `manual` |
| 2.4 | Have the invitee accept at `/accept-invite` | `POST /api/staff/accept-invite` creates the membership with `roles = '{}'` — a zero-role member is the only possible outcome of inviting anyone | `manual` |
| 2.4-a | Look for a roles control on the invite form | The invitation carries no roles at all | `missing-feature (RA-G8)` [#266](https://github.com/markgoho/doula-cloud/issues/266) |

### Stage 3 — Set the new Doula's roles

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/staff` | The roster loads; the `GET` is owner-gated and passes for Renata | `manual` |
| 3.2 | Read the Roles column | Raw enum strings render — an Admin shows as `office_manager`, the word `CONTEXT.md` rules out | `manual` |
| 3.3 | Change a member's roles from the screen | No control exists. The only row action is **End sessions everywhere** | `missing-feature (RA-G2)` [#261](https://github.com/markgoho/doula-cloud/issues/261) |
| 3.3-a | Call `PATCH /api/practices/{id}/staff/{staffId}/roles` directly with `["doula"]` | Succeeds for an Owner. **This is the only way to build a roster**, and every later plan depends on it as fixture setup | `manual` |

### Stage 4 — Assign the Doula to Engagements

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open an Engagement from the Clients list | The single-page Engagement view renders | `automated (birth-plan.e2e.ts)` |
| 4.2 | Look for an assignment control | No field, no endpoint, no screen. An Engagement carries no Doula | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| 4.3 | Add a Visit naming the new Doula as `staffId` | **The Visit cannot name anyone.** `POST .../visits` takes no body and assigns the *caller* (`api/internal/visit/create.go:32,47`); handing it to a colleague is a second act, the **Reassign to Staff id** free-text box, which wants a UUID no screen prints ([RA-G10](https://github.com/markgoho/doula-cloud/issues/268)). Assignment exists at Visit level only, on a record with no date | `manual` |

### Stage 5 — Reassign when someone is sick

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | `PATCH .../visits/{visitId}` with a new `staffId` | The Visit's Staff member changes; nothing dated moves, because nothing is dated | `manual` |

### Stage 6 — See the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open `/practices/[practiceId]/clients` | The list renders and each Client links to their Engagement | `automated (birth-plan.e2e.ts)` |
| 6.1-a | Create a Client as a *second* Staff member, then reload as Renata | It appears: the handler returns every Client with an Engagement at the Practice "regardless of which Staff member created it". **This half of her requirement passes** | `manual` |
| 6.1-b | Read the columns | Name and Status only — and Status is `intake` on every row forever ([MO-G4](https://github.com/markgoho/doula-cloud/issues/253)) | `manual` |
| 6.2 | Learn each Engagement's Contract and Invoice state | Reachable only by opening every Engagement in turn | `manual` |
| 6.2-a | Look for a roll-up of Contract state, Invoice state, or covering Doula | There is none at any level above one Engagement | `missing-feature (RA-G6)` [#264](https://github.com/markgoho/doula-cloud/issues/264) |

### Stage 7 — See the money across all Staff

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open `/practices/[practiceId]/billing` | Credit balance and purchase ledger render — Doula Cloud's own billing, not Client money | `automated (billing.e2e.ts)` |
| 7.2 | Look for unpaid Client Invoices | No Practice-wide Invoice list and no unpaid view exist | `missing-feature (RA-G7)` [#265](https://github.com/markgoho/doula-cloud/issues/265) |

### Stage 8 — Coverage, at 2 a.m. (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Sign in on a phone | The practice screen renders on a small viewport | `manual` |
| 8.2 | Find who is free tonight | No availability, on-call, or coverage surface exists — and dateless Visits ([MO-G1](https://github.com/markgoho/doula-cloud/issues/250)) mean the data one would read does not exist either | `missing-feature (RA-G5)` [#263](https://github.com/markgoho/doula-cloud/issues/263) |

### Stage 9 — Edit Plan Templates for the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Add a field to the Birth Plan template and save | `PUT .../plan-templates/{planType}` succeeds for an Owner and persists | `automated (plan-templates.e2e.ts)` |
| 9.2 | Reopen an already-filled Birth Plan | Unchanged — the Plan Instance snapshots the field definitions at creation. **Passes** | `manual` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 5 |
| `manual` | 16 |
| `missing-feature` | 7 ([RA-G1](https://github.com/markgoho/doula-cloud/issues/260), [RA-G2](https://github.com/markgoho/doula-cloud/issues/261), [RA-G4](https://github.com/markgoho/doula-cloud/issues/225), [RA-G5](https://github.com/markgoho/doula-cloud/issues/263), [RA-G6](https://github.com/markgoho/doula-cloud/issues/264), [RA-G7](https://github.com/markgoho/doula-cloud/issues/265), [RA-G8](https://github.com/markgoho/doula-cloud/issues/266)) |

RA-G3 is observed at 3.2 rather than given a step: the screen renders, so the step
is walkable — what fails is the word it prints. **RA-G9** and **RA-G10** were minted
by the walk and are observed the same way, inside 1.3 and 4.3: both steps can be
performed, and what the product does when they are is the finding.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.1 | `staff-login.e2e.ts` | pass |
| 4.1 | `birth-plan.e2e.ts` | pass |
| 6.1 | `birth-plan.e2e.ts` | pass |
| 7.1 | `billing.e2e.ts` | pass |
| 9.1 | `plan-templates.e2e.ts` | pass |

**5 automated steps: all pass.**

### 2026-08-22 — manual walk ([#235](https://github.com/markgoho/doula-cloud/issues/235))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x900 as Renata
Alvarez, with a second context for the invitee and a 390x844 iPhone context for
stage 8. Preconditions built as the plan allows: `POST /api/staff/signup` for
`Rooted Birth Collective`, then two Clients through `POST .../clients` — two of
the three signup credits, leaving one for 6.1-a. The 5 `automated` steps were
**not** re-run. No step is `blocked`: this plan has none, and Stripe is never
reached.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.2 | `manual` | **falsified** | There is nothing to choose. One membership, so `decideLanding` returned `{type:'redirect'}` and the browser went straight to `/practices/{id}` (`app/src/lib/landing.ts:24-26`); the `Choose a Practice` heading never rendered. The picker needs two memberships, which [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) says a person cannot have — so the step as written is unwalkable *for anyone*, and the path belongs to Lena's walk ([#238](https://github.com/markgoho/doula-cloud/issues/238)) with its `practice_memberships` bypass |
| 1.3 | `manual` | **falsified** | All seven render for Renata — `Clients Billing Invite a Staff member Staff Plan Templates Contract Template Payments`, plus `Sign out` — but the claim that five are owner-only is wrong. **Four** sit inside `{#if roles.includes('owner')}`; **Payments is outside it** (`app/src/routes/practices/[practiceId]/+page.svelte:76-81`). Seen from the other side at 2.4: Jo Mercer, holding *zero* roles, landed on `Clients Billing Payments`. New gap **[RA-G9](https://github.com/markgoho/doula-cloud/issues/267)** on the journey map |
| 2.1 | `manual` | as expected | `Invite a Staff member`, two fields — `Their name`, `Their email` — and **Send invite**. Nothing else |
| 2.2 | `manual` | as expected | `201`, and the screen says it plainly: `Invited. There is no email sending yet, so share this link with them directly:` followed by `http://localhost:5173/accept-invite?token=<uuid>` in a `<code>` block |
| 2.2-a | `missing-feature (RA-G1)` [#260](https://github.com/markgoho/doula-cloud/issues/260) | as expected | Confirmed unwalkable. Nothing in `api/` or `app/src` can send mail — no SMTP client, no mail vendor SDK, no `net/mail` import. There is no inbox to check |
| 2.3 | `manual` | as expected | The link is the whole message. It names no Practice, no sender, and no person; Renata pastes a bare URL with a UUID in it and the new hire has nothing to check it against |
| 2.4 | `manual` | as expected | Jo Mercer created an account at `/accept-invite` (`I'm new here`), landed on `Welcome to Rooted Birth Collective`, and the roster showed `no roles yet`; `GET .../staff` returned `"roles":[]`. **What the zero-role state actually gets her** is three links — Clients, Billing, Payments — so before anyone has given her a role she reads the whole Client list, the credit ledger, and the Practice's Stripe state ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) |
| 2.4-a | `missing-feature (RA-G8)` [#266](https://github.com/markgoho/doula-cloud/issues/266) | as expected | Confirmed unwalkable. The form's only controls are `INPUT:text` and `INPUT:email`. No roles field, no checkbox, nothing role-shaped anywhere on the page |
| 3.1 | `manual` | as expected | `Staff`, a Name / Email / Roles / Actions table, two rows. Owner-gated as claimed (`staffauth/staff.go:25`): Jo's own `GET` on the same URL answered `403 only a Practice Owner can do that` |
| 3.2 | `manual` | as expected, plus one fact the plan does not name | Renata's row reads `owner, office_manager, doula` — raw enums, **[RA-G3](https://github.com/markgoho/doula-cloud/issues/262)** — and that is the second finding: `POST /api/staff/signup` grants all three roles at once (`staffauth/signup.go:152`), so the "Owner" every plan walks is also an Admin and a Doula. Jo's empty row printed `no roles yet`, the column's one non-raw word |
| 3.3 | `missing-feature (RA-G2)` [#261](https://github.com/markgoho/doula-cloud/issues/261) | as expected | Confirmed unwalkable. Zero editable controls inside the table (`table select, table input` → 0). The only row action, on both rows, is **End sessions everywhere** |
| 3.3-a | `manual` | as expected, and worse than it reads | `PATCH .../staff/{staffId}/roles` with `["doula"]` → `200 {"staffId":…,"roles":["doula"]}`, and the roster then showed `doula`. But **the staff id came from the API**: no screen in the product prints one, so the only way to build a roster starts with a `GET .../staff` in a terminal |
| 4.2 | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. Every control on the Engagement page enumerated: `Send portal invite`, `Add a Visit`, `Create Care Plan`, `Create Birth Plan`, `Create Draft Contract`, `Send` (message). The Engagement's own facts are `Status: intake` and `Created: 8/22/2026`. Nothing names a Doula |
| 4.3 | `manual` | **falsified** | The Visit cannot name anybody. **Add a Visit** sends `POST .../visits` with no body and the handler assigns the *caller* (`api/internal/visit/create.go:32,47`) — the row came back `Renata Alvarez`. Handing it to Jo took a second act, the **Reassign to Staff id** free-text box, into which Renata pastes a UUID no screen prints. New gap **[RA-G10](https://github.com/markgoho/doula-cloud/issues/268)** on the journey map. Two smaller facts fell out: the endpoint requires the **Doula** role (`visit/roles.go:40`), which Renata holds only because signup granted all three, so an Owner who is only an Owner cannot add a Visit at all; and the row's `Date` cell is `createdAt` (**[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**) |
| 5.1 | `manual` | as expected | `PATCH .../visits/{visitId}` with a new `staffId` → `200`, and the table's Staff cell changed `Jo Mercer` → `Renata Alvarez` on reload. Nothing dated moved, because the only date on the row is when it was created |
| 6.1-a | `manual` | as expected — **this half passes** | Jo, now holding `doula`, created `Priya Raman Client` → `201`. Renata reloaded `/clients` and all three rows were there, hers and Jo's alike, exactly as the handler comment promises |
| 6.1-b | `manual` | as expected | Two columns, `Name` and `Status`. `intake` on all three rows (**[MO-G4](https://github.com/markgoho/doula-cloud/issues/253)**). Nothing about a Contract, an Invoice, or who is covering whom |
| 6.2 | `manual` | as expected | One Engagement page at a time. Each showed `Contract / Create Draft Contract` and **no Invoice section at all** — Invoices only exist once a Contract does, so the answer to "which invoices are outstanding" is not merely unaggregated, it is absent on most Engagement pages too |
| 6.2-a | `missing-feature (RA-G6)` [#264](https://github.com/markgoho/doula-cloud/issues/264) | as expected | Confirmed unwalkable. The Clients screen's complete link set is **Add a Client** and the three Client names |
| 7.2 | `missing-feature (RA-G7)` [#265](https://github.com/markgoho/doula-cloud/issues/265) | as expected | Confirmed unwalkable. **Billing** is `Credit balance: 0`, a Date / Origin / Quantity ledger, `Quantity` and **Buy credits** — Doula Cloud's money, not her Clients'. The only invoice route in the whole API hangs off one Engagement's Contract (`api/main.go:210-213`) |
| 8.1 | `manual` | as expected | Signed in on a 390x844 iPhone context. The practice screen renders, the seven links stack, and `document.documentElement.scrollWidth === clientWidth === 390` — no horizontal overflow. The phone is not the problem |
| 8.2 | `missing-feature (RA-G5)` [#263](https://github.com/markgoho/doula-cloud/issues/263) | as expected | Confirmed unwalkable. From the phone the only screens that exist are Clients (Name, Status) and Billing. No availability, no on-call, no coverage — and no route to build one from |
| 9.2 | `manual` | as expected — **passes** | Filled the Birth Plan (`atmosphere: filled-0`), saved, then added `Hospital transfer wishes` to the Birth Plan template and saved that. Reopening the Engagement showed the original five fields and the kept answer; the new field is in the template response and **absent** from the instance. The snapshot holds |

**16 `manual` steps walked; 7 `missing-feature` steps confirmed unwalkable; no
`blocked` step on this plan.** Three expected results were falsified — 1.2, 1.3
and 4.3 — minting **RA-G9** and **RA-G10** on the journey map; the plan's own
cells are corrected above. No `journey-gap` issue was filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and the one half that passes
is the half she did not ask about.** A new Doula did accept an invitation and does
hold the Doula role — but only because a `PATCH` was made from a terminal against
a staff id no screen prints; nothing in the product does it. She does **not**
appear as the assigned Doula on an Engagement (RA-G4); the nearest thing the
product has is a dateless Visit reassigned by pasting a UUID (RA-G10). And no
screen shows every Engagement with its Contract and Invoice state (RA-G6, RA-G7).
What does pass is 6.1-a: the Clients list really is Practice-wide, so a Client
created by anyone shows up for everyone.

Her moment of truth landed where the map put it, and the walk sharpened *why*.
Stage 8 does not fail on the phone — the screen renders cleanly at 390px with no
overflow. It fails because there is nothing on it: at 2 a.m. the product can tell
her the names of three Clients and that each is `intake`. The failure is absence,
not layout, which is what makes RA-G5 a feature and not a stylesheet.

**For the walks behind this one**: Priya's plan step 3.3 and her journey map's
stage 3.3 both carry the claim this walk falsified — that Payments is owner-gated
— and both are corrected. A non-owner will see **Clients, Billing and Payments**,
not two tiles.
