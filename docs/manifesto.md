# Doula Cloud manifesto

This is how Doula Cloud decides what to build, what to refuse, how the product treats the people using it, what to charge, how to reach the doulas it is for, what to measure, and where founder time goes. It distills three books and applies them to this product:

- **SP** — Rob Walling, *The SaaS Playbook* (2023).
- **SM** — Rob Walling, *Start Marketing the Day You Start Coding* (2023 edition; the essays date from 2006–2016).
- **NI** — Golden Krishna, *The Best Interface Is No Interface* (2015).

Page numbers are printed pages. Every figure from these books is a rule of thumb, never a target.

## How to use it

- Reach for the section that matches the decision in front of you. When a principle decides something on a ticket or a PR, cite it by its heading.
- Each principle has four parts. **Claim** is the book's idea. **Means here** is what it asks of this product. **Does not mean** is the misreading to avoid. **Broken when** is the signal that the principle was not followed; when a change would produce that signal, say so on the ticket.
- A principle is an input to an open decision ticket, never its answer. The ticket still decides.

## What wins

In order: Mark's own words; `CLAUDE.md`; recorded decisions (ADRs, `CONTEXT.md`, closed decision tickets); then this document. Where a principle meets a recorded decision, the principle names it under **Recorded**, and the recorded decision stands.

**The launch splits two rules.** Until the January 2027 launch, `CLAUDE.md`'s pre-launch rules hold: everything found is fixed, and nothing is ranked by severity. From launch on, the principles here that rank work, or that handle something by hand first, apply. Decided by Mark on 2026-09-10.

## Reading the books before launch

SP assumes some product-market fit, which it places at $10,000–$20,000 of monthly recurring revenue (SP p.39). At $20 a Credit that is 500–1,000 Engagements a month; the pilot runs 50–100 births a month (#439). So the pilot, October 2026 to January 2027, tests whether Practices want the product and pay for it. Before product-market fit, find out why a customer leaves instead of tuning the number (SP p.154).

## Subscription words, translated for Credits

Both books speak in subscriptions. Doula Cloud sells prepaid Credits, one per Engagement (#439), so every principle that measures reads against this table.

| Book term | Doula Cloud term |
| --- | --- |
| Monthly recurring revenue | **Credits consumed per month**, at the price paid. Not Credits purchased: a purchase is lumpy, and refundable for three years (pilot terms). |
| Growth rate | Month-over-month change in Credits consumed. |
| Revenue per account | Credits consumed per Practice per month, times price. A solo doula is about $40; a fourteen-doula agency about $333 (#439). |
| Annual contract value | Twelve months of the above. A solo doula is about $480; the agency about $4,000. |
| Expansion revenue | Built in. More births or more doulas consume more Credits, with no upgrade step. |
| Trial | Free Credits: `signup_bonus` for any new Practice, `founding_grant` for a pilot Practice (#439). Conversion is the first purchased Credit. |
| Churn | A **quiet** Practice: one that has started no Engagement for three times its own usual gap between Engagements. A Credit model has no cancel event, so a leaving Practice is silent unless it is measured. The one explicit exit is Practice deletion. |
| Acquisition cost | Marketing money plus founder hours at a fixed rate, divided by Practices that bought a first Credit. |
| Payback | Acquisition cost divided by monthly revenue per Practice. Bootstrappers keep it at 2–6 months (SP p.146). |
| Plateau | Credit revenue added per month divided by the monthly churn rate (SP p.149). |

## What to build, and what to refuse

### Market first, then marketing, then look, then function

`SM p.30` `SP p.34, p.43, p.63`

- **Claim:** a market that pays beats marketing, marketing beats aesthetics, and aesthetics beats features (SM). Product-market fit grows from a deep understanding of one market; specific beats general; a second vertical added early brings different needs and complexity (SP).
- **Means here:** one vertical: doulas and doula Practices, as `docs/personas/` describes them. A feature earns its place through a named persona or Practice. A request shaped for a midwife, a lactation consultant, or a birth center is a new market, not a feature. #999 names the primary buyer.
- **Does not mean:** shipping something broken or ugly; the cross-cutting expectations in `CLAUDE.md` still bind. Nor narrowing away the contractor doula, the Admin, or the Client, who are part of a doula Practice's world.
- **Broken when:** a feature ticket names no persona or Practice, or a screen exists that no doula persona uses.

### Solve the problem behind the request

`SP p.46–51`

- **Claim:** customers report problems well and design solutions badly. Sort requests into crackpots, no-brainers, and in-betweens. For an in-between, ask what the use case is, how many customers will use it, and whether it fits the product's vision. A feature few people use goes behind a setting, not into the default screen. Focus is saying no to good ideas.
- **Means here:** "how many customers" is noise with five pilot Practices, so ask instead which persona needs it and how many of the pilot's Staff would use it every week. A request from one Practice becomes a persona need when a second Practice or the persona document confirms it. A "no" is a scope decision, recorded on the ticket with its reason.
- **Does not mean:** discounting the fourteen-doula agency because it is one Practice; it holds about a quarter of the pilot's doulas. Nor leaving a found gap unfixed: before launch, everything found is fixed.
- **Broken when:** a feature ships whose ticket names one requesting Practice and no persona.

### Talk to four kinds of people, with open questions

`SP p.43–45`

- **Claim:** talk to prospects, customers, people who chose not to buy, and people who left. Ask how they solve the problem today and what frustrates them. A mockup shown for approval gets politeness, not data.
- **Means here:** #994 interviews the pilot agency. Every quiet Practice, and every prospect who chose something else, gets one founder question: why.
- **Does not mean:** a survey sent to the waitlist.
- **Broken when:** a roadmap decision cites a reaction to a mockup as its evidence.

### Build with at least two real Practices on real data

`SM p.88`

- **Claim:** build against at least two real customers entering real data. Friends checking that it does not crash are not customers.
- **Means here:** the pilot agency is today's one real customer contact (#991). A second Practice with a different shape, most naturally a solo doula, enters real Clients before launch.
- **Does not mean:** seed data or a test Practice counts.
- **Broken when:** a decision about how Practices work rests on one Practice's habits.

## How the product treats the people using it

These principles govern the product itself: what it asks of doulas, Admins, and Clients, and what it does for them without being asked.

### The ask is the last resort

`NI ch.11–12, p.131–142`

- **Claim:** before a screen asks a person for something, derive it, infer it from an act the person already did, or drop the field.
- **Means here:** a status the product can infer is a candidate for automation (ADR-0015). An Engagement becomes active on its first scheduled Visit (#895), a Visit's type is derived, Contract merge fields fill themselves (#258), and the Owner confirms the timezone the browser already supplied rather than typing it (#1166).
- **Does not mean:** removing a decision that belongs to the person.
- **Broken when:** a new field, or a status moved by hand, holds a value the system already has.

### Start from the goal and count the steps

`NI ch.2, ch.9–10, p.8–13, p.113`

- **Claim:** design starts from what the person is trying to get done, not from a screen. A car-key app took thirteen steps to do what a key does in two.
- **Means here:** **steps from goal** is every act from the person's trigger to their goal, including acts outside the product, with the acts on an interface marked. Each journey map in `docs/journeys/` records the count at its moment of truth (#1267), and a design is judged by whether the count goes down.
- **Does not mean:** a product with no screens.
- **Broken when:** a spec starts from a wireframe, or a journey's steps from goal go up.

### Take chores away; never add one

`NI ch.13, p.147–157`

- **Claim:** software keeps turning analog chores into digital ones, such as updating, filing, and marking things done, when its job is to remove them.
- **Means here:** keeping a record true is the product's job. Where only a person can know, as with ADR-0015's "is this care finished?", the product asks at the moment it matters (#1262) instead of leaving the record wrong until someone remembers.
- **Does not mean:** completing an Engagement automatically; bereavement Visits continue after a loss (ADR-0015).
- **Broken when:** a record stays wrong unless someone remembers to fix it.

### An act starts every message; a badge never exists

`NI ch.15, p.178–180` `ADR-0028, ADR-0035, ADR-0038`

- **Claim:** alarms driven by clocks and thresholds get ignored. A hospital monitoring system went from about 120 alarms to about 2 targeted ones and became useful.
- **Recorded:** a date passing alone notifies nobody (ADR-0038), and the shell has no notification bell (ADR-0028). A delay that the person's next act can cancel is allowed: the payout email waits 48 hours and is skipped if the Owner finished in that window (ADR-0035). A sweep with no starting act exists only for a legal duty: the dormancy notice sent before New York escheats an unspent balance (`api/internal/billing/dormancy.go`, APL §1315).
- **Means here:** a notification is caused by something a person did that another person must act on. NI adds a judgment: passing the timing rule does not make a message wanted, so a message must report a condition that needs the person, never only ask them to come back (ADR-0035, ADR-0038). SM argues the opposite for trial check-ins, which raised activation from about 25% to about 75% (SM p.213, p.224). Which rule governs the product is open, and #1266 decides it. When a coverage gap is saved with no cover, Owners and Admins get an email (#1093).
- **Does not mean:** chasing a Client on a timer. A founder writing personally to a Practice that went quiet or never started, working from a list the product derives, is not a product alert either.
- **Broken when:** the product gains a bell, a badge, or a timed sweep with no legal duty behind it, or a coverage hole can be found only by opening a list at 2 a.m.

### Success is time given back, not time spent

`NI ch.5–6, p.47–62`

- **Claim:** products built to hold attention measure the wrong thing. A tool succeeds when the person finishes and leaves.
- **Means here:** measure work done through the product (Engagements run, Visits logged, invoices paid), never daily active users, sessions, or minutes in the app. This is what "retention" means everywhere in this document.
- **Does not mean:** having no retention measure.
- **Broken when:** a metric rises when a task takes longer.

### Adapt to the Practice and the person from their own data

`NI ch.14–15, p.161–181`

- **Claim:** a good system learns from its own users' patterns instead of applying one default to everyone.
- **Means here:** a per-doula count of concurrent on-call windows, and a Practice-set rule for when on-call starts (#1093).
- **Does not mean:** predicting anything from a Client's health data.
- **Broken when:** a one-size default applies where the Practice's own data already says otherwise.

### Collect only what the task needs, and forget it when the task ends

`NI ch.17, p.187–197`

- **Claim:** every piece of data collected is a liability and a cost to trust.
- **Means here:** an Offer shows a thin copy of the Client and stops serving her details when it ends (#230); Erasure redacts in place (ADR-0027); the teaser is cookieless (ADR-0016); notification email carries no third-party open or click tracking (ADR-0030).
- **Does not mean:** collecting nothing.
- **Broken when:** a read outlives the job, or a field is collected "for later".

### Every automation keeps a manual path, and big decisions keep a person's act

`NI ch.19–20, p.203–205, p.208`

- **Claim:** automation fails, so a person must be able to correct it by hand, the way a smart thermostat still has a dial. Some decisions are big enough to keep a deliberate act, the way a $200,000 home loan keeps a button.
- **Means here:** the manual status move stays (ADR-0015), a Payment can be recorded by hand (#271), and nothing ever completes an Engagement automatically, because a bereaved Client's care continues after a loss (ADR-0015). Spending a Credit, signing, voiding, Erasure, deletion, and completing an Engagement each keep a person's act.
- **Does not mean:** building both the automatic and the manual path badly, or adding friction everywhere.
- **Broken when:** an automation has no correction by hand, or an automation spends money, signs, or ends care.

## How to compete

### Compete on sales model and product, not on price

`SP p.52–57`

- **Claim:** against incumbents there are three levers: price (more than 80% of the product at half the cost), sales model (a public price, no demo wall), and product (modern UX, faster shipping). The UX lead is temporary. Watch competitors' big moves and the deals you lose; ignore their polish, their funding, and their daily moves.
- **Means here:** every doula tool surveyed is a monthly per-seat subscription that leads with "start free" (#995). Doula Cloud's difference is its sales model (pay per Engagement, and no bill in a month with no births) and the smoothness the design brief commits to. The price is public (#285). When a Practice chooses a rival, the reason is written into the competitor dossier.
- **Does not mean:** meeting a cheaper per-seat tool on price. The answer is what the Engagement record does, not a lower number.
- **Broken when:** copy or a sales conversation offers a discount to win a comparison, or a feature is justified by "a competitor has it" with no lost Practice behind it.

### A moat is what builds up, not what is clever

`SP p.57–61`

- **Claim:** unique features are a false moat, a "hamster wheel" that competitors copy in months. The moats that last are integrations, brand, owned traffic channels, and switching costs.
- **Recorded:** the competitor dossier names the Offer/Attachment model as the moat. It stands as the lead.
- **Means here:** read through SP, that model is a lead a competitor can copy, and the moat is what accumulates inside it: Engagement history, contractor pay records, the audit trail, claim-ready Medicaid data. Design so a Practice's record grows more valuable the longer it lives here.
- **Does not mean:** keeping a Practice by making its data hard to take away. The switching cost comes from value, not from a missing export. (This clause is our reading, not the book's.)
- **Broken when:** a feature's whole value would be gone the day a competitor copies the screen.

## How to price

### One value metric that grows with the customer

`SP p.67, p.70–72`

- **Claim:** pricing is the biggest lever. Expansion revenue comes from a value metric tied to the customer's own success. Seat pricing fits only when two users see different things. A value metric combined with feature gating gets complicated fast.
- **Means here:** the Engagement is the value metric (#439). A Practice that grows pays more without making an upgrade decision. One axis only: no feature tiers.
- **Does not mean:** volume pricing. #439 rejected it: Stripe refuses tiered one-time Prices, and a bulk discount goes to the agency with the best margins.
- **Broken when:** a feature is proposed as a paid add-on.

### Revisit the price on a schedule, and change it with notice

`SP p.64, p.68, p.79–85`

- **Claim:** most founders underprice out of fear. "If no one's complaining about your price, you're probably priced too low." Revisit pricing every 6–12 months, and price at what you want the product to be worth, then build until it is (aspirational pricing). A higher price also opens more channels: about five are affordable at $20 a month, all of them at $5,000. To raise a price, give 2–4 months' notice, say why, never promise a price for life, and grandfather existing customers unless the change grows revenue by 10% or more.
- **Recorded:** #439 set $20 flat at the bottom of the doula-fee range on purpose, provisional until the pilot confirms it. It stands.
- **Means here:** a price review after the pilot, then every 6–12 months. Every price objection is written down with who raised it and what they compared it to (#243). A purchased Credit keeps the price paid (#420, three-year refund window), so a change honors what a Practice already bought, and copy promises today's price and nothing beyond it.
- **Does not mean:** raising the price before the pilot answers #243, or cutting it to win a comparison.
- **Broken when:** a year passes with no review, a complaint is answered with a discount, or a page calls a price permanent.

### Charge up front; free Credits are a bounded trial, not a free plan

`SM p.35–42, p.145` `SP p.73–78`

- **Claim:** a forever-free plan pushes revenue into the future and fills support with people who will never pay; at Bidsketch, ending the free plan multiplied paid conversions eight to ten times (SM). Among independent SaaS companies, 66% offer a trial and 17% a free plan. The default is a card up front, because a no-card trial brings about ten times the signups, the support load, and the noise from people outside the core market (SP).
- **Recorded:** Credits are prepaid. Every new Practice gets three `signup_bonus` Credits with no card, and a pilot Practice also gets a one-time `founding_grant` of three per Staff member (#439). Both stand.
- **Means here:** free Credits are a trial bounded by use. Watch conversion from free Credits to a first purchased Credit, and support contacts from Practices that never buy. The trial has no clock; its length is the Practice's own birth cadence.
- **Does not mean:** a free plan, a larger grant to win signups, or copy that leads with "free" the way ten of eleven competitors do (#995).
- **Broken when:** Practices spend their free Credits and never buy, or support time on Practices that never buy outgrows support time on paying ones.

## How to reach customers

### Marketing is build work

`SM p.56–58, p.61, p.89, p.191` `SP p.87–89, p.101, p.134`

- **Claim:** the product does not sell itself; products that seem to are marketed so well it is invisible, and marketing strategy cannot be handed off (SP). Start marketing alongside the code; a launch list costs hours (SM).
- **Means here:** go-to-market tickets (#991 and its children) sit on the same calendar as build tickets. A marketing date that gates another date is on the critical path.
- **Does not mean:** building an audience, such as a blog or a podcast, before launch. Only 5% of TinySeed companies did, and Walling calls it a waste of time for most founders (SP). A launch list is not an audience (SM).
- **Broken when:** a marketing milestone slips while build work lands on time.

### Know what a Practice is worth, and how many you can reach, before paying to get one

`SM p.43–55, p.146–148` `SP p.99, p.146`

- **Claim:** know what a customer is worth before spending to acquire one, and size the market bottom-up from the people you can actually reach, then halve it (SM). An event is worth it when one or two customers repay it; if it takes twenty, raise the price or skip the event. Bootstrappers recover acquisition cost in 2–6 months (SP).
- **Means here:** a solo doula is worth about $480 a year and a fourteen-doula agency about $4,000 (#439), so paid ads are unlikely to repay. A $400 conference booth repays with about one agency-month or ten solo-months. A target on #991 carries a reachable count behind it: co-op lists, Medicaid-enrolled doulas, local search volume.
- **Does not mean:** the books' ad-click figures as targets, or a top-down count of every doula in the country.
- **Broken when:** a channel-plan line (#1003) has no expected Practices per dollar, or a target has no reachable count behind it.

### Choose channels for quality, speed, cost, and scale

`SM p.18–21` `SP p.101–106`

- **Claim:** traffic quality beats volume. In order of quality: your own list, your own content, a targeted write-up, a direct recommendation, a search for your name, then referrals, other organic traffic, ads, and banners (SM). Weigh each channel on speed, cost, and scalability; run one fast channel and one slow one; keep a marketing changelog; change one variable at a time; double down on what works. At a low price, the affordable channels are other people's audiences, the places a market gathers, founder-run partnerships, content and search, and virality (SP p.103–104).
- **Means here:** the waitlist, doula podcasts, doula co-ops and groups, and trainers come before any paid placement. ADR-0016 carries each visitor's source.
- **Does not mean:** an account on every platform (#991 limits accounts to platforms that bring paying customers), or skipping basic on-page search work.
- **Broken when:** an untargeted traffic spike is counted as a win, ads run before the targeted venues are used, or a channel runs for a month with no recorded cost and result.

### Two funnels, one product

`SP p.90–94, p.109–114`

- **Claim:** high-touch selling fits $500 or more a month; low-touch selling fits a wide market at a low price. A dual funnel runs both: the wide one spreads the name, and the high-touch one grows its share of revenue. Qualify a demo request by the value metric, and send small accounts a recorded walkthrough. In a demo, be an unpaid expert: ask about the problem and today's workaround, show only what solves it, say no to a bad fit and name a better tool, and follow up.
- **Means here:** a solo doula (about $40 a month) signs up and reaches value on her own; an agency (about $333 a month and up) gets a founder conversation. The qualifying question is the number of doulas. A booth or demo script (#998) starts from the Practice's problem. #999's primary buyer sets the message, and both funnels run under it.
- **Does not mean:** a founder call for every solo doula after the pilot.
- **Broken when:** a solo doula cannot reach value without Mark, an agency cannot reach Mark, or a demo tours the settings screens.

### Say it in four seconds, in her words

`SM p.73–75`

- **Claim:** a visitor decides in seconds. Lead with what it is and whom it is for, with a promise, or with one remarkable feature.
- **Means here:** marketing surfaces speak the doula's words ("doula", "my clients", "birth", "on call"), not `CONTEXT.md` terms. The product glossary and the marketing vocabulary are separate on purpose (#991).
- **Broken when:** a headline or a post says "Engagement" or "Practice".

### The marketing site's first job is the second visit

`SM p.22–29, p.125`

- **Claim:** most first visitors will not buy, and returning visitors bought four to sixteen times more in Walling's data, so give the visitor who leaves a way back.
- **Means here:** an input to #868. An evaluator who is not ready still leaves with a way to come back; the evaluator-doula journey (`docs/journeys/evaluator-doula.md`) counts "an intention to come back", or a clear reason she left, as done. Price and signup stay visible (#285).
- **Does not mean:** the app. The app's job is to let a person finish and leave (see "Success is time given back, not time spent").
- **Broken when:** the January site's only call to action is creating a Practice.

### Ship on a public date; consistency beats a big break

`SM p.91–100`

- **Claim:** a public date forces shipping. Recognition comes from many steady appearances, not one big moment. Repeated death-march deadlines are the failure on the other side.
- **Means here:** January 2027 is public on the teaser. Founder-voice posts keep a steady cadence through #1003's before, during, and after phases, not one conference splash.
- **Broken when:** posting stops after one event, or the date moves without a public note.

### Build sharing into the product from the start

`SP p.151–153`

- **Claim:** a product is viral when each customer brings in part of another. Strong loops come from a product that connects people, such as Slack or an e-signature link; weak loops are a brand shown on customer-facing surfaces. Virality is hard to add later. Every founder should ask for referrals: by automated email at 60–90 days for a low-touch product, in person for a high-value one.
- **Means here:** Doula Cloud's loops are the people a Practice brings in: a Client invited to the portal, a contractor doula who works for several Practices, a doula who receives an Offer. Design each of those first touches as that person's introduction to the product. Ask an agency for a referral in person.
- **Does not mean:** showing that a person is another Practice's Client (ADR-0015: no Client fact crosses a Practice), or branding a payment surface so it suggests Doula Cloud takes the money (the pilot terms and Stripe Connect Terms §3.4(b)).
- **Broken when:** the invitation a contractor doula receives gives no sense of what the product is.

## What to measure

### Two north stars, six more, and no vanity

`SP p.144–154` `SM p.150–152`

- **Claim:** watch revenue and its growth rate every week. Then watch three numbers to push down (acquisition cost, sales effort, churn) and three to push up (contract value, expansion, referrals); they pull against each other. A number without its denominator is vanity (SP). Once customers have used the product for a while, ask how they would feel if they could no longer use it, where 40% "very disappointed" is the bar, and what they would use instead (SM).
- **Means here:** Credits consumed per month and its growth, per the table above, with the plateau computed. #991's target of five pilot Practices is a real number; its target of 150 waitlist subscribers counts only through what converts. Ask pilot Practices both survey questions after about 60 days of use.
- **Broken when:** a number is reported without its denominator, such as subscribers without conversion or signups without a first purchase.

### Keep, then convert, then attract

`SM p.9–17` `SP p.154–161`

- **Claim:** a lost customer costs as much as a hundred visitors at 1% conversion, so fix retention before conversion, and conversion before traffic (SM). Segment churn by price, channel, and cohort; churn in the first 60 days is an onboarding problem. Before product-market fit, ask why people leave, and never game the number with a hard cancel. Send a short founder note within minutes of a cancellation and ask for a reply (SP).
- **Means here:** retention means a Practice keeps running Engagements through the product, never sessions or time in the app (see "Success is time given back, not time spent"). A quiet Practice and a deleted Practice each get a founder question. Churn is segmented by persona (solo, agency, contractor-heavy), by source, and by signup month. From launch, a pilot Practice's request outranks channel work.
- **Does not mean:** ranking work by severity before launch, or making it harder to leave.
- **Broken when:** a Practice goes quiet and nobody knows why, or channel spend rises while a pilot Practice consumes no Credits.

### Know where every Practice came from

`SM p.23, p.150–152` `SP p.108–109`

- **Claim:** "word of mouth" usually means "we don't know" (SP). Store the first-touch source, and also ask "how did you hear about us?", because self-report and analytics disagree: in one survey 85% named search where analytics credited it with 37% (SM).
- **Means here:** ADR-0016 carries the source for the teaser. The source travels from first touch to the Practice record, beside a self-reported answer.
- **Does not mean:** cookies or fingerprinting (ADR-0016's cookieless posture stands), or measuring behavior inside a Practice's Client records. Measure the acquisition channel and customer outcomes.
- **Broken when:** a channel decision is made with no source data.

### Find the minimum path to awesome

`SP p.158–160`

- **Claim:** find the moment a new customer says "this is amazing", track each new account's progress toward it, and ask active customers when the value clicked.
- **Means here:** pilot interviews settle the moment. Candidates: a first Engagement with its Client in the portal, a first Offer a contractor accepts, a first invoice paid through Stripe Connect.
- **Broken when:** onboarding work ships without naming the step it shortens.

## How the founder works

### Spend founder time on risks, not certainties

`SP p.123–125, p.170` `SM p.109–115, p.155`

- **Claim:** certainties, work you already know how to do, can be handed off; risks, such as finding customers and finding fit, need the founder. Technical founders solve every business problem with code (SP). Of code, marketing, and money, a founder usually has two, and marketing is the common gap. Read only what changes the next one or two months (SM).
- **Means here:** agents carry the certainties. Before proposing a build to fix a growth symptom (conversion, churn, revenue), name the funnel stage it fixes and whether a customer conversation would answer it first. A research ticket names the decision waiting on it.
- **Broken when:** a revenue problem gets a feature before it gets a conversation.

### Ship when the criteria are met; put rigor where reversal is expensive

`SM p.88–95` `SP p.82, p.166, p.176–178`

- **Claim:** polish that delays shipping is fear posing as work (SM). Bias toward action: treat an uncertain change as an experiment you can undo, map plans B, C, and D, and treat most roadblocks as speed bumps (SP).
- **Recorded:** architecture is decided now, not later, and recorded in ADRs. It stands.
- **Means here:** once a ticket's acceptance criteria are met, "one more improvement" is a new ticket. The ADR and grilling process is for what is cheap now and expensive later: schema, money, law. Copy, onboarding, channels, and prices are reversible: change one variable, measure, and decide.
- **Does not mean:** skipping the cross-cutting expectations in `CLAUDE.md`.
- **Broken when:** a dated milestone slips for work its criteria never asked for, or a reversible copy change waits on the process a schema change needs.

### From launch, the founder works by hand until a written threshold

`SM p.132–135`

- **Claim:** do a task by hand until it hurts, and know before you start the point at which it gets automated. Walling's example went from 160 hours to 10.
- **Recorded:** before launch, everything found is fixed (`CLAUDE.md`; the launch split above).
- **Means here:** from launch, the founder may do by hand what the product will later do, and that manual path carries its threshold, a number, on its ticket. This covers the founder's own operations only.
- **Does not mean:** handing a customer a chore the product could infer (see "Take chores away; never add one"), or handling security, the audit trail, or money movement by hand.
- **Broken when:** a manual process has no number, or its number is passed and no ticket exists.

### Operate as if someone else will run it

`SM p.142–145, p.205, p.221–233`

- **Claim:** build the business so it could be sold: repeatable, teachable, one vertical. Stabilize before growth, and rehearse recovery; at HitTail, replication failed silently for 48 hours.
- **Means here:** every recurring operation has a runbook in `docs/runbooks/`. Backups are restored on a schedule, not only kept. Signup stays self-serve.
- **Broken when:** an operation only Mark can perform has no runbook, or a backup has never been restored.

### Expect the dip; the risk is burnout, not money

`SP p.23, p.126, p.139, p.179–186` `SM p.100, p.123–126, p.217`

- **Claim:** bootstrappers run out of motivation, not money. Single founders succeed, but they need a peer group; hire support even with no plan to grow; work where joy, skill, and need overlap (SP). The first months after launch are small; plan 12–18 months to a flywheel, and keep marketing through the dip (SM).
- **Recorded:** Doula Cloud is a bootstrapped, single-owner business with no employees planned. It stands.
- **Means here:** every support email costs one person an hour, so design the answer into the product: clear errors (ADR-0021) and help where the confusion happens. Marketing continues in February whatever January's numbers are.
- **Broken when:** the same question reaches hello@doula.cloud twice, or marketing stops because launch month was small.
