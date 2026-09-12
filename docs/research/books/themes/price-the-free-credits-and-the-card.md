# Theme synthesis: price, the free Credits, and the card

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about how a product like this one is priced, whether a person tries it free, and whether a card is taken before value is delivered; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule, stated in its Source line. The read-backs are in `docs/research/books/`.

## The question

What does a Practice pay, per what unit, at what moment, and what does a stranger get before paying anything?

## What each book claims

**The SaaS Playbook** (Walling, 2023). Price at $50 a month or more for breathing room, and $10–15 only for a no-touch consumer product; if nobody complains about the price it is too low (Pricing, pp. 67–68). Choose a value metric, and use seats only if two users from one company see different things (pp. 70–71). Freemium suits a simple, low-support, viral product; 66% of surveyed SaaS offer a trial and 17% a forever-free plan (pp. 73–74). A card up front is the default and a qualifying event; trials without one multiply support tenfold (pp. 75–78). No trial at all, with a refund policy in the first thirty days, runs the most pricing experiments (p. 78). Revisit pricing every six to twelve months; aspirational pricing builds the product up to the price (pp. 79–80). Grandfather unless the increase grows MRR by at least 10%, never promise it for life, and announce two to four months ahead (pp. 83–85).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Free plans convert at about 1% and drain support; ask for money up front ("Why Free Plans Don't Work", pp. 37–42). $1 a month cannot work; $50–100 a month is better; when asked to lower the price, build up to it instead ("You Can't Make Money Charging $1", pp. 146–148).

**Make: The Bootstrapper's Handbook** (Levels). Do not be afraid to charge; do not lower prices or go free, because the complainers are not the customers; expect no more than 5% of active users to pay (Monetize, pp. 133–135). Build with monetization in mind from the start, tested with a real Buy button that stops short of charging (pp. 136–138). Subscription revenue is the seller's holy grail but forgotten subscriptions may be half of it; a one-time lifetime price at predicted lifetime value is the author's middle ground, and pay-per-feature has potential (pp. 138–139, 145–149).

**Landing Page Hot Tips.** Offer more tiers, suggest one, minimize the options, add an annual discount, and consider a lifetime price (Tips #14, #50, #58, #32, #88; PDF pp. 39–41, 95–97, 109, 67, 161). Offer a free teaser, remove the obstacles to a demo, and lower the risk to commit with a free trial, no card, and a money-back guarantee (Tips #40, #46, #47; PDF pp. 83, 91, 92). The first question a SaaS visitor asks is how much it costs, answered with full transparency (Tip #85, PDF p. 157).

**Jobs To Be Done** (Ulwick, 2016). A product wins by getting the job done better or more cheaply; a dominant strategy is at least 20% better and 20% cheaper (ch. 3, pp. 62–63, 73). The buyer weighs 40 to 80 financial outcomes, and the outcome survey carries willingness-to-pay questions (ch. 2, p. 61; ch. 6, pp. 170, 172).

**Design for Cognitive Bias** (Thomas, 2020). Whatever is incentivized gets gamed, and a measure made a target ceases to measure (ch. 3, pp. 41, 49). Three plans laid out so one is visually favored steer the buyer toward the business's choice; the test is whether the designer would disclose why (ch. 4, pp. 87–88).

**Copyhackers: Uplift.** No position on price, trial, or card. The book is about page copy; its nearest claim is that the reader's stage of awareness decides how much a page says (Section 1, p. 11; Section 5, p. 48), which the marketing theme carries.

**The Almanack of Naval Ravikant** (Jorgenson, 2020). No position on price or trial mechanics. Its nearest claim, that wealth comes from giving society something it wants and cannot yet get (Building Wealth, pp. 31, 39), sits in the moat theme.

**The Best Interface Is No Interface** (Krishna, 2015). No position on price, trial, or card.

## Where the books agree

- Charge money, and do not go down when asked: Walling (both books), Levels, and Hot Tips all say build up to the price rather than lower it.
- A forever-free plan is a support cost, not a channel: Walling in both books; Levels expects at most 5% of active users to pay.
- Fewer choices, one suggested: Hot Tips's suggested tier and Thomas's warning about a visually favored plan describe the same layout from opposite sides.
- A refund is a legitimate substitute for a trial: Walling (p. 78) and Hot Tips's money-back guarantee.

## Where the books conflict

- **The card.** Walling wants a card before value, as a qualifying event; Hot Tips wants no card and a money-back promise. Levels tests willingness with a Buy button that charges nothing.
- **The unit.** Walling's $50 floor and Levels's 5% figure assume a monthly subscription; Levels then argues against subscriptions for the seller's own reasons; Ulwick's method prices an outcome, not a month; none of the three names a per-transaction unit.
- **Tiers.** Hot Tips wants several tiers; Thomas treats a favored tier as a bias to disclose; Walling's value-metric advice does not require tiers at all.
- **Where the evidence comes from.** Walling's percentages are surveys of SaaS companies; Hot Tips's tips are uncited; Ulwick's willingness-to-pay figure comes from a survey run on 180 to 3,000 customers (ch. 4 §IV, p. 98), which the read-back's section 8 notes a one-product company is told to outsource.

## What the repo already decided

- #439 (2026-08-29): one Credit costs $20.00, flat; the unit is the Engagement, not the Client and not the seat; both Engagement kinds cost the same so that a price varying by kind does not invite mis-declaration; no volume discount, because a bulk price hands the discount to the agency with the best margins; the anchor is 2.5% of the measured $800 Monroe County median package; the price is provisional pending the pilot, with the confirming question on #243.
- #257: three Credits are granted at signup with no card (`api/internal/staffauth/signup.go`), the `signup_bonus`; purchase runs through Stripe Checkout (#242, #285).
- #390, amended by #439: unspent purchased Credits refund at the price paid for three years, matching New York's dormancy period.
- #421 (2026-08-28): the pilot runs at list price with a founding grant of free Credits, so that January is not a price rise for every pilot Practice at once; #444 keeps the pilot terms unlisted.
- #45 (2026-08-14): no subscription, no tiers; a flat fee per Engagement with all features included; the ledger is credited only after a server-side Stripe webhook confirms payment.
- #285: the Billing screen states what one Credit costs before the buyer leaves for Stripe. `docs/copy/support-page.md` records that the published price is January work.
- Open: #868, whether the marketing site publishes the price, where, and in what framing.
