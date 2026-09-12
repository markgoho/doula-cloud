# Theme synthesis: who it is for, and how the product learns

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about naming the customer, what a persona is worth, and how many people a product has to hear from before its makers know anything; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

Who is the buyer, what do the nine proto-personas count as, and how many real people has the product heard from?

## What each book claims

**Jobs To Be Done** (Ulwick, 2016). Personas built on demographic and psychographic data create phantom targets; segments come from statistically valid research, not observation (ch. 2, p. 52; ch. 4 §IV, pp. 95–96); a segment description is the same thing as an outcome-based persona (ch. 8, pp. 195–196). Asking what job a customer hired the product for is a common mistake; the question is what job they are ultimately trying to get done (ch. 4 §II, p. 87). Managers first agree exactly who the customer is; there are three job executors, and buyer and user can be one person (ch. 4 §I, pp. 83–86; ch. 2, p. 61). A segmentation survey runs on 180 to 3,000 customers, a small one-product company is told to hire the author's firm, and a sprint takes four weeks (ch. 4 §IV, p. 98; ch. 7, pp. 177, 182, 186). A large and a small dairy can share the same unmet needs; demographics nearly always fail to explain them (ch. 5, p. 138; ch. 4 §IV, p. 96).

**The SaaS Playbook** (Walling, 2023). Have conversations with prospects, customers, the people who declined, and the people who canceled; ask open-ended questions; sort feature requests by use case, share of customers, and vision (Market, pp. 44–50).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Working directly with at least two customers is the test of whether the features add value ("Five Reasons You Haven't Launched", p. 88). Ask customers how they found you and what they would use instead; the benefit they name may not be the one marketed ("Four Things I Learned", pp. 150–152).

**Make: The Bootstrapper's Handbook** (Levels). Ideas come from solving your own problems; the founder is the greatest expert at them; talking to customers to find problems is an outside perspective; users who write to the feedback box are usually the best users (Idea, pp. 20–23; Grow, pp. 126–127). Start in a micro niche and widen niche by niche (Idea, pp. 25–28).

**Design for Cognitive Bias** (Thomas, 2020). Users lie: memory is reconstructed from current belief, people are poor reporters of themselves, and Netflix's survey answer lost to its A/B test (ch. 2, pp. 10–12). Nothing about us without us: participatory design's top rung implements what the participants decide, and the people most impacted tend to have the least input (ch. 4, pp. 77, 80–83). Survivorship bias and duty of care: consider the unhappy path as thoroughly as the happy one and design for the most vulnerable first (ch. 4, p. 85; Resources, p. 96).

**Copyhackers: Uplift.** Go back to voice-of-customer research; the winning team spoke to customers about the customer's needs, not the product (Section 2, p. 20; Section 4, p. 35). The One Reader and her stage of awareness decide the page (Section 1, p. 11; Section 5, p. 48).

**Landing Page Hot Tips.** No position on research or personas; its reader is assumed.

**The Almanack of Naval Ravikant** (Jorgenson, 2020). No direct position. Its nearest claim, that a founder escapes competition through authenticity (Building Wealth, pp. 41, 44), sits in the moat theme.

**The Best Interface Is No Interface** (Krishna, 2015). No position on research method. Its chapter 5 argument that screen time is the wrong measure of engagement sits in the measurement theme.

## Where the books agree

- A persona built from assumptions is a hypothesis, not evidence: Ulwick says so directly; Thomas's "users lie" and Copyhackers's voice-of-customer both reach for observed behavior or the customer's own words over a constructed profile.
- Talk to real people, including the ones who said no: Walling (both books), Copyhackers, Thomas.
- Name the customer before anything else: Ulwick's job executors, Copyhackers's One Reader, Levels's micro niche.

## Where the books conflict

- **How many.** Walling's test is two customers; Ulwick's survey is 180 to 3,000; Levels says the founder's own problem is enough. The three answers differ by three orders of magnitude.
- **Whose problem.** Levels's founder-as-expert assumes the founder is the customer; Ulwick and Thomas assume the founder is not and warn against the founder's own reconstruction.
- **What an interview is worth.** Thomas's "users lie" undercuts the interviews Walling and Copyhackers rely on; Ulwick answers with a structured survey that a pre-launch company cannot run.
- **Evidence.** Ulwick's method is his firm's product; Walling's two-customer rule is his own experience; Thomas cites Netflix's published test.

## What the repo already decided

- `docs/personas/README.md`: the nine personas are proto-personas, built from assumptions and from what the schema and handlers do, not from interviews, surveys, or analytics; each is a hypothesis to be falsified and is never to be cited as user research. The **Persona** entry in `CONTEXT.md` says the same. `docs/personas/loss-client.md`, Nadia Haddad, exists because software written for the happy path alone would hurt her; her journey is applied in ADR-0038.
- #991 (2026-09-08): Practice owners and solo doulas are the buyers, one to be named primary by #999; parents and referral sources are out of scope; the pilot agency is the only real customer contact; positioning is a hypothesis until the market answers.
- #994: the interview guide of 2026-09-08 for the pilot agency, which bans offering the words "one place", "save time", "all in one", and "free", and records that solo doulas outside an agency are not represented and that #999 is not to read the interviews as evidence for both segments.
- #243: the standing list of questions for pilot Practices, to which #439 added the price question and #257 the welcome-grant size.
- #1265, a second Practice on real Clients before launch, was closed by the founder as not planned on 2026-09-10; a not-planned ticket decides nothing beyond its own closure.
- Open: #994 (the interview), #999 (positioning and the primary buyer).
