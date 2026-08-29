# `/support` — the page Stripe reviews

The copy below is **final and verbatim**. It was settled on [#390](https://github.com/markgoho/doula-cloud/issues/390), on the map [#375](https://github.com/markgoho/doula-cloud/issues/375), and is built by [#358](https://github.com/markgoho/doula-cloud/issues/358) and verified by [#419](https://github.com/markgoho/doula-cloud/issues/419).

Do not reword it without reading #390 first. Every sentence in the refund section is load-bearing against a specific New York statute or a Stripe contract term, and several of them read like ordinary marketing prose while doing legal work.

## Where it lives, and how it is served

- URL: `https://doula.cloud/support`
- **Indexable.** No `noindex`, no canonical tag — the same posture [#368](https://github.com/markgoho/doula-cloud/issues/368) set for the teaser root. Stripe is indifferent either way (verified first-party: unlisted is fine, gated is not), so this is our choice, and the page carries nothing that is being kept quiet — there is no price on it and no feature detail. Reversed on [#390](https://github.com/markgoho/doula-cloud/issues/390) after `noindex` was first recommended: it bought nothing, and a long-`noindex`ed page is slow to get re-crawled once the tag comes off.
- Reachable with no password, no cookie and no JavaScript.
- Linked from the teaser footer, beside `/privacy`.
- `business_profile.url` on the live Stripe platform account points at **this page**, not at the site root. The teaser root is the exact shape of Stripe's named rejection code `invalid_url_website_incomplete_under_construction`.

## The standing constraint on this URL

**Stripe re-checks the declared URL for as long as the account is live** — it is not a gate passed once at activation. So `doula.cloud/support` is committed to carrying a service description, a customer support contact and a refund position indefinitely.

It was `/about` until it was pointed out that January's marketing site will want that URL for a company-story page. `/support` was chosen over `/terms-of-service` deliberately: the real terms of service, privacy policy and pilot agreements are **out of scope** on [#375](https://github.com/markgoho/doula-cloud/issues/375) and belong to their own effort, so a page claiming that URL would promise something it does not deliver and would have to move again the moment the real one is drafted. A support page means the same thing in January as it means now, so it squats on nothing. Two ways to keep it true, and one of them has to be chosen deliberately rather than discovered:

- keep the required content on `/support`, however the page is redesigned around it; or
- move the content to its new home **and change `business_profile.url` in the same change**.

Silently redesigning `/support` without doing either takes the live platform account into `invalid_url_website_incomplete` territory, with no deploy-time warning.

## Constraints the copy is written under

- **Connect Terms §3.4(b)** forbids holding ourselves out as a payment facilitator, intermediary or aggregator. A Practice *invoices its clients from* Doula Cloud; money never moves *through* it. See [#383](https://github.com/markgoho/doula-cloud/issues/383).
- **No price.** [#285](https://github.com/markgoho/doula-cloud/issues/285) owns the published price and is January work. Stripe does not require one.
- **No feature detail** beyond what Stripe's requirement list forces.
- **Refunds**: purchased Credits only, at the price paid, to the original payment method, on the Practice's own request, within three years.

---

# Support and billing

## What Doula Cloud is

Doula Cloud is practice-management software for doulas and doula agencies. A practice uses it to keep its client records, plan and schedule visits, message its clients, send and sign its contracts, and invoice its own clients for its own services.

Doula Cloud is not a payment service. When a practice invoices a client, the practice is the merchant: it holds its own agreement with Stripe, the money is paid into its own account, and it is responsible for the care it provides and for what it charges. Doula Cloud never receives or holds a practice's money.

Doula Cloud is in a private pilot with a small number of practices, ahead of a public launch in January 2027.

## What we sell

Practices buy **Credits**. One Credit covers one client engagement — a single client relationship, from intake through the end of care.

Credits do not expire. There is no subscription and no recurring charge: a practice buys Credits when it wants them, and is charged nothing in between.

## Refunds and cancellation

There is nothing to cancel. Doula Cloud bills no recurring fee, so a practice that stops buying Credits is charged nothing further, and may close its account at any time.

Unspent Credits that a practice has purchased can be refunded within three years of the date they were bought, at the price paid for them and together with any sales tax charged on them, to the original payment method. Credits given free of charge are not refundable. A Credit already used to start an engagement has been spent, and is not refundable.

Credits themselves do not expire, and a practice can spend them whenever it likes. It is only the right to a cash refund that runs out.

To ask for a refund, email us. We do not need a reason.

## If you think a charge is wrong

Email us before disputing the charge with your bank. If we have charged you in error we will refund it. We would rather fix it than argue about it.

## Contact us

Email **hello@doula.cloud**. That address reaches a person, and it is also where privacy and data requests go.

---

## Why each part is there

| Sentence | What it is doing |
| --- | --- |
| *"Doula Cloud is practice-management software for doulas and doula agencies…"* | Stripe's minimum: the business name and a description of the goods or services. Without it the account cannot be activated at all. |
| *"Doula Cloud is not a payment service… never receives or holds a practice's money."* | Connect Terms §3.4(b). Also puts [#383](https://github.com/markgoho/doula-cloud/issues/383)'s money-transmission ruling somewhere a regulator or a Practice's lawyer can read it without asking. |
| *"…in a private pilot… ahead of a public launch in January 2027."* | Pre-empts the reviewer's real question — why does a product that is not available need live payments? |
| *"One Credit covers one client engagement…"* | A refund policy must name what is being refunded. Publicly answers [#286](https://github.com/markgoho/doula-cloud/issues/286). |
| *"…at the price paid for them…"* | Credits rise in price over time. Refunding at today's price would let a Practice profit on an unused balance. |
| *"…to the original payment method."* | A refund cheque that sits uncashed becomes escheatable property in its own right under [APL §1315(1-a)](https://www.nysenate.gov/legislation/laws/ABP/1315). |
| *"To ask for a refund, email us."* | A refund the platform initiates inherits the original balance's dormancy date; one issued on the Practice's recorded request restarts the clock. |
| *"…within three years…"* | Matches two clocks that are already three years long. [§1139](https://www.nysenate.gov/legislation/laws/TAX/1139) is how long New York gives us to reclaim sales tax we have already remitted, so every refundable dollar of tax stays recoverable for the life of the promise. And [APL §1315(1-b)](https://www.nysenate.gov/legislation/laws/ABP/1315) escheats an unspent balance to New York at three years' dormancy — so a shorter window would mean refusing a practice its own money and then surrendering the same money to the State. The practice can ask for it back for as long as we are allowed to hold it. Cost accepted: a refund outside the current or next filing quarter needs an **AU-11** claim rather than [§1132(e)](https://www.nysenate.gov/legislation/laws/TAX/1132)'s straight exclusion from the ST-100. Amended on [#439](https://github.com/markgoho/doula-cloud/issues/439); it was 90 days when a Credit cost $5.00. |
| *"Email us before disputing the charge with your bank."* | Stripe asks for a dispute policy. A business that invites contact first is a business with a low chargeback rate, which is what the reviewer is assessing. |
| *"Email **hello@doula.cloud**."* | Stripe's required customer service contact. Email is explicitly accepted; no phone number exists, and inventing one would be worse. |
