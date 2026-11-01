# Maya Okonkwo — test plan

- **Journey**: [solo-birth-doula.md](../journeys/solo-birth-doula.md)
- **Persona**: [solo-birth-doula.md](../personas/solo-birth-doula.md)
- **A pass means**: one Client carries a signed Contract, a filled Birth Plan the Client can read in the portal, at least one Visit, an open message thread, and an Invoice — reached by one person with no help.

## Preconditions

None. Maya's journey starts at a cold `/signup`, so this plan builds its own fixture and is the only practice-side plan that needs no seeded state.

## Steps

### Stage 1 — Sign up and create the Practice

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open `/signup` | Three question pages, then a check page ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)): `About you` (First name, Last name, Email address, Password, **Continue**), `What is your Practice called? (optional)` (left empty, the Practice takes her own name, first name then last name, [#1536](https://github.com/markgoho/doula-cloud/issues/1536)), `Where do you work?` (the state she works from and the timezone), then `Check your answers`, each answer with its own **Change**. Directly above **Create your Practice**, the sentence "By creating your Practice, you agree to the Terms of Service and the Privacy Policy.", each document a link that opens in a new tab and the sentence the button's accessible description; no checkbox ([#1547](https://github.com/markgoho/doula-cloud/issues/1547)) | `automated (signup-form.e2e.ts)` |
| 1.2 | Answer every page and press **Create your Practice** | `POST /api/staff/signup` succeeds; a Practice, a Staff row, a membership holding `owner`, `admin` and `doula`, and the Practice's `terms_accepted` Activity row are created together. The Practice's Activity reads "Terms of Service and Privacy Policy agreed to" with her name ([#1547](https://github.com/markgoho/doula-cloud/issues/1547)) | `automated (signup-form.e2e.ts)` |
| 1.3 | Land on `/practices/[practiceId]` | The heading `Overview` and the Practice's name in the top bar (#2141). The body says she holds every role and offers one action, **Add your first Client** ([#452](https://github.com/markgoho/doula-cloud/issues/452)) | `automated (signup-form.e2e.ts)` |
| 1.3-a | Read the credit balance on **Credits** | `Credit balance: 3`, one ledger row `Welcome credits +3` | `automated (billing.e2e.ts)` |
| 1.B-screens | Count the screens from the stage's entry to its end | **Screens**: 5 (Hick's Law, Flow). About you, the Practice name, Where do you work?, Check your answers, then the Practice landing ([#2049](https://github.com/markgoho/doula-cloud/issues/2049)) | `manual` |
| 1.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). The largest is Check your answers: five **Change** links and **Create your Practice**. About you is 5 (four boxes and **Continue**), Where do you work? 3, the Practice name page 2 | `manual` |
| 1.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 1.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 1.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 1.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

#318 closed the seam this note used to describe: `signup-form.e2e.ts` drives the `/signup` screen itself, rather than provisioning through `POST /api/staff/signup` directly the way every other spec still does.

### Stage 2 — Judge the seeded Plan Templates

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/practices/[practiceId]/settings/plan-templates` | The seeded Care Plan questions are listed, none of them blank, each with its kind of answer and a **Change** (#2193) | `automated (plan-templates.e2e.ts)` |
| 2.2 | Switch to the Birth Plan tab | A separate seeded field set; edits to one plan type do not appear on the other | `automated (plan-templates.e2e.ts)` |
| 2.3 | **Add a question**, choose its kind of answer, fill in its own page and **Save question** | `PUT /api/practices/{id}/plan-templates/{planType}` succeeds for an Owner; the list it lands on says "Care Plan questions saved." and holds the question after a reload (#2193) | `automated (plan-templates.e2e.ts)` |
| 2.4 | Open `/practices/[practiceId]/settings/contract-template`, edit the prose, **Save**, reload | The seeded prose renders with `{{client_name}}` visible; the edit persists | `automated (contract-template.e2e.ts)` |
| 2.B-screens | Count the screens from the stage's entry to its end | **Screens**: 3 (Hick's Law, Flow). The Plan Templates list, a question's own page (its **Change**), whose save lands back on the list, then the Contract Template by **Next, check your Contract Template** (#2193) | `manual` |
| 2.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law). The Plan Templates list counts its list of questions as one decision, however many rows it holds (#2193, 2026-10-30): the tabs, the list, **Add a question** and **Next, check your Contract Template**, 4. Then a question's own page and the Contract Template (the prose and **Save**) | `manual` |
| 2.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 2.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 2.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 3 — Add the first Client (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | On her empty Practice, press **Add your first Client** | Intake opens on its one question, `What is the Client's name?` — Given name, Family name (optional), Preferred name (optional) — with one button, **Save and continue**, and no search in front of it (ADR-0017, [#1609](https://github.com/markgoho/doula-cloud/issues/1609), [#1611](https://github.com/markgoho/doula-cloud/issues/1611)) | `automated (add-client-visits.e2e.ts)` |
| 3.1-a | Look for due date, phone, address, hospital, or intake notes | The due date is on the Start work form, the next screen. Phone, address and date of birth are added from the Client's record, through **Add {name}'s details**, one question per page ([#1610](https://github.com/markgoho/doula-cloud/issues/1610)), and the folder's remaining facts — hospital, the partner's name, the page of notes — go on the Practice's own Client Field Template, which that journey then asks as its own pages. The paper folder can be transcribed ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252) is closed). `manual` rather than `automated`, because the claim reaches the whole details journey and the Practice's own template pages, and no spec answers them all — `accessibility.e2e.ts` opens two of the routes to run axe over them, which tests the pages and not the folder | `manual` |
| 3.2 | Answer the name question and press **Save and continue** | The Client is saved and **the save is free**: no Engagement row, no `credit_ledger` row. She lands on the Start work form for that Client, which says "{name} saved as a Client." and has focus on that message ([#1710](https://github.com/markgoho/doula-cloud/issues/1710)) | `manual` |
| 3.2-a | Re-read the Billing balance | `Credit balance: 3`, still, with the `signup_bonus` row alone. Adding a Client costs nothing. `manual`: `billing.e2e.ts` reads that balance on a Practice that has added no Client, so it never asserts the thing this step checks — that the balance is unmoved *after* one | `manual` |
| 3.2-b | Read the Start work form and press **Start work with {name}** | Before she presses **Start work with {name}**, the form says `Starting work with {name} uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2.` ([#1612](https://github.com/markgoho/doula-cloud/issues/1612)). She is the Owner, so ADR-0017's solo-Practice collapse fires: the Request is written and approved in the same instant, mailing nobody, and it is *this* act that creates the Engagement and locks the Credit. She lands on the Engagement's page, which says "Work with {name} started." | `automated (add-client-visits.e2e.ts)` |
| 3.3 | Open the Engagement from the Clients list | The Clients list's **Birth** link opens the Engagement page, which holds On call, Birth outcome, Contract, Visits, Care Plan, Birth Plan, Invoices, Offers, Messages and Activity on one page, under an **On this page** list | `manual` |
| 3.4 | Add a second and third Client, start work with all three, then attempt a fourth Engagement | Clients are free and unlimited; the wall is on Engagements. On an empty balance the fourth Start work form says "There are no credits left on this Practice's balance." with a **Buy credits** link, and no sentence about what is left after ([#1235](https://github.com/markgoho/doula-cloud/issues/1235), [#1612](https://github.com/markgoho/doula-cloud/issues/1612)). **Start work with {name}** is still offered; pressing it returns `402 no credits remaining, ask a practice owner or admin to buy more` and the whole act rolls back, leaving no half-written Request behind, but nothing on screen changes ([#2186](https://github.com/markgoho/doula-cloud/issues/2186)). What she typed is kept: back from **Buy credits**, the form still holds it | `manual` |
| 3.4-a | Follow **Buy credits** and buy two | The Credits screen, then Stripe Checkout for the chosen quantity at $20.00 each, with New York sales tax added from the billing address; paying returns to `?checkout=success` and the `checkout.session.completed` webhook credits the ledger with a `Purchase` row | `manual` |
| 3.B-screens | Count the screens from the stage's entry to its end | **Screens**: 5 (Hick's Law, Flow). The empty Practice, the name question, the Start work form, the Engagement: four ([#1611](https://github.com/markgoho/doula-cloud/issues/1611)) | `manual` |
| 3.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 6 (Hick's Law, Miller's Law). The largest is the Start work form: the kind of work, the due date, "Who is the Doula?", the Note, **Start work with {name}** and **Go to {name}'s record without starting work**, with a radio group or a select as one decision. The name question is 4: the given name, the two optional names, and **Save and continue** | `manual` |
| 3.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory). Each folder fact she has already entered is on screen when a later question needs it | `manual` |
| 3.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 3.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). The save names the Client and says nothing was spent; the Engagement Request names the credit it spent and the balance after | `manual` |
| 3.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 3.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

**The doula's card statement says `DOULA.CLOU`.** Found on Dee's walk ([#236](https://github.com/markgoho/doula-cloud/issues/236)) while tracing the Client-facing `DOULA.CLOU` that `7261a59` fixed. This is the *other* half of that bug and it is **not** fixed: the credits Checkout session sets no descriptor (`billing/purchase.go`), so the charge falls back to the platform account's, and the platform's `statement_descriptor` is `DOULA.CLOUD` — 11 characters. Stripe caps a card prefix at 10 and truncates, giving `DOULA.CLOU`. The Client never sees it, because a connected account now carries its own `display_name`; the **doula** sees it every time she buys credits.

**No code change fixes this** — it is one Dashboard field, so it is a launch checklist item rather than a `journey-gap`. Set the *shortened descriptor* (the prefix) on the DoulaCloud account to `DoulaCloud`, which is exactly 10 characters and drops the period that makes the truncation read as a cut-off URL. Stripe only exposes it in the Dashboard (`settings/business-details`), not the API. It is set on the **live** account only: on 2026-10-04 the Sandbox's Business details were empty until products are activated and its payments settings were live-only, so the Sandbox cannot hold a descriptor, although it showed one on 2026-08-22. On the live account it sits beside the public business name, which still said "Doula Cloud", in the activation that [Take the Stripe platform account to production (#387)](https://github.com/markgoho/doula-cloud/issues/387) covers; its criteria carry both. <!-- brand:ignore: quotes the name Stripe still showed -->


### Stage 4 — Fill the Care Plan and the Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Create and fill the Care Plan section, save | `PUT .../plans/care` persists; the values survive a reload | `manual` |
| 4.2 | Press **Start the Birth Plan**, fill a field, **Save Birth Plan** | The value persists, and the Client's portal Birth Plan view shows the same text | `automated (birth-plan.e2e.ts)` |
| 4.2-a | Edit the Birth Plan **template** afterwards, then reopen the filled plan | The filled Plan Instance is unchanged — it snapshots the field definitions at creation | `manual` |
| 4.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 4.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law). A plan with more fields than this is grouped into sections, so no group shows more than seven at once | `manual` |
| 4.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 4.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 4.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 4.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 5 — Contract and signature

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Send the portal invite from the Engagement page | `POST .../portal-invite` returns a token and the invite is emailed to the Client's address (RA-G1 is closed); the page also shows the link to share by hand. The Contract section says, before a Contract exists, that the Client signs only once she has accepted the invite ([MO-G6](https://github.com/markgoho/doula-cloud/issues/255) is closed) | `manual` |
| 5.2 | Build the Contract from the Practice template | Needs a birth rate first: with none, the Contract section says "Your Practice has no birth rate yet, so you cannot create a Contract." and links **Set the birth rate**. With one, **Create a Draft Contract** opens the Draft with Practice name and Client name filled from the Practice and the Client, and the Fee from the rate ([MO-G10](https://github.com/markgoho/doula-cloud/issues/258) is closed); scope and dates are typed. Saved, it reads "Draft, not sent yet" | `automated (contract-lifecycle.e2e.ts)` |
| 5.3 | Press **Send** | Status moves to `sent` | `automated (contract-lifecycle.e2e.ts)` |
| 5.4 | As the Client, accept the invite and sign in | The accept → login path lands on `/portal/engagements/{engagementId}` | `automated (portal-invite-accept.e2e.ts)` |
| 5.4-a | Sign the Contract from the portal | `POST /api/portal/engagements/{id}/contract/sign` renders the signed PDF, puts it in the object store, **then** sets status `signed` | `automated (contract-lifecycle.e2e.ts)` |
| 5.5 | Back as Maya, reload the Engagement | After a reload, the Contract reads "Signed on {date}", "Signed by {name}", with **Download signed Contract (PDF)** and **Void this Contract**. An Engagement page already open does not change until it is reloaded | `manual` |
| 5.5-a | Move the Engagement past `intake` | The Engagement page's **Care phase** has **Change**: **Mark care as active** and **Mark care complete**. The first scheduled Visit moves it from Intake to Active on its own. **Mark care complete** asks why the Engagement is ending; on a birth Engagement with no birth outcome it refuses until **Record what happened** is answered ([MO-G4](https://github.com/markgoho/doula-cloud/issues/253) is closed, [#2199](https://github.com/markgoho/doula-cloud/issues/2199)) | `manual` |
| 5.B-screens | Count the screens from the stage's entry to its end | **Screens**: 2 (Hick's Law, Flow). Her own screens only; the Client signs on hers | `manual` |
| 5.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 7 (Hick's Law, Miller's Law) | `manual` |
| 5.B-memory | Note every later step that needs an earlier answer, and whether that answer is still on screen | **Memory**: yes (Working Memory) | `manual` |
| 5.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule). Sending says what the Client receives next, and `signed` reaches her without leaving the app | `manual` |
| 5.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 6 — Schedule Visits

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Visits section | A list and an add control | `automated (add-client-visits.e2e.ts)` |
| 6.2 | Add a Visit | **Add a Visit** asks who it is for and an optional scheduled date and time; **Save Visit** says "Your Visit is added." and the row names the Staff member | `automated (add-client-visits.e2e.ts)` |
| 6.2-a | Set a date and time for a future prenatal | The Visit's "Scheduled date and time" is saved and shown in the Date column, with Type "Prenatal" ([MO-G1](https://github.com/markgoho/doula-cloud/issues/250) is closed) | `manual` |
| 6.2-b | Write what was covered on the Visit | The Visit's **Change** opens a Notes field beside the scheduled time; saved, the note shows in the Visits table's Notes column ([MO-G2](https://github.com/markgoho/doula-cloud/issues/251) is closed) | `manual` |
| 6.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 6.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). When the Visit is, which kind, what was covered, and the save | `manual` |
| 6.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 6.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 6.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 7 — Get paid

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Open `/practices/[practiceId]/settings/payments` and start Connect onboarding | **Getting paid** first asks the website question (**Answer the website question**); saving **Your website** returns to **Getting paid** with "Your website is saved. You can connect Stripe now." ([MO-G15](https://github.com/markgoho/doula-cloud/issues/2196)). **Getting paid** then lists what Stripe will ask for and offers **Connect Stripe**. `POST .../payments/connect` passes for the Owner and opens a real v2 Account Link. Stripe's hosted flow runs login and two-step (an authenticator app), business type, personal details, business details, payout account, public details, Radar, Climate, Tax and review, then returns to `?connect=return`, which reads "Awaiting Stripe review" until Stripe verifies. The `capability_status_updated` webhook writes the account `active` (`practices.stripe_connect_status_event_id` holds the event), and **Getting paid** reads "Taking payments". Stripe challenges an automation-launched browser at its email step, so this step is walked in a person's own Chrome ([connect-onboarding.md](connect-onboarding.md)) | `manual` |
| 7.2 | Raise an Invoice against the signed Contract | The first **Create Invoice** asks "How does this Practice bill Clients?", **By card, through Stripe.** or **By hand.**, and saves the answer as the Practice's billing mode. By card on a connected Practice: "Invoice created." and a row with the Invoice number, the Raised day, the Contract's amount and **Open**. Stripe holds a finalized Invoice on the connected account, and the Client pays it on Stripe's hosted page; `invoice.paid` marks it **Paid** on the Engagement, on the Practice's **Invoices** list and in the Client's portal. On a Practice whose Stripe onboarding is not finished, the same press refuses: "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe before an Invoice can be sent." ([#2200](https://github.com/markgoho/doula-cloud/issues/2200)) | `manual` |
| 7.2-a | Compare the **Credits** and **Settings → Getting paid** screens | Each names the other: Credits says "Practices buy Credits from DoulaCloud." and that getting paid is a separate screen; Getting paid says it is where a Practice connects Stripe so its Clients can pay it, and that Credits is separate ([MO-G7](https://github.com/markgoho/doula-cloud/issues/256) is closed) | `manual` |
| 7.B-screens | Count the screens from the stage's entry to its end | **Screens**: 7 (Hick's Law, Flow). From a Practice that has not answered the website question, on **Publish a page for me**: **Getting paid**, **Your website**, **What your page will say**, **Check your page before you publish it**, **Getting paid** again (its save returns there), back from Stripe on `?connect=return`, then the Engagement's Invoice. The own-site answer is 5, without the two page screens. Stripe's own pages do not count ([MO-G15](https://github.com/markgoho/doula-cloud/issues/2196), [#2302](https://github.com/markgoho/doula-cloud/issues/2302)). Walked on 2026-11-01 as far as **Connect Stripe** on both answers; the leg back from Stripe and the Invoice wait on hosted onboarding in a person's own Chrome ([#2264](https://github.com/markgoho/doula-cloud/issues/2264)) | `manual` |
| 7.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 3 (Hick's Law, Miller's Law). A radio group is one decision: **Your website** on the own-site answer is the question, the web address and **Save**; on **Publish a page for me** it is the question and **Continue**, and **What your page will say** is the two answers and **Continue**, with **Back** not counted ([#2302](https://github.com/markgoho/doula-cloud/issues/2302), walked 2026-11-01); the first **Create an Invoice** is the billing-mode question, **Create Invoice** and **Cancel** | `manual` |
| 7.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 7.B-cold | Walk the stage in a fresh browser profile, with nothing explained first | **Cold**: yes (Paradox of the Active User) | `manual` |
| 7.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

### Stage 8 — Message the Client

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Send a message from the Engagement page | **Write a message** opens Message, Attachment, **Send** and **Cancel**; sent, the message is appended to the one Engagement thread with "Message sent.", and it cannot be edited or deleted | `manual` |
| 8.2 | Reply from the portal and reload Maya's view | One continuous thread, both sides in order | `manual` |
| 8.2-a | With the portal thread open, deliver a push event | The open tab refetches and renders the new message | `automated (push-notification.e2e.ts)` |
| 8.B-screens | Count the screens from the stage's entry to its end | **Screens**: 1 (Hick's Law, Flow) | `manual` |
| 8.B-decisions | On each screen, count the decisions visible at once; record the largest | **Decisions**: 4 (Hick's Law, Miller's Law). Message, Attachment, **Send** and **Cancel**, after **Write a message** | `manual` |
| 8.B-time | Time each routine act on the stage's device, from the input to the result settled on screen | **Time**: 400 ms, 100 ms (Doherty Threshold) | `manual` |
| 8.B-confirmation | Read the screen the stage ends on | **Confirmation**: yes (Peak-End Rule) | `manual` |
| 8.B-320px | Sweep the stage's screens from 320px up | **320px**: yes (ADR-0024) | `automated (route-continuum.svelte.spec.ts)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 26 (8 of them [budget steps](README.md#budget-steps)) |
| `manual` | 54 (36 of them [budget steps](README.md#budget-steps)) |
| `blocked` | 0 (7.1 and 7.2 cleared on 2026-10-28: hosted onboarding finished in a person's own Chrome, and the Invoice raised and paid) |
| `missing-feature` | 0 |

MO-G1, MO-G2, MO-G3 and MO-G4 ([#250](https://github.com/markgoho/doula-cloud/issues/250), [#251](https://github.com/markgoho/doula-cloud/issues/251), [#252](https://github.com/markgoho/doula-cloud/issues/252), [#253](https://github.com/markgoho/doula-cloud/issues/253)) are closed, and 6.2-a, 6.2-b, 3.1-a and 5.5-a are walkable, so none of them holds a step.

MO-G5 to MO-G9 are experience-layer or infrastructure findings; they are observed inside the steps above (3.4, 7.1, 7.2-a) rather than given steps of their own. MO-G9 ([#257](https://github.com/markgoho/doula-cloud/issues/257)) is closed as well — 3.4 still meets a wall, but a paid one she can walk through, not the dead end that gap named.

## Run log

### 2026-11-01 — the fixed steps walked again ([#2264](https://github.com/markgoho/doula-cloud/issues/2264))

**Stack**: `SEED_DEMO=0 bun run --cwd app dev:full` from a worktree at port offset 1 (the app at `http://localhost:5273`, the mail catcher at `http://127.0.0.1:19180`), on trunk at `3459116a`, in WSL2 with rootless Podman. Stripe in the Sandbox with a test-mode key; `stripe listen` was not run, because no step here reached a Stripe event.

**How it was walked**: by the blind `persona-walker` and the sighted `walk-evaluator` ([`docs/agents/persona-walk.md`](../agents/persona-walk.md)), 69 acts with a shot and a timing each and no `u` entry, so the walk is admissible. **Device**: headless Chrome in WSL, driven by the chrome-devtools MCP, at `1280x800x1` with a pointer and no throttling. Maya's persona points at a phone walk; #2264's own count for 2.B-decisions is at desktop width, and the 2026-10-27 run was at 1280x900, so the desktop profile was kept so the numbers compare. Each Decisions count is by [the README's rules](../journeys/README.md#the-budget-line) and #2264's "How to count" notes: a radio group, select, date or checkbox group is one; a list of the Practice's own records is one; **Back** (except where #2264's 2.B list counts it), the skip link and the Staff top bar are not counted.

**Accounts**: two fresh signups, each in its own isolated context. **Practice A**, `Okonkwo Birth Support` (`maya.okonkwo.walk3@sim.doula.cloud`, Rhode Island, Eastern time), walked stages 1 to 5, 7 and 8 with one Client, Nadia Haddad (`nadia.haddad.walk3@sim.doula.cloud`, birth due Feb 15, 2027), answering the website question with her own site. Between stage 5 and stage 7 the orchestrating agent, not the walker, accepted Nadia's portal invite and signed the Contract as her, as precondition setup. **Practice B**, `Okonkwo Doula Care` (`maya.okonkwo.walk3b@sim.doula.cloud`), walked stages 1, 2 and 7 with no Client, answering **Publish a page for me**; it is where 2.B's **Next, add your first Client** shows.

**Not walked**: the leg of stage 7 after Stripe. On both Practices **Connect Stripe** opened Stripe's hosted onboarding, and the walk stopped there and typed nothing, because Stripe challenges an automation-launched browser and the onboarding is finished by a person in their own Chrome ([connect-onboarding.md](connect-onboarding.md)). The return on `?connect=return` and the Invoice after it wait on that.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-confirmation | yes | yes. On arrival from signup, "You created your Practice, Okonkwo Birth Support." before the heading, focused (B); Practice A read the same. The next visit to Overview shows it no more, on both | **yes**, [TB-G9](https://github.com/markgoho/doula-cloud/issues/2182) built |
| 2.B-screens | 3 | **3** on Practice B: the Plan Templates list, a question's own page from its **Change** (she reworded "Preferred pain management"), **Save question** back to the list (not counted again), then the Contract Template by **Next, check your Contract Template**. Practice A added a question instead (the list, the kind of answer, the new question's page, the list) and did not go on | **yes** |
| 2.B-decisions | 7 | **7**, a saved question's page (**Back**, Question, Kind of answer, Answers to choose from, Place in the list, **Save question**, **Remove question**); at 1280x800 four of them are in view at once. The list 4 (the tabs, the list, **Add a question**, **Next, check your Contract Template**), the kind of answer 3, a new question's page 4, the Contract Template 3 (the prose, **Save**, **Next, add your first Client**) | **yes** |
| 2.B-confirmation | yes | yes, on Practice B. **Save question** lands on the list with a focused "Care Plan questions saved. Every Care Plan filled in from now on asks them, and a Care Plan already filled in keeps the questions it had." Saving the Contract Template says, under **Save** with focus left there, "Contract Template saved. Every Contract written from now on uses these terms, and a Contract already written keeps the terms it was written from.", with **Next, add your first Client** | **yes**, [MO-G13](https://github.com/markgoho/doula-cloud/issues/2194) built |
| 3.B-decisions | 6 | **6** on the Start work form (the kind of work, the due date, "Who is the Doula?", the Note, **Start work with Nadia**, **Go to Nadia's record without starting work**); the name question 4. Off the note's path: the details journey's check page (3.1-a) holds seven **Change** links and **Save these details**, 8, and all eight fit in one scroll position at 1280x800 (measured by the orchestrating agent on the same page after the walk) | **yes** on the note's path; the check page is recorded below |
| 4.B-decisions | 7 | **6**, the largest group, on a Practice signed up after #2299. The Birth Plan opens in two groups: **When Labor Starts** 1 (one checkbox group of five), **Birth Setting** 6 (location, atmosphere, photos, her own hospital-bag question, **Save Birth Plan**, **Cancel**; the seed's 5 plus the question she added). The Care Plan's one group 6 | **yes**, [MO-G14](https://github.com/markgoho/doula-cloud/issues/2195) built |
| 7.B-screens | 7 | Own-site answer (A): **3 of 5** walked (**Getting paid**, **Your website**, **Getting paid** again, "Your website is saved. You can connect Stripe now."). **Publish a page for me** (B): **5 of 7** walked (**Getting paid**, **Your website**, **What your page will say**, **Check your page before you publish it**, **Getting paid** again, "Your page is published. You can connect Stripe now."). Neither path is over its budget as far as walked | not measurable to the end: the leg back from Stripe waits on a person's own Chrome |
| 7.B-decisions | 3 | **3**, the largest: **Getting paid** 3 (**Change**, **Check status again**, **Answer the website question** or **Connect Stripe**); **Your website** on the own-site answer 3 (the question, the web address, **Save**), on **Publish a page for me** 2 (the question, **Continue**); **What your page will say** 3 (two answers, **Continue**); the check page 1 (**Publish page**); the first **Create an Invoice** 3 (the billing-mode question, **Create Invoice**, **Cancel**) | **yes**, [MO-G15](https://github.com/markgoho/doula-cloud/issues/2196) and [#2302](https://github.com/markgoho/doula-cloud/issues/2302) built |
| 8.B-decisions | 4 | **4**: Message, Attachment, **Send**, **Cancel**, after **Write a message**, on an Engagement whose Client has accepted the invite | **yes**, [MO-G16](https://github.com/markgoho/doula-cloud/issues/2197)'s budget |

No mark moved, so the Marks summary and [README.md](README.md)'s run-status row are unchanged.

**Seen on the way, outside the steps above**, for [`docs/simulation/findings.md`](../simulation/findings.md)'s pass: the details check page's 8 decisions (above); a Birth Plan's untouched yes-or-no questions read "No" while its progress counts them unanswered ("2 of 5 questions answered"); on the own-site path, **By card, through Stripe.** on the first **Create an Invoice** is refused only after the press ("Connect Stripe on Getting paid before Clients can pay by card"), and a by-hand Invoice then raised at once with nothing saying the Client was told; the Clients list's name link opens the Client record, so reaching the Engagement takes its **Birth** link too; the Invoices list's intro says "reach the Engagement it belongs to", against the map's Words; **What your page will say** and **Check your page before you publish it** change in place at one address, so no route announcement is made for either.

### 2026-10-27 — second manual walk ([#2171](https://github.com/markgoho/doula-cloud/issues/2171))

**Stack**: `bun run --cwd app dev:full` from a worktree at port offset 1, so the app at `http://localhost:5273`, the BFF at `127.0.0.1:18180` and the mail catcher at `http://127.0.0.1:19180`, on trunk at `b4497400`, in WSL2 with rootless Podman. The seed brought up Rooted Birth Collective only, which this walk does not touch. Stripe ran in the Sandbox with a test-mode key, through `stripe listen` with the three forwards of `scripts/stripe-listen.sh` pointed at `18180` by hand, because the script names offset 0's port ([#2201](https://github.com/markgoho/doula-cloud/issues/2201)). The secret it printed matched all three in `app/.env.local`.

**Device**: headless Chrome inside WSL, driven by the chrome-devtools MCP, at 1280x900, one isolated browser context for Maya and another for Nadia. Every timing below is on this stack: an unbundled Vite dev server, which compiles a route's modules the first time it is visited.

**Accounts and preconditions**: none from the seed, as the plan says. Maya signed up at `/signup` in a fresh isolated context (the Cold steps) as `maya.okonkwo@sim.doula.cloud`, Practice `Okonkwo Birth Support`, New York, Eastern time. Her four Clients: **Nadia Haddad** (`nadia.haddad@sim.doula.cloud`, a birth Engagement due Feb 15, 2027, carried through stages 4 to 8), **Hannah Sorensen** (`hannah.sorensen@sim.doula.cloud`, a birth Engagement due Mar 10, 2027, started and nothing else, for her own walk), **Imani Brooks** (a birth Engagement, completed at 5.5-a) and **Grace Lin** (saved, no Engagement, the fourth at 3.4). Nadia accepted her portal invite from the mail catcher in her own context, signed the Contract, and replied in the thread; she did not press **I've read this** on the Birth Plan, so her own walk meets it unread.

The 24 `automated` steps were **not** re-run; the walk read only what each partly-driven spec leaves out (the run sheet on #2171).

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `automated (signup-form.e2e.ts)` | cell corrected | Three question pages and a check page, not one form: `About you`, `What is your Practice called? (optional)`, `Where do you work?`, `Check your answers`, then **Create your Practice** |
| 1.2 | `automated (signup-form.e2e.ts)` | cell corrected | The Practice's Activity screen reads "Terms of Service and Privacy Policy agreed to", Maya Okonkwo, as its oldest row after "Maya Okonkwo joined the Practice" |
| 1.3 | `automated (signup-form.e2e.ts)` | cell corrected | `Overview`, `Okonkwo Birth Support` in the top bar. The seven tiles are gone; the body says "You hold every role — Owner, Admin and Doula." and offers **Add your first Client** |
| 1.3-a | `automated (billing.e2e.ts)` | cell corrected | The screen is **Credits**: `Credit balance: 3`, one row `Welcome credits +3` |
| 3.1-a | `manual` | as expected | Nadia's folder went in whole. From her record: email, phone `(585) 555-0142`, address `41 Linden Street, Rochester, NY 14620`. Under **Settings → Client Fields** Maya added three questions, "Hospital or birth center", "Partner's name" and "Intake notes", and Nadia's record then asked them on one page under "What else Okonkwo Birth Support asks": `Highland Hospital`, `Sami Haddad`, and a line of notes. The due date is on the Start work form |
| 3.2 | `manual` | as expected | "Nadia saved as a Client.", focused, on the Start work form. Read from the database at that moment: 1 `clients` row, 0 `engagements`, 0 `engagement_requests`, and the ledger's `signup_bonus 3` alone |
| 3.2-a | `manual` | as expected | Same read: the balance is unmoved by a save. The Start work form says "This Practice has 3 Welcome credits." |
| 3.3 | `manual` | as expected, cell corrected | From the Clients list ("Showing 1 Client with work."), the **Birth** link on Nadia's row opens her Engagement. Ten sections, not six, under **On this page**. "This Client has no email address on file" appears three times until an email is added |
| 3.4 | `manual` | as expected, cell corrected | Hannah (through **Find or add a Client**, which shows "You searched for "Hannah Sorensen". Type the name into the fields below.") and Imani took the second and third Credits. Grace's Start work form loaded with "There are no credits left on this Practice's balance." and **Buy credits**. Pressing **Start work with Grace** sent the request, `402` `{"code":"PAYMENT_REQUIRED","message":"no credits remaining, ask a practice owner or admin to buy more"}`, and nothing on screen changed ([#2186](https://github.com/markgoho/doula-cloud/issues/2186), open). The database held 4 Clients, 3 Engagements, 3 Requests: nothing written for Grace. An email also arrived, "DoulaCloud: your Practice is out of Credits" |
| 3.4-a | `manual` | as expected | **Buy credits** led to the Credits screen; quantity 2 opened Stripe Checkout: `Engagement credit`, `Qty 2`, `$20.00 each`, then `Sales Tax (8%) $3.20` once a Rochester billing address was typed, `$43.20`. Paid with `4242 4242 4242 4242`. Back on `?checkout=success`: `Credit balance: 2`, a `Purchase +2` row, and "Credit purchase complete. The balance updates once Stripe confirms payment." `checkout.session.completed` was delivered and answered `200`. Back on Grace's form, what had been typed (Postpartum, the note) was still there. Grace was left without an Engagement, so the Practice holds 2 Credits |
| 4.1 | `manual` | as expected | **Start the Care Plan**, four answers, **Save Care Plan**: "Care Plan saved." After a reload the section shows "Progress: Complete" and "Last edited Oct 27, 2026 by Maya Okonkwo", and **Change the Care Plan** opens the four answers as saved |
| 4.2-a | `manual` | as expected | Added "Is the hospital bag packed?" to the Birth Plan template (`PUT .../plan-templates/birth_plan` `200`, "Saved."). Nadia's filled Birth Plan, reopened, has no such question and keeps its answers |
| 5.1 | `manual` | **better than the plan claimed**, cell corrected | "Invited. An email has been sent to them. If you need to share the link directly, here it is:" and the link. The mail reached the catcher at `/inbox/nadia.haddad%40sim.doula.cloud`, "You've been invited to view your care details online". The line calls Nadia "them" ([#2198](https://github.com/markgoho/doula-cloud/issues/2198)). The Contract section had already said "Send a portal invite to this Client before sending the Contract" (MO-G6 closed) |
| 5.2 | `automated (contract-lifecycle.e2e.ts)` | re-marked, cell corrected | With no rate, the Contract section said "Your Practice has no birth rate yet, so you cannot create a Contract." and linked **Set the birth rate**. **Rates** took `1800` ("Rates saved. Birth: $1,800.00.") and offers no way back to the Engagement. **Create a Draft Contract** opened with Practice name `Okonkwo Birth Support` and Client name `Nadia Haddad` filled, Fee `$1,800.00`. Engagement start and end date are free-text boxes |
| 5.5 | `manual` | as expected, cell corrected | The Engagement page open in Maya's context still read "Sent, waiting for the Client to sign" after Nadia signed. After a reload: "Signed on Oct 27, 2026", "Signed by Nadia Haddad", **Download signed Contract (PDF)**, **Void this Contract** |
| 5.5-a | `manual` (was `missing-feature (MO-G4)`) | **walked**, cell corrected | Nadia's first scheduled Visit moved her Care phase from Intake to Active with no other press. On Imani's Engagement, **Change** → **Mark care complete** asked "Why is this Engagement ending?" (six answers and a note); **Confirm completion** then refused with the BFF's own words, "record what happened to the pregnancy before completing this Engagement; if the Practice never learned, say so, and that answer needs no date" ([#2199](https://github.com/markgoho/doula-cloud/issues/2199)). After **Record what happened** (born alive, Oct 25, 2026), the same press read "The care phase is now Completed." |
| 6.2 | `automated (add-client-visits.e2e.ts)` | re-marked, cell corrected | **Add a Visit** asks "Who is this Visit for?" and "Scheduled date and time (optional)"; **Save Visit**: "Your Visit is added.", a row of Date, Type, Doula, Notes |
| 6.2-a | `manual` (was `missing-feature (MO-G1)`) | **walked** | Scheduled for Nov 12, 2026, 10:00am and Jan 18, 2027, 2:30pm; each row reads the time, Type `Prenatal`, `Maya Okonkwo`. The Schedule and On call destinations exist in the top bar |
| 6.2-b | `manual` (was `missing-feature (MO-G2)`) | **walked** | The Visit's **Change** opens the scheduled time and a Notes field; saved, "The Visit for Maya Okonkwo is saved." and the note in the Notes column. Notes take a second act after the Visit is added |
| 7.1 | `blocked` (was `manual`) | **re-marked** | **Getting paid** first refused **Connect Stripe** until the website question was answered ("Stripe will not process Client payments until it can see this Practice online."); **Your website** took `https://okonkwobirthsupport.com` and has no way back. Then **Getting paid** listed what Stripe asks for, and **Connect Stripe** opened a v2 Account Link (`acct_1UVHSU1rKoN5fitT`). At the email step Stripe served an hCaptcha challenge to this headless Chrome, and the walk did not try to pass it. The fallback the walk was given, finishing the same account through the Sandbox API, is refused by design: `tos_acceptance_on_behalf_not_allowed` ("You cannot accept the Terms of Service on behalf of accounts where requirement collection is owned by Stripe.") and `forbidden` on `defaults.profile.product_description` ("the Account was onboarded via an Account Link"). The account stays `pending` with 12 requirements, and **Getting paid** says so: "Onboarding incomplete", "Stripe still needs 12 more details from you", **Continue Stripe onboarding** (MO-G11 closed in fact). `blocked`, by [the rule](README.md#the-four-marks): the step can be attempted, it stops for a reason outside the screen, and no product decision is missing. A person in their own Chrome finishes it ([connect-onboarding.md](connect-onboarding.md)) |
| 7.2 | `blocked` (was `manual`) | **re-marked** | On Nadia's signed Contract, **Create Invoice** asked "How does this Practice bill Clients?". **By card, through Stripe.** then refused: "Clients cannot pay this Practice yet. A Practice Owner has to connect Stripe before an Invoice can be sent." to the Owner, with no link to **Getting paid** ([#2200](https://github.com/markgoho/doula-cloud/issues/2200)). The billing mode was not saved by the refused press: **Getting paid** still reads "Not chosen yet". No Invoice exists. `blocked` for the reason 7.1 is |
| 7.2-a | `manual` | **better than the plan claimed**, cell corrected | Each money screen names the other: "Practices buy Credits from DoulaCloud. … Getting paid is a separate screen, where this Practice connects Stripe so its Clients can pay it directly." and "This is where a Practice connects Stripe, so its Clients can pay it directly. Credits is a separate screen, where this Practice buys Credits from DoulaCloud." MO-G7 is closed in fact |
| 8.1 | `manual` | as expected, cell corrected | **Write a message** opens Message, Attachment, **Send**, **Cancel**. Sent: "Message sent.", the message under "Maya Okonkwo, Oct 27, 2026, 8:58pm", and no edit or delete control |
| 8.2 | `manual` | as expected | Nadia replied from the portal thread ("You and Okonkwo Birth Support"). Maya's open page did not change; after a reload the thread showed both in order, Nadia's marked "Unanswered", and the Overview listed her under "Clients waiting on a reply" |

**Re-marked**: 5.2 and 6.2 to `automated`, each cell corrected first, as the run sheet set out; 5.5-a, 6.2-a and 6.2-b from `missing-feature` to `manual`, their gaps built; 7.1 and 7.2 from `manual` to `blocked`. 8.1 stays `manual`: `message-attachment.e2e.ts` sends the message and does not assert that it cannot be edited or deleted. The Marks summary is recounted to 26 / 52 / 2 / 0, and [README.md](README.md)'s run-status row and Total move with it.

#### Budget steps — measured

**How Decisions is counted on this walk**, as Tasha Bell's was: fields, options, and links or buttons that move her somewhere or change what happens, enabled and fully in view at once in the page body, at the scroll position where most are. **Back**, **Show password**, the skip link and links that open a policy in a new tab are not counted. The Staff shell's top bar is left out of every number and given beside it: it adds 12 (ten destinations, the account menu, the pilot banner's feedback link). Maya's map counts no stage's shell, so no stage here takes the top bar into its number. **Time** is measured in the page: a script presses the control and polls each animation frame until the result has settled on screen, and records when the control first showed it was busy.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 1.B-screens | 2 | **5**: four signup pages, then Overview | **no**, [TB-G8](https://github.com/markgoho/doula-cloud/issues/2181) |
| 1.B-decisions | 5 | **6** on `Check your answers` (five **Change** links and **Create your Practice**); `About you` 5, `Where do you work?` 3, the Practice name page 2 | **no**, [TB-G8](https://github.com/markgoho/doula-cloud/issues/2181) |
| 1.B-memory | yes | yes. `Check your answers` shows every answer with its own **Change** | yes |
| 1.B-confirmation | yes | **no**. Overview says what to do next, not that the Practice was created | **no**, [TB-G9](https://github.com/markgoho/doula-cloud/issues/2182) |
| 1.B-cold | yes | yes. Each page says why it asks | yes |
| 2.B-screens | 2 | **3**: Plan Templates, Settings, Contract Template. Plan Templates has no way on | **no**, [MO-G12](https://github.com/markgoho/doula-cloud/issues/2193) |
| 2.B-decisions | 7 | **16** on Plan Templates (two questions' five controls each, and six **Add a …** buttons; 35 on the screen); Contract Template 2. Plus 12 in the shell | **no**, [MO-G12](https://github.com/markgoho/doula-cloud/issues/2193) |
| 2.B-confirmation | yes | **no**. "Saved." and nothing else | **no**, [MO-G13](https://github.com/markgoho/doula-cloud/issues/2194) |
| 2.B-cold | yes | yes. Each screen's opening paragraph says the seeded set is meant to be changed and that a filled plan or a written Contract keeps what it was written from | yes |
| 3.B-screens | 5 | 4 for the first Client: Overview, the name question, the Start work form, the Engagement. 5 for the second, through the Clients list and **Find a Client** | yes |
| 3.B-decisions | 3 | **8** on the Start work form (Birth, Postpartum, Due date, two Doula options, Note, **Start work with {name}**, **Go to {name}'s record without starting work**); the name question 4; the address page 6. Plus 12 in the shell | **no**, [TB-G11](https://github.com/markgoho/doula-cloud/issues/2184) |
| 3.B-memory | yes | yes. The Client's name carries onto the Start work form and the Engagement; the due date onto the Engagement; each folder fact is on her record. A search's name is shown on the name question, to be typed again | yes |
| 3.B-time | 400 ms, 100 ms | **Add your first Client** 86 ms. **Save and continue** 172, 109, 107 ms. **Start work with {name}**: **638 ms** on the first press (the first visit to the Engagement route), then 175 and 191 ms. A details save 98 ms. Every button showed busy within 5 ms; **Create your Practice** settled in 709 ms, also a first visit, on stage 1, which has no Time budget | **not settled on this stack**, as on [Tasha Bell's walk](evaluator-doula.md#run-log): the only samples over budget are a route's first load on a dev server |
| 3.B-confirmation | yes | yes. "Nadia saved as a Client." on the save, and "Work with Nadia started." on the Engagement. The budget note says the Engagement names the Credit spent and the balance after; the form says both before the press ("uses 1 Credit … After this, it has 2."), and the Engagement does not repeat them | yes |
| 3.B-cold | yes | yes | yes |
| 4.B-screens | 1 | 1, the Engagement page | yes |
| 4.B-decisions | 7 | **10** in the Birth Plan's one section, "Birth Setting" (location, five people to notify, atmosphere, photos, **Save Birth Plan**, **Cancel**); the Care Plan 6 | **no**, [MO-G14](https://github.com/markgoho/doula-cloud/issues/2195) |
| 4.B-memory | yes | yes. The Client's name and due date are at the top of the page. Saved answers are behind **Change** and the printable view; the section shows progress and who last edited | yes |
| 4.B-time | 400 ms, 100 ms | **Start the Birth Plan** 46 ms to the open form; **Save Care Plan** 45 ms; **Save Birth Plan** 39 ms; busy within 3 ms | yes |
| 4.B-confirmation | yes | yes. "Birth Plan saved.", "Nadia has not read this yet", **Print for the hospital** | yes |
| 5.B-screens | 2 | 2: the Engagement page and **Rates**, which the Contract section sends her to first | yes |
| 5.B-decisions | 7 | 7 on the Draft Contract (five fields, **Save Contract**, **Cancel**). Plus 12 in the shell | yes |
| 5.B-memory | yes | yes. Practice name, Client name and Fee arrive filled | yes |
| 5.B-confirmation | yes | yes. "Contract sent." with "Sent, waiting for the Client to sign"; `Signed` reaches her on the Engagement page, on a reload | yes |
| 6.B-screens | 1 | 1 | yes |
| 6.B-decisions | 4 | 4 on **Add a Visit** (who, when, **Save Visit**, **Cancel**) and 4 on its **Change** (when, notes, **Save changes**, **Cancel**). The kind is not asked: the row reads `Prenatal` | yes |
| 6.B-time | 400 ms, 100 ms | **Save Visit** 68 and 70 ms; **Save changes** 46 ms; busy within 3 ms | yes |
| 6.B-confirmation | yes | yes. "Your Visit is added." and "The Visit for Maya Okonkwo is saved." | yes |
| 7.B-screens | 3 | 3 of the Practice's own so far: **Getting paid**, **Your website**, **Getting paid** again, then Stripe. Not finished: 7.1 is `blocked` | not measurable to the end |
| 7.B-decisions | 3 | **4** on **Your website** (two answers, the address, **Save**) and 4 on the first **Create an Invoice** (two billing modes, **Create Invoice**, **Cancel**); **Getting paid** 3. Plus 12 in the shell | **no**, [MO-G15](https://github.com/markgoho/doula-cloud/issues/2196) |
| 7.B-confirmation | yes | not measurable: the stage ends after Stripe, which 7.1 did not reach | — |
| 7.B-cold | yes | yes, as far as it went. **Getting paid** lists what Stripe will ask for and how long it takes before she starts | yes |
| 8.B-screens | 1 | 1 | yes |
| 8.B-decisions | 2 | **4**: Message, Attachment, **Send**, **Cancel**, after **Write a message**. The portal's thread is 3 | **no**, [MO-G16](https://github.com/markgoho/doula-cloud/issues/2197) |
| 8.B-time | 400 ms, 100 ms | **Send** 39 ms, busy within 2 ms | yes |
| 8.B-confirmation | yes | yes. "Message sent." and the message in the thread | yes |

The 320px budget steps are `automated (route-continuum.svelte.spec.ts)` and were not re-measured.

#### Moment of truth — the verdict, against the first run's

The run sheet on #2171 points at stage 3, where the map and the plan put it; the [journeys README](../journeys/README.md) still names the signed Contract. Both are answered.

**Stage 3, does the one place hold the Client record she needs?** First run (2026-08-22): no. "The Add Client form takes a name and an email, so at minute five the paper folder survives the move." **This run: yes.** Nadia's whole folder went in: the name on intake's one question, the due date on the Start work form, email, phone and address from her record one question at a time, and the hospital, the partner's name and a page of notes as three questions Maya added to her own Client Fields. All of it is on one record. What it cost is the stage's Decisions budget (TB-G11), and that the Practice's own questions live under Settings, not on the record she is filling in.

**The Contract comes back signed without leaving the app?** First run: not until the addendum stood up an object store, and then yes, with every merge field typed by hand (MO-G10). **This run: yes, on a reload.** The Draft arrived with the Practice name, the Client name and the price filled in, Nadia signed it in the portal from the emailed invite, and Maya's reloaded Engagement page read "Signed on Oct 27, 2026", "Signed by Nadia Haddad", with the PDF. An Engagement page already open does not change by itself.

#### Is the journey completable end to end?

**Yes, once stage 7 was finished on 2026-10-28** (the addendum below). Against "a pass means", Nadia now carries every item: a signed Contract, a filled Birth Plan she can read in the portal, two Visits, an open thread with a message each way, and an Invoice, raised on her Engagement and paid. Two caveats. Stripe's hosted onboarding was finished in the founder's own Chrome, because Stripe challenges an automation-launched browser, and the founder typed what only the account holder may: the Stripe login and two-step code, the SSN, and the two agreements. Nadia paid on Stripe's hosted Invoice page, because on this stack the portal's own pay control is refused for want of a publishable key (the addendum says why that is the stack, not the product).

On the first pass (2026-10-27) it was **not completable, and stopped at 7.1.** Against "a pass means": Nadia carries a signed Contract, a filled Birth Plan she can read in the portal, two Visits, and an open thread with a message each way, all reached by one person with no help. The Invoice is missing. Stripe challenged this browser at the first step of hosted onboarding, and the account the product makes cannot be finished from the platform's API, so neither 7.1 nor 7.2 could finish. No product decision is missing: the first walk (2026-08-22, #247) raised and paid an Invoice on a Practice onboarded in a person's own Chrome, and the account here waits on **Continue Stripe onboarding**. Every stage before stage 7, and stage 8, is completable today; the first run's three `missing-feature` steps are all walked.

**Findings filed**: [MO-G12 (#2193)](https://github.com/markgoho/doula-cloud/issues/2193), [MO-G13 (#2194)](https://github.com/markgoho/doula-cloud/issues/2194), [MO-G14 (#2195)](https://github.com/markgoho/doula-cloud/issues/2195), [MO-G15 (#2196)](https://github.com/markgoho/doula-cloud/issues/2196) and [MO-G16 (#2197)](https://github.com/markgoho/doula-cloud/issues/2197), each on the [journey map](../journeys/solo-birth-doula.md#gaps-found) with its budget line quoted; and four bugs, [#2198](https://github.com/markgoho/doula-cloud/issues/2198), [#2199](https://github.com/markgoho/doula-cloud/issues/2199), [#2200](https://github.com/markgoho/doula-cloud/issues/2200) and [#2201](https://github.com/markgoho/doula-cloud/issues/2201). Cited, not filed: TB-G8, TB-G9 and TB-G11, which own the same signup and Start work screens, and [#2186](https://github.com/markgoho/doula-cloud/issues/2186).

#### Addendum, 2026-10-28 — stage 7 finished

**Stack**: not the offset-1 stack above, which an Ubuntu upgrade wiped. Maya's Practice was rebuilt on the shared walk stack (`bun run --cwd app dev:full` from the main checkout, port offset 0: the app at `http://localhost:5173`, the BFF at `127.0.0.1:18080`, the mail catcher at `http://127.0.0.1:19080`), through the BFF and the Auth emulator, as precondition setup and not as a walk. The ids and state are in the "Rebuilt on the shared walk stack" comment on #2171. That rebuild answered the website question (`https://okonkwobirthsupport.com`) and left **Getting paid** at "Not connected". `scripts/stripe-listen.sh` ran against it, and its secret matched `app/.env.local`. Stripe ran in the Sandbox, with a test-mode key.

**Device**: 7.1 in the founder's own Chrome on Windows, driven through Claude in Chrome, which reaches the WSL stack on `localhost`. 7.2 and Nadia's payment in headless Chrome inside WSL, driven by the chrome-devtools MCP, one isolated context for Maya and another for Nadia (signed in by a magic link from the catcher). Both headless contexts run in UTC.

**How 7.1 was finished.** The orchestrating agent pressed **Connect Stripe** and filled the screens that hold nothing sensitive. The founder did the parts only the account holder may do: the Stripe login, its password and two-step code, the SSN's last four, Link's **Agree and continue**, and the final **Agree and submit**. Stripe's screens, in order:

1. Stripe login: email, a password of 10 or more characters, then two-step authentication. It has to be the authenticator app: a fictional 555 number cannot receive a text, and `000000` is refused there.
2. Business type: Unregistered business; EIN: No.
3. Personal details: Maya Okonkwo, DOB 01/01/1990, street `address_full_match`, Rochester, New York, 14604, phone (201) 555-0123, SSN last 4 `0000`.
4. Business details: the website prefilled from the Practice, and a short product description.
5. Payout account: the "Test (Non-OAuth)" institution, Link's **Agree and continue**, **Connect accounts**, **Finish without saving**, **Done**, then the Success account ••6789, Manual schedule, **Continue**.
6. Public details: prefilled, **Continue**.
7. Radar: the Lite link did not respond, so **Continue with Pro**, which is free in test mode.
8. Climate: **No thanks**.
9. Stripe Tax: **Not right now**.
10. Review: **Agree and submit**.

Back on `?connect=return`, **Getting paid** read "Awaiting Stripe review" for about 40 seconds while Stripe verified, then the account `acct_1UVJsh1rKouyfdrJ` went `active` with no requirements, around 00:00 UTC on 2026-10-28.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 7.1 | `manual` (was `blocked`) | **re-marked**, as the plan claims, cell corrected | **Getting paid** reads "Stripe status: Taking payments" and "Clients can pay their invoices, and payouts reach this Practice's bank.", with **Check status again**. `GET .../payments/connect` answers `active`, `cardPaymentsStatus` and `payoutsStatus` `active`, `requirementsDue` empty. **The webhook wrote it**: `practices.stripe_connect_status_event_id` holds `evt_test_65VeiFK6…`, a thin `capability_status_updated` event, and the Activity log holds `stripe_connect_form_returned`. Billing mode still read "Not chosen yet. You are asked the first time an Invoice is raised." No Stripe challenge on any screen in the founder's own Chrome |
| 7.2 | `manual` (was `blocked`) | **re-marked**, as the plan claims, cell corrected | On Nadia's Engagement, **Invoices** read "No Invoices yet." and **Create Invoice**. The press opened **Create an Invoice**, "How does this Practice bill Clients?", "You answer this one time. A Practice Owner can change it later in Settings.", **By card, through Stripe.** ("DoulaCloud sends the bill and the Client pays by card.") or **By hand.**, then **Create Invoice** and **Cancel**. No amount is asked: the Invoice takes the Contract's $1,800. **By card** then **Create Invoice**: "Invoice created." and a row `7ETF7DK3-0001`, Raised Oct 27, 2026, $1,800.00, **Open**, with **Record payment**. Stored: an `invoices` row `open`, `180000`, due 2026-11-27 00:03 UTC, on Stripe Invoice `in_1UVKL81rKouyfdrJ…` on the connected account (`collection_method: send_invoice`); the Practice's billing mode is now `stripe`; Activity holds `billing_mode_changed` and `invoice_raised` |

**The payment round trip.** Nadia's portal shows the Invoice: "What you still owe", Total to pay $1,800.00, `7ETF7DK3-0001` "Not yet paid". She paid it on Stripe's hosted Invoice page, the link Stripe mails her (Stripe sends no mail from a Sandbox, so the walk read the `hosted_invoice_url` from the Invoice). The page reads "Okonkwo Birth Support", Sandbox, $1,800.00, To Nadia Haddad, From Okonkwo Birth Support (the `display_name` fix from the first walk holds). Card `4242 4242 4242 4242`, 12/30, CVC 123, ZIP 14620, Link's "Save my information" unchecked, **Pay**: "Invoice paid", Visa •••• 4242. No challenge at any point. Within a second:

- `invoice.paid` (`evt_1UVKQo1rKouyfdrJ…`, from the connected account) reached our webhook and was recorded in `stripe_webhook_events`;
- `invoices.status` went to `paid`, `paid_at` 00:09:30 UTC, and exactly one `payments` row was written, `stripe`, `180000`, with `stripe_payment_reference` `pi_3UVKL91rKouyfdrJ…` (not empty, so the first walk's second fix holds too); Activity holds `invoice_paid`;
- **Maya's side**, on a reload: the Engagement's row reads **Paid**, Paid Oct 27, 2026, with **Return money**; the Practice's **Invoices** list reads Clients owe $0.00, Paid $1,800.00, and the row Nadia Haddad, Paid, Billed via Stripe, Payment due Nov 26, 2026, Paid Oct 27, 2026;
- **Nadia's side**, on a reload: Total to pay $0.00, "You have paid every Invoice for this care.", and under "What you have paid" `7ETF7DK3-0001` $1,800.00 **Paid**.

**What the portal's own pay control does on this stack.** Opened from the portal, the Invoice page said "Your Practice cannot take this payment yet." `GET /api/portal/.../invoices/{id}/payment` answered `409` `FAILED_PRECONDITION`. The account was `active`, so that is the handler's other branch (`api/internal/payments/client_pay.go`): the BFF has no `STRIPE_PUBLISHABLE_KEY`, so the Payment Element cannot be built. `app/.env.local` on the main checkout has no such line, and [`docs/environment.md`](../environment.md) says it belongs there as `pk_test_…`. That is this stack's setup, not the product, so nothing is filed for it; the deployed BFF's version of the same absence is [#1572](https://github.com/markgoho/doula-cloud/issues/1572). Adding the line needs a BFF restart, which this walk did not do.

**Two days for one Invoice.** Raised at 00:03 UTC, which is 8:03pm on Oct 27 in the Practice's zone: Maya's pages print Raised Oct 27 (the Practice-zone `raisedOn`), and Nadia's portal prints "Sent Oct 28, 2026", the instant read in her browser's zone. Filed as [#2205](https://github.com/markgoho/doula-cloud/issues/2205), the raised-day sibling of what [#1965](https://github.com/markgoho/doula-cloud/issues/1965) settled for the paid and due days.

**Time, recorded but not budgeted.** **Create Invoice** settled in 3,015 ms and showed busy within 7 ms: it creates and finalizes the Invoice on Stripe before it answers. Stage 7 carries no Time budget.

| Step | Budget | Measured | Met |
| --- | --- | --- | --- |
| 7.B-screens | 3 | **3 on this run**: **Getting paid**, Stripe's pages (not counted), **Getting paid** on `?connect=return`, the Engagement's Invoice. The website question had been answered in the rebuild. **5 from a cold start**, with the first pass's **Your website** detour and the return to **Getting paid** | **no** from cold, [MO-G15](https://github.com/markgoho/doula-cloud/issues/2196), extended to quote this budget line: the detour is the one MO-G15 already names |
| 7.B-decisions | 3 | 4 on the first **Create an Invoice**, as the first pass found; **Getting paid** 3 | **no**, [MO-G15](https://github.com/markgoho/doula-cloud/issues/2196), unchanged |
| 7.B-confirmation | yes | yes. "Invoice created." and the row with its number, day, amount and **Open**; once paid, **Paid** on the Engagement and on the Invoices list | yes |
| 7.B-cold | yes | yes. The billing-mode question says what each answer means and that it is asked once | yes |

These replace the first pass's "not measurable" rows above. **Recount**: 7.1 and 7.2 move from `blocked` to `manual`, so the Marks summary is 26 / 54 / 0 / 0, and [README.md](README.md)'s row and Total move to 116 / 403 / 0 / 47. The moment of truth is unchanged.

**Findings**: [#2205](https://github.com/markgoho/doula-cloud/issues/2205) filed; [MO-G15](https://github.com/markgoho/doula-cloud/issues/2196) extended to the Screens line. [#2200](https://github.com/markgoho/doula-cloud/issues/2200) still stands, for a Practice that has not connected.

### 2026-10-27 — automated marks re-derived for the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329))

A desk pass, not a walk: every `automated` and `manual` step was read against the current `app/e2e/` suite before the second walk.

- **3.3** moves from `automated (birth-plan.e2e.ts)` to `manual`. `birth-plan.e2e.ts` now opens the Engagement by its URL, not from the Clients list, and no spec asserts the six sections this cell names. `add-client-visits.e2e.ts` does open it from the Clients list, but asserts only the heading.
- **3.2-b** moves to `automated (add-client-visits.e2e.ts)`. The spec reads the Start work form's Credit sentence word for word, presses **Start work with {name}** as the founding Owner, and lands on the Engagement's page with "Work with {name} started."
- **5.3** and **5.4-a** move to `automated (contract-lifecycle.e2e.ts)`. The spec sends the Contract as the Owner and reads "Sent, waiting for the Client to sign", then signs it as the Client, reads **Signed**, and fetches the signed PDF from the store.
- **6.1** moves to `automated (add-client-visits.e2e.ts)`. The spec reads "No Visits yet." and the **Add a Visit** control on the Engagement's page.

No `missing-feature` step moved: re-marking those is the walk's act.

### 2026-10-23 — the route walked as a solo Owner, against the spec ([#1649](https://github.com/markgoho/doula-cloud/issues/1649))

`bun run dev:full` in `app/` on trunk at `6c1985d4`, walked in a fresh headless Chrome profile through `playwriter`, at 1280x900 and at 320x640, with a new throwaway account for each walk and **Practice name left empty**. Her stage 1 and the first Client of her stage 3 are the route that [`docs/onboarding-route.md`](../onboarding-route.md) specifies. No mark moved. One cell is rewritten: the note on 1.B-decisions, which named the four fields of the old form, now names the rule it measures. Renata Alvarez's plan has the agency Owner's walks and the step-by-step table for the same five screens, in its own Run log.

| Walk | Width | Acts | Presses | Screens | Exits from the app |
| --- | --- | --- | --- | --- | --- |
| Solo Owner, "Maya Okonkwo" | 1280px | 3 | 4 | 5 | 0 |
| Solo Owner, "Maya Okonkwo-Fairweather" | 320px | 3 | 4 | 5 | 0 |

**The count agrees with the spec**: 3 acts and 4 presses on 5 screens, with no exit from the app, and no ticket is filed for it.

- **1.2**: with Practice name empty, the heading was "Welcome to Maya Okonkwo" and "Welcome to Maya Okonkwo-Fairweather": her own name, first name then last name (#1536).
- **3.2-b**: "Who is the Doula?" opened with "Maya Okonkwo (you)" selected and "No Doula yet" after it, so the question added no press. The Engagement's page named her as the Doula, and its On call section listed her.
- **Sentences**: each sentence on the five screens agrees with the spec but two, and the spec was corrected for both because the product follows a recorded decision: the two links in the agreement sentence read "(opens in new tab)" after the document name (#1557), and the Start work form opens with "Pat saved as a Client." (#1710).
- **1.B-screens**: 2, passes. **1.B-decisions**: 5 at most, passes; at 1280x900 the form shows five of its seven fields at once, and **Create Practice** is below them. **3.B-screens**: 4 from the empty Practice to the Engagement's page, under the budget of 5. **1.B-cold** and **3.B-cold**: yes, in a fresh profile with nothing explained first. The other budget steps of the two stages were not measured on this walk; they wait for [#329](https://github.com/markgoho/doula-cloud/issues/329).
- **Layout**: at 1280px screens 2 to 5 scroll sideways by 57 to 62px ([#1803](https://github.com/markgoho/doula-cloud/issues/1803)); at 320px no screen does. At 320px **Create Practice**, **Save and continue** and **Start work with Pat Client** are 41.25 to 41.3px tall ([#1805](https://github.com/markgoho/doula-cloud/issues/1805)), and **Add your first Client** is 23.3px ([#1828](https://github.com/markgoho/doula-cloud/issues/1828)). None is a gap, so no gap ID is minted.

### 2026-10-02 — budget steps added ([#1683](https://github.com/markgoho/doula-cloud/issues/1683))

A desk pass, not a walk. The journey map's stages 1, 2, 3, 4, 5, 6, 7, 8 each gained a **Budget**, and each budget line is a step here, last in its stage's table: **44 budget steps**, 8 `automated`, 36 `manual`. No existing step is re-marked and no cell is rewritten. Every `automated` budget step is a 320px line the continuum sweep or `portal-320.e2e.ts` already asserts; every `manual` one is measured on the second walk ([#329](https://github.com/markgoho/doula-cloud/issues/329)), and none has been measured yet. The Marks summary above and [README.md](README.md)'s run-status row move with them.

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s). Stack per [docs/testing/e2e.md](../testing/e2e.md) — Postgres in compose, the goose migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.3-a | `billing.e2e.ts` | pass |
| 2.1 | `plan-templates.e2e.ts` | pass |
| 2.2 | `plan-templates.e2e.ts` | pass |
| 2.3 | `plan-templates.e2e.ts` | pass |
| 2.4 | `contract-template.e2e.ts` | pass |
| 3.3 | `birth-plan.e2e.ts` | pass |
| 4.2 | `birth-plan.e2e.ts` | pass |
| 5.4 | `portal-invite-accept.e2e.ts` | pass |
| 8.2-a | `push-notification.e2e.ts` | pass |

**9 automated steps: all pass.**

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**. That is [#234](https://github.com/markgoho/doula-cloud/issues/234).

### 2026-09-10 — Marks summary recounted ([#685](https://github.com/markgoho/doula-cloud/issues/685))

No step is re-marked and no cell is rewritten here. The `manual` count above read 16 against a Steps table holding 19, drift left by later tickets that added or moved a step without bringing the summary along. Recounted to 19, and [docs/test-plans/README.md](README.md)'s run-status row with it. This plan's own Add Client narrative is still stale — a two-field form, a Client and an Engagement in one request, a credit spent — and that correction is [#1121](https://github.com/markgoho/doula-cloud/issues/1121), deliberately not half-done here.

### 2026-09-10 — Add Client corrected against ADR-0017 ([#1121](https://github.com/markgoho/doula-cloud/issues/1121))

A desk pass, not a walk, over 3.1, 3.1-a, 3.2, 3.2-a and 3.4 — the cells [#685](https://github.com/markgoho/doula-cloud/issues/685) named and deliberately did not half-correct. Everything below was read out of the code.

| Cell | Was | Is, and what settled it |
| --- | --- | --- |
| 3.1 | "A form of exactly two fields, Name and Email" -> a door that redirects onto intake's first question | `clients/new`'s `load` redirects to the name question and carries the search's query string. ADR-0017, `#466`, and the intake routes |
| 3.1-a | `missing-feature (MO-G3)` -> `manual` | Phone, address and date of birth are their own questions; hospital, partner and notes are Practice-defined fields on the Client Field Template, which intake then asks as its own pages. [#252](https://github.com/markgoho/doula-cloud/issues/252) is closed and the step can be performed |
| 3.2 | Client **and** Engagement in one request, one credit consumed -> a free save of a Client alone, landing on her detail hub | `client.CreateHandler`'s own test asserts zero `engagements` and zero `credit_ledger` rows after a save |
| 3.2-a | `Credit balance: 2` and a consumption row -> `Credit balance: 3` and the `signup_bonus` row alone | Same test. Nothing debits the ledger on the intake path |
| 3.2-b | **new** — the act that creates the Engagement and locks the Credit | `RequestHandler` -> `collapse` for an Owner, then `approve` -> `billing.ConsumeCredit`. The form names `Credit cost` and `Balance after` first |
| 3.4 | The fourth **Client** returns `402` -> Clients are free and unlimited; the fourth **Engagement** is what is refused | `writeApproveErr`'s `402 no credits remaining, ask a practice owner or admin to buy more`, with the request transaction rolled back so no half-written Request survives. Reading the form for this cell also found the `Balance after -1` preview, filed as [#1235](https://github.com/markgoho/doula-cloud/issues/1235) rather than left in prose |

**One mark moved to `automated` and one to `manual`, and one step was appended.** 3.1 is `automated (add-client-visits.e2e.ts)` — the spec is the founding Owner, which is who Maya is, and it opens `clients/new` and asserts that the name question is what comes back, which is this step's Action and its whole Expected result. The Action names the URL the spec opens rather than the search that leads there, because claiming the search would be claiming coverage the spec does not have. 3.1-a is `manual` because its gap is closed. **3.2 is held at `manual` on purpose**: the spec asserts where the save lands and asserts nothing about the Engagement and ledger rows that do not appear, which is what that cell now claims — the [four marks](README.md#the-four-marks)' rule is that a spec counts only where it asserts the step's own result. 3.2-b and 3.4 are `manual` because no spec asks for an Engagement through the UI at all.

The Marks summary is recounted from the Steps table above — 13 / 20 / 0 / 3 — and [README.md](README.md)'s run-status row and Total move with it. MO-G3 leaves the `missing-feature` row; MO-G9 ([#257](https://github.com/markgoho/doula-cloud/issues/257)) is noted as closed where the summary observes it at 3.4.

`bun run test:e2e -- add-client-visits` in `app/`, one spec, one run: **1 passed** (51.5s), on the stack [docs/testing/e2e.md](../testing/e2e.md) describes. The re-marked step is the only one this pass moved onto a spec, so the whole suite was not re-run for a Markdown change; CI on the pull request is the run that covers the rest.

| Step | Spec | Result |
| --- | --- | --- |
| 3.1 | `add-client-visits.e2e.ts` | pass |

**3.2-b is appended to the map as well as to the plan**, for the reason ADR-0017 gives: one act became two, and a map whose stage 3 ran straight from the save to the Engagement would leave the plan's new step with nothing to sit beside. No gap ID on the map is touched, and no step is renumbered.

**Left alone on purpose.** The 2026-08-22 walk log below, including its verdict prose about a two-field form, is a record of what was seen on the day. Stages 4 to 8 are untouched: nothing in them turns on ADR-0017.


### 2026-09-03 — new automated steps ([#318](https://github.com/markgoho/doula-cloud/issues/318))

`bun run test:e2e` in `app/`, whole suite, one run: **30 passed, 0 failed** (29.0s).

| Step | Spec | Result |
| --- | --- | --- |
| 1.1 | `signup-form.e2e.ts` | pass |
| 1.2 | `signup-form.e2e.ts` | pass |
| 1.3 | `signup-form.e2e.ts` | pass |

**3 newly automated steps: all pass**, bringing the plan's total to 12. The 2026-08-22 manual walk below is unchanged as a historical record; it walked these three steps by hand before `signup-form.e2e.ts` existed.

### 2026-08-22 — manual walk ([#234](https://github.com/markgoho/doula-cloud/issues/234))

`bun run dev:full` in `app/`, walked in a desktop browser at 1280x900 as Maya Okonkwo, with a second 390x844 context for the Client. Preconditions: none, as the plan says — signed up from `/signup` with no fixture, twice (one pass for stages 1–8, a second to observe 4.2-a's template edit, 7.2's Invoice with an amount, and stage 8). The 9 `automated` steps were **not** re-run.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `Sign up your Practice`, four fields — `Practice name`, `Your name`, `Email`, `Password` — and **Create Practice**. Nothing else on the screen |
| 1.2 | `manual` | as expected | `POST /api/staff/signup` -> `201 {"staffId":…,"practiceId":…}`. One request, no confirmation step |
| 1.3 | `manual` | as expected | `Welcome to {practice name}` and seven links: Clients, Billing, Invite a Staff member, Staff, Plan Templates, Contract Template, Payments, plus `Sign out` |
| 3.1 | `manual` | as expected | `Add a Client`: exactly two fields, `Their name` and `Their email` |
| 3.1-a | `missing-feature (MO-G3)` [#252](https://github.com/markgoho/doula-cloud/issues/252) | as expected | Confirmed unwalkable. No due date, phone, address, hospital, partner, or notes field exists on the form or anywhere behind it. The paper folder cannot be transcribed |
| 3.2 | `manual` | as expected | `201 {"clientId":…,"engagementId":…,"status":"intake"}`, and the browser lands **straight on the Engagement page** — the Clients list is never passed through, as Tasha's walk also found |
| 3.2-a | `manual` | as expected | `Credit balance: 2`, ledger `consumption / -1` above `signup_bonus / +3`. Nothing on the Add Client screen had said a credit would be spent |
| 3.4 | `manual` | as expected | Clients 2 and 3 -> `201`. The fourth -> `402 no credits remaining, ask a Practice Owner to buy more`, printed verbatim under the form. Maya is the Owner it tells her to ask |
| 3.4-a | `manual` (was `blocked`) | **better than the plan claimed** | The Sandbox exists now ([#242](https://github.com/markgoho/doula-cloud/issues/242)), and the leg runs end to end. **Buy credits** with `Quantity 2` -> Stripe Checkout, `$10.00`, `Engagement credit`, `Qty 2, $5.00 each`. Paid with `4242 4242 4242 4242` -> back to `?checkout=success`, `Credit balance: 5`, ledger row `purchase / +2`. **The webhook is what credited it**: four events were delivered and all answered `200`, but only `checkout.session.completed` was recorded in `stripe_webhook_events` — `payment_intent.created`, `payment_intent.succeeded` and `charge.updated` were acknowledged and ignored, so the single purchase credited once. The 500 this step used to answer (`You did not provide an API key`) is gone |
| 4.1 | `manual` | as expected | **Create Care Plan** -> `201` with the seeded field set (Support People, pain management, requests, backup-doula checkbox). Filled, **Save Care Plan** -> `PUT … 200`, and the value survived a reload |
| 4.2-a | `manual` | as expected | Added a field to the **Birth Plan** template afterwards (`PUT .../plan-templates/birth_plan` -> `200`, `Saved.`), reopened the filled plan: the new field is **absent**. The Plan Instance snapshot holds, exactly as claimed |
| 5.1 | `manual` | as expected | `201 {"clientPortalUserId":…,"inviteToken":…}` and the screen says it plainly: `Invited. There is no email sending yet, so share this link with them directly:` followed by the raw URL in a `<code>` block. Maya must copy it out by hand (**[MO-G6](https://github.com/markgoho/doula-cloud/issues/255)** territory) |
| 5.2 | `manual` | **falsified** | `201`, status `draft`, prose snapshotted — but **no merge field is resolved**. The form shows six empty text inputs, `Practice name` and `Client name` among them, and the create handler sets `Values: MergeFieldValues{}` unconditionally (`api/internal/contracts/contract.go:124`). The product knows the Practice name and the Client name and asks her to retype both. New gap **[MO-G10](https://github.com/markgoho/doula-cloud/issues/258)** on the journey map |
| 5.3 | `manual` | as expected, with a second finding | **Send Contract** -> `200`, status `sent`. It sent with **every merge field blank** — no validation, no warning — and the Client's portal then rendered `This agreement is between  and  for doula services.` Folded into **[MO-G10](https://github.com/markgoho/doula-cloud/issues/258)** |
| 5.4-a | `blocked` (was `manual`) | **re-marked** | `POST /api/portal/engagements/{id}/contract/sign` -> `500 internal error`. Not a missing feature: signing renders the PDF and calls `store.Put` **before** the status write (`api/internal/contracts/sign.go:85-89`), and the walking stack points the object store at an unreachable host on purpose (`app/e2e/stack.ts:220`, `STORAGE_EMULATOR_HOST: 'storage-emulator-disabled.invalid:1'`). The bucket itself is real in the deployed service (`.github/workflows/ci.yml:516`). Infrastructure absent from the stack, not from the product |
| 5.5 | `blocked` (was `manual`) | **re-marked** | Unreachable while 5.4-a is. Maya's Engagement reads `Status: sent` |
| 5.5-a | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | as expected | Confirmed unwalkable. Every control on the Engagement page was enumerated: nothing anywhere reads or writes the Engagement's status. It shows `intake` in a read-only description list and stays there |
| 6.1 | `manual` | as expected | A `Visits` heading, an **Add a Visit** button, and a table of `Staff` / `Date` / `Reassign`, empty message `No Visits yet.` |
| 6.2 | `manual` | as expected | `201 {"visitId":…,"staffId":…}`, one row: `Maya Okonkwo / 8/22/2026`. No prompt, no form — the button *is* the Visit |
| 6.2-a | `missing-feature (MO-G1)` [#250](https://github.com/markgoho/doula-cloud/issues/250) | as expected | Confirmed unwalkable. The only input the Visits section carries is `Reassign to Staff id`, a free-text box for a UUID. No date, no time. The `Date` column renders `visit.createdAt` |
| 6.2-b | `missing-feature (MO-G2)` [#251](https://github.com/markgoho/doula-cloud/issues/251) | as expected | Confirmed unwalkable. No notes field anywhere in the section |
| 7.1 | `manual` (was `blocked`) | **better than the plan claimed** | Walked end to end on 2026-08-22 (#247). `Payments` reads `Not connected` with `Connect Stripe so Clients can pay their invoices.`; the button opens a real v2 Account Link, and Stripe's hosted flow (business type, personal details, business details, bank, payouts, public details, Radar, review) completes and returns to `?connect=return`. The screen then read `Awaiting Stripe review` — `card_payments: restricted`, `payouts: pending`, **no** requirements outstanding — and **hid** the onboarding button, since there was nothing to supply. Once the capability went active, `capability_status_updated` landed and the screen read `Active` / `Clients can pay their invoices and payouts reach your bank.` **The webhook is what wrote it**: `practices.stripe_connect_status_event_id` holds the delivering event id, not a poll. The old 500 here was the Stripe-401, and is gone |
| 7.2 | `manual` (was `blocked`) | **better than the plan claimed** | Walked 2026-08-22 (#247), on a Practice whose Connect account is active. `Amount (USD)` `1800` -> **Create Invoice** -> the Invoices section shows `$1,800.00 — Open`, and Stripe holds a finalized Invoice on the connected account. Paid on the hosted page with `4242 4242 4242 4242` -> `Invoice paid`, and `invoice.paid` flipped `invoices.status` to `paid` and created exactly one `payments` row for `180000`. **Two bugs fell out of this step**: the Client's invoice read `From DOULA.CLOU` (no `display_name` on the v2 Account, so Stripe fell back to the statement descriptor), and every `payments` row was written with an empty `stripe_payment_reference` (`payment_intent` no longer exists on an Invoice under `2026-07-29.dahlia`). Both fixed. The `connectRequired` gate this row used to record is still correct for a Practice that has *not* connected — it is just no longer what Maya hits |
| 7.2-a | `manual` | as expected | Side by side: **Billing** is `Credit balance: 0`, a ledger, `Quantity`, **Buy credits**. **Payments** is `Stripe Connect status: Not connected`, **Connect Stripe**. Neither screen carries one word about what it is for, or that the other exists. **[MO-G7](https://github.com/markgoho/doula-cloud/issues/256)** stands as written |
| 8.1 | `manual` | as expected | `201`, and the message renders as `Maya Okonkwo (staff) — 8/22/2026, 2:04:28 PM`. No edit and no delete control on the message, as claimed |
| 8.2 | `manual` | as expected | The Client replied from the portal (`201`, `senderType: "client"`), and Maya's reloaded thread showed both in order, staff then client, each labeled with its sender type |

**17 `manual` steps walked; 4 `missing-feature` steps confirmed unwalkable; 5 `blocked` steps attempted and their real responses recorded (2 re-marked from `manual`).** One expected result was falsified (5.2), minting **MO-G10** on the journey map. No `journey-gap` issue was filed — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means"**: **it does not pass, and one of the two reasons is not the product's fault.** One Client carries a filled Birth Plan the Client can read in the portal, a Visit, an open two-way message thread, and a Contract — all reached by one person with no help. The Contract is **not signed**, because the walking stack has no object store; that is the harness, and the deployed service has the bucket. The **Invoice does not exist**, and that is the product's own missing leg, blocked on a Stripe account nobody has opened.

Her moment of truth landed where the map put it. The Add Client form takes a name and an email, so at minute five the paper folder survives the move — and the walk found the same shape one stage later, in a Contract that asks her to retype the Practice name and the Client name the product already holds, and then lets her send it blank.

**For the walks behind this one** ([#235](https://github.com/markgoho/doula-cloud/issues/235)–[#241](https://github.com/markgoho/doula-cloud/issues/241)): every plan with a Contract-signing step meets the same 500. To walk it, run a fake GCS (`fsouza/fake-gcs-server`) and point `STORAGE_EMULATOR_HOST` at it in `app/e2e/stack.ts` instead of `storage-emulator-disabled.invalid:1`. That is test infrastructure, not a product change, and it belongs to [#209](https://github.com/markgoho/doula-cloud/issues/209) rather than to any one walk.

#### Addendum, same day — the two `blocked` steps walked

The walk above left 5.4-a and 5.5 `blocked` on an object store the stack did not have, and recommended standing one up. That was done the same day: `app/compose.e2e.yaml` gained a pinned `fsouza/fake-gcs-server`, `stack.ts` creates the bucket and points the BFF at it. Both steps went back to `manual` and were walked.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 5.4-a | `manual` (was `blocked`) | as the plan originally claimed | `POST /api/portal/engagements/{id}/contract/sign` -> `200`, `"status":"signed"`, and the portal screen changed from `Status: sent` to `Status: signed` and dropped the signing form. The signed PDF is really in the store: `contracts/{engagementId}/signed.pdf`, 1245 bytes, `application/pdf` |
| 5.5 | `manual` (was `blocked`) | as expected, plus one control the plan does not name | Maya's Engagement reads `Status: signed` without leaving the app, and the Contract section now offers **Void Contract** — the button Dee's plan walks at 7.1-a |

**Final marks for this plan: 9 `automated`, 19 `manual`, 3 `blocked` (all Stripe), 4 `missing-feature`.** The Contract leg of "a pass means" now passes end to end — invite, build, send, sign, and Maya sees it — and the only leg still short is the Invoice, which is blocked on a Stripe account nobody has opened.

One thing the fix made visible that the first walk could not: with the merge fields **filled**, the Client reads `This agreement is between Okonkwo Birth Support and Hannah Sorensen for doula services.` The prose resolves correctly the moment values exist — which is what makes **MO-G10** a gap about the product not filling in what it already knows, rather than a broken template.
