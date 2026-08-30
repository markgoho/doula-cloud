# `/pilot-terms` — the promotion terms a pilot Practice agrees to

The copy below is **final and verbatim**. It was settled on [#444](https://github.com/markgoho/doula-cloud/issues/444), on the map [#375](https://github.com/markgoho/doula-cloud/issues/375), and every number in it comes from a decision made elsewhere: the $20.00 Credit and the three-per-Staff-member founding grant from [#439](https://github.com/markgoho/doula-cloud/issues/439), the shape of the pilot from [#421](https://github.com/markgoho/doula-cloud/issues/421).

Built as `hugo/content/pilot-terms.md` on `hugo/layouts/page.html`.

## Where it lives, and how it is served

- URL: `https://doula.cloud/pilot-terms`
- **Unlisted, not secret.** `noindex, nofollow`, absent from the sitemap, absent from every navigation, and linked from nothing on the site. The only route to it is the disclaimer line in the waitlist confirmation email.
- Stripe's rule for *"any promotion, discount, or trial"* is about **placement** — the terms are visible where a person agrees to take part — **not** about publicity. So an unlisted page satisfies it, and the pilot stays private. This does not contradict [#390](https://github.com/markgoho/doula-cloud/issues/390), which took `noindex` **off** `/support`: there it bought nothing, and here it buys the privacy the pilot is meant to have.
- **Not behind authentication.** The reader has no account yet; a login wall would put the terms behind the thing she is agreeing to.
- Reachable with no password, no cookie and no JavaScript.

Three mechanisms make "unlisted" true, and all three are needed:

- `<meta name="robots" content="noindex, nofollow">`, the default in `layouts/page.html`. A page that wants to be indexed sets `robots: index` in its front matter and says so out loud — `/support` ([#419](https://github.com/markgoho/doula-cloud/issues/419)) is the one that will.
- `sitemap.disable: true` in the page's front matter. A `noindex` page listed in `sitemap.xml` is a mixed signal that publishes the URL anyway.
- `disableKinds = [..., 'rss']` in `hugo.toml`. The feed at `/index.xml` republished every page's URL **and its whole body**, which handed out exactly what the `noindex` tag withholds. The site has nothing to subscribe to, so the feed was never earning its place.

## The line that links it

The page is reached from the waitlist confirmation email ([#361](https://github.com/markgoho/doula-cloud/issues/361)), beside the link a Practice accepts the pilot from. That email is composed in Buttondown and is not built yet, so this line is a **specification** for it, carried as an acceptance criterion on #361:

> The pilot's founding grant of free Credits, and what a Credit costs once it runs out, are set out in the [pilot terms](https://doula.cloud/pilot-terms).

It must sit **beside the accept link**, not in the footer. Placement is the whole requirement.

## Constraints the copy is written under

- **These are promotion terms, not a pilot agreement.** The page is one-directional: what Doula Cloud grants and what it charges. It asks **nothing** of the Practice — no feedback obligation, no participation commitment, no confidentiality, no data or termination terms. The moment it asks something of her it becomes a pilot agreement, which is **out of scope** on [#375](https://github.com/markgoho/doula-cloud/issues/375) and needs a lawyer reading drafted text.
- **The price is on this page, and that is deliberate.** `/support` carries no price because [#285](https://github.com/markgoho/doula-cloud/issues/285) owns the *published* price on January's marketing site and Stripe does not require one there. Here the number is unavoidable: [#421](https://github.com/markgoho/doula-cloud/issues/421) obliges the terms to state the grant's size and that list pricing applies beyond it, and "what it costs" cannot be said without saying $20.00.
- **The refund position is `/support`'s, word for word in substance.** Any divergence between the two pages is a bug, not a nuance. See the note at the top of [`support-page.md`](support-page.md).
- **Connect Terms §3.4(b)** forbids holding ourselves out as a payment facilitator, intermediary or aggregator. The last section says so in the place a Practice is agreeing to something.
- **The product name is a token.** Every mention goes through the `{{< product >}}` shortcode, which reads `title` from `hugo.toml`, so [#338](https://github.com/markgoho/doula-cloud/issues/338) stays a one-edit change.

---

# Pilot terms

Doula Cloud is in a private pilot with a small number of practices, from October 2026 until it opens to everyone in January 2027.

This page says what a practice is offered for taking part, and what it is charged. It applies to every practice invited into the pilot.

## What a pilot practice is given

A practice that joins the pilot is granted **three free Credits for each person on its staff**. A solo doula is granted three. A practice of fourteen doulas is granted forty-two.

The grant is counted once, at the moment the practice joins. Adding a doula later grants nothing further, and the grant is not repeated.

Granted Credits do not expire. They run out by being used, not by a date, and any left unused stay in the account.

## What a Credit is, and what it costs

One Credit covers one engagement — a single client relationship centered on one baby, from intake through the end of care. Birth and postpartum engagements cost the same.

A Credit costs **$20.00**, plus sales tax where it applies. That is the ordinary price of a Credit, not a pilot price, and it is what a practice pays for every Credit beyond its grant.

There is no subscription and no recurring charge. A practice buys Credits when it wants them and is charged nothing in between.

## What happens when the pilot ends

Nothing changes when Doula Cloud opens to everyone in January 2027.

- A Credit still costs $20.00.
- Unused Credits stay, granted and purchased alike.
- The account carries on. There is nothing to renew, and nothing to cancel.

The founding grant is the one thing the pilot offers that January does not, and it is granted once, when a practice joins.

## Refunds

Credits given free of charge are not refundable.

Purchased Credits that have not been spent can be refunded within three years of the date they were bought, at the price paid for them and together with any sales tax charged on them, to the original payment method. A Credit already used to start an engagement has been spent, and is not refundable. To ask for a refund, email us. We do not need a reason.

## The pilot does not change who the merchant is

Doula Cloud is not a payment service. When a practice invoices a client, the practice is the merchant: it holds its own agreement with Stripe, the money is paid into its own account, and it is responsible for the care it provides and for what it charges. Doula Cloud never receives or holds a practice's money.

## Contact us

Email **hello@doula.cloud**. That address reaches a person.

---

## Why each part is there

| Sentence | What it is doing |
| --- | --- |
| *"…a private pilot… from October 2026 until it opens to everyone in January 2027."* | Stripe's promotion rule needs a stated duration. It also tells the reader she is joining something that ends, before any of the money words appear. |
| *"…three free Credits for each person on its staff."* | The grant's **size**, which [#421](https://github.com/markgoho/doula-cloud/issues/421) obliges the terms to state. Per Staff member rather than per Practice, so a solo doula's whole pilot is not free and an agency's grant is not trivial — [#439](https://github.com/markgoho/doula-cloud/issues/439). |
| *"A solo doula is granted three. A practice of fourteen doulas is granted forty-two."* | The pilot's two real shapes, arithmetic done for the reader. The fourteen-doula agency is a specific Practice in the pilot, and it would otherwise do this sum itself and ask. |
| *"The grant is counted once, at the moment the practice joins. Adding a doula later grants nothing further…"* | The question an agency asks next, answered before it is asked. [#439](https://github.com/markgoho/doula-cloud/issues/439) made it a one-time count, not a running per-head entitlement, and a Practice that hired in November would otherwise expect three more. |
| *"Granted Credits do not expire… not by a date."* | [#439](https://github.com/markgoho/doula-cloud/issues/439) gave the grant no expiry deliberately: the sizing exhausts it inside the pilot, and a date would rebuild the January cliff that granting was chosen to avoid. Saying so is what makes it a promise rather than an omission. |
| *"One Credit covers one engagement — a single client relationship centered on one baby…"* | A promotion has to name what is being granted, and a refund policy has to name what is being refunded. It also states the unit correctly: a Client who returns for a second pregnancy costs a **second** Credit ([#439](https://github.com/markgoho/doula-cloud/issues/439) corrected the map on this). |
| *"Birth and postpartum engagements cost the same."* | Pre-empts the fairness question a doula selling hourly postpartum care will raise. One price was chosen over weighting by `kind` because a per-kind price invites mis-declaration. |
| *"A Credit costs **$20.00**, plus sales tax where it applies."* | The list price, from [#439](https://github.com/markgoho/doula-cloud/issues/439). The tax clause is not decoration: [#389](https://github.com/markgoho/doula-cloud/issues/389) apportions it, so a Practice with nobody in New York is charged none, and a flat "$20.00 including tax" would be false for the Rochester agency. |
| *"That is the ordinary price of a Credit, not a pilot price…"* | The load-bearing sentence of the whole page. [#421](https://github.com/markgoho/doula-cloud/issues/421) rejected cheap pilot Credits precisely so that January is not a price increase asked of every pilot Practice at once. The terms are where that becomes something she can hold us to. |
| *"…what a practice pays for every Credit beyond its grant."* | [#421](https://github.com/markgoho/doula-cloud/issues/421)'s third obligation: list pricing applies to everything past the grant, including after January. |
| *"There is no subscription and no recurring charge."* | Matches `/support`. A reader agreeing to a "pilot" reasonably fears an auto-renewing trial, which is the pattern this is not. |
| *"Nothing changes when Doula Cloud opens to everyone in January 2027."* + the three bullets | "What happens when it ends" is one of this ticket's acceptance criteria, and the honest answer is *nothing*. Written as a list because a reader scanning for the catch should find it in one pass and not find one. |
| *"Credits given free of charge are not refundable."* | `/support`'s position, and the reason the grant needs no cash-out mechanism. |
| *"…within three years… at the price paid… to the original payment method."* | `/support` verbatim in substance, and every clause in it is load-bearing against a specific statute — [§1139](https://www.nysenate.gov/legislation/laws/TAX/1139), [APL §1315](https://www.nysenate.gov/legislation/laws/ABP/1315). **Do not reword this paragraph on one page only.** Read [#390](https://github.com/markgoho/doula-cloud/issues/390) and [#439](https://github.com/markgoho/doula-cloud/issues/439) first, then change both. |
| *"Doula Cloud is not a payment service… never receives or holds a practice's money."* | Connect Terms §3.4(b), and [#383](https://github.com/markgoho/doula-cloud/issues/383)'s money-transmission ruling, said at the moment a Practice is agreeing to take part — which is the moment she could otherwise conclude the platform is the one handling her fees. |
| *"Email **hello@doula.cloud**."* | The same contact as `/support`. A promotion with no way to ask a question about it is a promotion a reviewer distrusts. |
