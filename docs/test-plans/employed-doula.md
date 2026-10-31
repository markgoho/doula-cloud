# Priya Raman — test plan

- **Journey**: [employed-doula.md](../journeys/employed-doula.md)
- **Persona**: [employed-doula.md](../personas/employed-doula.md)
- **A pass means**: an active membership holding the Doula role, an Engagement she is on opened, a Visit logged, messages exchanged — **and she never saw a screen she had no right to see.**

She is the negative-permission Persona, so half this plan is the [Permission boundary](#permission-boundary) matrix, which is tester work rather than anything Priya would do.

## Preconditions

- A Practice with an Owner, **two or more** Clients, at least one of them created by a different Staff member — otherwise stage 4 cannot fail the way it should.
- One Engagement carrying a filled Birth Plan and a Contract with an amount on it.
- Priya invited as a `doula` and accepted. The role rides the Invitation now (#316), so there is no follow-up call to make.
- A phone, or a phone-sized viewport, for stage 6. It is the moment of truth and it is device-specific.

## Steps

### Stage 1 — Accept the invitation on the phone

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link on the device the email arrived on | The accept form renders on a small viewport, nothing overflowing sideways | `manual` |
| 1.2 | Set email and password, press **Accept invite** | The accept screen is two steps: the account question with email and password, **Continue**, then first name, last name and work state, **Accept invite**. `POST /api/staff/accept-invite` creates the membership with the role the Invitation named (#316), and she lands on `Overview` | `automated (staff-invite-role.e2e.ts)` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). The accept form, then the Practice landing it signs her into | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `manual` |
| 1.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 1.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 2 — Receive the Doula role

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Have the Owner set `doula` from a screen | The Invitation already carried it (#316). On Priya's own page under **Staff**, **Change** beside Roles asks "Which roles does {name} have?" with Owner, Admin and Doula, and **Save** says "{name}'s roles are now Doula."; employment type has its own **Change** | `manual` |
| 2.1-b | Compare every later step with and without the role | **The without-role state cannot be produced.** A membership always holds a role: **Save** with every box cleared answers "Select at least one role", and the endpoint `400 at least one role is required` (`staffauth/membership.go`, `parseMembership`). What the Doula role and employment type gate is ADR-0008's write table, and the Permission boundary below walks its refusals ([PR-G3](https://github.com/markgoho/doula-cloud/issues/278), closed) | `manual` |

### Stage 3 — Sign in and choose the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Sign in at `/login` | `POST /api/session` succeeds and lands on `/practices/[practiceId]` | `automated (staff-login.e2e.ts)` |
| 3.2 | Choose Rooted Birth Collective | **There is nothing to choose.** One membership redirects straight to `/practices/[practiceId]`; no picker renders. Same as Renata's 1.2 and Dee's 1.3; the picker needs a second membership, which only Lena Vasquez's journey gives a Persona | `manual` |
| 3.3 | Read the tiles | The tiles are gone (#452). The top bar is Overview, Clients, Schedule, On call, Offers and Settings: no Invoices, Contracts, Credits or Staff. The Overview draws no Quick links ([#2398](https://github.com/markgoho/doula-cloud/issues/2398)), and no Credits or Getting paid card is offered to her ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267), [DW-G4](https://github.com/markgoho/doula-cloud/issues/272), closed) | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). `/login`, then the Practice landing; one membership needs no picker | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law) | `manual` |
| 3.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 4 — Find her Clients

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Every Client with work in the Practice**, including other doulas', 30 to a page with **Load more**; **See everyone** adds the Clients with none. Correct for an employee: [ADR-0008](../adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md) gives her every Engagement at the Practice ([PR-G1](https://github.com/markgoho/doula-cloud/issues/225), closed) | `manual` |
| 4.2-a | Look for a column saying which rows are hers | **The Doula column.** It names who is on each open Engagement, so her rows read "Priya Raman" and the others name their own Doula or "No Doula yet" ([RA-G15](https://github.com/markgoho/doula-cloud/issues/2232)). Each Engagement page names its Doula too ("Doula Priya Raman"), since an Attachment records it ([RA-G4](https://github.com/markgoho/doula-cloud/issues/225), closed) | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow) | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 5 — Open the Engagement

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open an Engagement that is **not** hers | It opens, with its Doula row, Visits, both plans, Contract, Invoices, Messages and Activity, and every act an employee may take on it. Correct under [ADR-0008](../adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md): an employed Doula reads and acts on every Engagement at the Practice | `manual` |
| 5.2 | Read the Contract and Invoices sections | The Contract's status, who signed and sent it and when, the **Fee**, **Read the Contract text** (the prose, in a disclosure), the signed PDF and **Request void**; the Invoices table with **Create Invoice**. Correct under ADR-0008, whose money row was amended to ✓ for an employed Doula ([#282](https://github.com/markgoho/doula-cloud/issues/282); [PR-G2](https://github.com/markgoho/doula-cloud/issues/277), closed) | `manual` |
| 5.2-a | Set the Price on a draft Contract, send it, and raise an Invoice | **Nobody sets the Price.** **Create a Draft Contract** fills the Fee from the Practice's rate for the Engagement's kind (#967), and a `PUT .../contract` naming a price answers `400 price is resolved automatically from the practice's rate card and cannot be set directly`. The rest is hers, as ADR-0008's write table says: she drafts and saves; **Send Contract** waits for the Client's portal invite (`409 this client has not been invited to the portal yet`); **Create Invoice** renders on a signed Contract and answers a draft `409 This Contract is still a draft` — the Contract's state, never her role. Once a Contract is voided, nothing on the page starts the next one ([#2260](https://github.com/markgoho/doula-cloud/issues/2260)) | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow) | `manual` |
| 5.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — Read the Birth Plan (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | On the phone, reach this Client's Birth Plan from a cold start, timed | `/` offers **Staff sign in**, then `/login`, the Overview, **Menu**, **Clients**, the Engagement ([PR-G9](https://github.com/markgoho/doula-cloud/issues/283), closed). The Birth Plan is a section far down the Engagement page, reached by its **On this page** link; the section shows progress and who last edited it, and its answers are on the Birth Plan page, through **Print for the hospital** | `manual` |
| 6.2 | Read the filled values | The Birth Plan page (`/engagements/[engagementId]/birth-plan`) renders the Plan Instance's answers, under "{first name}'s Birth Plan" and the lines that print with it: **Name** (the Client's legal name), **Doula** (who is on the Engagement, **Doulas** for more than one, and no row while nobody is), **Practice** (the Practice's name) and **Last changed** (the day an answer last changed; [HS-G9](https://github.com/markgoho/doula-cloud/issues/2209), [#2355](https://github.com/markgoho/doula-cloud/issues/2355)) | `manual` |
| 6.2-a | Deep-link straight to it, collapse what she does not need, or print it for hospital staff from her side | **All three, with one hole.** The Birth Plan has its own address (#955) and the Engagement page an anchor, `#birth-plan`; on its own page there is nothing else to collapse; **Print** and **Download Birth Plan (PDF)** are on her side. A saved address opened signed out is lost through sign-in ([PR-G12](https://github.com/markgoho/doula-cloud/issues/2259)) | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow). Measured from the Engagement page she opened at 5.1. The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) found the Birth Plan heading 0.83 screens down on a Pixel 7: one screen, scrolled | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 1 (Hick's Law, Miller's Law) | `manual` |
| 6.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold). The walk of [#237](https://github.com/markgoho/doula-cloud/issues/237) took about 1.7 s across 2 clicks from the Practice landing to the Birth Plan in view, reading included, and timed neither click alone; the budget holds each click, so the second walk times them one by one | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The Birth Plan names whose it is and when it last changed, so she knows she has the right one | `manual` |
| 6.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User). Measured from a cold start on the phone, the way 6.1 is timed | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — Log a Visit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Add a Visit | **Add a Visit** in the Visits section asks only "Scheduled date and time (optional)", with **Save Visit** and **Cancel**; "Your Visit is added." and a row: Date, Type, Doula (her), Notes, **Change** | `manual` |
| 7.1-a | Record when the Visit was | The date and time are asked at **Add a Visit** and printed on the row ([MO-G1](https://github.com/markgoho/doula-cloud/issues/250), closed) | `manual` |
| 7.1-b | Record which of the three kinds it was — prenatal, birth, postpartum | Not chosen: the row's Type is worked out from when the Visit falls against the pregnancy-end date (#954), so a Visit before the due date reads Prenatal ([PR-G6](https://github.com/markgoho/doula-cloud/issues/281), closed) | `manual` |
| 7.1-c | Record what was covered, for "what was I told last time" | Notes are Staff-only and written through the row's **Change**, after the Visit is added; the row prints them (#888; [MO-G2](https://github.com/markgoho/doula-cloud/issues/251), closed) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). When the Visit was, which kind, what was covered, and the save | `manual` |
| 7.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 7.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 8 — Message the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Send a message | `POST .../messages` appends to the one Engagement thread | `manual` |
| 8.2 | Have the Client reply | One continuous thread, in order, immutable | `manual` |
| 8.2-a | With the Client's thread open, deliver a push event | The tab refetches and shows the message; the push itself carries no content (ADR-0002) | `automated (push-notification.e2e.ts)` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). Message, Attachment, **Send** and **Cancel**, after **Write a message** | `manual` |
| 8.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**Nadia crossing, settled.** Stages 7 and 8 are where an Engagement ending in loss lands on the Doula's side — a bereavement Visit, and a thread that must not carry on unchanged. [Her plan](loss-client.md) now exists and **neither stage changes**: her 7.1 walks the bereavement Visit against **PR-G6**, **MO-G1** and **MO-G2**, already given steps here, and the thread with no way to mark that its subject has changed is hers to own (**NH-G7**). No step id here moves.

## Permission boundary

**Not stages.** Priya would never type these URLs. Each is walked as Priya, signed in, holding `doula` and not `owner`. No owner-only route has a `+page.ts` load guard, so the page always renders; what differs is whether the API hands over data (PR-G4).

| Step | Route | Expected result | Mark |
| --- | --- | --- | --- |
| PR-B1 | `/practices/[practiceId]/staff` | **Holds.** The heading renders, then an error instead of the roster — the `GET` requires Owner or Admin (ADR-0008) | `manual` |
| PR-B2 | `/practices/[practiceId]/invite` | **Holds on submit.** The form renders; `POST .../invitations` refuses | `automated (staff-invite-role.e2e.ts)` |
| PR-B3 | `/practices/[practiceId]/settings/plan-templates` | **Reads through.** `GET` is ungated, `PUT` is Owner. Under ADR-0006 Templates are open to every Staff role, so this is a correct read and a refusal only at **Save** | `manual` |
| PR-B4 | `/practices/[practiceId]/settings/contract-template` | Same shape, same verdict as PR-B3 | `manual` |
| PR-B5 | `/practices/[practiceId]/settings/payments` | **Holds, at the endpoint.** **Getting paid** shows her the billing mode and payment terms, which ADR-0008 gives every Staff member, then "Only a Practice Owner or Admin can see how this Practice gets paid." in place of the Stripe status, and no Connect control. `GET .../payments/connect` answers her `403`, and `POST` too ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267), closed) | `manual` |
| PR-B6 | `/practices/[practiceId]/billing` | **Holds.** The balance and ledger are Owner-and-Admin ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272), closed): `GET .../billing` answers `403`, and the screen is "You cannot view this", with **Go to your Practice overview** | `manual` |

The pattern under test was that protection sat on the write endpoint, never on the read. The second walk (2026-10-28) found it gone: the roster, Credits and the Stripe status are refused on the read, and the Templates, which ADR-0008 opens to every Staff role, are refused only at **Save**. Only PR-B2's refusal is asserted by a spec.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 11 (7 of them [budget steps](README.md#budget-steps)) |
| `manual` | 50 (five of them the permission boundary; 5.2-a added by the walk; 25 of them [budget steps](README.md#budget-steps)) |
| `missing-feature` | 0 (RA-G4, PR-G5, PR-G6, MO-G1 and MO-G2 are built, and their steps were walked on 2026-10-28) |

PR-G1, PR-G2, PR-G3, PR-G4 and PR-G8 are observed inside walkable steps (4.2, 5.2, 5.2-a, 2.1-b, PR-B3 to PR-B6) rather than given steps of their own: the step can be performed, and what it hands back is the finding. All five are closed, and [ADR-0008](../adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md) now rules most of what they named correct for an employed Doula. **PR-G9** is observed at 6.1, in what happens *before* the step rather than in the step itself.

## Run log

### 2026-10-28 — second manual walk ([#2176](https://github.com/markgoho/doula-cloud/issues/2176))

**Stack**: the shared walk stack, `bun run --cwd app dev:full` from the main checkout at port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`, on trunk (this record written at `35395d5c`), in WSL2 with rootless Podman. `scripts/stripe-listen.sh` was running against it with the Stripe test-mode key; this walk made no Stripe call. Not restarted.

**Devices**: headless Chrome inside WSL, driven by the chrome-devtools MCP, in UTC. Stages 1 and 6 on an emulated Pixel 7 (412x839, device pixel ratio 2.625, mobile and touch, a Pixel 7 user agent), everything else at 1280x900. No CPU or network throttling: the viewport is emulated, the processor is not. One isolated browser context per person: Renata Alvarez (Owner), the Stage 1 invitee, the seeded Priya Raman (one desktop context, one phone context, and one new phone context for 6.B-cold), and Camille Boyd in the portal for 8.2. Every timing is on an unbundled Vite dev server, which compiles a route the first time it is visited; each timed act was repeated warm where it could be, and a first load is marked as one.

**Accounts and preconditions**, all on the seeded Rooted Birth Collective (`19a0107e-…`), as Dee's walk ([#2175](https://github.com/markgoho/doula-cloud/issues/2175)) left it: billing by card and connected to Stripe, 17 Staff, Ines Calloway with Priya as her Doula, a signed Contract and an open Stripe Invoice.

- **Stage 1**: Renata invited `priya.walk@sim.doula.cloud` from **Invite a Staff member** with **Doula** and **Employee**; the mail was opened from the catcher in the phone context and accepted as "Priya Ramanathan" (a different family name from the seeded Priya, so the roster can tell them apart), New York. Her password is a generated local-emulator value, kept out of this document. 2.1 was walked as Renata on her membership.
- **Stages 3 to 8 and the Permission boundary**: the seeded `priya.raman@sim.doula.cloud` (Doula, Employee), with the seed password ([walk-a-screen-locally.md](../testing/walk-a-screen-locally.md)). Her book: Camille Boyd (signed Contract, open `INV-0001`, a filled Birth Plan and Care Plan, Visits, a thread with an unanswered message), Tessa Moreau (Contract voided by Dee), Imani Brooks, Grace Whitfield, Mira Castellanos (a recorded loss) and Ines Calloway.
- **"Created by a different Staff member"**: the seed saved every Client as Renata, and eight other Doulas have Clients. 5.1 opened Avery Falk's (Kimiko Nakashima's and Joan Walker's); 5.2-a drafted on Morgan Grier's (Marisol Terrazas's).
- **Stage 6**: Camille Boyd's Birth Plan ("2 of 4 questions answered", by Priya).
- **Stage 7**: a Visit on Ines Calloway's Engagement, so that Camille's stays as her walk ([#2178](https://github.com/markgoho/doula-cloud/issues/2178)) expects.
- **Stage 8**: Camille's thread. Camille signed in by a magic link from the catcher in her own context and replied.
- **The first run's bypasses are all gone**: no `POST /api/staff/signup`, no API invitation, no `PATCH .../staff/{staffId}/roles`.

The plan's `automated` steps were **not** re-run as specs; 1.2 was walked anyway, since the stage opens on it.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | The catcher's mail, "You've been invited to join a practice on DoulaCloud", names neither the Practice nor who invited her (by decision: ADR-0009 and ADR-0011, recorded on [#339](https://github.com/markgoho/doula-cloud/issues/339)). Its link opened "Accept your Staff invite" at 412px, `scrollWidth` 412: "Do you already have a DoulaCloud account?", two radios, **Continue**. The screen names no Practice either |
| 1.2 | `automated (staff-invite-role.e2e.ts)` (was `manual`) | cell corrected | "I'm new here" revealed Email address (not filled from the invite) and Password; **Continue**; "Tell us about yourself", First name, Last name, "Which state do you work from?", **Accept invite**; the Overview. Her roster row: "Doula", "Employee"; her Membership history: "Joined as Doula (Employee)". The spec asserts the same landing, at desktop size |
| 2.1 | `manual` | as expected, cell corrected | Renata, **Staff**, Priya Ramanathan, **Change** beside Roles: "Which roles does Priya Ramanathan have?", Doula already checked by the Invitation. **Save**: "Priya Ramanathan's roles are now Doula." Employment type has its own **Change** |
| 2.1-b | `manual` | **falsified**, cell corrected | No battery was run: the without-role state cannot be made. Clearing Doula and pressing **Save** answered "There is a problem", "Select at least one role", and the handler refuses an empty set (`400 at least one role is required`). Her roles stayed Doula |
| 3.1 | `automated (staff-login.e2e.ts)` | read on the walk | `/login`, **Sign in**, the Overview |
| 3.2 | `manual` | as expected | One membership, so no picker |
| 3.3 | `manual` | **falsified** (the tiles are gone), cell corrected | Top bar: Overview, Clients, Schedule, On call, Offers, Settings. Quick links: Clients "23 active", Your offers "None waiting on you", Plan Templates, Contract Template. Then "Offers awaiting your answer", "Clients waiting on a reply" (Camille Boyd), and Recent activity, which shows her the Practice's money entries ("Invoice created for Ines Calloway", "Payment from Imani Brooks recorded"), as ADR-0008's money-in-the-ledger row allows |
| 4.1 | `manual` | as expected | **Clients** renders for her |
| 4.2 | `manual` | as expected, cell corrected | "Showing 30 Clients with work so far. More remain.", every Doula's, with **Load more** and **See everyone** off. Correct under ADR-0008 |
| 4.2-a | `manual` (was `missing-feature (RA-G4)`) | **walked** | The list's columns are Name and Status (Birth/Postpartum, phase, Contract, money); nothing marks hers. Each Engagement says "Doula Priya Raman" or names another. [RA-G15](https://github.com/markgoho/doula-cloud/issues/2232), cited |
| 5.1 | `manual` | as expected, cell corrected | Avery Falk's Engagement ("Doulas Kimiko Nakashima, Joan Walker") opened from the Clients list, with **Change** on care, due date, phase and Doula, **Create a Draft Contract**, **Add a Visit**, and both plans' **Start** |
| 5.2 | `manual` | as expected, cell corrected | On Camille Boyd's: "Signed on Oct 27, 2026", "Signed by Camille Boyd", "Sent Oct 27, 2026 by Renata Alvarez", "Fee $1,500.00", **Read the Contract text** (the prose), **Download signed Contract (PDF)**, **Request void** (not **Void**); Invoices `INV-0001`, $1,500.00, Open, and **Create Invoice** |
| 5.2-a | `manual` | **falsified in part**, cell corrected | On Morgan Grier's Engagement (Marisol Terrazas's Client), **Create a Draft Contract**: Practice name and Client name filled, no price field; **Save Contract**: "Contract saved.", "Draft, not sent yet", "Fee $1,500.00". A `PUT .../contract` with `price` `$99.00` answered `400 price is resolved automatically from the practice's rate card and cannot be set directly`. `POST .../contract/send` `409 this client has not been invited to the portal yet` (she has no email); `POST .../contract/invoices` `409 This Contract is still a draft, so you cannot create an Invoice for it.` The draft was left unsent. On Tessa Moreau's voided Contract the Invoices section says invoicing waits for a new Contract, and nothing starts one ([#2260](https://github.com/markgoho/doula-cloud/issues/2260)) |
| 6.1 | `manual` | **falsified** (she arrives now), cell corrected | From a fresh phone context at `/`: "Sign in or set up a Practice", **Staff sign in**, **Client portal sign in**, **Set up a Practice**. Then `/login`, the Overview, **Menu**, **Clients** (the Menu stays open over the page, [#2235](https://github.com/markgoho/doula-cloud/issues/2235)), **Close menu**, Camille's Birth row. The page is **7955 CSS px** tall and the **Birth Plan** heading sits at y=4080, **4.9 screens** down; **On this page** starts at y=1100, below the first screen. Its **Birth Plan** link jumps there in 18 ms, to "2 of 4 questions answered", "Last edited Oct 27, 2026 by Priya Raman", "Camille has not read this yet", **Change the Birth Plan**, **Print for the hospital** |
| 6.2 | `manual` | as expected, cell corrected | **Print for the hospital** opened "Camille's Birth Plan", "Engagement created Oct 27, 2026", **Print**, **Download Birth Plan (PDF)**, then Planned birth location "Hospital", People to notify "—", atmosphere "Lights down low, my own playlist on quietly, and Marcus right beside me the whole time.", photos "No". The page is 969 px, 1.15 screens |
| 6.2-a | `manual` (was `missing-feature (PR-G5)`) | **walked** | Deep link: `#birth-plan`, and the Birth Plan's own address. Collapse: the Birth Plan page holds nothing else; the Engagement page's one `<details>` is **Read the Contract text**. Print: **Print** and the PDF. The saved address, opened signed out on a new phone, went to `/login?sessionEnded=true` ("For your security, we signed you out.") and, after **Sign in**, to the Overview ([PR-G12](https://github.com/markgoho/doula-cloud/issues/2259)) |
| 7.1 | `manual` | as expected, cell corrected | On Ines Calloway's Engagement, **Add a Visit**: "Scheduled date and time (optional)", **Save Visit**, **Cancel**. Oct 28, 2026, 10:00am, **Save Visit**: "Your Visit is added." and a row "Oct 28, 2026, 10:00am", Prenatal, Priya Raman, "No notes yet.", **Change**. Rows sort by date, newest first |
| 7.1-a | `manual` (was `missing-feature (MO-G1)`) | **walked** | The date and time are asked and printed |
| 7.1-b | `manual` (was `missing-feature (PR-G6)`) | **walked** | Type "Prenatal", worked out, not asked; she cannot choose it |
| 7.1-c | `manual` (was `missing-feature (MO-G2)`) | **walked** | **Change** on the row: "Change the Visit for Priya Raman on Oct 28, 2026, 10:00am", the date, Notes, **Save changes**, **Cancel**. Notes "Walked through her birth preferences and the hospital bag. Told her to call me if contractions are five minutes apart."; "The Visit for Priya Raman is saved." and the notes on the row. They cannot be written at **Add a Visit** |
| 8.1 | `manual` | as expected | Camille's **Write a message**, Message, Attachment, **Send**, **Cancel**; "Yes, Thursday at 3pm works. I will see you then.", **Send**: "Message sent." and the message last in the thread, "Priya Raman, Oct 28, 2026, 9:44pm" |
| 8.2 | `manual` | as expected | Camille, in the portal: "Your care" quoted Priya's message under Messages; she replied "Thursday at 3pm is perfect, thank you Priya." On Priya's open page the reply appeared on a reload, in order, labeled "Camille"; no edit or delete control. The headless browser holds no push subscription, which 8.2-a's spec covers |
| 8.2-a | `automated (push-notification.e2e.ts)` | not re-run | |
| PR-B1 | `manual` | as expected, cell corrected | "Staff", a link **Invite a Staff member**, and `only a Practice Owner or Admin can do that` in place of the roster; `GET .../staff` `403`. The API's words, and a link to an act she is refused ([#2261](https://github.com/markgoho/doula-cloud/issues/2261)) |
| PR-B2 | `automated (staff-invite-role.e2e.ts)` | read on the walk | The form renders; **Send invite** answers "There is a problem", `only a Practice Owner can do that` ([#2261](https://github.com/markgoho/doula-cloud/issues/2261)) |
| PR-B3 | `manual` | as expected | Every Birth Plan and Care Plan question with **Move up**, **Move down**, **Remove**, six **Add** buttons and **Save**, and no word that she cannot save; **Save** answered `only a Practice Owner can do that` ([#2244](https://github.com/markgoho/doula-cloud/issues/2244), cited; [#2261](https://github.com/markgoho/doula-cloud/issues/2261)) |
| PR-B4 | `manual` | as expected | "Only a Practice Owner can change these terms.", the prose in an editable field, no **Save** ([#2244](https://github.com/markgoho/doula-cloud/issues/2244), cited) |
| PR-B5 | `manual` | **holds, at the read now**, cell corrected | "Getting paid", "Billing mode By card, through Stripe", "Payment terms Due 30 days after an invoice is raised.", "Only a Practice Owner or Admin can see how this Practice gets paid."; no Connect control. `GET .../payments/connect` `403 only a Practice Owner or Admin can do that`; `POST` `403 only a Practice Owner can do that` |
| PR-B6 | `manual` | **holds** (was fails), cell corrected | "You cannot view this", "Your role does not have permission to see this. Trying again will not change that.", **Go to your Practice overview**; `GET .../billing` `403` |

**Re-marked**: 4.2-a, 6.2-a, 7.1-a, 7.1-b and 7.1-c from `missing-feature` to `manual`, their gaps built. 1.2 from `manual` to `automated (staff-invite-role.e2e.ts)`, its cell corrected. 2.1-b, 3.3, 5.2-a and 6.1 were **falsified** and corrected; 2.1, 4.2, 5.1, 5.2, 6.2, 7.1, PR-B1, PR-B5 and PR-B6 are corrected; their marks hold. The Marks summary is recounted to 11 / 50 / 0 / 0, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted**: this plan's budget notes name the decisions only on stage 7 ("when, which kind, what was covered, and the save") and say nothing on groups, so the rule of the earlier second-run walks is used, as on Renata Alvarez's and Dee Whitlock's: fields, options, and links or buttons that move her somewhere or change what happens, enabled and in view at once in the page body. A radio group, a checkbox group, a select and a date-time field are one decision each; **Cancel** counts. **Back** links, the skip link, **Show password**, `/login`'s secondary links (as Renata's walk counted `/login` 3) and the Staff top bar are not counted. **Time** is measured in the page: a script presses one control and polls each animation frame until the result has settled on screen, and records when the control showed it was busy, or for a move when the page body first changed. Stage 6 was timed one press per script, as the run sheet asked.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 2 | **3**: the account question, "Tell us about yourself", the Overview | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), cited |
| 1.B-decisions | 3 | **4** on each accept screen (the account radio group, Email address, Password, **Continue**; First name, Last name, work state, **Accept invite**); the Overview's body 6 | **no**, [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), cited |
| 1.B-confirmation | yes | **no**. The Overview names the Practice in the top bar and says nothing about joining or the role | **no**, [DW-G10 (#2239)](https://github.com/markgoho/doula-cloud/issues/2239), cited |
| 1.B-cold | yes | yes, in a fresh phone context. Each question says what it is for | yes |
| 3.B-screens | 2 | 2: `/login`, the Overview | yes |
| 3.B-decisions | 3 | **6** in the Overview's body: four Quick links, Camille Boyd (waiting on a reply), **See all activity**. `/login` 3 | **no**, [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), cited |
| 3.B-time | 400 ms, 100 ms | **Sign in** 478 ms and 490 ms in two fresh desktop contexts, busy within 10 ms; on the phone, 1014 ms (stage 6). Top-bar moves 86 to 152 ms | **no**, [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), cited |
| 4.B-screens | 2 | 2: the Overview, Clients | yes |
| 4.B-time | 400 ms, 100 ms | **Clients** from the top bar 113 to 152 ms over four warm presses, the body changing within 63 to 86 ms | yes |
| 5.B-screens | 2 | 2: Clients, the Engagement | yes |
| 5.B-time | 400 ms, 100 ms | The Engagement from its Clients row, Activity included, 362 to 385 ms over four warm presses, acknowledged in 41 to 45 ms | yes |
| 6.B-screens | 1 | **2** from the Engagement page she opened at 5.1: the Engagement (the Birth Plan section holds no answers), the Birth Plan page. From `/` on a new phone: 6 | **no**, [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), cited |
| 6.B-decisions | 1 | **4** on the Engagement page's first phone screen (**Message the Client**, three **Change**), none of them the Birth Plan; the Birth Plan page **2** (**Print**, **Download Birth Plan (PDF)**) | **no**, [PR-G10 (#2257)](https://github.com/markgoho/doula-cloud/issues/2257) |
| 6.B-memory | yes | yes. Her Client's name is on the Clients list and in the Birth Plan page's heading; nothing else is carried | yes |
| 6.B-time | 400 ms, 100 ms | One press at a time, on the phone: **Staff sign in** 45 ms; **Sign in** **1014 ms**, busy within 8 ms; **Menu** 6 ms; **Clients** 176 ms (acknowledged 88 ms); Camille's Engagement, warm: header 61 to 65 ms, Birth Plan section 216 to 238 ms, whole page **501 to 784 ms**, acknowledged 55 to 99 ms (181 ms on the first press); **Birth Plan** in **On this page** 18 ms; **Print for the hospital** 80 to 125 ms, acknowledged 46 to 86 ms | **no**, [PR-G11 (#2258)](https://github.com/markgoho/doula-cloud/issues/2258); the sign-in [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), cited |
| 6.B-confirmation | yes | **no**. The Birth Plan page names whose it is ("Camille's Birth Plan", first name only) and "Engagement created Oct 27, 2026", not when the plan last changed; "Last edited" is on the Engagement's section, a screen back | **no**, [HS-G9 (#2209)](https://github.com/markgoho/doula-cloud/issues/2209), cited (the Staff page shares the portal's `BirthPlanView`) |
| 6.B-cold | yes | **no**. From `/` nothing needs explaining up to the section, but the answers sit behind **Print for the hospital**, which does not say it is the way to read them; and a saved Birth Plan address, opened signed out, tells a new device "we signed you out" and drops her on the Overview after sign-in | **no**, [PR-G12 (#2259)](https://github.com/markgoho/doula-cloud/issues/2259); [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), cited |
| 7.B-screens | 1 | 1: the Engagement page | yes |
| 7.B-decisions | 4 | 3 on **Add a Visit** (the date and time, **Save Visit**, **Cancel**), 4 on the row's **Change** (the date and time, Notes, **Save changes**, **Cancel**). Which kind is not asked | yes |
| 7.B-time | 400 ms, 100 ms | **Add a Visit** 12 ms; **Save Visit** 52 ms; **Save changes** 55 ms; each busy within 2 ms | yes |
| 7.B-confirmation | yes | yes. "Your Visit is added." and "The Visit for Priya Raman is saved.", with the row showing the date, type and notes | yes |
| 8.B-screens | 1 | 1: the Engagement page | yes |
| 8.B-decisions | 2 | **4**: Message, Attachment, **Send**, **Cancel**, after **Write a message** | **no**, [MO-G16 (#2197)](https://github.com/markgoho/doula-cloud/issues/2197), cited |
| 8.B-time | 400 ms, 100 ms | **Write a message** 9 ms; **Send** 50 ms, busy within 2 ms | yes |
| 8.B-confirmation | yes | yes. "Message sent." and the message last in the thread with her name and the time | yes |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` and were not re-measured; the phone stages had no sideways scroll at 412px. **Failed**: 1.B-screens, 1.B-decisions, 1.B-confirmation, 3.B-decisions, 3.B-time, 6.B-screens, 6.B-decisions, 6.B-time, 6.B-confirmation, 6.B-cold, 8.B-decisions.

#### Moment of truth — the verdict, against the first run's

**Stage 6, the Birth Plan, on a phone, in a hospital corridor.** First run ([#237](https://github.com/markgoho/doula-cloud/issues/237), 2026-08-23): it failed at arriving. `/` was the SvelteKit scaffold and the Engagement page had no links, so only a remembered URL reached it; once there the Birth Plan was 0.83 screens down and read correctly in about 1.7 s from the Practice landing, with no deep link, no collapse and no print. **This run: the arrival is fixed and the corridor still fails, on its budgets.** From a cold phone she now finds her way unaided: `/` offers **Staff sign in**, the **Menu** reaches Clients, the Engagement has **On this page**, and the Birth Plan has its own page with **Print** and a PDF. But on a Pixel 7 the Birth Plan section is 4.9 screens down a 7955 px page whose jump links start below the first screen, the section holds no answers, and the answers are one more screen on, behind a control named for printing. From the Practice landing that is four presses and about 0.9 s of product time (**Menu**, **Clients**, the Engagement, **Print for the hospital**), against the first run's two clicks and 1.7 s with reading; from `/` it is seven presses, with sign-in alone 1.0 s on the phone. The page she reads says whose plan it is by first name and not when it last changed. And the shortcut the new address offers is lost if she is signed out: sign-in drops her on the Overview. She gets there in seconds now, not only by memory; she does not get there in one screen, one decision, or under 400 ms a press.

#### Is the journey completable end to end?

**Yes.** Against "a pass means": an active membership holding the Doula role (Priya Ramanathan joined through an emailed Invitation as Doula, Employee; the seeded Priya holds the same), an Engagement she is on opened (Camille Boyd's and Ines Calloway's both say "Doula Priya Raman"), a Visit logged (Ines Calloway, Oct 28, 2026, 10:00am, with notes), and messages exchanged (Camille, both ways, in one thread). **And she never saw a screen she had no right to see**, read against [ADR-0008](../adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md), which supersedes the ADR-0006 table the first run used: the roster, Credits and the Stripe status are refused on the read, the top bar and Quick links do not offer them, and what she does read (every Engagement, a Contract's money, Invoices, the Templates, the billing mode) is hers to read. Two things fall short of the letter: refused screens answer in the API's words and the Staff screen still links to the invite form (#2261), and the Plan Templates screen hands her a full editor refused only at **Save** (#2244). No step stops her.

**Findings filed**: [PR-G10 (#2257)](https://github.com/markgoho/doula-cloud/issues/2257), [PR-G11 (#2258)](https://github.com/markgoho/doula-cloud/issues/2258) and [PR-G12 (#2259)](https://github.com/markgoho/doula-cloud/issues/2259), each on the [journey map](../journeys/employed-doula.md#gaps-found) with its budget line quoted; and two bugs, [#2260](https://github.com/markgoho/doula-cloud/issues/2260) (no way to start a Contract after a void) and [#2261](https://github.com/markgoho/doula-cloud/issues/2261) (refusals in the API's words). Cited, with a comment on each: [DW-G9 (#2238)](https://github.com/markgoho/doula-cloud/issues/2238), [DW-G10 (#2239)](https://github.com/markgoho/doula-cloud/issues/2239), [DW-G14 (#2243)](https://github.com/markgoho/doula-cloud/issues/2243), [RA-G11 (#2228)](https://github.com/markgoho/doula-cloud/issues/2228), [RA-G15 (#2232)](https://github.com/markgoho/doula-cloud/issues/2232), [RA-G16 (#2233)](https://github.com/markgoho/doula-cloud/issues/2233), [HS-G9 (#2209)](https://github.com/markgoho/doula-cloud/issues/2209), [MO-G16 (#2197)](https://github.com/markgoho/doula-cloud/issues/2197), [#2235](https://github.com/markgoho/doula-cloud/issues/2235) and [#2244](https://github.com/markgoho/doula-cloud/issues/2244).

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 3, 4, 5, 6, 7, 8 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **32 budget steps**, 7 `automated`, 25 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 3.1 | `staff-login.e2e.ts` | pass |
| 8.2-a | `push-notification.e2e.ts` | pass |

**2 automated steps: all pass.**

### 2026-09-10 — Marks summary recounted ([#685](https://github.com/markgoho/doula-cloud/issues/685))

No step is re-marked and no cell is rewritten here. The `missing-feature` count above read 6 and listed RA-G2, which no step in the Steps table carries any more — 2.1's roster question was answered by **Edit membership**, and the summary kept counting it. Recounted to 5, and [docs/test-plans/README.md](README.md)'s run-status row with it.

### 2026-09-03 — new automated step ([#318](https://github.com/markgoho/doula-cloud/issues/318))

`bun run test:e2e` in `app/`, whole suite, one run: **30 passed, 0 failed** (29.0s).

| Step | Spec | Result |
| --- | --- | --- |
| PR-B2 | `staff-invite-role.e2e.ts` | pass |

**1 newly automated step: pass**, bringing the plan's total to 3. `staff-invite-role.e2e.ts` also drives Stage 1/2's invite-and-accept shape, but with the Invitation carrying the `doula` role directly (#316) rather than the empty-then-edited-membership shape Stage 1.2 describes, so those two steps are left `manual` rather than reconciled here. The 2026-08-23 manual walk below is unchanged as a historical record.

### 2026-08-23 — manual walk ([#237](https://github.com/markgoho/doula-cloud/issues/237))

`bun run dev:full` in `app/`, walked as Priya Raman: stages 1 and 6 on an emulated Pixel 7 (412x839), everything else at 1280x900, with separate contexts for Renata (Owner) and for Marisol Vega (the Client who replies). This plan's two `automated` steps were **not** re-run.

Preconditions built as the plan allows, and no further: `POST /api/staff/signup` for `Rooted Birth Collective` (Renata, whom signup grants all three roles), the Client **Marisol Vega** created by Renata and the Client **Tabitha Nunes** created by a second Staff member, **Jo Mercer**, so stage 4 can fail the way it should; a filled Care Plan and Birth Plan and a `sent` Contract priced `$1,800` on Marisol's Engagement; and her portal invite accepted, so stage 8 has a reply side. Priya's own invitation was only *sent* — accepting it **is** steps 1.1 and 1.2, and the `PATCH .../staff/{staffId}/roles` **is** step 2.1-a, so neither was provisioned away.

**On "compare every later step with and without the role" (2.1-b).** Walking all 28 steps twice would say nothing 2.1-b does not, so the second pass is a fixed battery rather than the whole plan: **21 endpoints**, captured once at `roles = '{}'` — the state acceptance actually leaves her in — and again at `['doula']`, then diffed with ids and timestamps normalized away. The session's call, recorded here rather than asked, following Dee's precedent.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | On a 412px viewport: `Accept your Staff invite`, an Email box, a Password box, an **Account mode** radio pair, and one **Accept invite** button. `scrollWidth` equals `clientWidth`, so nothing overflows sideways. The tab title is empty ([DW-G8](https://github.com/markgoho/doula-cloud/issues/276)) |
| 1.2 | `manual` | as expected, plus one fact | `200 POST /api/staff/accept-invite`, and `GET .../staff` then read `{"name":"Priya Raman","roles":[]}` — zero roles, as claimed. It also lands her **straight on** `/practices/{id}`, greeting her `Welcome to Rooted Birth Collective` with three tiles, which pre-empts 3.2 below |
| 2.1 | `missing-feature (RA-G2)` [#261](https://github.com/markgoho/doula-cloud/issues/261) | as expected | Confirmed unwalkable from the Owner's side: the Staff screen prints a **Roles** column (`no roles yet`, `doula`, `owner, office_manager, doula` — the raw enum, [RA-G3](https://github.com/markgoho/doula-cloud/issues/262)) and the only control under **Actions** is **End sessions everywhere**. Nothing sets a role |
| 2.1-a | `manual` | as expected | `PATCH .../staff/{staffId}/roles` with `["doula"]` -> `200 {"roles":["doula"]}`. The staff id came from `GET .../staff`, which no screen prints — the same out-of-band start Dee's 2.1-a and Renata's 3.3-a have |
| 2.1-b | `manual` | **falsified** | The battery diff is **two lines, one of them behavior**: `GET .../session` echoes `roles:["doula"]`, and `POST .../visits` goes `403 only a Staff member with the Doula role can do that` -> `201`. Everything else — the Clients list, the Engagement, both plans, the Contract, Invoices, Billing, both Templates, Payments, and every refusal — is byte-identical. The role is read in **one package**: `visit/roles.go:40` for the caller and `:57` for a reassignment target (a target without `doula` is refused `400 staff member does not hold the Doula role at this practice`). No front-end file reads it. So `doula` is **not** decorative — it gates the Visit, the one act this journey is named for — and it gates nothing else. [PR-G3](https://github.com/markgoho/doula-cloud/issues/278) narrowed on the map, not deleted |
| 3.1 | `automated (staff-login.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| 3.2 | `manual` | **falsified** | There is nothing to choose. Signing in at `/login` went straight to `/practices/{id}`; no picker rendered. One membership, and [LV-G2](https://github.com/markgoho/doula-cloud/issues/225) makes a second unreachable, so this step is unwalkable for anyone. Third plan to carry the same false claim, after Renata's 1.2 and Dee's 1.3 |
| 3.3 | `manual` | as expected | Exactly three links: `Clients`, `Billing`, `Payments`. Invite, Staff, Plan Templates and Contract Template are gone. Payments is outside the owner block — [RA-G9](https://github.com/markgoho/doula-cloud/issues/267), now confirmed from a third membership |
| 4.1 | `manual` | as expected | The list renders for a non-owner, `200 GET .../clients` |
| 4.2 | `manual` | as expected | **Two rows: `Marisol Vega` and `Tabitha Nunes`** — she reads the Client Jo Mercer created, which is not hers and which she has no Engagement with. [PR-G1](https://github.com/markgoho/doula-cloud/issues/225) confirmed against a Practice built to test it |
| 4.2-a | `missing-feature (RA-G4)` [#225](https://github.com/markgoho/doula-cloud/issues/225) | as expected | Confirmed unwalkable. The table is two columns, `Name` and `Status`. Nothing names a Doula, because no Engagement carries one |
| 5.1 | `manual` | as expected | Marisol's Engagement — Renata's Client, not Priya's — opened on the first try. Seven `GET`s fired, all `200`: the Engagement, Visits, Messages, both plans, the Contract and the Invoices. No read path is role-checked |
| 5.2 | `manual` | **falsified in part** | The money is all there: `Price` reads `$1,800`, and `Scope of service`, both dates and both names are filled; the Invoices section reads `No Invoices yet.` beside an Amount box and **Create Invoice**. [PR-G2](https://github.com/markgoho/doula-cloud/issues/277) confirmed. What does **not** render is the **prose** — the Engagement page shows `Status: sent` and the six merge-field inputs only. The cell claimed prose; the prose is on the Contract Template screen and in the Client's portal |
| 5.2-a | `manual` | **new — and the worst result on this plan** | Priya, holding `doula` and nothing else, opened Tabitha's draft Contract, set `price` to `$99` and `scope_of_service` to `Priya wrote this` (`PUT .../contract` -> `200`), then **sent it to the Client** (`POST .../contract/send` -> `200 {"status":"sent"}`). `POST .../contract/invoices` answered `200 {"connectRequired":true}` — the Stripe gate, not a role refusal — so on a connected Practice she raises the Invoice too. The only refusals a Contract makes are about its own status (`409 contract is no longer a draft`, `409 contract is not signed`). New gap **[PR-G8](https://github.com/markgoho/doula-cloud/issues/282)** |
| 6.1 | `manual` | as expected in kind, and sharper in fact | **From a genuine cold start she reaches nothing**: the app's `/` is the unmodified SvelteKit scaffold — `Welcome to SvelteKit` and a link to the framework's docs (`app/src/routes/+page.svelte`) — and the Engagement page renders **zero** `<a>` elements, so there is no way in and no way back. New gap **[PR-G9](https://github.com/markgoho/doula-cloud/issues/283)**. Timed from the Practice landing instead, the URL she would have kept: **2 clicks and ~1.7 s** to the Birth Plan loaded and in view. The page is **1755 CSS px** tall at 412px wide and the **Birth Plan** heading sits at y=692, **0.83 screens** down, behind `Marisol Vega`, `Visits` and `Care Plan`. Milder burial than [PR-G5](https://github.com/markgoho/doula-cloud/issues/280) claimed, and it degrades as Visits accumulate, since Visits is the one section above it that grows |
| 6.2 | `manual` | as expected | `GET .../plans/birth_plan` (not `.../plans/birth`; cell corrected) renders the snapshot: `Hospital`, `Partner`/`Doula`/`Midwife` ticked, `Dim lights, my own playlist. No epidural unless I ask twice.`, photos consented. The one thing she came for is right there and correct |
| 6.2-a | `missing-feature (PR-G5)` [#280](https://github.com/markgoho/doula-cloud/issues/280) | as expected | Confirmed unwalkable, all three ways. The only `id` on the page is `svelte-announcer`, so there is nothing to deep-link to; there are **0** `<details>` elements, so nothing collapses; and no stylesheet loaded on that page carries an `@media print` rule, with no print, export or share control anywhere |
| 7.1 | `manual` | as expected | **Add a Visit** -> `201`, and the Visits table gained a row reading `Priya Raman`, `8/23/2026`. She could only do this because of 2.1-a |
| 7.1-a | `missing-feature (MO-G1)` [#250](https://github.com/markgoho/doula-cloud/issues/250) | as expected, and sharper | Confirmed unwalkable. There **is** a `Date` column and it prints `8/23/2026` — `created_at`, not when the Visit was, exactly as Tasha's walk found on the Visits screen. Nothing on the page takes a date |
| 7.1-b | `missing-feature (PR-G6)` [#281](https://github.com/markgoho/doula-cloud/issues/281) | as expected | Confirmed unwalkable. The Visits section's only input is `Reassign to Staff id`. No type, no prenatal/birth/postpartum anywhere |
| 7.1-c | `missing-feature (MO-G2)` [#251](https://github.com/markgoho/doula-cloud/issues/251) | as expected | Confirmed unwalkable. No notes field, so "what was I told last time" has nowhere to live |
| 8.1 | `manual` | as expected | `POST .../messages` -> `201`, appended to the one Engagement thread with her name, `(staff)`, and a timestamp |
| 8.2 | `manual` | as expected | Marisol replied from the portal and the message landed in the same thread, in order, labeled `(client)`. Immutable as claimed: no edit or delete control renders, and `api/main.go` mounts no `PUT`, `PATCH` or `DELETE` on messages |
| 8.2-a | `automated (push-notification.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| PR-B1 | `manual` | as expected | **Holds.** The heading `Staff` renders, then `only a Practice Owner can do that` where the roster would be. `403 GET .../staff` |
| PR-B2 | `manual` | as expected | **Holds on submit.** The form renders with **no API call at all** on load — name, email, **Send invite** — and the press answers `403 only a Practice Owner can do that`, printed on the screen |
| PR-B3 | `manual` | as expected | **Reads through.** `200 GET .../plan-templates/care_plan` hands her the Practice's five real Care Plan fields with every editing control — **Move up**, **Move down**, **Remove**, **Add field**, **Save**. `403 PUT` at **Save**. Under ADR-0006 the read is correct and only the refusal at Save matters |
| PR-B4 | `manual` | as expected | Same shape, same verdict. The Practice's real template prose and the whole six-token merge-field legend render; `403 PUT` at **Save** |
| PR-B5 | `manual` | **falsified in shape** | It holds, but not where the cell said. The screen hands her the Practice's Stripe status first — `Stripe Connect status: Not connected` ([RA-G9](https://github.com/markgoho/doula-cloud/issues/267)) — and then renders **no Connect button at all**, only `Ask a Practice Owner to connect Stripe.` The direct `POST .../connect` also `403`s. So the refusal she meets is a hidden control, not a rejected press |
| PR-B6 | `manual` | as expected, with one correction | **Fails on read**: `Credit balance: 1` and the whole ledger, three rows, `signup_bonus +3` and two `consumption -1`. She reads what the Practice spends ([DW-G4](https://github.com/markgoho/doula-cloud/issues/272)). Buying is refused twice over — **Buy credits** renders `disabled` for a non-owner (`billing/+page.svelte:84`), and the endpoint answers `403 only a Practice Owner can do that` on a direct call |

**22 `manual` steps walked (5.2-a added by the walk); 6 `missing-feature` steps confirmed unwalkable; no `blocked` step on this plan.** Four expected results were falsified — 2.1-b, 3.2, 5.2 and PR-B5 — and none was re-marked, because every one of them is a performable step whose claim was simply wrong ([#235](https://github.com/markgoho/doula-cloud/issues/235)'s precedent). Two gaps minted on the journey map, **PR-G8** and **PR-G9**, and **PR-G3** narrowed rather than deleted. No `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and the walk made the failure worse than the map argued.** Three of the four clauses hold: she has an active membership holding the Doula role, she logged a Visit — and only the role let her — and she exchanged messages with the Client in one clean thread. The Engagement she opened is **not assigned to her**, because no Engagement is assigned to anyone (RA-G4). The last clause fails hardest. The map's case was that she *sees* screens she has no right to: the whole Practice's Client list, another Doula's Contract price, the Practice's credit ledger, both Templates, the Stripe status. All of that is confirmed. What the map did not claim, and the walk found, is that she can **act** on what she should not even see — set the price on a Contract and send it to a Client (**PR-G8**). Reading follows no role; writing follows only the Contract's own status.

**Her moment of truth stands where the map put it, and fails for a different reason.** Once she is on the Engagement page the Birth Plan is 0.83 screens away and reads correctly in under two seconds, which is not the corridor disaster the map feared. Getting there is: the product's front door is the framework's demo page, the Engagement page has no links on it, and there is no deep link, no collapse and no print. The corridor problem is real; it is a problem of arrival, not of scrolling.

