# Hannah Sorensen — test plan

- **Journey**: [first-time-client.md](../journeys/first-time-client.md)
- **Persona**: [first-time-client.md](../personas/first-time-client.md)
- **A pass means**: a portal account, a signed Contract, a Birth Plan she has read and printed, a thread with traffic both ways, and an Engagement that reflects where she actually is. All five were reached on the second walk (2026-10-28). The fifth held for her care phase and Visits but not for the due date after a live birth (HS-G11), and the 2026-11-01 re-walk found the due date gone once the birth was recorded.

She is the full-arc Client and the only Persona who walks every client-facing screen the product has, so it carries the most `automated` steps of any client-side plan — every portal spec was written along her path. Only Maya's, at nine, carries more, and hers are the Staff side of the same walk. Where her path crosses Nadia Haddad's, Nadia's plan was written first ([loss-client.md](loss-client.md)).

## Preconditions

- A Practice with an Owner and one Client created for Hannah — name and email only, which is all `POST .../clients` takes (**MO-G3**).
- A Contract Template with merge fields, so stage 4 has prose to render.
- Nothing else. She is the one Persona the product can provision end to end.
- A phone, or a phone-sized viewport, for stages 6 and 7. Stage 6 is the moment of truth and it is device-specific: she prints from whatever she has to hand.

## Steps

### Stage 1 — Getting invited

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Press **Send portal invite** on the Engagement page | "Invited. An email has been sent to them. If you need to share the link directly, here it is:" and the `…/portal/accept-invite?token=…` link; the Portal invite row reads "Invite pending", and the email goes to the Client's address (RA-G1 closed, [#260](https://github.com/markgoho/doula-cloud/issues/260)). `portal-invite-accept.e2e.ts` calls this endpoint as fixture setup, so **the button itself is unautomated** | `manual` |
| 1.2 | Receive the invite | An email from "DoulaCloud", "You've been invited to view your care details online", with the link and "If you weren't expecting this, you can safely ignore this email." It names no Practice and no Doula, by decision: the invite's copy is content-free (ADR-0009, [#221](https://github.com/markgoho/doula-cloud/issues/221), `portalinvite/outbox.go`). Sending the link by hand is still possible from the page | `manual` |

### Stage 2 — Making an account

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/portal/accept-invite` with no token | "Accept your portal invite", "This link is not complete." and "Ask your doula to send the invite again." — no form | `manual` |
| 2.1-a | Open it with the token | A single **Continue** button — no email, password or account-mode choice. #617 (ADR-0026) closed [HS-G1](https://github.com/markgoho/doula-cloud/issues/300): a Client has no password | `automated (portal-invite-accept.e2e.ts)` |
| 2.2 | Press **Continue** | `POST /api/portal/accept-invite` claims the pending row and sets the session cookie on its own response (#145). Nothing is asked of her — the Practice already holds her name, and there is no account-setup step to complete | `automated (portal-invite-accept.e2e.ts)` |
| 2.3 | Watch where she lands | `GET /api/portal/session` resolves one Engagement, so she is redirected straight to it rather than shown a chooser | `automated (portal-invite-accept.e2e.ts)` |
| 2.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The accept page, then her Engagement | `manual` |
| 2.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 1 (Hick's Law, Miller's Law). **Continue** | `manual` |
| 2.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). She lands on her own Engagement, which says whose care this is | `manual` |
| 2.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 2.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 3 — The first screen

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Read the `<h1>` | **Your care**, with her Practice's name under it (NH-G4 closed, [#296](https://github.com/markgoho/doula-cloud/issues/296)) | `automated (portal-invite-accept.e2e.ts)` |
| 3.1-a | Read the lines under it | **What happens next**: "No visit is booked yet." and "Your due date is Mar 10, 2027." No raw status and no created date ([#212](https://github.com/markgoho/doula-cloud/issues/212) and **[MO-G3](https://github.com/markgoho/doula-cloud/issues/252)** closed). Once a Visit is booked the first line names who is coming and when | `manual` |
| 3.2 | Read what is offered below | **Your visits**, **Your birth plan**, **Your contract**, **Messages**, **What you still owe** and **Invoices**, then **Everything that has happened**. Each card says its state in a line ("Nothing is written in it yet.", "No Contract has been sent for your care yet."). The top bar holds the five destinations: Your care, Messages, Birth plan, Contract, Notifications | `manual` |
| 3.3 | Watch for anything asking about notifications | Nothing asks, and nothing registers: no push subscription exists and the browser's permission stays unasked until she turns notifications on herself (#716) | `manual` |
| 3.3-a | Review, mute, or explain that subscription afterwards | **Notifications** in the top bar: "This device can tell you when a message is waiting. Do not rely on it for anything urgent.", **What a notification shows** ("Never who sent the message or what it says."), and **Turn on notifications** or **Turn off notifications**, each recorded in her Activity (HS-G4 closed, [#303](https://github.com/markgoho/doula-cloud/issues/303)) | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 5 (Hick's Law, Miller's Law). The portal's five destinations | `manual` |
| 3.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 4 — Signing the Contract

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open the Contract link once Maya has sent it | The Contract link opens the sent Contract under **Ready for your signature** — the Client register's own words for `sent`, never the Staff component's raw enum ([#212](https://github.com/markgoho/doula-cloud/issues/212)). The prose renders below it; the spec asserts the link and the label here, and asserts the merge-field values on the Practice side before sending | `automated (contract-lifecycle.e2e.ts)` |
| 4.1-a | Open it while the Contract is still `draft` | "No Contract has been sent for your care yet." — RLS gives her no row, and that is the right message, now said in her own register's word for the Engagement | `manual` |
| 4.1-b | As Maya, take a signature before sending the portal invite | **She is stopped before she can.** On a Client who has never been invited, a notice names the ordering and the way out of it ("Send a portal invite to this Client before sending the Contract — the Contract can only be viewed and signed once portal access exists."), with **Send portal invite** at the top of the same page, and **Send Contract** is absent, never disabled (`govuk-alignment.md`, the Button row), rather than reporting a refusal after the click. The BFF enforces the same precondition itself ([MO-G6](https://github.com/markgoho/doula-cloud/issues/255) closed) | `manual` |
| 4.2 | Type her full legal name, tick the attestation, submit | `POST .../contract/sign` succeeds and the page re-renders at **Signed**. **The strongest screen in the portal**, and a spec covers it now: `contract-lifecycle.e2e.ts` agrees to sign electronically, fills the full legal name, ticks the attestation and presses **Sign**, as the Client, in her own browser context | `automated (contract-lifecycle.e2e.ts)` |
| 4.3 | Keep a copy of what she signed | **Download signed Contract (PDF)** sits under the signed prose and hands her the file ([HS-G3](https://github.com/markgoho/doula-cloud/issues/302) closed), and the BFF can read the object back out of the store to serve it ([HS-G6](https://github.com/markgoho/doula-cloud/issues/305) closed). The control is gated on the Contract read's own `hasSignedPdf` rather than on `status`, so it is offered here and stays offered if the Contract is ever voided ([#1119](https://github.com/markgoho/doula-cloud/issues/1119) closed) — that half is Nadia's plan, not this one | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). Her full legal name, the attestation, and the submit | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The Contract's prose and values stay on screen while she signs | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The page says it is signed, when, and how to keep a copy | `manual` |
| 4.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 5 — Reading the Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open the Birth Plan link before Maya has filled one | "No Birth Plan has been created for your care yet.", with no indication of whether one is coming, or when. The **Your care** card says "Nothing is written in it yet." | `manual` |
| 5.1-a | Open it once filled | The Plan Instance's snapshot renders read-only | `automated (birth-plan.e2e.ts)` |
| 5.2 | Correct the two things she wants changed | She cannot correct it: it is staff-drafted and read-only to her, by decision (HS-G2 closed, [#301](https://github.com/markgoho/doula-cloud/issues/301)). She can press **I've read this**, which reads back "You confirmed you've read this on {date}." and shows Maya "Hannah read this on {date}"; a Staff edit clears it, and Maya's page returns to "Hannah has not read this yet" (#880) | `manual` |
| 5.2-a | Message Maya about them instead | Succeeds. Her only route to a correction is a re-type by Staff, and nothing links the message to the document | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). **Print**, **Download birth plan (PDF)** and **I've read this**; the thread, for telling Maya, is in the top bar and not counted (HS-G8, closed by a budget change) | `manual` |
| 5.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 6 — Print it and hand it over — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Birth Plan on her phone and press **Print** | The mechanism works: one tap from **Your care** or the top bar, and a print stylesheet hides the top bar, the Back link, the two export buttons and **I've read this** (`birth-plan/+page.svelte`, `@media print`). The pilot banner and the footer no longer print ([#2212](https://github.com/markgoho/doula-cloud/issues/2212)). The sheet opens with **Name** (her legal name), **Doula** (Maya, who is on the Engagement; **Doulas** for more than one, and no row while nobody is), **Practice** (the Practice's name) and **Last changed** (the day the plan's answers last changed, in the Practice's zone), before the first question (HS-G9, [#2209](https://github.com/markgoho/doula-cloud/issues/2209), [#2355](https://github.com/markgoho/doula-cloud/issues/2355)) | `manual` |
| 6.1-a | Have Maya hand it over on her behalf | **Print for the hospital** on the Engagement's Birth Plan section opens the plan at its own address (`/practices/{id}/engagements/{id}/birth-plan`), with **Print** and **Download Birth Plan (PDF)**; the sheet reads "Hannah Sorensen's Birth Plan" ([#2350](https://github.com/markgoho/doula-cloud/issues/2350)), then the same lines as hers: **Name**, **Doula**, **Practice** and **Last changed** (PR-G5 closed, [#280](https://github.com/markgoho/doula-cloud/issues/280); HS-G9) | `manual` |
| 6.1-b | Press **Download Birth Plan (PDF)** | The PDF arrives (`GET .../birth-plan/pdf`, `200`, `application/pdf`), under "Birth Plan": "Name: {her legal name}", "Doula: {her Doula}", "Practice: {the Practice}" and "Last changed: {day}", then the sections and answers. The same PDF from Maya's side (HS-G7 closed, [#306](https://github.com/markgoho/doula-cloud/issues/306); HS-G9) | `manual` |
| 6.2 | Hand the paper over | Leaves the product entirely. The last mile is paper she remembered to print in advance, handed to a stranger in scrubs | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). The Birth Plan, then the browser's own print dialog, which does not count | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). **Print**, **Download birth plan (PDF)**, and **I've read this** until she has confirmed or after a Staff edit (HS-G8, closed by a budget change) | `manual` |
| 6.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Nothing she must remember to print in advance; the Birth Plan is one tap from her Engagement | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The printed page names her, her Practice and when the plan last changed, so the stranger in scrubs knows whose it is | `manual` |
| 6.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User). A first print, in a hospital, with nothing explained | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 7 — Living in the thread

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Send a message at 11pm from her phone, with a photo | `POST .../messages` as `multipart/form-data`; the image renders inline. Attachments work both ways | `manual` |
| 7.2 | Receive Maya's reply with the tab open | The tab refetches on the push message and the reply appears without a reload. The push itself carries no content (ADR-0002) | `automated (push-notification.e2e.ts)` |
| 7.3 | Press **Load older messages** and read back through the Engagement | The thread opens on the newest 30; **Load older messages** at its top pages in the rest, oldest first, and goes away when there is nothing older. No message carries an edit or delete control | `manual` |
| 7.3-a | Find out that a push is **not an alarm** | The **Notifications** screen says "Do not rely on it for anything urgent." The thread itself says nothing about it (HS-G4 closed, [#303](https://github.com/markgoho/doula-cloud/issues/303)) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). The message, a photo, and **Send** | `manual` |
| 7.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 7.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 8 — Birth, and afterwards

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Open the portal after the birth | **Your care** shows no due date once the birth is recorded, and says nothing in its place: **What happens next** names who is coming next, or reads "No visit is booked yet." The Birth Plan card still offers her plan (HS-G11 closed, [#2211](https://github.com/markgoho/doula-cloud/issues/2211)) | `manual` |
| 8.1-a | Have the status reflect where she actually is | The care phase moves: the first scheduled Visit moved it from Intake to Active, and her **Everything that has happened** says "Your care is now ongoing." (MO-G4 closed, [#253](https://github.com/markgoho/doula-cloud/issues/253)). A recorded live birth takes the due date off **Your care** and changes nothing else she sees, because the outcome is staff-only (HS-G11 closed, ADR-0015) | `manual` |
| 8.1-b | Find any record of the birth itself | **Your visits** shows her Visits, who is coming and when (`GLOSSARY.md`, Visit). The birth outcome is staff-only, so nothing in the portal records the birth itself, and **Everything that has happened** has no entry for it | `manual` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

### Stage 9 — Her partner asks for the login

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Give her partner access | No invite flow, no guest role, no read-only share of the Birth Plan. What refuses a second invitation today is `invite()` (`portalinvite/invite.go`), a handler-level check against the accepted Client, not the schema — `client_portal_users` now allows more than one `(identity_uid, client_id)` pair. The path nobody designed is sharing the mailbox that receives her sign-in links, not her password — she has none (ADR-0026). HS-G5 is closed without a build: partner access is ruled out of v1 ([#176](https://github.com/markgoho/doula-cloud/issues/176)), so the step keeps citing it as the reason there is nothing to walk | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) |
| 9.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). No screen to count while partner access is out of v1 | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) |
| 9.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). No screen to count while partner access is out of v1 | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) |
| 9.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen says her partner has been asked in, and what the partner does next | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) |
| 9.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) |
| 9.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (portal-320.e2e.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 16 (8 of them [budget steps](README.md#budget-steps)) |
| `manual` | 51 (28 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 5 ([HS-G5](https://github.com/markgoho/doula-cloud/issues/304) ×5: 9.1 and four of its budget steps; closed without a build, partner access ruled out of v1) |

No step is `blocked`. HS-G2, HS-G4, PR-G5 and MO-G4 are closed, and 3.3-a, 5.2, 6.1-a, 7.3-a and 8.1-a are walkable, so none of them holds a step. Her plan has no Stripe step, but the portal now has a money surface (**What you still owe** and **Invoices**, the Stripe half of NH-G6; the off-Stripe half stays open on [#297](https://github.com/markgoho/doula-cloud/issues/297)), and on the second walk she paid a Stripe Invoice from it (the Run log's note on the Pay button).

NH-G4, **RA-G1**, **PR-G7**, **MO-G3** and [#212](https://github.com/markgoho/doula-cloud/issues/212) are observed inside walkable steps (3.1, 1.2, 3.1-a) rather than given steps of their own, and all are closed. HS-G1 is closed ([#300](https://github.com/markgoho/doula-cloud/issues/300)) and its exposing step, 2.2-a, is gone with it.

The Birth Plan's absent export is now [her map](../journeys/first-time-client.md)'s **HS-G7**, and 4.3's absent Contract copy sharpened into **HS-G6** — both minted at the run ([#240](https://github.com/markgoho/doula-cloud/issues/240)). The one fact this plan used to hand forward — **stage 4 is the strongest screen in the portal with no spec on it** — is answered: `contract-lifecycle.e2e.ts` signs the Contract as the Client, and 4.1 and 4.2 are `automated` on the strength of it.

## Run log

### 2026-11-01 — the fixed steps walked again ([#2264](https://github.com/markgoho/doula-cloud/issues/2264))

**Stack**: `SEED_DEMO=0 bun run --cwd app dev:full` from a worktree at port offset 3 (the app at `http://localhost:5473`, the mail catcher at `http://127.0.0.1:19380`), on trunk at `8d89c32c`, in WSL2 with rootless Podman. No Stripe step was needed, and no Contract was made: the Practice has no birth rate, and none of the steps below needs one.

**Accounts and preconditions**: `seed:staff-session` made Maya Okonkwo's Practice, **Okonkwo Birth Support** (zone `America/New_York`), with Maya as Owner, Admin and Doula. As Maya, at `/login`, the orchestrating agent added Hannah Sorensen (`hannah.sorensen@sim.doula.cloud`) and started birth work with Maya as the Doula, due Mar 10, 2027. Maya then sent the portal invite and filled the Birth Plan: three answers, one changed in a second save (16:36:25 UTC), then a third save that changed nothing (16:36:47 UTC). After the stage 5 to 7 walk, Maya recorded "The baby was born alive", dated Nov 1, 2026. Maya never replied in the thread.

**How it was walked**: by the blind `persona-walker` and the sighted `walk-evaluator` ([`docs/agents/persona-walk.md`](../agents/persona-walk.md)), as two walks, each with its own profile. **Walk 1**: stages 5, 6 and 7 at `390x844x3,mobile,touch`, pointer, no throttling, with the network Offline partway through stage 7. She got in from the invite in the catcher (stage 2, as the way in). 18 acts, each with a shot and a timing. **Walk 2**: stage 8 at `320x640x2,mobile,touch`, signed in from `/portal/login` and a new sign-in link. 10 acts. Her persona says "usually at night, usually from her phone". Stage 8's note asks for 320px and desktop, so walk 2 took the 320px floor. The evaluators found no `u` entry in either walk, so both are admissible. Some reads the blind walker could not make were done by the orchestrating agent, in separate isolated contexts: both print sheets, both PDFs, the focus and live region after **Send**, **Your care** at 1280x800, and the 320px sweep of walk 1's screens. Headless Chrome opens no print preview. So each "print" below is the page with its own `@media print` rules applied to the screen, read and screenshotted. The PDFs were fetched in the signed-in page and read with `pdftotext`.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 5.B-decisions | 3 | **3** at 390x844, before she confirmed: **Print**, **Download birth plan (PDF)**, **I've read this**. 2 once she pressed **I've read this** ("You confirmed you've read this on Nov 1, 2026."). Back, the top bar and the skip link are not counted. At 390x844, **I've read this** sits below the fold | **yes**, [HS-G8 (#2208)](https://github.com/markgoho/doula-cloud/issues/2208)'s budget |
| 6.B-decisions | 3 | **3** at 390x844 on the unconfirmed plan, 2 after, so the largest is 3 | **yes**, [HS-G8 (#2208)](https://github.com/markgoho/doula-cloud/issues/2208)'s budget |
| 6.B-confirmation | yes | yes, on all four. **Portal page and print** (390x844): Name "Hannah Sorensen", Doula "Maya Okonkwo", Practice "Okonkwo Birth Support", Last changed "Nov 1, 2026", before the first question. The print rules hide the top bar, the pilot banner, Back, both exports and the read status. **Portal PDF** (`GET /api/portal/…/birth-plan/pdf`, `200`, `application/pdf`, 12,781 bytes): "Birth Plan", "Name: Hannah Sorensen", "Doula: Maya Okonkwo", "Practice: Okonkwo Birth Support", "Last changed: Nov 1, 2026". **Maya's Print for the hospital** (1280x800): "Hannah Sorensen's Birth Plan" and the same four rows, with nothing about when the Engagement was created. **Maya's PDF**: the same text as Hannah's. The day is the save that changed an answer, in New York. The save that changed nothing left `plan_instances.changed_at` at 16:36:25, read from the database, though Staff's Last edited moved to 16:36:47. Both saves fell on the same day, so the screen alone cannot show this | **yes**, [HS-G9 (#2209)](https://github.com/markgoho/doula-cloud/issues/2209) built |
| 7.B-confirmation | yes | yes at 390x844. **Send** put the message last under "You" with its time, and the status read "Message sent to Okonkwo Birth Support. A reply will show here. Do not rely on messages for anything urgent." It is `role="status"` (polite), and focus stayed on **Send** (read from the accessibility tree; no screen reader was run). **Offline**: two sends each read "There is a problem", "We could not reach the service. Check your connection and try again.", kept the draft in the box, and said nothing about sent. Leaving for **Notifications** and coming back lost the draft. The portal keeps nothing on the device, by decision (ADR-0059, position 4, recorded on the page) | **yes**, [HS-G10 (#2210)](https://github.com/markgoho/doula-cloud/issues/2210) built |

**The numbered steps read on the way**:

- **8.1** and **8.1-a** ([HS-G11 (#2211)](https://github.com/markgoho/doula-cloud/issues/2211)): **as the corrected cells say**, at 320x640 (walk 2) and at 1280x800 (the orchestrating agent). After the recorded live birth, **What happens next** reads only "No visit is booked yet." No due date, and nothing in its place. **Your birth plan** still offers **Read your birth plan** ("You confirmed you read it on Nov 1, 2026."). **Everything that has happened** has no entry for the birth. To her, "It looks exactly like it did when I was pregnant." The outcome is staff-only by decision (ADR-0015).
- **5.2**: as expected. No field to correct, and **I've read this** read "You confirmed you've read this on Nov 1, 2026.". Maya's page read "Hannah read this on Nov 1, 2026". The save that changed nothing did not clear it.
- **6.1** and **6.1-b**: **Print** called the browser's print, and the headless browser drew nothing (0 ms, twice). **Download birth plan (PDF)** answered `200` twice. In the headless browser the screen showed nothing when the file arrived, since on success the page draws no notice and leaves the download to the browser.
- **6.1-a**: Maya's sheet is now titled "Hannah Sorensen's Birth Plan", after [#2350](https://github.com/markgoho/doula-cloud/issues/2350). The cell is corrected.
- **7.1**: a message with no photo, at 390x844. The thread shows times in the reader's zone, and this browser's is UTC, so the time is not graded.
- **3.3-a** (outside the brief; she went there next): **Turn on notifications** read "Notifications are turned on, but this device is not receiving them. To fix this, allow notifications for this site in your browser's own settings.", as on 2026-10-28. The headless browser grants no permission.

**320px**: the orchestrating agent swept walk 1's portal Birth Plan and Messages at 320x640. `scrollWidth` was 320 on each, so there was no sideways scroll, and nothing reached past the viewport. Walk 2's screens were walked at 320x640.

**Timing**, against stages 5 to 8's 400 ms and 100 ms: every in-page act took 16 to 70 ms. The two exceptions were **Continue** on the accept page (267 ms to **Your care**, walk 1) and **Continue** on the sign-in link page (181 ms, walk 2). The Time steps were not re-graded.

**Seen on the way, outside the steps above**, each filed:

- The starter Birth Plan asks nothing about pain relief. That question is in the staff-only starter Care Plan, so it never reaches the printed plan, the PDF or the Client. She went looking for "what I said about the epidural": [#2580](https://github.com/markgoho/doula-cloud/issues/2580).
- A plan save that changes no answer is logged as "Birth Plan for the Client changed" on Maya's Activity and as "Your birth plan was updated." on Hannah's, while Last changed rightly stays put: [#2581](https://github.com/markgoho/doula-cloud/issues/2581).

Not filed, each settled by a record: the invite email names no Practice (ADR-0009, #221). She can only acknowledge the plan, not correct it (#301). The draft is lost when she leaves the thread (ADR-0059, position 4). Notifications are "on" but not received in a headless browser, which grants no permission.

No mark moved, so the Marks summary and [README.md](README.md)'s run-status row are unchanged.

### 2026-10-28 — second manual walk ([#2172](https://github.com/markgoho/doula-cloud/issues/2172))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, on trunk at `af630ebd`, in WSL2 with rootless Podman. `scripts/stripe-listen.sh` was running against it, Stripe in the Sandbox with a test-mode key, and the BFF now has `STRIPE_PUBLISHABLE_KEY`.

**Device**: headless Chrome inside WSL, driven by the chrome-devtools MCP, one isolated browser context for Maya and another for Hannah, both in UTC. Stages 1 to 5 at 1280x900; stages 6, 7 and 8 on an emulated Pixel 7 (412x839, 2.625x, mobile and touch, an Android user agent), as on #237. Every timing below is on an unbundled Vite dev server, which compiles a route the first time it is visited.

**Accounts and preconditions**: Maya Okonkwo's Practice, **Okonkwo Birth Support** (`5c88b7f9-…`), as #2171 left it: card billing through Stripe on an active connected account, 2 Credits. Hannah Sorensen (`hannah.sorensen@sim.doula.cloud`, Client `7efce577-…`, Engagement `d5722d52-…`, birth, due Mar 10, 2027) was started with an email and nothing else: never invited, no Contract, no Doula. Every Practice-side act was done as Maya, signed in at `/login`, through the product: the Draft Contract before the invite (4.1-a, 4.1-b), the invite (1.1), **Send Contract**, the Birth Plan (filled, then changed after Hannah's message), a reply in the thread, one scheduled Visit, a recorded live birth (dated Oct 28, 2026, because the stack cannot move the clock to March), and an Invoice. Hannah accepted her invite from the catcher in a fresh context, which is also the Cold walk for stages 2 to 6. For 7.3 she sent 29 more short messages from the thread, so it held 33 and paged.

The 16 `automated` steps were **not** re-run. 4.1 and 4.2 were walked anyway, because the Contract had to be signed for the rest of the journey.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | **better than the plan claimed**, cell corrected | "Invited. An email has been sent to them. If you need to share the link directly, here it is:" and the link; Portal invite "Invite pending". **Send portal invite** stays at the top of the page until she accepts, then goes. The line calls Hannah "them" ([#2198](https://github.com/markgoho/doula-cloud/issues/2198), open) |
| 1.2 | `manual` | **better than the plan claimed**, cell corrected | The catcher's `/inbox/hannah.sorensen%40sim.doula.cloud` held "You've been invited to view your care details online" from "DoulaCloud", "Hello," and the link. No Practice or Doula is named, by decision (ADR-0009, #221), so "Is this real?" is answered only by her expecting it |
| 2.1 | `manual` | as expected, cell corrected | "Accept your portal invite", "This link is not complete.", "Ask your doula to send the invite again." No form |
| 3.1-a | `manual` | **better than the plan claimed**, cell corrected | "What happens next", "No visit is booked yet.", "Your due date is Mar 10, 2027." No status enum, no created date |
| 3.2 | `manual` | **better than the plan claimed**, cell corrected | Six cards under "Your care", "Okonkwo Birth Support", each with its state in a line, and **Everything that has happened**. Top bar: Your care, Messages, Birth plan, Contract, Notifications, and the account menu |
| 3.3 | `manual` | **better than the plan claimed**, cell corrected | Nothing asked. `Notification.permission` stayed `default`, and the service worker held no push subscription |
| 3.3-a | `manual` (was `missing-feature (HS-G4)`) | **walked** | **Notifications**: "This device can tell you when a message is waiting. Do not rely on it for anything urgent." **Turn on notifications** in this headless browser read "Notifications are turned on, but this device is not receiving them. To fix this, allow notifications for this site in your browser's own settings." (the browser never granted permission); **Turn off notifications** read "Notifications are off." in 31 ms. Both presses are in her Activity and Maya's |
| 4.1-a | `manual` | as expected | With the Contract still a Draft: "No Contract has been sent for your care yet." |
| 4.1-b | `manual` | as expected, cell corrected | On the never-invited Hannah, after **Save Contract**: "Draft, not sent yet", Fee $1,800.00, and "Send a portal invite to this Client before sending the Contract — the Contract can only be viewed and signed once portal access exists." **Send Contract** is not rendered at all, by design (the page's #255 comment: "A withheld Send is absent, never disabled") |
| 4.1, 4.2 | `automated (contract-lifecycle.e2e.ts)` | as the spec asserts | "Ready for your signature", the prose with Okonkwo Birth Support, Hannah Sorensen, the scope, the dates and $1,800.00, then the electronic-signature disclosure and **I agree to sign electronically, continue**; then Full legal name, the attestation and **Sign**: "Signed" |
| 4.3 | `manual` | as expected | "You signed this Contract. Okonkwo Birth Support can see it is signed, and your copy is ready to download." **Download signed Contract (PDF)**: `GET .../contract/pdf` `200`, `application/pdf`, a `%PDF` file. HS-G6 holds closed |
| 5.1 | `manual` | as expected, cell corrected | "No Birth Plan has been created for your care yet." |
| 5.2 | `manual` (was `missing-feature (HS-G2)`) | **walked** | No field to correct. **I've read this**: "You confirmed you've read this on Oct 28, 2026." Maya's Birth Plan section read "Hannah read this on Oct 28, 2026"; after Maya changed two answers it read "Hannah has not read this yet", and Hannah's **Your care** card "It is ready for you to read." |
| 5.2-a | `manual` | as expected | Her message naming the two changes went into the thread. Maya's reply and edit were her own acts; nothing links the message to the plan |
| 6.1 | `manual` | as expected, cell corrected | On the phone, **Read your birth plan** from **Your care**, then **Print**: the browser's print opened at once. The page's print rules hide the top bar, Back, both exports and **I've read this**. The pilot banner and the footer have none and print ([#2212](https://github.com/markgoho/doula-cloud/issues/2212)). What prints is "Birth Plan" and the answers, with nothing to say whose ([HS-G9](https://github.com/markgoho/doula-cloud/issues/2209)). Read from the page's own `@media print` rules: headless Chrome cannot open a print preview |
| 6.1-a | `manual` (was `missing-feature (PR-G5)`) | **walked** | Maya's **Print for the hospital** opened "Hannah's Birth Plan", "Engagement created Oct 27, 2026", **Print** and **Download Birth Plan (PDF)**. Her print rules leave the same pilot banner |
| 6.1-b | `manual` (new) | **walked** | `GET .../birth-plan/pdf` `200`, 1,583 bytes. Its text: "Birth Plan", "Birth Setting", then the five questions and their answers. No name, no Practice, no date ([HS-G9](https://github.com/markgoho/doula-cloud/issues/2209)) |
| 6.2 | `manual` | as expected, not literally walkable | The product has nothing left to observe once the paper is handed over |
| 7.1 | `manual` | as expected | On the phone, at 02:56 UTC, which is 10:56pm on Oct 27 in Rochester: a message with `ankle-swelling.png`. It appeared under "You" with the photo inline, on her side and on Maya's after a reload. The thread shows times in the reader's zone; this browser's is UTC |
| 7.3 | `manual` | as expected, cell corrected | With 33 messages the thread opened on 30; **Load older messages** brought in the first three, oldest first, in 23 ms, and went away. No edit or delete control on any message |
| 7.3-a | `manual` (was `missing-feature (HS-G4)`) | **walked** | The Notifications screen says it. The thread says nothing |
| 8.1 | `manual` | as expected, cell corrected | After the live birth was recorded: **Your care** still reads "Your due date is Mar 10, 2027." and the Birth Plan card is unchanged ([HS-G11](https://github.com/markgoho/doula-cloud/issues/2211)) |
| 8.1-a | `manual` (was `missing-feature (MO-G4)`) | **walked** | The first scheduled Visit made Maya her Doula and moved the care phase to Active; her list says "Your care is now ongoing." and "A visit was added to your care." The recorded birth moved nothing |
| 8.1-b | `manual` | **falsified**, cell corrected | Visits are not invisible: **Your visits** reads "Wed Nov 18, 10:00am", Who "Maya Okonkwo". The birth itself appears nowhere, since the outcome is staff-only |
| 9.1 | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) | confirmed | The account menu holds **Account** (her sign-in address, and sign out of every device) and **Sign out**. Nothing gives a second person access; once she accepted, Maya's page no longer offers **Send portal invite**. #304 is closed as ruled out of v1 on #176, and the step keeps citing it, as Tasha Bell's walk kept citing TB-G6 |

**Re-marked**: 3.3-a, 5.2, 6.1-a, 7.3-a and 8.1-a from `missing-feature` to `manual`, their gaps built. 9.B-screens, 9.B-decisions, 9.B-confirmation and 9.B-cold from `manual` to `missing-feature (HS-G5)`, because there is no screen to measure while partner access is out of v1 ([the rule](README.md#budget-steps), and Tasha Bell's stage 7). 6.1-b is new, the check the closed HS-G7 needed. 5.2 stays `manual`: `birth-plan.e2e.ts` presses **I've read this** but does not assert that a Staff edit clears it or that nothing on the page is editable. The Marks summary is recounted to 16 / 51 / 0 / 5, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**, as on Tasha Bell's and Maya Okonkwo's walks: fields, options, and links or buttons that move her somewhere or change what happens, enabled and in view at once in the page body. **Back**, the skip link and the Privacy Policy link are not counted. The portal's top bar is given beside each number: five destinations and the account menu, 6. Stage 3's budget note counts the five destinations, so stage 3's number is the top bar's. **Time** is measured in the page: a script presses the control and polls each animation frame until the result has settled on screen, and records when the control first showed it was busy.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 2.B-screens | 2 | 2: the accept page, then **Your care** | yes |
| 2.B-decisions | 1 | 1: **Continue** | yes |
| 2.B-confirmation | yes | yes. She lands on **Your care**, "Okonkwo Birth Support", with what happens next. The accept page itself names no Practice ("Your doula invited you to see your care online.") | yes |
| 2.B-cold | yes | yes, in a fresh context | yes |
| 3.B-screens | 1 | 1 | yes |
| 3.B-decisions | 5 | 5 destinations in the top bar, as the budget note counts; the body holds 4 (**Read your birth plan**, **Open your messages**, **Invoices**, **Show what has happened**) | yes |
| 3.B-time | 400 ms, 100 ms | Top-bar moves 48, 80, 66, 66 and 50 ms on first visits, then 16 to 50 ms. **Continue** on the accept page settled on **Your care** in 225 ms, busy within 2 ms | yes |
| 3.B-cold | yes | yes. Each card says what it holds; Notifications explains itself before it asks | yes |
| 4.B-screens | 1 | 1 | yes |
| 4.B-decisions | 3 | 1 on the disclosure (**I agree to sign electronically, continue**), then 3: Full legal name, the attestation, **Sign** | yes |
| 4.B-memory | yes | yes. The prose and its values stay above the name field | yes |
| 4.B-confirmation | yes | yes. "Signed", "Okonkwo Birth Support can see it is signed", and the download. The note's "when" is not on this page: the signed date is on **Your care** ("You signed this on Oct 28, 2026.") | yes |
| 4.B-cold | yes | yes | yes |
| 5.B-screens | 1 | 1 | yes |
| 5.B-decisions | 2 | **3**: **Print**, **Download Birth Plan (PDF)**, **I've read this**. Neither of the note's two (correct it, or tell Maya) is on the page | **no**, [HS-G8](https://github.com/markgoho/doula-cloud/issues/2208) |
| 5.B-time | 400 ms, 100 ms | Open 37 ms; **I've read this** 37 ms, busy within 2 ms | yes |
| 6.B-screens | 1 | 1: the Birth Plan, one tap from **Your care** or the top bar | yes |
| 6.B-decisions | 1 | **2** once read (**Print**, **Download Birth Plan (PDF)**), 3 before | **no**, [HS-G8](https://github.com/markgoho/doula-cloud/issues/2208) |
| 6.B-memory | yes | yes. Nothing to remember: the plan is one tap away, on the phone, whenever she needs it | yes |
| 6.B-time | 400 ms, 100 ms | Open 59 ms on the phone; **Print** called the browser's print in under 1 ms | yes |
| 6.B-confirmation | yes | **no**. Neither the printed sheet nor the PDF names her, her Practice, or when the plan last changed. Maya's printout names "Hannah" and when the Engagement was created | **no**, [HS-G9](https://github.com/markgoho/doula-cloud/issues/2209) |
| 6.B-cold | yes | yes. **Print** is the first control on the page | yes |
| 7.B-screens | 1 | 1 | yes |
| 7.B-decisions | 3 | 3: Message, Attachment, **Send** | yes |
| 7.B-time | 400 ms, 100 ms | **Send** 47 ms, with a photo 52 ms, busy within 4 ms; 29 more sends 16 to 33 ms | yes |
| 7.B-confirmation | yes | **no**. The message appears under "You"; nothing says it was sent or what happens next, and no status region announces anything. Maya's side says "Message sent." | **no**, [HS-G10](https://github.com/markgoho/doula-cloud/issues/2210) |
| 8.B-screens | 1 | 1 | yes |
| 8.B-time | 400 ms, 100 ms | **Your care** from the top bar 53 ms on the phone | yes |

The 320px budget steps are `automated (portal-320.e2e.ts)` or `automated (route-continuum.svelte.spec.ts)` and were not re-measured. Stage 9's four are `missing-feature (HS-G5)` and have nothing to measure. **Failed**: 5.B-decisions, 6.B-decisions, 6.B-confirmation, 7.B-confirmation.

#### Moment of truth — the verdict, against the first run's

**Stage 6, printing the Birth Plan and handing it to a stranger in scrubs.** First run (2026-08-23): the mechanism worked (the print stylesheet removed the chrome) and everything around it was missing: no export but Print, no way for Maya to hand it over, and a plan Hannah could read but not acknowledge. **This run: the mechanism holds and grew, and the paper fails at its one job.** From the phone she reaches the plan in one tap and prints it, or downloads a PDF; Maya can print the same plan from her side; Hannah confirmed she had read it, and Maya's edit after her message cleared that until she reads it again. But the stranger in scrubs is handed a sheet headed "Birth Plan" that does not say whose it is, which Practice wrote it, or whether it is current, and the portal's copy also carries the pilot banner. The first run could not hand it over well; this run hands over a page that cannot say whose it is ([HS-G9](https://github.com/markgoho/doula-cloud/issues/2209), [#2212](https://github.com/markgoho/doula-cloud/issues/2212)).

#### Is the journey completable end to end?

**Yes.** Against "a pass means": a portal account from the emailed invite; a signed Contract with her copy downloaded; a Birth Plan she read, confirmed and printed; a thread with traffic both ways, a photo included; and an Engagement whose care phase and Visits follow her care. No step stops her, and every `missing-feature` step from the first run but 9.1 is walked. Two things fall short of the letter: the printed plan does not say whose it is (HS-G9), and after the birth **Your care** still names her due date as ahead (HS-G11), so "reflects where she actually is" holds until the baby comes. Stage 9 is out of v1 by decision, not a stop.

**The portal's Pay button works on this stack.** Not a step of her plan. Maya raised an Invoice on the signed Contract (**Create Invoice**, no billing-mode question, since the Practice bills by card already): `9ZFHEKB6-0001`, $1,800.00, Raised Oct 27, 2026 on Maya's side and "Sent Oct 28, 2026" on Hannah's (the zone split [#2205](https://github.com/markgoho/doula-cloud/issues/2205) holds). On the phone, the portal's Invoice page loaded Stripe's Payment Element (Card, Bank, Cash App Pay, Klarna) with no challenge. Card `4242 4242 4242 4242`, 12/30, 123, ZIP 14620, Link left empty, **Pay $1,800.00**: busy within 2 ms, and in 3,194 ms "Your payment was sent. This Invoice will show as Paid once it is confirmed." Her **Invoices** then read Total to pay $0.00, "You have paid every Invoice for this care.", `9ZFHEKB6-0001` **Paid**; Maya's row read **Paid**, Oct 27, 2026, with **Return money**. The `409` the #2171 addendum met was the missing publishable key, now present.

**Findings filed**: [HS-G8 (#2208)](https://github.com/markgoho/doula-cloud/issues/2208), [HS-G9 (#2209)](https://github.com/markgoho/doula-cloud/issues/2209) and [HS-G10 (#2210)](https://github.com/markgoho/doula-cloud/issues/2210), failed budget steps, each on the [journey map](../journeys/first-time-client.md#gaps-found) with its budget line quoted; [HS-G11 (#2211)](https://github.com/markgoho/doula-cloud/issues/2211), the due date after a live birth; and one bug, [#2212](https://github.com/markgoho/doula-cloud/issues/2212), the pilot banner on the printed plan. Cited, not filed: [#2198](https://github.com/markgoho/doula-cloud/issues/2198) ("them") and [#2205](https://github.com/markgoho/doula-cloud/issues/2205) (the raised day). Not filed: the invite email naming no Practice, which ADR-0009 and #221 decided.

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 2, 3, 4, 5, 6, 7, 8, 9 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **40 budget steps**, 8 `automated`, 32 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 2.1-a | `portal-invite-accept.e2e.ts` | pass |
| 2.2 | `portal-invite-accept.e2e.ts` | pass |
| 2.3 | `portal-invite-accept.e2e.ts` | pass |
| 3.1 | `portal-invite-accept.e2e.ts` | pass |
| 5.1-a | `birth-plan.e2e.ts` | pass |
| 7.2 | `push-notification.e2e.ts` | pass |

**6 automated steps: all pass.**

### 2026-09-10 — narrative reconciliation and two newly automated steps ([#685](https://github.com/markgoho/doula-cloud/issues/685))

`bun run test:e2e` in `app/`, whole suite, one run, as the reporter printed it: **1 failed, 1 did not run, 65 passed** (56.1s). The failure is `simulation-world.e2e.ts`, the known flake [#958](https://github.com/markgoho/doula-cloud/issues/958) tracks, and it takes its own dependent case down with it; neither touches a step on this plan. The two specs that do were then re-run on their own: **2 passed** (24.2s).

| Step | Spec | Result |
| --- | --- | --- |
| 4.1 | `contract-lifecycle.e2e.ts` | pass |
| 4.2 | `contract-lifecycle.e2e.ts` | pass |

**2 newly automated steps: both pass**, bringing the plan's total to 8. This is the fact the first run handed forward rather than filing — *stage 4 is the strongest screen in the portal and no spec is on it* — and it is answered: the spec signs the Contract as the Client, in her own browser context, through the same disclosure, name field and attestation Hannah walks.

Two more cells are re-marked, both `missing-feature` -> `manual`, because the gap each named is closed and the step can now be performed:

| Step | Was | Now | What settled it |
| --- | --- | --- | --- |
| 4.1-b | `missing-feature (MO-G6)` | `manual` | A notice names the ordering and the way out of it, **Send Contract** renders disabled beneath it, and the BFF enforces the same precondition. [#255](https://github.com/markgoho/doula-cloud/issues/255) is closed |
| 4.3 | `missing-feature (HS-G3, HS-G6)` | `manual` | **Download signed Contract (PDF)** sits under the signed prose, and the BFF reads the object back to serve it. [#302](https://github.com/markgoho/doula-cloud/issues/302) and [#305](https://github.com/markgoho/doula-cloud/issues/305) are both closed |

And two cells are corrected without moving: 4.1's `Status: sent` and 4.1-a's "for this Engagement" are both the team's words, and the portal speaks the Client register's now (`app/src/lib/clientRegister.ts`, [#212](https://github.com/markgoho/doula-cloud/issues/212)).

The 2026-08-23 walk below is unchanged as a historical record; it walked 4.2 by hand before any spec did.

### 2026-09-20 — stage 9 and stage 2, against #309/#819/#617 as built ([#1244](https://github.com/markgoho/doula-cloud/issues/1244))

A desk pass, correcting two stale claims. Nothing was re-walked. Two steps are corrected and one is dropped.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 2.1-a, 2.2 | password field + account-mode radio -> a single **Continue** button, no password | #617 (ADR-0026): a Client has no password, so accepting an invitation is pressing Continue, nothing else |
| 9.1 | `client_portal_users` holds one `identity_uid` per Client, password-sharing -> `invite()`'s handler-level refusal, mailbox-sharing | The table-wide `UNIQUE` on `identity_uid` was dropped and `UNIQUE (identity_uid, client_id)` put in its place (#309, #819, `00107_portal_account_client_pair_unique.sql`); `invite()` (`portalinvite/invite.go`) refuses a second invite on an already-accepted Client by `client_id`, not by a schema constraint |

**One step is dropped, not corrected.** 2.2-a asked her to repeat accept-invite with "I already have an account" chosen — a choice the screen no longer offers (#617). HS-G1, the gap row it exposed, closes with it ([#300](https://github.com/markgoho/doula-cloud/issues/300), already implemented by #617). The plan's Marks summary and README's run-status row and Total are recounted for the one mark this drops. The walk logs above and below are records of what was seen on the day and are untouched.

### 2026-08-23 — manual and missing-feature steps ([#240](https://github.com/markgoho/doula-cloud/issues/240))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Rooted Birth Collective", Owner+Admin+Doula in one Staff row) with one Client (Hannah Sorensen), a Contract sent with real merge-field values entered, and a filled Birth Plan. Walked in Chrome via playwriter, one browser profile, phone viewport (390×844) for stages 6 and 7.

**Note on the fixture**: confirms the same shared-`__session`-cookie fact [#239](https://github.com/markgoho/doula-cloud/issues/239) already recorded — signing in as Staff or as the Client silently invalidates whichever side was signed in on the other route. Not re-argued here; walked by re-authenticating whichever side was about to act, immediately before each of its steps.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `POST .../portal-invite` returns the token; the page prints the accept-invite URL as a `<code>` block, no email sent |
| 1.2 | `manual` | as expected | The link was used directly, exactly as printed — no verification mechanism exists |
| 2.1 | `manual` | as expected | "Missing invite token", no form |
| 2.2-a | `manual` | as expected | Chosen on Hannah's real, still-unclaimed token: `400`, generic "Accept invite failed"; the token stayed unconsumed and signup succeeded right after |
| 3.1-a | `manual` | as expected | `Status: intake`, `Created: 8/23/2026` |
| 3.2 | `manual` | as expected | **Birth Plan** and **Contract** links only, nothing explains either |
| 3.3 | `manual` | as expected | Nothing on screen asks about notifications |
| 3.3-a | `missing-feature (HS-G4)` [#303](https://github.com/markgoho/doula-cloud/issues/303) | confirmed | No notification setting anywhere in the portal — the banner holds only **Sign out** |
| 4.1-a | `manual` | as expected | "No Contract has been sent for this Engagement yet." while the Contract was still `draft` |
| 4.1-b | `missing-feature (MO-G6)` [#255](https://github.com/markgoho/doula-cloud/issues/255) | confirmed | On a second, never-invited Client, **Send Contract** succeeded (`Status: sent`) with no prior portal invite at all; nothing on the Staff side warns of the ordering |
| 4.1 | `manual`, **[MO-G10](https://github.com/markgoho/doula-cloud/issues/258) reconfirmed** | as expected, with a correction | `Status: sent` renders, but every merge field is blank — real values (`Rooted Birth Collective`, `Hannah Sorensen`, `$2200`, dates) were typed and **Send Contract** pressed, and neither the staff-side form nor the client-side prose ever showed them on reload. Same defect MO-G10 already names (`contracts/contract.go:124`); not a data-entry mistake, confirmed by re-checking the Staff form after a fresh login |
| 4.2 | `manual` | as expected | Full legal name typed, attestation checked, **Sign** re-renders the page at `Status: signed` |
| 4.3 | `missing-feature (HS-G3)` [#302](https://github.com/markgoho/doula-cloud/issues/302), sharpened, new gap **[HS-G6](https://github.com/markgoho/doula-cloud/issues/305)** minted | **falsified in part** | The plan's claim was that the endpoint is merely unlinked. It is worse: `GET /api/portal/engagements/{id}/contract/pdf` itself answers `500 internal error` when called directly (`fetch()` from the page), even though the signed Contract's `status` is `signed`. Querying `fake-gcs-server`'s own API directly (`GET /storage/v1/b/doula-cloud-e2e-attachments/o`) shows the PDF *is* present at `contracts/{engagementId}/signed.pdf` and downloads fine through the emulator's own `mediaLink` — so the object write and the object itself are both fine. The break is in the BFF's read path: `objectstore.GCSStore.Get` (`api/internal/objectstore/gcs.go`) is backed by a `storage.Client` constructed with no options at all (`storage.NewClient(context.Background())`, `main.go:263`), a known incompatibility class between the Go GCS client's newer default read behavior and `fake-gcs-server`. Whether this also breaks against real GCS in Cloud Run is unverified from this walk — a local-only harness defect and a real product defect look identical from here, and that ambiguity belongs to whoever picks up HS-G6, not this ticket |
| 5.1 | `manual` | as expected | "No Birth Plan has been created for this Engagement yet." |
| 5.2 | `missing-feature (HS-G2)` [#301](https://github.com/markgoho/doula-cloud/issues/301) | confirmed | Fully read-only rendering (`Hospital`, notify list, atmosphere text, photo preference) — no editable field, no comment box, no acknowledgement control anywhere on the page <!-- spelling:ignore: a dated Run log entry keeps the words it was written with (#869) --> |
| 5.2-a | `manual` | as expected | A message referencing the Birth Plan sends and appears in the thread immediately |
| 6.1 | `manual` | as expected | `page.emulateMedia({ media: 'print' })` removes **Sign out**, **Back** and **Print** from the accessibility tree, leaving only the Plan content — the print stylesheet works |
| 6.1-a | `missing-feature (PR-G5)` [#280](https://github.com/markgoho/doula-cloud/issues/280) | confirmed | Staff's Birth Plan section offers only the edit form and **Save Birth Plan** — no print-for-client, deep link, or hand-over control of any kind |
| 6.2 | `manual` | as expected, not literally walkable | Conceptual — the product has nothing left to observe once the paper is handed over |
| 7.1 | `manual`, new gap **[HS-G6](https://github.com/markgoho/doula-cloud/issues/305)** | **as expected on upload, falsified on delivery** | A message with a `pixel.png` attachment posts successfully (`201`) and appears in the thread as a clickable `pixel.png` control — but clicking it, and fetching the attachment URL directly, both answer `500 internal error` and surface a visible **internal error** alert to the Client. Same root as 4.3: the object is written (confirmed present in `fake-gcs-server`) and the BFF's `Get` cannot read it back. "Attachments work both ways" does not hold on this stack — nothing renders inline, on either side of the exchange, once the read path is exercised for real for the first time |
| 7.3 | `manual`, not exercised | inconclusive | Only one message existed in the thread; no **Load older** control rendered. Not evidence of a defect — the fixture never produced enough history to trigger it |
| 7.3-a | `missing-feature (HS-G4)` [#303](https://github.com/markgoho/doula-cloud/issues/303) | confirmed | Nothing near the thread says a push is not an alarm |
| 8.1 | `manual` | as expected | Same heading, same `intake`, same two links, revisited after a reload |
| 8.1-a | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | confirmed | No status control exists anywhere on either side |
| 8.1-b | `manual` | as expected | No Visits section anywhere on the Client side of the product |
| 9.1 | `missing-feature (HS-G5)` [#304](https://github.com/markgoho/doula-cloud/issues/304) | confirmed | No second-account, guest, or share control anywhere in the portal — the banner holds only **Sign out** |

**26 steps walked: 18 `manual`, 8 `missing-feature`. Every existing mark holds; 4.3 was falsified in part (sharpened, not deleted) and two new gaps minted — HS-G6 and HS-G7 — added to [docs/journeys/first-time-client.md](../journeys/first-time-client.md).** No `blocked` step exists on this plan, confirmed. No `journey-gap` issue filed from this ticket — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict**: this plan cannot pass, as written, and does not. Every `missing-feature` step confirmed genuinely unwalkable; the Contract's own merge fields never render, and what looked like a merely-unlinked PDF copy (HS-G3) turned out to be a PDF copy that cannot be fetched at all — the same defect silently breaks message attachments in both directions.

**Facts handed forward, not gaps**: stage 4 remains the strongest screen in the portal with no spec on it — 4.2 signs a Contract and the suite never does. And the "Birth Plan has no export but Print" callout the plan itself flagged for this run is now minted as HS-G7, so it no longer needs carrying forward.
