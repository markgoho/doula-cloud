# Maya Okonkwo — from empty account to one live Engagement

- **Persona**: [solo-birth-doula.md](../personas/solo-birth-doula.md)
- **Goal**: replace the paper folder — one place that holds the Client record, the Contract, the Birth Plan, and the messages
- **Entry point**: cold signup at `/signup`
- **Done looks like**: a Client with a signed Contract, a completed Birth Plan the Client can read in the portal, at least one Visit, an open message thread, and an Invoice she can point at — done without asking anyone for help.

## Moment of truth

**Stage 3 — she finds out the folder does not fit.** Her stated why is "one place that holds the Client record, the Contract, the Birth Plan, and the messages". The Client record is the first of those four, and when this map was drawn it held two fields, name and email — no due date, no phone number, no address, no intake notes. **That is closed** ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252), ADR-0017): the record now carries a real column set — name, date of birth, email, phone and address — asked one question at a time, and a Practice adds the facts we did not think of on its own Client Field Template.

Maya is the most motivated Persona in the cast and will push through friction, so the question is what makes her stop. At minute five she learns she must keep the paper folder anyway — and the premise of the product dies there, before a Contract exists. The Contract signature round trip (Stage 5) is the visible payoff, and the 3 a.m. Birth Plan retrieval is the test of whether she *stays*, but neither is the break point.

## Words

Maya is close to a domain expert but not inside the model.

| Domain term | What Maya says | Note |
| --- | --- | --- |
| Engagement | "her file", "my client" | The word Engagement is invisible in her workflow and should stay that way |
| Care Plan | "my notes" | She would not call her own notes a plan |
| Plan Instance | — | She has no word for it, and needs none. It is a modeling term |
| Plan Template | "the form I use every time" | |
| Visit | "a visit", "the birth" | Matches |
| Client | "my client", "the mom" | `GLOSSARY.md` avoids "mom" deliberately |

## Stages

### Stage 1 — Sign up and create the Practice

**Thinking**: "I lost an intake form last spring. I am not doing that again." **Pain points**: none — she has the strongest motivation of any Persona. Signup is four short pages and not one form, by decision ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)), so the stage crosses five screens and no page asks more than six decisions of her at once ([TB-G8](evaluator-doula.md#gaps-found) holds the reason).

**Budget**:

* **Screens**: 5 (Hick's Law, Flow)
* **Decisions**: 6 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **1.1** — Open `/signup`.
- **1.2** — Answer three question pages ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)), each with **Continue**: "About you" (First name, Last name, Email address, Password); "What is your Practice called? (optional)"; "Where do you work?" (the state she works from, Timezone). On "Check your answers", read her answers, each with a **Change** link, and the sentence above the button, "By creating your Practice, you agree to the Terms of Service and the Privacy Policy.", whose two links open in a new tab; press **Create your Practice** (`POST /api/staff/signup`). She works under her own name, so she can leave the Practice name empty: the BFF gives the Practice her own name, first name then last name, and she types it only once (#1536). Pressing the button is her Practice's agreement to the two documents, recorded as a `terms_accepted` Activity row with the version of each (#1547, ADR-0053). There is no checkbox.
- **1.3** — Land on `/practices/[practiceId]`, whose first line now carries the role notice. She now holds Owner, Admin, and Doula on one membership, granted in a single statement (`signup.go:152`).

She is **not** a test of role separation. Every permission boundary rides on Renata's invite route instead.

### Stage 2 — Judge the seeded Plan Templates

**Thinking**: "Is this form going to fit how I actually work?" **Pain points**: she is asked to evaluate an abstract form before she has a single Client to picture. This is the least motivating work in the journey, placed at the point where she is least invested. Changing a question takes its own page: the Plan Templates list, the question's page (its **Change**), whose save lands back on the list, then the Contract Template, three screens. That is the founder's decision on [#2193](https://github.com/markgoho/doula-cloud/issues/2193): GOV.UK's "Add another" shape (2026-10-29), and step 2.3, "Edit and save", on the counted path (2026-10-30). The list counts 4 decisions, because a list of the Practice's own questions is one decision however long it grows (the [README](README.md)'s count rules).

**Budget**:

* **Screens**: 3 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **2.1** — Open `/practices/[practiceId]/settings/plan-templates`.
- **2.2** — Review the seeded default Care Plan and Birth Plan fields.
- **2.3** — Edit and save (`PUT /api/practices/{id}/plan-templates/{planType}`), owner-gated at `plans/template.go:220`. She is Owner, so this passes.
- **2.4** — Review the seeded Contract Template at `/practices/[practiceId]/settings/contract-template`. Signup seeds all three templates in one transaction — Care Plan, Birth Plan, and Contract (`signup.go:160`, `signup.go:168`) — so no Practice ever has to create one before sending its first Contract.

### Stage 3 — Add the first Client — moment of truth

**Thinking**: "Everything in the folder goes in here." **Pain points**: the folder holds a due date, a phone number, an address, a hospital, a partner's name, and a page of intake notes. The form took two fields when this map was drawn; intake now asks for all of it, the fixed facts as its own questions and the rest through the Practice's Client Field Template ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed). The Start work form asks the kind of work, the due date and who the Doula is in one act, by decision ([#1515](https://github.com/markgoho/doula-cloud/issues/1515)), so its six decisions are the stage's largest screen ([TB-G11](evaluator-doula.md#gaps-found) holds the reason).

**Budget**:

* **Screens**: 5 (Hick's Law, Flow)
* **Decisions**: 6 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **3.1** — On her empty Practice, press **Add your first Client**: while the Practice holds no Client, there is nobody to search for, so intake opens directly (ADR-0017). Intake is one question, the name.
- **3.2** — Answer the name question; press **Save and continue**. A Client is saved and **nothing else is** — no Engagement, no credit — and the Start work form for that Client opens.
- **3.2-b** — **Start work with {name}** on that form. As the Owner she is her own approver, so asking and approving collapse into one act, and **that** act creates the Engagement and consumes the credit (`billing.ConsumeCredit`), after naming its cost and the balance after it on the form. She lands on the Engagement's page, which says "Work with {name} started."
- **3.3** — Open the Engagement.
- **3.4** — Repeat for her second and third Clients. Signup granted three credits (`api/internal/staffauth/signup.go`), so her **fourth Engagement** — not her fourth Client, which costs nothing — returns `402` with "no credits remaining, ask a practice owner or admin to buy more". She *is* the Practice Owner, and the refusal offers **Buy credits** on the same screen.

### Stage 4 — Fill the Care Plan and the Birth Plan

**Thinking**: "This is the part that actually helps me at 3 a.m." **Pain points**: long-form typing, on a phone, by someone who is on call. Anything needing a laptop and twenty quiet minutes will not get done.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **4.1** — On the Engagement page, fill the Care Plan section (`PUT /api/practices/{id}/engagements/{id}/plans/care`).
- **4.2** — Fill the Birth Plan section (`.../plans/birth`). Each Plan Instance snapshots the template's field definitions at creation, so a later template edit cannot alter it.

### Stage 5 — Contract and signature

**Thinking**: "If she can sign this on her phone tonight, I am sold." **Pain points**: the ordering is not obvious. The Client cannot sign anything until a portal invite has been sent, and nothing on the Contract section says so.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **5.1** — Send the portal invite (`POST /api/practices/{id}/engagements/{id}/portal-invite`) so the Client has an account to sign in with.
- **5.2** — Build the Contract from the Practice's contract template (`POST /api/practices/{id}/engagements/{id}/contract`), status `draft`.
- **5.3** — Send it (`.../contract/send`), status `sent`. Nothing checks that the merge fields carry values, so a Contract can be sent blank (**MO-G10**).
- **5.4** — The Client signs in the portal (`POST /api/portal/engagements/{id}/contract/sign`), status `signed`.
- **5.5** — Maya sees `signed` on the Engagement page without leaving the app.

### Stage 6 — Schedule Visits

**Thinking**: "Prenatals at 32 and 36 weeks, then the birth." **Pain points**: this stage cannot be walked. A Visit is `(engagement_id, staff_id, created_at)` and nothing else — no date, no time, no type, no notes. `POST .../visits` records that a Visit *happened*, now, by somebody. It cannot schedule one, and the only edit is reassignment to another Staff member.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **6.1** — Open the Visits section on the Engagement page.
- **6.2** — Add a Visit (`POST /api/practices/{id}/engagements/{id}/visits`) and get a row with a creation timestamp.

This is the largest single gap on the practice side. It also removes any possibility of a calendar, which is what Renata's coverage view would need.

### Stage 7 — Get paid

**Thinking**: "I am tired of chasing money by text." **Pain points**: two money screens with names that invite the wrong click. **Billing** is where the Practice *buys credits from DoulaCloud*. **Settings → Payments** is where the Practice *gets paid by Clients*. Nothing on either screen explains the difference. The first time through, **Getting paid** sends her to answer the website question before it offers **Connect Stripe**, and she comes back to read what Stripe will ask for before she presses it. That detour is two screens more than the stage's own path, and each is a recorded choice: Stripe will not take Client payments without a website it can see, so the website is a gate before Connect ([#421](https://github.com/markgoho/doula-cloud/issues/421), enforced by [#442](https://github.com/markgoho/doula-cloud/issues/442)); it is asked on its own screen, **Your website** ([#440](https://github.com/markgoho/doula-cloud/issues/440), and ADR-0021's one thing per page); and what Stripe will ask for is said on **Getting paid**, before the button ([#442](https://github.com/markgoho/doula-cloud/issues/442)). So the Screens budget is 5 ([MO-G15](https://github.com/markgoho/doula-cloud/issues/2196)).

**Budget**:

* **Screens**: 5 (Hick's Law, Flow)
* **Decisions**: 3 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **7.1** — Open `/practices/[practiceId]/settings/payments`. The first time, it asks the website question first: **Answer the website question** opens **Your website**, "How will Clients and Stripe find you online?", where she gives her own website or social profile (or has a page published for her), and **Save** brings her back to **Getting paid** saying "Your website is saved. You can connect Stripe now." ([MO-G15](https://github.com/markgoho/doula-cloud/issues/2196)). Then start Stripe Connect onboarding (`POST /api/practices/{id}/payments/connect`, owner-gated). On return from Stripe the screen reads "Awaiting Stripe review" until Stripe verifies, then "Taking payments"; it re-reads status on a short poll and offers **Check status again** (**MO-G11**, closed).
- **7.2** — Raise an Invoice against the signed Contract (`POST /api/practices/{id}/engagements/{id}/contract/invoices`).

The leg is walked end to end (second walk, finished 2026-10-28, [#2171](https://github.com/markgoho/doula-cloud/issues/2171)): Stripe's hosted onboarding, finished in a person's own Chrome, leaves the account `active` by the `capability_status_updated` webhook; the first **Create Invoice** asks the billing mode once; by card, the Invoice is created on the connected account, the Client pays it on Stripe's hosted page, and `invoice.paid` marks it **Paid** for Maya and in the Client's portal. Before the account is active, the same press refuses with "Clients cannot pay this Practice yet." ([#2200](https://github.com/markgoho/doula-cloud/issues/2200)).

### Stage 8 — Message the Client

**Thinking**: "Now everything about her is in one thread." **Pain points**: the message form shows four decisions at once, Message, Attachment, **Send** and **Cancel**, and each is a recorded choice. The Engagement page shows a section's form only when asked, so **Write a message** opens it and **Cancel** closes it, handing focus back to **Write a message** ([#2004](https://github.com/markgoho/doula-cloud/issues/2004), [#2103](https://github.com/markgoho/doula-cloud/issues/2103)); and a Message may carry an image or PDF (GLOSSARY.md, Message), so Attachment sits under Message rather than one more press away. Messages are Engagement-scoped, immutable, and delivered by push-triggered fetch (ADR-0002), which she must not treat as a substitute for a phone call in labor.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **8.1** — Send a message from the Engagement page (`POST /api/practices/{id}/engagements/{id}/messages`).
- **8.2** — The Client replies from the portal; the thread is continuous.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| MO-G1 | 6 | Interaction | A Visit has no date or time. Visits can be recorded, never scheduled. No calendar is possible. | [#250](https://github.com/markgoho/doula-cloud/issues/250) |
| MO-G2 | 6 | Both | A Visit has no notes. "What was I told last time" has nowhere to live. | [#251](https://github.com/markgoho/doula-cloud/issues/251) |
| MO-G3 | 3 | Both | A Client record holds name and email only — no due date, phone, address, or intake notes. The paper folder cannot be replaced. | [#252](https://github.com/markgoho/doula-cloud/issues/252) |
| MO-G4 | 3 | Interaction | An Engagement's status never changes. There is no update path anywhere, so every Engagement stays `intake` for its whole life. | [#253](https://github.com/markgoho/doula-cloud/issues/253) |
| MO-G5 | 2 | Experience | She must judge a seeded Plan Template before she has any Client to picture, at her lowest motivation. | [#254](https://github.com/markgoho/doula-cloud/issues/254) |
| MO-G6 | 5 | Experience | The Client cannot sign until a portal invite is sent, and nothing in the Contract section says so. The ordering is implicit. | [#255](https://github.com/markgoho/doula-cloud/issues/255) |
| MO-G7 | 7 | Experience | "Billing" (credits she buys) and "Payments" (Stripe Connect, so she gets paid) are two unexplained money screens whose names invite the wrong click. | [#256](https://github.com/markgoho/doula-cloud/issues/256) |
| MO-G8 | 7 | Interaction | Stripe Connect is not configured, so getting paid is blocked. Expected — record it, do not chase it. **Closed by infrastructure, not by code**: a Stripe Sandbox account was opened at [#242](https://github.com/markgoho/doula-cloud/issues/242) and Renata's Practice completed hosted onboarding to `card_payments: active` on Dee's walk ([#236](https://github.com/markgoho/doula-cloud/issues/236)), so no `journey-gap` issue is filed for it. | not filed — [#242](https://github.com/markgoho/doula-cloud/issues/242) |
| MO-G9 | 3 | Both | **Narrowed 2026-09-20, [#1236](https://github.com/markgoho/doula-cloud/issues/1236); the credit-per-Client premise is closed** ([#257](https://github.com/markgoho/doula-cloud/issues/257)). A Credit no longer locks per Client, only when an Engagement Request is approved, gated to an Owner or Admin — and the wall names itself before she meets it: the Engagement Request form previews `Credit cost` and `Balance after`, and the `402` itself reads "no credits remaining, ask a practice owner or admin to buy more" rather than a bare code. Buying more, once a dead end, now works against the Stripe Sandbox ([#242](https://github.com/markgoho/doula-cloud/issues/242)). What remains is whether the founding grant — three Credits per Staff member — fits a doula who attends 20–30 births a year rather than the four-to-six-Client estimate this row was written against ([#439](https://github.com/markgoho/doula-cloud/issues/439) corrected the estimate); that question moved to [#243](https://github.com/markgoho/doula-cloud/issues/243)'s pilot questions. | [#257](https://github.com/markgoho/doula-cloud/issues/257) closed, [#243](https://github.com/markgoho/doula-cloud/issues/243) |
| MO-G10 | 5 | Both | A Contract resolves no merge field. `POST .../contract` snapshots the template prose and sets every value empty (`api/internal/contracts/contract.go:124`), so Maya retypes the Practice name and the Client name the product already holds — and **Send** accepts a Contract with all six values blank, which the Client then reads as "This agreement is between  and  for doula services." Found by the [#234](https://github.com/markgoho/doula-cloud/issues/234) walk. | [#258](https://github.com/markgoho/doula-cloud/issues/258) |
| MO-G11 | 7 | Both | **The Payments screen never refreshes itself.** It fetches Connect status once in `onMount` and never again (`settings/payments/+page.svelte:21`), so the one moment an Owner is guaranteed to be looking at it — Stripe's redirect back to `?connect=return` — is the moment it is most likely to be wrong. Its own banner says "Status updates once Stripe confirms your account is active", and nothing ever updates it. Observed on Dee's walk: seconds after a successful submission the screen still read `Onboarding incomplete` / "Stripe still needs some details before Clients can pay you" and offered **Continue Stripe onboarding**, a dead end, while the API already said `pending` with zero requirements. A manual reload showed `Awaiting Stripe review` / "Nothing is needed from you." | [#259](https://github.com/markgoho/doula-cloud/issues/259) |
| MO-G12 | 2 | Interaction | The Plan Templates editor shows every question's controls at once, and has no way on to the Contract Template. `**Decisions**: 7 (Hick's Law, Miller's Law)` — measured 16 in view at once (35 on the screen); `**Screens**: 2 (Hick's Law, Flow)` — measured 3, through the Settings hub. Second walk, 2026-10-27. **Screens: built and settled, waiting on the walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** Plan Templates links an Owner on with **Next, check your Contract Template**, so the stage no longer passes through the Settings hub. A Doula, who may not change the Contract Template, gets no link. Since the "Add another" shape below, changing a question opens its own page, and by the founder's decision on #2193 (2026-10-30) step 2.3 is on the counted path, so the stage's **Screens** budget moved from 2 to 3: the list, the question's page, then the Contract Template. **Decisions: built and settled, waiting on the walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** By the founder's decision on #2193 (2026-10-29), the Plan Templates and Client Fields screens are GOV.UK's "Add another" list: a Summary list of the questions, each row its question and kind of answer with a **Change** to the question's own page (its words, its kind, its answers to choose from, and its place in the list), and one **Add a question**, which asks the kind of answer first as one radio question in #1252's names. Each question's page saves on its own, so the list has no **Save**. The builder's own count at 1280x900 on the seeded Rooted Birth Collective: the kind page 3 and a question's page 7 (Back included), and the list 8 on either tab if each of its five **Change** links counts. By the founder's decision on #2193 (2026-10-30), a list that grows with the Practice's own records is one decision, so the list counts **4** (the tabs, the list, **Add a question** and **Next, check your Contract Template**), and the stage's largest is a saved question's page, 7, within the budget of 7. | [#2193](https://github.com/markgoho/doula-cloud/issues/2193) |
| MO-G13 | 2 | Experience | Saving a template says "Saved." and nothing about what changes or what is next. `**Confirmation**: yes (Peak-End Rule)` — measured no. Second walk, 2026-10-27. **Built, waiting on a walk.** Each save now says what was saved and what it changes. On the Contract Template it is a status directly under **Save**, announced politely, that leaves focus on **Save** (at 320px, scrolled to **Save**, its first line shows without a scroll; at the top of the form it was a screen away). On Plan Templates and Client Fields, since #2193 moved each save to the question's own page, it is said on the list the save lands on, before its heading and focused on arrival (#1710's notification banner): "Contract Template saved. Every Contract written from now on uses these terms, and a Contract already written keeps the terms it was written from."; "(Care Plan or Birth Plan) questions saved. Every (plan) filled in from now on asks them, and a (plan) already filled in keeps the questions it had."; and "Client Fields saved. These questions are on every Client's record now, and an archived field keeps what was already recorded." What is next: Plan Templates has **Next, check your Contract Template** (MO-G12), and while the Practice holds no Client, the Contract Template and Client Fields have **Next, add your first Client**, under **Save** on the one and at the end of the list on the other, which opens the name question, as the empty overview's link does (ADR-0017's amendment for #1516). This also answers `practice-owner.md` stage 9's Confirmation line. | [#2194](https://github.com/markgoho/doula-cloud/issues/2194) |
| MO-G14 | 4 | Interaction | The seeded Birth Plan is one section. `**Decisions**: 7 (Hick's Law, Miller's Law)` — measured 10 in "Birth Setting", a choose-several question counted per option. Second walk, 2026-10-27. **Built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The product already let a Practice chunk its own plan, since a Section heading is one of the kinds of answer on Plan Templates and the filled plan draws each one as its own headed group; only the seeded template put every question under one heading. A new Practice's Birth Plan now opens in two sections, chunked by what the questions are about: "When Labor Starts", holding People to notify when labor starts, then "Birth Setting", holding Planned birth location, Preferences for atmosphere and OK to take photos/video during labor, with **Save Birth Plan** and **Cancel** under it. Counted by [#2300](https://github.com/markgoho/doula-cloud/issues/2300)'s rule, a choose-several question is one decision however many boxes it holds, so that is 1 and 5. Every question keeps its words and its id, so a Birth Plan already filled in keeps the questions it had (ADR-0001). `TestSignupHandler_SeededPlansShowNoGroupOverSeven` holds both seeded plans to at most 7 per group. | [#2195](https://github.com/markgoho/doula-cloud/issues/2195) |
| MO-G15 | 7 | Interaction | Two one-time questions the stage does not name, the website before **Connect Stripe** and the billing mode on the first Invoice, each on a screen of four. `**Decisions**: 3 (Hick's Law, Miller's Law)` — measured 4. Second walk, 2026-10-27. The website detour also fails `**Screens**: 3 (Hick's Law, Flow)` — measured 5 from a cold start, 2026-10-28. **Screens: closed by a budget change; built, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The budget is now 5, the website detour with the stage's own path, because each screen on it is a recorded choice: the website is a gate before Connect (#421, #442), asked on its own screen (#440), and **Getting paid** says what Stripe will ask for before its button (#442); the reason is in stage 7's Pain points. Stage 7.1 now names the question. The build: **Answer the website question** (and **Go to website settings**, when a published page fails) marks the visit as from **Getting paid**, and saving **Your website** then returns her there, saying "Your website is saved. You can connect Stripe now." (or "Your page is published." for a page published for her), where she found her own way back before. **Decisions: within the budget by the count rule, waiting on a walk.** The walk counted each radio option: under the README's count rules a radio group is one decision, so **Your website** is 3 (the question, the web address, **Save**), the first **Create an Invoice** is 3 (the billing-mode question, **Create Invoice**, **Cancel**), and **Getting paid** is 3. The billing mode stays on the first Invoice, as #271 and #1922 chose. The other website answer, **Publish a page for me**, shows 4 at once and makes the path 6 screens; that is [#2302](https://github.com/markgoho/doula-cloud/issues/2302). | [#2196](https://github.com/markgoho/doula-cloud/issues/2196) |
| MO-G16 | 8 | Interaction | `**Decisions**: 2 (Hick's Law, Miller's Law)` — measured 4: Message, Attachment, **Send** and **Cancel**, behind **Write a message**. Second walk, 2026-10-27. **Closed by a budget change, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** The budget is now 4, because recorded decisions chose each of the four. The Engagement page is a record page: no section shows a form until asked, and Messages has **Write a message** ([#2004](https://github.com/markgoho/doula-cloud/issues/2004)); its **Cancel** hands focus back to the opener ([#2103](https://github.com/markgoho/doula-cloud/issues/2103), the GOV.UK alignment table's "Sections of a record page" row). A Message may carry an image or PDF (GLOSSARY.md, Message), and an optional field is still a field to fill, so Attachment counts. No build reaches 2 without undoing #2004: with the attachment off the form, Message, **Send** and **Cancel** are still 3, and any control that brings it back is a fourth. The brief's tie-break keeps the attachment (Tesler wins on what exists), and a disclosure in front of it would move the decision, not remove it. Lena Vasquez's stage 6 already holds the same form to 4 and passes (`contractor-doula.md`). The reason is in stage 8's Pain points, and Priya Raman's stage 8 (`employed-doula.md`), the same form, takes the same budget. | [#2197](https://github.com/markgoho/doula-cloud/issues/2197) |
