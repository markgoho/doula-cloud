# Nadia Haddad — test plan

- **Journey**: [loss-client.md](../journeys/loss-client.md)
- **Persona**: [loss-client.md](../personas/loss-client.md)
- **A pass means**: the record is accurate, Maya can still reach her, and no screen, label, or link addresses her as a pregnant person. The first run (2026-08-23) could not pass it: there was nowhere to record the loss. The second run (2026-10-28) can, and its Run log says how far it gets.

Written first, before the two other client-side plans, because the method standard says to walk Nadia before any journey hers overlaps.

Her journey turns mid-way, so half this plan is walked as Maya on the staff side and observed from Nadia's screen. Those steps are marked as her map marks them — the stage belongs to her even where the click does not.

## Preconditions

- A solo Practice (Owner + Admin + Doula in one person) with one Client and one Engagement carrying a **signed** Contract, a **filled** Birth Plan Instance, several Visits, and a message thread with traffic both ways.
- **The Engagement cannot be aged.** Status now moves (**MO-G4** is closed), but `created_at` is set when the Engagement is created and the stack cannot move the clock, so "four months in" cannot be represented. Run against a fresh Engagement and treat it as the four-month one; Visits can be backdated, so add two or three in the past.
- Stage 2 is Maya recording the loss, so do **not** record it beforehand.
- Stage 6 needs a **voided** Contract. Sign it as the Client first, then, as Maya, press **Void this Contract** on the signed Contract and confirm it. This is Dee's step 7.1-a; see the crossing note there.
- Two browser sessions or two profiles: Nadia in the portal, Maya on the staff side, both open at once. Stages 2, 3 and 7 need them side by side.

## Steps

### Stage 1 — A live Engagement, four months in

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Sign in at `/portal/login` | `GET /api/portal/session` resolves one Engagement and lands on `/portal/engagements/[engagementId]` | `automated (client-portal-login.e2e.ts)` |
| 1.2 | Read the thread and send a message | One continuous thread on **Messages**, its own page, in order, immutable (ADR-0002), with no edit or delete control. Attachments upload both ways; each is a download button in the thread | `manual` |
| 1.2-a | Find anything on screen saying this care is four months old | No status and no **Created** line. **Your care** says "Your due date is {day}." under "What happens next", and **Your visits** lists past Visits by day. The tab title reads "Your care — {Practice}, started {created day}", and **Show what has happened** opens with "Your care was set up." The Client record's own details (ADR-0017) are Staff-only | `manual` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 4 (Hick's Law, Flow). `/portal/login`, the sign-in link page (**Continue**), **Your care**, then **Messages** (NH-G10, [#2215](https://github.com/markgoho/doula-cloud/issues/2215)) | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 5 (Hick's Law, Miller's Law). **Your care**, each headed group one decision: Your birth plan, Your contract, Messages, What you still owe, Everything that has happened (NH-G11, [#2216](https://github.com/markgoho/doula-cloud/issues/2216)) | `manual` |
| 1.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 2 — The loss; the Practice tries to mark it

Walked as Maya. Nadia is not in the app.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Record **what happened** against the Engagement | The Engagement page's **Birth outcome** section: **Record what happened**, three answers ("The baby was born alive", "The pregnancy ended without a living baby", "The Practice never learned what happened"), then "When did the pregnancy end?" as Month, Day, Year, and **Record this outcome**. It then reads "What happened" and "Date the pregnancy ended", with **Correct what was recorded** and **Remove this record**. Staff-only (ADR-0015, #944) | `manual` |
| 2.1-a | Choose a status that is true after a loss | There is no status to choose: the loss is the birth outcome, not a status value (ADR-0015). The care phase stays **Active**, which is true while bereavement care goes on | `manual` |
| 2.1-b | Set the status to any value at all | **Care phase** has **Change**, and **Mark care complete** asks why the care is ending (stage 8). The first scheduled Visit had already moved it from Intake to Active | `manual` |
| 2.1-c | Read the rest of the Engagement page after the outcome is recorded | Visits after the day the pregnancy ended retype to **Postpartum** (ADR-0015 derives the type); the earlier ones stay **Prenatal**. The **On call** section reads "The baby arrived before the window would have opened." ([#2220](https://github.com/markgoho/doula-cloud/issues/2220)). The Birth Plan section and **Print for the hospital** stay on the Staff side | `manual` |

### Stage 3 — Three weeks of not opening the portal

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | With her thread open, have Maya send a message | The tab refetches and the message appears. The push itself carries no content (ADR-0002) — asserted at the in-app fetch level, which is where the map draws the transport boundary | `automated (push-notification.e2e.ts)` |
| 3.1-a | Close the tab and repeat | Nothing arrives carrying content, and no email: the catcher holds only the sign-in links she asked for. Real device delivery is out of scope for this effort | `manual` |
| 3.2 | Wait, and watch for a reminder, a nudge, or a countdown | **Nothing happens**, and that is the finding. Visits now carry a date (**[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)** closed), but no Client reminder exists in `api/` (the only reminders are Practice deletion's and the Connect nudge, both to Staff), and the only push fires on a Message | `manual` |

### Stage 4 — She opens the portal — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/portal/engagements/[engagementId]` after three weeks away | `GET /api/portal/engagements/{id}` returns `practiceName`, `status`, `createdAt` and the page renders | `automated (client-portal-login.e2e.ts)` |
| 4.2 | Read the `<h1>` | **Your care**, the register's literal for every Client, with the Practice's name under it ([NH-G4](https://github.com/markgoho/doula-cloud/issues/296) closed). No greeting | `manual` |
| 4.2-a | Confirm the greeting is not gated on a first visit | The spec re-asserts the same `<h1>` on a fresh page in the same session. That proves it is not visit-gated; it does not prove what three weeks look like, which is 4.2's job | `automated (client-portal-login.e2e.ts)` |
| 4.2-b | Read the status line and the date under it | No status line and no created date. "What happens next" names the next booked Visit, "{Doula} is coming to you on {day, time}.", and after a loss no due date (#2003) | `manual` |
| 4.3 | Read what is offered below | **Your visits**, **Your contract**, **Messages** with the newest message quoted, **What you still owe** with **Invoices**, and **Everything that has happened** behind a closed disclosure. No Birth Plan card after a loss. The top bar: Your care, Messages, Contract, Notifications, and the account menu | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). The Engagement she lands on, with nothing to pass through first | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 5 (Hick's Law, Miller's Law). The portal's five destinations; nothing on the screen asks her anything | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Nothing asks her to recall a date or a stage of the pregnancy | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen says where her care stands now, in words that are true after a loss | `manual` |
| 4.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User). Three weeks away, she meets it as if for the first time | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 5 — The Birth Plan will not go away

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Look at the portal home without tapping anything | Once the loss is recorded, no **Birth plan** in the top bar and no **Your birth plan** card: `offersBirthPlan` is false. Before it, both were there | `manual` |
| 5.1-a | Retire, hide, archive or dismiss it — from her side or Maya's | Nobody has to: the suppression is derived from the birth outcome on every read (`engagement.OffersBirthPlan`), with no control and no stored flag ([NH-G2](https://github.com/markgoho/doula-cloud/issues/294) closed). The Plan Instance is untouched on the Staff side. Maya's Birth Plan section still shows the plan, its Client row reads "Not shown to Nadia", and **Print for the hospital** is gone; with no plan started, the section reads "No Birth Plan. The birth outcome is recorded." with no **Start the Birth Plan**, and the BFF refuses a start (`409`) ([#2317](https://github.com/markgoho/doula-cloud/issues/2317)) | `manual` |
| 5.1-b | Open **Show what has happened** | The ledger lists no Birth Plan row once the loss is recorded: the portal reader leaves every Birth Plan action out wherever `offersBirthPlan` is false (PR #2316). It listed "Your Birth Plan was updated." and "Your Birth Plan was started." before ([#2219](https://github.com/markgoho/doula-cloud/issues/2219)) | `manual` |
| 5.2 | Open the Birth Plan URL directly | The portal's ordinary not-found page, "Page not found", with **Go to your care**. The read, the PDF and the acknowledgment endpoints answer `404` | `manual` |
| 5.2-a | Press **Print** | Nothing to press: the Birth Plan page is not offered after a loss | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). Since #294 there is no Birth Plan to put away or print; counted on the screen she is on | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). Since #294, nothing is put away and the portal says nothing of the Birth Plan, by decision ("no nav item, no link, and no empty state"); the stage ends on **Your care**, which says where her care stands | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 6 — Money, and the word "voided"

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Contract link | The full signed prose renders. A `voided` Contract is not withheld; only the **Sign** form is, and only because it keys on `status === 'sent'`. The seeded template's line reads "Engagement dates: {ISO day} through {ISO day}" ([#2222](https://github.com/markgoho/doula-cloud/issues/2222)) | `manual` |
| 6.2 | Read the status | **No longer active**, then "{Practice} ended this Contract.", the same line on the **Your contract** card The portal stopped reusing the Staff component's raw enum: the label and the notice both come from the Client register (`app/src/lib/clientRegister.ts`), which is the one place either wording is decided, and the notice names the Practice rather than leaving her with the ledger's word ([NH-G5](https://github.com/markgoho/doula-cloud/issues/212) closed) | `manual` |
| 6.2-a | Keep a copy of what she signed | **Download signed Contract (PDF)** is offered on her voided Contract, and pressing it fetches the signed-PDF route, which serves what she signed ([#299](https://github.com/markgoho/doula-cloud/issues/299)). Whether the downloaded file reads correctly is what the walk is for; the mark is about the step being performable at all. The control is gated on the Contract read's own `hasSignedPdf`, which the Go BFF fills from the same lookup the signed-PDF route streams from ([#1119](https://github.com/markgoho/doula-cloud/issues/1119) closed) — so the screen asks the endpoint's own question rather than a second reading of `status` that drifted away from it. A Contract that was never signed still offers nothing, and the endpoint still 404s for it ([HS-G3](https://github.com/markgoho/doula-cloud/issues/302), [NH-G8](https://github.com/markgoho/doula-cloud/issues/299) closed). Below the prose the file reads "Signed electronically by: {the name she typed}", "Signed on: {day}, {time} {zone}" in the Practice's zone, and "Confirmed: I have read this contract and I am signing it electronically", and the Practice-side **Download signed Contract (PDF)** serves the same file ([NH-G9](https://github.com/markgoho/doula-cloud/issues/2214) closed) | `manual` |
| 6.2-b | Find what she still owes, or what was refunded, on a Stripe Invoice | **Your care**'s "What you still owe" reads Total to pay $0.00, beside the Contract card. **Invoices**: "You have paid every Invoice for this care. {amount} of it was returned to you.", and the row reads "Paid — {amount} returned to you", the same on the Invoice's own page. Its Sent and Paid days are read in two zones ([#2205](https://github.com/markgoho/doula-cloud/issues/2205)) | `manual` |
| 6.2-c | The same, on a bill that did not come from Stripe | No such bill can reach the portal (#946). Exempt from the run's gate on 2026-10-27 (#329) | `missing-feature (NH-G6)` [#297](https://github.com/markgoho/doula-cloud/issues/297) |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The Contract, then what she still owes or was refunded | `manual` |
| 6.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). What she signed and what she owes are on screen together; nothing has to be carried from one page to the next | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 7 — Postpartum support continues anyway

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | As Maya, log a bereavement Visit | The row now carries a scheduled date and time, a type and notes (**[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**, **[PR-G6](https://github.com/markgoho/doula-cloud/issues/281)**, **[MO-G2](https://github.com/markgoho/doula-cloud/issues/251)** all closed), so it is no longer indistinguishable from the prenatal ones. The type is not chosen, though — ADR-0015 derives it from when the Visit falls against the end of the pregnancy, and a bereavement Visit types as `postpartum` for the same reason postpartum care after a loss is still postpartum. That is the honest word the model holds, and it is not the word for what Maya went to do | `manual` |
| 7.1-a | As Nadia, find any trace of that Visit | **Your visits** lists it by day, time and Doula, and "What happens next" names it when it is the next one ([#478](https://github.com/markgoho/doula-cloud/issues/478)). No type and no notes reach her, in any wording, so nothing calls it prenatal or postpartum. Her history reads "A visit was added to your care." and "Notes on a visit were updated." | `manual` |
| 7.2 | Continue the thread both ways | Unchanged and unchangeable — immutable by design, which is correct here | `manual` |
| 7.2-a | Mark that the thread's subject has changed, or pause push | **Notifications** turns push on and off from her side ([NH-G7](https://github.com/markgoho/doula-cloud/issues/298) closed as a duplicate of #303). No subject-change control, by design: Messages are immutable | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). Message, Attachment and **Send**; the optional Attachment counts ([#2197](https://github.com/markgoho/doula-cloud/issues/2197); NH-G11, [#2216](https://github.com/markgoho/doula-cloud/issues/2216)) | `manual` |
| 7.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 7.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). After **Send**, her reply is last under "You" and a status reads "Message sent to {Practice}. A reply will show here. Do not rely on messages for anything urgent.", with focus kept on **Send**. Offline, the send is refused with "We could not reach the service. Check your connection and try again." and the draft stays ([HS-G10](https://github.com/markgoho/doula-cloud/issues/2210), ADR-0059) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 8 — The record closes without erasing her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Close the Engagement truthfully | As Maya, **Mark care complete** asks "Why is this Engagement ending?" with six reasons and an optional note; with the loss already recorded as the outcome, "The work finished as agreed" is true (ADR-0015: the loss is an outcome, not an ending). Care phase reads **Completed**, "The care phase is now Completed." The portal stays reachable, and her history reads "Your care ended." | `manual` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). Her portal says her care has ended, in words that are true after a loss, and keeps the record | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

8.1 is 2.1-a and 2.1-b met a second time, from her side rather than Maya's. Both stages are kept because the map keeps both. On the first run the same absence was hit twice by two different people; on the second, both acts exist.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 10 (6 of them [budget steps](README.md#budget-steps)) |
| `manual` | 45 (20 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 1 ([NH-G6](https://github.com/markgoho/doula-cloud/issues/297)) |

No step is `blocked`. The Practice's Stripe account is set up on the Practice side, so nothing on her side waits on infrastructure. 6.2-c, the one `missing-feature` step, is a bill that did not come from Stripe, exempt from the run's gate (#329, 2026-10-27).

NH-G4, NH-G5, **MO-G1**, **MO-G2**, **PR-G6** and [#212](https://github.com/markgoho/doula-cloud/issues/212) are observed inside walkable steps (4.2, 6.2, 7.1, 4.2-b) rather than given steps of their own: the step can be performed, and what it hands back is the finding.

Her automated steps all come from specs written for a happy path. On the first run `client-portal-login` greeted her, `push-notification` reached her and `birth-plan` showed her the document, and the suite could not tell her journey from Hannah's. The heading is now true for both, and 5.2 left `birth-plan.e2e.ts` on the second run because no spec records a loss: what the portal does after one is walked, not asserted.

## Run log

### 2026-11-01 — the fixed steps walked again ([#2264](https://github.com/markgoho/doula-cloud/issues/2264))

**Stack**: `SEED_DEMO=0 bun run --cwd app dev:full` from a worktree at port offset 1 (the app at `http://localhost:5273`, the mail catcher at `http://127.0.0.1:19180`), on trunk at `3004697e`, in WSL2 with rootless Podman. No Stripe step was needed: no Invoice was raised, and none of the steps below needs one.

**Accounts and preconditions**: `seed:staff-session` made Maya Okonkwo's Practice, **Okonkwo Birth Support** (zone `America/New_York`), with Maya as Owner, Admin and Doula. As Maya, at `/login`, the orchestrating agent set a $1,800 birth rate and added Nadia Haddad (`nadia.haddad@sim.doula.cloud`), with birth work, Maya as the Doula, due Feb 15, 2027. Maya sent the portal invite, which Nadia accepted from its link. Maya sent a Contract, and Nadia signed it in the portal at 17:23 UTC on Nov 1, 2026, typing "Nadia Haddad". Maya filled the Birth Plan (three answers) and added Visits on Oct 6, Oct 20 and Nov 12, 2026, the first two backdated. Maya then sent one message. After walk 1, Maya recorded "The pregnancy ended without a living baby", dated Oct 25, 2026. She sent a condolence message and pressed **Void this Contract**. After walk 2, she pressed **Mark care complete** with "The work finished as agreed", leaving the Nov 12 Visit booked. For 5.1-a's no-plan case, a second Client, Rana Saleh, got birth work and a loss recorded with no Birth Plan started.

**How it was walked**: by the blind `persona-walker` and the sighted `walk-evaluator` ([`docs/agents/persona-walk.md`](../agents/persona-walk.md)), as three walks, each with its own profile. Her persona file names no device. #2264 asks for phone width, so walks 1 and 2 ran at `390x844x3,mobile,touch`. Stage 8's note asks for 320px and desktop, so walk 3 took the 320px floor.

- **Walk 1**: stage 1, before the loss. Signed in from `/portal/login` with a link from the catcher. 9 acts.
- **Walk 2**: stages 5, 6 and 7, after the loss and the void, passing through stage 4 on the way in. The network went Offline partway through stage 7. 12 acts.
- **Walk 3**: stage 8 at `320x640x2,mobile,touch`, after **Mark care complete**. 9 acts.

Every walk was pointer input with no throttling. The first try at walk 3 moved down the page with a script, so its last three acts had no input to time, and its evaluator found three `u` entries. It was not admissible ([`persona-walk.md`](../agents/persona-walk.md), section 4), so stage 8 was walked again with **PageDown**. The numbers below are from the second try, and the evaluators found no `u` entry in it or in walks 1 and 2.

Between the two tries, the dev server stopped answering page requests: running the spelling spec regenerated `.svelte-kit` under it. Only the dev server was restarted, against the same database and the same BFF, and the mail was drained by hand on the same 3-second interval.

The orchestrating agent made the reads the blind walker could not make, in separate isolated contexts:

- both PDFs, fetched in the signed-in page and read with `pdftotext`;
- **Your care** at 1280x800;
- **Your care** again at both widths after Maya cleared the Nov 12 Visit's time;
- the 320px sweep of **Your care**, **Messages** and the Contract.

A Visit cannot be deleted. A Visit's status is parked with the richer time model ([#330](https://github.com/markgoho/doula-cloud/issues/330), `GLOSSARY.md`, Visit). So Maya took it off what is booked by clearing its scheduled time, which turns it into "Not yet scheduled". She did that after the first try at walk 3 and put the time back before the second.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 5.1-a | `manual` | as expected, cell corrected | With the plan filled, after the loss, Maya's Birth Plan section reads "Not shown to Nadia" and offers no **Print for the hospital**. On Rana's Engagement, with no plan started, the section turned to "No Birth Plan. The birth outcome is recorded." without a reload, with no **Start the Birth Plan**. `POST .../plans/birth_plan` answered `409`, "A Birth Plan can't be started now: the birth outcome is recorded." |
| 5.1-b | `manual` | as expected, cell corrected | **Show what has happened** listed ten rows, newest first, from "Your contract is no longer active." to "Your care was set up.". No row names the Birth Plan ([#2219](https://github.com/markgoho/doula-cloud/issues/2219) holds closed) |
| 6.2-a | `manual` | as expected, cell corrected | **Download signed contract (PDF)** on her voided Contract: `GET .../contract/pdf` `200`, `application/pdf`, 7,912 bytes. Below "By signing below, both parties agree to the terms described above." it reads "Signed electronically by: Nadia Haddad", "Signed on: Nov 1, 2026, 12:23pm EST" (17:23 UTC in the Practice's zone, after that morning's change from daylight time) and "Confirmed: I have read this contract and I am signing it electronically". The Practice-side download is byte for byte the same file (SHA-256 `31ca2483…`) ([NH-G9](https://github.com/markgoho/doula-cloud/issues/2214) closed). Headless Chrome shows no download, so her screen said nothing after the press |

#### Budget steps — measured

Decisions are counted as on the 2026-10-28 walk, on a quiet day ([#2570](https://github.com/markgoho/doula-cloud/issues/2570)). The count is what is enabled and in view in the page body, with each headed group on **Your care** as one decision (the hub rule). The top bar, the account menu, the pilot feedback link, the skip link and the Privacy Policy link are not counted.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 4 | **4** at 390x844: `/portal/login`, the sign-in link page (**Continue**), **Your care**, **Messages**. The three catcher pages are outside the product | **yes**, [NH-G10](https://github.com/markgoho/doula-cloud/issues/2215) |
| 1.B-decisions | 5 | **5** at 390x844 on **Your care**: Your birth plan, Your contract, Messages, What you still owe and Everything that has happened. "What happens next" and **Your visits** hold no control. Messages 3, the sign-in form 2 | **yes**, [NH-G11](https://github.com/markgoho/doula-cloud/issues/2216) |
| 7.B-decisions | 3 | **3** at 390x844: Message, Attachment, **Send** | **yes**, [NH-G11](https://github.com/markgoho/doula-cloud/issues/2216) |
| 7.B-confirmation | yes | **yes** at 390x844. After the loss, "Thank you, Maya." went last under "You" at "Nov 1, 2026, 5:36pm", with "Message sent to Okonkwo Birth Support. A reply will show here. Do not rely on messages for anything urgent." The box emptied and focus stayed on **Send**. Nothing in it assumes a pregnancy. **Offline**: "There is a problem", "We could not reach the service. Check your connection and try again.", focused. Her draft "Are you still coming on the 12th?" stayed in the box, nothing was added to the thread, and the sent line was gone. The status role was not read again here; it is the same component Hannah Sorensen's walk read as `role="status"` that day | **yes**, [HS-G10 (#2210)](https://github.com/markgoho/doula-cloud/issues/2210) built |
| 8.B-confirmation | yes | **yes**. With the Nov 12 Visit booked, "What happens next" reads **Care ended**, then "Maya Okonkwo is coming to you on Thu Nov 12, 3:00pm." It reads that at 320x640 (walk 3) and at 1280x800. Once Maya cleared the Visit's time, it reads **Care ended** alone at both widths, and **Your visits** lists only Oct 20 and Oct 6. No due date and no Birth Plan. The Contract card reads "Okonkwo Birth Support ended this contract." | **yes**, [NH-G12](https://github.com/markgoho/doula-cloud/issues/2217) built |

**320px**: **Your care** (before and after **Mark care complete**), **Messages** and the Contract page each read `scrollWidth` 320 at 320x640: no sideways scroll. The 1.B and 7.B numbers are from 390x844, and the 8.B reading is from both 320x640 and 1280x800.

**Re-marked**: none. The cells of 5.1-a, 5.1-b, 6.2-a and 7.B-confirmation are corrected to what the screens do. **Failed**: none.

**What she said, in the walker's words**:

- On **Your care** after the loss: "It doesn't ask me anything."
- On stage 8's two lines: "'Care ended', and then right under it Maya is coming on the 12th. Which is it?" On the first try she said "I can't tell whether it's over or not".

The second is what NH-G12 built on purpose: completing care ends no Visit (ADR-0015), so a booked Visit is still named. The words on the screens are fixed wording, true for every Client, except two lines people wrote, which stay as written: her signed Contract's "continuous support in labor" and Maya's earlier "Bring any questions about the birth plan." in the thread, which cannot be edited (ADR-0002).

**Findings filed**: [#2587](https://github.com/markgoho/doula-cloud/issues/2587). **Your care** reads `GET /api/portal/engagements/{id}` twice on every load, once through `window.fetch` inside the layout's `load`, which SvelteKit warns about in the console.

### 2026-10-28 — second manual walk ([#2173](https://github.com/markgoho/doula-cloud/issues/2173))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, on trunk at `1cc382b9`, in WSL2 with rootless Podman. `scripts/stripe-listen.sh` was running against it, Stripe in the Sandbox with a `sk_test_` key and the publishable key set. Not restarted.

**Device**: headless Chrome inside WSL, driven by the chrome-devtools MCP, all in UTC, at desktop size, as the first walk. Three isolated browser contexts: one for Maya, one for Nadia's stage 1, and a fresh one for Nadia from stage 4 on, which is the Cold walk. Every timing below is on an unbundled Vite dev server, which compiles a route the first time it is visited; first loads are slower and are not read as findings.

**Accounts and preconditions**: Maya Okonkwo's Practice, **Okonkwo Birth Support** (`5c88b7f9-…`), as #2171 and #2172 left it: card billing through Stripe on an active connected account. Nadia Haddad (`nadia.haddad@sim.doula.cloud`, Client `6ba03d2a-…`, Engagement `2b38f9c2-…`, birth, due Feb 15, 2027) came with a signed Contract ($1,800), a filled Care Plan and Birth Plan (not marked read), two scheduled prenatal Visits (Nov 12, 2026 and Jan 18, 2027), one message each way, care phase Active, no birth outcome, and Invoice `7ETF7DK3-0001` ($1,800) already paid on Stripe's hosted page. Before stage 1, Maya added two backdated Visits (Oct 6 and Oct 20, 2026), since the Engagement itself cannot be aged. Every Practice-side act was done as Maya, signed in at `/login`, through the product: the two Visits, a reply with an attachment, the outcome (a loss, the pregnancy ended Oct 25, 2026), a message after the loss, **Void this Contract**, **Return money** ($1,350 of the $1,800, through Stripe), a bereavement Visit on Nov 2, 2026 with notes, and **Mark care complete**. Nadia signed in by magic link from the catcher each time.

The 10 `automated` steps were **not** re-run. 4.1 and 4.2-a were walked anyway, since stage 4 opens on them.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.2 | `manual` | as expected, cell corrected | **Messages** from the top bar in 54 ms: the thread, in order, with no edit or delete control. She sent "Here is the scan letter from the clinic…" with `clinic-letter.png`; Maya replied with `visit-list.pdf`. After a reload, each side's file was a download button and fetched `200` |
| 1.2-a | `manual` (was `missing-feature (MO-G3)`) | **walked** | Before the loss **Your care** said "Your due date is Feb 15, 2027." and listed the two backdated Visits by day. No status line, no **Created** line. The tab title reads "Your care — Okonkwo Birth Support, started Oct 27, 2026", and her history opens with "Your care was set up." Nothing can say four months: the Engagement was created yesterday |
| 2.1 | `manual` (was `missing-feature (NH-G3)`) | **walked** | **Record what happened**, "The pregnancy ended without a living baby" ("Care after a loss continues on this same Engagement."), "When did the pregnancy end?" 10 / 25 / 2026, **Record this outcome**: "What happened: The pregnancy ended without a living baby", "Date the pregnancy ended: Oct 25, 2026" |
| 2.1-a | `manual` (was `missing-feature (NH-G1)`) | **walked** | No status is asked for: the loss is the outcome. Care phase stays **Active** while bereavement care goes on |
| 2.1-b | `manual` (was `missing-feature (MO-G4)`) | **walked** | **Change** on Care phase offers **Mark care complete** (8.1). The first scheduled Visit had moved it from Intake to Active on #2171 |
| 2.1-c | `manual` (new) | **walked** | After a reload the Nov 12 and Jan 18 Visits read **Postpartum**, the Oct ones **Prenatal**; the Nov 12 row keeps its notes, "First prenatal visit at home. Went through the birth plan and the hospital bag list." **On call** changed from "On call from Jan 25, 2027 to Mar 1, 2027." to "**The baby arrived before the window would have opened.**" ([#2220](https://github.com/markgoho/doula-cloud/issues/2220)). Due still reads Feb 15, 2027 on the Staff side |
| 3.1-a | `manual` | as expected, cell corrected | With her page closed, Maya sent "Nadia, I am so sorry. There is nothing you need to do here. I am around whenever you want to talk, and I will check in again next week." The catcher held no new mail for her. Headless Chrome grants no push permission, so no device delivery was observed |
| 3.2 | `manual` | as expected, cell corrected, confirmed by inspection | Not waited out. `api/` holds no Client reminder: its reminders are Practice deletion's and the Connect nudge, both to Staff |
| 4.1 | `automated (client-portal-login.e2e.ts)` | as the spec asserts | In a fresh context: `/portal/login`, the link from the catcher, "You are signing in as nadia.haddad@sim.doula.cloud." and **Continue**, then **Your care** |
| 4.2 | `manual` | as expected, cell corrected | `<h1>` **Your care**, then "Okonkwo Birth Support". No greeting ([NH-G4](https://github.com/markgoho/doula-cloud/issues/296) holds closed) |
| 4.2-b | `manual` | **better than the plan claimed**, cell corrected | No status and no created date. "What happens next: Maya Okonkwo is coming to you on Thu Nov 12, 3:00pm." and no due date. For the first 34 ms it read "No visit is booked yet." and "Nothing is booked yet." ([#2218](https://github.com/markgoho/doula-cloud/issues/2218)) |
| 4.3 | `manual` | **better than the plan claimed**, cell corrected | Your visits (five), Your contract, Messages quoting Maya's condolence, What you still owe ($0.00) with **Invoices**, and the closed **Show what has happened**. No Birth Plan card; the top bar is Your care, Messages, Contract, Notifications |
| 5.1 | `manual` | **falsified**, cell corrected | After the loss: no **Birth plan** in the top bar, no card. Before it, both were there |
| 5.1-a | `manual` (was `missing-feature (NH-G2)`) | **walked** | Nothing to press on either side; the portal's Engagement read carries `offersBirthPlan: false` and no outcome field. Maya's Birth Plan section is unchanged |
| 5.1-b | `manual` (new) | **walked** | **Show what has happened** still ends "Your Birth Plan was updated.", "Your Birth Plan was started." ([#2219](https://github.com/markgoho/doula-cloud/issues/2219)), beside "Your login was created." ([#2148](https://github.com/markgoho/doula-cloud/issues/2148), open) |
| 5.2 | `manual` (was `automated (birth-plan.e2e.ts)`) | **walked** | `/portal/engagements/{id}/birth-plan`: "Page not found", "Check the web address is correct, or the page may have been removed. Trying again will not change that.", **Go to your care**. `GET .../birth-plan` and `.../birth-plan/pdf` `404` |
| 5.2-a | `manual` | **falsified**, cell corrected | No Print to press |
| 6.1 | `manual` | as expected, cell corrected | After Maya's **Void this Contract** and its confirm ("Voiding ends this Contract for good. Nothing more can be invoiced under it, and a new Contract starts again from a Draft."): the full prose, including "Engagement dates: 2026-10-27 through 2027-04-15" ([#2222](https://github.com/markgoho/doula-cloud/issues/2222)) and "continuous support in labor", which is what she signed |
| 6.2 | `manual` | as expected | **No longer active**, "Okonkwo Birth Support ended this Contract." The **Your contract** card says the same and its link reads **Read your contract** |
| 6.2-a | `manual` | **falsified in part** | **Download signed Contract (PDF)**: `GET .../contract/pdf` `200`, `application/pdf`, 1,351 bytes. Its text is the prose and nothing else: no name, no date, nothing below "By signing below" ([NH-G9](https://github.com/markgoho/doula-cloud/issues/2214)) |
| 6.2-b | `manual` (was `missing-feature (NH-G6)`) | **walked** | Before the refund: "Total to pay $0.00", "You have paid every Invoice for this care.", `7ETF7DK3-0001` $1,800.00 **Paid**. Maya then returned $1,350 ("Stripe sends this money back to the Client's card…", **Confirm and return money**, 3,426 ms). After a reload: "You have paid every Invoice for this care. $1,350.00 of it was returned to you." and "Paid — $1,350.00 returned to you"; the Invoice page reads Sent Oct 28, 2026, Paid Oct 27, 2026 ([#2205](https://github.com/markgoho/doula-cloud/issues/2205), open). Her history adds "Money was returned to you." Maya's Invoices section kept "This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract." beside the refund ([#2221](https://github.com/markgoho/doula-cloud/issues/2221)) |
| 6.2-c | `missing-feature (NH-G6)` [#297](https://github.com/markgoho/doula-cloud/issues/297) | confirmed | New step, splitting 6.2-b along the #329 exemption: a bill that did not come from Stripe cannot reach the portal (#946) |
| 7.1 | `manual` | as expected | A Visit on Nov 2, 2026, 3:00pm, then **Change** and notes, "Bereavement visit at home…": typed **Postpartum**, as the cell says |
| 7.1-a | `manual` | **falsified**, cell corrected | **Your visits** lists "Mon Nov 2, 3:00pm, Maya Okonkwo", and "What happens next" names it. No type or notes reach her |
| 7.2 | `manual` | as expected | Her reply, "Thank you for Monday, Maya. It helped to talk.", in 31 ms |
| 7.2-a | `manual` (was `missing-feature (NH-G7)`) | **walked** | **Notifications**: "This device can tell you when a message is waiting. Do not rely on it for anything urgent.", "Notifications are currently off.", **Turn on notifications** |
| 8.1 | `manual` (was `missing-feature (NH-G1)`) | **walked** | **Mark care complete**, "Why is this Engagement ending?", six reasons; "The work finished as agreed" with the note "Bereavement support finished…", **Confirm completion**: "The care phase is now Completed." in 35 ms. Nov 2, Nov 12 and Jan 18 were left booked, since the clock cannot move to after them |

**Re-marked**: 1.2-a, 2.1, 2.1-a, 2.1-b, 5.1-a, 6.2-b, 7.2-a and 8.1 from `missing-feature` to `manual`, their gaps built. 5.2 from `automated (birth-plan.e2e.ts)` to `manual`: the spec drives a Client with no outcome, and on her walk the link is gone. New: 2.1-c and 5.1-b, the two checks the plan had no row for, and 6.2-c, which keeps the off-Stripe half of 6.2-b `missing-feature (NH-G6)` as the #329 exemption says. The Marks summary is recounted to 10 / 45 / 0 / 1, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**, as on the earlier second-run walks: fields, options, and links or buttons that move her somewhere or change what happens, enabled and in view at once in the page body. **Back**, the skip link, the Privacy Policy link and an attachment's own download button in the thread are not counted. The portal's top bar after the loss holds four destinations and the account menu. **Time** is measured in the page: a script presses the control and polls each animation frame until the result has settled on screen, and records when the control first showed it was busy.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 2 | **4**: `/portal/login`, the sign-in link page (**Continue**), **Your care**, **Messages** | **no**, [NH-G10](https://github.com/markgoho/doula-cloud/issues/2215) |
| 1.B-decisions | 2 | **5** on **Your care** (Read your birth plan, Read what you signed, Open your messages, Invoices, Show what has happened); 3 on Messages; 2 on the sign-in form | **no**, [NH-G11](https://github.com/markgoho/doula-cloud/issues/2216) |
| 1.B-time | 400 ms, 100 ms | **Continue** to **Your care** 164 ms, busy within 2 ms; Messages 54 ms; **Send** with a photo 71 ms, busy within 10 ms; Your care from the top bar 109 ms | yes |
| 4.B-screens | 1 | 1: **Your care**, with nothing to pass through once signed in | yes |
| 4.B-decisions | 5 | 4 in the body (Read your contract or Read what you signed, Open your messages, Invoices, Show what has happened); the top bar's 4 destinations | yes |
| 4.B-memory | yes | yes. Nothing asks her for a date or a stage of the pregnancy; nothing names one | yes |
| 4.B-time | 400 ms, 100 ms | **Send me a sign-in link** 35 ms, busy within 2 ms; **Continue** to the settled page 194 ms (busy within 2 ms on stage 1's press) | yes |
| 4.B-confirmation | yes | yes. "Your care", the Practice, who is coming and when, the Contract, what she owes. No due date, no Birth Plan, no greeting. For its first 34 ms (575 ms on Slow 4G) it says nobody is coming ([#2218](https://github.com/markgoho/doula-cloud/issues/2218)) | yes |
| 4.B-cold | yes | yes, in a fresh context: the sign-in page says what it sends, the link page says who she is signing in as | yes |
| 5.B-screens | 1 | 1: **Your care**; no Birth Plan screen is offered | yes |
| 5.B-decisions | 2 | 0 Birth Plan decisions; nothing asks her to put it away | yes |
| 5.B-confirmation | yes | yes, by #294's decision: nothing names the Birth Plan on the screen she is on. Her closed history still does ([#2219](https://github.com/markgoho/doula-cloud/issues/2219)) | yes |
| 6.B-screens | 2 | 2: the Contract, then **Invoices**, each one tap from **Your care** | yes |
| 6.B-memory | yes | yes. **Your care** holds the Contract's status and "Total to pay" together | yes |
| 7.B-screens | 1 | 1: **Messages** | yes |
| 7.B-decisions | 2 | **3**: Message, Attachment, **Send** | **no**, [NH-G11](https://github.com/markgoho/doula-cloud/issues/2216) |
| 7.B-time | 400 ms, 100 ms | **Send** 31 ms, busy within 3 ms; Maya's **Save Visit** 78 ms, busy within 3 ms | yes |
| 7.B-confirmation | yes | **no**. Her reply appears under "You"; nothing says it was sent, and the live regions stay empty | **no**, [HS-G10 (#2210)](https://github.com/markgoho/doula-cloud/issues/2210), cited |
| 8.B-screens | 1 | 1: **Your care** | yes |
| 8.B-confirmation | yes | **no**. After **Mark care complete**, **Your care** reads as before: "Maya Okonkwo is coming to you on Mon Nov 2, 3:00pm." Only the closed history says "Your care ended." | **no**, [NH-G12](https://github.com/markgoho/doula-cloud/issues/2217) |

The 320px budget steps are `automated (portal-320.e2e.ts)` or `automated (route-continuum.svelte.spec.ts)` and were not re-measured. **Failed**: 1.B-screens, 1.B-decisions, 7.B-decisions, 7.B-confirmation, 8.B-confirmation.

#### Moment of truth — the verdict, against the first run's

**Stage 4, the first screen after three weeks away.** First run (2026-08-23): "Welcome to Willow Creek Doula Care", `Status: intake`, a **Created** date, and **Birth Plan** second from the top. Every one of the three pain points on the map was on the screen. **This run: the screen is true after a loss.** She meets "Your care" and her Practice's name, who is coming next and when, her Contract, a thread whose last line is Maya's condolence, and $0.00 to pay. No greeting, no status, no due date, no Birth Plan, and no portal response carries the outcome. Two things fall short. For a moment as the page loads, it tells her nobody is coming, which on a slow connection is half a second of the wrong thing ([#2218](https://github.com/markgoho/doula-cloud/issues/2218)). And one disclosure below, her history still begins with "Your Birth Plan was started." ([#2219](https://github.com/markgoho/doula-cloud/issues/2219)). The first run's screen spoke to her as a pregnant person. This one does not, except in a line she has to open.

#### Is the journey completable end to end?

**Yes.** Against "a pass means": the record is accurate (the loss and its date are recorded on the Engagement, later Visits type as postpartum, and the care closes with a true reason); Maya can still reach her (the thread both ways, the bereavement Visit on her list, Notifications hers to turn off); and no screen, label or link she is shown addresses her as a pregnant person. Every `missing-feature` step from the first run but the exempt off-Stripe bill (6.2-c) is walked. Three things fall short of the letter: two lines in the closed history name her Birth Plan (#2219); the signed PDF she keeps does not show she signed it (NH-G9); and when her care ends, her first screen does not say so (NH-G12). None of them stops her.

**Copy a grieving reader meets**, read against the brief's Voice section and the map's experience layer. On her side: "Your Birth Plan was started." and "Your Birth Plan was updated." (#2219), "No visit is booked yet." while the page loads (#2218), "Your login was created." (#2148), and her signed Contract's "continuous support in labor", which is what she signed and stays. On Maya's side: "The baby arrived before the window would have opened." (#2220) on the page Maya opens right after recording a loss, and "until Staff issues a new Contract" (#2221) in the third person. Everything else she reads is fixed wording, true for every Client.

**Findings filed**: [NH-G9 (#2214)](https://github.com/markgoho/doula-cloud/issues/2214), the signed PDF without a signature; [NH-G10 (#2215)](https://github.com/markgoho/doula-cloud/issues/2215), [NH-G11 (#2216)](https://github.com/markgoho/doula-cloud/issues/2216) and [NH-G12 (#2217)](https://github.com/markgoho/doula-cloud/issues/2217), failed budget steps, each on the [journey map](../journeys/loss-client.md#gaps-found) with its budget line quoted; and five bugs, [#2218](https://github.com/markgoho/doula-cloud/issues/2218) (the loading Visits), [#2219](https://github.com/markgoho/doula-cloud/issues/2219) (the Birth Plan in her history), [#2220](https://github.com/markgoho/doula-cloud/issues/2220) (the On call line) and [#2221](https://github.com/markgoho/doula-cloud/issues/2221) (the voided-Contract invoicing line) and [#2222](https://github.com/markgoho/doula-cloud/issues/2222) (the seeded template's ISO dates). Cited, not filed: [#2148](https://github.com/markgoho/doula-cloud/issues/2148) ("login"), [#2205](https://github.com/markgoho/doula-cloud/issues/2205) (Sent after Paid) and [HS-G10 (#2210)](https://github.com/markgoho/doula-cloud/issues/2210), where this walk's 7.B-confirmation is recorded in a comment.

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 4, 5, 6, 7, 8 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **26 budget steps**, 6 `automated`, 20 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-09-10 — 6.2-a re-marked ([#1119](https://github.com/markgoho/doula-cloud/issues/1119))

**6.2-a moves from `missing-feature (HS-G3, NH-G8)` to `manual`.** The step is performable: the portal's Contract page offers **Download signed Contract (PDF)** on a voided Contract, and the Staff Engagement page offers the Practice's own copy of it in the same case. What settled it is the gate itself rather than a walk — the Contract read now carries `hasSignedPdf`, filled by the Go BFF from `signedPDFObjectPath`, the one lookup the signed-PDF routes stream from, and both screens gate on that field. Two screen-level specs cover the voided case on each surface (the portal Contract route's own spec and the Staff Engagement page's), beside the Go tests that hold the field to what the route serves. Both gap IDs the step cited are closed and stay cited; no new gap ID is minted, because the finding arrived as a ticket and leaves as a fix rather than as a hole in [the journey map](../journeys/loss-client.md).

The Marks table is recounted with it: `manual` 13 -> 14, `missing-feature` 9 -> 8. The [README's run-status table](README.md#the-run-and-the-gap-issues) moves the same way.

**No other step is re-marked.** 6.1 and 6.2 are unchanged, and 6.2-b — the Client-facing money surface that does not exist — is untouched by this.


### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.1 | `client-portal-login.e2e.ts` | pass |
| 3.1 | `push-notification.e2e.ts` | pass |
| 4.1 | `client-portal-login.e2e.ts` | pass |
| 4.2-a | `client-portal-login.e2e.ts` | pass |
| 5.2 | `birth-plan.e2e.ts` | pass |

**5 automated steps: all pass.**

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**. That is [#239](https://github.com/markgoho/doula-cloud/issues/239).

### 2026-09-10 — narrative reconciliation ([#685](https://github.com/markgoho/doula-cloud/issues/685))

A desk pass over this plan's Contract and Visits cells, which [#318](https://github.com/markgoho/doula-cloud/issues/318) left alone. Nothing was re-walked and no mark moved.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 6.2 | "`Status: voided`" and the Staff component's ledger word -> **No longer active**, then the Practice's own name in the notice | `app/src/lib/clientRegister.ts`'s `contractStatusLabel` and `contractVoidedNotice`, and the portal Contract page that reads them. [NH-G5](https://github.com/markgoho/doula-cloud/issues/212) is closed |
| 6.2-a | "the endpoint 404s once voided" -> the endpoint serves her, and the screen hides the control | `serveSignedPDF`'s own doc comment in `api/internal/contracts/signed_pdf.go` ("has ever been signed"), against the portal page's `status === 'signed'` gate. [NH-G8](https://github.com/markgoho/doula-cloud/issues/299) and [HS-G3](https://github.com/markgoho/doula-cloud/issues/302) are both closed, and what is left of the step is [#1119](https://github.com/markgoho/doula-cloud/issues/1119) |
| 7.1 | "no date, no type, no notes" -> the row carries all three, and a bereavement Visit types as `postpartum` because ADR-0015 derives the type rather than asking for it | The Visits table's Type / Date / Notes columns, and `api/internal/visit/type.go`. [MO-G1](https://github.com/markgoho/doula-cloud/issues/250), [MO-G2](https://github.com/markgoho/doula-cloud/issues/251) and [PR-G6](https://github.com/markgoho/doula-cloud/issues/281) are closed |

**No step is re-marked.** 6.2-a stays `missing-feature` because the step still cannot be performed from any screen she has; the two gap IDs it cites are the ones it came from, and the remaining half is now [#1119](https://github.com/markgoho/doula-cloud/issues/1119). A plan never mints a gap ID, so that finding is a ticket rather than a new `-a` row. `contract-lifecycle.e2e.ts` drives a Contract through to `signed`, never to `voided`, so it exercises no step on this plan.

### 2026-08-23 — manual and missing-feature steps ([#239](https://github.com/markgoho/doula-cloud/issues/239))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Willow Creek Doula Care", Owner+Admin+Doula in one Staff row — Maya's shape, since her journey carries this plan's staff-side stages) with one Client (Nadia Haddad), one Engagement, four Visits, a filled Birth Plan, a Contract signed by the Client and then voided by staff, and a four-message thread. Walked in Chrome via playwriter, one browser profile.

**Note on the fixture**: Staff and Client-portal sessions share one `__session` cookie per origin (confirmed via `Network.getCookies`), so a single profile can hold only one of the two logged in at a time on `localhost:5173` — signing in as the other silently 403s the first ("no matching staff account" on next request). This is exactly what the plan's Preconditions warn about ("Two browser sessions or two profiles"); not a new finding, and not evidence of a production defect, since staff and portal are reached via different routes there. Walked by re-authenticating whichever side was about to act, immediately before each of its steps.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.2 | `manual` | as expected | One continuous thread; Maya's and Nadia's messages both appear in order, immutable |
| 1.2-a | `missing-feature (MO-G3)` [#252](https://github.com/markgoho/doula-cloud/issues/252) | confirmed | Engagement page (both sides) shows only `Status` and `Created` — no due date, no gestation, no other Client detail |
| 2.1 | `missing-feature (NH-G3)` [#295](https://github.com/markgoho/doula-cloud/issues/295) | confirmed | No control anywhere on the Engagement page records an outcome |
| 2.1-a | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) | confirmed | No status value is offered anywhere in the UI — there is no status control to read a value from |
| 2.1-b | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | confirmed | Same absence — no status control exists to set any value with |
| 3.1-a | `manual` | as expected, confirmed by inspection not by walking | Closing the tab was not exercised live; grounded instead in the same fact the journey map already cites (no reminder machinery in `api/`, content-free push by design, ADR-0002) — nothing was found to contradict it |
| 3.2 | `manual` | as expected, confirmed by inspection not by walking | Not literally waited out; grounded the same way as 3.1-a — no reminder/nudge/countdown code exists anywhere in `api/` |
| 4.2 | `manual` | as expected, one correction | `<h1>` reads **"Welcome to Willow Creek Doula Care"** — the actual Practice name, confirming [NH-G4](https://github.com/markgoho/doula-cloud/issues/296)'s `{practiceName}` template. (The plan's illustrative text names "Rooted Birth Collective", a different Practice used as the shared fixture in other personas' plans; the mechanism, not the literal string, is what NH-G4 claims, and it holds.) |
| 4.2-b | `manual` | as expected | `Status: intake`, `Created: 8/23/2026` — the only date the portal holds |
| 4.3 | `manual` | as expected | **Birth Plan** then **Contract**, then the thread — same order every time |
| 5.1 | `manual` | as expected | **Birth Plan** link renders unconditionally, second from the top |
| 5.1-a | `missing-feature (NH-G2)` [#294](https://github.com/markgoho/doula-cloud/issues/294) | confirmed | No retire/hide/archive/dismiss control on either side — Staff's Birth Plan section offers only **Save Birth Plan** |
| 5.2-a | `manual` | as expected | `page.emulateMedia({ media: 'print' })` hides both **Back** and **Print** (`isVisible()` false for both); the print mechanism works |
| 6.1 | `manual` | as expected | A voided Contract still renders its full signed prose on the Client side |
| 6.2 | `manual` | as expected | `Status: voided`, then "Voided — this Contract is no longer active." — the ledger's word, no Void button, no human context ([NH-G5](https://github.com/markgoho/doula-cloud/issues/212)) |
| 6.2-a | `missing-feature (HS-G3)` [#302](https://github.com/markgoho/doula-cloud/issues/302), reasoning corrected, new gap **[NH-G8](https://github.com/markgoho/doula-cloud/issues/299)** minted | **falsified in part** | The plan's claim was that the endpoint is merely unlinked. It is worse: `GET /api/portal/engagements/{id}/contract/pdf` itself 404s ("no signed contract found for this engagement") once the Contract is `voided`, confirmed with a direct `fetch()` from the page. `serveSignedPDF` (`api/internal/contracts/signed_pdf.go:66-69`) queries `WHERE ... AND status = $2::contract_status` bound to `statusSigned` — a Contract that has since moved to any other status, voided included, can never resolve a row, even though `signed_pdf_object_path` is still set and the PDF still exists in the store. Fixing HS-G3 (adding a link) would not fix this: the endpoint itself refuses. Distinct root from HS-G3, which is about the missing link on a Contract that is still `signed`; this is about the query excluding every Contract that is not. Same shared function backs the Staff-side signed-PDF route (`main.go:208`), so a Practice loses its own copy of a voided Contract too, once it is voided — noted here since this map owns the void transition, not asserted as a Staff-side finding of this plan. |
| 6.2-b | `missing-feature (NH-G6)` [#297](https://github.com/markgoho/doula-cloud/issues/297) | confirmed | No Invoice, balance, or payment surface anywhere in the portal — the portal's links are Birth Plan and Contract only |
| 7.1 | `manual` | as expected | A fourth Visit row: `Maya Okonkwo`, `8/23/2026` — indistinguishable from the first three |
| 7.1-a | `manual` | as expected | No Visit surface on the Client side at all — the portal home's only links remain Birth Plan and Contract |
| 7.2 | `manual` | as expected | The thread continues both ways; nothing marks that its subject has changed |
| 7.2-a | `missing-feature (NH-G7)` [#298](https://github.com/markgoho/doula-cloud/issues/298) | confirmed | No mute, pause, or subject-change control on either side's Messages section |
| 8.1 | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) | confirmed | Same absence as 2.1-a/2.1-b, observed a second time from the Client's own side |

**22 steps walked: 13 `manual`, 9 `missing-feature`. Every mark holds; one reasoning corrected (6.2-a) and one new gap minted (NH-G8, added to [docs/journeys/loss-client.md](../journeys/loss-client.md)).** No `blocked` step exists on this plan, confirmed. No `journey-gap` issue filed from this ticket — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict**: this plan cannot pass, as written, and does not. Every `missing-feature` step confirmed genuinely unwalkable; the record stays at `intake` forever, the Birth Plan cannot be retired, and money has no client-facing surface at all.

**Unexplained, not chased**: three `404`s in the browser console right after **Save Contract** and one after **Void Contract** (staff side), origin not identified. Neither action's own result was wrong — the Contract saved and voided correctly both times — so this was not chased down to a source. Named here rather than dropped, since an unexplained `404` on a write path is exactly what turned into real findings on Maya's and Dee's walks.
