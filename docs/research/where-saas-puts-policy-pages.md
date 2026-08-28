# Where real software businesses put the content Stripe asks for

Question: **a pre-launch B2B SaaS needs one publicly reachable page carrying the business name, a description of the service, a customer service contact, and a refund/dispute/cancellation position — Stripe's activation requirement. Where do real businesses actually put that content, and is a single consolidated page for it normal, unusual, or a smell?**

Researched **28 August 2026** by visiting the sites and reading the pages. Every claim below is labelled by how it was verified: **observed** means the page was fetched and the quoted text came out of it; **absent** means the page was fetched or probed and the thing looked for was not there; **unverified** means a first-party fetch failed and no secondary source was substituted. There are no claims sourced to review sites, cancellation-service blogs, or search-result summaries.

Eighteen sites were checked across three groups: practice-management competitors, small indie SaaS billing through Stripe, and businesses that sell a prepaid credit balance rather than a subscription.

## 0. What Stripe actually asks for — first-party

Before measuring anyone against it, here is the requirement itself. **Observed**, from [Website checklist](https://docs.stripe.com/get-started/checklist/website) on docs.stripe.com:

> This page contains a list of the common elements—such as accurate product descriptions, clear policies, and proper security features—that each business on Stripe should address on its website.

The checklist items that generate our page:

> **A description of what you're selling.** Besides only listing the name of the product or service, you can help customers with their purchasing decision by providing detailed text descriptions of what you're selling. [...] If we review your website and find that it isn't clear what you're selling, we may contact you with recommendations for improving the description.

> **Customer service contact information.** Make sure your customers can find multiple contact methods on your site, including direct communication channels, such as email addresses, phone numbers, and live chat (something besides contact forms). [...] If we review your website and can't find a clear way to contact you, we may ask that you add some contact options to the site.

> **Your fulfillment policies.** For most businesses, you must clearly explain your order fulfillment policies to your customers. Some examples of policies required for many businesses include: Refund policy: Describe the conditions under which customers can receive a refund. [...] Cancellation policy: Describe the conditions under which customers can cancel subscriptions or reservations.

Two things follow, and the rest of this document tests both against what real sites do.

**Stripe says "on its website", never "on one page".** The word "page" does not appear in the requirement. Stripe reviews a *site*. Nothing in the first-party text asks for consolidation onto a single URL. Consolidation is our own convenience, not a Stripe demand — which is a large part of why, as it turns out, nobody else has built one.

**Stripe explicitly disfavours a contact form on its own.** "something besides contact forms" is Stripe's own parenthetical, not a paraphrase. A published email address satisfies it; a form by itself does not. Every site in this sample that plainly clears the requirement publishes a direct email address, a phone number, or both.

## 1. The direct competitors — practice management for solo practitioners

Six sites: Practice Better, SimplePractice, Jane App, Cliniko, Halaxy, and Doulado. **All six put their refund position inside a Terms of Service document.** None has a standalone refund page. None has a consolidated processor-facing page.

**A doula-specific SaaS product does exist**, so this document does not have to record that absence. [**Doulado**](https://doulado.co/) is a live commercial practice-management product sold to doulas, priced Starter $19/month, HIPAA Premium $29/month, Impact custom, with "2 Months Free" on annual — **observed** on the homepage. [**Enginehire**](https://enginehire.io/doula-business-software/) sells a staffing platform with a doula vertical. Neither changes the pattern; both put the money terms in the ToS.

### Practice Better

- Footer "Company" column carries **Privacy** (`/privacy`), **Terms** (`/terms`), **Contact Us** (`/contact`), **Help Center** (`https://help.practicebetter.io/hc/en-us/`), **Trust Center**, **System Status**, **Cookies**. **Observed** — no refund link, no `/support`, no `/legal`.
- The sitemap confirms the whole legal set: `/privacy`, `/terms`, `/terms-client`, `/terms-charting-assistant`, `/terms-api`, `/affiliate-agreement`, plus `/summer-sale-terms-conditions`. **Observed.** Five terms documents, zero refund documents.
- The refund position is a sentence inside `/terms` — **observed**:

> Certain refund requests for Subscriptions may be considered by Green Patch Inc. on a case-by-case basis and granted at the sole discretion of Green Patch Inc.

> You may cancel your Subscription renewal either through your online account management page or by contacting Green Patch Inc.'s customer support team.

- The contact email and the Help Center link both appear **on the terms page itself**, alongside the refund sentence. **Observed.**
- `https://www.practicebetter.io/support` returns **404**. **Observed** by HTTP probe.

### SimplePractice

- Footer has a dedicated **"Help & Support"** column — Help Center (`support.simplepractice.com/hc/en-us`), FAQs (`/features/faqs/`), product tutorials, Trust Center — and a separate **"Legal & Policy"** column: Terms (`/terms/`), SMS Terms (`/sms-terms/`), Privacy (`/privacy/`), BAA (`/baa/`), a Do-Not-Share opt-out, system status. **Observed.** Support and legal are deliberately two different columns. No refund link in either.
- `https://www.simplepractice.com/support` **301s to `/features/support/`**, which is a marketing page — "SimplePractice™ Live Customer Support for Your Practice" — selling the quality of the support offering. It contains **no refund policy and no email address**; it points at the Help Center. **Observed.** This is the single most useful data point for our own URL decision.
- The refund position is inside `/terms/`. The page is JavaScript-rendered and the first fetch returned navigation only; the text below was **observed** by pulling the raw HTML and reading the embedded document:

> You will not be entitled to any refund on termination or expiration of the Agreement. All payments once made to SimplePractice shall have been earned by SimplePractice as of the date of payment. You will not be entitled to any refund or credits for the partial use of the Service[.]

> Upon termination or expiration of this Agreement (which will automatically result in termination of Your Account), You will not receive any refund of any amounts previously paid and You will remain liable for any charges incurred or unpaid amounts owed by You to SimplePractice.

> Under no circumstances, will You be entitled to compensation or a refund for any interruption, suspension or termination[.]

### Jane App

- Footer legal block: **Terms of Use** (`/legal/terms-of-use`), **Privacy Policy** (`/legal/privacy-notice`), **Cookie Policy** (`/legal/cookie-policy`), plus Security & Trust (`/security-and-trust`) and a status page. **Observed.** Jane is the one site in the sample using a `/legal/` **path prefix** — and even so, there is no `/legal/refunds`.
- `https://www.jane.app/support` returns **404**. **Observed.** Support lives at `/contact`, and the footer publishes two phone numbers directly.
- Refund position, inside `/legal/terms-of-use` — **observed**:

> Except as set forth below under Termination, all fees are non-refundable.

> If a Subscriber terminates its subscription due to a breach by Jane or Jane discontinues the Services, we will refund any fees you had pre-paid for the remaining unused portion of your subscription term.

- The terms page itself carries the support email, a postal address (Jane Software Inc., 500 - 138 13th St E., North Vancouver, BC V7L 0E5 Canada) and the phone number 1-844-310-5263. **Observed.**

### Cliniko

- Cliniko is the closest thing in the sample to a policy *hub*: the footer's Resources column links **Policies** at `https://www.cliniko.com/policies/`. **Observed.** The page title is literally "Policies" and it links exactly three documents: [Terms](https://www.cliniko.com/policies/terms/), [Privacy](https://www.cliniko.com/policies/privacy/), [Cookies](https://www.cliniko.com/policies/cookies/). No refund document.
- Support is a separate footer column: community forum (`/community/`), help centre (`/help/`), FAQ (`/faq/`), and a plain `mailto:info@cliniko.com`. **Observed.**
- `https://www.cliniko.com/support` returns **404**. **Observed.**
- Refund position, inside `/policies/terms/` — **observed**:

> If you cancel your account before the end of your currently paid month, the Service will end immediately and you will not be charged again. You will not be entitled to a refund.

> Refunds will not be issue for unused SMS reminder credits if you cancel your account.

(The typo is Cliniko's.) That second sentence matters more than its length suggests: **Cliniko sells prepaid SMS credits and declares them non-refundable.** A near-direct competitor has already answered our exact question, and answered it the opposite way.

- The terms page also carries `support@cliniko.com` and the legal entity, Red Guava Pty. Ltd., ABN 56 147 311 466. **Observed.**

### Halaxy

Halaxy is the sample's clearest **absence**, and it is instructive.

- The homepage footer contains **no legal links at all** — no terms, no privacy, no policy. Grepping the raw homepage HTML for any href containing `term`, `polic`, `privacy`, `legal`, `refund`, `support` or `contact` returns exactly two hits: `/feature/24-hour-customer-support` (a marketing page) and `https://support.halaxy.com/` (the help centre). **Observed.**
- The terms exist, but they are reachable only by search or by signup flow, at jurisdiction-split URLs: `https://www.halaxy.com/terms/practitioner-au`, `.../practitioner-us`, `.../consumer-us`, and an EU host `https://eu.halaxy.com/terms/practitioner-ie`. Requesting `practitioner-us` served the `practitioner-au` document, so the quote below is labelled as the **AU practitioner terms**. **Observed.**
- Halaxy sells **prepaid "Halaxy Credits"** and takes the standard line — **observed**, AU practitioner terms:

> If you terminate your account with us, you will not be entitled to any refund for unused Halaxy Credits allocated to your account.

> You are not entitled to the cash equivalent value of any Halaxy Credit that has been credited to your account for no fee.

- The `consumer-us` terms were fetched and contain **no** refund, credit, cancellation or prepaid language at all; the only money sentence is a liability cap. **Absent**, verified by fetch.
- Contact: `community@halaxy.com`, `+61 1800 984 334`, `https://support.halaxy.com/`, `privacy@halaxy.com`. **Observed** — but on the terms page and in nav, not in a footer.

### Doulado — the doula-specific competitor

- Footer: **Terms of Service** (`https://doulado.co/terms-of-service/`), **Privacy Policy** (`/privacy-policy/`), **Disclosure Policy** (`/security-vulnerability-disclosure-policy/`), **Help Center** (`https://help.doulado.co`), Blog, Videos, Book a Demo. **Observed.** No refund page, no `/support` — `https://doulado.co/support` returns **404**. **Observed.**
- Refund position, inside `/terms-of-service/` — **observed**:

> Our refund policy is guided by our commitment to equitable access and social impact. To maintain low costs and provide a sustainable service, we are unable to offer refunds. However, you can cancel or pause your plan anytime should your needs change.

> You will not be entitled to any refund on termination or expiration of the Agreement.

> Upon termination by You or by Doulado of your Account, You will not receive any refund of any amounts previously paid.

- Contact on the same page: `support@doulado.co`, a Sheridan, Wyoming mailing address, and the help centre. **Observed.**

**A note worth recording.** Doulado's second and third quotes are word-for-word SimplePractice's. Both are running the same lawyer's SaaS terms template, and that template puts refunds in clause 24 of a Terms of Service. The apparent unanimity in this group is partly a unanimity of *boilerplate* — which is itself the reason a standalone refund page is rare. The template does not produce one, so nobody has one.

## 3. Businesses that sell prepaid credits — the wording, verbatim

This is the group whose wording is directly useful, because our refund question is about an unspent prepaid balance rather than a cancelled month. Seven businesses were checked here, plus the two prepaid-credit positions already found among the competitors (Cliniko's SMS credits, Halaxy Credits). Every quote below is **observed** from the named page.

### Anthropic — standalone credit terms, incorporated by reference

[`https://www.anthropic.com/legal/credit-terms`](https://www.anthropic.com/legal/credit-terms), "Supplemental Credit Terms". This is one of only two pages in the entire survey that is a **dedicated legal page about credits**, and it is not free-standing: it is incorporated by reference from [`/legal/commercial-terms`](https://www.anthropic.com/legal/commercial-terms).

> Credits issued by Anthropic, which includes Usage Credits, Promotional Credits and any other credits Anthropic offers ("Credits") are non-refundable.

> Usage Credits expire one calendar year from the date Anthropic sends the Confirmation Notice or otherwise issues the Credits.

> Promotional Credits expire at the time indicated when issued (or otherwise one calendar year from when issued if no time is specified).

### OpenAI — the other standalone credit page

[`https://openai.com/policies/service-credit-terms/`](https://openai.com/policies/service-credit-terms/), "Service credit terms". Same shape: its own policy page, separate from the main Services Agreement. (Direct fetch returned HTTP 403; the content was read through a reader proxy — **observed**, but flagged as not fetched from the origin directly.)

> All sales of Services, including sales of prepaid Services, are final. Service Credits are not refundable except where required by law

> expire one year after the date of purchase or issuance if not used, unless otherwise specified at the time of purchase

### ElevenLabs

[`https://elevenlabs.io/terms-of-use`](https://elevenlabs.io/terms-of-use), "ElevenLabs Terms of Service (non-EEA)". Inside the ToS.

> Except where required by applicable law, Prepaid Credits purchases are non-refundable, and no refunds or credits will be provided for unused, partially used, expired, or forfeited Prepaid Credits.

> Unless otherwise specified at the time of purchase or issuance, all Prepaid Credits expire twelve (12) months after the date of purchase or issuance, as applicable.

### Replicate

[`https://replicate.com/terms`](https://replicate.com/terms), "Terms of Service". Inside the ToS.

> All Prepaid Balances are non-refundable, except as otherwise required by law or as expressly set forth in this Agreement.

> Each payment to fund the Prepaid Balance will expire at the end of twelfth (12th) month after the date of payment if not fully used.

> Upon expiration, any unused amounts will be forfeited and will not be refunded or credited to Customer.

### Bunny.net — the "never expire" pole

[`https://bunny.net/tos/`](https://bunny.net/tos/), "Terms of Service | bunny.net", with the detail on [`https://bunny.net/docs/faq`](https://bunny.net/docs/faq).

> All payments are paid in advance and are not refundable.

> bunny.net operates on a prepaid model, so payments are generally non-refundable.

> Your credits are safe and will never expire.

> When your trial period ends, any remaining trial credits are removed from your account.

Bunny is the interesting hybrid: **no refunds, but no expiry either**. The money stays yours in the form of service, forever, and never in the form of money. The FAQ adds a discretionary exception for mistaken payments or unused balance within one month of account creation.

### Postmark — the only refund found, and it is narrow

[`https://postmarkapp.com/terms-of-service`](https://postmarkapp.com/terms-of-service), "Terms of Service | Postmark". The page carries **two co-existing terms versions**. The current terms, effective 10 December 2024, contain **no prepaid-credit language at all** — the word "credit" appears only in the sense of credit card. **Absent**, verified by fetch. The credit language survives only in the legacy terms effective 3 May 2022, still published on the same page for existing customers:

> If you are not satisfied with the Service, AC PM will issue a refund for any unused Postmark credits from your first purchase within 90 days of that purchase.

> Your credits to send emails will never expire.

Subsequent purchases are "not eligible for refunds". So even the field's single refund is: **first purchase only, ninety days, and only under a superseded version of the terms.**

### Twilio

[`https://www.twilio.com/en-us/legal/tos`](https://www.twilio.com/en-us/legal/tos), "Twilio Terms of Service", section 3.3:

> payment obligations are non-cancelable and fees, Taxes, and Communications Surcharges (collectively, "Fees"), once paid, are non-refundable.

Twilio's **expiry** rule is **unverified**. The help-centre articles that would carry it are client-rendered and returned empty shells to every fetch attempted. Third-party claims of a twelve-month expiry exist and are deliberately not repeated here as fact.

### Negative results in this group

- **Telnyx** (`telnyx.com/terms-and-conditions`) — fetched; contains **no** mention of refunds, prepaid balances or expiry. **Absent.**
- **Resend** (`resend.com/legal/terms-of-service`) — fetched; a subscription non-refundability clause only, no prepaid-credit mechanism. Not a comparable.
- fal.ai, Bandwidth and Loops.so were **not tested**.

### The distribution, and where our policy sits

| Business | Unspent balance refundable? | Expires? |
| --- | --- | --- |
| Anthropic | No | 12 months |
| OpenAI | No | 12 months |
| ElevenLabs | No | 12 months |
| Replicate | No | 12 months, then forfeited |
| Twilio | No | unverified |
| Bunny.net | No (narrow discretionary exception) | Never |
| Postmark (legacy terms) | First purchase only, 90 days | Never |
| Postmark (current terms) | Silent | Silent |
| Cliniko (SMS credits) | No | not stated |
| Halaxy (Halaxy Credits) | No | not stated |
| **Doula Cloud (proposed)** | **Yes, 3 years, at price paid** | **3 years** |

**On refundability, our position is not generous — it is outside the field.** Nine of the ten observed positions are "no refund on unspent credits", stated flatly. The single exception is capped at one purchase and ninety days, and lives in a superseded document. Nobody in this sample refunds an unspent balance on request, at any point, let alone for three years. That is a real and deliberate difference, not a matter of degree.

**On expiry, three years is generous but not unique.** The field has two poles and nothing in between: four businesses expire credits at **twelve months** (Anthropic, OpenAI, ElevenLabs, Replicate) and two never expire them at all (Bunny.net, legacy Postmark). Three years is three times the finite-expiry norm, and still less generous than the never-expire pole. No business in the sample used a multi-year window — 24 or 36 months appear nowhere.

**The pairing is what has no precedent.** Every business here either keeps the money and eventually voids the credit (12-month expiry), or keeps the money and honours the credit forever (never expires). Ours does neither: it gives the money back on request for three years, and then voids what is left. It is the most customer-favourable position in the sample on both axes at once. Given a doula practice buying credits it may not spend for a season, that is defensible on its own terms — but it should be chosen, not assumed to be normal, because it is not.
