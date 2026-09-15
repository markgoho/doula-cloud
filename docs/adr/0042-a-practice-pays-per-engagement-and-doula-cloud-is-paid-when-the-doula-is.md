# A Practice pays per Engagement, and Doula Cloud is paid when the doula is

How a Practice pays Doula Cloud was decided in August 2026, one mechanism at a time: no subscription and no tiers on [#45](https://github.com/markgoho/doula-cloud/issues/45); one Credit at $20.00 per Engagement, flat, anchored and provisional, on [#439](https://github.com/markgoho/doula-cloud/issues/439); three Credits at signup with no card on [#257](https://github.com/markgoho/doula-cloud/issues/257) and the founding grant on [#449](https://github.com/markgoho/doula-cloud/issues/449); the three-year refund of an unspent Credit on [#390](https://github.com/markgoho/doula-cloud/issues/390); the pilot at list price with no recurring charge on [#421](https://github.com/markgoho/doula-cloud/issues/421) and [#444](https://github.com/markgoho/doula-cloud/issues/444); the price stated before Stripe on [#285](https://github.com/markgoho/doula-cloud/issues/285); and Credits as Doula Cloud's own billing surface in [ADR-0032](0032-billing-is-credits-payments-is-getting-paid.md). Those records hold the mechanism, and the **Credit** entry in `CONTEXT.md` holds the glossary. None of them records the value the mechanism serves.

On 2026-09-14 the founder tested every one of those decisions against the twenty-nine books on the values track, in the first grilling session under [ADR-0041](0041-a-book-enters-the-repo-as-evidence-and-becomes-a-rule-only-through-a-decision.md), on [#1369](https://github.com/markgoho/doula-cloud/issues/1369), from the theme synthesis at [`docs/research/books/themes/price-the-free-credits-and-the-card.md`](../research/books/themes/price-the-free-credits-and-the-card.md). Thirteen questions over three rounds; every recorded decision held, and the questions the record left open were settled. He confirmed the positions record on that ticket. This ADR is the value behind the mechanism: why each position holds, in his reasons, with the evidence weighed and what was rejected. Page citations are as the read-backs in `docs/research/books/` give them, each under its own page rule. A book is evidence here and nowhere the reason.

**Doula Cloud is paid when the doula is. A Practice pays one flat price for each Engagement and nothing in between, every feature is included, and a stranger runs three families through the product before any card is taken.**

## The decision

### One Credit per Engagement, and no recurring charge

The unit stays the Engagement, and there is no subscription, no minimum, and no recurring element of any kind. The founder's reason, in his words: "our success is tied together, when doula does a birth, doulacloud gets paid, so win/win, we both profit together." A doula is paid per family, so her cost falls per family, and in a slow month she pays nothing, which is what the pilot terms already promise on #444.

Weighed for: Godin's claim that a subscription is an act of permission rather than a billing shape, so what matters is that she comes back each time a family does ([This Is Marketing](../research/books/this-is-marketing.md), Ch. 17, PDF p. 154); Levels's observation that forgotten subscriptions may be half of subscription revenue, which is the honesty argument for a charge that only happens when work does ([Make](../research/books/make-the-bootstrappers-handbook.md), Monetize, pp. 148–149).

Weighed against and rejected: *The 1-Page Marketing Plan*'s rule that every business carries a subscription or recurring element (Chapter 8, pp. 247, 255); *Start Small, Stay Small*'s recurring hosted application as the default for business software (Ch. 3, pp. 101–103); *Company of One*'s renewals as the metric worth watching (ch. 4, PDF pp. 55–56; ch. 7, PDF p. 86). All three are written for a product a customer uses every month. A doula's work is paced by births, and a monthly charge would bill her for months in which the product did nothing for her.

Accepted as a risk to watch, not a reason to move: *Software as a Science*'s warning against charging on the very unit that makes a customer use the product less, the reason Netflix prices on account size rather than hours and Slack gives messaging away (ch. 9, PDF pp. 197–198, 201–202). A Practice that keeps a birth off the system to save a Credit has done what that warning predicts, and loses the charting, the contract, and the invoice that live inside the Engagement. The pilot watches for it on [#243](https://github.com/markgoho/doula-cloud/issues/243).

### Three free Credits, and the card at the first purchase

Signup grants three Credits and takes no card. The card is taken at the first purchase, by Stripe Checkout, which is the qualifying event. The founder's reason: three Engagements "is enough to get a good feel for how the platform can help doulas." An Engagement runs weeks, so a trial measured in Engagements has to cover more than one birth to be a trial at all, and three is a solo doula's first quarter.

Weighed for: *Company of One*'s free version that feeds word of mouth and converts without much extra effort (ch. 7, PDF p. 89); *Lean Marketing*'s own churn case, a thirty-day trial taken with a card and never set up because the import looked daunting (Chapter 14, pp. 231–232); *Landing Page Hot Tips*'s no-card trial as the way to lower the risk to commit (Tips #46, #47; PDF pp. 91, 92).

Weighed against and rejected: *The SaaS Playbook*'s card before value as a qualifying event, with trials without one multiplying support tenfold (Pricing, pp. 75–78), and *Software as a Science*'s card asked for on the sales call itself (ch. 5, PDF p. 114). Both assume a sales motion with a call in it; here the first purchase is the qualifying event, and it happens without anyone from Doula Cloud in the room.

Held in view: *The Mom Test*'s rule that a free thing counts as commitment only in proportion to what it costs the person, and a cheap trial has to be made more expensive before it means anything (ch. 5, p. 70). Spending a Credit costs her a real family run through the product, which is not cheap. A recommendation to cut the grant from three to one on that reasoning was put to the founder and not taken. The pilot watches for the Practice that spends three and never buys, on #243.

### $20.00 through the pilot; the workaround is the anchor, the package share the ceiling

The price stays $20.00 per Credit through the pilot, as #439 set it. The founder's reason: it was set with an anchor and nothing in the pilot has yet argued against it. This ADR states two anchors, because #439 stated one. The **anchor** is the workaround: what a doula pays today for the tools a Credit replaces, the charting app, the contract tool, the invoicing tool, the calendar. That is the usable price signal in *The Mom Test* (ch. 1, pp. 16, 19–20) and the pricing hypothesis in *The Four Steps to the Epiphany* (Chapter 3, pp. 161–163). The competitor dossier of 2026-09-09, a private artifact the founder keeps out of the repo, holds the figures for that comparison. The **ceiling** is the package share: a Credit is never more than a few percent of what a family pays her, which is #439's 2.5% of the $800 Monroe County median package. Both anchors land near $20 today; they part at the first review, because one moves with competitor prices and the other with doula fees.

The first price review is in the second quarter of 2027, after the pilot, on one test: if no pilot Practice objected to the price, the price is low. That is *The SaaS Playbook*'s test that if nobody complains it is too low (Pricing, pp. 67–68), *Software as a Science*'s that nobody saying no means the same, and that not changing a price is a decrease (ch. 9, PDF p. 192; ch. 10, PDF pp. 217–218), and Blank's that no objection at all may mean too low (Chapter 4, p. 327). The review is a review, not a rise: the founder decides then.

### One price, every feature, nothing sold beside it

There is one price, every feature is included, and nothing is sold beside it: no tiers, no add-ons, no paid onboarding, no paid support tier. The founder's reason: a doula buys a family at a time, not a plan, and "all this should be dirt cheap with AI doing it," onboarding and support included.

Weighed against and rejected: three tiers with one suggested, in *Marketing Made Simple* (ch. 5, p. 103), *The 1-Page Marketing Plan* (Chapter 6, pp. 191–192), *Start Small, Stay Small* (Ch. 3, pp. 98–100), *Software as a Science* (ch. 9, PDF pp. 205–206), and *Landing Page Hot Tips* (Tips #14, #50, #58; PDF pp. 39–41, 95–97, 109); all of them sell to buyers who compare plans. *Software as a Science*'s paid onboarding at 10 to 20 percent of annual contract value and paid priority support about a fifth of customers take (ch. 11, PDF pp. 233–234, 242), written for annual contracts sold to buyers with budgets, which a doula agency does not have; a paid support tier would put the Practice that needs help most behind the one that pays more.

Weighed for: *Company of One*'s simplicity, Casper making three mattresses rather than 108 (ch. 1, PDF pp. 25–27); Thomas's warning that a visually favored plan is a bias the designer would have to disclose ([Design for Cognitive Bias](../research/books/design-for-cognitive-bias.md), ch. 4, pp. 87–88), which one price with no plans never raises.

Open elsewhere: whether claims submission carries a per-claim charge, if [#809](https://github.com/markgoho/doula-cloud/issues/809) ends with Doula Cloud in the claims path, is decided there, not here.

### The price is published

$20 per family is published on the marketing site: the first line of the pricing page, and in the home page's first screen after the message that [#1001](https://github.com/markgoho/doula-cloud/issues/1001) settles. The founder's reason: one number with no tiers is the simplest pricing story there is, and hiding it would read as custom pricing that does not exist.

Weighed for: *Company of One*'s claim that publishing what you charge where anyone can compare it is itself the trust move (ch. 10, PDF pp. 116–117); Godin's that telling buyers the price is the price is a story worth telling (Ch. 16, PDF pp. 146–147); *Lean Marketing*'s services page that carries the price (Chapter 9, pp. 150, 152–153); *Landing Page Hot Tips*'s first question a SaaS visitor asks (Tip #85, PDF p. 157). Rejected as not applying: *Marketing Made Simple*'s leave-prices-off where pricing is custom or the catalog long (ch. 5, p. 103).

### The refund of an unspent Credit is the only guarantee

The published guarantee is the one #390 already made: an unspent purchased Credit refunds at the price paid for three years, in the support page's own words, "To ask for a refund, email us. We do not need a reason." The pricing page carries those words. There is no promise of any kind on a spent Credit; #45 decided that a locked Credit is never returned, and the three free Credits are the walk-through that a money-back promise on a spent one would duplicate. A refund to an upset Practice stays the founder's discretion, never a published policy.

Weighed for: *Start Small, Stay Small*'s thirty-day no-questions guarantee as what the pricing page carries failing a free trial (Ch. 4, pp. 131–133); *The 1-Page Marketing Plan*'s specific guarantee that reverses risk better than a vague one (Chapter 6, pp. 187–189); *Building a StoryBrand 2.0*'s agreement plan that may be titled as a guarantee (ch. 7, PDF pp. 110–111). Kept as discretion: Godin's refund of the upset customer over a small sum rather than the argument (Ch. 14, PDF p. 134; Ch. 19, PDF p. 165).

### A price rise never touches a bought Credit

A Credit is a unit, so a price rise cannot touch one already bought; grandfathering is built into the shape and needs no term. Once a rise is announced, a Practice may buy as many Credits as it likes at the old price, with no cap. A rise is announced sixty days ahead, by email, with the reason stated. No price is ever promised for life. The founder's reason: a Practice that prepays a year has handed over cash a bootstrapped company wants, and the three-year refund bounds her risk.

Weighed for: *Software as a Science*'s advice to sell a year of the current price ahead of a rise (ch. 10, PDF pp. 225–226) and its email that says plainly this is a price increase (ch. 10, PDF pp. 222–224); *The 1-Page Marketing Plan*'s rise with a stated reason (Chapter 8, pp. 240–241). Rejected as written for a monthly price rather than a unit: *The SaaS Playbook*'s grandfather-unless-the-rise-grows-MRR-by-10% and never for life (pp. 83–85), of which only never-for-life is kept; *Software as a Science*'s six months of the old price (ch. 10, PDF pp. 222–224); *Start Small, Stay Small*'s seven-day offer to the mailing list (Ch. 3, pp. 100–101).

## What was considered instead

**A subscription, or any recurring element.** Rejected because it bills a doula for months in which the product did nothing for her, and severs the tie the founder wants: Doula Cloud paid when she is.

**A card before value.** Rejected because the first purchase is the qualifying event and no sales call exists to take a card on; a card at signup would only stop the doula who was going to try three families and buy.

**One free Credit instead of three.** Put and not taken, because one Engagement shows the product and three show whether it helps.

**Tiers, add-ons, paid onboarding, paid support.** Rejected because a doula buys a family, not a plan, and because the cost of onboarding and support is to be driven toward nothing rather than charged for.

**A money-back promise on a spent Credit.** Rejected because #45 closed the reclaim loophole for a reason that still holds, and the free Credits already carry the risk a guarantee would.

**A term-limited grandfather.** Not needed, because a bought Credit is already the buyer's at the price she paid.

## Consequences

- `docs/manifesto.md` now exists, with one line for this ADR, as ADR-0041 requires once the first value ADR does. It holds one line per value ADR and nothing else.
- The **Credit** entry in `CONTEXT.md` is unchanged: the mechanism did not move, and the entry keeps citing the tickets that decided it.
- [#868](https://github.com/markgoho/doula-cloud/issues/868) and #1001 carry the price on the site as this ADR states; #243 carries the two pilot watches; #809 keeps the claims question.
- The first price review is a calendar fact for the second quarter of 2027. A rise, if one comes, gives sixty days' notice by email with its reason.
- The competitor dossier's figures are not in the repo and this ADR does not repeat them; the founder holds the artifact.
- Where a later grilling session on another theme touches the price, the Credits, or the card, this ADR is the record it tests against, and a change lands as a superseding ADR, not an edit here.
