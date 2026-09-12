# Theme synthesis: the moat

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about what keeps a customer from leaving, whether features count, and how narrow a first market is; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

What keeps a Practice here once she has tried it, and does the feature list follow from that answer?

## What each book claims

**The SaaS Playbook** (Walling, 2023). Four moats: integrations, brand, owned traffic, and high switching costs; unique features are a false moat (Market, pp. 57–61).

**Make: The Bootstrapper's Handbook** (Levels). Start in a micro niche, such as booking software for hairdressers who focus on African hair, then widen niche by niche and later widen the market (Idea, pp. 25–28; Monetize, p. 159). A business built on another company's API is exposed: they can shut it down at any time, and renaming a key breaks the app (Build, p. 72).

**Jobs To Be Done** (Ulwick, 2016). Comparing competitors' feature sets is a waste of time; table stakes are outcomes very important and very satisfied by existing products (ch. 4 §VI, pp. 103, 105). Customers switch only when a product gets the job done upwards of 20% better; under 5% is a sustaining strategy that entices nobody (ch. 3, p. 78; ch. 2, pp. 48–49). Related jobs make a platform valuable, and the long-term strategy is a solution that gets the entire job done on one platform (ch. 2, p. 58; ch. 4 §III, p. 92; §X, p. 122).

**The Almanack of Naval Ravikant** (Jorgenson, 2020). Escape competition through authenticity; nobody can compete with a person at being that person; avoid frameworks built on what others do (Building Wealth, pp. 41, 44; Saving Yourself, pp. 158–159). Wealth comes from giving society something it wants and cannot yet get; technology is what does not quite work yet (Building Wealth, pp. 31, 39).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Ask customers what they would use instead ("Four Things I Learned", pp. 150–152). No position on moats as such.

**Design for Cognitive Bias** (Thomas, 2020). No position on competition. Its nearest claim, that design for the most vulnerable first (ch. 4, p. 85), is about who the product serves, carried by the who-it-is-for theme.

**Landing Page Hot Tips.** No position on moats. Its social-proof tips assume an established product.

**Copyhackers: Uplift.** No position on moats.

**The Best Interface Is No Interface** (Krishna, 2015). No direct position. Its argument against screen-time engagement (ch. 5) bears on what a "sticky" product optimizes for.

## Where the books agree

- Features are not a moat: Walling says so directly; Ulwick calls feature comparison a waste; Naval's authenticity argument rejects competing on what others do.
- Narrow first: Levels's micro niche and Ulwick's core functional job define a market by one group and one job.
- A dependency is a risk: Levels names the API; Walling's integrations moat is the same dependency viewed from the other side.

## Where the books conflict

- **What the moat is.** Walling lists four and none is "a pricing model" or "execution"; Ulwick's moat is the entire job on one platform; Naval's is the founder's own authenticity; Levels's is being first in a niche too small for anyone else.
- **The entire job.** Ulwick's long-term strategy is what the read-back of #995 records ten of eleven competitors already claiming, which Ulwick's own 20%-better threshold would call table stakes.
- **Evidence.** Walling's four moats are asserted; Ulwick's 20% figure is his firm's rule of thumb; Naval's claims are aphorisms; Levels's examples are his own products.

## What the repo already decided

- ADR-0008, decided on #228, #229, and #230: the Offer and Attachment model; employment type gates the Practice and the Attachment gates the Engagement.
- #173, closed as completed: a survey of competing doula CRMs sorted into table stakes, differentiators, and absent-across-the-board, at the "whether" altitude, recommending nothing; it feeds the map's first essential test, that a Practice cannot stop using her current tool without it.
- #995 (2026-09-08), in #991's decisions so far: ten of eleven competing tools claim "one place"; every one sells a monthly per-seat subscription; no tool prices per Engagement; no tool says how many doulas use it.
- #439: the unit is the Engagement, and nobody in the market prices per Engagement at all.
- `docs/design/brief.md`, chosen 2026-08-28 on #409: conventional in pattern and behavior, distinctive in execution, with Jakob's Law as the governing reason and smooth UX as a primary goal.
- `docs/personas/evaluator-doula.md`: the evaluator's one question is whether the product is built for what she does and whether she can get out again.
- #994: the interviewer does not offer the words "one place", "save time", "all in one", or "free", and writes it down as a finding if she says them unprompted.
- The **Connected account** entry in `CONTEXT.md` and ADR-0007: Stripe refuses to create the older Accounts v1 shape for a new integration; #421 reads v2 requirements for a v2 account.
