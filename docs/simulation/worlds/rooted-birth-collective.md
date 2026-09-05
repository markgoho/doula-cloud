# The world: Rooted Birth Collective

The seeded starting state that [run one](https://github.com/markgoho/doula-cloud/issues/759) walks — four Practices, the people in them, and the nine years of work that exists **outside** the product on day zero. Settled on [#761](https://github.com/markgoho/doula-cloud/issues/761).

Read this beside [the friction log format](../README.md), which says what a walk emits, and [the calendar](../calendar.md), which says how much work there is and when it arrives. This file says who is walking and what they walked into.

## What a World is, and what it is not

A **World** is a description of businesses that exist *in the world*, not a description of rows. Rooted Birth Collective is nine years old and none of it is in Doula Cloud. Its roster is in Renata's head and a shared calendar; its Contracts are on paper in a filing cabinet; the money it is owed is in a spreadsheet. **Day zero is the moment that agency meets the product** — Renata signs up into an empty tenancy and has to get nine years of reality into it while the work does not stop.

That is deliberate, and it is the single most expensive commitment in this map. Inserting the roster and the client book through SQL was rejected during charting because it skips the one path nobody has ever walked, and because it quietly assumes the answer to the question the run exists to ask: *can a real agency actually get itself in here?*

Three things follow, and each one is a rule a later session will be tempted to break:

- **Nothing in this file is a row.** Where it says a doula has been at Rooted seven years, that is a fact about the agency. Whether Doula Cloud can hold it is exactly what the run finds out — and where it cannot, that is a finding, not a defect in this document.
- **This World is an input, not a constant.** The map is a *standing harness*: the shape of the world, the size of the cast and the span of time are parameters. This file is the setting for run one. A later run may change it, and its run README says which run it supersedes.
- **The World is fiction, sized to reality.** It is not a portrait of any real agency, and no fact below was taken from one. It is sized to the pilot's 14-doula agency because `CLAUDE.md` makes performance under *a real Practice's data* a standing expectation, and a fixture-sized world cannot produce it.

## Walked, or provisioned

The map allows that background volume may be provisioned through the API, and warns that drawing that line generously is how this map produces nothing. Here is the line:

> **An act by a named cast member goes through the UI. Only the anonymous tail is provisioned.**

Concretely:

| Act | How | Why |
| --- | --- | --- |
| Every Invitation, and every acceptance of one | **Walked** | The invite-and-accept route is the only path that produces a role-limited Doula or a distinct Admin at all, it has never been walked at scale, and it has a known collision waiting in it — `staff.identity_uid` is `UNIQUE`, and `InviteHandler` always inserts a fresh `staff` row, so Lena joining Rooted on an account she already holds at Ridgeline is a live question, not a hypothetical. |
| Every Offer, and every answer to one | **Walked** | A contractor Doula's only door in. Four contractors at Rooted means it is walked repeatedly, by more than one person, on real work. |
| Every act by any of the nine Personas | **Walked** | The premise of the map. |
| Every Client a Persona speaks to, plans for, bills, or visits | **Walked**, from Engagement Request through Contract | These are the Clients whose records a friction log will cite. |
| Practice signup, Practice Page, Connect onboarding, Credit purchase | **Walked** | Each is a first-contact path and each is where a Practice can fail to start at all. Stripe's own hosted forms are driven with `playwriter` against real Chrome, per `docs/test-plans/connect-onboarding.md`. |
| The anonymous tail of the Client book — Clients nobody in the cast talks to | **Provisioned** | Their whole job is to make lists long and queries slow. Walking them buys nothing and costs the run its six months. |

A run README states how many of its Clients were provisioned rather than walked, so no later reader mistakes one for the other.

If day zero still costs too much, **the lever is the book, not the roster** — cut Clients, never fake an Invitation. The book is **58 live Engagements** at Rooted, set on [#765](https://github.com/markgoho/doula-cloud/issues/765) and read at [the calendar](../calendar.md#the-book-on-day-zero); that file also draws the line one level finer, at the act rather than the person — a door is always walked, routine work only where a Persona is on it.

## The four Practices

One Practice cannot show tenant isolation, and one Practice cannot show a person who works at two. Four exist, and each earns its place:

| Practice | Where | Shape | Why it exists | Day zero |
| --- | --- | --- | --- | --- |
| **Rooted Birth Collective** | Rochester, NY | 15 Staff, 14 of them holding the Doula role | The agency the map is about. The only Practice with enough people, enough Clients and enough money for a query to get slow. | The run's day zero. |
| **Okonkwo Birth Support** | Providence, RI | Maya Okonkwo alone, all three roles | Three of the nine Personas live here — Maya, Hannah, Nadia. A solo Practice is not a small agency; it is a different product, and Maya is the person for whom every default must be right the first time. | A second, staggered first-contact, three simulated weeks after Renata's. |
| **Ridgeline Doula Group** | Rochester, NY | One Owner, plus Lena Vasquez's contractor Membership | Lena's second agency, across town. **Without it, Lena is not Lena** — LV-G2 is her two-Practice problem, and a person holding two Memberships is unobservable in a one-Practice world. Nothing else happens here. | Already on the product before the run starts, because Lena's account has to pre-date her invitation to Rooted for the collision to be real. |
| **Bell & Ortiz Birth Services** | Tucson, AZ | Tasha Bell, two doulas on a spreadsheet | Tasha's evaluation. She is the only cast member allowed to abandon halfway, and where she stops is the finding. | Created mid-run, at `/signup`, cold. Her marketing-site leg is out of scope — the simulation starts where an agency meets the product, not where it hears about it. |

Ridgeline's Owner is the one cast member with no walk of her own at all: she exists so that Lena's Membership there has somebody who created it.

## The cast

`CONTEXT.md` fixes **Persona** at nine — a Persona has a journey map, a test plan, and a documented interior life. Everyone else in the World is an **Extra**: a person with a name, a Practice, a role, an employment type, and one reason to open the app, and nothing more. An Extra is a person, not a row — she is invited and she accepts, in a browser, like anybody else — but no friction log is written in her voice, and she has no journey to walk.

That distinction is the answer to "how thin may a cast member be": **thin enough to have no journey, never thin enough to skip the door.**

### The nine Personas

| Persona | Practice | Roles | Employment type | Where she is on day zero |
| --- | --- | --- | --- | --- |
| Renata Alvarez | Rooted | Owner, Doula | employee | Signing up. Everything else in this file is what she is carrying. |
| Dee Whitlock | Rooted | Admin | employee | Waiting for an invitation. They run the back office and will live in the app all day. |
| Priya Raman | Rooted | Doula | employee | Waiting for an invitation. Carrying four Clients, on a phone, in a hospital corridor. |
| Lena Vasquez | Rooted **and** Ridgeline | Doula | contractor | Already holds a Ridgeline account. Rooted's invitation arrives at an address that already exists. |
| Maya Okonkwo | Okonkwo Birth Support | Owner, Admin, Doula | employee | Three weeks behind Renata, signing up alone. |
| Tasha Bell | Bell & Ortiz | Owner, Admin, Doula | employee | Not yet in the product at all. |
| Hannah Sorensen | Client of Okonkwo | — | — | 18 weeks with her first, hired Maya after two interviews, on paper. |
| Nadia Haddad | Client of Okonkwo | — | — | 20 weeks, four months into an Engagement with Maya, signed Contract and filled Birth Plan — all of it on paper. |
| Camille Boyd | Client of Rooted | — | — | Not pregnant yet, and not in the product. Her first birth with Rooted was two years ago and is **not** back-entered. |

**Camille comes back as a stranger, on purpose.** The World is what the agency carries on day zero — live work, not finished work. A completed Engagement from two years ago is not live work, so Renata does not back-enter it. Camille's own need is "do not make me re-explain myself from scratch", and she returns to a Practice whose record of her does not exist. That is the honest test, and if it hurts, that is the finding.

### The Extras at Rooted

Rooted has **fifteen Staff**: Renata, Dee, Priya, Lena and eleven Extras. Fourteen of the fifteen hold the Doula role — everyone except Dee — which is what makes it "a fourteen-doula agency". Ten of the fourteen are employees and four are contractors.

| Extra | Role | Employment type | Work State | Who she is, in one line |
| --- | --- | --- | --- | --- |
| Joss Adeyemi | Doula | employee | NY | Seven years in, the person Renata trusts with a difficult birth. Opens the app to see who is on call tonight. |
| Marisol Terrazas | Doula | employee | NY | Six years. Splits birth and postpartum work and is the only person who does both well. |
| Bethany Kroll | Doula | employee | NY | Five years, left for eighteen months, came back. Half her Clients remember her and half do not. |
| Aditi Sundaram | Doula | employee | NY | Four years. Postpartum only, nights, and therefore never in the office. |
| Charlene Boateng | Doula | employee | NY | Three years. Fastest intake in the Practice and the one who notices a Contract has not come back. |
| Rowan Petrosyan | Doula | employee | NY | Two years. Carries the most Clients of anyone and is the first to feel a slow list. |
| Delia Marchetti | Doula | employee | NY | Nine years — the only person besides Renata who has been there from the start. Two births a month, no admin, no interest in software. |
| Kimiko Nakashima | Doula | employee | NY | Eleven months. The newest employee, and the one whose Membership Renata will change during the six months. |
| Fern Okada | Doula | contractor | NY | Takes three or four overflow births a year and turns down more than she takes. |
| Yolanda Prieto | Doula | contractor | PA | Works both sides of the state line. Her Work State is the reason Rooted's Credit purchase is not simply taxable in full. |
| Trish Halvorsen | Doula | contractor | NY | Took one Client from Rooted, two years ago, and has not been offered another since. She is the attachment that should have ended. |

Two of these are load-bearing beyond their names:

- **Yolanda Prieto's Work State is PA.** Sales tax on a Credit purchase is the Practice's New York-located Staff over all its Staff, so a roster that is uniformly New York makes the tax computation unobservable. Fourteen of fifteen is a fraction somebody has to render.
- **Trish Halvorsen has no live work.** A contracted job ends, and an Attachment ends without being deleted. She is in the World so the run can ask what a Practice sees when it looks at somebody it used once.

### The Extras elsewhere

| Extra | Practice | Role | Employment type | Why |
| --- | --- | --- | --- | --- |
| Deborah Ridge | Ridgeline | Owner, Doula | employee | Somebody has to have created Lena's Membership at Ridgeline. She does nothing else. |
| Sofia Ortiz | Bell & Ortiz | Doula | employee | Tasha's business partner, and the reason Tasha evaluates a *multi*-doula tool rather than a notebook. Instantiated only if Tasha gets far enough to invite her. |

Okonkwo Birth Support has no Extras. Maya is alone, and that is the point of her.

## Rooted Birth Collective, as it exists outside the product

Nine years old, Rochester, New York. Renata Alvarez started it alone in a rented room and now runs a business she can no longer hold in her head — which is the whole reason she is signing up.

**How the work is held today.** A shared Google Calendar that only Renata and Dee maintain. A filing cabinet of signed paper Contracts, one folder per Client, with the current year's folders on Dee's desk and everything older in a cupboard. A spreadsheet of who owes what, updated when Dee remembers. A group text thread for coverage, and Renata's phone for everything else. Nothing is wrong with any of it individually and all of it fails at once when two Clients labor on the same night.

**What the agency is carrying on day zero.** A live book of Clients, some of them mid-pregnancy with due dates inside the six months the run walks, some of them in postpartum care, some of them owing money on work already finished. Every one of them signed a paper Contract that Doula Cloud has never seen. Every one of them has a doula who has been on the job for weeks or months and who is not recorded anywhere the product can read. The counts, the due-date distribution and the birth-against-postpartum split are settled at [the calendar](../calendar.md#the-book-on-day-zero) ([#765](https://github.com/markgoho/doula-cloud/issues/765)): **58 live Engagements, 32 birth and 26 postpartum**, with 15 of them walked and 43 provisioned, and birth due dates spread across all six months and past the end of the run.

**What Renata is anxious about**, in her own order: coverage first — if two Clients go into labor tonight, who is free — then money, then whether her doulas will actually use the thing she is about to make them use. Nothing about software is in the first three.

**Its billing position.** Rooted joins as a **pilot** Practice, so it receives a founding grant of three Credits for each Staff member it has on the day it joins. Fifteen Staff is forty-five Credits, against a live book considerably larger than that, so **Renata runs out of Credits during day zero and has to buy more before she has finished moving her agency in**. That is not an accident of the numbers and it is not to be softened by granting extra: the moment a person hits a paywall while still setting up is one of the most consequential moments in this product, and it is unobservable if the grant covers the book. She has a Stripe Connected account to set up as well, and a website question to answer — Rooted has no website of its own, so it publishes a **Practice Page**.

**Nine years leaves residue**, and the residue is where the interesting failures live:

- Delia Marchetti has been there since the second year and has never used software for any of it.
- Bethany Kroll left for eighteen months and came back. Her Clients from before are finished; her Clients now are new. One person, two eras, one record.
- Trish Halvorsen took one Client two years ago and was never offered another. She is still, in Renata's head, "one of ours".
- Kimiko Nakashima was hired as an employee eleven months ago and wants to go contractor, which is a Membership change Renata makes during the six months, not on day zero.
- Camille Boyd was a Client two years ago and will be one again. Nothing carries across, because nothing about her was ever entered.

## Okonkwo Birth Support

Maya Okonkwo, alone, out of a spare room in Providence, Rhode Island. Six years certified, four to six birth Clients at a time, and a paper folder per Client that she stopped trusting when she lost an intake form last spring.

Her Practice enters the product **three simulated weeks after Renata's**, deliberately staggered so that two independent first contacts happen at different points in the run rather than being one event walked twice. She signs up cold at `/signup`, which grants all three roles in one statement — so Maya is never a test of role separation, and her walk must not be allowed to stand in for Priya's or Dee's.

She carries two live Clients into the product, both of them Personas:

- **Hannah Sorensen**, 18 weeks with her first, hired after two interviews. Her Contract is signed on paper; her birth preferences are a list in a notebook.
- **Nadia Haddad**, 20 weeks, four months into the Engagement, signed Contract, eleven Visits already worked, and a Birth Plan Maya filled in with her by hand.

Maya's day zero is small and complete, and it is the only place in the World where **every** default the product ships with — the seeded Plan Templates, the signup bonus of three Credits, the Practice Page — meets somebody with nobody to ask for help. Her four-to-six Client load against a three-Credit signup bonus means she, too, meets the paywall, and she meets it with no pilot grant behind her.

Nadia's pregnancy ends in stillbirth at 31 weeks. **It falls in run week 14**, settled at [the calendar](../calendar.md#nadia-haddads-place-in-it) ([#765](https://github.com/markgoho/doula-cloud/issues/765)) — arithmetic from the two facts this file fixes, and early enough that her remaining six stages fit inside the span. What this file fixes is that she is Maya's Client, she is in the product before it happens, and she is 20 weeks on day zero — so the run cannot place the loss outside its own span.

## Ridgeline Doula Group

Rochester, New York, across town from Rooted. Deborah Ridge owns it and Lena Vasquez contracts for it. It exists for one reason and does one thing.

It is **already on the product** before the run's day zero, because the collision this map wants to see requires Lena to hold an account first: Rooted's invitation must arrive at an address that Doula Cloud already knows. `InviteHandler` always inserts a fresh `staff` row and acceptance claims it by writing an identity onto it, while `staff.identity_uid` is `UNIQUE` — and the same migration's own comment says a person may work at more than one Practice. Both cannot be true. The run finds out which.

Beyond that, Ridgeline gives Lena somewhere else to be. She lands on the Practice picker more often than any other cast member, she must never see a Rooted Client from a Ridgeline session, and "what she is at each Practice" is two Memberships and must behave as two.

Deborah Ridge is walked exactly as far as creating the Practice and inviting Lena. She has no journey and no friction log.

## Bell & Ortiz Birth Services

Tucson, Arizona. Tasha Bell and Sofia Ortiz, two doulas on a spreadsheet and a shared Google Drive. It does not exist in Doula Cloud until Tasha creates it, mid-run, in fifteen minutes she does not really have, with three other tabs open.

Tasha's first leg — the marketing site — is out of scope for this map, so her walk starts at `/signup` cold, with none of Maya's motivation and none of Renata's commitment. She is the only cast member permitted to stop, and **where she stops is the entry**. Sofia Ortiz is instantiated only if Tasha gets as far as inviting her; if she does not, Sofia never exists, and that is a result rather than a gap in the World.

Her Practice is not seeded with any prior work at all. She has nothing to move in, which is the contrast the World needs against Renata: one agency that arrives carrying nine years, and one person who arrives carrying a question.

## Day zero

Day zero is a Practice's own, not the run's: Ridgeline is already in, Rooted's is the run's first simulated day, Okonkwo's is three weeks later, and Bell & Ortiz has one whenever Tasha arrives.

**The order Renata is forced into is itself a finding, so this file does not script it.** It states only what she is trying to do, which is what a walking session needs and all it should be given:

> Renata has fifteen people and a book of live Clients, and the work does not stop while she moves in. She wants, in this order of anxiety: her doulas able to see their own Clients; her Clients' Contracts and money somewhere she can look at without asking Dee; and never to lose a night's coverage to the transition.

What the product makes her do to get there — whether the roster must precede the book, whether a Client can exist before an Engagement, whether a doula can be put on work that predates her Membership, how many Credits in she discovers there are not enough — is the observation. A session that arrives with a prescribed sequence has answered the question it was sent to ask.

Two ordering facts are fixed, and only because the World would otherwise be incoherent:

- **Lena's Ridgeline account predates her Rooted invitation.** Without that the collision is not real.
- **Nadia and Hannah are Maya's Clients before Maya signs up.** Their care is already under way; the product is joining it late, which is the ordinary case for every Client in this World and the unusual case for a product that has only ever been walked from an empty fixture.

## The entity surface of run one

The smallest set that still produces real collisions is most of the model, because the collisions this map exists to find are between entities rather than inside them. What matters more is the short list at the bottom — what is knowingly left out.

**In**, because a Persona's journey touches it or because the World cannot be stated without it:

| | |
| --- | --- |
| Practice, Staff, Membership, Invitation | Fifteen invitations at Rooted, one collision expected, one Membership changed mid-run. |
| Employment type, Offer, Attachment | Four contractors at Rooted, an attachment that should have ended (Trish), and the whole of Lena's read rule. |
| Work State | Fourteen of fifteen in New York, which is what makes the tax share on a Credit purchase a real fraction. |
| Client, Client Field Template, Portal Account | Three client-side Personas, one of whom (Camille) reaches two Practices' worth of history that is not there. |
| Engagement Request, Engagement (kind, due date, status) | Every piece of live work moved in on day zero, and everything that arrives during six months. |
| Visit, Care Plan, Birth Plan, Plan Template, Plan Instance | Maya's seeded defaults, Renata's Practice-wide template edits, and Nadia's filled Birth Plan after a loss. |
| Contract, Invoice, Payment | Paper Contracts re-signed, money already owed, and Dee's cheque that has nowhere to be recorded. |
| Message, Notification, Email Suppression | Every Engagement's thread, and the mail that is the *only* way several journeys start. |
| Activity | `CLAUDE.md`'s standing expectation, and the only way a run can answer "how did this come to be?" about its own world. |
| Credit | The paywall Renata and Maya both meet, the pilot founding grant, and the purchase itself. |
| Connected account, Practice Page | Rooted and Okonkwo both need to be paid, and neither has a website. |

**Knowingly out of run one**, each for a stated reason:

- **Erasure.** It is the one act that leaves the record deliberately less complete, and it ends a Client's observability for everything after it. It belongs to a run that has something to erase. [#765](https://github.com/markgoho/doula-cloud/issues/765) took the question and confirmed the exclusion: erasing a Client costs the run everything she would have done afterwards.
- **A Credit refund.** Three-year window, nothing in six months forces it, and it would consume acts to observe a path no Persona wants.
- **TOTP multi-factor authentication.** Landed recently and worth a run, but it sits in front of every sign-in in the cast — turning it on for fifteen people is a harness problem wearing a domain costume. Run two.
- **A Connected account at Ridgeline or Bell & Ortiz.** Neither takes money in this World. Two Connect onboardings are enough to see the flow twice.
- **A person who is both Staff and a Client.** The model deliberately cannot join those two records, so nothing in a run can observe the fact — only the absence of it, which is already written down.

## What this file does not decide

Three boundaries, so the next session does not redo this one's work:

- **Counts, calendar and failure modes** belong to [Six months of an agency's work](https://github.com/markgoho/doula-cloud/issues/765): how many Clients are in the book, how many inquiries arrive a month, how births distribute across the weeks, when Nadia's loss falls, which doula quits, which Invoice goes unpaid. This file says *who*; that one says *how much, when, and what goes wrong*.
- **Where the sandbox runs**, and therefore what any of this costs to stand up, belongs to [Where the sandbox runs](https://github.com/markgoho/doula-cloud/issues/763).
- **How fifteen people receive fifteen invitations** belongs to [How a persona receives her email](https://github.com/markgoho/doula-cloud/issues/764). This file only makes the requirement unavoidable: every Membership in this World is created by an email somebody had to open.
