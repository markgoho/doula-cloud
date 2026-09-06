# Six months of an agency's work

What arrives, when, and to whom — the counts, the calendar, and the things that go wrong. Settled on [#765](https://github.com/markgoho/doula-cloud/issues/765), under the map [Six months in a sandbox](https://github.com/markgoho/doula-cloud/issues/759).

Four files describe a run. [README.md](README.md) is the **instrument** — what a friction log is and what an entry must carry. [worlds/rooted-birth-collective.md](worlds/rooted-birth-collective.md) is the **World** — who is walking and what they arrive from. [environment.md](environment.md) is the **sandbox** — where the walking happens and what it cannot show. This file is the **calendar**: how much work there is, how it distributes across twenty-six simulated weeks, and what breaks.

## Every number here is an estimate, and it is labelled one

No real agency's book has ever been read. Every count below was reasoned from the World's fourteen-doula shape, from what `CONTEXT.md` says a Credit does, and from what the nine journey maps need in order to be walkable — not from an agency's records. Confirmed at the user's direction on 2026-09-05: the estimates stand, and they are corrected against reality once a real Practice is on the product, not before.

That is not a hedge, it is a constraint on how these numbers may be cited. A run's findings are about **what the product did when it held this much work**. They are never about how much work a doula agency has. `docs/personas/README.md` forbids citing the nine proto-personas as user research; the same line holds here, one level down.

## The parameters of a run

The map calls itself a standing harness with wall-clock as an input, so this file's job is as much to say **which knobs exist** as to say where run one sets them. A knob is something a later run changes without invalidating the comparison; everything else changes the World or the instrument, and a run that moves one is a new baseline rather than a delta ([README.md](README.md#comparing-one-run-against-the-next)).

### Knobs the harness accepts

| Parameter | What it sets | Run one |
| --- | --- | --- |
| `seed` | The RNG seed every distribution below draws from. Two runs with the same seed and the same knobs produce the same calendar. | Recorded in the run README |
| `span_weeks` | How long the run walks in simulated time. | 26 |
| `start_date` | The first simulated day. | Monday 4 January 2027 |
| `book` | Live Engagements at Rooted on day zero. | 58 |
| `walked_clients` | How many of the book a named cast member actually speaks to. The rest are the anonymous tail. | 15 |
| `inquiry_rate` | Engagement Requests arriving per simulated month, per Practice. | Rooted 35, Okonkwo 4 |
| `conversion` | Fraction of Requests that become Engagements. | 0.40 |
| `birth_rate` | Births per simulated month at Rooted, before the calendar redistributes them. | 14 |
| `postpartum_share` | Fraction of new Engagements whose kind is `postpartum`. | 0.45 |
| `visits_per_engagement` | Visits worked over an Engagement's life. | birth 5, postpartum 14 |
| `messages_per_engagement_month` | Messages in an Engagement's thread per simulated month. | 6 |
| `invoice_lag_days` | Days from an Invoice being sent to being paid. | median 12, long tail to 70 |
| `failure_rates` | The table in [What goes wrong](#what-goes-wrong-and-how-often). | As listed |
| `jump_schedule` | Where simulated time is fine-grained and where it is coarse. | [The jump schedule](#the-jump-schedule) |

### Fixed, and not a knob

The cast and the four Practices, including Okonkwo's three-week stagger and Ridgeline pre-dating the run — the World's, not this file's. Nadia's loss at 31 weeks, likewise. The 45-Credit founding grant, the 3-Credit signup bonus, and one Credit per Engagement — facts about the product, so a run that changes them is testing a different product. The 1 s and 10 s timing bands, and journey step ids — the instrument's, and moving either silently reclassifies every entry in every past run.

## Where the line between a walked act and a provisioned one actually falls

The World draws it at the person: *an act by a named cast member goes through the UI, and only the anonymous tail of the Client book is provisioned*. Read on its own that sentence sizes run one at several thousand acts, because Rooted's eleven Extras are named cast members and 210 inquiries over six months are acts somebody performs.

The World's own table is narrower than its headline, and the table is the operative rule. It walks four things: **every Invitation and acceptance**, **every Offer and answer**, **every act by one of the nine Personas**, and **every Client a Persona speaks to**. It does not walk an Extra's ordinary Tuesday.

So this file states the line at the act rather than at the person, which is a reading of the World rather than a departure from it:

> **A door is always walked. Routine work is walked only where a Persona is on it.**

An Extra's **door** acts — her Invitation, her acceptance, an Offer to her, her answer — go through the UI without exception, because those are the paths that produce a role-limited Doula or a contractor Attachment at all, and no other route creates one. An Extra's **routine** acts — the intake calls she takes, the Visits she logs for a Client nobody in the log is reading — are provisioned, along with the Clients they belong to. An Engagement Request for a provisioned Client is itself provisioned; only Requests for walked Clients are walked.

The run README states how many Clients, Requests and Invoices were provisioned rather than walked, so no later reader mistakes one for the other.

## The book on day zero

### Rooted: fifty-eight live Engagements, against forty-five Credits

The size is not chosen, it is forced. `CONTEXT.md` fixes that **one Credit covers one Engagement and locks when the Engagement is created**, and the World fixes that Renata must run out *while she is still moving in* — the paywall met mid-setup is one of the most consequential moments in the product, and it is unobservable if the grant covers the book. Fifteen Staff × three Credits is a founding grant of **45**. So the book has to exceed 45, and 58 is four Engagements per doula, which is what a fourteen-doula agency in its ninth year carries.

She therefore runs dry on the forty-sixth Engagement she moves in, with twelve still in her spreadsheet.

| | Count | Notes |
| --- | --- | --- |
| Live Engagements | 58 | 32 `birth`, 26 `postpartum` |
| Walked | 15 | Priya's four, Lena's three, Renata's own three, and Maya's five at Okonkwo |
| Provisioned | 43 | The anonymous tail. Its whole job is to make lists long and queries slow |
| Credits held | 45 | Founding grant. Nothing purchased yet |

**Due dates on the 32 birth Engagements** are not uniform, because a live book on day zero is a cross-section of pregnancies at every stage, not an intake cohort: 4 due in month 1 (already at term), 6 in month 2, 6 in month 3, 5 in month 4, 4 in month 5, 3 in month 6, and 4 falling after the run ends. A doula's list must therefore hold work that outlives the run, which is the ordinary case and has never been walked.

**The book is not spread evenly across the roster**, because the World says it is not. Rowan Petrosyan carries seven — she "carries the most Clients of anyone and is the first to feel a slow list", so hers is the list to time in month 6. Delia Marchetti carries three and no admin at all. Aditi Sundaram's are all postpartum and all nights. The remaining eleven doulas carry between two and five.

### Okonkwo: five Engagements, against three Credits

Maya's four-to-six Clients are five, two of them Personas (Hannah, Nadia). Her signup bonus is three Credits and she has no pilot grant behind her, so **she is stopped on her third Client, before she has finished moving in either** — the same wall as Renata's, met alone, with nobody to ask.

### Ridgeline and Bell & Ortiz

Ridgeline carries no book: Deborah Ridge created the Practice and invited Lena, and nothing else happens there. Bell & Ortiz has nothing to move in at all — Tasha arrives carrying a question, which is the contrast the World wants against Renata arriving carrying nine years.

## What flows through six months

At Rooted, 35 Engagement Requests a month at 40% conversion is **210 Requests and 84 new Engagements** over the run. Against 58 on day zero and roughly 50 Engagements ending inside the span, the book finishes near **90** — an agency that grew a third after it got software. That growth is deliberate: month six is where every list is longest, and a run that ends the size it started buys no performance reading it could not have taken on day one.

**The Credits arithmetic follows.** 142 Engagements over the run means 142 Credits, against a 45-Credit grant. Renata buys **three times** — once during move-in, once around month 3, once in month 5 — so the purchase path, the Stripe Checkout, and the New York sales-tax fraction that Yolanda Prieto's PA Work State creates are each walked more than once, on a balance that is not the same each time.

At Okonkwo, 4 Requests a month at the same conversion is 9 or 10 new Engagements, and Maya buys Credits twice.

**Visits and Messages** are what make an Engagement's detail page slow: 5 Visits over a birth Engagement's life against 14 over a postpartum one, and 6 Messages per Engagement per month. By month six, a two-year postpartum-heavy doula's list holds a few hundred Visits and a low four figures of Messages.

**Invoices** are sent as work completes, with a median of 12 days to payment and a long tail out to 70. That tail is the point: it is what puts Invoices past their due date while the clock is genuinely running, which is the one thing the nine single-instant walks structurally could not see.

## The shape of the six months

Twenty-six weeks, Monday 4 January to Friday 2 July 2027, in simulated time. (The run **id** is dated by the real day the run started, not by this — see [README.md](README.md#where-a-log-lives).) Totals alone would let a harness spread fourteen births evenly across four weeks and find nothing; birth work does not arrive evenly, and the weeks that hurt are the deliverable.

**Month 1 — the move-in.** Rooted's day zero is week 1. Renata is carrying fifteen people, a book of 58, and a business that does not stop while she moves it in. *The order she is forced into is the observation and this file does not script it* — what falls in the month is: fourteen Invitations sent and answered, the roles set on fourteen Memberships that arrive holding none, a Practice Page published because Rooted has no website, a Connect onboarding, the paywall at Engagement 46, and the first Credit purchase. The acceptances land **across the first two weeks, not one afternoon**: Kimiko Nakashima accepts in four minutes, Delia Marchetti takes nine days and needs the invitation sent again, and **Lena Vasquez's collides** — her address already holds a Ridgeline account, and `staff.identity_uid` is `UNIQUE`. Week 3 is Okonkwo's day zero, three weeks behind, so two first contacts are two events rather than one walked twice. Four births.

**Month 2 — the first ordinary month.** Six births. Month 1's Invoices go out, and the first of them passes its due date with the clock actually running. The first Offer goes to a contractor — Fern Okada, one overflow birth. The first inquiry vanishes after its Request is filed. **A Client's address is typed wrong and hard-bounces**, which writes an `email_suppressions` row that then silently refuses every later message to her; nobody at Rooted is told. Dee Whitlock is handed a cheque and has nowhere to record it.

**Month 3 — the week that hurts.** Week 11 carries **four births in five days**, two of them overlapping on one night, against a roster where three doulas are already working postpartum nights. One of the four is **three weeks early, at 02:00, and the assigned doula is already at another birth**. This is the week the run exists for, and it holds four of the five named probes. Renata's second Credit purchase. Mid-month, Tasha Bell signs up cold at `/signup` in fifteen minutes she does not have.

**Month 4 — the loss, and the departure.** Week 14: Nadia Haddad's pregnancy ends in stillbirth at 31 weeks. Also this month, **Bethany Kroll resigns with three open Engagements**, which have to be reassigned and whose Membership has to become something other than active. Camille Boyd calls Priya. Six births.

**Month 5 — money goes wrong.** An Invoice is disputed. A Client marks a notification as spam, suppressing herself, and her doula goes on believing she was told. A Client ghosts mid-Engagement and the enum has no state that describes her. **Kimiko Nakashima moves from employee to contractor** — a Membership change on a person with live work. Renata's third Credit purchase, on a roster whose New York share has changed. Five births.

**Month 6 — the book at its largest.** Around ninety Engagements, several hundred Visits, a few thousand Messages. Every list is at its longest, so this is where a timing means the most and where the 1–10 s band starts doing its work without an agent's opinion. Rowan Petrosyan's list is timed deliberately. Trish Halvorsen — one Client two years ago, never offered another — is looked at, so the run can say what a Practice sees when it looks at somebody it used once. The run closes on Renata reading her whole Practice: coverage, money, activity. Five births, and 4 due dates left standing after the run ends.

### Nadia Haddad's place in it

**Run week 14**, mid-April. That is arithmetic, not a choice: she is 20 weeks at Okonkwo's day zero, Okonkwo's day zero is run week 3, and 31 weeks is eleven weeks later.

Placing it there is what makes the rest of her journey walkable inside the span. Her eight stages then have twelve weeks: three weeks of not opening the portal (weeks 14–17), the portal read that is her moment of truth (week 17), the Birth Plan that will not go away (week 17), the Invoice and the word "voided" (week 18), postpartum support continuing anyway (weeks 17–22), and the record closing without erasing her (week 24). A loss placed any later pushes her Stage 8 past the end of the run, and Stage 8 is the one that asks whether `engagement_status` has anywhere to put her.

Between week 3 and week 14 she is an ordinary live Engagement — Visits, Messages, a Birth Plan filled in — which is what makes every automated prompt that fires at her afterwards a real hazard rather than a hypothetical one. She is walked first among the client-side Personas, per #203's standing rule that the harder person goes first.

### Camille Boyd books care that happens after the run ends

Camille is **not pregnant on day zero**, so no baby of hers can arrive inside twenty-six weeks, and a session must not try to make one. She calls Priya in **week 15**, newly pregnant, and books postpartum-only care against a due date beyond the run's span.

Every one of her stages happens at booking, not at care: the Practice types her in again because nothing about her was back-entered, she declares it postpartum-only, the second invitation refuses her because she already has a portal account, she ends up looking at two accounts for one person, she is offered a Birth Plan she does not need, and nothing came with her. "Nothing came with her" is the entry, and it lands the moment she is entered.

## The jump schedule

[#762](https://github.com/markgoho/doula-cloud/issues/762) measured that advancing 180 days costs the same 0.6 ms as advancing one, so **the number of jumps is the budget and the size of them is free**, and [#763](https://github.com/markgoho/doula-cloud/issues/763) made a run resumable, which turned granularity back into an observational choice. So the schedule is coarse where nothing is being watched and fine where something is:

| Stretch | Granularity | Jumps |
| --- | --- | --- |
| Weeks 1–2, the move-in and the acceptances | daily | 14 |
| Ordinary weeks (3–10, 12–13, 15–26, less the dense ones) | weekly | ~18 |
| Week 11, the four-birth week | hourly, at named hours only | ~24 |
| Week 14, the loss | hourly, at named hours only | ~16 |
| Inside a named probe | sub-hour | ~10 |
| **Total** | | **~82** |

Each jump costs one `UPDATE`, a drain of all thirteen `process-*` endpoints, and a lockstep Stripe advance — call it fifteen seconds. **Eighty-two jumps is about twenty minutes of the run's wall-clock**, against twelve to thirteen hours of acts. #762's conclusion holds: the clock is not what a run costs.

What the schedule does cost is sign-ins. A jump longer than twelve simulated hours expires every open session, because the Firebase ID-token verifier is hard-wired to real time. Thirty-two of the eighty-two jumps clear that line, and each costs a sign-in per persona who is active across it — roughly ninety acts, counted below. That is not waste: it is what a doula's morning looks like, and it exercises `sessionnotice.QueueNewSignInIfDue` about once a simulated day.

## The named probes

[#763](https://github.com/markgoho/doula-cloud/issues/763) interleaves the cast on one clock and reserves **genuine simultaneity for named probes** — two cast members acting on the same object at the same moment, written into the run as deliberate acts. Unbounded parallel personas is synthetic concurrency by another name, and that is out of scope. Five probes, each naming its object:

| | When | Who | The object they contend for |
| --- | --- | --- | --- |
| **P1 — Two labors, one night** | Week 11, Tuesday 02:00 | Priya Raman and Lena Vasquez at two births; Renata reading coverage | The Practice's coverage view, read while two Engagements change under it. This is Renata's own moment of truth, walked while it is actually true |
| **P2 — Reassigned twice** | Week 11, Wednesday 09:15 | Renata and Dee Whitlock | One Engagement's assignment. Both open it to move it off a doula who is already at a birth, and both submit |
| **P3 — One birth, two contractors** | Week 11, Thursday | Fern Okada and Yolanda Prieto | One overflow birth offered to both, accepted by both within a simulated minute. Whether that is one Offer or two is what the run finds out |
| **P4 — The paywall and the purchase** | Week 2 | Renata creating Engagement 46, Dee buying Credits | The Practice's Credit balance, at zero, from two sessions |
| **P5 — Both ends of a thread** | Week 16 | Hannah Sorensen and Maya Okonkwo | One Message thread, two posts in one simulated minute |

A probe is an ordinary sequence of entries in both personas' logs, cross-referenced by probe id. It is not a load test and it is not repeated to see if it flakes — it is walked once, and what happened is recorded.

## What goes wrong, and how often

A world where every Client converts, every Invoice is paid and every doula stays is not an agency, and it will find nothing.

| Failure | How often | Where | Why it is in run one |
| --- | --- | --- | --- |
| An inquiry vanishes after the Request is filed | 4 walked; the provisioned tail carries the rest of the 60% that does not convert | Rooted | A refusal is durable and carries a reason; what the Practice sees afterwards has never been walked |
| A Client ghosts mid-Engagement | 2 | Rooted, Okonkwo | `engagement_status` has no state that describes her — Nadia's problem in a milder form, and reached by a different road |
| A doula resigns with open work | 1 — Bethany Kroll, month 4, three open Engagements | Rooted | Reassignment at volume, and what a Membership becomes when the person leaves |
| A contractor used once and never again | Already seeded — Trish Halvorsen, looked at in month 6 | Rooted | An Attachment that ended without being deleted |
| Employment type changes mid-run | 1 — Kimiko Nakashima, employee to contractor, month 5 | Rooted | Fixed by the World. A Membership change on a person holding live work |
| An Invoice is disputed | 1, month 5 | Rooted | |
| An Invoice is simply never paid | 3 across the run | Rooted | Overdue behaviour over genuinely elapsed time is this map's headline claim |
| An Invoice is paid off-platform, by cheque | 1, month 2 | Rooted | Dee's known problem: money that arrives with nowhere to be recorded |
| A birth three weeks early, at 02:00, assigned doula elsewhere | 1 — week 11 | Rooted | P1 |
| Four births in five days | 1 — week 11 | Rooted | The week meant to hurt |
| A Client changes her mind about her plan after signing | 2 | Rooted, Okonkwo (Hannah) | Plan Instance edits after a Contract is signed |
| A pregnancy ends in loss | 1 — Nadia, week 14 | Okonkwo | Fixed by the World |
| A Client's address hard-bounces | 1, month 2 | Rooted | Confirmed into run one on 2026-09-05. The suppression row then refuses every later send, and nobody at the Practice is told |
| A Client marks a notification as spam | 1, month 5 | Rooted | Same loop, arrived at by the Client's own hand rather than by a typo |
| An identity collides on invitation | 1 — Lena, week 1 | Rooted | Fixed by the World |
| A Practice is abandoned mid-evaluation | Possibly 1 — Tasha, month 3 | Bell & Ortiz | **Not scripted.** She is the only cast member permitted to stop, and where she stops is the entry |

### Knowingly left out

- **Erasure.** Confirmed out of run one on 2026-09-05, agreeing with [#761](https://github.com/markgoho/doula-cloud/issues/761) rather than overriding it. It is the one act that ends a Client's observability for everything after it, so it costs a Client whose later journey somebody is reading. It belongs to a run that has something to erase.
- **A second loss.** One is the finding. A second is repeating the cruelest walk in the map for no new information.
- **A doula who is dismissed rather than resigning.** One departure walks reassignment and Membership end; a second buys a different reason, not a different path.
- **A Practice that hits the paywall and refuses to buy.** Both Practices that hit it, buy. A Practice that stops generating work spends months of run on one screen.
- **A Credit refund, TOTP across the cast, and a Connected account at Ridgeline or Bell & Ortiz.** All three are [#761](https://github.com/markgoho/doula-cloud/issues/761)'s, each with its reason recorded there.
- **Deliberate load testing.** The map's own out-of-scope. Observed slowness under this book is an ordinary friction entry with a number on it; synthetic concurrency beyond the five named probes is a different effort.

## What run one costs

[#762](https://github.com/markgoho/doula-cloud/issues/762) expressed a run's cost as `Σ (observed UI acts × seconds per act)` and said the volume ticket supplies the act count. Here it is, multiplied out so that cutting the book visibly cuts the number.

| Block | Acts |
| --- | --- |
| Rooted day zero: signup, Practice Page, Connect onboarding | 40 |
| 14 Invitations, sent | 56 |
| 14 acceptances — inbox, message, link, accept, first sign-in | 70 |
| Roles set on 14 Memberships that arrive holding none | 28 |
| 4 Offers, and their answers | 24 |
| 5 Credit purchases across two Practices, including the two paywalls | 36 |
| 15 walked day-zero Clients × (Client, Request, Engagement, Contract, backfilled Visits, Invoice) | 210 |
| 8 new walked Clients over six months, same shape | 112 |
| Three client-side portal journeys — Hannah, Nadia, Camille — walked across their arcs | 110 |
| Six Staff-side Persona journeys, stages re-walked over six months | 330 |
| Ongoing Visits, Messages, Invoices and Payments on walked Engagements | 260 |
| The sixteen failure modes | 120 |
| Five named probes | 40 |
| Sign-ins after the 32 jumps longer than 12 simulated hours | 90 |
| **Total** | **≈ 1,526** |

At roughly **30 seconds of agent wall-clock per act** — the Playwright interaction, the screenshot, the timing capture and the log entry — that is **twelve to thirteen hours**, plus about twenty minutes of jump overhead. A run is resumable, so that is sittings, not one sitting. Confirmed as acceptable on 2026-09-05.

**The lever is the book, never the roster.** Cutting the anonymous tail from 43 to 33 removes list length and nothing else; cutting doulas removes Invitations, Offers, Memberships and the whole reason Rooted exists. The floor on the book is 46 live Engagements, below which Renata never meets the paywall and the World stops being the World.

### What this costs at Stripe, for #780

A test clock holds at most three Customers and is deleted 30 real days after creation ([#762](https://github.com/markgoho/doula-cloud/issues/762)), so `clocks = ceil(Customers ÷ 3)`, per connected account. There is no ceiling on the number of clocks worth designing around: 80 were made on one connected account with no refusal, measured during #780's triage.

**The Customers are counted per Client, not per Engagement or per Invoice.** Since [#780](https://github.com/markgoho/doula-cloud/issues/780) a Client has at most one Stripe Customer per connected account, however many times she is billed. Before it, every Invoice raised its own, which is what made this sum look like a function of Engagements.

**The provisioned tail needs no Stripe Customer at all.** `invoices.stripe_customer_id` is nullable and `stripe_invoice_id` carries no live Stripe object behind it, and no persona ever observes a tail Invoice — in the product or in mail. So run one is sized by the **walked, invoiced Clients** only: about 23 across Rooted and Okonkwo, so **8 clocks**. That settles the question this section used to hand to #780.

Eight clocks issue their advances in parallel at about 0.4 s each on top of one roughly six-second wait, so a jump costs one wait rather than one per clock. **The client book sizes the clocks, and the Practice count does not.**

## What this file does not decide

- **How any of it is built** — the seeding, the provisioning path, the capture, the drain, the browser orchestration — is [Build the harness](https://github.com/markgoho/doula-cloud/issues/779)'s.
- **How a friction entry becomes a filed, deduplicated finding** is [From friction log to filed finding](https://github.com/markgoho/doula-cloud/issues/766)'s. This file produces the volume that makes deduplication necessary; it does not do it.
- **Who is in the World** is the [World](worlds/rooted-birth-collective.md)'s. Where this file names Bethany Kroll as the doula who resigns or Kimiko Nakashima as the Membership that changes, it is placing a person the World already put there on a date the World deliberately left open.
- **Whether Tasha finishes.** She is permitted to stop, and where she stops is a finding rather than a parameter.
