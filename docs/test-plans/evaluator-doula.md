# Tasha Bell — test plan

- **Journey**: [evaluator-doula.md](../journeys/evaluator-doula.md)
- **Persona**: [evaluator-doula.md](../personas/evaluator-doula.md)
- **A pass means**: a Practice with one test Client in it and an intention to come back — **or** a clear, recorded reason she left. Both close the journey.

Tasha is the only Persona who may legitimately abandon, so every stage carries an **abandon check**: the tester records whether a real evaluator would close the tab there. That check is the point of the plan, and it is a judgment, not an assertion — record what was seen, in her words, not a pass or a fail.

## Preconditions

None, and that is the test. Tasha arrives from outside with no account and no fixture. Stages 1 and 2 land on a marketing site that has not been built, so this plan is the only practice-side one whose first steps have no product to run against — they are `missing-feature`, not work to be done here (building the marketing site is out of scope for this map).

## Steps

### Stage 1 — Find it and judge it in thirty seconds

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Follow a link from a search result or a Facebook group | There is nowhere to arrive. No marketing site exists | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| 1.2 | Read what the product is for, looking for "doula" and "birth plan" before giving anyone an email address | No such page exists in or out of the product | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). One page that says what the product is for, "doula" and "birth plan" in it, before any email address is asked for | `missing-feature (TB-G1)` |
| 1.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `missing-feature (TB-G1)` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `missing-feature (TB-G1)` |

**Abandon check**: she never arrives at all.

### Stage 2 — Find the price

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Look for a price, per month, for two doulas | No price is published on the marketing site. TB-G2's in-product half is closed ([#285](https://github.com/markgoho/doula-cloud/issues/285)); the marketing-site half moved to [#868](https://github.com/markgoho/doula-cloud/issues/868) (what the site says) and [#284](https://github.com/markgoho/doula-cloud/issues/284) (building it) | `missing-feature (TB-G2)` [#868](https://github.com/markgoho/doula-cloud/issues/868) [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| 2.1-a | Open the only money surface that does exist, `/practices/[practiceId]/billing`, once signed up | The top bar's **Credits** opens it, under the heading `Credits`. It states `One Credit covers one Engagement` and `One Credit costs <price>.`, where `<price>` is the price the Terms of Service in force state, and the ledger's Origin column runs every value through a label instead of printing the raw enum (TB-G2's in-product half, closed by [#285](https://github.com/markgoho/doula-cloud/issues/285), and TB-G3, closed by [#286](https://github.com/markgoho/doula-cloud/issues/286)) | `manual` |
| 2.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). The price for two doulas, one screen from where she judged the product | `manual` |
| 2.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 2.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Abandon check**: an unanswered price reads as expensive.

### Stage 3 — Sign up

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/signup` | Three question pages and a check page ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)): `About you`, the Practice name, `Where do you work?`, then `Check your answers`. No email confirmation step before she is in. This is the strongest leg of her journey | `automated (signup-form.e2e.ts)` |
| 3.2 | Answer the three question pages of the signup journey ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)): First name, Last name, Email, Password; then the Practice name (optional); then the state she works from and Timezone | "Practice name" asks her to name a business she may not think of as one; it is optional since [#1536](https://github.com/markgoho/doula-cloud/issues/1536), and an empty one gives the Practice her own name | `automated (signup-form.e2e.ts)` |
| 3.3 | Press **Create your Practice** on "Check your answers" | The sentence directly above the button says that creating the Practice agrees to the Terms of Service and the Privacy Policy, each a link that opens in a new tab, with no checkbox ([#1547](https://github.com/markgoho/doula-cloud/issues/1547)). `POST /api/staff/signup` creates the Practice, the Staff row, a membership holding Owner + Admin + Doula in one statement, and the Practice's `terms_accepted` Activity row | `automated (signup-form.e2e.ts)` |
| 3.3-a | Look for anything telling her roles exist, or that she now holds three | Nothing on the four signup pages names a role. The first screen after it does: Overview opens with "You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose." ([#290](https://github.com/markgoho/doula-cloud/issues/290), closed; moved there from signup by [#2049](https://github.com/markgoho/doula-cloud/issues/2049)). So she learns it just after she commits, not before. The role-scoped **Staff** nav item leads to the roster, which reads `Owner, Admin, Doula`, the team's words, not the schema's ([#262](https://github.com/markgoho/doula-cloud/issues/262), closed) | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 5 (Hick's Law, Flow). About you, the Practice name, Where do you work?, Check your answers, then the Practice landing ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)) | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). The largest is Check your answers: five **Change** links and **Create your Practice**. About you is 5 (four boxes and **Continue**), Where do you work? 3, the Practice name page 2. The Terms and Privacy links, **Back** and **Show password** are not counted | `manual` |
| 3.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 3.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Abandon check**: low risk. Time the whole stage — under a minute is the claim.

### Stage 4 — The first screen (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Land on `/practices/[practiceId]` | The heading `Overview`, with the Practice's name in the top bar (#2141). The old seven-link menu is gone ([#452](https://github.com/markgoho/doula-cloud/issues/452), closed). The shell's persistent, role-scoped top bar holds ten destinations for an Owner: Overview, Clients, Schedule, On call, Invoices, Contracts, Credits, Staff, Offers and Settings. Plan Templates, Contract Template and Getting paid sit under **Settings**, and the screen's body is `OverviewHub`'s own content | `manual` |
| 4.1-a | Search the screen for the words "birth plan" and "visit" | With no Client yet, `OverviewHub`'s body reads three sentences: "You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose."; "Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice."; and "Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits." ([#1612](https://github.com/markgoho/doula-cloud/issues/1612)). Both words appear, naming the work rather than administration | `manual` |
| 4.2 | Choose where to go first | The body offers one action, **Add your first Client**, not a menu. The top bar's ten destinations sit above it | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 5 (Hick's Law, Miller's Law). The top bar's destinations and the one way forward, **Add your first Client** | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The screen says her Practice exists and what to do next | `manual` |
| 4.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Abandon check**: none remaining. TB-G4 is closed by [#287](https://github.com/markgoho/doula-cloud/issues/287).

### Stage 5 — Kick the tires

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | On her empty Practice, press **Add your first Client** | It opens intake's one question, `What is the Client's name?`, with no search in front of it. Given name, Family name (optional) and Preferred name (optional) are on that page, and nothing follows it: date of birth, email, phone, address and the Practice's own Client Field Template sections are added later from the Client's record (ADR-0017, [#1610](https://github.com/markgoho/doula-cloud/issues/1610), [#1611](https://github.com/markgoho/doula-cloud/issues/1611)). She must still invent a Client to see any real screen, but she is no longer asked to invent her in two boxes | `automated (add-client-visits.e2e.ts)` |
| 5.2 | Answer the name question and press **Save and continue** | A Client is saved and **nothing else is**: no Engagement, no `credit_ledger` row, and her balance still reads 3. She lands on the Start work form for that Client, not on an Engagement, and the form says "{name} saved as a Client." with focus on that message ([#1710](https://github.com/markgoho/doula-cloud/issues/1710)) | `manual` |
| 5.2-a | Look for any warning that the trial has a size | Nothing on the name question mentions credits, and there is nothing to warn about — the save is free. The size of the trial is named on the next screen, at 5.2-b, before she spends anything. `manual`, because no spec opens the name question looking for what is *not* on it | `manual` |
| 5.2-b | Read the Start work form and press **Start work with {name}** | Before she commits, the form says what the start uses and what is left after it: `Starting work with {name} uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2.` ([#1612](https://github.com/markgoho/doula-cloud/issues/1612)). No price is on the form. She holds Owner, so ADR-0017's solo-Practice collapse fires and the request is created and approved in one act — this is where the Engagement is created and the Credit locks. She lands on the Engagement's page, which says "Work with {name} started." with focus on it. On an empty balance the form says so as it loads, before any press: "There are no credits left on this Practice's balance.", with **Buy credits** under it, and in place of the Credit sentence. **Start work with {name}** is still offered; pressing it answers `402` and nothing on screen changes, no focus move and no announcement ([#2186](https://github.com/markgoho/doula-cloud/issues/2186)). The second action, **Go to {name}'s record without starting work**, opens the Client's record with the Client saved and no Engagement | `manual` |
| 5.3 | Open the Engagement from the Clients list | The Engagement page renders | `automated (add-client-visits.e2e.ts)` |
| 5.4 | Read the page | An **On this page** list, then On call, Birth outcome, Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages and Activity, each with its empty state and one way to start it. **The first moment the product looks like doula work** — three presses past the first screen (**Add your first Client**, **Save and continue**, **Start work with {name}**) | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 5 (Hick's Law, Flow). The empty Practice, the name question, the Start work form, the Engagement: four ([#1611](https://github.com/markgoho/doula-cloud/issues/1611)) | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). The largest is the Start work form: the kind of work, the due date, "Who is the Doula?", the Note, **Start work with {name}** and **Go to {name}'s record without starting work**, with a radio group or a select as one decision. The name question is 4: the given name, the two optional names, and **Save and continue** | `manual` |
| 5.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 5.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 5.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — Judge the exit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Export her data — any format — or delete the account | **Settings** offers both, as two separate acts, at the bottom of the hub: a whole-Practice data export ([TB-G5](https://github.com/markgoho/doula-cloud/issues/288), closed), one ZIP of spreadsheet-ready files, and **Delete this Practice**, a 30-day countdown before erasure that any Owner can restore ([#871](https://github.com/markgoho/doula-cloud/issues/871), closed, split out of #288). Both need a second factor. A new Owner has none ([#1531](https://github.com/markgoho/doula-cloud/issues/1531)), so each row says "You need two-factor authentication before you can …" and offers **Set up two-factor authentication**, and the export answers `403 SECOND_FACTOR_REQUIRED` ([TB-G12](https://github.com/markgoho/doula-cloud/issues/2185)) | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 6 (Hick's Law, Flow). For an Owner with no second factor, deletion's path: **Settings**, **Delete this Practice**, enrollment's password, "Verify your email address first", the QR code, then **Delete this Practice** again ([#2185](https://github.com/markgoho/doula-cloud/issues/2185)). The export's is one fewer: enrolling returns her to **Settings** at **Security and data**. With a second factor, 1 for the export and 2 for deletion. An email outside the product is not counted | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 6.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Abandon check**: both halves of her stated question are now answered.

### Stage 7 — Judge the way in (migrating owner)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Import two years of Clients from a spreadsheet | No import exists. [#289](https://github.com/markgoho/doula-cloud/issues/289) closed without a build: import was ruled out of v1 on [#175](https://github.com/markgoho/doula-cloud/issues/175) and named the first priority after launch | `missing-feature (TB-G6)` [#289](https://github.com/markgoho/doula-cloud/issues/289) |
| 7.1-a | Reproduce one spreadsheet row by hand instead | Possible now, one row at a time: intake asks given, family and preferred name, the Client's details journey on her record asks date of birth, email, phone and address ([#1610](https://github.com/markgoho/doula-cloud/issues/1610)), and a Practice adds the columns we did not think of to its own Client Field Template ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed). What still cannot happen is two years of rows arriving together — that is [TB-G6](https://github.com/markgoho/doula-cloud/issues/289) at 7.1, and it is untouched. `manual`, because the claim is about the whole sequence and every Practice-defined field behind it, and no spec answers a question past the first one — `accessibility.e2e.ts` opens four of the routes to run axe over them, which tests the pages and not the transcription | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The import's own screen, then the Clients it made | `missing-feature (TB-G6)` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `missing-feature (TB-G6)` |
| 7.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `missing-feature (TB-G6)` |
| 7.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `missing-feature (TB-G6)` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Abandon check**: her existing data is the reason switching is expensive, and it cannot come with her.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 11 (6 of them [budget steps](README.md#budget-steps)) |
| `manual` | 34 (23 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 11 steps over 3 gaps ([TB-G1](https://github.com/markgoho/doula-cloud/issues/284), [TB-G2](https://github.com/markgoho/doula-cloud/issues/868), [TB-G6](https://github.com/markgoho/doula-cloud/issues/289); 7 of them [budget steps](README.md#budget-steps)) |

Five gaps are closed: TB-G2's in-product half and TB-G3 (unexplained credits) are both observed at 2.1-a; TB-G4 (the admin-menu first screen) at 4.1-a; TB-G7 (the unsignposted roster model) at 3.3-a; and TB-G5 (export and account deletion) at 6.1. All four steps now show the state that closed the gap rather than the gap itself. TB-G1 backs two steps.

3.3-a moved from `missing-feature (TB-G7)` to `manual` in the 2026-08-22 walk below: the step can be performed, and what the Staff screen hands back is the result.

5.1 moved from `manual` to `automated (add-client-visits.e2e.ts)` in the 2026-09-10 desk pass below, and 5.2-b was appended: the free save and the act that spends the Credit are two acts now, so they are two steps.

Her plan is the least automatable of the six, and for a reason worth keeping: two of her seven stages happen before the product exists, and a Playwright spec cannot run against a marketing site that has not been written.

## Run log

### 2026-10-27 — second manual walk ([#2170](https://github.com/markgoho/doula-cloud/issues/2170))

**Stack**: `bun run --cwd app dev:full` from a worktree at port offset 1, so the app at `http://localhost:5273` and the mail catcher at `http://127.0.0.1:19180`, on trunk at `e39ee559`, in WSL2 with rootless Podman. The seed brought up Rooted Birth Collective only, which this walk does not touch.

**Device**: headless Chrome inside WSL, driven by the chrome-devtools MCP, at 1280x900, as the first walk was. Every timing below is on this stack: an unbundled Vite dev server, so the first visit to a route compiles its modules on demand, which a production build does not do.

**Accounts and preconditions**: none from the seed, as the plan says. Tasha signed up at `/signup` in a fresh isolated browser context (the Cold steps) as `tasha.bell.walk2@sim.doula.cloud`, Practice `Bell Birth Support Walk Two`, Georgia, Eastern time. Her verification email was opened from the mail catcher. Her Clients are `Jordan Testcase` (a birth Engagement), `Riley` and `Sam` (postpartum Engagements, started only to spend the three Welcome credits for 5.2-b's empty balance) and `Avery` (saved, no Engagement). The second-factor acts at 6.1 were walked a second time under a separate Owner, `Riverside Doulas`, made by `SEED_SECOND_FACTOR=1 bun run --cwd app seed:staff-session`, in its own isolated context, for the reason given at 6.1.

The 11 `automated` steps were **not** re-run; the walk read only what each partly-driven spec leaves out (the run sheet on #2170).

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1, 1.2 | `missing-feature (TB-G1)` | as expected | Unwalkable. TB-G1 ([#284](https://github.com/markgoho/doula-cloud/issues/284)) is open and exempt from this run (#329, 2026-10-08) |
| 2.1 | `missing-feature (TB-G2)` | as expected | No marketing site, so no price outside the product ([#868](https://github.com/markgoho/doula-cloud/issues/868), [#284](https://github.com/markgoho/doula-cloud/issues/284)) |
| 2.1-a | `manual` | as expected, cell corrected | The top bar says **Credits**, and so does the heading. "One Credit covers one Engagement — a single Client relationship centered on one baby, from intake through the end of care." "One Credit costs $20.00." `Credit balance: 3`, one ledger row `Welcome credits +3`. A quantity field, a $20.00 price per Credit, a subtotal, and "New York sales tax is added at checkout where it applies." For a Georgia Practice that line reads oddly but is right: the taxable share is the Practice's New York-located Staff (`GLOSSARY.md`, **Work State**), so hers is zero. Not filed. **Buy credits** was not pressed |
| 3.1 | `automated (signup-form.e2e.ts)` | cell corrected | Four pages: `About you` (First name, Last name, Email address, Password, **Continue**), `What is your Practice called? (optional)`, `Where do you work?` (state and timezone), `Check your answers`. No email confirmation before she is in; a verification email arrives, and is needed only later, for a second factor |
| 3.3-a | `manual` | **falsified**, cell corrected | Signup names no role; `Check your answers` has no "Owner" anywhere in it. The first screen after it does: "You hold every role — Owner, Admin and Doula. Anyone you invite holds only the roles you choose." #2049 moved it there on purpose. The Staff roster reads `Owner, Admin, Doula`, `Employee`, `Georgia, self-reported Oct 27, 2026` |
| 4.1 | `manual` | as expected, cell corrected | `Overview`, `Bell Birth Support Walk Two` in the top bar, and ten top-bar destinations, not the four the cell named. A pilot banner sits under the bar: "DoulaCloud is new, and you are one of the first to use it. Tell us what is not working or what you need." |
| 4.1-a | `manual` | as expected, cell corrected | Both words appear. The body is three sentences, not one: the roles sentence, the "Nothing is here yet" sentence, and the Credit sentence |
| 4.2 | `manual` | as expected | One action in the body, **Add your first Client** |
| 5.2 | `manual` | as expected | "Jordan saved as a Client." with focus on it, on the Start work form. Read from the database at that moment: 1 `clients` row, 0 `engagements` rows, and one `credit_ledger` row, `signup_bonus +3`. Balance still 3 |
| 5.2-a | `manual` | as expected | The name question names no Credit. It reads "Only the given name is needed to save the record. The rest can be added at any time." |
| 5.2-b | `manual` | as expected, cell corrected | "Starting work with Jordan uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2." (it arrives a beat after the form, once the balance is read). No price. **Start work with Jordan** landed on the Engagement with "Work with Jordan started." focused, and wrote `consumption -1`. With the balance spent (Riley, Sam), Avery's form loaded with "There are no credits left on this Practice's balance." and **Buy credits** in place of the Credit sentence. Pressing **Start work with Avery** anyway sent the request, which answered `402`, and **nothing on screen changed**: same notice, focus unmoved, nothing announced. Filed as [#2186](https://github.com/markgoho/doula-cloud/issues/2186). **Go to Avery's record without starting work** opened her record, Engagements empty; the database held 4 Clients, 3 Engagements, balance 0 |
| 5.4 | `manual` | as expected, cell corrected | On call, Birth outcome, Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages and Activity, under an **On this page** list. Birth Plan reads "The Birth Plan is not started yet." with **Start the Birth Plan**. The Contract section says "Your Practice has no birth rate yet, so you cannot create a Contract." and links **Set the birth rate**. "This Client has no email address on file" appears three times on the page |
| 6.1 | `manual` | as expected, cell corrected | Both exit acts are at the bottom of **Settings**, and both refuse her: "You need two-factor authentication before you can export this Practice's data." with **Set up two-factor authentication**; `/settings/delete` says the same for deletion. `GET …/export` answers `403 SECOND_FACTOR_REQUIRED`. Following the link: password, then "Verify your email address first" (the link came from the mail catcher and verified), then **I have verified my email address**, which answered "There is a problem with the service. Try again in a few minutes." **That last refusal is the stack, not the product**: the local Firebase Auth emulator has no TOTP enrollment, and `mfaEnrollment:start` answers 400 ([`e2e.md`](../testing/e2e.md), firebase/firebase-tools#6224). So both acts were walked under the `SEED_SECOND_FACTOR=1` Owner: **Download** gave `Riverside Doulas export 2026-10-27.zip`, 23 files (a `README.txt` and 22 CSVs, from `practice.csv` to `activity.csv`); **Delete this Practice** opened a dialog naming what is erased and what is kept, and confirming it showed "Deletion pending. This Practice is scheduled to be deleted on Nov 26, 2026. Restore it before then to keep it." with **Restore this Practice**. Settings calls the same thing "Multi-factor authentication" and "two-factor authentication" three rows apart, which is [#1784](https://github.com/markgoho/doula-cloud/issues/1784), open |
| 7.1 | `missing-feature (TB-G6)` | as expected | No import on the Clients list, Settings or the route table. #289 is closed without a build, ruled out of v1 on #175. **7.1 keeps citing TB-G6 and #289**: the step still cannot be performed, and TB-G6 is the gap that says why; a closed issue that names its own reason is a better citation than none |
| 7.1-a | `manual` | as expected | One row by hand: the name question, then from Jordan's record, each detail its own question page that saves and returns to the record (date of birth saved as "Apr 9, 1993", "Jordan's details saved."). Email, phone and address have the same **Change** pages. "What else Bell Birth Support Walk Two asks" offers **Add a question**, to the Practice's Client Fields |

**Re-marked**: 7.B-screens, 7.B-decisions, 7.B-confirmation and 7.B-cold move from `manual` to `missing-feature (TB-G6)`. Each budget is set for the import's own screen, and there is no import screen to measure. The [README](README.md#budget-steps)'s rule for an unwalkable stage was written for an open gap. TB-G6 closed by ruling, not by build, so the screen it names still does not exist; the rule's reason (nothing to measure) applies all the same. 7.B-320px stays `automated`. No other step changed mark. The Marks summary is recounted to 11 / 34 / 0 / 11, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted on this walk**: fields, options, and links or buttons that move her somewhere or change what happens, visible at once in the page body. **Back**, **Show password**, the skip link and links that open a policy in a new tab are not counted. The Staff shell's top bar is counted only on stage 4, because 4.B-decisions' own note counts it; on stages 5 and 6 it is left out and given beside the number, so the body's own count can be read. The shell adds 12: ten destinations, the account menu and the pilot banner's feedback link. **Time** is measured in the page: a script presses the control and polls each animation frame until the result has settled on screen, and records when the control first showed it was busy (the acknowledgment).

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens, 1.B-cold, 1.B-320px | 1, yes, yes | not measurable (TB-G1) | — |
| 2.B-screens | 1 | 1. **Credits** is one top-bar press from her first screen | yes |
| 2.B-cold | yes | yes | yes |
| 3.B-screens | 2 | **5**: four signup pages, then Overview | **no**, [TB-G8](https://github.com/markgoho/doula-cloud/issues/2181) |
| 3.B-decisions | 5 | **6** on `Check your answers` (five **Change** links and **Create your Practice**). `About you` is 5, `Where do you work?` is 3, the Practice name page is 2 | **no**, [TB-G8](https://github.com/markgoho/doula-cloud/issues/2181) |
| 3.B-memory | yes | yes. `Check your answers` shows every answer with its own **Change** | yes |
| 3.B-confirmation | yes | **no**. Overview says what to do next but not that the Practice was created | **no**, [TB-G9](https://github.com/markgoho/doula-cloud/issues/2182) |
| 3.B-cold | yes | yes. Nothing needed a tour; each page's hint says why it asks (the state: "We need the state for sales tax on your practice's credits, not your address.") | yes |
| 4.B-screens | 1 | 1 | yes |
| 4.B-decisions | 5 | **13**: ten top-bar destinations, the account menu, the feedback link, and **Add your first Client** | **no**, [TB-G10](https://github.com/markgoho/doula-cloud/issues/2183) |
| 4.B-memory | yes | yes. Her Practice's name is in the top bar and her balance in the body | yes |
| 4.B-time | 400 ms, 100 ms | Top-bar presses settled in 59 ms (Settings) and 86 ms (Credits); **Add your first Client** in 71 ms. **Create your Practice** showed busy at once, and `POST /api/staff/signup` took 49 ms. A full reload of Overview painted its largest content at 449 ms, which is a page load on the dev server, not a routine act | yes |
| 4.B-confirmation | yes | **no**, for the same reason as 3.B-confirmation. What to do next: yes | **no**, [TB-G9](https://github.com/markgoho/doula-cloud/issues/2182) |
| 4.B-cold | yes | yes | yes |
| 5.B-screens | 5 | 4: Overview, the name question, the Start work form, the Engagement | yes |
| 5.B-decisions | 3 | **8** on the Start work form (Birth, Postpartum, Due date, two Doula options, Note, **Start work with {name}**, **Go to {name}'s record without starting work**); 4 on the name question. Plus 12 in the shell on each | **no**, [TB-G11](https://github.com/markgoho/doula-cloud/issues/2184) |
| 5.B-memory | yes | yes. Her Client's name carries onto the Start work form and the Engagement | yes |
| 5.B-time | 400 ms, 100 ms | **Save and continue**: 154 ms, then 92 ms. **Start work with {name}**: **499 ms** on the first press (Jordan, the first visit to the Engagement route), then 173 ms and 168 ms (Riley, Sam). Every press showed busy within 17 ms | **not settled on this stack**. Two of three starts met it. The one over-budget sample is the route's first load on a dev server that compiles on demand, so it is recorded and not filed; the next walk should time a production build |
| 5.B-confirmation | yes | yes. "Work with Jordan started.", focused | yes |
| 5.B-cold | yes | yes | yes |
| 6.B-screens | 2 | **at least 5** for an Owner with no second factor: Settings, enrollment's password, its verify-first step (and an email), its QR-code step, Settings again. Not finishable on this stack (6.1). With a second factor, 1 for the export and 2 for deletion | **no**, [TB-G12](https://github.com/markgoho/doula-cloud/issues/2185) |
| 6.B-decisions | 2 | **14** on Settings: twelve **Change** links, **Buy** credits, and the export's or deletion's own link. Plus 12 in the shell | **no**, [TB-G12](https://github.com/markgoho/doula-cloud/issues/2185) |
| 6.B-confirmation | yes | yes, for deletion: "This Practice is scheduled to be deleted on Nov 26, 2026. Restore it before then to keep it." The export ends in the downloaded file | yes |
| 6.B-cold | yes | yes. Each refusal names what it needs and links to it | yes |
| 7.B-screens, 7.B-decisions, 7.B-confirmation, 7.B-cold | 2, 3, yes, yes | not measurable: no import screen (TB-G6) | — |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` and were not re-measured.

#### Moment of truth — the verdict, against the first run's

**First run (2026-08-22)**: "This is where she closes the tab." Seven links and a Sign out button, and neither "birth plan" nor "visit" anywhere on the screen.

**This run**: **she stays.** The first screen speaks her words back to her: "the Client's birth plan, the visits with the Client, and the contract and invoices". It tells her she holds every role, that adding a Client is free, and what starting work costs. It offers one thing to do, and that thing leads to doula work in three presses. TB-G4 is closed in fact, not only on paper. What the screen still gets wrong is around that body, not in it. The top bar puts ten destinations over an empty Practice (TB-G10), and nothing says her Practice was just created (TB-G9). Neither would make her leave; both are what she would mention.

#### Abandon checks — what a real evaluator would do

- **Stage 1** — She still never arrives (TB-G1).
- **Stage 2** — Outside the product, still no price. Inside it, one press from the first screen, the answer is plain: $20.00 a Credit, one Credit per Client relationship, three free. "What does it cost?" is answered the moment she is in, not before.
- **Stage 3** — She would not leave here. Four short pages, every one saying why it asks, and a check page. It is longer than the one screen the map promised (TB-G8), and fast enough not to matter to her.
- **Stage 4** — She stays (above).
- **Stage 5** — She stays. Inventing "Jordan" costs her one box. The Start work form asks more than she expected for a pretend client (Birth or Postpartum, a due date, who the Doula is: TB-G11), but it tells her what it spends before it spends it. The Engagement page is the product she came to judge.
- **Stage 6** — "Can I get out again?" — yes, and the screen says so plainly, but not today. Both doors ask her to set up an authenticator app first (TB-G12). An evaluator reads that two ways: "they take my clients' data seriously", or "they are making it hard to leave". Which one is a question for the usability sessions ([#1685](https://github.com/markgoho/doula-cloud/issues/1685)), not this walk.
- **Stage 7** — Her spreadsheet still cannot come with her; import is ruled out of v1. It can be retyped now, one Client at a time, with every column a Practice adds. For two years of rows, that is the expensive half of switching, and the product still has nothing to say about it.

#### Is the journey completable end to end?

**From `/signup`, yes. From where her journey starts, no.** Stages 3 to 6 can each be done by a person today. The pass condition, a Practice with one test Client in it and an intention to come back, was met. Stage 6's exit needs a second factor, which a person can set up against a real Identity Platform, and only this stack's emulator could not. What stops the journey is **1.1**: there is still no page to arrive at (TB-G1, exempt from this run). Stage 7's import stays missing by decision (TB-G6). Against "a pass means", **this run passes**, and closes on the first reason, where the first run closed on the second.

**Findings filed**: [TB-G8 (#2181)](https://github.com/markgoho/doula-cloud/issues/2181), [TB-G9 (#2182)](https://github.com/markgoho/doula-cloud/issues/2182), [TB-G10 (#2183)](https://github.com/markgoho/doula-cloud/issues/2183), [TB-G11 (#2184)](https://github.com/markgoho/doula-cloud/issues/2184) and [TB-G12 (#2185)](https://github.com/markgoho/doula-cloud/issues/2185), each on the [journey map](../journeys/evaluator-doula.md#gaps-found) with its budget line quoted; and one bug, [#2186](https://github.com/markgoho/doula-cloud/issues/2186). Cited, not filed: [#1784](https://github.com/markgoho/doula-cloud/issues/1784).

### 2026-10-27 — automated marks re-derived for the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329))

A desk pass, not a walk: every `automated` and `manual` step was read against the current `app/e2e/` suite before the second walk.

- **5.3** moves from `automated (birth-plan.e2e.ts)` to `automated (add-client-visits.e2e.ts)`. `birth-plan.e2e.ts` now opens the Engagement by its URL, not from the Clients list. `add-client-visits.e2e.ts` follows the Client's row on the Clients list to her Engagement and asserts its page.

No mark moved from one kind to another, so the Marks summary is unchanged.

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 2, 3, 4, 5, 6, 7 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **36 budget steps**, 6 `automated`, 27 `manual`, 3 `missing-feature`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 5.3 | `birth-plan.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-09-20 — six falsified findings corrected ([#873](https://github.com/markgoho/doula-cloud/issues/873))

A desk pass, not a walk, over the six findings the 2026-09-06/07 triage of [#319](https://github.com/markgoho/doula-cloud/issues/319)'s children falsified: TB-G2's price and scope, TB-G3, TB-G4, TB-G5 and TB-G7 (five gaps, the table below) and the Credit-per-Engagement unit claim (below the table, already correct, needed no change). Every claim below was read out of the code and the closed gap issues, not out of the stale prose.

| Gap | Was | Is, and what settled it |
| --- | --- | --- |
| TB-G2 | A credit costs $5.00; no price anywhere | The in-product half is closed ([#285](https://github.com/markgoho/doula-cloud/issues/285)): `Billing` states `One Credit costs $20.00` ([#439](https://github.com/markgoho/doula-cloud/issues/439), [#448](https://github.com/markgoho/doula-cloud/issues/448)). The marketing-site half is still open, now at [#868](https://github.com/markgoho/doula-cloud/issues/868) and [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| TB-G3 | Nothing says what a credit buys; the ledger origin prints as a raw enum | Closed ([#286](https://github.com/markgoho/doula-cloud/issues/286)): `Billing` states one Credit covers one Engagement, and `originLabel` maps every ledger origin to prose |
| TB-G4 | The first screen is an admin link menu, the abandon point | Closed ([#287](https://github.com/markgoho/doula-cloud/issues/287)): the seven-link menu is gone (redistributed to the shell's top bar and a **Settings** hub), and this screen is `OverviewHub`'s zero-Client state, naming the work and offering one action |
| TB-G5 | No export, no account deletion | Closed, split in two: a whole-Practice export ([#288](https://github.com/markgoho/doula-cloud/issues/288)) and a 30-day-countdown deletion ([#871](https://github.com/markgoho/doula-cloud/issues/871)), both on **Settings** |
| TB-G7 | The Staff screen reads the schema's word, `office_manager`; nothing signposts the roster | Closed: the roster reads `Admin` ([#262](https://github.com/markgoho/doula-cloud/issues/262)), a role-scoped **Staff** nav item signposts it, and signup itself now states what it grants ([#290](https://github.com/markgoho/doula-cloud/issues/290), [#878](https://github.com/markgoho/doula-cloud/issues/878)) |

The Credit-unit claim ("one Credit covers one Engagement, not one Client") was already correct on both documents and needed no correction.

**One mark moved**: 6.1 leaves `missing-feature (TB-G5)` for `manual`, because the feature it named is built. The Marks summary above is recounted from the Steps tables, to 5 / 11 / 0 / 4, and [README.md](README.md)'s run-status row and Total move with it.

[#869](https://github.com/markgoho/doula-cloud/issues/869) merged just ahead of this pass and already swapped `office_manager` for `admin` on the TB-G7 gap row and step 3.3-a, and recorded [#262](https://github.com/markgoho/doula-cloud/issues/262) as closed — its own commit message explicitly deferred the Staff-nav-signposting half to this ticket. This pass builds on that: the Staff-nav fact, the signup-silence fact ([#878](https://github.com/markgoho/doula-cloud/issues/878)) and the full TB-G7 closure are new here. The remaining `office_manager` mentions elsewhere in the repo are either dated walk-log rows #869 deliberately left untouched or other Personas' documents, both out of scope here.

### 2026-09-10 — Add Client corrected against ADR-0017 ([#1121](https://github.com/markgoho/doula-cloud/issues/1121))

A desk pass, not a walk, over the cells [#685](https://github.com/markgoho/doula-cloud/issues/685) held back because they could not be half-corrected: 5.1, 5.2, 5.2-a and 7.1-a. Every claim below was read out of the code, not out of the ticket.

| Cell | Was | Is, and what settled it |
| --- | --- | --- |
| 5.1 | "Two fields" -> a redirecting door onto a one-question-per-page sequence | `clients/new`'s `load` redirects to the name question; the sequence asks name, date of birth, email, phone, address, then one page per section of the Practice's Client Field Template. ADR-0017, and the intake routes under `clients/new` |
| 5.2 | A Client **and** an Engagement in one request, one credit silently spent -> a free save of a Client alone, landing on her detail hub | `client.CreateHandler`'s own test asserts zero `engagements` rows and zero `credit_ledger` rows after a save. The Credit is consumed in `engagementrequest.approve`, tagged to the Engagement it creates |
| 5.2-a | "None. The wall arrives at her fourth Client with a `402`" -> nothing on intake mentions credits because nothing is spent there; the size of the trial is named at 5.2-b | The Engagement Request form renders `Credit cost` and `Balance after` to an approver before the act, and the refusal that follows an empty balance offers **Buy credits** inline |
| 5.2-b | **new** — the act that costs, split out from 5.2 | `RequestHandler` collapses request and approval into one act for an Owner (ADR-0017's solo-Practice rule); `writeApproveErr` answers `402 no credits remaining, ask a practice owner or admin to buy more` and rolls the whole act back |
| 7.1-a | "only name and email can be typed" -> the row can be transcribed by hand, one at a time | Same intake routes, plus `settings/client-fields`. [MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed; [TB-G6](https://github.com/markgoho/doula-cloud/issues/289) (no bulk import) is untouched and still owns 7.1 |

**One mark moved and one step was appended.** 5.1 is `automated (add-client-visits.e2e.ts)`: the spec opens `clients/new` as the founding Owner — which is who Tasha is — and asserts that the first question is what comes back, which is this step's Action and its whole Expected result. The Action is deliberately written as the URL the spec opens rather than as the search that leads to it: the search is 5.1's context, not its assertion, and a cell that claimed the search would be claiming coverage the spec does not have. **5.2 is deliberately held at `manual`**, and the reason is the [four marks](README.md#the-four-marks)' own rule: the spec asserts the landing on the Client detail hub and asserts nothing at all about the absence of an Engagement or a ledger row, which is the whole claim of that cell. The API-level assertion of it lives in `client`'s Go tests, and fixture-shaped evidence is not coverage of a step. 5.2-b is `manual` for the plainer reason that no spec asks for an Engagement through the UI.

The Marks summary is recounted from the Steps table above — 5 / 10 / 0 / 5 — and [README.md](README.md)'s run-status row and Total move with it.

`bun run test:e2e -- add-client-visits` in `app/`, one spec, one run: **1 passed** (51.5s), on the stack [docs/testing/e2e.md](../testing/e2e.md) describes. The re-marked step is the only one this pass moved onto a spec, so the whole suite was not re-run for a Markdown change; CI on the pull request is the run that covers the rest.

| Step | Spec | Result |
| --- | --- | --- |
| 5.1 | `add-client-visits.e2e.ts` | pass |

**5.2-b is appended to the map as well as to the plan.** A plan never renumbers a map, and this does not — but ADR-0017 split one act into two, and a map whose stage 5 still runs 5.1, 5.2, 5.3 would leave the plan's new step with nothing to sit beside. The map carries the same id, and the gap IDs on it are untouched.

**Left alone on purpose.** The 2026-08-22 walk log below is a record of what was seen on the day and is not rewritten. Tasha's other stale cells — TB-G2's credit price, TB-G4's closure, TB-G7's `office_manager`, and the ledger origin printed as a raw enum at 2.1-a — belong to [#873](https://github.com/markgoho/doula-cloud/issues/873), which was open when this pass ran.


### 2026-09-03 — new automated steps ([#318](https://github.com/markgoho/doula-cloud/issues/318))

`bun run test:e2e` in `app/`, whole suite, one run: **30 passed, 0 failed** (29.0s).

| Step | Spec | Result |
| --- | --- | --- |
| 3.1 | `signup-form.e2e.ts` | pass |
| 3.2 | `signup-form.e2e.ts` | pass |
| 3.3 | `signup-form.e2e.ts` | pass |

**3 newly automated steps: all pass**, bringing the plan's total to 4. The 2026-08-22 manual walk below is unchanged as a historical record.

### 2026-08-22 — manual walk ([#233](https://github.com/markgoho/doula-cloud/issues/233))

`bun run dev:full` in `app/`, walked once end to end in a desktop browser at 1280x900 as Tasha Bell. Preconditions: none, as the plan says — signed up from `/signup` with no fixture. Practice `Bell & Co Birth Support`, one Client `Test Client One`. The 1 `automated` step was **not** re-run.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) | as expected | Unwalkable, and the gap needs one word changed: a Hugo **scaffold** exists (`hugo/hugo.toml`, `hugo/layouts/index.html`), it is wired to its own Firebase Hosting target (`firebase.json`, target `hugo`), and what it publishes is `<h1>Hello, World!</h1>` and nothing else (`hugo/public/index.html`). There is no `content/` directory. So the site is not absent — it is empty, which is the same nowhere to arrive at |
| 1.2 | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) | as expected | The words "doula" and "birth plan" appear nowhere in `hugo/` |
| 2.1 | `missing-feature (TB-G2)` [#285](https://github.com/markgoho/doula-cloud/issues/285) | as expected | No price in `hugo/`, and none in the app. **Sharper than the plan claimed**: the one screen that sells credits shows no price either — see 2.1-a |
| 2.1-a | `manual` | as expected | `Billing` — `Credit balance: 3`, one ledger row `8/22/2026, 12:43:58 PM / signup_bonus / +3`, then `Quantity [1]` and a **Buy credits** button. Nothing says what a credit buys, what one costs, or what `signup_bonus` is — the origin is printed as the raw enum. Pressing **Buy credits** used to return `internal error` (HTTP 500) with no Stripe key. Re-run 2026-08-22 against the Sandbox ([#242](https://github.com/markgoho/doula-cloud/issues/242)): it now opens Stripe Checkout and a paid Session credits the ledger from the `checkout.session.completed` webhook. **[TB-G2](https://github.com/markgoho/doula-cloud/issues/285) and [TB-G3](https://github.com/markgoho/doula-cloud/issues/286) stand unchanged** — a credit costs $5.00 and the screen still never says so, nor what a credit buys; the price is visible only once the Practice has already left for Stripe |
| 3.1 | `manual` | as expected | `/signup`: heading `Sign up your Practice`, four fields, one **Create Practice** button. No email confirmation, no terms, no link back to anything |
| 3.2 | `manual` | as expected | The first field is labeled `Practice name`, exactly as the abandon check predicted |
| 3.3 | `manual` | as expected | Landed on `/practices/{id}`. DB after: **one** `practices` row, **one** `staff` row, one `practice_memberships` row with `{owner,office_manager,doula}` — the three roles in one statement, as claimed. **The stage-3 timing claim ("under a minute") was not tested**: a driven browser types faster than a person, so any number here would be fiction |
| 3.3-a | `manual` (was `missing-feature (TB-G7)` [#290](https://github.com/markgoho/doula-cloud/issues/290)) | **falsified** | The **Staff** link on the first screen opens a roster with a `Roles` column reading `owner, office_manager, doula`. She *can* learn that roles exist, in one click. TB-G7 is narrowed, not deleted: nothing signposts it, the only Action offered is `End sessions everywhere` (no role can be changed — [RA-G2](https://github.com/markgoho/doula-cloud/issues/261), owned by Renata's map), and the word on screen is `office_manager`, which is the schema's word and not **Admin** (#204). See the journey map's revised TB-G7 row |
| 4.1 | `manual` | as expected | `Welcome to Bell & Co Birth Support` and the seven links in the order the plan lists them. One control the plan does not name: a `Sign out` button in the banner. That is the whole screen |
| 4.1-a | `manual` | as expected | The complete body text is `Sign out / Welcome to Bell & Co Birth Support / Clients Billing Invite a Staff member Staff Plan Templates Contract Template Payments`. Neither "birth plan" nor "visit" appears |
| 4.2 | `manual` | as expected | Seven links, flat, no descriptions, no empty-state prompt, no ordering cue |
| 5.1 | `manual` | as expected | `Add a Client`: two fields, `Their name` and `Their email`. Nothing on the screen mentions credits |
| 5.2 | `manual` | as expected | One `clients` row, one `engagements` row at `intake`, and a `credit_ledger` row `consumption / -1` at the identical timestamp. Nothing on screen said a credit was spent. **The plan's route is one click long, not two**: **Add Client** lands straight on the Engagement page — she never passes through the Clients list |
| 5.2-a | `manual` | as expected | No warning of any kind, on either screen |
| 5.4 | `manual` | as expected, with one correction | The Engagement page shows `Test Client One`, `Status intake` (raw enum), `Created`, `Send portal invite`, Visits, Care Plan, Birth Plan, Contract, Messages. **Three clicks from the first screen, not four** (Clients → Add a Client → Add Client), because 5.2 lands her here. The Visits table's `Date` column is `visit.createdAt` (`app/src/routes/practices/[practiceId]/engagements/[engagementId]/+page.svelte:525`) — a column labeled Date that holds when the row was typed. Corroborates **[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**; no new gap |
| 6.1 | `missing-feature (TB-G5)` [#288](https://github.com/markgoho/doula-cloud/issues/288) | as expected | Unwalkable and confirmed at the route table: `api/main.go:137-236` registers no export, no download, and no account deletion. The seven links carry no account or settings screen, and the Clients list offers no export |
| 7.1 | `missing-feature (TB-G6)` [#289](https://github.com/markgoho/doula-cloud/issues/289) | as expected | Same route table: no import endpoint exists |
| 7.1-a | `manual` | as expected | Walked and impossible, as claimed: `Add a Client` accepts a name and an email and nothing else (**[MO-G3](https://github.com/markgoho/doula-cloud/issues/252)**) |

**12 `manual` steps walked, 1 re-marked from `missing-feature`; 5 remaining `missing-feature` steps confirmed unwalkable; 0 `blocked` steps (this plan has none).** One gap wording was falsified and rewritten (TB-G7); two were sharpened in place (TB-G1, TB-G2). No gap ID was minted, and no `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

#### Abandon checks — what a real evaluator would do

Recorded as judgment, in Tasha's words, not as pass or fail.

- **Stage 1** — She never arrives. Nothing to close the tab on.
- **Stage 2** — "What does it cost?" is unanswered before signup *and* after it. The Billing screen is worse than silence: it names a currency she has never heard of, prints `signup_bonus` at her, and the one button that might explain the price returns `internal error`. She would not put a card near this.
- **Stage 3** — She would not leave here. Four fields, one screen, no confirmation email. This is the strongest leg, exactly as the map says.
- **Stage 4** — **This is where she closes the tab.** Seven links and a Sign out button. She came to see whether it is built for doulas; the screen answers with a filing cabinet. Nothing on it says the word she was shopping for.
- **Stage 5** — If she pushes past stage 4, the Engagement page rescues her: Birth Plan is on it, in her own words, at three clicks. The finding is not that the page is bad — it is that it sits behind the screen she would have left at, and that reaching it costs her a credit and a Client she had to invent.
- **Stage 6** — "Can I get out again?" — no. Half her stated question has no answer anywhere in the product.
- **Stage 7** — Her spreadsheet cannot come with her, and cannot be retyped either. For a two-doula practice with two years of history, this is the expensive half of switching and the product has nothing to say about it.

**Verdict against "a pass means"**: a Practice with one test Client in it exists, so the mechanical half passed. The intention to come back does not — she leaves at stage 4, for a recorded reason. Both close the journey, and this journey closes on the second one.
