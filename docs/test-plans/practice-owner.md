# Renata Alvarez — test plan

- **Journey**: [practice-owner.md](../journeys/practice-owner.md)
- **Persona**: [practice-owner.md](../personas/practice-owner.md)
- **A pass means**: a new Doula has accepted an invitation, holds the Doula role, and is on a live Engagement; and one screen shows every Engagement in the Practice with its Contract and Invoice state.

## Preconditions

- **Stage 0** starts from no account: an address that has no Identity Platform account, on the pilot allowlist or with the pilot gate open (`dev:full` and the e2e stack open it as they start, `docs/environment.md`). After step 0.3, and before step 0.4, two employee Doulas accept Invitations to her Practice. This is fixture setup, not a step, because the route has no Invitation on it (`docs/onboarding-route.md`): `POST /api/practices/{id}/staff/invitations` with `roles` `["doula"]` and `employmentType` `employee`, as Renata; then for each invitee an Identity Platform account and `POST /api/staff/accept-invite` with the token from `staff_invite_outbox`.
- **Stages 1 to 9** start from the Practice that Stage 0 makes, with at least two Clients with Engagements. A second Client is added from the Clients list (search, then **Add a new Client**), or provisioned with `POST /api/practices/{id}/clients` as the specs do.
- A second Identity Platform account for the invitee in Stage 2, and a way to read the invitation token (`staff_invite_outbox`, or the sandbox mailbox).

## Steps

### Stage 0 — Sign up the agency and start work with the first Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 0.1 | Open `/signup` | "About you", a question page in the journey "Setting up your Practice" (step 1 of 3), with First name, Last name, Email and Password (with its show and hide control and the 15-character rule as its hint). No checkbox, no second password field, no question about a team. **Continue** opens "What is your Practice called? (optional)", then "Where do you work?" with the state she works from and Timezone (opening on the zone the browser reports), then "Check your answers", where directly above **Create your Practice** the agreement sentence is the button's accessible description and each document is a link that opens in a new tab ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)) | `manual` |
| 0.2 | Answer each page, with "Rooted Birth Collective" as the Practice name, and press **Create your Practice** on the check page | `POST /api/staff/signup` succeeds, and the browser lands on `/practices/[practiceId]` with the heading "Welcome to Rooted Birth Collective" and, first, "You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose." No screen is between, and no second factor is asked for | `automated (signup-form.e2e.ts)` |
| 0.3 | Read the empty Practice | "Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice.", then "Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits.", then one link, **Add your first Client**. No Getting paid card, no second-factor notice, no link to the Invitation form, and no word "Stripe" | `manual` |
| 0.3-a | After the fixture's two Invitations are accepted, reload the empty Practice | The same words and the same one link. Her first Invitation sent is one of the three acts after which a second factor is recommended (#1533), and the notice still does not show on the empty Practice | `manual` |
| 0.4 | Press **Add your first Client** | "What is the Client's name?", with Given name, Family name (optional) and Preferred name (optional), and one button, **Save and continue**. No search is in front of it | `manual` |
| 0.5 | Answer the name and press **Save and continue** | The Client is saved, and the Start work form opens with focus on "{given name} saved as a Client." and the sentence "Starting work with {name} uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2." | `automated (add-client-visits.e2e.ts)` |
| 0.6 | Read "Who is the Doula?" on a Practice with three employee Doulas, then press **Start work with {name}** with no answer to it | "Renata Alvarez (you)", then each employee Doula, then "No Doula yet", and **no option is selected**. The press is refused with "Select who the Doula is, or select No Doula yet", in the error summary and at the question. `manual`: `offer.e2e.ts` opens the form only on a Practice whose one Doula is the Owner, where her option opens selected | `manual` |
| 0.7 | Select **No Doula yet**, Birth and a due date, and press **Start work with {name}** | The Engagement's page, with "Work with {given name} started.". The Doula is "No Doula yet", and On call says "Nobody is on this birth yet." One Credit is locked, and the Practice overview then shows the second-factor notice and the Getting paid card | `manual` |
| 0.7-a | On a second walk of the stage, select an employee Doula in place of **No Doula yet** | The Engagement's page names her as the Doula, and On call gives her an On-call window around the due date | `manual` |
| 0.B-screens | Count the screens from the stage's entry to its end | **Screens**: 5 (Hick's Law, Flow). The signup form, the empty Practice, the name question, the Start work form, the Engagement's page | `manual` |
| 0.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law). A radio group is one decision | `manual` |
| 0.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The Client's name is in the Start work form's heading and button | `manual` |
| 0.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). "Work with {name} started.", and the Doula row says who is on the Engagement | `manual` |
| 0.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 0.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 1 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Sign in at `/login` | `POST /api/session` sets `__session`; the browser lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 1.2 | Choose Rooted Birth Collective from her memberships | **There is no choosing.** With one membership `decideLanding` redirects straight to `/practices/{id}` (`app/src/lib/landing.ts:24-26`); the `Choose a Practice` picker renders only for two or more, which her own journey never gives her — only Lena Vasquez's does | `manual` |
| 1.3 | Read the tiles | All seven render for her — but only **four** are owner-gated (Invite, Staff, Plan Templates, Contract Template). **Payments sits outside the gate** (`app/src/routes/practices/[practiceId]/+page.svelte:76-81`), so every member sees it, roles or none ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) | `manual` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). `/login`, then the Practice landing; one membership needs no picker | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `manual` |
| 1.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

The membership picker (1.2) is exercised by no spec — every spec's Staff member belongs to exactly one Practice, so the multi-membership path is untested here and is Lena's normal case.

### Stage 2 — Invite a new Doula

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/practices/[practiceId]/invite` | The form renders for an Owner | `manual` |
| 2.2 | Enter a name and email, press **Send invite** | `POST /api/practices/{id}/invitations` succeeds and the screen **prints a link**; no email leaves the system | `manual` |
| 2.2-a | Check the invitee's inbox | Nothing arrives. No invitation email is sent by anything | `missing-feature (RA-G1)` [#260](https://github.com/markgoho/doula-cloud/issues/260) |
| 2.3 | Deliver the link out of band | The invitee receives a raw URL by text or Renata's own mail client, which they cannot verify is genuine | `manual` |
| 2.4 | Have the invitee accept at `/accept-invite` | `POST /api/staff/accept-invite` creates the membership with `roles = '{}'` — a zero-role member is the only possible outcome of inviting anyone | `manual` |
| 2.4-a | Look for a roles control on the invite form | The invitation carries no roles at all | `missing-feature (RA-G8)` [#266](https://github.com/markgoho/doula-cloud/issues/266) |
| 2.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 2.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). Name, email, role, and **Send invite** | `manual` |
| 2.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen says the invitation went, to whom, and what the invitee does next | `manual` |
| 2.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 2.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 3 — Set the new Doula's roles

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/practices/[practiceId]/staff` | The roster loads; the `GET` is owner-gated and passes for Renata | `manual` |
| 3.2 | Read the Roles column | Renders the team's words via `rolesLabel` — an Admin shows as `Admin` (closed by [#262](https://github.com/markgoho/doula-cloud/issues/262)) | `manual` |
| 3.3 | Change a member's roles from the screen | **Edit membership** opens roles and employment type on one form and saves both (#316). No staff id is needed and no terminal is involved, so a roster can be built in the product | `manual` |
| 3.4 | Read the Pending invitations group | Addresses asked but not answered, each with its roles, employment type, expiry and a **Revoke** action — a second group, not a row in the members list | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 5 (Hick's Law, Miller's Law). **Edit membership**: the three roles, the employment type, and the save | `manual` |
| 3.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 3.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 4 — Put the Doula on Engagements

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open an Engagement from the Clients list | The single-page Engagement view renders | `automated (birth-plan.e2e.ts)` |
| 4.2 | Look for a control that puts a Doula on the Engagement | No field, no endpoint, no screen. An Engagement carries no Doula | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| 4.3 | Add a Visit naming the new Doula as `staffId` | **The Visit names her.** Create carries its own assignee now, and the Engagement page asks **Who is this Visit for?** as a picker of the Doulas this Engagement can admit — no UUID, no free-text box ([RA-G10](https://github.com/markgoho/doula-cloud/issues/268) closed). `resolveAssignee` (`api/internal/visit/roles.go`) keeps naming a colleague with the Owner and the Admin, and logging your own Visit with the Doula, so Renata reaches it from either seat. The row she writes carries a scheduled date and time, a type and notes. Assignment still exists at Visit level; whether an Engagement itself names a Doula is 4.2's question, not this one | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). Which Doula, and the save | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 5 — Reassign when someone is sick

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | `PATCH .../visits/{visitId}` with a new `staffId` | The Visit's Staff member changes, chosen from the row's own **Reassign to** picker rather than pasted as an id, and it is refused unless the person named is a Doula this Engagement can admit. Something dated does move now: a Visit carries a scheduled instant, rescheduled from **Update schedule** on the same row | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). Who takes it instead, and the save | `manual` |
| 5.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — See the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open `/practices/[practiceId]/clients` | The list renders and each Client links to their Engagement | `automated (birth-plan.e2e.ts)` |
| 6.1-a | Create a Client as a *second* Staff member, then reload as Renata | She appears: the list is one row per Client at the Practice, whoever saved her, and no longer one row per Client-and-Engagement pair (ADR-0017 — a Client needs no Engagement to exist at all). **This half of her requirement passes** | `manual` |
| 6.1-b | Read the columns | Name and Status only — and Status is `intake` on every row forever ([MO-G4](https://github.com/markgoho/doula-cloud/issues/253)) | `manual` |
| 6.2 | Learn each Engagement's Contract and Invoice state | Reachable only by opening every Engagement in turn | `manual` |
| 6.2-a | Look for a roll-up of Contract state, Invoice state, or covering Doula | There is none at any level above one Engagement | `missing-feature (RA-G6)` [#264](https://github.com/markgoho/doula-cloud/issues/264) |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). One list answers each Engagement's Contract, Invoice and covering Doula, with no Engagement opened | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — See the money across all Staff

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open `/practices/[practiceId]/billing` | Credit balance and purchase ledger render — DoulaCloud's own billing, not Client money | `automated (billing.e2e.ts)` |
| 7.2 | Look for unpaid Client Invoices | No Practice-wide Invoice list and no unpaid view exist | `missing-feature (RA-G7)` [#265](https://github.com/markgoho/doula-cloud/issues/265) |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). One list of unpaid Client Invoices across every Engagement | `manual` |
| 7.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 8 — Coverage, at 2 a.m. (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Sign in on a phone | The practice screen renders on a small viewport | `manual` |
| 8.2 | Find who is free tonight | No availability, on-call, or coverage surface exists — and dateless Visits ([MO-G1](https://github.com/markgoho/doula-cloud/issues/250)) mean the data one would read does not exist either | `missing-feature (RA-G5)` [#263](https://github.com/markgoho/doula-cloud/issues/263) |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). Signed in on the phone, then the one screen that says who is free tonight | `manual` |
| 8.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law) | `manual` |
| 8.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 8.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen names who is free and who is already at a birth, as of now | `manual` |
| 8.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User). At 2 a.m., on a screen she may never have opened | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 9 — Edit Plan Templates for the whole Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Add a field to the Birth Plan template and save | `PUT .../plan-templates/{planType}` succeeds for an Owner and persists | `automated (plan-templates.e2e.ts)` |
| 9.2 | Reopen an already-filled Birth Plan | Unchanged — the Plan Instance snapshots the field definitions at creation. **Passes** | `manual` |
| 9.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 9.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law) | `manual` |
| 9.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 9.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 17 (10 of them [budget steps](README.md#budget-steps)) |
| `manual` | 61 (37 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 6 ([RA-G1](https://github.com/markgoho/doula-cloud/issues/260), [RA-G4](https://github.com/markgoho/doula-cloud/issues/225), [RA-G5](https://github.com/markgoho/doula-cloud/issues/263), [RA-G6](https://github.com/markgoho/doula-cloud/issues/264), [RA-G7](https://github.com/markgoho/doula-cloud/issues/265), [RA-G8](https://github.com/markgoho/doula-cloud/issues/266)) |

RA-G2 and RA-G3 are observed at 3.3 and 3.2 rather than given steps of their own: both screens render, so both steps are walkable. RA-G3's failure is the word the screen prints, and 3.3's **Edit membership** has since answered RA-G2's own question, which is why that step reads `manual` and RA-G2 is no longer counted above. **RA-G9** and **RA-G10** were minted by the walk and are observed the same way, inside 1.3 and 4.3: both steps can be performed, and what the product does when they are is the finding.

## Run log

### 2026-10-23 — Stage 0 added, and the route walked against the spec ([#1649](https://github.com/markgoho/doula-cloud/issues/1649))

Stage 0 is new on the journey map and here: **15 steps**, 3 `automated` (one of them a budget step) and 12 `manual` (five of them budget steps). 0.1 and 0.4 are `manual` because no spec asserts their whole result: `signup-form.e2e.ts` asserts the agreement sentence and the landing but not the heading, the field order, the role notice or the timezone the form opens on, which it sets itself; `add-client-visits.e2e.ts` asserts the name question's heading but not its third field, its one button or the absence of the search. 0.2 and 0.5 are `automated`, because each spec asserts the result that the step names. No step on stages 1 to 9 is re-marked. The Preconditions no longer send the reader to Maya's plan for signup. The Marks summary above and [README.md](README.md)'s run-status row move with them.

`bun run dev:full` in `app/` on trunk at `6c1985d4`, walked in a fresh headless Chrome profile through `playwriter`, at 1280x900 and at 320x640, with a new throwaway account for each walk. The two employee Doulas were made by fixture setup after step 0.3 (see Preconditions), and the fixture has no mark. Each step was walked by hand, the `automated` ones too; the e2e suite was not re-run. **No Doula yet** was answered on the 1280px walk and a named Doula on the 320px walk, and the question opened with no option selected on both. Maya Okonkwo's plan records the walk as a solo Owner, in its own Run log.

**The count**, against `docs/onboarding-route.md`:

| Walk | Width | Acts | Presses | Screens | Exits from the app |
| --- | --- | --- | --- | --- | --- |
| Agency Owner, "Rooted Birth Collective", Doulas Priya Raman and Jo Mercer, answer **No Doula yet** | 1280px | 3 | 4 | 5 | 0 |
| Agency Owner, "Rooted Birth Collective of the Greater Rochester and Finger Lakes Region", Doulas Priya Raman and Alexandrina Vasquez-Montgomery-Okafor, answer **Priya Raman** | 320px | 3 | 4 | 5 | 0 |

**The count agrees with the spec**: 3 acts and 4 presses on 5 screens, with no exit from the app. No ticket is filed for the count. The walk on [#1488](https://github.com/markgoho/doula-cloud/issues/1488) found 5 acts, 17 presses and 2 exits on 2026-09-27. The refusals that the walk pressed for on purpose (an empty form, a short password, a common password, an empty name, no answer to "Who is the Doula?") are not counted as presses. The Staff shell's navigation, the Pilot notice and its feedback link are on screens 2 to 5 and are not part of the five screens' words; nothing on the route needs them.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 0.1 | `manual` | as expected | Each label, hint and refusal word for word as Screen 1 of the spec: "Enter your first name", "Enter your last name", "Choose the state you work from", "Enter your email address", "Enter a password" in the error summary in the order of the fields; "Password must be 15 characters or more" for `short`; "Password is too common. Enter a different password" for `passwordpassword`. Timezone opened on Eastern, the zone the browser reported. `autocomplete` is `given-name` and `family-name`, one password field, no checkbox. **The links' visible words are "Terms of Service (opens in new tab)" and "Privacy Policy (opens in new tab)"**, from the `Link` atom ([#1557](https://github.com/markgoho/doula-cloud/issues/1557)'s GOV.UK rule), where the spec quoted the sentence without them. **Corrected in the document**: Screen 1's row 8 now says so. The two links go to `https://doula.cloud/terms` and `/privacy`, which wait for the domain cutover |
| 0.2 | `automated (signup-form.e2e.ts)` | as expected | Press 1. The empty Practice in 804 ms from the click, wall clock to the heading, with no screen between and no second factor asked for |
| 0.3 | `manual` | as expected | Each sentence word for word as Screen 2 of the spec, and one link. No word "Stripe" on the page, no Getting paid card, no second-factor notice, and nothing in `main` but the heading, the two paragraphs and the link |
| 0.3-a | `manual` | as expected | After the two Invitations were sent and accepted, the reload showed the same words, and no second-factor notice ([#1533](https://github.com/markgoho/doula-cloud/issues/1533)) |
| 0.4 | `manual` | as expected | Press 2. Each word as Screen 3 of the spec, and "Enter the Client's given name" for an empty name. **Back** goes to the empty Practice |
| 0.5 | `automated (add-client-visits.e2e.ts)` | as expected, plus one sentence the spec did not have | Press 3. The Credits sentence word for word. The form opens with focus on "Hannah saved as a Client." ([#1710](https://github.com/markgoho/doula-cloud/issues/1710)), which the spec's Screen 4 did not list. **Corrected in the document**: Screen 4 now has the row |
| 0.6 | `manual` | as expected | "Renata Alvarez (you)", "Jo Mercer", "Priya Raman", "No Doula yet", none checked. With Birth and a due date and no Doula, the press was refused with "Select who the Doula is, or select No Doula yet", in the summary and at the question. At 320px "Alexandrina Vasquez-Montgomery-Okafor" wraps to two lines beside its circle, so [#1518](https://github.com/markgoho/doula-cloud/issues/1518)'s fix holds |
| 0.7 | `manual` | as expected | Press 4. "Work with Hannah started.", Doula "No Doula yet", On call "Nobody is on this birth yet.", and **Put me on this Engagement** offered. The overview then showed "Add a second step when you sign in" first in the main column, and the Getting paid card with the words of the spec's section 4 |
| 0.7-a | `manual` | as expected | At 320px with **Priya Raman** selected: "Work with Camille started.", Doula "Priya Raman", and On call "On call from Feb 8, 2027 to Mar 15, 2027." for a due date of Mar 1, 2027 |
| 0.B-screens | `manual` | **5**, passes | The signup form, the empty Practice, the name question, the Start work form, the Engagement's page, at both widths |
| 0.B-decisions | `manual` | **6** at most, under the budget of 7, passes | The Start work form has six: the kind of work, the due date, "Who is the Doula?", the Note, the button and the second action. At 1280x900 the signup form shows five of its seven fields at once and the button is below them |
| 0.B-memory | `manual` | **yes**, passes | The only later need for an earlier answer is the Client's name, which is in the Start work form's heading, its Credits sentence and its button |
| 0.B-confirmation | `manual` | **yes**, passes | "Work with {name} started.", the Doula row, and **Add Hannah Sorensen's details** as the next thing to do |
| 0.B-cold | `manual` | **yes**, passes | A fresh profile, nothing explained first, no tour and no tooltip. Each refusal says what to do at its field |
| 0.B-320px | `automated (route-continuum.svelte.spec.ts)` | **yes**, passes | Not re-run as a spec. On the walk each of the five screens was complete at 320px with no sideways scroll (`scrollWidth` 320), with a 72-character Practice name and a 37-character Doula name. The hit targets under 44px below do not fail this line: ADR-0024's 320px commitment is that the screen is complete and never scrolls sideways, and it promises nothing about touch-target size |

**Every other sentence on the five screens agrees with the spec**, at both widths: the role notice, the agreement sentence apart from the two links' words, Screen 2's two paragraphs, Screen 3's question, hint, labels and refusal, Screen 4's Credits sentence, labels, refusals, the empty Note hint and the second action, and Screen 5's status message. "{name}" is the given name in "Work with Hannah started." and the full name in **Add Hannah Sorensen's details** and in Screen 4's heading; each is the Client's name "as the product shows it", so the spec's "(name)" holds.

**What the walk found that is not a count or a sentence**, each filed or added to a ticket that exists. None is a gap: each is a screen that is built and has a layout defect, not a capability that is missing, so no gap ID is minted on this map, the same way the walk on #1488 filed [#1512](https://github.com/markgoho/doula-cloud/issues/1512) and [#1518](https://github.com/markgoho/doula-cloud/issues/1518).

- **At 1280px, screens 2 to 5 scroll sideways** by 57 to 62px: the Staff header's account button sits past the right edge. At 320px no screen scrolls sideways. Measured and added to [The Staff header's account menu may reach past a 1280px viewport (#1803)](https://github.com/markgoho/doula-cloud/issues/1803). ADR-0024 does forbid sideways scroll at every width, so this is a layout defect and not only a polish item. It is not a gap of this stage: no budget line of Stage 0 is broken by it (its 320px line passes), and the cause is the Staff shell's header, which is on every signed-in Staff screen and not only on this route. #1803 owns it.
- **At 320px, the default Button is under 44px**: **Create Practice** 41.25px, **Save and continue** and **Start work with {name}** 41.3px. That is [A default Button is under the 44px hit target at 320px (#1805)](https://github.com/markgoho/doula-cloud/issues/1805).
- **A link on its own line is under 44px at every width**: **Add your first Client**, the empty Practice's one action, is 23.3px tall at 320px and 26px at 1280px, and **Back** is 21.7px. Filed as [A link on its own line is under the 44px hit target (#1828)](https://github.com/markgoho/doula-cloud/issues/1828).

**Time is not a budget line on Stage 0**, because signup and the first Client are not acts she repeats. The presses took 620 to 804 ms from the click to the next screen's heading, wall clock, through the navigation; that is not the DevTools measure that a Time step names, and it is recorded here only as what it is.

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 2, 3, 4, 5, 6, 7, 8, 9 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **41 budget steps**, 9 `automated`, 32 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.1 | `staff-login.e2e.ts` | pass |
| 4.1 | `birth-plan.e2e.ts` | pass |
| 6.1 | `birth-plan.e2e.ts` | pass |
| 7.1 | `billing.e2e.ts` | pass |
| 9.1 | `plan-templates.e2e.ts` | pass |

**5 automated steps: all pass.**

### 2026-09-10 — narrative reconciliation ([#685](https://github.com/markgoho/doula-cloud/issues/685))

A desk pass, not a walk. [#318](https://github.com/markgoho/doula-cloud/issues/318) added `contract-lifecycle.e2e.ts` and `add-client-visits.e2e.ts` and deliberately left this plan's own Add Client and Visits wording alone, because that wording had already gone stale for reasons of its own. This corrects the wording against the code as it stands; nothing here was re-walked and no mark moved.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 4.3 | The Visit could not name anybody -> it names her, from a picker of the Doulas this Engagement can admit | `api/internal/visit/roles.go`'s `resolveAssignee`, and the Engagement page's **Who is this Visit for?** field. [RA-G10](https://github.com/markgoho/doula-cloud/issues/268) is closed |
| 5.1 | "nothing dated moves, because nothing is dated" -> a Visit carries a scheduled instant, and the reassign is a picker rather than a pasted id | The Visits table's own Type / Date / Notes columns and its **Update schedule** control. [MO-G1](https://github.com/markgoho/doula-cloud/issues/250), [MO-G2](https://github.com/markgoho/doula-cloud/issues/251) and [PR-G6](https://github.com/markgoho/doula-cloud/issues/281) are all closed |
| 6.1-a | "every Client with an Engagement" -> one row per Client, whoever saved her, and a Client needs no Engagement to exist | ADR-0017, and `api/internal/client/list.go`'s `ListItem` doc comment |

**The Marks summary above is also recounted.** It read 16 `manual` and 7 `missing-feature` against a Steps table holding 17 and 6: step 3.3 stopped being `missing-feature (RA-G2)` when **Edit membership** landed, and the summary was never brought along. The count is arithmetic rather than a re-mark, and the same drift is corrected on three other plans and in the README's run-status table under this ticket.

**No step is re-marked.** Neither new spec drives a step on this plan the way Renata would: `add-client-visits.e2e.ts` logs a Visit for the Owner herself, which is not 4.3's act of naming a colleague, and this plan has no Contract-signing step at all.

**Left alone on purpose.** 4.2 still reads `missing-feature (RA-G4)` against a closed [#225](https://github.com/markgoho/doula-cloud/issues/225). It is an Engagement-assignment step, not one of the three this ticket reviewed, and re-marking it needs the walk [#329](https://github.com/markgoho/doula-cloud/issues/329) owns.

### 2026-08-22 — manual walk ([#235](https://github.com/markgoho/doula-cloud/issues/235))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x900 as Renata Alvarez, with a second context for the invitee and a 390x844 iPhone context for stage 8. Preconditions built as the plan allows: `POST /api/staff/signup` for `Rooted Birth Collective`, then two Clients through `POST .../clients` — two of the three signup credits, leaving one for 6.1-a. The 5 `automated` steps were **not** re-run. No step is `blocked`: this plan has none, and Stripe is never reached.

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
| 7.2 | `missing-feature (RA-G7)` [#265](https://github.com/markgoho/doula-cloud/issues/265) | as expected | Confirmed unwalkable. **Billing** is `Credit balance: 0`, a Date / Origin / Quantity ledger, `Quantity` and **Buy credits** — DoulaCloud's money, not her Clients'. The only invoice route in the whole API hangs off one Engagement's Contract (`api/main.go:210-213`) |
| 8.1 | `manual` | as expected | Signed in on a 390x844 iPhone context. The practice screen renders, the seven links stack, and `document.documentElement.scrollWidth === clientWidth === 390` — no horizontal overflow. The phone is not the problem |
| 8.2 | `missing-feature (RA-G5)` [#263](https://github.com/markgoho/doula-cloud/issues/263) | as expected | Confirmed unwalkable. From the phone the only screens that exist are Clients (Name, Status) and Billing. No availability, no on-call, no coverage — and no route to build one from |
| 9.2 | `manual` | as expected — **passes** | Filled the Birth Plan (`atmosphere: filled-0`), saved, then added `Hospital transfer wishes` to the Birth Plan template and saved that. Reopening the Engagement showed the original five fields and the kept answer; the new field is in the template response and **absent** from the instance. The snapshot holds |

**16 `manual` steps walked; 7 `missing-feature` steps confirmed unwalkable; no `blocked` step on this plan.** Three expected results were falsified — 1.2, 1.3 and 4.3 — minting **RA-G9** and **RA-G10** on the journey map; the plan's own cells are corrected above. No `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and the one half that passes is the half she did not ask about.** A new Doula did accept an invitation and does hold the Doula role — but only because a `PATCH` was made from a terminal against a staff id no screen prints; nothing in the product does it. She does **not** appear as the assigned Doula on an Engagement (RA-G4); the nearest thing the product has is a dateless Visit reassigned by pasting a UUID (RA-G10). And no screen shows every Engagement with its Contract and Invoice state (RA-G6, RA-G7). What does pass is 6.1-a: the Clients list really is Practice-wide, so a Client created by anyone shows up for everyone.

Her moment of truth landed where the map put it, and the walk sharpened *why*. Stage 8 does not fail on the phone — the screen renders cleanly at 390px with no overflow. It fails because there is nothing on it: at 2 a.m. the product can tell her the names of three Clients and that each is `intake`. The failure is absence, not layout, which is what makes RA-G5 a feature and not a stylesheet.

**For the walks behind this one**: Priya's plan step 3.3 and her journey map's stage 3.3 both carry the claim this walk falsified — that Payments is owner-gated — and both are corrected. A non-owner will see **Clients, Billing and Payments**, not two tiles.
