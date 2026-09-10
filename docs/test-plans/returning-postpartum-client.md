# Camille Boyd — test plan

- **Journey**: [returning-postpartum-client.md](../journeys/returning-postpartum-client.md)
- **Persona**: [returning-postpartum-client.md](../personas/returning-postpartum-client.md)
- **A pass means**: two Engagements at one Practice — one closed and still readable, one live and postpartum — reachable from one portal account. **The third clause passes.** One Portal Account reaches her Client, `engagements_identity_visibility` (`00082`) makes every Engagement it holds readable before one is chosen, the portal root lists them, and `engagementLabel` tells two at one Practice apart by when each began ([#309](https://github.com/markgoho/doula-cloud/issues/309), [#310](https://github.com/markgoho/doula-cloud/issues/310), [#312](https://github.com/markgoho/doula-cloud/issues/312)). The first two clauses have code behind them too — `engagement/transition.go` writes `engagements.status`, and `00042_client_intake_schema.sql` gave `engagements` a `kind` — and the cells that still call them missing are held at [#1241](https://github.com/markgoho/doula-cloud/issues/1241) rather than half-corrected here.

Her persona file says the schema supports her, and it does: `clients` carries no `practice_id`. This plan is walkable to the end — the one refusal still on her path is one she can observe, and it is a refusal the product means.

## Preconditions

- A Practice with an Owner and Priya holding `doula`, set either by the
  Invitation that brought her in or by **Edit membership** on the Staff screen
  (#316).
- **Her first Engagement, created and left alone.** It cannot be closed
  (**MO-G4**), so create it fresh and treat it as the 2024 one. That two years of
  finished work is indistinguishable from an Engagement made this morning is stage
  1's finding, not a defect in the fixture.
- **At least two Client credits.** Stage 3 spends a second one on a person the
  Practice has already paid for (**MO-G9**).
- **One email address, and one deliberate duplicate.** She needs only the address she already signs in with: 5.2 is her existing login, not a second account. Step 5.3 alone needs a second Client record for her at this Practice, saved on purpose by answering intake's duplicate screen with *a different person* — that branch is the only way to reach the accept-side refusal, and reaching it is the point of the step.

## Steps

### Stage 1 — Two years ago, the first Engagement ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Mark her first Engagement finished | No handler writes `UPDATE engagements`. The record of her first birth still reads `intake`, and nothing distinguishes it from one created this morning | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) |

### Stage 2 — She calls Priya

No step: the product is not involved, and **that is the finding**. Every fact she
is relying on — who she is, who her doula was, how her first birth went — lives in
a message thread and in Priya's memory, not in a field. Nothing to walk, nothing to
mark; the consequences land in stages 3 and 8.

### Stage 3 — The Practice types her in again

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/clients` and find Camille's existing record | The row is there, and it is no longer a dead end: it opens her Client detail hub, which carries **Start new work with Camille** — the ask for a second Engagement, raised against the record she already is | `manual` |
| 3.1-a | Look her up by email, or add an Engagement to the Client she already is | **Both, and the first is the only door to the second.** Intake begins at **Find or add a Client**, a search; `CreateHandler` runs lookup-before-insert and refuses with the matches rather than inserting blind; and asking for new work with an existing Client is its own act, the Engagement Request, whose approval creates the Engagement and locks the Credit. A Client and an Engagement are no longer one indivisible request (ADR-0017) ([CB-G1](https://github.com/markgoho/doula-cloud/issues/307) closed) | `manual` |
| 3.2 | Create a new Client with the same name and email | **It is stopped and asked about.** `FindCollisions` answers the save with `409` and the matching records, and intake's own duplicate screen puts the question ADR-0017 wrote it for: this is her, or a different person. Choosing *this is her* means editing the Client who already exists rather than saving a second one. No credit rides on the answer either way, because saving a Client is free now (**[MO-G9](https://github.com/markgoho/doula-cloud/issues/257)** closed) | `manual` |
| 3.2-a | Carry anything from her first Engagement across by hand | **There is nothing to carry, which is the point.** She is one Client record — twelve structural columns plus whatever the Practice added to its own Client Field Template — and the second Engagement hangs off that same record rather than off a retyped copy of her. Her Practice-defined values are read live, so what the Practice knows about her today is what the new work sees ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252) closed) | `manual` |

### Stage 4 — Declaring it postpartum-only

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Record that this Engagement is postpartum work, not a birth | `engagements` has no type or kind column, only `status`, and the create handler names `intake` as the constant with no create-time alternative. `CONTEXT.md` calls Engagement "deliberately generic so it fits both birth-doula and postpartum-doula work"; **generic turns out to mean silent** | `missing-feature (CB-G2)` [#308](https://github.com/markgoho/doula-cloud/issues/308) |
| 4.1-a | Approximate it by moving the status to `postpartum` | Unavailable anyway (**MO-G4**) — and it would say she has given birth under this Engagement, which she has not | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) |

### Stage 5 — The second invitation, and the login she already has

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Priya sends the portal invite on the new Engagement | **`409`, and it is the right answer**: an invitation is raised per Client, not per Engagement, so `invite()` finds the accepted `client_portal_users` row Camille already holds and returns "this client already has portal access". There is nothing to send because she can already get in. Where an invitation *is* raised, the link now travels as a Practice-voice Notification email rather than by hand (**[RA-G1](https://github.com/markgoho/doula-cloud/issues/260)** closed) | `manual` |
| 5.2 | Sign in as herself and reach the new Engagement | **She reaches it through the login she already has.** A sign-in link to her existing address lands her on the portal root list, holding both Engagements. There is no "I already have an account" fork to choose: accepting an invitation is one **Continue** button (ADR-0026 — the invitation is the first sign-in link, and a Client has no password), and the table-wide `UNIQUE` on `client_portal_users.identity_uid` that used to refuse her is gone ([CB-G3](https://github.com/markgoho/doula-cloud/issues/309) closed, [#819](https://github.com/markgoho/doula-cloud/issues/819) replaced it with `UNIQUE (identity_uid, client_id)`) | `manual` |
| 5.3 | Down the duplicate-Client branch only: press **Continue** on an invitation raised against a second Client record for her at this Practice | **`409`, and the page prints the string**: "you already have portal access at this practice -- sign in instead of accepting a new invitation". `portal_account_reuse_for_accept` (`00081`) answers the one question ADR-0015 makes the rule — does this sign-in address's Portal Account already reach a Client at this Practice — and accept refuses rather than silently merging. This is the only refusal left on her path, and it tells her what to do instead | `manual` |

### Stage 6 — One account, two Engagements — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Sign in | **The portal root list, holding both Engagements.** `decidePortalLanding` redirects only where there is exactly one; with two it lists them. `client-portal-login.e2e.ts` and `portal-invite-accept.e2e.ts` each provision a Client who has never been seen before and assert the single-Engagement redirect, so neither drives this step the way she would | `manual` |
| 6.2 | Open her 2024 birth Engagement, then the postpartum one | Both open, from one login. Every Engagement her Portal Account reaches is readable under `engagements_identity_visibility` (`00082`), which is the identity-tier read the root list needs before any `app.current_client_id` is set | `manual` |
| 6.2-a | Move between the two without signing out | **The chrome carries the way back.** The authenticated portal layout's top bar holds a link to the root list, labeled with `engagementLabel` for the Engagement she is in, so the list is one press away from anywhere inside either ([CB-G4](https://github.com/markgoho/doula-cloud/issues/310) closed) | `manual` |
| 6.2-b | Tell the two apart in the list | `engagementLabel` names each one **"{Practice}, started {date}"**, so two at Rooted Birth Collective differ by when each began — the one fact that is honest for every Client, including one whose care ended in loss | `manual` |

### Stage 7 — Offered a Birth Plan she does not need

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open the portal home on the postpartum Engagement | **Birth Plan** and **Contract** links, both rendered unconditionally | `manual` |
| 7.2 | Open the Birth Plan link | "No Birth Plan has been created for this Engagement yet" — which promises one is coming rather than saying it does not apply | `manual` |
| 7.2-a | Mark that no Birth Plan applies to this Engagement | There is no way to mark an Engagement as anything ([CB-G2](https://github.com/markgoho/doula-cloud/issues/308)). Priya may fill one in just to clear the empty state, which puts a labour-preferences document on a postpartum Engagement | `missing-feature (CB-G5)` [#311](https://github.com/markgoho/doula-cloud/issues/311) |

### Stage 8 — Nothing came with her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Read her new message thread | Empty. Messages are one thread per Engagement and Plan Instances are per Engagement by ADR-0001's snapshot rule — correct scoping, and it means her history does not travel | `manual` |
| 8.1-a | See a person's Engagements over time, from her side or Priya's | No such view exists on either side. "They know me" is true of Priya and false of the product | `missing-feature (CB-G6)` [#312](https://github.com/markgoho/doula-cloud/issues/312) |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 0 |
| `manual` | 14 |
| `missing-feature` | 5 ([MO-G4](https://github.com/markgoho/doula-cloud/issues/253) ×2, [CB-G2](https://github.com/markgoho/doula-cloud/issues/308), [CB-G5](https://github.com/markgoho/doula-cloud/issues/311), [CB-G6](https://github.com/markgoho/doula-cloud/issues/312)) |

No step is `blocked`. Nothing on her path touches Stripe.

CB-G3, **MO-G9**, **RA-G1** and NH-G4 are observed inside walkable steps (5.2, 3.2, 5.1, 6.1) rather than given steps of their own. CB-G3 is closed, and 5.2 is now where that shows: the step is performed, and what it produces is her existing login reaching both Engagements rather than a `409`.

**She has no automated step any more.** Every spec in the suite provisions a Client who has never been seen before, holding exactly one Engagement, so none of them drives the two-Engagement root list her whole path now ends in — which is the same reason her first login used to be her only automated step, read the other way round.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 6.1 | `client-portal-login.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-09-10 — narrative reconciliation ([#685](https://github.com/markgoho/doula-cloud/issues/685))

A desk pass over this plan's Add Client cells, which [#318](https://github.com/markgoho/doula-cloud/issues/318) left alone. Nothing was re-walked. Two steps are re-marked, both from `missing-feature` to `manual`, because the capability their gap named now exists and the step can be performed.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 3.1 | "a dead end" -> her row opens a Client detail hub carrying **Start new work with Camille** | The Client detail page's own `Start new work with {name}` link into the Engagement Request route |
| 3.1-a | `missing-feature (CB-G1)` -> `manual`: a search, a lookup-before-insert create, and an Engagement Request against the Client she already is | ADR-0017, `api/internal/client/create.go`, and the intake routes under `clients/new` and `clients/search`. [CB-G1](https://github.com/markgoho/doula-cloud/issues/307) is closed |
| 3.2 | "Succeeds... a second credit is consumed" -> a `409` carrying the matches, and intake's duplicate screen asking whether this is her | `FindCollisions` in `CreateHandler`, and ADR-0017 on when a Credit locks. [MO-G9](https://github.com/markgoho/doula-cloud/issues/257) is closed |
| 3.2-a | `missing-feature (MO-G3)` -> `manual`: there is nothing to carry, because she is one record and the second Engagement hangs off it | ADR-0017's twelve structural columns and its Practice-defined layer, read live. [MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed |

**`add-client-visits.e2e.ts` drives none of these.** The spec's Client is brand new and has no prior match, so it lands straight on the detail hub and never passes through the match-review screens these four steps are entirely about. The README's rule — a mark counts only where the spec exercises the step the way the Persona would — keeps all four `manual`.

**Left alone on purpose.** 5.2's CB-G3 is her moment of truth and a portal-account step rather than an Add Client one; it, and every other cell on this plan, waits on the walk [#329](https://github.com/markgoho/doula-cloud/issues/329) owns.

### 2026-09-10 — the portal stages, against #309 as built ([#1135](https://github.com/markgoho/doula-cloud/issues/1135))

A desk pass over stages 5 and 6, which [#685](https://github.com/markgoho/doula-cloud/issues/685) left alone on purpose. Nothing was re-walked. Three steps are re-marked and one refusal moved house.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 5.1 | `201` and a link delivered by hand -> `409` "this client already has portal access" | `invite()` reads `client_portal_users` by `client_id` and refuses where an accepted row exists; after ADR-0017 her second Engagement hangs off the Client she already is. `queueOutboxSend` mails the link where one is raised ([RA-G1](https://github.com/markgoho/doula-cloud/issues/260) closed) |
| 5.2 | `409` "a portal account already exists for this identity" -> her existing login reaching both Engagements | `00081_portal_account_reuse.sql` dropped `client_portal_users_identity_uid_key`; [#819](https://github.com/markgoho/doula-cloud/issues/819) put `UNIQUE (identity_uid, client_id)` in its place. The accept screen is one **Continue** button (ADR-0026), so the step's own action text was stale too |
| 5.3 | a second account under a different email -> the one refusal that remains, down the duplicate-Client branch | `portal_account_reuse_for_accept` (`00081`) and `acceptInvite`'s use of it: "you already have portal access at this practice -- sign in instead of accepting a new invitation" |
| 6.1 | `automated (client-portal-login.e2e.ts)` -> `manual`: the root list, holding both | `decidePortalLanding` lists rather than redirects above one Engagement. Both portal specs provision a never-seen Client with one Engagement and assert the single-Engagement redirect, so neither drives this step the way she would |
| 6.2, 6.2-a, 6.2-b | two accounts and no switcher -> one account, a persistent way back to the list, and a label that distinguishes | `engagements_identity_visibility` (`00082`), the authenticated layout's `switcherLabel`, and `engagementLabel`'s "{Practice}, started {date}" ([CB-G4](https://github.com/markgoho/doula-cloud/issues/310) closed) |

**Two refusals, and her path meets the earlier one.** The refusal this plan was written around is gone; a *different*, deliberate refusal took its place at the accept, and a second one sits at the invite. Which of them a walker meets depends on how intake's duplicate screen was answered — that is why 5.1 and 5.3 now name different strings, and why 5.3 says which branch it needs.

**Left alone on purpose.** Stages 1, 4, 7 and 8 name gaps that are also closed ([#253](https://github.com/markgoho/doula-cloud/issues/253), [#308](https://github.com/markgoho/doula-cloud/issues/308), [#311](https://github.com/markgoho/doula-cloud/issues/311), [#312](https://github.com/markgoho/doula-cloud/issues/312)); correcting them means reading each gap issue rather than reading prose, and they are held at [#1241](https://github.com/markgoho/doula-cloud/issues/1241). The map's CB-G1 row is held at [#1236](https://github.com/markgoho/doula-cloud/issues/1236). The walk logs above and below are records of what was seen on the day and are untouched.

### 2026-08-23 — manual and missing-feature steps ([#241](https://github.com/markgoho/doula-cloud/issues/241))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Rooted Birth
Collective", Owner+Admin+Doula in one Staff row — Priya's shape, since her
journey carries this plan's staff-side stages) with two `clients` rows for
one person (Camille Boyd), her two Engagements, and two portal accounts
under two email addresses. Walked in Chrome via playwriter, one browser
profile, re-authenticating whichever side was about to act — staff and
Client-portal share one `__session` cookie per origin on `localhost:5173`,
as prior walks found; not a new finding.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | confirmed | No status-change control anywhere on the Engagement page — only `Status`/`Created` in a description list, no way to mark it finished |
| 3.1 | `manual` | as expected | The Clients list row is a plain link back to the same Engagement; nothing on it opens a second one |
| 3.1-a | `missing-feature (CB-G1)` [#307](https://github.com/markgoho/doula-cloud/issues/307) | confirmed | Add a Client (name + email) is the only entry point; `POST .../clients` inserted a fresh `clients` row both times — no lookup, no search, no add-an-Engagement action anywhere |
| 3.2 | `manual` | as expected | A second `clients` row for Camille was created without complaint; credit balance went `3 -> 1` across the two creations (**[MO-G9](https://github.com/markgoho/doula-cloud/issues/257)**) |
| 3.2-a | `missing-feature (MO-G3)` [#252](https://github.com/markgoho/doula-cloud/issues/252) | confirmed | The Add a Client form takes name and email only, both times |
| 4.1 | `missing-feature (CB-G2)` [#308](https://github.com/markgoho/doula-cloud/issues/308) | confirmed | Neither Engagement page shows anything beyond `Status`/`Created` — no kind field, no create-time alternative to `intake` |
| 4.1-a | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | confirmed | Same absence — no status control exists on either Engagement to approximate with |
| 5.1 | `manual` | as expected | `POST .../portal-invite` returned `201`; the link was printed in plain text for hand delivery (**[RA-G1](https://github.com/markgoho/doula-cloud/issues/260)**) |
| 5.2 | `manual` | as expected | `POST /api/portal/accept-invite` returned `409 CONFLICT`; the page showed the exact string "a portal account already exists for this identity" |
| 5.3 | `manual` | as expected | A second account under a different email address succeeded and landed straight on the postpartum Engagement |
| 6.2 | `manual` | as expected | Account A resolves only to the 2024 Engagement (still `intake`); account B resolves only to the postpartum one; no route connects them beyond signing out and back in |
| 6.2-a | `missing-feature (CB-G4)` [#310](https://github.com/markgoho/doula-cloud/issues/310) | confirmed | The authenticated layout's entire chrome is one Sign-out button, on both accounts — no switcher anywhere |
| 6.2-b | `missing-feature (CB-G4)` [#310](https://github.com/markgoho/doula-cloud/issues/310) | confirmed by inspection, not by walking | [CB-G1](https://github.com/markgoho/doula-cloud/issues/307) and [CB-G3](https://github.com/markgoho/doula-cloud/issues/309) keep any one identity from ever holding two Engagements in this stack, so the two-Engagement chooser this step asks about cannot be produced live; the claim rests on `login/+page.svelte` labeling an Engagement by `practiceName` alone, as the map already argued |
| 7.1 | `manual` | as expected | **Birth Plan** and **Contract** links both render unconditionally on the postpartum Engagement |
| 7.2 | `manual` | as expected | "No Birth Plan has been created for this Engagement yet." |
| 7.2-a | `missing-feature (CB-G5)` [#311](https://github.com/markgoho/doula-cloud/issues/311) | confirmed | Same absence as 4.1 — no way to mark an Engagement as not needing a Birth Plan |
| 8.1 | `manual` | as expected | The postpartum Engagement's message thread is empty, isolated from the 2024 thread |
| 8.1-a | `missing-feature (CB-G6)` [#312](https://github.com/markgoho/doula-cloud/issues/312) | confirmed | No view, on either side, shows a person's Engagements over time |

**18 steps walked: 9 `manual`, 9 `missing-feature`. Every mark holds.** No
`blocked` step exists on this plan, confirmed — nothing on her path touches
Stripe. No `journey-gap` issue filed from this ticket — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209), still blocked on
this ticket alone now that it is the last of the nine.

**Verdict**: this plan cannot pass, as written, and does not, on all three
clauses. The 2024 Engagement stays `intake` forever; the postpartum one
cannot declare what it is; and reaching both from one portal account is
refused at the exact step the map names as her moment of truth.
