# The onboarding route: from `/signup` to First Value

This is the spec of the route that a new Owner walks from `/signup` to **First Value**, which is her first Client's Engagement existing ([ADR-0048](adr/0048-nothing-holds-a-practice-here-but-the-next-family.md)). It is the destination of the map [Practice onboarding: the route an Owner walks from signup to her first Engagement (#1487)](https://github.com/markgoho/doula-cloud/issues/1487).

This document decides nothing. Each sentence in it was decided on a ticket of that map or by a record that the ticket names, and each section says which. Where this document and a record disagree, the record is correct and this document has a defect.

**It describes the route as decided, not the product as built.** On 2026-10-02 the product does not match it. [The build tickets](#the-build-tickets) are the difference.

## The route in short

There is one route. It has five screens and four presses, and it is three acts.

| Act (ADR-0048) | Press | On the screen | She lands on |
| --- | --- | --- | --- |
| 1. Sign up | 1. **Create Practice** | 1. The signup form, `/signup` | 2. The empty Practice |
| 2. Add a Client | 2. **Add your first Client** | 2. The empty Practice | 3. The Client's name |
| | 3. **Save and continue** | 3. The Client's name | 4. The Start work form |
| 3. Start the Engagement | 4. **Start work with (name)** | 4. The Start work form | 5. The Engagement's page: First Value |

An act is one thing she sets out to do, which leaves something of hers changed. A press is one step inside an act: a button, a link, a message opened. The definition is ADR-0048's amendment on [When a new Owner verifies her email and enrolls a second factor, against ADR-0048's three acts (#1492)](https://github.com/markgoho/doula-cloud/issues/1492), and the count of four presses is its amendment on [What stands between the empty Practice and First Value, and what an act is (#1516)](https://github.com/markgoho/doula-cloud/issues/1516).

Five resolutions gave sentences to this spec. Each one is quoted as its resolution wrote it.

- From [The signup screen and the screens after it, as a prototype (#1496)](https://github.com/markgoho/doula-cloud/issues/1496): "the route from `/signup` to First Value is five screens and four presses, the same for a solo Owner and an agency Owner but for one answer, and the founder walked it."
- From [Who the Doula is on a first Engagement, and what an agency Owner does before it (#1515)](https://github.com/markgoho/doula-cloud/issues/1515): "the route is one route for a solo Owner and an agency Owner, and the Start work form is where the Doula is named or "No Doula yet" is said."
- From [What stands between the empty Practice and First Value, and what an act is (#1516)](https://github.com/markgoho/doula-cloud/issues/1516): "the route from the empty Practice to First Value is three presses on three screens; the search is in front of intake from the second Client on; the other details are added from her record and nothing demands them."
- From [When the product first asks an Owner to connect Stripe, and in what words (#1495)](https://github.com/markgoho/doula-cloud/issues/1495): "connecting Stripe is not on the route, and the first ask is the overview card after First Value."
- From [When a new Owner verifies her email and enrolls a second factor, against ADR-0048's three acts (#1492)](https://github.com/markgoho/doula-cloud/issues/1492), which named no sentence for the spec, so its answer in short is used: "The order for a new Owner: Create Practice, then her empty Practice. Verification and enrollment are not on the route to First Value. When she enrolls, the order is verify, then enroll." and "A second factor is required for five acts and recommended after three acts."

The founder walked the route as a prototype on 2026-10-02, as a solo Owner and as an agency Owner, at 320px and at a wide size. His words for it: "One thing".

## What the route does not have

Each line names the record that keeps the thing off the route.

- **No email verification and no second-factor enrollment.** Signup still sends the verification message, and nothing on the route waits for it (#1492, [ADR-0026](adr/0026-two-populations-two-sign-in-methods-and-one-token-table.md) amended).
- **No mention of Stripe.** No screen from `/signup` to First Value has the word (#1495).
- **No question that tells a solo Owner from an agency Owner**, on the signup form or on the empty Practice. The route does not divide (#1515).
- **No Invitation.** An agency Owner reaches First Value before a Doula accepts one, and the empty Practice has no link to the Invitation form (#1515, ADR-0048).
- **No search in front of the first Client.** A Practice that holds no Client record has nothing to find (#1516, [ADR-0017](adr/0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md) amended).
- **No question about the Client but her name.** The date of birth, the email address, the phone number, the address and the Practice's own sections are added afterward from her record (#1516).
- **No tour, no checklist, no welcome screen, no progress bar** (ADR-0048, [ADR-0046](adr/0046-the-product-counts-what-a-practice-did-and-never-who-she-is.md) P5).
- **No price.** The Credits screen is the one place that says what a Credit costs ([No price is published anywhere, inside the product or outside it (#285)](https://github.com/markgoho/doula-cloud/issues/285)).
- **No second password field and no checkbox** ([What the signup form asks: Practice name, her name, and the password rule (#1493)](https://github.com/markgoho/doula-cloud/issues/1493), [What a Practice agrees to at signup, and where the record is kept (#1494)](https://github.com/markgoho/doula-cloud/issues/1494)).
- **No marketing opt-in** ([ADR-0043](adr/0043-a-date-is-a-trigger-only-when-someone-set-it.md), [ADR-0044](adr/0044-the-founder-goes-where-doulas-already-gather-and-the-site-is-arrived-at.md)).
- **No message at signup but the verification message.** The one later message is the fourteen-day message to a Practice that started nothing ([One message to a Practice that signed up and started no Engagement, fourteen days after signup (#1381)](https://github.com/markgoho/doula-cloud/issues/1381)).

## The five things the map had to decide

The map's destination names five things. Each one has its answer here.

### 1. What the signup form asks

Her first name and last name, the Practice name (optional), the state she works from, the timezone, her email and a password. The fields, their order and their words are in [Screen 1](#screen-1-the-signup-form). Decided on #1493.

### 2. What she agrees to

At signup she agrees to the Terms of Service and the Privacy Policy, by one sentence directly above the button, with no checkbox. Decided on #1494; the record is [ADR-0053](adr/0053-a-practice-agrees-through-its-owner-and-a-change-that-matters-asks-again.md).

| Moment | What is agreed |
| --- | --- |
| Signup | The Terms of Service and the Privacy Policy |
| Connecting Stripe | Stripe's Connected Account Agreement, captured by Stripe in its own flow |
| Buying a Credit | Nothing new. The Terms disclose the price, so the price is agreed at signup |

- **The record** is one `activity` row ([ADR-0022](adr/0022-one-activity-log-with-a-subject-and-three-kinds-of-actor.md)) written in the signup transaction: subject the Practice, actor the Owner, action `terms_accepted`, and a diff with the version of the Terms, the version of the Privacy Policy, and the IP address from `clientip.From`. A version is the date a document takes effect.
- **No Practice signs up before the two documents are live** at `/terms` and `/privacy` on the marketing site, a pilot Practice included.
- **A material change after signup** sends each Owner an email 30 days or more before the date, then asks an Owner at her next sign-in. No other Staff member and no Client is stopped. This is after the route; ADR-0053 holds it.

### 3. When she verifies her email and enrolls a second factor

Not on the route. She is in her empty Practice immediately after **Create Practice**. Decided on #1492; the record is ADR-0026's amendment.

- **Being an Owner does not close the door of a Practice.** The Practice boundary refuses a person with no second factor for one reason only: the Practice has "require MFA for all staff" on.
- **A second factor is required for five acts**, and the BFF refuses each one without it: download the Practice archive, start Practice deletion, erase a Client, vouch for a Staff member who lost her second factor, and turn on "require MFA for all staff". None is on the route.
- **A second factor is recommended after three acts of hers**: First Value, her first Staff Invitation sent, and Stripe connected. The recommendation is one notice in the app and no email. It does not show on the empty Practice. She can close it, and it shows again one time at each new act on the list.
- **When she enrolls, the order is verify, then enroll.** Identity Platform refuses enrollment for an unverified email ([What Identity Platform requires of an account before it can enroll a second factor (#1489)](https://github.com/markgoho/doula-cloud/issues/1489)). She leaves the app two times, for her mailbox and for her authenticator app, and the two exits are off the route.
- **A sole Owner's recovery codes are made when she enrolls**, not at signup.

### 4. When and how she is first asked to connect Stripe

After First Value, by the "Getting paid" card on the Practice overview. The card shows only when the Practice has a Client, so the empty Practice does not have it. Decided on #1495. The words, for an Owner whose Practice has no Stripe account and no Billing mode:

> **Getting paid**
>
> Clients cannot pay you by card yet. To take card payments, connect Stripe. It takes about fifteen minutes, and then Stripe reviews your details. If you collect payment yourself, you do not need Stripe.
>
> [Set up card payments]

- The link opens the Getting paid screen, where the button named "Connect Stripe" is. There is no "Not connected" badge in this state.
- The second moment is the first Invoice's question "How does this Practice bill Clients?", which opens with no option selected. [No manual Payment recording (#271)](https://github.com/markgoho/doula-cloud/issues/271) decided that moment, and it does not move.
- A Practice that bills by hand does not see the card.
- Stripe's review gets no number until the pilot gives one. The Getting paid screen says that the first payout is typically 7 to 14 days after the first card payment.

### 5. What an agency Owner does before her first Engagement

Nothing that a solo Owner does not do. Decided on #1515; the records are the amendments to ADR-0017 and [ADR-0008](adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md).

- She sends no Invitation first. First Value is the Engagement existing, with or without a Doula on it.
- She gives one answer that a solo Owner does not give. On the Start work form, the question "Who is the Doula?" has no option selected when the Practice has more than one employee Doula, so she selects a Doula or "No Doula yet". A solo Owner finds her own name selected.
- The Offer or the Invitation that puts a different Doula on the Engagement is after First Value and is not on this route.

## The route, screen by screen

The words below are the words to build. Where a build ticket and this document differ, the build ticket's body is correct. "(name)" is the Client's name as the product shows it. The words of the prototype are in `app/src/routes/prototype-onboarding/fixtures.ts` on the branch `prototype/1496-signup-to-first-value`, which never lands on `trunk`.

### Screen 1: the signup form

`/signup`. The heading is "Sign up your Practice".

| # | Field | Words | Decided by |
| --- | --- | --- | --- |
| 1 | Her name, in two fields | Labels "First name" and "Last name", with `autocomplete` `given-name` and `family-name`. Refusals: "Enter your first name", "Enter your last name" | #1493 (Mark, Q2); build #1537 |
| 2 | Practice name | Label "Practice name (optional)". Hint: "If you leave this empty, your Practice takes your own name. Clients see this name on a Contract, in the portal and on a card statement." An empty field gives the Practice her own name, first name then last name | #1493 (Mark, Q1); build #1536 |
| 3 | The state she works from | Label "Which state do you work from?". Hint: "Sales tax on your practice's credits is worked out from where its team works — so this needs to be right, and needs updating if you move. We only need the state, not your address." Refusal: "Choose the state you work from" | [Record where each Staff member works, and require it at onboarding (#415)](https://github.com/markgoho/doula-cloud/issues/415); not reopened |
| 4 | Timezone | Label "What timezone does this Practice work in?". Hint: "DoulaCloud works out which day a Visit falls on in this timezone — which is what decides whether a Visit counts as birth or postpartum work." The field opens on the zone that the browser reports. Refusal: "Choose the timezone this Practice works in" | [A Practice's timezone cannot be set by anyone, so every Practice holds the column default (#1166)](https://github.com/markgoho/doula-cloud/issues/1166), [ADR-0036](adr/0036-a-practice-carries-one-timezone-and-it-decides-what-a-calendar-day-means.md); not reopened |
| 5 | Email | Label "Email". Refusal: "Enter your email address" | #1493; no change |
| 6 | Password | Label "Password". Hint: "Must be 15 characters or more, and not a commonly used password". A show and hide control. No composition rule. No second field. Refusals: "Enter a password", "Password must be 15 characters or more", "Password is too common. Enter a different password" | #1493 (Mark, Q3), ADR-0026 amended; build #1538 and #1539 |
| 7 | The role notice | "This account will hold every role — Owner, Admin and Doula — because a Practice's founder is usually its only Doula and needs to hold their own Visits. Anyone you invite later holds only the roles you choose for them. Roles are shown and changed on your Practice's staff roster." | [Signup grants Owner, Admin and Doula silently, and never says so (#290)](https://github.com/markgoho/doula-cloud/issues/290); stays before the button (#1493) |
| 8 | The agreement sentence | "By creating your Practice, you agree to the Terms of Service and the Privacy Policy." Each document name is a link that opens in a new tab. No checkbox | #1494 (Mark, Q2); build #1547 |
| 9 | The button | **Create Practice**. Its accessible description is the agreement sentence | #1494 (Mark, Q1) |

- **The order** is Mark's answer on #1493. Her name is first because an empty Practice name copies it. The error summary lists refusals in the order of the fields.
- **One departure from GOV.UK**: two name fields where GOV.UK prefers one. The reason is the one `docs/design/govuk-alignment.md` records for a Client: the product addresses a person by her first name. The row is written on #1537's commit.
- **The email is not sent to the BFF.** The browser gives the address and the password to Identity Platform, and the BFF reads the address from the verified ID token ([Staff signup writes staff.email from the request body, not from the verified token (#614)](https://github.com/markgoho/doula-cloud/issues/614)).
- **What the press writes**, in one transaction: the Practice, her Staff record, a Membership with the roles Owner, Admin and Doula and the Employment type `employee`, the Membership's `joined` record, three Credits (the signup bonus, which the Credits screen calls Welcome credits), the first record of her work state, the Practice's seeded templates, the verification message in the outbox, and the `terms_accepted` row. Each is her own act, so "how did this Practice come to be?" has an answer.
- **She lands on** the empty Practice. No screen is between.

### Screen 2: the empty Practice

The Practice overview, while the Practice holds no Client record. The heading is "Welcome to (Practice name)".

| Part | Words | Decided by |
| --- | --- | --- |
| The words | "Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, the visits with the Client, and the contract and invoices between the Client and your Practice." | #1515 (Mark, Q4); built by #1599 |
| The Credits sentence, as its own paragraph before the link | "Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has 3 Welcome credits." | #1516 under delegation; place confirmed on #1496 ("A: own paragraph before the link"); build #1612 |
| The one link | **Add your first Client**. It opens the name question, not the search | #1516, ADR-0017 amended; build #1609 |

- **The words are one set**, true for a solo Owner and for an agency Owner, because nothing asks her which she is before this screen.
- **The number is the balance at that moment.** "Welcome credits" is used only where the balance is the signup bonus and nothing else. In each other case the words are "N Credits". A person who cannot read the balance sees "Adding a Client is free. Starting work with a Client uses 1 of the Practice's Credits."
- **The screen has one action.** No Getting paid card, no Credits card, no second-factor notice, no link to the Invitation form (ADR-0048).
- **The test for "empty" is the count of Client records**, with the Clients that have no work and the erased Clients included. From the first Client on, the search is the only door to intake again.

### Screen 3: the Client's name

Intake for a new Client is one question.

| Part | Words | Decided by |
| --- | --- | --- |
| The question | "What is the Client's name?" Hint: "Only the given name is needed to save the record. The rest can be added at any time." | #1516 position 3; confirmed on #1496 ("Keep: name, work, details"); build #1611 |
| The fields | "Given name", "Family name (optional)", "Preferred name (optional)" with the hint "What the Client is called day to day, if different". Refusal: "Enter the Client's given name" | As built; "(optional)" is GOV.UK's Question pages pattern ([ADR-0021](adr/0021-govuk-is-the-reference-for-service-patterns.md)) |
| The one button | **Save and continue** | #1516 position 4; build #1611 |

- **The press saves the Client**, with the collision check, and opens the Start work form. The save is free and starts nothing. It writes the `client_events` `created` record.
- **No "Save and come back later"** and no later steps. The page draws no journey rail, so it does not say "Step 1 of 1".
- **The Back link** goes to the screen she came from.
- **This is the rule for each new Client**, not only the first. A second new Client is five presses from the Clients list: Find or add a Client, Search, Add a new Client, Save and continue, Start work.
- **The other details are added from her record and nothing demands them.** A link "Add (name)'s details" on the Client's record and on the Engagement's page opens one journey of question pages that saves as one edit (build #1610).
- **The cost that is accepted**: a save with a name only gives the collision check less to compare, so some duplicates are found when the details are added. [ADR-0040](adr/0040-a-client-record-merges-and-the-engagement-moves-with-her.md)'s merge then moves the Engagement.

### Screen 4: the Start work form

The heading and the button are "Start work with (name)".

| Part | Words | Decided by |
| --- | --- | --- |
| The Credits sentence | "Starting work with (name) uses 1 Credit. This Practice has 3 Welcome credits. After this, it has 2." | #1516 under delegation; the rule for "Welcome credits" settled on #1496; build #1612 |
| Kind of work | "Birth" or "Postpartum", with no option selected. Refusal: "Select whether this is birth or postpartum work" | As built (ADR-0017) |
| Due date | Needed for birth work. For postpartum work the hint is "Optional for postpartum work". Refusal: "Enter the due date" | As built |
| Who is the Doula? | The employee Doulas at the Practice, the person at the form first as "(her name) (you)", then "No Doula yet". Refusal: "Select who the Doula is, or select No Doula yet" | #1515 (Mark, Q1 and Q2); built by #1596 |
| Note | Label "Note (optional)", with no hint about an approver | [Start work tells an Owner to write a note for an approver who is herself (#1512)](https://github.com/markgoho/doula-cloud/issues/1512), a defect from the walk |
| The button | **Start work with (name)** | As built |
| The second action | **Go to (name)'s record without starting work** | Settled from ADR-0021 on #1496; build #1611 |

- **This is the one answer that differs between a solo Owner and an agency Owner.** Where the person at the form is the only Doula at the Practice, her option is selected when the form opens, and the question adds no press. With more than one Doula at the Practice, no answer is selected. The pre-selected answer is a recorded departure in `docs/design/govuk-alignment.md`.
- **Who is in the list.** Employee Doulas only. A contractor Doula is not in it, because only her acceptance of an Offer attaches her. A person with a pending Invitation is not in it, because she is not Staff until she accepts. The form names one Doula at most.
- **What the press writes**: the Engagement Request, asked and approved by her; the Engagement, with one Credit locked at approval; and, unless she said "No Doula yet", a granted Attachment with no fee that records her as the person who attached. Nobody offers herself work.
- **The model does not change.** The save of the Client and the start of the Engagement are two writes with two audit records. They are two acts in one flow.
- **The second action** leaves the Client saved and nothing started. Its words do not say "Cancel", because nothing is canceled.
- **A request that waits for an approver** has no Engagement, so that person lands on the Client's record with the pending block. An Owner is her own approver, so this is not on the route.

### Screen 5: the Engagement's page

First Value. The flow ends here (#1516 position 4; confirmed on #1496, "The Engagement's page"; build #1611).

- **A status message**: "Work with (name) started." The prototype showed these words and Mark asked for no change. Product copy has no gendered pronoun (`copy.pronoun.usage.spec.ts`), so the message cannot say "her".
- **The link "Add (name)'s details"** opens the details journey (build #1610).
- **The page's own sections** are as built. With "No Doula yet" on a birth, nobody is on call, because a birth Engagement has an On-call window only when it has a granted Attachment.
- **After this screen** the Practice is not empty. The overview has the Getting paid card ([section 4](#4-when-and-how-she-is-first-asked-to-connect-stripe)), and First Value is the first of the three acts after which the product recommends a second factor ([section 3](#3-when-she-verifies-her-email-and-enrolls-a-second-factor)).

### At 320px

The route holds at 320px with long, real content: no sideways scroll on any screen. Mark's answer on #1496 was "Holds; fix the two". The two:

- A long Doula name in "Who is the Doula?" drops below its circle ([RadioGroup draws the browser's small native circle, off-center from its label (#1518)](https://github.com/markgoho/doula-cloud/issues/1518)).
- A long Practice name pushes the empty Practice's one link below the first screen ([On the empty Practice a long Practice name pushes the one action below the first screen at 320px (#1632)](https://github.com/markgoho/doula-cloud/issues/1632)).

## When signup does not go through

The map kept "the signup paths that fail" as fog. This spec takes the patch in, because each path is built and each has a record, so there is nothing to decide. The decisions of the map change none of them.

| Path | What the product does | Record |
| --- | --- | --- |
| A field is empty or incorrect | The error summary lists each refusal in the order of the fields, and each field says what to do | [No form in the app recovers from a validation error: add the error summary, once (#467)](https://github.com/markgoho/doula-cloud/issues/467); the words are in Screen 1 |
| The address has an account already, and the password is not that account's | The Email field says "This email address already has an account. Log in instead." | #467 |
| The account was created and the Practice was not | The same form, sent again with the same address and password, signs her in to the account and finishes the signup. No second verification message is sent. Her name and her work state from the form replace the stored ones | [A failed signup strands the Identity Platform account, and sign-in has nothing to say to it (#745)](https://github.com/markgoho/doula-cloud/issues/745) |
| She has a Practice already | "This account already belongs to a Practice. Log in instead." No second Practice is made | #745 |
| A Client is signed in to the portal in the same browser | The form is replaced by the warning "Continuing signs you out of the client portal in this browser.", with the buttons **Continue and sign out** and **Cancel**. Nothing is created before she continues | [Staff signup and invitation acceptance still evict a live portal session silently (#816)](https://github.com/markgoho/doula-cloud/issues/816) |

Three notes for the builders, each inside a ticket that exists:

- A resumed signup makes a Practice, so it writes the `terms_accepted` row the same as a first signup (#1547: the row is in the same transaction as the Practice).
- A resumed signup replaces `staff.name` today. With two name columns it replaces the two (#1537).
- An empty Practice name on a resumed signup takes her own name, because the fallback is in the BFF (#1536).

One path was not in the map's list, and the decision on #1492 makes it matter: **an address that she typed incorrectly.** The form accepts it, and she is in her Practice with an address that is not hers. The BFF can change a Staff email and no screen asks for it. That is [A Staff member cannot correct her email address: the BFF can change it and no screen asks (#1650)](https://github.com/markgoho/doula-cloud/issues/1650).

## Facts about a pilot Practice

A pilot Practice walks the same route. There is no second signup form, and nothing in `/signup` knows that a Practice is a pilot (the map's charting answers; fact 6 of the walk on [Walk the real route from signup to First Value as a new Owner, and count every act (#1488)](https://github.com/markgoho/doula-cloud/issues/1488)).

- **The founding grant is an operator's call after signup**: `POST /api/internal/billing/founding-grants`, with the Practice's id and the grantor's name.
- **Its size is fixed at the moment it is issued**: three Credits for each Staff member then on the roster. A second grant is refused. An agency's grant is the correct size only if it is issued after its Doulas have accepted their Invitations.
- **The three Welcome credits from signup are separate**, so a solo pilot Practice holds six. After a founding grant the balance is not the signup bonus alone, so the two Credits sentences say "N Credits".
- **No pilot Practice signs up before `/terms` and `/privacy` are live** (#1494).
- The procedure has no document yet. That is [The founding grant has no written procedure, and its size depends on when it is issued (#1514)](https://github.com/markgoho/doula-cloud/issues/1514).

## The two patches that the map left as fog

The map's "Not yet specified" section held two patches when its last decision ticket closed. Each one has its answer here, and the section on the map is now empty.

| Patch | In or out | Reason |
| --- | --- | --- |
| The signup paths that fail | **In.** [When signup does not go through](#when-signup-does-not-go-through) | Each path is built under a closed ticket (#467, #745, #816), and no decision of the map changes one. There was nothing to decide, so no ticket is filed for the patch. One path that the patch did not name is filed as #1650 |
| The signup journey and test plan | **Out.** [An agency Owner's journey starts at /signup: Renata's map and plan gain the route, and the built route is walked against the spec (#1649)](https://github.com/markgoho/doula-cloud/issues/1649) | A journey map has an experience layer and a fixed structure, one for each Persona (`docs/journeys/README.md`), and a test plan step is "a claim about the product read out of the code" (`docs/test-plans/README.md`). This spec is neither. The work waits for the route to be built, so it is a ticket with blocking edges and not a section here |

The build tickets #1531, #1536, #1547 and #1611 each have a criterion that brings the journeys and plans that exist into agreement with the new form and the new intake. #1649 adds what none of them has: the agency Owner's stage, and a walk of the built route against this document.

## The build tickets

Each ticket that a decision of the map filed, with its state and its blocking edges as the tracker held them on 2026-10-02. The tracker is the truth for a state and an edge; this table is the index. "On the route" means that a screen from `/signup` to First Value does not match this document until the ticket lands.

**Each ticket is in the map's tree.** A ticket has one parent. The tickets of #1492, #1493, #1494, #1495 and #1515 are native sub-issues of the decision ticket that filed them, as each resolution recorded, and each decision ticket is a native sub-issue of the map. The other tickets are native sub-issues of the map.

### Tickets of the second-factor decision (#1492)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [A new Owner enters her Practice straight after signup: the Practice boundary no longer refuses an Owner for her role (#1531)](https://github.com/markgoho/doula-cloud/issues/1531) | Yes, screens 1 to 2 | Open | #1532 |
| [Five acts need a second factor: the archive, Practice deletion, Erasure, vouching, and the MFA switch (#1532)](https://github.com/markgoho/doula-cloud/issues/1532) | No, but it gates #1531 | Open | None |
| [The product recommends a second factor to an Owner after First Value, her first Invitation, and Stripe connected (#1533)](https://github.com/markgoho/doula-cloud/issues/1533) | No, after First Value | Open | #1531 |

### Tickets of the signup form decision (#1493)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [The signup form asks her name first, and an empty Practice name takes her own name (#1536)](https://github.com/markgoho/doula-cloud/issues/1536) | Yes, screen 1 | Open | None |
| [A Staff member has a first name and a last name: two fields at signup and at invitation acceptance (#1537)](https://github.com/markgoho/doula-cloud/issues/1537) | Yes, screen 1 | Open | None |
| [A Staff password is 15 characters or more, enforced by Identity Platform on every screen that sets one (#1538)](https://github.com/markgoho/doula-cloud/issues/1538) | Yes, screen 1 | Open | None |
| [A commonly used password is refused on every screen that sets one (#1539)](https://github.com/markgoho/doula-cloud/issues/1539) | Yes, screen 1 | Open | None |
| [An Owner cannot change her Practice's name: nothing updates it after signup (#1540)](https://github.com/markgoho/doula-cloud/issues/1540) | No, after signup | Open | None |

#1536 and #1537 change the same fields and have no edge between them by design: #1536's body says that her name is "one field until that ticket lands".

### Tickets of the agreement decision (#1494)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [The signup form carries the agreement sentence, and the Practice's agreement is recorded (#1547)](https://github.com/markgoho/doula-cloud/issues/1547) | Yes, screen 1 | Open | [Point doula.cloud at the Hosting site (#364)](https://github.com/markgoho/doula-cloud/issues/364), open. [Publish the Terms of Service and the Privacy Policy at /terms and /privacy (#1556)](https://github.com/markgoho/doula-cloud/issues/1556) is closed |
| [A material change to the Terms or the Privacy Policy sends each Owner an email 30 days before it starts (#1548)](https://github.com/markgoho/doula-cloud/issues/1548) | No | Open | #1547 |
| [After a material change starts, an Owner is asked to agree at her next sign-in, and nobody else is stopped (#1549)](https://github.com/markgoho/doula-cloud/issues/1549) | No | Open | #1547, #1548 |

#1494's resolution named the legal seed (#1505) as the blocker of #1547. The seed was closed when #1556 took the documents, and #1556 is closed. The edge that stays is #364, and it is the gate on the first pilot signup.

### Tickets of the Stripe decision (#1495)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [The overview's Getting paid card asks an Owner to connect Stripe, and a Practice that bills by hand does not see it (#1589)](https://github.com/markgoho/doula-cloud/issues/1589) | No, after First Value | Open | None |
| [The first Invoice's billing question opens with no option selected, and says what Stripe needs (#1590)](https://github.com/markgoho/doula-cloud/issues/1590) | No | Open | None |
| [The Getting paid screen says what comes after Stripe's form: a review, and a first payout in 7 to 14 days (#1592)](https://github.com/markgoho/doula-cloud/issues/1592) | No | Open | None |
| [Tell an Owner how long Stripe's review takes, from the pilot's own numbers (#1591)](https://github.com/markgoho/doula-cloud/issues/1591) | No | Open | #1589, #1592. It cannot start before pilot Practices have connected in live mode |

### Tickets of the Doula decision (#1515)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [The Start work form asks who the Doula is, and starting the Engagement attaches her (#1596)](https://github.com/markgoho/doula-cloud/issues/1596) | Yes, screen 4 | Closed | None |
| [The Clients list says "No Doula yet", not "No Doula assigned" (#1597)](https://github.com/markgoho/doula-cloud/issues/1597) | No | Closed | None |
| [Nobody offers herself work: the Offer list drops the sender, and an employee Doula puts herself on an Engagement directly (#1598)](https://github.com/markgoho/doula-cloud/issues/1598) | No, after First Value | Closed | #1596 and #1432, the two closed |
| [The empty Practice's words are true for an Owner who is not the Doula (#1599)](https://github.com/markgoho/doula-cloud/issues/1599) | Yes, screen 2 | Closed | None |

### Tickets of the intake decision (#1516), the prototype (#1496) and the walk (#1488)

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [An empty Practice opens the name question: no search while the Practice holds no Client (#1609)](https://github.com/markgoho/doula-cloud/issues/1609) | Yes, screens 2 to 3 | Open | None |
| [A Client's other details are added from her record: the question pages become one journey that saves as one edit (#1610)](https://github.com/markgoho/doula-cloud/issues/1610) | No, but it gates #1611 | Open | None |
| [A new Client is the name, then Start work: the save continues to the Start work form and the flow ends on the Engagement (#1611)](https://github.com/markgoho/doula-cloud/issues/1611) | Yes, screens 3 to 5 | Open | #1610, #1609, and #1596 (closed) |
| [She is told what uses a Credit, and how many the Practice has, before she starts (#1612)](https://github.com/markgoho/doula-cloud/issues/1612) | Yes, screens 2 and 4 | Open | #1599 and #1596, the two closed |
| [On the empty Practice a long Practice name pushes the one action below the first screen at 320px (#1632)](https://github.com/markgoho/doula-cloud/issues/1632) | Yes, screen 2 | Open | #1612 |
| [RadioGroup draws the browser's small native circle, off-center from its label (#1518)](https://github.com/markgoho/doula-cloud/issues/1518) | Yes, screen 4 | Open | None |
| [Start work tells an Owner to write a note for an approver who is herself (#1512)](https://github.com/markgoho/doula-cloud/issues/1512) | Yes, screen 4 | Open | None |

### The ticket of this spec

| Ticket | On the route | State | Blocked by |
| --- | --- | --- | --- |
| [An agency Owner's journey starts at /signup: Renata's map and plan gain the route, and the built route is walked against the spec (#1649)](https://github.com/markgoho/doula-cloud/issues/1649) | It checks the route | Open | #1531, #1536, #1537, #1538, #1539, #1547, #1609, #1611, #1612, #1632, #1518, #1512 |

### What the check of the tickets changed

The check for this spec, on 2026-10-02, read each ticket's parent and each blocking edge against the resolution that filed it. It made four changes:

- **One edge added: #1611 is blocked by #1609.** #1611 has the criterion "An e2e spec shows the first Client in three presses from the empty Practice to the Engagement's page", and its body says "The count of three presses needs the two tickets". The criterion cannot pass while the search is in front of the first Client.
- **One edge added: #1632 is blocked by #1612.** #1632 measures where the link "Add your first Client" starts, and #1612 puts one more sentence above that link. The measure is true only with the sentence on the screen.
- **Two links added: #1512 and #1518 are sub-issues of the map.** Each had no parent. Screen 4 does not match this document until the two land: #1512 owns the Note's hint, and #1518 owns one of "the two" that Mark asked to fix at 320px.
- **One ticket filed with its edges: #1649.**

No other edge was missing.

### The order that the edges give

The route matches this document when twelve open tickets land: #1531, #1536, #1537, #1538, #1539, #1547, #1609, #1611, #1612, #1632, #1518 and #1512. Three more must land first because they block one of the twelve: #1532 (for #1531), #1610 (for #1611) and #364 (for #1547). Then #1649 walks the result.

## Tickets near the route that the map does not own

Each of these was found on the map's work or stands beside the route. None is a sub-issue of the map, and none stops the route from matching this document.

| Ticket | Why it is near | Why the map does not own it |
| --- | --- | --- |
| [A new Owner is sent to enroll a second factor before she can have verified her email, and the refusal names no cause (#1504)](https://github.com/markgoho/doula-cloud/issues/1504) | The dead end that the walk found on the first screen | #1531 takes the enrollment screen off the route. The defect stays at each place where a person with an unverified email starts enrollment |
| [The second-factor enrollment screen explains none of its terms to an Owner who has never seen one (#1510)](https://github.com/markgoho/doula-cloud/issues/1510) | The screen that the recommendation (#1533) links to | The screen is off the route |
| [Nothing links to a new Engagement: the Client's page, the Clients list and the overview all name it and none opens it (#1511)](https://github.com/markgoho/doula-cloud/issues/1511) | #1611 meets its third criterion for an approved start | Its other criteria are screens off the route |
| [A full name typed in the Client search lands whole in Given name (#1513)](https://github.com/markgoho/doula-cloud/issues/1513) | It matters more when the name is the only question | The search is not in front of the first Client |
| [The founding grant has no written procedure, and its size depends on when it is issued (#1514)](https://github.com/markgoho/doula-cloud/issues/1514) | The pilot fact in this document | A runbook for an operator, after signup |
| [The Start work form demands a due date for a birth and the API does not: one rule, at the endpoint (#1614)](https://github.com/markgoho/doula-cloud/issues/1614) | Screen 4 | The form's words are correct; the endpoint's rule is the defect |
| [The Practice switcher gives a long Practice name too little width at a wide size (#1631)](https://github.com/markgoho/doula-cloud/issues/1631) | Found on the prototype | The shell, not the route |
| [StepRail's disclosure marker is drawn in the page gutter, and at 320px the page edge cuts it (#1633)](https://github.com/markgoho/doula-cloud/issues/1633) | Found on the prototype | The name question loses its rail (#1611) |
| [A Staff member cannot correct her email address: the BFF can change it and no screen asks (#1650)](https://github.com/markgoho/doula-cloud/issues/1650) | An address typed incorrectly at signup | The repair is on `/account`, after the route |
| [One message to a Practice that signed up and started no Engagement, fourteen days after signup (#1381)](https://github.com/markgoho/doula-cloud/issues/1381) | The one message after signup | Decided before the map; it can use the words of #1612 |
| [The January signup carries four hidden UTM fields, and the Practice records her source (#1393)](https://github.com/markgoho/doula-cloud/issues/1393) | It adds hidden fields to the signup form | How she gets to `/signup` is out of the map's scope |

## Sources

The decision tickets of the map, in the order they closed. Each resolution comment holds the detail that this document gists.

| Ticket | What it gave this document |
| --- | --- |
| [What Identity Platform requires of an account before it can enroll a second factor (#1489)](https://github.com/markgoho/doula-cloud/issues/1489) | Verification comes before enrollment |
| [What a platform must get a Practice's agreement to at signup, and what form counts (#1490)](https://github.com/markgoho/doula-cloud/issues/1490) | A sentence above the button is sufficient; what the record holds |
| [Walk the real route from signup to First Value as a new Owner, and count every act (#1488)](https://github.com/markgoho/doula-cloud/issues/1488) | The route as built on 2026-09-27: 5 acts, 17 presses, 2 exits from the app |
| [When a new Owner verifies her email and enrolls a second factor, against ADR-0048's three acts (#1492)](https://github.com/markgoho/doula-cloud/issues/1492) | Section 3; what an act is |
| [What the signup form asks: Practice name, her name, and the password rule (#1493)](https://github.com/markgoho/doula-cloud/issues/1493) | Screen 1 |
| [What a Practice agrees to at signup, and where the record is kept (#1494)](https://github.com/markgoho/doula-cloud/issues/1494) | Section 2 |
| [When the product first asks an Owner to connect Stripe, and in what words (#1495)](https://github.com/markgoho/doula-cloud/issues/1495) | Section 4 |
| [Who the Doula is on a first Engagement, and what an agency Owner does before it (#1515)](https://github.com/markgoho/doula-cloud/issues/1515) | Section 5; screens 2 and 4 |
| [What stands between the empty Practice and First Value, and what an act is (#1516)](https://github.com/markgoho/doula-cloud/issues/1516) | Screens 2 to 5; the count of presses |
| [The signup screen and the screens after it, as a prototype (#1496)](https://github.com/markgoho/doula-cloud/issues/1496) | Mark's walk of the route; the words that no ticket had |

The records that the decisions wrote or amended: ADR-0008, ADR-0017, ADR-0026, ADR-0033, ADR-0043, ADR-0048 and ADR-0053, and the `GLOSSARY.md` entries **Attachment**, **Billing mode**, **Credit** and **Engagement Request**.

`docs/design/govuk-alignment.md` is not changed by this document. Its rows change on the commit that builds each screen.
