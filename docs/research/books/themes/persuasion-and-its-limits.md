# Theme synthesis: persuasion and its limits

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about the techniques a page or a product may use to move a person, and where those techniques become manipulation; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

Which ways of moving a person are open to the product and to the site that sells it, and what test tells a nudge from a trick?

## What each book claims

**Design for Cognitive Bias** (Thomas, 2020). Reactance: a polite sign draws less graffiti than one shouting in capitals, and telling people what to do invites the opposite (ch. 1, pp. 4–5). The framing effect: 95% lean and 5% fat are the same beef, and the frame supersedes the facts (ch. 2, pp. 34–35). Sometimes friction is good: slowing a person where a decision matters reduces errors (ch. 2, pp. 35–36). Notational bias: a male/female field wipes out identities, and design components represent mental models of what a user can express (ch. 4, pp. 72–74). Honorifics foreground marital status; leaving metadata out is also a choice (ch. 4, pp. 75–76). Every request for personal information builds a potentially unhealthy relationship and deserves pause (ch. 4, pp. 74–75). An optional email field placed between a terms checkbox and a CAPTCHA reads as required, a dark pattern (ch. 4, p. 87). The contrast effect steers a buyer to the visually favored plan, and the test is whether the designer would disclose why (ch. 4, pp. 87–88).

**Landing Page Hot Tips.** Create haste with a time-limited discount, increase demand by limiting supply, and soft-launch with a discount to a select audience (Tips #24, #51, #76; PDF pp. 56, 98–99, 142). Add social proof, prove you are established with a founding year and usage figures, and put the best testimonial above the fold (Tips #52, #82, #83; PDF pp. 100, 153, 154–155). Offer tiers and suggest one (Tips #14, #50; PDF pp. 39–41, 95–97).

**Copyhackers: Uplift.** A trusted-by logo row and social proof made the user-friendly claim more believable; the original page had no persuasive elements to build trust (Section 2, pp. 21–22). Copy written from the customer's own words outperformed copy written about the product (Section 2, p. 20; Section 4, p. 35).

**The Best Interface Is No Interface** (Krishna, 2015). Chapter 5 argues that a product built to maximize screen time is optimizing against the person using it.

**Make: The Bootstrapper's Handbook** (Levels). The complainers about price are not the customers (Monetize, pp. 133–135); build in public and show mistakes, because it makes a founder look human (Grow, pp. 118–121). The read-back's section 8 notes the book's own controversy over tactics it recommends.

**The SaaS Playbook** (Walling, 2023). Grandfathering: never promise it for life; announce a price increase two to four months ahead (Pricing, pp. 83–85). A founder email within ten minutes of a cancellation (80/20 SaaS Metrics, p. 161).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). No position on the ethics of persuasion; its funnel advice assumes the techniques work.

**Jobs To Be Done** (Ulwick, 2016). No position on persuasion. Its claim that buyers search by outcome rather than product name (ch. 4 §IX, pp. 115–116) is about being found, not moved.

**The Almanack of Naval Ravikant** (Jorgenson, 2020). No direct position. Its nearest claim is that a person escapes competition through authenticity (Building Wealth, pp. 41, 44).

## Where the books agree

- Proof that is real works: Copyhackers's logo row, Hot Tips's founding year and testimonial, and Thomas's disclosure test all admit a true claim displayed plainly.
- The customer's own words beat the maker's: Copyhackers and Thomas's reactance chapter both describe the shouting page losing.
- Framing is unavoidable: Thomas says every frame is a choice; Hot Tips and Copyhackers spend their pages choosing frames.

## Where the books conflict

- **Urgency and scarcity.** Hot Tips recommends both as tips; Thomas names the mechanisms they exploit as biases and offers the disclosure test, which a manufactured deadline fails.
- **The favored plan.** Hot Tips's suggested tier and Thomas's contrast effect are the same layout, recommended by one and questioned by the other.
- **Friction.** Thomas argues friction is sometimes good; Hot Tips removes every obstacle to a demo.
- **Evidence.** Thomas cites named studies; Hot Tips cites nothing; Copyhackers reports single-page tests with no controls beyond the A/B itself.

## What the repo already decided

- ADR-0021: an error message never says "please", "valid", "invalid", or "required"; `app/src/lib/formErrors.usage.spec.ts` fails a commit on those words (#467).
- ADR-0038 (decided on #981): an open Invoice keeps the Client-facing label "Not yet paid", because "Overdue" in red on a bill read by a woman whose pregnancy ended in loss is the product shaming her.
- #473, recorded in the Dialog paragraph of `docs/design/govuk-alignment.md`: block over warn for all four undoable actions, a hard block with a deliberate, action-named override.
- ADR-0017: pronouns stay Practice-defined because the product reads none of them; only `given_name` is required; `clients.email` is nullable because a fake value is worse than an empty one (the **Client** entry in `CONTEXT.md`). The Names departure in `docs/design/govuk-alignment.md` keeps no title or honorific column.
- The signup screen, `app/src/routes/(signed-out)/signup/+page.svelte`, renders six controls and no consent checkbox and no optional-looking field between them.
- #439: one Credit at one flat price, no volume discount, so nothing on the Billing screen is visually favored; #285 reads the price from the Stripe Price onto the screen.
- #421 and #444: the pilot runs at list with a founding grant, and the pilot terms are unlisted.
- #991, under Not yet specified: proof on the January site, a testimonial or a named pilot Practice, waits on the pilot terms and on someone saying yes. #995: no competing tool says how many doulas use it.
- The brief (#409): conventional in pattern and behavior, distinctive in execution.
- No record governs which persuasion techniques the marketing site uses; #868 and #1001, which hold the site's words, are open.
