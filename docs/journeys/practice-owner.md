# Renata Alvarez — grow and run the roster

- **Persona**: [practice-owner.md](../personas/practice-owner.md)
- **Goal**: see the whole Practice at once — who is on which Engagement, which Contracts are unsigned, which Invoices are unpaid — without asking four people
- **Entry point**: cold signup at `/signup` (Stage 0); from Stage 1 on she already has a Practice and signs in at `/login`
- **Done looks like**: a new Doula has accepted an invitation, holds the Doula role, and is on a live Engagement. Renata can see, in one place, every Engagement in the Practice and its Contract and Invoice state.

> **This persona is contradicted by the schema.** Her file once stated that she puts a Doula on an Engagement and that the new Doula is then on a live Engagement. The `engagements` table has no staff column (`00005_client_engagement.sql`); only `visits.staff_id` exists. The need is real and stays in this map as Stage 4. The persona sentence that asserted the capability exists has since been revised. See RA-G4.

## Moment of truth

**Stage 8 — two Clients go into labor the same night, and she needs to know within a minute who is free.** Coverage is her stated anxiety, and it is the one thing that cannot wait until morning. Today no screen in the product answers it, and the underlying data to answer it does not exist either.

## Words

Renata is a domain expert. Her language and `GLOSSARY.md` mostly agree, which is itself worth recording: the divergences below are the exceptions, not the rule.

| Domain term | What Renata says | Note |
| --- | --- | --- |
| Staff | "my doulas", "my team" | She says Staff only about Dee |
| Engagement | "a client", "a birth" | She counts her year in births, not Engagements |
| Admin | "the office" | |
| Visit | "a prenatal", "the birth" | She distinguishes types the model does not |
| Attachment | "the assignment", "who is assigned to whom" | `GLOSSARY.md` avoids "Assignment" for an Attachment, the record that a Doula is on an Engagement, and keeps the word for a Visit, whose `staff_id` is assigned |

## Stages

### Stage 0 — Sign up the agency and start work with the first Client

The route that `docs/onboarding-route.md` specifies: three acts, four presses, five screens, from `/signup` to the Engagement's page, which is First Value ([ADR-0048](../adr/0048-nothing-holds-a-practice-here-but-the-next-family.md)). That document counts the signup journey as its first screen; the Budget counts each of its four pages, so here the route is eight screens. It is the same route that Maya Okonkwo walks in her stages 1 and 3, with one answer that differs. The stage is numbered 0 so that stages 1 to 9, and each step id that another document cites, keep their numbers.

**Thinking**: "I have a team and a waiting list. Is this for an agency, or for one doula on her own?" **Pain points**: nothing on the route says that the Practice is for a team, by decision (#1515): signup and the empty Practice are one set of words for a solo Owner and for her. The one place where her Practice differs is the Start work form, where "Who is the Doula?" waits for her answer because her Practice holds more than one employee Doula. Her Doulas are on the list only if they accepted an Invitation before she starts work; the route itself sends no Invitation, and nothing on it says that a Doula can be put on the Engagement later. Signup is four short pages and not one form, by decision ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)), so the route crosses eight screens and no signup page asks more than six decisions of her at once ([TB-G8](evaluator-doula.md#gaps-found) holds the reason).

**Budget**:

* **Screens**: 8 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

The stage has no Time line, because signup and the first Client are not acts that she repeats.

- **0.1** — Open `/signup` and answer three question pages ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)), each with **Continue**: "About you" (First name, Last name, Email address, Password); "What is your Practice called? (optional)"; "Where do you work?" (the state she works from, Timezone). Then "Check your answers": her answers, each with a **Change** link, and directly above **Create your Practice** the sentence "By creating your Practice, you agree to the Terms of Service and the Privacy Policy.", whose two links open in a new tab. There is no checkbox, no second password field, and no question that asks if she is a solo Owner or an agency.
- **0.2** — Answer "Rooted Birth Collective" as the Practice name because the agency does not work under her own name, and press **Create your Practice** (`POST /api/staff/signup`). Act 1, press 1. Her membership holds Owner, Admin and Doula, with the Employment type `employee`, and the press writes the Practice's `terms_accepted` Activity row (#1547, ADR-0053).
- **0.3** — Land on the empty Practice, `/practices/[practiceId]`, with no screen between and no second factor asked for (#1531). The heading is "Overview", and the top bar names Rooted Birth Collective (#2141). The words, the Credits sentence ("Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits.") and one link, **Add your first Client**. No Getting paid card, no second-factor notice and no link to the Invitation form are on it.
- **0.4** — Press **Add your first Client**. Act 2, press 2. The name question opens, "What is the Client's name?", with no search in front of it (#1609).
- **0.5** — Answer the name and press **Save and continue**. Press 3. The Client is saved, and nothing is spent. The Start work form opens and says "{given name} saved as a Client." (#1710), then "Starting work with {name} uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2." (#1612).
- **0.6** — Read "Who is the Doula?". Her two Doulas, Priya Raman and Jo Mercer, accepted Invitations as employee Doulas before she added her first Client, so the Practice holds three employee Doulas: "Renata Alvarez (you)" first, then each employee Doula, then "No Doula yet". **No option is selected**, so this is the one answer that she gives and a solo Owner does not (#1515, #1596). A press of **Start work with {name}** with no answer is refused: "Select who the Doula is, or select No Doula yet".
- **0.7** — Answer **No Doula yet**, with the kind of work and the due date, and press **Start work with {name}**. Act 3, press 4. She is her own approver, so the Engagement exists, one Credit is locked, and nobody is attached. She lands on the Engagement's page, which says "Work with {name} started.": First Value. The Doula is "No Doula yet", and the On call section says "Nobody is on this birth yet." A Doula is put on the Engagement after First Value, by an Offer or from the Engagement's page (#1598), and that is not on this route.

### Stage 1 — Sign in and choose the Practice

**Thinking**: routine. **Pain points**: the landing screen offers her whole Practice's Stripe state to anyone who is a member at all (RA-G9), which she will not notice until she invites someone. The Overview she lands on holds four decisions on a day like the second walk's, each headed group one under the hub rule: the Quick links ([#2006](https://github.com/markgoho/doula-cloud/issues/2006)), Recent activity ([#486](https://github.com/markgoho/doula-cloud/issues/486), bounded by #2006), Clients waiting on a reply ([#455](https://github.com/markgoho/doula-cloud/issues/455)) and the second-factor notice ([#1533](https://github.com/markgoho/doula-cloud/issues/1533)) until she sets one up or closes it. Each was chosen by its own record, so the budget is 4 ([#2228](https://github.com/markgoho/doula-cloud/issues/2228)). The rest of the page is drawn only where it asks something of her: an Offer, a waiting Request, Stripe wanting more, an expired invitation.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **1.1** — `/login`, sign in (`POST /api/session`).
- **1.2** — Choose Rooted Birth Collective from her memberships. **There is nothing to choose**: `decideLanding` redirects a person with one membership straight to their Practice (`app/src/lib/landing.ts:24-26`), and her own journey gives her only the one — a second membership is Lena Vasquez's journey alone.
- **1.3** — Land on `/practices/[practiceId]`, the Overview: the Quick links her roles open, then what needs her (the second-factor notice, her Offers, the Clients waiting on a reply, and any waiting Request, Stripe step or expired invitation), then Recent activity.

### Stage 2 — Invite a new Doula

**Thinking**: "She starts in two weeks. Get her set up." **Pain points**: no email is sent. The screen says so and prints a link that Renata must deliver herself, by text or by her own email client. Her new hire's first impression of the Practice's software is a pasted URL.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **2.1** — Open `/practices/[practiceId]/invite`.
- **2.2** — Enter the new Doula's name and email; press **Send invite** (`POST /api/practices/{id}/invitations`, owner-gated).
- **2.3** — Copy the printed link and send it out of band.
- **2.4** — The invitee accepts at `/accept-invite`. The membership is created holding **zero** roles (`invite.go` inserts `'{}'`).

### Stage 3 — Set the new Doula's roles

**Thinking**: "Now make her a doula, not an owner." **Pain points**: changing her roles takes three screens: the roster, her page, and "Which roles does {name} have?", which saves back to her page. That is [#2041](https://github.com/markgoho/doula-cloud/issues/2041)'s shape. A roster row holds facts and one link, her name. Her page shows each fact with a **Change**, GOV.UK's Summary list ([ADR-0021](../adr/0021-govuk-is-the-reference-for-service-patterns.md)). Each **Change** is its own question page. To show the roles on one screen, the build would have to put a form back in a roster row, and #2041 took that out. So the budget is 3 ([#2229](https://github.com/markgoho/doula-cloud/issues/2229)). Changing her employment type as well is one more question page, reached from the same member page. Stage 2's invitation already asks her roles and employment type, so this stage is for changing them afterward.

**Budget**:

* **Screens**: 3 (Hick's Law, Flow)
* **Decisions**: 5 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **3.1** — Open **Staff**, `/practices/[practiceId]/staff`.
- **3.2** — Read the Roles column, which renders the team's words via `rolesLabel`: Dee appears as `Admin`, not a raw enum ([#262](https://github.com/markgoho/doula-cloud/issues/262), closed).
- **3.3** — Open the Doula's page from her name, and press **Change** on the Roles row.
- **3.4** — Answer "Which roles does {name} have?" and press **Save** (`PATCH /api/practices/{id}/staff/{staffId}/roles`, Owner only). Her page says "{name}'s roles are now {roles}.", and her Membership history records the change and who made it.

### Stage 4 — Put the Doula on Engagements

**Thinking**: "Priya takes the two October clients." **Pain points**: an Engagement has no Doula. There is no field, no endpoint, and no screen. What she has in mind, Priya on those two Engagements, does not exist in the model.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **4.1** — Open an Engagement.
- **4.2** — Look for a control that puts a Doula on the Engagement. There is none.
- **4.3** — The nearest available act is to add a Visit — but **it cannot name anyone**. `POST .../visits` takes no body and assigns the caller (`api/internal/visit/create.go:32,47`); handing the Visit to a colleague is a second act through the **Reassign to Staff id** free-text box, which wants a staff UUID no screen prints (RA-G10). It also requires the **Doula** role (`api/internal/visit/roles.go:40`), which an Owner holds only because signup grants all three. Assignment exists at Visit level only, and a Visit has no date, so this cannot express "Priya covers this birth".

### Stage 5 — Reassign when someone is sick

**Thinking**: "Priya is ill. Move her Thursday to Jo." **Pain points**: reassignment works, but only over the Visit-level assignment from Stage 4, so it moves a dateless record rather than a booking.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **5.1** — `PATCH /api/practices/{id}/engagements/{id}/visits/{visitId}` with a new `staffId`.

### Stage 6 — See the whole Practice

**Thinking**: "Show me everyone." **Pain points**: the list is Practice-wide, which is what she needs — but it has two columns, Name and Status, and Status is `intake` for every row forever (MO-G4). Nothing about Contracts, Invoices, or who is covering whom.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **6.1** — Open `/practices/[practiceId]/clients` (`GET /api/practices/{id}/clients`). The handler is explicitly Practice-scoped: "every Client with an Engagement at the current Practice, regardless of which Staff member created it". **This half of her requirement passes.**
- **6.2** — Open each Engagement one at a time to learn its Contract and Invoice state. There is no roll-up.

### Stage 7 — See the money across all Staff

**Thinking**: "Which invoices are unpaid?" **Pain points**: Invoices exist only inside one Engagement's Contract. There is no Practice-wide invoice list and no unpaid view. The screen named **Billing** is about credits she buys from DoulaCloud, not money her Clients owe her.

**Budget**:

* **Screens**: 1 (Hick's Law, Flow)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **320px**: yes (ADR-0024)

- **7.1** — Open `/practices/[practiceId]/billing` and find a credit balance and ledger.
- **7.2** — Look for unpaid Client invoices. Find none, at any level above a single Engagement.

### Stage 8 — Coverage, at 2 a.m. — moment of truth

**Thinking**: "Who is already at a birth?" **Pain points**: no screen shows availability, on-call state, or who is where. Because Visits carry no dates (MO-G1), the data a coverage view would read does not exist. This is the highest-value gap in the whole practice-side set, and it came from the experience layer — a click-by-click map would never have surfaced it, because there is no click to record.

**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 2 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)

- **8.1** — Sign in on a phone.
- **8.2** — Look for who is free. There is nowhere to look.

### Stage 9 — Edit Plan Templates for the whole Practice

**Thinking**: "Add the hospital-transfer question to every birth plan." **Pain points**: adding a question takes three screens, not one: the Plan Templates list, the kind of answer, and the new question's own page, whose save returns her to the list. That is the founder's decision on [#2193](https://github.com/markgoho/doula-cloud/issues/2193) (2026-10-29): GOV.UK's "Add another" shape, one question per page, so that no screen shows every question's controls at once. The list counts 4 decisions however many questions the Practice writes, because a list of the Practice's own records is one decision (the founder's decision on #2193, 2026-10-30, in the [README](README.md)'s count rules).

**Budget**:

* **Screens**: 3 (Hick's Law, Flow)
* **Decisions**: 7 (Hick's Law, Miller's Law)
* **Confirmation**: yes (Peak-End Rule)
* **320px**: yes (ADR-0024)

- **9.1** — `/practices/[practiceId]/settings/plan-templates`, **Add a question**, its kind of answer, then the question's own page, whose save is `PUT /api/practices/{id}/plan-templates/{planType}` (owner-gated) with the one question added.
- **9.2** — Confirm an already-filled Plan Instance is unchanged. It snapshots the field definitions at creation (`00012_plan_instances.sql`), so this **passes**.

## Gaps found

| ID | Stage | Layer | Gap | Issue |
| --- | --- | --- | --- | --- |
| RA-G1 | 2 | Both | Invitations send no email. The Owner must copy a link and deliver it out of band. | [#260](https://github.com/markgoho/doula-cloud/issues/260) |
| RA-G2 | 3 | Interaction | **Closed.** No role-assignment UI. The `PATCH .../roles` endpoint existed but nothing called it, so the roster could not be built in the product. Roles are now changed from the member's page ([#2041](https://github.com/markgoho/doula-cloud/issues/2041)). | [#261](https://github.com/markgoho/doula-cloud/issues/261) |
| RA-G3 | 3 | Both | **Closed.** The Staff list rendered raw enum values, so an Admin showed on screen as `office_manager` (later `admin`) rather than the word `GLOSSARY.md` prescribes; `rolesLabel` now renders `Admin`. | [#262](https://github.com/markgoho/doula-cloud/issues/262) |
| RA-G4 | 4 | Interaction | An Engagement has no Doula on it. No column, no endpoint, no screen. Assignment exists only per Visit. | [#225](https://github.com/markgoho/doula-cloud/issues/225) |
| RA-G5 | 8 | Experience | No coverage or availability view, and no dated Visits to build one from. Her stated anxiety has no surface at all. | [#263](https://github.com/markgoho/doula-cloud/issues/263) |
| RA-G6 | 6 | Both | The Clients list shows Name and Status only. No Contract state, Invoice state, or Doula — so "see the whole Practice" needs one Engagement page per Client. | [#264](https://github.com/markgoho/doula-cloud/issues/264) |
| RA-G7 | 7 | Interaction | No Practice-wide Invoice or unpaid list. Invoices are reachable only inside a single Engagement's Contract. | [#265](https://github.com/markgoho/doula-cloud/issues/265) |
| RA-G8 | 2 | Interaction | An invitation carries no roles. `InviteHandler` takes a name and an email and inserts a membership with `roles = '{}'` (`api/internal/staffauth/invite.go:67`), so a zero-role membership is the only possible outcome of inviting anyone. Distinct root from RA-G2: a role-assignment UI alone would not fix the endpoint's contract. [ADR-0006](../adr/0006-read-follows-the-role.md) abolishes the zero-role state by putting roles on the invitation. | [#266](https://github.com/markgoho/doula-cloud/issues/266) |
| RA-G9 | 1 | Interaction | The **Payments** tile is not owner-gated, and `GET .../payments/connect` carries no role gate either (`api/internal/payments/connect.go:163-165`, unlike its `POST` sibling at line 96). So every Staff member — including the zero-role member RA-G8 says an invitation must produce — is *offered* the Practice's Stripe account state and reads it: a Doula's `GET` answered `200 {"status":"not_connected",…}`. [ADR-0006](../adr/0006-read-follows-the-role.md)'s table has no row for Stripe Connect state, so this is a read rule the ADR cannot yet be implemented against, not only a mis-placed link. | [#267](https://github.com/markgoho/doula-cloud/issues/267) |
| RA-G10 | 4 | Both | A Visit cannot be created for a colleague, only reassigned to one — and reassignment asks for a staff UUID by hand. `POST .../visits` takes no body and assigns the caller; the only way to put Jo on a Visit is to paste her id into a free-text box, and no screen in the product prints a staff id. Distinct root from RA-G4: even the Visit-level assignment the model *does* have is unreachable for anyone but yourself. | [#268](https://github.com/markgoho/doula-cloud/issues/268) |
| RA-G11 | 1 | Experience | **Closed by a budget change, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** `**Decisions**: 3 (Hick's Law, Miller's Law)` — measured 13. The Overview an Owner lands on holds seven Quick links, the second-factor notice's two controls, a Client waiting on a reply, **See all activity**, **Manage staff** and **Buy credits** in its body at once, several of them repeating the top bar. Second walk ([#2174](https://github.com/markgoho/doula-cloud/issues/2174)). Under the hub rule that was 6. The Overview now draws Requests, Getting paid and Your people only where they ask something of her, puts them in the column rather than a rail, drops the Credits block (the Credits card holds the balance), and offers **Your offers** only to a Doula. That leaves 4: the Quick links ([#2006](https://github.com/markgoho/doula-cloud/issues/2006)), Recent activity ([#486](https://github.com/markgoho/doula-cloud/issues/486)), Clients waiting on a reply ([#455](https://github.com/markgoho/doula-cloud/issues/455)) and the second-factor notice ([#1533](https://github.com/markgoho/doula-cloud/issues/1533)), each chosen by its record, so stage 1's Decisions budget is now **4**, with the reason in its Pain points. Lena Vasquez's stage 3 meets her Offer on the same page, so its budget is now 3. Measured again 2026-10-31 at 320px: 4. | [#2228](https://github.com/markgoho/doula-cloud/issues/2228) |
| RA-G12 | 3 | Experience | **Closed by a budget change, waiting on a walk ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)).** `**Screens**: 1 (Hick's Law, Flow)` — measured 3. Roles are changed from the member's page, one question page per row (roster, her page, "Which roles does {name} have?"), not one **Edit membership** form; employment type is a fourth screen. Second walk. The budget was set for the **Edit membership** form that #2041 replaced. #2041 chose the roster row with one link, the member page with a **Change** per fact, and a question page for each **Change**, so stage 3's Screens budget is now **3**, with the reason in its Pain points. Measured again 2026-10-31: 3. | [#2229](https://github.com/markgoho/doula-cloud/issues/2229) |
| RA-G13 | 4 | Both | `**Screens**: 1 (Hick's Law, Flow)` — measured 3; `**Decisions**: 2 (Hick's Law, Miller's Law). Which Doula, and the save` — measured 8. On an existing Engagement the Doula row's **Change** offers only **Put me on this Engagement**; another employee Doula goes on only by **Offer this work**, a form shaped for a stranger (first initial, general area, the due date typed again), and only once she accepts it. The Clients row opens the Client's record, not the Engagement. Second walk. | [#2230](https://github.com/markgoho/doula-cloud/issues/2230) |
| RA-G14 | 5 | Experience | `**Decisions**: 2 (Hick's Law, Miller's Law). Who takes it instead, and the save` — measured 5. Reassigning is the Visit row's one **Change** form, which also asks the time and the notes. Second walk. | [#2231](https://github.com/markgoho/doula-cloud/issues/2231) |
| RA-G15 | 6 | Both | `**Screens**: 1 (Hick's Law, Flow). One list answers each Engagement's Contract, Invoice and covering Doula, with no Engagement opened` — measured 2. The Clients list rolls up Contract and Invoice state (RA-G6) but names no Doula; that needs **On call**, which lists only births in their window. Second walk. | [#2232](https://github.com/markgoho/doula-cloud/issues/2232) |
| RA-G16 | 7, 8 | Experience | `**Time**: 400 ms, 100 ms (Doherty Threshold)` — **Invoices** settles in 108 to 125 ms with nothing on screen before it; **Sign in** on a phone takes 516 ms. Second walk, on the local dev stack. | [#2233](https://github.com/markgoho/doula-cloud/issues/2233) |
| RA-G17 | 8 | Experience | `**Screens**: 2 (Hick's Law, Flow)` — measured 3; `**Decisions**: 2 (Hick's Law, Miller's Law)` — measured 4. On a phone, **On call** is behind the landing and the **Menu**, and opens on the next 30 days, so "tonight" is From, To and **Apply**. Second walk. | [#2234](https://github.com/markgoho/doula-cloud/issues/2234) |
| RA-G18 | 8 | Experience | `**Confirmation**: yes (Peak-End Rule). The screen names who is free and who is already at a birth, as of now` — measured no. **On call** names who is on call for each birth tonight and who carries two, but nothing records who is at a birth now, so every Doula reads Available, and two births on one Doula are summed up as "all covered". Second walk; recorded on the older open issue that owns the root. | [#428](https://github.com/markgoho/doula-cloud/issues/428) |

Also hit here, filed on their owning maps: **MO-G4** (an Engagement's status never changes, so her one status column is dead), **MO-G1** (dateless Visits, which is why RA-G5 has no data to read).
