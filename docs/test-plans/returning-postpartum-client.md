# Camille Boyd — test plan

- **Journey**: [returning-postpartum-client.md](../journeys/returning-postpartum-client.md)
- **Persona**: [returning-postpartum-client.md](../personas/returning-postpartum-client.md)
- **A pass means**: two Engagements at one Practice — one closed and still readable, one live and postpartum — reachable from one portal account. **All three clauses pass.** Her first Engagement completes through `TransitionHandler` (`PATCH .../engagements/{id}/status`) and stays readable afterward ([MO-G4](https://github.com/markgoho/doula-cloud/issues/253), closed); her second is created already declared `kind: postpartum` through the Engagement Request Priya raises from her Client detail hub ([CB-G2](https://github.com/markgoho/doula-cloud/issues/308), closed); and one Portal Account reaches her Client, `engagements_identity_visibility` (`00082`) makes every Engagement it holds readable before one is chosen, the portal root lists them, and `engagementLabel` tells two at one Practice apart by when each began ([#309](https://github.com/markgoho/doula-cloud/issues/309), [#310](https://github.com/markgoho/doula-cloud/issues/310), [#312](https://github.com/markgoho/doula-cloud/issues/312)).

Her persona file says the schema supports her, and it does: `clients` carries no `practice_id`. This plan is walkable to the end — the one refusal still on her path is one she can observe, and it is a refusal the product means.

## Preconditions

- A Practice with an Owner and Priya holding `doula`, set either by the Invitation that brought her in or by **Edit membership** on the Staff screen (#316).
- **Her first Engagement, created and completed.** On the Engagement page, **Record what happened** (a live birth, with a past date), then **Change** on Care phase, **Mark care complete**, and an ending reason. Completion refuses a birth Engagement with no outcome. Then treat it as the 2024 one. That she can close it and it stays readable afterward is stage 1's finding.
- **At least two Client credits.** Stage 4's Engagement Request locks one when it is approved, and 5.3's duplicate locks a second.
- **A portal login she already holds.** Her 2024 Client must carry an *accepted* `client_portal_users` row before stage 5 begins — invite it and accept it as part of building the fixture. `invite()` refuses only where `identity_uid` is already set, so a Client whose portal row is still pending answers 5.1 with `201`, and the walk reads the plan as wrong rather than the fixture as unfinished.
- **One email address, and one deliberate duplicate.** She needs only the address she already signs in with: 5.2 is her existing login, not a second account. Step 5.3 alone needs a second Client record for her at this Practice, saved on purpose by answering intake's duplicate screen with *a different person* — that branch is the only way to reach the accept-side refusal, and reaching it is the point of the step.

## Steps

### Stage 1 — Two years ago, the first Engagement ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Mark her first Engagement finished | **Record what happened**, then **Mark care complete** with "Why is this Engagement ending?" answered: "The care phase is now Completed." `TransitionHandler` (`PATCH .../engagements/{id}/status`) writes the status and ending reason, and Activity records both moves with who made them ("Care for the Client ended: Active to Completed. Reason: …"). The Doula row leaves the header, and On call reads "Care has ended." once reloaded | `manual` |

### Stage 2 — She calls Priya

No step: the product is not involved, and **that is the finding**. Every fact she is relying on — who she is, who her doula was, how her first birth went — lives in a message thread and in Priya's memory, not in a field. Nothing to walk, nothing to mark; the consequences land in stages 3 and 8.

### Stage 3 — The Practice types her in again

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/clients` and find Camille's existing record | The row is there, and it is no longer a dead end: it opens her Client detail hub, which carries **Start new work with Camille** — the ask for a second Engagement, raised against the record she already is. With her only Engagement completed, the row is her name alone, with no kind or status | `manual` |
| 3.1-a | Look her up by email, or add an Engagement to the Client she already is | **Both, and the first is the only door to the second.** Intake begins at **Find or add a Client**, a search; `CreateHandler` runs lookup-before-insert and refuses with the matches rather than inserting blind; and asking for new work with an existing Client is its own act, the Engagement Request, whose approval creates the Engagement and locks the Credit. A Client and an Engagement are no longer one indivisible request (ADR-0017) ([CB-G1](https://github.com/markgoho/doula-cloud/issues/307) closed) | `manual` |
| 3.2 | Create a new Client with the same name and email | **It is stopped and asked about.** `FindCollisions` answers the save with `409` and the matching records, and intake's own duplicate screen puts the question ADR-0017 wrote it for: this is her, or a different person. Choosing *this is her* means editing the Client who already exists rather than saving a second one. No credit rides on the answer either way, because saving a Client is free now (**[MO-G9](https://github.com/markgoho/doula-cloud/issues/257)** closed) | `manual` |
| 3.2-a | Carry anything from her first Engagement across by hand | **There is nothing to carry, which is the point.** She is one Client record — twelve structural columns plus whatever the Practice added to its own Client Field Template — and the second Engagement hangs off that same record rather than off a retyped copy of her. Her Practice-defined values are read live, so what the Practice knows about her today is what the new work sees ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252) closed) | `manual` |

### Stage 4 — Declaring it postpartum-only

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Record that this Engagement is postpartum work, not a birth | **She can, at the ask.** The Engagement Request Priya raises from Camille's Client detail hub demands "Select whether this is birth or postpartum work" before it can be sent. An Owner or Admin approves it ("Uses 1 credit"), and approval creates the Engagement already carrying `kind: postpartum` (`engagements.kind`, `00042_client_intake_schema.sql`), with no due date asked and "Postpartum care is not on-call work." `GLOSSARY.md`'s "deliberately generic" claim now holds because the product knows which it is | `manual` |
| 4.1-a | Approximate it by moving the status to `postpartum` | **Refused, and no longer needed.** `engagement_status` carries no `postpartum` member any more (dropped by `00057_engagement_status_drop_postpartum.sql`), and `TransitionHandler` refuses any target besides `active`/`completed` with a 400. 4.1's `kind` field already records the fact directly, so there is nothing left to approximate | `manual` |

### Stage 5 — The second invitation, and the login she already has

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Priya opens the new Engagement and sends the portal invite from it | **There is nothing to press.** The header reads "Portal invite: Accepted" and the page offers no **Send portal invite**. Called directly, `POST .../portal-invite` answers **`409`, and it is the right answer**: an invitation is raised per Client, not per Engagement, so `invite()` finds the accepted `client_portal_users` row Camille already holds and returns "this client already has portal access". There is nothing to send because she can already get in. Where an invitation *is* raised, the link now travels as a Practice-voice Notification email rather than by hand (**[RA-G1](https://github.com/markgoho/doula-cloud/issues/260)** closed) | `manual` |
| 5.2 | Sign in as herself and reach the new Engagement | **She reaches it through the login she already has.** A sign-in link to her existing address lands her on the portal root list, holding both Engagements. There is no "I already have an account" fork to choose: accepting an invitation is one **Continue** button (ADR-0026 — the invitation is the first sign-in link, and a Client has no password), and the table-wide `UNIQUE` on `client_portal_users.identity_uid` that used to refuse her is gone ([CB-G3](https://github.com/markgoho/doula-cloud/issues/309) closed, [#819](https://github.com/markgoho/doula-cloud/issues/819) replaced it with `UNIQUE (identity_uid, client_id)` in `00107_portal_account_client_pair_unique.sql`) | `manual` |
| 5.3 | Down the duplicate-Client branch only: press **Continue** on an invitation raised against a second Client record for her at this Practice | **`409`, and the page prints the string** under "There is a problem": "you already have portal access at this practice -- sign in instead of accepting a new invitation". `portal_account_reuse_for_accept` (`00081`) answers the one question ADR-0015 makes the rule — does this sign-in address's Portal Account already reach a Client at this Practice — and accept refuses rather than silently merging. This is the only refusal left on her path. It is the API's own words, it offers no way on to her care, and **Continue** stays on the page ([CB-G9](https://github.com/markgoho/doula-cloud/issues/2279)) | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The sign-in she always uses, then the portal root list | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law) | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). She reaches the new Engagement under the login she already has, and the screen says so | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — One account, two Engagements — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Sign in | **The portal root list, holding both Engagements.** `decidePortalLanding` redirects only where there is exactly one; with two it lists them. `client-portal-login.e2e.ts` and `portal-invite-accept.e2e.ts` each provision a Client who has never been seen before and assert the single-Engagement redirect, so neither drives this step the way she would | `manual` |
| 6.2 | Open her 2024 birth Engagement, then the postpartum one | Both open, from one login. Every Engagement her Portal Account reaches is readable under `engagements_identity_visibility` (`00082`), which is the identity-tier read the root list needs before any `app.current_client_id` is set | `manual` |
| 6.2-a | Move between the two without signing out | **The chrome carries the way back.** The authenticated portal layout's top bar holds a link to the root list from every screen inside an Engagement, so the list is one press away and no sign-out is involved ([CB-G4](https://github.com/markgoho/doula-cloud/issues/310) closed). Watch what that link *reads*: its visible text is the Practice's name alone, and `engagementLabel` is only its accessible name (`PortalTopBar`), so a sighted Camille sees the same two words inside either Engagement | `manual` |
| 6.2-b | Tell the two apart in the list | `engagementLabel` names each one **"{Practice}, started {date}"**, so two at Rooted Birth Collective differ by when each began — the one fact that is honest for every Client, including one whose care ended in loss. Under each label, a status: "Care ended" or "Getting started". Inside either one, the visible page says only "Rooted Birth Collective" ([CB-G7](https://github.com/markgoho/doula-cloud/issues/2276)) | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 3 (Hick's Law, Flow). The root list, one Engagement, then the other by way of the list | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). Her two Engagements | `manual` |
| 6.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Each Engagement is named by Practice and start date, so she need not recall which is which | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The Engagement she opens names which one it is | `manual` |
| 6.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User). The root list is new to her; she has only ever had one Engagement | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — Offered a Birth Plan she does not need

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open the portal home on the postpartum Engagement | The top bar holds **Your care**, **Messages**, **Contract** and **Notifications**, and no **Birth plan**. `offersBirthPlan` is false (`kind == postpartum`), so the authenticated layout's nav leaves **Birth plan** out entirely, and **Your care** carries no Birth Plan card | `manual` |
| 7.2 | Open the Birth Plan URL directly | The portal's ordinary not-found page, with a way back to her care — not a "No Birth Plan has been created … yet" promise that one is coming | `manual` |
| 7.2-a | Confirm nothing marks the Engagement "no Birth Plan" by hand | There is no separate control, and none is needed: `OffersBirthPlan` (`engagement/kind.go`) derives the suppression from `kind` and the birth outcome on every read, so nothing can drift out of sync with a stored flag ([CB-G5](https://github.com/markgoho/doula-cloud/issues/311), closed) | `manual` |
| 7.2-b | From the Practice side, start a Birth Plan on the postpartum Engagement | **The Practice side offers one, and it goes nowhere.** **Start the Birth Plan** is on the Staff page. A saved answer reads "Camille has not read this yet", with **Print for the hospital**. Her portal still 404s the Birth Plan, but **Show what has happened** lists "Your Birth Plan was started." ([CB-G8](https://github.com/markgoho/doula-cloud/issues/2277), [#2219](https://github.com/markgoho/doula-cloud/issues/2219)) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). The four destinations a postpartum Engagement offers, with no Birth plan among them | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 8 — Nothing came with her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Read her new message thread | Empty. Messages are one thread per Engagement and Plan Instances are per Engagement by ADR-0001's snapshot rule — correct scoping, unchanged, and it means her history does not travel | `manual` |
| 8.1-a | See a person's Engagements over time, from her side or Priya's | **Both sides answer it now.** Priya's Client detail hub lists every Engagement Camille's Client record holds (kind, status, started) beside a merged History table of Engagement Requests and Client edits (`clientDetail.ts`'s `EngagementSummary`, #494); Camille's own portal root list is the same fact from her side (stage 6, `engagements_identity_visibility` / `00082`). An approved Request's History row names the Staff member who asked, at the time she asked, as the one who approved it ([#2280](https://github.com/markgoho/doula-cloud/issues/2280)) | `manual` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Nothing asks her to retell her first Engagement; the Practice already holds it | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 4 (4 of them [budget steps](README.md#budget-steps)) |
| `manual` | 33 (13 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 0 |

No step is `blocked`. Nothing on her path touches Stripe.

CB-G3, **MO-G9**, **RA-G1** and NH-G4 are observed inside walkable steps (5.2, 3.2, 5.1, 6.1) rather than given steps of their own. CB-G3 is closed, and 5.2 is now where that shows: the step is performed, and what it produces is her existing login reaching both Engagements rather than a `409`.

**She has no automated step any more.** Every spec in the suite provisions a Client who has never been seen before, holding exactly one Engagement, so none of them drives the two-Engagement root list her whole path now ends in — which is the same reason her first login used to be her only automated step, read the other way round.

## Run log

### 2026-10-28 — second manual walk ([#2178](https://github.com/markgoho/doula-cloud/issues/2178))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, in WSL2 with rootless Podman. The main checkout was held at `6d65dc61`, and the record was written on trunk at `bf2be115`. `scripts/stripe-listen.sh` was running. This walk made no Stripe call. The stack was not restarted.

**Devices**: headless Chrome inside WSL, driven by the chrome-devtools MCP, at desktop size, in UTC. Her journey names no device. There was no CPU or network throttling. There were three isolated browser contexts: Priya Raman, Renata Alvarez, and a fresh one for Camille from stage 5 on, which is the Cold walk. Every timing is on an unbundled Vite dev server, which compiles a route the first time it is visited. A first load is not read as a finding.

**Accounts and preconditions**, on the seeded Rooted Birth Collective (`19a0107e-…`) as Lena Vasquez's walk ([#2177](https://github.com/markgoho/doula-cloud/issues/2177)) left it: 18 Staff (two Priyas, the seeded Priya Raman and Priya Ramanathan), 13 Credits.

- **Camille** is the seeded `camille.boyd@sim.doula.cloud` (Client `fe17bcd6-…`). She holds an accepted portal login and a birth Engagement (`dffc8ebf-…`, due Nov 16, 2026, Active, Doula Priya Raman). It has a signed Contract, `INV-0001` Open, and three Visits, one of them on Fri Oct 30, 4:00pm. Her thread holds six messages, the last two from #2176. Priya and Renata used the seed password ([walk-a-screen-locally.md](../testing/walk-a-screen-locally.md)). Camille signed in by magic link from the catcher.
- **The "2024" Engagement is yesterday's.** The clock cannot move, so the seeded Engagement stands in for 2024. Priya recorded the birth as Oct 26, 2026, before the seed created the Engagement. Two of its Visits are unscheduled, and an unscheduled Visit is typed from when it was logged, so all three now read **Postpartum**, one with the note "First prenatal visit at home". That comes from the backdated outcome, not from the product, and is not filed.
- **Stage 3 and 4**: Priya asked for postpartum work from Camille's Client record, and Renata approved it (13 to 12 Credits). That gives Engagement `c512d1a5-…`.
- **5.3**: Renata saved a second "Camille Boyd" (`d3c9be0e-…`) with the same email, answered "No, this is a different person", and started postpartum work on it, Doula Priya Raman (12 to 11 Credits). That gives Engagement `a385971b-…`. She sent its portal invite, and Camille pressed **Continue** on it while signed in.
- **Not touched**: Mira Castellanos's Engagement, Maya Okonkwo's Practice, every other Client.

The 4 `automated` steps were **not** re-run as specs.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected, cell corrected | Priya, on the Engagement page. **Record what happened**, "The baby was born alive", 10 / 26 / 2026, **Record this outcome**: "What happened: The baby was born alive", "Date the pregnancy ended: Oct 26, 2026". Then **Change** on Care phase, **Mark care complete**, "Why is this Engagement ending?" (six reasons), "The work finished as agreed", a note, **Confirm completion**: "The care phase is now Completed." in 37 ms. After a reload the Doula row was gone and On call read "Care has ended." (before the reload it still listed Priya). Activity: "Care for the Client ended: Active to Completed. Reason: The work finished as agreed" and "Birth outcome recorded for the Client: Not recorded to The baby was born alive, Oct 26, 2026", both by Priya Raman. The Practice-wide **On call** no longer lists her. It read Active, not `intake`, before the move |
| 3.1 | `manual` | as expected, cell corrected | Her Clients row is now her name alone, with no kind or status. It opens her Client record: **Start new work with Camille**, and Engagements "Birth, Completed, Oct 27, 2026" |
| 3.1-a | `manual` | as expected | **Find or add a Client** with her email: "1 match", "Camille Boyd", "Added Oct 27, 2026 · 1 Engagement · ended Oct 28, 2026", "Not the right Client?", **Add a new Client** |
| 3.2 | `manual` | as expected | **Add a new Client**: the name page carried her email "From your search". Camille / Boyd, **Save and continue**, then "Is this the same person?", "Nothing has been saved yet.", her record as one choice ("Keeps this record."), "No, this is a different person" ("A second record is created under the same name. This is the only way to make one."), **Continue**. Choosing her went back to her record and saved nothing |
| 3.2-a | `manual` | as expected | Nothing to carry: the new work hangs off the same record, whose fields are read live |
| 4.1 | `manual` | as expected, cell corrected | Priya: "Ask to start work with Camille Boyd". Sent empty, it answered "Select whether this is birth or postpartum work" and "Select who the Doula is, or select No Doula yet". With **Postpartum**, "Priya Raman (you)" and a note: "Your request to start work with Camille is sent." Renata, from the Overview's "Requests awaiting approval 1 waiting": "Camille Boyd — already known here", "Credit cost 1 credit", "Balance after 12", "Birth work, started Oct 27, 2026 — Completed", **Approve and start the work**, **Continue**. The new Engagement reads "Care Postpartum", "Care phase Intake", "Doula Priya Raman", "Portal invite Accepted", "Postpartum care is not on-call work.", and no due date |
| 4.1-a | `manual` | as expected | `PATCH .../status` with `postpartum`: `400` "status must be 'active' or 'completed'". The UI's **Change** offers only **Mark care as active** and **Mark care complete** |
| 5.1 | `manual` | as expected, cell corrected | The page offers no **Send portal invite**; the header says "Portal invite Accepted". `POST .../portal-invite` directly: `409` "this client already has portal access" |
| 5.2 | `manual` | as expected | In a fresh context: `/portal/login`, **Send me a sign-in link** ("If that address is on our records, we have sent a sign-in link."), the link from the catcher, "You are signing in as camille.boyd@sim.doula.cloud.", **Continue**, the root list holding both |
| 5.3 | `manual` | as expected, and judged a gap | The duplicate's invite mail, "You've been invited to view your care details online", names no Practice. Its page: "Accept your portal invite", "Your doula invited you to see your care online.", **Continue**. Then "There is a problem", "you already have portal access at this practice -- sign in instead of accepting a new invitation". She is already signed in, nothing links on, and **Continue** stays ([CB-G9, #2279](https://github.com/markgoho/doula-cloud/issues/2279)). Renata's page reads "Portal invite Invite pending", and "An email has been sent to them" ([#2198](https://github.com/markgoho/doula-cloud/issues/2198)). The Clients list now holds two identical rows, "Camille Boyd, Postpartum, Intake, No contract yet" |
| 6.1 | `manual` | as expected | `/`: "Your care", then two links, "Rooted Birth Collective, started Oct 27, 2026" / "Care ended" and "Rooted Birth Collective, started Oct 28, 2026" / "Getting started". The list carries no account menu or sign-out, by design: each Engagement it opens carries one (`(signed-out)/+page.svelte`, #1116) |
| 6.2 | `manual` | as expected | Both open from one login. The completed one still says "Priya Raman is coming to you on Fri Oct 30, 4:00pm." ([NH-G12, #2217](https://github.com/markgoho/doula-cloud/issues/2217)) and "Your due date is Nov 16, 2026." ([HS-G11, #2211](https://github.com/markgoho/doula-cloud/issues/2211)). The postpartum one: "No visit is booked yet.", "No Contract has been sent for your care yet.", "No messages yet.", $0.00 |
| 6.2-a | `manual` | as expected | The top bar's first link reads "Rooted Birth Collective", with the accessible name "Rooted Birth Collective, started Oct 27, 2026" (or "Oct 28"), and goes to `/` |
| 6.2-b | `manual` | **better than the plan claimed**, cell corrected | Each label has its status under it ("Care ended", "Getting started"). Inside either one, the page heading is **Your care** over "Rooted Birth Collective", and only the tab title carries the date ([CB-G7, #2276](https://github.com/markgoho/doula-cloud/issues/2276)) |
| 7.1 | `manual` | as expected, cell corrected | Postpartum top bar: Your care, Messages, Contract, Notifications. The birth one's adds **Birth plan**. No Birth Plan card |
| 7.2 | `manual` | as expected | `/portal/engagements/{postpartum}/birth-plan`: "Page not found", "Check the web address is correct, or the page may have been removed. Trying again will not change that.", **Go to your care** (to the postpartum Engagement). `GET .../birth-plan` `404` |
| 7.2-a | `manual` | as expected | No control marks it by hand, and after 7.2-b the portal still hides it. The derivation holds |
| 7.2-b | `manual` (new) | **walked** | The Practice side's **Start the Birth Plan** on the postpartum Engagement. Priya chose "Home", **Save Birth Plan**: "Birth Plan saved.", "1 of 4 questions answered", "Camille has not read this yet", **Print for the hospital**. Her portal: no nav item, `404`, and **Show what has happened** lists "Your Birth Plan was updated." and "Your Birth Plan was started." ([CB-G8, #2277](https://github.com/markgoho/doula-cloud/issues/2277); [#2219](https://github.com/markgoho/doula-cloud/issues/2219), commented) |
| 8.1 | `manual` | as expected | **Messages** on the postpartum Engagement: "You and Rooted Birth Collective", "No messages yet.", Message, Attachment, **Send**. The birth Engagement's six messages stay on that one |
| 8.1-a | `manual` | as expected, and **one row is wrong**, cell corrected | Priya's Client record: Engagements "Birth, Completed, Oct 27, 2026" and "Postpartum, Intake, Oct 28, 2026". History: "Postpartum Engagement approved, Priya Raman". Renata approved it. The API carries `decidedByName: "Renata Alvarez"`, and the row shows the requester and the request's time. The "requested" row is gone ([#2280](https://github.com/markgoho/doula-cloud/issues/2280)) |

**Re-marked**: none. Every step was `manual` and stays `manual`. **New**: 7.2-b, the Practice-side check 7.2-a's claim needed. **Corrected without a mark change**: 1.1, 3.1, 4.1, 5.1, 5.3, 6.2-b, 7.1 and 8.1-a, and the Preconditions. The Marks summary is recounted from 4 / 32 / 0 / 0 to 4 / 33 / 0 / 0, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**: this plan's budget notes name the decisions only on stage 6 ("Her two Engagements") and stage 7 ("The four destinations a postpartum Engagement offers"). For the rest they are silent, so this walk uses the rule from the earlier second-run walks. Count fields, options, and links or buttons that move her somewhere or change what happens, if they are enabled and in view at once in the page body. A radio group is one decision, and a list of links is one decision per link. On stage 7 the portal top bar is counted, since the plan's note names its destinations. Elsewhere it is not counted, and neither are the skip link and the Privacy Policy link. **Time** is measured in the page: a script presses one control and polls each animation frame until the result has settled on screen. It records when the control first showed it was busy, or for a move, when the page body first changed.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 5.B-screens | 2 | **3**: `/portal/login`, the sign-in link page (**Continue**), the root list | **no**, [NH-G10 (#2215)](https://github.com/markgoho/doula-cloud/issues/2215), cited |
| 5.B-decisions | 2 | 2 on the sign-in form (Email address, **Send me a sign-in link**); 1 on the link page; 2 on the root list | yes |
| 5.B-confirmation | yes | yes. The root list holds the new Engagement beside the old one, labeled "started Oct 28, 2026" and "Getting started", under the login she already had | yes |
| 6.B-screens | 3 | 3: the root list, the birth Engagement, and the postpartum one by way of the list | yes |
| 6.B-decisions | 2 | 2 on the root list. **5** on the birth Engagement's **Your care** (Read your birth plan, Read what you signed, Open your messages, Invoices, Show what has happened); 3 on the postpartum one | **no**, [NH-G11 (#2216)](https://github.com/markgoho/doula-cloud/issues/2216), cited |
| 6.B-memory | yes | yes. Each line on the list names its Practice, start date and status | yes |
| 6.B-time | 400 ms, 100 ms | **Continue** to the settled list 87 ms, busy within 2 ms; list to the birth Engagement 169 ms, body changed at 78 ms; back to the list 26 ms; list to the postpartum one 82 ms, body changed at 47 ms. **Send me a sign-in link** 27 ms, busy within 2 ms | yes |
| 6.B-confirmation | yes | **no**. Inside either Engagement the page says **Your care** and "Rooted Birth Collective". Only the tab title says which one, and nothing says postpartum | **no**, [CB-G7 (#2276)](https://github.com/markgoho/doula-cloud/issues/2276) |
| 6.B-cold | yes | yes, in a fresh context. The sign-in page says what it sends, the link page says who she is signing in as, and the list needs no explaining | yes |
| 7.B-screens | 1 | 1: **Your care** | yes |
| 7.B-decisions | 4 | 4 in the top bar (Your care, Messages, Contract, Notifications); 3 in the body | yes |
| 8.B-screens | 1 | 1: **Messages**, in 32 ms | yes |
| 8.B-memory | yes | yes. Nothing asks her to retell her first Engagement. That one stays on the list, with its thread | yes |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` or `automated (portal-320.e2e.ts)` and were not re-measured. **Failed**: 5.B-screens, 6.B-decisions, 6.B-confirmation.

#### Moment of truth — the verdict, against the first run's

**Stage 6, both of her Engagements, in one list, under the login she already has.** First run ([#241](https://github.com/markgoho/doula-cloud/issues/241), 2026-08-23): refused. Her existing login met `409` "a portal account already exists for this identity". A second account under a second address was the only way in, and each account reached one Engagement, with no route between them. **This run: the moment of truth holds, and it is overturned from refused to working.** She asks for a link at the address she always uses, presses **Continue**, and lands on one list. It holds her completed birth Engagement and her new postpartum one, labeled "Rooted Birth Collective, started Oct 27, 2026" (Care ended) and "…, started Oct 28, 2026" (Getting started). Both open, and the top bar takes her back. Nobody invited her a second time, because the Practice could not and did not need to. It falls short in one place. Once she is inside an Engagement, the screen does not say which one it is ([CB-G7](https://github.com/markgoho/doula-cloud/issues/2276)). Her finished Engagement still tells her a visit is coming and a due date is ahead (NH-G12, HS-G11).

#### Is the journey completable end to end?

**Yes.** Against "a pass means":

- **"Two Engagements at one Practice."** Yes. Both are at Rooted Birth Collective, on one Client record.
- **"One closed and still readable."** Yes. It is Completed with a recorded outcome and reason, and it opens from the list with its Contract, Visits and thread.
- **"One live and postpartum."** Yes. It is `kind: postpartum` from the request that created it, with no on-call window, no due date, and no Birth Plan in her portal.
- **"Reachable from one portal account."** Yes. One magic link, one list.

The deliberate refusal down the duplicate branch (5.3) holds, but in the API's words (CB-G9). None of the findings stops her.

**Findings filed**: [CB-G7 (#2276)](https://github.com/markgoho/doula-cloud/issues/2276), a failed budget step; [CB-G8 (#2277)](https://github.com/markgoho/doula-cloud/issues/2277) and [CB-G9 (#2279)](https://github.com/markgoho/doula-cloud/issues/2279), each on the [journey map](../journeys/returning-postpartum-client.md#gaps-found), with its budget line quoted where a budget failed; and one bug, [#2280](https://github.com/markgoho/doula-cloud/issues/2280) (the History names the requester as the approver). Cited, with a comment on each: [NH-G10 (#2215)](https://github.com/markgoho/doula-cloud/issues/2215), [NH-G11 (#2216)](https://github.com/markgoho/doula-cloud/issues/2216), [NH-G12 (#2217)](https://github.com/markgoho/doula-cloud/issues/2217), [HS-G11 (#2211)](https://github.com/markgoho/doula-cloud/issues/2211) and [#2219](https://github.com/markgoho/doula-cloud/issues/2219). Cited without a comment: [#2198](https://github.com/markgoho/doula-cloud/issues/2198) ("them").

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 5, 6, 7, 8 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **17 budget steps**, 4 `automated`, 13 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

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
| 5.1 | "succeeds for a non-owner; the link goes by hand" -> `409` "this client already has portal access", and the link travels by email where one is raised | `invite()` reads `client_portal_users` by `client_id` and refuses where an accepted row exists; after ADR-0017 her second Engagement hangs off the Client she already is. `queueOutboxSend` mails the link where one is raised ([RA-G1](https://github.com/markgoho/doula-cloud/issues/260) closed) |
| 5.2 | `409` "a portal account already exists for this identity" -> her existing login reaching both Engagements | `00081_portal_account_reuse.sql` dropped `client_portal_users_identity_uid_key`; [#819](https://github.com/markgoho/doula-cloud/issues/819) put `UNIQUE (identity_uid, client_id)` in its place (`00107_portal_account_client_pair_unique.sql`). The accept screen is one **Continue** button (ADR-0026), so the step's own action text was stale too |
| 5.3 | a second account under a different email -> the one refusal that remains, down the duplicate-Client branch | `portal_account_reuse_for_accept` (`00081`) and `acceptInvite`'s use of it: "you already have portal access at this practice -- sign in instead of accepting a new invitation" |
| 6.1 | `automated (client-portal-login.e2e.ts)` -> `manual`: the root list, holding both | `decidePortalLanding` lists rather than redirects above one Engagement. Both portal specs provision a never-seen Client with one Engagement and assert the single-Engagement redirect, so neither drives this step the way she would |
| 6.2, 6.2-a, 6.2-b | two accounts and no switcher -> one account, a persistent way back to the list, and a label that distinguishes | `engagements_identity_visibility` (`00082`), the authenticated layout's `switcherLabel`, and `engagementLabel`'s "{Practice}, started {date}" ([CB-G4](https://github.com/markgoho/doula-cloud/issues/310) closed) |

**Two refusals, and her path meets the earlier one.** The refusal this plan was written around is gone; a *different*, deliberate refusal took its place at the accept, and a second one sits at the invite. Which of them a walker meets depends on how intake's duplicate screen was answered — that is why 5.1 and 5.3 now name different strings, and why 5.3 says which branch it needs.

**Left alone on purpose.** Stages 1, 4, 7 and 8 name gaps that are also closed ([#253](https://github.com/markgoho/doula-cloud/issues/253), [#308](https://github.com/markgoho/doula-cloud/issues/308), [#311](https://github.com/markgoho/doula-cloud/issues/311), [#312](https://github.com/markgoho/doula-cloud/issues/312)); correcting them means reading each gap issue rather than reading prose, and they are held at [#1241](https://github.com/markgoho/doula-cloud/issues/1241). The map's CB-G1 row is held at [#1236](https://github.com/markgoho/doula-cloud/issues/1236). The walk logs above and below are records of what was seen on the day and are untouched.

### 2026-08-23 — manual and missing-feature steps ([#241](https://github.com/markgoho/doula-cloud/issues/241))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Rooted Birth Collective", Owner+Admin+Doula in one Staff row — Priya's shape, since her journey carries this plan's staff-side stages) with two `clients` rows for one person (Camille Boyd), her two Engagements, and two portal accounts under two email addresses. Walked in Chrome via playwriter, one browser profile, re-authenticating whichever side was about to act — staff and Client-portal share one `__session` cookie per origin on `localhost:5173`, as prior walks found; not a new finding.

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

**18 steps walked: 9 `manual`, 9 `missing-feature`. Every mark holds.** No `blocked` step exists on this plan, confirmed — nothing on her path touches Stripe. No `journey-gap` issue filed from this ticket — that is [#209](https://github.com/markgoho/doula-cloud/issues/209), still blocked on this ticket alone now that it is the last of the nine.

**Verdict**: this plan cannot pass, as written, and does not, on all three clauses. The 2024 Engagement stays `intake` forever; the postpartum one cannot declare what it is; and reaching both from one portal account is refused at the exact step the map names as her moment of truth.

### 2026-09-20 — stages 1, 4, 7 and 8 correction ([#1241](https://github.com/markgoho/doula-cloud/issues/1241))

A desk pass over the four stages [#685](https://github.com/markgoho/doula-cloud/issues/685) and [#1135](https://github.com/markgoho/doula-cloud/issues/1135) left alone on purpose, corrected against the closed issues they named. Nothing was re-walked. Five steps are re-marked, all from `missing-feature` to `manual`, because the capability each gap named is now built.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 1.1 | "No handler writes `UPDATE engagements`" -> `TransitionHandler` completes an Engagement, recording an ending reason and an `engagement_events` row | `engagement/transition.go`, `engagement/activate.go`, `00090_engagement_status_transition.sql`. [MO-G4](https://github.com/markgoho/doula-cloud/issues/253) is closed |
| 4.1 | "No type or kind column" -> the Engagement Request that creates a second Engagement asks birth-or-postpartum and the Engagement carries `kind` from creation | `engagements.kind` (`engagement_kind`), `00042_client_intake_schema.sql`, and the Engagement Request form's own required field. [CB-G2](https://github.com/markgoho/doula-cloud/issues/308) is closed |
| 4.1-a | `missing-feature (MO-G4)` -> `manual`: the status-to-`postpartum` approximation is refused (the status value itself no longer exists) and no longer needed, now that 4.1 records the fact directly | `00057_engagement_status_drop_postpartum.sql`, `TransitionHandler`'s target validation, and 4.1's own `kind` field |
| 7.1, 7.2, 7.2-a | "Birth Plan link renders unconditionally" / "promises one is coming" -> the nav item is left out entirely where `offersBirthPlan` is false, and the route itself renders the portal's ordinary not-found page | `engagement.OffersBirthPlan` (`engagement/kind.go`), the authenticated portal layout's nav, and the Birth Plan route's own not-found branch. [CB-G5](https://github.com/markgoho/doula-cloud/issues/311) is closed |
| 8.1-a | "No such view exists on either side" -> the Client detail hub's Engagements table and merged History answer it for Priya, and the portal root list answers it for Camille | `clientDetail.ts`'s `EngagementSummary`/`HistoryEntry` (#494) on the Practice side, `engagements_identity_visibility` (`00082`) and the portal root list (stage 6) on the Client side. [CB-G6](https://github.com/markgoho/doula-cloud/issues/312) is closed |

**Left alone on purpose.** Stage 3's CB-G1 row still tells the pre-ADR-0017 story and is held at [#1236](https://github.com/markgoho/doula-cloud/issues/1236), a parallel ticket correcting the same map's stale rows. The walk logs above are records of what was seen on the day and are untouched. The run-status table in [docs/test-plans/README.md](README.md) is recounted from this plan's Steps table, Total with it, to 40 / 158 / 0 / 44.
