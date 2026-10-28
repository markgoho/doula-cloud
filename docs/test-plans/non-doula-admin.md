# Dee Whitlock — test plan

- **Journey**: [non-doula-admin.md](../journeys/non-doula-admin.md)
- **Persona**: [non-doula-admin.md](../personas/non-doula-admin.md)
- **A pass means**: an Engagement with a Doula on it, a signed Contract, and an Invoice with a recorded Payment — reached without Dee opening a Care Plan or logging a Visit.

## Preconditions

- A Practice with an Owner (Renata) and at least one other Staff member holding `doula`, so there is somebody for Dee to put on the Engagement.
- An invitation issued by the Owner. The link is emailed to the invited address and is not printed on the invite screen (#316), so read it from the mailbox.
- Dee's role rides that Invitation, and **Edit membership** on the Staff screen changes it afterwards (#316). The zero-role membership DW-G1's second run needed is no longer reachable, so run the plan once, with `admin`.

## Steps

### Stage 1 — Accept the invitation

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link at `/accept-invite` | The accept form renders | `automated (admin-invite-role.e2e.ts)` |
| 1.2 | Set email and password, press **Accept invite** | The accept screen is two steps: the account question with email and password, **Continue**, then first name, last name and work state, **Accept invite**. `POST /api/staff/accept-invite` creates the membership with the role the Invitation named (#316), she lands on `Overview`, and her roster row reads `Admin` | `automated (admin-invite-role.e2e.ts)` |
| 1.3 | Choose the Practice from the membership list | **Nothing to choose** — one membership, so `decideLanding` redirects straight to `/practices/[practiceId]` (`app/src/lib/landing.ts:24-26`) and the picker never renders. The picker needs a second membership, which only Lena Vasquez's journey gives a Persona | `manual` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The accept form, then the Practice landing; one membership needs no picker | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `manual` |
| 1.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 1.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 2 — Receive the Admin role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | As the Owner, set `admin` on Dee's membership from a screen | **Edit membership** on Dee's row saves it. The Staff screen prints `Admin`, the team's word — `rolesLabel` replaced the raw enum ([RA-G3](https://github.com/markgoho/doula-cloud/issues/262), closed) | `manual` |

### Stage 3 — Discover what the Admin role grants

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Land on `/practices/[practiceId]` | The tiles are gone (#452). The Overview's top bar is the Owner's ten destinations (Overview, Clients, Schedule, On call, Invoices, Contracts, Credits, Staff, Offers, Settings), and its Quick links include **Credits** and **Getting paid**, which an Admin reads ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267), closed) | `manual` |
| 3.1-a | Compare this run against the zero-role run | **No longer identical.** [DW-G1](https://github.com/markgoho/doula-cloud/issues/269) is closed: `admin` is read now for the Practice's Stripe Connect state (`staffauth.OwnerAndAdmin`, [#267](https://github.com/markgoho/doula-cloud/issues/267)) — see the journey map's Stage 3 note | `manual` |
| 3.2 | Open Credits | The Practice's credit balance and purchase ledger render for an Admin — the balance read is `staffauth.OwnerAndAdmin` since [DW-G4](https://github.com/markgoho/doula-cloud/issues/272), so an Admin is inside the seat and a Doula meets the refusal screen instead | `automated (admin-invite-role.e2e.ts)` |
| 3.2-a | Buy credits | **Not refused.** The purchase declares the same Owner-and-Admin seat the balance does ([#257](https://github.com/markgoho/doula-cloud/issues/257), [#910](https://github.com/markgoho/doula-cloud/issues/910)), and the button is drawn plainly enabled for every session that reaches the screen ([#1162](https://github.com/markgoho/doula-cloud/issues/1162)) | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The Practice landing, then Credits | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

The Billing screen is automated for an *Owner* (`billing.e2e.ts`); no spec reads it as a non-owner, which is the case that matters here.

### Stage 4 — Take the call and create the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | From **Find or add a Client** on the Clients list, search, then follow **Add a new Client** | A Client is found before she is created (ADR-0017). Intake is one question, `What is the Client's name?`, and what she typed into the search (date of birth, email, phone) is listed under it, to be saved with the name ([#1611](https://github.com/markgoho/doula-cloud/issues/1611)). Her page of notes from the call has somewhere to go: the Client's other details, the Practice's own Client Field Template sections among them, are added from her record ([#1610](https://github.com/markgoho/doula-cloud/issues/1610); [MO-G3](https://github.com/markgoho/doula-cloud/issues/252) closed) | `manual` |
| 4.2 | Press **Save and continue** on the name question | `client.CreateHandler` refuses only a contractor doula, so it **passes** for Dee — and the save is **free**: a Client, no Engagement, no credit. She lands on the Start work form for that Client, which says "{name} saved as a Client." ([#1710](https://github.com/markgoho/doula-cloud/issues/1710)). Starting paid work with her is a separate act, the Engagement Request, whose approval is what creates the Engagement and locks the Credit (ADR-0017); its second action, **Go to {name}'s record without starting work**, leaves the Client saved and nothing started | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 8 (Hick's Law, Flow). The Clients list, the search, the name question and the Start work form: four ([#1611](https://github.com/markgoho/doula-cloud/issues/1611)) | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). The largest is the Start work form: the kind of work, the due date, "Who is the Doula?", the Note, **Start work with {name}** and **Go to {name}'s record without starting work**, with a radio group or a select as one decision. The name question is 4: the given name, the two optional names, and **Save and continue** | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The name question lists what she typed into the search before the save | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 5 — Put a Doula on the Engagement

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open the Engagement and look for a control that puts a Doula on it | On a new Engagement, the Start work form's **Who is the Doula?** puts an employee Doula on directly, and the Engagement's Doula row and Activity name her ([RA-G4](https://github.com/markgoho/doula-cloud/issues/225), closed). On an existing one, an Admin who holds no Doula role sees no **Change** on the Doula row; **Offer this work** is the only route ([RA-G13](https://github.com/markgoho/doula-cloud/issues/2230)) | `manual` |
| 5.2 | Create a Visit naming the Doula instead | **It works, and it is her act.** The form asks **Who is this Visit for?** and `resolveAssignee` (`api/internal/visit/roles.go`) puts naming a colleague with the Owner and the Admin — a plain Doula cannot read the roster, so she has nobody to pick from. What Dee cannot do is log a Visit for *herself*, holding no Doula role, and the screen offers her no control for that rather than a button that 403s ([DW-G6](https://github.com/markgoho/doula-cloud/issues/274) closed) | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). On a new Engagement, which Doula is a question on the Start work form ([#1515](https://github.com/markgoho/doula-cloud/issues/1515)), so the largest screen is that form's six, as 4.B-decisions counts them | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — Send the Contract

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Send the portal invite | `POST .../portal-invite` succeeds for a non-owner | `manual` |
| 6.2 | Build the Contract | `POST .../contract` creates it at `draft`, declared `AnyStaff` at the mount, so an Admin passes. The Client's name, the Practice's name and the price are resolved from data the product already holds rather than typed, and `price` is reserved outright — the form renders no input for it at all, and the other two arrive already filled | `manual` |
| 6.3 | Send it | Status `sent` — but **Send Contract** is withheld until every remaining merge field is filled, and again until the Client has been sent a portal invite, each with the reason on screen rather than a refusal after the click | `manual` |
| 6.3-a | Try to edit the Practice's **Contract Template** | Refused: the template write is declared Owner-only at the mount now, rather than checked inside the handler. The page still renders and shows the real template first ([PR-G4](https://github.com/markgoho/doula-cloud/issues/279)) | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The Engagement, then its Contract | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law) | `manual` |
| 6.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). Sending says what the Client receives next | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — Track the signature

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open the Engagement and read the Contract status | One of `draft` / `sent` / `signed` / `voided`, and any open or declined void request standing against it | `manual` |
| 7.1-a | Void a **signed** Contract from the Engagement page | The **Void** button renders only on a `signed` Contract, and `POST .../contract/void` still succeeds for Dee — it is declared **Owner and Admin** at the mount now, rather than carrying no role check at all, and she is the Admin. A Doula in her place gets **Request a void** instead, which an Owner or Admin then grants or declines. `signed` is the only status the transition accepts; anything else 409s. The Contract becomes `voided`, terminal, and still renders in full | `manual` |
| 7.2 | Find every Engagement whose Contract is unsigned | **Contracts** in the top bar lists every Contract the Practice is still waiting on, oldest first, with its status and how long it has waited, and the void requests nobody has decided ([DW-G5](https://github.com/markgoho/doula-cloud/issues/273), closed) | `manual` |
| 7.2-a | Raise an Invoice against the Contract just voided | **Refused.** **Create Invoice** is gone from a `voided` Contract, the Invoices section says invoicing waits for a new Contract, and `POST .../contract/invoices` answers `409` ([DW-G7](https://github.com/markgoho/doula-cloud/issues/275), closed) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). One list of every unsigned Contract, with no Engagement opened | `manual` |
| 7.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Nadia crossing, settled.** `POST .../contract/void` had no step here. [Her plan](loss-client.md) needs a voided Contract for its stage 6, and the only control that produces one is on this screen — so **7.1-a is added** and the void path is walked once, on the Staff side that owns it. Stages 8 and 9 are **unchanged**: what she adds there is the Client's side of the same moment — the bare word `voided` (**NH-G5**) and the absence of any Invoice surface in the portal (**NH-G6**) — and both gaps are hers to own. The step ids here are otherwise untouched.

### Stage 8 — Raise the Invoice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | `POST .../contract/invoices` | **Creates the Invoice.** On a Practice that bills by card and whose Stripe account is connected, **Create Invoice** on a signed Contract takes the Contract's amount, asks nothing, and answers "Invoice created." with a row: the reference, the day raised, the amount and **Open**, Stripe-backed (`billingMode: "stripe"`). Dee is not an Owner; the endpoint never was owner-gated. On a card-billing Practice that has *not* connected, **Create Invoice** is withheld and the endpoint answers `409 FAILED_PRECONDITION` "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe before an Invoice can be sent." | `manual` |
| 8.2 | Read the message the UI shows | On a Practice that bills by card and has not connected: no **Create Invoice**, and "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.", with no link. **Getting paid** says the same and offers **Email the Practice Owners** ([DW-G2](https://github.com/markgoho/doula-cloud/issues/270), closed; the Engagement's missing link is on [#2200](https://github.com/markgoho/doula-cloud/issues/2200)) | `manual` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 2 (Hick's Law, Miller's Law). The amount, and **Create Invoice** | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**8.1 was `blocked` for most of 2026-08-22, and cleared on Dee's own walk.** The reason changed four times in a day, which is worth keeping because each change was real work rather than a re-reading.

1. The Sandbox did not exist. [#242](https://github.com/markgoho/doula-cloud/issues/242) created it, and Credits cleared.
2. Stripe refused `POST /v1/accounts` for new integrations while every merged Connect path was Accounts v1. [#247](https://github.com/markgoho/doula-cloud/issues/247) moved the leg to Accounts v2.
3. No Practice had been through the hosted onboarding. Two attempts from a Playwright-launched Chromium were **CAPTCHA'd** — headless at the email step, headed at the password step.
4. Driven through the user's own Chrome with `playwriter`, **no CAPTCHA appeared on any screen** and it completed. `card_payments` and `payouts` both went `active`, written by the `capability_status_updated` thin event rather than by a poll.

**The CAPTCHA is not the boundary; the browser is.** Stripe challenged a Playwright-launched Chromium in *both* headless and headed mode — at the email step and at the password step respectively — and never challenged the user's own Chrome driven through `playwriter`, across all nine screens. So this is not an attendance requirement and the step is not closed to automation: it is walkable unattended **provided it is driven through a real browser**, which is why 8.1 is `manual` rather than `blocked`. See [connect-onboarding.md](connect-onboarding.md) for the recipe, so no later walk re-derives it.

### Stage 9 — Record the Payment (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Mark an Invoice paid after a bank transfer | **Record payment** on the Invoice's row asks Method (Check, Bank transfer, Cash, Other), Note and Date received, then a check step, **Confirm and record**. The row reads `Paid` with the date, and Activity names who recorded it ([DW-G3](https://github.com/markgoho/doula-cloud/issues/271), closed). The method and the note are read back nowhere ([DW-G12](https://github.com/markgoho/doula-cloud/issues/2241)) | `manual` |
| 9.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The Invoice, then the form that records the Payment | `manual` |
| 9.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). The amount, the date it arrived, how it was paid, and the save | `manual` |
| 9.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). The Invoice's amount and reference stay on screen while the Payment is recorded | `manual` |
| 9.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 9.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The Invoice reads paid, says by what and when, and the book is closed | `manual` |
| 9.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 9.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

A live Stripe account would not fix this step. It is not the out-of-scope Stripe gap; the missing capability is manual Payment recording.

### Stage 10 — Read a filled Care Plan or Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 10.1 | Open an Engagement and read both plan sections | Both render. `GET .../plans/{planType}` has no role check, and per [ADR-0006](../adr/0006-read-follows-the-role.md) an Admin reading both is **correct**, not a leak | `manual` |
| 10.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 10.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 10.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 12 (9 of them [budget steps](README.md#budget-steps)) |
| `manual` | 52 (31 of them [budget steps](README.md#budget-steps)) |
| `blocked` | 0 (8.1 cleared on the walk — Connect completed) |
| `missing-feature` | 0 (RA-G4, DW-G3 and DW-G5 are built, and their steps were walked on 2026-10-28) |

Stages 8 and 9 sat either side of the `blocked` / `missing-feature` line on the first run, and that walk proved the line was drawn in the right place: connecting a live Stripe account **did** clear 8.1 and did **not** touch 9.1, because recording a bank transfer had no code path at all. [DW-G3](https://github.com/markgoho/doula-cloud/issues/271) built that path, and the second walk (2026-10-28) recorded a bank transfer by hand, so this plan now has neither a `blocked` nor a `missing-feature` step.

`admin-invite-role.e2e.ts` ([#965](https://github.com/markgoho/doula-cloud/issues/965)) reaches the app as an Admin who holds neither `owner` nor `doula`, invited and accepted through the real invite and accept screens. It drives 1.1 and 3.2, and asserts two things no step here names: her roster row reads `Admin` and nothing else, and the Owner-only invite send refuses her with `only a Practice Owner can do that`. Every other spec in the suite still runs as an Owner who signed up.

## Run log

### 2026-10-28 — second manual walk ([#2175](https://github.com/markgoho/doula-cloud/issues/2175))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, on trunk (this record written at `30de97c3`), in WSL2 with rootless Podman. `scripts/stripe-listen.sh` was running against it, with the Stripe test-mode key (`sk_test_`) and the publishable key set. Not restarted.

**Device**: headless Chrome inside WSL, driven by the chrome-devtools MCP, all at 1280x900 in UTC. One isolated browser context per person: Renata Alvarez (Owner), the Stage 1 invitee, the seeded Dee Whitlock for stage 9, and Ines Calloway (the stage 6 Client) in the portal. The invitee's context was new when she opened the invite link, so it is stages 1 and 3's Cold walk; the seeded Dee's context was new when she signed in, so it is stage 9's. Every timing below is on an unbundled Vite dev server, which compiles a route the first time it is visited; each timed act was repeated warm and the warm numbers are given.

**Accounts and preconditions**, all on the seeded Rooted Birth Collective (`19a0107e-…`), built through the product:

- **Stage 1**: Renata invited `dee.walk@sim.doula.cloud` from **Invite a Staff member** with **Admin** and **Employee**; the mail was opened in the catcher and accepted in its own context as "Dee Walker-Whitlock" (a different family name from the seeded Dee, so the roster can tell them apart), New York. Her password is a generated local-emulator value, kept out of this document. Stages 1 to 7 and 10 were walked as her.
- **Stage 9**: the seeded `dee.whitlock@sim.doula.cloud` (Admin, not a Doula), with the seed password ([walk-a-screen-locally.md](../testing/walk-a-screen-locally.md)), on Imani Brooks's seeded overdue Invoice `INV-0003` ($950.00, by hand).
- **Stages 4 to 7**: a new Client, Ines Calloway (`ines.calloway@sim.doula.cloud`), birth, due Mar 14, 2027, Doula Priya Raman, saved and started by Dee (Credits 15 to 14). Ines accepted the portal invite and signed the Contract in her own context. 7.1-a and 7.2-a used **Tessa Moreau's** signed Contract, as the run sheet says.
- **Stage 8**: after 9.1, Renata switched **Getting paid**'s billing mode from **By hand** to **By card, through Stripe**, and 8.2 was walked unconnected. Then Rooted was connected, in the founder's own Chrome through Claude in Chrome, along [connect-onboarding.md](connect-onboarding.md)'s path: as Renata, the orchestrating agent answered **Getting paid**'s website question ("I have my own website", `https://rootedbirthcollective.com`), which **Connect Stripe** waits on, and pressed **Connect Stripe**. The first Account Link expired while waiting for the founder, and **Continue Stripe onboarding** opened a fresh one. The founder typed the Stripe login and its authenticator code, the SSN's last four, and pressed Link's "Agree and continue" and the final "Agree and submit"; the agent filled the rest: an unregistered business with no EIN (a test simplification; a real collective would be an LLC), Renata Alvarez as the representative with a Stripe test identity in Rochester, NY 14604, the Test (Non-OAuth) bank's "Success" account, Radar Pro (free in test mode; the Lite link did not respond), no climate contribution and no Stripe Tax. The connected account went `card_payments` `active` with no requirements about two minutes after submit, at about 21:22:45 UTC, and 8.1 was walked after it.
- **The first run's bypasses are all gone**: the Practice, its Doulas, Dee's membership and her role came from the seed or from screens, with no `POST /api/staff/signup`, no API invitation and no `PATCH .../staff/{staffId}/roles`.

The plan's `automated` steps were **not** re-run as specs; 1.1 and 1.2 were walked anyway, since the stage opens on them.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `automated (admin-invite-role.e2e.ts)` | as the spec asserts | "Accept your Staff invite", "Do you already have a DoulaCloud account?" with "I'm new here — create an account" and "I already have an account — sign in", **Continue** |
| 1.2 | `automated (admin-invite-role.e2e.ts)` (was `manual`) | cell corrected | Choosing "I'm new here" revealed Email address (not filled from the invite) and Password; **Continue**; "Tell us about yourself", First name, Last name, "Which state do you work from?", **Accept invite**; the Overview. Her roster row: "Admin", "Employee"; her Membership history: "Joined as Admin (Employee)". The spec asserts the same landing and row |
| 1.3 | `manual` | as expected | One membership, so the accept went straight to the Overview |
| 2.1 | `manual` | as expected | Renata opened Dee Walker-Whitlock's page, **Change** on Roles: "Which roles does Dee Walker-Whitlock have?", Admin already checked by the Invitation. **Save**: "Dee Walker-Whitlock's roles are now Admin." No history row was added for an unchanged save. The announced page title after the save was "— DoulaCloud" ([#2246](https://github.com/markgoho/doula-cloud/issues/2246)) |
| 3.1 | `manual` | **falsified** (the tiles are gone), cell corrected | The top bar's ten destinations, as an Owner's. Quick links: Clients "22 active", Credits "14 Credits", Your offers "None waiting on you", Staff, Plan Templates, Contract Template, Getting paid |
| 3.1-a | `manual` | as expected | `GET .../payments/connect` answers her `200` with the Practice's status; **Getting paid** shows her the billing mode, payment terms and Stripe status, without the billing-mode **Change** |
| 3.2 | `automated (admin-invite-role.e2e.ts)` | read on the walk | "Credit balance: 14" and a ledger of "Engagement started -1" rows, newest first. The seeded balance differs from the spec's 3, as the run sheet said |
| 3.2-a | `manual` | as expected | Quantity 1, **Buy credits**: Stripe Checkout, "Engagement credit", $20.00. Paid with the 4242 test card; back on Credits with "Credit purchase complete. The balance updates once Stripe confirms payment." and "Credit balance: 15", a "Purchase +1" row |
| 4.1 | `manual` | as expected | **Find or add a Client**, "Find a Client", Name, Date of birth, Email address, Phone number, **Search**: "No matches", **Add a new Client**. "What is the Client's name?", "You searched for "Ines Calloway". Type the name into the fields below.", and under "From your search, also saved with the name:" Apr 9, 1992, the email and (585) 555-0142 |
| 4.2 | `manual` | as expected | **Save and continue**: "Ines saved as a Client." on "Start work with Ines Calloway", with "Starting work with Ines uses 1 Credit. This Practice has 15 Credits. After this, it has 14." **Start work with Ines** created the Engagement: "Work with Ines started." |
| 5.1 | `manual` (was `missing-feature (RA-G4)`) | **walked** | "Who is the Doula?" on the Start work form listed "No Doula yet" and the twelve employee Doulas; Priya Raman. The Engagement: "Doula Priya Raman", "On call from Feb 21, 2027 to Mar 28, 2027.", and Activity "Put on this Engagement as the Doula: Priya Raman", Dee Walker-Whitlock. The Doula row carries no **Change** for her ([RA-G13](https://github.com/markgoho/doula-cloud/issues/2230)) |
| 5.2 | `manual` | as expected | **Add a Visit**, "Who is this Visit for?" (every Doula, contractors without an accepted Offer marked "(cannot be named yet)"; it opens on "Choose a Doula", not on Priya), Priya Raman, Nov 10, 2026, 2:00pm, **Save Visit**: "The Visit for Priya Raman is added." No control logs a Visit for herself |
| 6.1 | `manual` | as expected | **Send portal invite**: "Invited. An email has been sent to them. …" with the link ([#2198](https://github.com/markgoho/doula-cloud/issues/2198)); Portal invite "Invite pending". The mail reached Ines in the catcher |
| 6.2 | `manual` | as expected | **Create a Draft Contract**: Practice name and Client name filled, Scope of service and both dates blank, no price input. **Save Contract**: "Contract saved.", "Draft, not sent yet", Fee $1,500.00. The dates are free text, and the prose prints them as typed ([#2222](https://github.com/markgoho/doula-cloud/issues/2222)) |
| 6.3 | `manual` | as expected | **Send Contract**: "Contract sent.", "Sent, waiting for the Client to sign", "Sent Oct 28, 2026 by Dee Walker-Whitlock" |
| 6.3-a | `manual` | as expected | "Only a Practice Owner can change these terms." above the real prose and the merge-field legend, no **Save**; a `PUT .../contract-template` from her session answers `403 only a Practice Owner can do that`. The prose is in an editable field all the same ([#2244](https://github.com/markgoho/doula-cloud/issues/2244)) |
| 7.1 | `manual` | as expected | Ines's: "Sent, waiting for the Client to sign", then, after she signed in the portal, "Signed on Oct 28, 2026", "Signed by Ines Calloway", **Void this Contract**. Tessa's: "Signed on Oct 27, 2026" |
| 7.1-a | `manual` | as expected | On Tessa Moreau's: **Void this Contract**, a dialog "Void this Contract?" with its warning, **Void this Contract**: "Voided, no longer active" in 57 ms; the prose and **Download signed Contract (PDF)** stay. Activity "Contract for the Client voided", Dee Walker-Whitlock (after a reload) |
| 7.2 | `manual` (was `missing-feature (DW-G5)`) | **walked** | **Contracts**: Grace Whitfield, Sent, 5 hours ago; Ines Calloway, Sent, 1 minute ago; and "No void request is waiting on you." Ines left the list once she signed. The intro lower-cases "practice" and "engagement" ([#2247](https://github.com/markgoho/doula-cloud/issues/2247)) |
| 7.2-a | `manual` | **falsified** (it is refused now), cell corrected | **Create Invoice** gone, and "This Contract has been voided. Invoicing is unavailable until Staff issues a new Contract." ([#2221](https://github.com/markgoho/doula-cloud/issues/2221)); the direct `POST .../contract/invoices` answers `409 This Contract is voided, so you cannot create an Invoice for it.` DW-G7 holds closed |
| 8.1 | `manual` | **passes**, cell corrected | Before Connect, the direct `POST .../contract/invoices` on Ines's signed Contract answered `409 FAILED_PRECONDITION` "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe before an Invoice can be sent.", not the `200 {"connectRequired":true}` the cell named. After Connect, as the seeded Dee in a new context: "No Invoices yet." and **Create Invoice**; one press, busy within 8 ms, and 2.45 s later (the Stripe round trip) "Invoice created." and a row `BMQ31P9W-0001`, Raised Oct 28, 2026, $1,500.00, **Open**, with **Record payment**. Stored: `open`, `150000`, due 2026-11-27, `billingMode: "stripe"`. No question was asked, since the billing mode was already set |
| 8.2 | `manual` | as expected, cell corrected | Billing by card, not connected. Ines's Invoices section: "No Invoices yet." and "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe.", no **Create Invoice**, no link. **Getting paid**: "A Practice Owner has to connect Stripe." and **Email the Practice Owners**, which she pressed: "Every Practice Owner is being emailed about connecting Stripe. DoulaCloud sends this at most once a week."; Renata's inbox got "DoulaCloud: your Practice still has to connect Stripe". By hand, earlier, the Invoices screen told her the same "cannot pay" sentence ([#2200](https://github.com/markgoho/doula-cloud/issues/2200)) |
| 9.1 | `manual` (was `missing-feature (DW-G3)`) | **walked** | As the seeded Dee, **Invoices**, Imani Brooks "INV-0003 $950.00 Overdue By hand … 13 days overdue", her Engagement, **Record payment**: Method, Bank transfer; Note "Bank transfer from Imani Brooks, reference IB-0950, seen on the statement Oct 26."; Date received Oct 26, 2026; **Continue**: "Record a payment of $950.00" with each answer and **Change**; **Confirm and record**: the row reads "Paid Oct 26, 2026", with **Return money** and **Reverse payment**. Activity: "Payment from the Client recorded", Dee Whitlock. The **Invoices** screen then reads it Paid. The method and the note are shown nowhere ([DW-G12](https://github.com/markgoho/doula-cloud/issues/2241)) |
| 10.1 | `manual` | as expected | Camille Boyd's Engagement: Care Plan "2 of 4 questions answered", "Last edited Oct 27, 2026 by Renata Alvarez"; Birth Plan the same, by Priya Raman, "Camille has not read this yet", **Print for the hospital**. Reading the answers took **Change the Care Plan** and **Change the Birth Plan**, each an editor; **Cancel** left both as they were ([DW-G14](https://github.com/markgoho/doula-cloud/issues/2243)) |

**Re-marked**: 5.1, 7.2 and 9.1 from `missing-feature` to `manual`, their gaps built. 1.2 from `manual` to `automated (admin-invite-role.e2e.ts)`, its cell corrected to what the spec asserts. 3.1 and 7.2-a were **falsified** and corrected; 8.1's and 8.2's cells are corrected; their marks hold. The Marks summary is recounted to 12 / 52 / 0 / 0, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**: this plan's own budget notes count each field singly and each choice as one ("how it was paid"), and say nothing on groups, so the rule of the earlier second-run walks is used, as on Renata Alvarez's: fields, options, and links or buttons that move her somewhere or change what happens, enabled and in view at once in the page body. A radio group, a checkbox group, a select and a day-month-year date are one decision each; **Cancel** counts. **Back**, the skip link, the Privacy link and **Show password** are not counted, nor the Staff top bar. **Time** is measured in the page: a script presses the control and polls each animation frame until the result has settled on screen, and records when the control showed it was busy, or for a top-bar move when anything on the page first changed.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 2 | **3**: the account question, "Tell us about yourself", the Overview | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238) |
| 1.B-decisions | 3 | **4** on each accept screen (the account radio group, Email address, Password, **Continue**; First name, Last name, work state, **Accept invite**); the Overview's body 9 ([RA-G11](https://github.com/markgoho/doula-cloud/issues/2228), cited) | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238) |
| 1.B-confirmation | yes | **no**. The Overview names the Practice in the top bar and says nothing about joining or the role | **no**, [DW-G10 (#2239)](https://github.com/markgoho/doula-cloud/issues/2239) |
| 1.B-cold | yes | yes, in a fresh context. Each question says what it is for, and the refusal on an empty **Continue** names both fields | yes |
| 3.B-screens | 2 | 2: the Overview, then Credits | yes |
| 3.B-cold | yes | yes. Credits says what a Credit is and costs before the balance | yes |
| 4.B-screens | 8 | 5: Clients, Find a Client, the name question, the Start work form, the Engagement | yes |
| 4.B-decisions | 3 | **6** on the Start work form; Find a Client 5, the name question 4 | **no**, [TB-G11 (#2184)](https://github.com/markgoho/doula-cloud/issues/2184), cited |
| 4.B-memory | yes | yes. Find a Client's answers are listed under the name question, and the name she searched for is quoted there; the Start work form names the Client in its heading and button | yes |
| 4.B-time | 400 ms, 100 ms | **Search** 32 ms, **Add a new Client** 37 ms, **Save and continue** 117 ms, **Start work with Ines** 203 ms, each busy within 3 ms | yes |
| 4.B-confirmation | yes | yes. "Work with Ines started." with the Doula and the on-call window | yes |
| 5.B-screens | 1 | 1: the Start work form (for a new Engagement); **Add a Visit** is on the Engagement page | yes |
| 5.B-decisions | 2 | **6** on the Start work form, where "Who is the Doula?" is; **Add a Visit** 4 (who, when, **Save Visit**, **Cancel**) | **no**, [TB-G11 (#2184)](https://github.com/markgoho/doula-cloud/issues/2184) and [RA-G14 (#2231)](https://github.com/markgoho/doula-cloud/issues/2231), cited |
| 5.B-confirmation | yes | yes. The Doula row, Activity "Put on this Engagement as the Doula: Priya Raman", and "The Visit for Priya Raman is added." | yes |
| 6.B-screens | 2 | 1: the Engagement; the Contract is built and sent in its section | yes |
| 6.B-decisions | 7 | 7 on the Draft Contract (five fields, **Save Contract**, **Cancel**) | yes |
| 6.B-memory | yes | yes. The Client's name, the Practice's name and the fee are filled in, and **Read the Contract text** shows the prose before **Send Contract** | yes |
| 6.B-confirmation | yes | yes. "Contract sent." and "Sent, waiting for the Client to sign", by whom and when. Nothing is mailed to the Client; she is told by push (`contracts/send.go`) or on her next portal visit | yes |
| 7.B-screens | 1 | 1: **Contracts** | yes |
| 7.B-time | 400 ms, 100 ms | **Contracts** from the top bar settled in 50 to 51 ms over four warm presses | yes |
| 8.B-screens | 1 | 1: the Engagement, for 8.1 and for 8.2 | yes |
| 8.B-decisions | 2 | 1: **Create Invoice**, which takes the Contract's amount and asks nothing on a Practice whose billing mode is set | yes |
| 8.B-confirmation | yes | yes. "Invoice created." and the row with its reference, day, amount and **Open**, as Maya Okonkwo's 7.B-confirmation read it. It does not say the Client has been sent it, or when it is due | yes |
| 9.B-screens | 2 | **4**: Invoices, the Engagement's Invoices section, **Record payment**, its check step | **no**, [DW-G11 (#2240)](https://github.com/markgoho/doula-cloud/issues/2240) |
| 9.B-decisions | 4 | **5**: Method, Note, Date received, **Continue**, **Cancel**. No amount field | **no**, [DW-G11 (#2240)](https://github.com/markgoho/doula-cloud/issues/2240) |
| 9.B-memory | yes | yes. The Invoice's row, `INV-0003` and $950.00, stays above the form, and the check step repeats the amount and every answer | yes |
| 9.B-time | 400 ms, 100 ms | **Invoices** from the top bar 100 to 114 ms over five presses, nothing drawn before it settled; the Engagement's Invoices section 295 to 363 ms, acknowledged in 37 to 94 ms; **Continue** 3 ms; **Confirm and record** 167 ms, busy within 2 ms | **no**, [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), cited |
| 9.B-confirmation | yes | **no**. The row reads Paid and the date. Nothing says by what, no message says it was recorded, and focus falls to the page | **no**, [DW-G12 (#2241)](https://github.com/markgoho/doula-cloud/issues/2241) |
| 9.B-cold | yes | yes, in a fresh context. **Record payment** is on the overdue Invoice's row, and each answer is named | yes |
| 10.B-screens | 1 | **3**: Clients, the Client's record, the Engagement | **no**, [DW-G13 (#2242)](https://github.com/markgoho/doula-cloud/issues/2242) |
| 10.B-time | 400 ms, 100 ms | The record to the Engagement, plans settled, 166 to 183 ms over four warm presses, acknowledged in 55 to 56 ms | yes |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` and were not re-measured. **Failed**: 1.B-screens, 1.B-decisions, 1.B-confirmation, 4.B-decisions, 5.B-decisions, 9.B-screens, 9.B-decisions, 9.B-time, 9.B-confirmation, 10.B-screens.

#### Moment of truth — the verdict, against the first run's

**Stage 9, recording the Payment.** First run (2026-08-22): the book stays open; money that arrived by bank transfer cannot be written down anywhere. **This run: overturned.** Dee, an Admin who is not an Owner and not a Doula, found Imani Brooks's overdue $950.00 Invoice on **Invoices**, recorded a bank transfer received Oct 26 with a note, checked it, and confirmed it: the Invoice reads Paid on Oct 26, 2026, the Practice-wide total moves, and Activity says she recorded it. The book closes. What it does not do is keep the answer to "how was this paid?": the method and the note she typed are shown on no screen afterwards, and nothing tells her the Payment was recorded ([DW-G12](https://github.com/markgoho/doula-cloud/issues/2241)).

#### Is the journey completable end to end?

**Yes.** Against "a pass means": an Engagement with a Doula on it (Ines Calloway, Priya Raman, put on by Dee), a signed Contract (sent by Dee, signed by Ines), an Invoice Dee raised herself through Stripe once Renata connected (`BMQ31P9W-0001`), and an Invoice with a recorded Payment (Imani Brooks's bank transfer, recorded by Dee), all without Dee opening a Care Plan or logging a Visit. Every `missing-feature` step from the first run is walked, and no step stops her. Two things fall short of the letter: on an existing Engagement she cannot change the Doula except by an Offer (RA-G13), and the Payment she recorded does not say how it was paid (DW-G12). Ines's Invoice and Imani's Payment are on two different Engagements, because Rooted billed by hand until 9.1 was walked and by card after.

**Findings filed**: [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), [DW-G10 (#2239)](https://github.com/markgoho/doula-cloud/issues/2239), [DW-G11 (#2240)](https://github.com/markgoho/doula-cloud/issues/2240), [DW-G12 (#2241)](https://github.com/markgoho/doula-cloud/issues/2241), [DW-G13 (#2242)](https://github.com/markgoho/doula-cloud/issues/2242) and [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), each on the [journey map](../journeys/non-doula-admin.md#gaps-found), the five failed budget steps with their budget lines quoted; and four bugs, [#2244](https://github.com/markgoho/doula-cloud/issues/2244) (Contract Template's editable field for a non-Owner), [#2245](https://github.com/markgoho/doula-cloud/issues/2245) (the Overview's empty lines while loading), [#2246](https://github.com/markgoho/doula-cloud/issues/2246) (a placeholder page title announced) and [#2247](https://github.com/markgoho/doula-cloud/issues/2247) (lower-case domain words). Cited, with a comment on each: [TB-G11 (#2184)](https://github.com/markgoho/doula-cloud/issues/2184), [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), [RA-G13 (#2230)](https://github.com/markgoho/doula-cloud/issues/2230), [RA-G14 (#2231)](https://github.com/markgoho/doula-cloud/issues/2231), [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), [#2198](https://github.com/markgoho/doula-cloud/issues/2198), [#2200](https://github.com/markgoho/doula-cloud/issues/2200), [#2221](https://github.com/markgoho/doula-cloud/issues/2221) and [#2222](https://github.com/markgoho/doula-cloud/issues/2222).

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 3, 4, 5, 6, 7, 8, 9, 10 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **40 budget steps**, 9 `automated`, 31 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-09-28 — the first Admin spec ([#965](https://github.com/markgoho/doula-cloud/issues/965))

`admin-invite-role.e2e.ts` added. Steps 1.1 and 3.2 move from `manual` to `automated`: the spec opens the invite link on `/accept-invite` as the invited Admin, and opens Credits as her to read the balance and the `Welcome credits` ledger row. 1.2 stays `manual`, because its Expected result ("zero roles") predates the role riding the Invitation; the spec asserts the accepted membership reads `Admin` instead, which is a different claim. Nothing else moved. Counts go from 0 automated / 21 manual to 2 / 19. No step is renumbered.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

This plan has **no** `automated` step, so the suite says nothing about it. Every step below stage 1 waits on the walk.

### 2026-09-10, later — one button name ([#1121](https://github.com/markgoho/doula-cloud/issues/1121))

4.2's Action read **Add Client**. There is no such button: intake ends on a check-answers page whose control is **Save this Client**. The Expected result beside it, corrected by [#685](https://github.com/markgoho/doula-cloud/issues/685), was already right. No mark moved and no count changed. Found sweeping for the same pre-ADR-0017 story on Tasha Bell's and Maya Okonkwo's plans, and corrected here rather than left because the map's own stage 4 was corrected in the same pass.

### 2026-09-10 — narrative reconciliation ([#685](https://github.com/markgoho/doula-cloud/issues/685))

A desk pass, not a walk. [#318](https://github.com/markgoho/doula-cloud/issues/318) left this plan's Add Client, Visits and Contract wording alone because it had already gone stale independently of the specs #318 added. Corrected here against the code as it stands; nothing was re-walked and no mark moved.

| Step | Cell corrected | What settled it |
| --- | --- | --- |
| 4.1 | Two fields, Name and Email -> a one-question-per-page intake sequence reached through **Find or add a Client** | ADR-0017, and the intake routes under `clients/new`. [MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed |
| 4.2 | "Client and Engagement are created at `intake`" -> the save is free, creates no Engagement and locks no Credit; the Engagement Request is the separate ask | ADR-0017, and `api/internal/client/create.go`'s `CreateHandler` doc comment |
| 5.2 | Refused with a `403` -> naming a colleague on a Visit is exactly the Admin's act; what she cannot do is log one for herself | `resolveAssignee` in `api/internal/visit/roles.go`, and the Engagement page's `canAssignVisits` / `canLogOwnVisit` split. [DW-G6](https://github.com/markgoho/doula-cloud/issues/274) is closed |
| 6.2 | "not owner-gated" and "six blank merge-field inputs" -> declared `AnyStaff` at the mount, and three of the fields are resolved rather than typed, `price` having no input at all | `api/internal/contracts/mount.go`, `editableMergeFields` in `app/src/lib/contract.ts`, and `resolveMergeFieldValues` in `api/internal/contracts/contract.go` |
| 6.3 | "Status `sent`" -> **Send Contract** is withheld until the merge fields are filled and the Client has portal access | The Engagement page's two `Notice` blocks on the draft Contract |
| 7.1 | Added the void requests standing against the Contract to what the status line carries | `ContractStatus.svelte`'s `voidRequests` |
| 6.3-a | "`contracts/template.go:75` requires Owner" -> the Owner gate is declared at the mount now | `api/internal/contracts/mount.go` |
| 7.1-a | "no role check, only `staffauth.Middleware`" -> declared Owner-and-Admin at the mount; a Doula gets **Request a void** instead | `api/internal/contracts/mount.go`, and `TransitionVoid` in `lifecycle.go` for the `signed`-only rule, which is unchanged |

**No step is re-marked.** Both new specs run as the founding Owner. Every cell above is a claim about what an Admin gets, which is the thing neither spec exercises — the README's rule is that a mark counts only where the spec drives the step the way the Persona would.

**The Marks summary above is also recounted**, from 4 `missing-feature` to 3: step 2.1 stopped being `missing-feature (RA-G2)` when **Edit membership** landed, and the summary kept counting it. Arithmetic, not a re-mark.

**Left alone on purpose.** 7.2-a's [DW-G7](https://github.com/markgoho/doula-cloud/issues/275) and 5.1's [RA-G4](https://github.com/markgoho/doula-cloud/issues/225) both cite closed issues. Neither is an Add Client, Visits or Contract-signing step, and re-marking either needs the walk [#329](https://github.com/markgoho/doula-cloud/issues/329) owns.

### 2026-08-22 — manual walk ([#236](https://github.com/markgoho/doula-cloud/issues/236))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x720 as Dee Whitlock, with separate contexts for Renata (Owner) and for the Client who has to sign. Preconditions built as the plan allows: `POST /api/staff/signup` for `Rooted Birth Collective`, then `POST .../invitations` for Priya Raman and for Dee, and `PATCH .../staff/{staffId}/roles` giving Priya `doula` so stage 5 has a target. This plan has no `automated` step, so nothing was skipped as already-run.

**On "run the plan twice".** Walking all 24 steps twice would prove DW-G1 no better than walking the *comparison* twice, so the second run is a fixed battery rather than the whole plan: the same 10 endpoints and the same 7 screens, captured once at `roles = '{}'` and again at `office_manager`, and diffed. The session's call, recorded here rather than asked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `Accept your Staff invite`, an email box, a password box, and an **Account mode** radio pair — `I'm new here -- create an account` / `I already have an account -- log in`. One button, **Accept invite** |
| 1.2 | `manual` | as expected | `200 POST /api/staff/accept-invite`, and `GET .../staff` then showed `{"name":"Dee Whitlock","roles":[]}`. Zero roles, as claimed |
| 1.3 | `manual` | **falsified** | There is nothing to choose. One membership, so the browser went straight to `/practices/{id}` and `Welcome to Rooted Birth Collective`; no picker rendered. Identical to Renata's 1.2 and unwalkable for anyone until [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) |
| 2.1 | `missing-feature (RA-G2)` [#261](https://github.com/markgoho/doula-cloud/issues/261) | as expected | Confirmed unwalkable. The Staff screen has no editable control; the roster is read-only |
| 2.1-a | `manual` | as expected | `PATCH .../staff/{staffId}/roles` with `["office_manager"]` -> `200`, and the roster then read `office_manager` — the raw enum, not **Admin** ([RA-G3](https://github.com/markgoho/doula-cloud/issues/262)). The staff id came from `GET .../staff`, which no screen prints |
| 3.1 | `manual` | **falsified** | **Three** tiles, not two: `Clients Billing Payments`. Payments sits outside `{#if roles.includes('owner')}` — [RA-G9](https://github.com/markgoho/doula-cloud/issues/267), which #235 found from Jo Mercer's zero-role membership and this walk confirms from Dee's |
| 3.1-a | `manual` | as expected — **and this is the strongest result on the plan** | The battery was run at `roles = '{}'` and again at `["office_manager"]` and the two are **byte-identical**: 10 endpoint responses the same status and the same body, 7 screens the same headings, links, buttons, inputs and text. `office_manager` changed nothing anywhere. [DW-G1](https://github.com/markgoho/doula-cloud/issues/269) is not an inference from reading the code; it is a measured null result |
| 3.2 | `manual` | as expected | `Credit balance: 3`, and a Date / Origin / Quantity ledger showing `signup_bonus +3`. A non-owner reads what the Practice spends ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272)) |
| 3.2-a | `manual` | as expected — **correct refusal** | `POST .../billing/purchases` -> `403 only a Practice Owner can do that`, though **Buy credits** and its Quantity box render for her first |
| 4.1 | `manual` | as expected | Two fields, `Their name` and `Their email`, and **Add Client**. Nothing about a due date, a referral, or the call ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252)) |
| 4.2 | `manual` | as expected, plus one fact | `201`, and it lands **straight on the Engagement page** — not back on the list — reading `Marisol Vega / Status intake / Created 8/22/2026`. Same landing Tasha's walk found |
| 5.1 | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. Every control on the page: `Send portal invite`, `Add a Visit`, `Create Care Plan`, `Create Birth Plan`, `Create Draft Contract`, `Send`. Nothing names a Doula |
| 5.2 | `manual` | **falsified** | The workaround does not exist. **Add a Visit** renders for Dee and answers `403 only a Staff member with the Doula role can do that`, in the page and on a direct `POST` alike — `visit/roles.go:41` requires `doula`, which she does not hold. New gap **[DW-G6](https://github.com/markgoho/doula-cloud/issues/274)** on the journey map, and the map's moment-of-truth argument is re-stated, because it rested on this step working |
| 6.1 | `manual` | as expected | `201` for a non-owner, and the screen prints `Invited. There is no email sending yet, so share this link with them directly:` with a raw `/portal/accept-invite?token=<uuid>` URL |
| 6.2 | `manual` | as expected | `201`, `Status: draft`, and six blank merge-field inputs — `Practice name`, `Client name`, `Scope of service`, two dates and `Price` (**[MO-G10](https://github.com/markgoho/doula-cloud/issues/258)**, Maya's) |
| 6.3 | `manual` | as expected | `PUT` then `POST .../contract/send` -> `200`, `Status: sent`. With the fields filled by hand the Client's portal read the prose resolved: `This agreement is between Rooted Birth Collective and Marisol Vega for doula services.` |
| 6.3-a | `manual` | as expected | The page renders the Practice's real template prose and the whole merge-field legend to Dee first; **Save** -> `403 only a Practice Owner can do that`. Read open, write shut — [PR-G4](https://github.com/markgoho/doula-cloud/issues/279)'s shape, and under ADR-0006 the read is correct |
| 7.1 | `manual` | as expected | After the Client signed (two-step: an electronic-signature disclosure, then full legal name plus a checkbox) the Engagement page read `Status: signed` and offered **Void Contract** |
| 7.1-a | `manual` | as expected | **Dee, a non-owner, voided a signed Contract**: `200`, `Status: voided`, `Voided — this Contract is no longer active.` Terminal as claimed — a second void `409`s and so does `send`. The prose and all six fields still render |
| 7.2 | `missing-feature (DW-G5)` [#273](https://github.com/markgoho/doula-cloud/issues/273) | as expected | Confirmed unwalkable. The Clients screen is two columns, `Name` and `Status`, and the Status is the *Engagement's* (`intake`), not the Contract's. No cross-Engagement view of signature state exists |
| 7.2-a | `manual` | **new** | The void does not close the till. **Create Invoice** and its Amount box keep rendering on the `voided` Contract, and `POST .../contract/invoices` answers `200 {"connectRequired":true}` — the Connect gate, not a refusal. The handler never reads the Contract's status, so with a connected account the next thing it meets is Stripe. New gap **[DW-G7](https://github.com/markgoho/doula-cloud/issues/275)** |
| 8.1 | `manual` (was `blocked`) | **re-marked — it clears** | Pre-connect: `200 {"connectRequired":true}`. The Owner then completed Stripe's hosted onboarding (see the addendum below), and the same call as Dee answered `201 {"connectRequired":false,"invoice":{…,"status":"open","amountCents":90000}}`. A non-owner Admin raised a $900 Invoice on a connected account |
| 8.2 | `manual` | as expected | `Ask a Practice Owner to connect Stripe.` — and the same sentence is on the **Payments** screen for a non-owner. [DW-G2](https://github.com/markgoho/doula-cloud/issues/270) is not confined to the Invoice |
| 9.1 | `missing-feature (DW-G3)` [#271](https://github.com/markgoho/doula-cloud/issues/271) | as expected | Confirmed unwalkable. No control on the Engagement page matches pay / paid / mark / record, and `api/main.go` has no route that writes a Payment. The only writer is the `invoice.paid` webhook |
| 10.1 | `manual` | as expected — **and correct** | Dee opened the Engagement and read both filled plans in full: the Care Plan's support people, pain management and backup-doula checkbox, and the Birth Plan's setting, people to notify and atmosphere. Per ADR-0006 an Admin reading both is right, not a leak |

**21 `manual` steps walked (8.1 among them, after the addendum below); 4 `missing-feature` steps confirmed unwalkable; no `blocked` step left on this plan.** Three expected results were falsified — 1.3, 3.1 and 5.2 — and one new step, 7.2-a, was added by the walk. Two gaps minted on the journey map (**DW-G6**, **DW-G7**), plus **DW-G8** below. No `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**One finding is not Dee's and is not any stage's: no screen in the product has a title.** `document.title` is `""` on `/login`, the practice landing, Clients, Billing and the Engagement page, so every browser tab is blank and SvelteKit's own live region announces `untitled page` to a screen reader at every client-side navigation. It surfaced here because it is on every screen this walk opened. Filed as **DW-G8** on Dee's map for want of a better owner; it belongs to all nine.

**Verdict against "a pass means": it does not pass, and it fails at both ends.** The middle holds and holds well — Dee creates the Client, sends the portal invite, builds, sends and voids the Contract, and reads both plans, all as a non-owner, refused only where the refusal is correct. What fails is the paperwork the journey is named for. There is **no Doula assigned**, and now no proxy for one either: the one act the map offered as a workaround is Doula-gated and refuses them (DW-G6). There is **no recorded Payment**, and no route that could write one (DW-G3).

Their moment of truth landed where the map put it, and the walk made the argument for it honest rather than merely plausible. Stage 5 lost the comparison for the wrong reason — the map credited Dee with a product workaround they do not have. Stage 9 still wins, on the ground that survives: a doula can be told by phone, and the work happens anyway; money that arrived by bank transfer cannot be written down anywhere, and the book stays open.

The role, meanwhile, is a null. Two runs of the same battery, one with no roles at all and one holding `office_manager`, came back identical in every byte. Whatever Dee is allowed to do, they are allowed to do it because they are Staff.

### 2026-08-22, later — Connect completed, and 8.1 cleared ([#236](https://github.com/markgoho/doula-cloud/issues/236))

The walk above left 8.1 `blocked` because Stripe's hosted onboarding served a CAPTCHA to the Playwright-launched browser. It was then driven again through the **user's own Chrome**, which Stripe did not challenge the same way, and it completed. Recipe kept at [connect-onboarding.md](connect-onboarding.md).

`acct_1U7Rwv1rKod8tdZe`, an unregistered US business, industry *Other personal services*, Stripe test bank, Radar Pro, Climate and Tax declined. On submission Stripe redirected to `?connect=return`; `status` went `onboarding_incomplete` -> `pending` -> **`active`**, with `cardPaymentsStatus` and `payoutsStatus` both `active` and no requirements outstanding. **The webhook wrote it**: the `capability_status_updated` thin event landed on `/api/stripe/account-webhook` and every delivery answered `200`.

Four things fell out of it.

**8.1 clears, and the practice side has no `blocked` step left.** As Dee — not an Owner — `POST .../contract/invoices` answered `201 {"connectRequired":false,…}` and created a `$900.00` Invoice at `open`. The endpoint never was owner-gated and the walk now proves it end to end rather than at a gate.

**DW-G7 is worse than the pre-connect walk could show.** The Contract was voided and the *same* call was made again: `201`, a second Invoice, `$500.00`, `open`. Stripe holds a real, finalized, payable invoice with a hosted payment URL against an agreement the Practice has already voided. Before Connect this only reached the `connectRequired` gate, so the gap read as a missing status check; it is in fact a bill a Client can pay for a Contract that no longer exists.

**An Invoice can be raised before the Practice can take payment.** The $900 one was created while `cardPaymentsStatus` was still `restricted` and the account was `pending`. The product's gate is *has a connected account*, not *can actually accept a card payment*, so a Client can be sent a bill during Stripe's review window. Recorded here rather than minted: whether that is wrong is a decision about what the gate should mean, and [#209](https://github.com/markgoho/doula-cloud/issues/209) is where it gets argued.

**The Practice name reaches the Client, and the Payments screen does not reach the Owner.** Both Stripe invoices carry `account_name: "Rooted Birth Collective"`, so the `display_name` fix from `7261a59` holds on a second, independent Practice — the `DOULA.CLOU` regression has not come back. The Payments screen is the opposite story: seconds after a successful submission it still read `Onboarding incomplete` / "Stripe still needs some details before Clients can pay you" and offered **Continue Stripe onboarding**, a dead end, while the API already said `pending` with zero requirements. It fetches once in `onMount` and never again (`settings/payments/+page.svelte:21`), so its own banner — "Status updates once Stripe confirms your account is active" — describes something the page never does. A manual reload showed `Awaiting Stripe review` / "Nothing is needed from you." New gap **MO-G11** on [Maya's map](../journeys/solo-birth-doula.md), which owns the Connect step.
