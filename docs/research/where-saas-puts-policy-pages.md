# Where real software businesses put the content Stripe asks for

Question: **a pre-launch B2B SaaS needs one publicly reachable page carrying the business name, a description of the service, a customer service contact, and a refund/dispute/cancellation position — Stripe's activation requirement. Where do real businesses actually put that content, and is a single consolidated page for it normal, unusual, or a smell?**

Researched **28 August 2026** by visiting the sites and reading the pages. Every claim below is labelled by how it was verified: **observed** means the page was fetched and the quoted text came out of it; **absent** means the page was fetched or probed and the thing looked for was not there; **unverified** means a first-party fetch failed and no secondary source was substituted. There are no claims sourced to review sites, cancellation-service blogs, or search-result summaries.

Twenty-three sites were fetched across three groups: practice-management competitors, small indie SaaS billing through Stripe, and businesses that sell a prepaid credit balance rather than a subscription.

## The five findings that matter

1. **Nobody has the page we are about to build.** Across twenty-three sites, **zero** carried a thin consolidated "here is our business, here is what we sell, here is how to reach us, here is our refund position" page distinct from both marketing and legal. Our page is an invention. It is not a smell — nothing about it misleads a reader, and Stripe's own checklist would be satisfied by it — but we should stop describing it as following a convention, because there is no convention to follow.
2. **The refund position lives inside the Terms of Service, and that is not a considered choice — it is a template.** Eighteen of the twenty verified positions are a section of a ToS. Zero of twenty-three sites have a `/refund-policy`, `/refunds` or equivalent URL. Doulado and SimplePractice, two unrelated companies, publish word-for-word identical refund clauses; the position is in the ToS because the lawyer's template put it there.
3. **`/support` in the wild means "how to reach a human", never "our refund rule".** It was probed or observed on eight domains: 404 on five, a redirect to a marketing page on one, and one real page — [Buttondown's](https://buttondown.com/support) footer-linked "Customer Support" page carrying two email addresses. That is a genuine precedent for our URL, and it covers the contact half of the requirement exactly. It carries no policy on any of the eight.
4. **Prepaid credits are non-refundable almost everywhere, including at two direct competitors.** Eight of the nine observed prepaid-credit positions refuse refunds on unspent balance outright — among them **Cliniko's SMS credits and Halaxy Credits**. Expiry has two poles and nothing between: twelve months (Anthropic, OpenAI, ElevenLabs, Replicate) or never (Bunny.net, legacy Postmark). Our "refundable within three years, at the price paid, expiring at three years" is not a generous version of the norm. It is a position nobody in the field holds.
5. **Stripe does not appear to enforce this uniformly.** [Buttondown](https://buttondown.com/legal/terms) bills through Stripe, publishes **ten** separate legal pages, and states **no refund position anywhere on its site** — verified by fetching its terms, its support policy, its support page and its pricing page. This work is about doing the thing properly, not about clearing a gate that would otherwise stop us.

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

Eight sites: Practice Better, SimplePractice, Jane App, Cliniko, Halaxy, Healthie, Doulado and Enginehire. **Every one whose refund position was verified — seven of the eight — states it inside a Terms of Service document.** None has a standalone refund page. None has a consolidated processor-facing page.

**A doula-specific SaaS product does exist**, so this document does not have to record that absence. [**Doulado**](https://doulado.co/) is a live commercial practice-management product sold to doulas, priced Starter $19/month, HIPAA Premium $29/month, Impact custom, with "2 Months Free" on annual — **observed** on the homepage. [**Enginehire**](https://enginehire.io/doula-business-software/) sells a staffing platform with a doula vertical. Doulado does not change the pattern: its money terms are in its ToS. Enginehire's terms document was not read, so it is counted as a site visited and not as a verified position.

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


### Healthie — a seventh competitor, same shape

- Footer legal set: **Terms of Services** (`https://www.gethealthie.com/terms`), **Privacy Policy** (`/privacy`), **Business Associate Agreement** (`/baa`). Support set, kept separate: **Help Center** (`https://help.gethealthie.com/`), **Contact Us** (`/contact`), **Platform Status**, **FAQs** (`/healthie-faq`). **Observed.** No refund page.
- `https://www.gethealthie.com/support` returns **404**. **Observed.**
- Refund position, inside `/terms` — **observed** from the raw page:

> All fees are non-refundable and non-transferrable, including annual contracts. In the case of canceling an annual contract, subscription will end at the 365th day from payment.

### The competitor group, probed for the pages that are not there

The absence is worth stating as a probe result, not an impression. Each of these was requested and returned **404**: `practicebetter.io/support`, `practicebetter.io/legal`, `jane.app/support`, `jane.app/legal/refunds`, `cliniko.com/support`, `cliniko.com/policies/refunds/`, `doulado.co/support`, `gethealthie.com/support`, `simplepractice.com/legal`. The one `/support` that resolved, `simplepractice.com/support`, **301s to a marketing page**. **Observed** by HTTP probe.

## 2. Small indie SaaS billing through Stripe

Six sites: Buttondown, Pirsch, Fathom Analytics, Plausible, Tinylytics, Baremetrics. These are the closest structural match to our size and situation, and they were the group most likely to have invented a standalone refund page cheaply. **None of them did.** All six put the position — where they have one at all — inside the Terms of Service.

**One correction to the brief.** [Plausible](https://plausible.io/terms) states on its own terms page that "Our payment process is handled by **Paddle**", not Stripe. **Observed.** It is kept in the sample because its URL shape is still evidence, but it is not a Stripe-activation data point.

### Buttondown — the one site with a footer-linked `/support`

This is the most directly relevant single site in the survey, and it cuts both ways.

- Footer carries, verbatim: About, Blog, Testimonials, Docs, Features, Integrations, Alternatives, Guides, Climate, Brand guidelines, Kudos, **Terms of service** (`/legal/terms`), Use cases, **Privacy policy** (`/legal/privacy`), **Customer support** (`/support`), Status, Open source. **Observed** from the raw footer markup.
- [`https://buttondown.com/support`](https://buttondown.com/support) exists, is titled **"Customer Support — Buttondown"**, is reached from the footer, and carries two direct email addresses: `support@buttondown.com` and `concierge@buttondown.com`. **Observed.** So `/support` as a footer-linked, human-facing contact page is a real pattern with a real precedent, in a Stripe-billed indie SaaS we already use.
- But it carries **no refund or cancellation position**. Neither does anything else on the site. The 800-URL sitemap contains no `/refund`, `/cancel` or `/billing` URL; `/legal/terms` contains **zero** occurrences of "refund", "cancel", "billing", "subscription", "payment", "prorate", "downgrade" or "trial"; `/legal/support-policy` and `/pricing` have none either. **Absent**, verified by fetching all four pages and searching the text.
- Buttondown's legal set is unusually deep for its size — `/legal/acceptable-use-policy`, `/legal/adult-content-policy`, `/legal/cookies`, `/legal/data-processing-agreement`, `/legal/gdpr-eu-compliance`, `/legal/law-enforcement-requests`, `/legal/subprocessors`, `/legal/support-policy`, `/legal/privacy`, `/legal/terms` — and *still* has no refund policy. **Observed** from the sitemap.

That last point deserves its own sentence. **Buttondown, a Stripe-billed SaaS with ten legal pages, does not publish a refund policy at all.** Whatever Stripe enforces at activation, it is not enforcing this uniformly.

### Fathom Analytics

- Footer: **Privacy** (`/legal/privacy`), **Terms** (`/legal/terms`), GDPR compliance, **Contact** (`/about/contact`), **Help Centre** (`/docs`), Imprint, Status, Sitemap. **Observed.** No `/sitemap.xml`; `robots.txt` points at `/sitemap-index.xml`. No standalone refund URL anywhere in it.
- Refund position, a **titled subsection inside** `/legal/terms` — **observed**:

> Refunds
> All Fees paid by you to us are non-refundable, except if required by law.

> In the event of any account suspension or deletion, you acknowledge and agree that you will not be entitled to any refunds of any amounts previously paid to us.

- `support@usefathom.com` appears on the terms page itself. `/about/contact` is titled "Get in touch with us" and offers an email address only — no form, no chat: "We do not offer live demos or phone calls at this time." **Observed.**

Fathom is the pattern our page is closest to in spirit: a heading called **Refunds**, with the position under it — but the heading is a section of the terms document, not a URL.

### Plausible

- Footer: **Contact** (`/contact`), **Privacy policy** (`/privacy`), **Data policy** (`/data-policy`), **Terms** (`/terms`), **DPA** (`/dpa`), Security (`/security`), Compliance (`/compliance`), Imprint (`/imprint`), Documentation, Status. **Observed.** Nine policy-ish URLs, none of them about refunds.
- Two **titled subsections inside** `/terms` — **observed**:

> Our payment process is handled by Paddle, which also manages customer service inquiries and returns. [...] Fees paid hereunder are non-refundable.

> You are solely responsible for properly canceling your account. An email to cancel your account is not considered cancellation. [...] If you cancel the service before the end of your current paid period, your cancellation will take effect at the end of the current billing cycle and you will not be charged again.

- The terms page carries no raw email; it links to `/contact`, which is a topic-selector help page: "Select what your question is about for a direct answer, or email us at hello@plausible.io. We typically reply within one business day." **Observed.**

### Pirsch

- Footer: **Terms and Conditions** (`/terms`), **Privacy Policy** (`/privacy`), DPA (PDF, English and German), **Legal Notice** (`/legal`), **Contact Us** as a plain `mailto:support@pirsch.io`. **Observed.** Note that `/legal` here is a German *Impressum*-style legal notice — an identity disclosure — not a policy hub.
- The `/terms` page is Iubenda-generated. The literal word "refund" never appears; the position is expressed as EU withdrawal rights — **observed**:

> Users who are European Consumers are granted a statutory cancellation right under EU rules, to withdraw from contracts entered into online (distance contracts) within the specified period applicable to their case, for any reason and without justification.

> The suspension or deletion of User accounts shall not entitle Users to any claims for compensation, damages or reimbursement.

- `support@pirsch.io` appears on the same page. **Observed.**

### Tinylytics — the one generous refund policy in the whole survey

- A one-person product. Homepage footer: Documentation (`/docs`), Changelog, RSS, `llms.txt`, the creator's personal site, **Terms** (`/docs/terms`), **Privacy** (`/docs/privacy`). **No contact or support link in the homepage footer at all.** **Absent**, verified against the raw footer markup.
- Refund position, a **titled subsection inside** `/docs/terms` — **observed**:

> Refund Policy
> We offer a 30-day, no-questions-asked refund policy on initial payments.

> Violations of these Terms of Service will result in immediate account termination without refund.

- The contact route is a mailto that appears only in the *doc pages'* own mini-footer, not on the homepage. **Observed.** By Stripe's checklist this is the weakest contact surface in the sample.

### Baremetrics

- Footer LEGAL submenu: **Privacy** (`/privacy`), Security (`/security`), **Terms of Use** (`/terms`), GDPR (`/gdpr`). Help & Support submenu: Help Center (`help.baremetrics.com`), Cancellation Insights, Status. **Contact** is a plain `mailto:hello@baremetrics.com`. **Observed.** No standalone refund URL in the sitemap.
- Refund position, **inline clauses inside** `/terms` with no distinct heading — **observed**:

> You can cancel anytime, but there are no refunds.

> Payments are non-refundable. There will be no refunds or credits for partial months of service, upgrade/downgrade refunds, or refunds for months unused with an open account.

> Please call +1-855-948-6210 or schedule a date and time to receive a phone or video call. Any other emails or chat requests to cancel Your account will not be considered a cancellation. You will not receive any refunds if You cancel Your account.

- **Baremetrics requires a phone or video call to cancel**, and says explicitly that email and chat do not count. That is the most restrictive cancellation mechanism observed anywhere in this survey, and it is worth knowing that a well-regarded SaaS ships it. Both `hello@baremetrics.com` and the phone number appear on the terms page. **Observed.**
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

**On refundability, our position is not generous — it is outside the field.** Eight of the nine observed positions are "no refund on unspent credits", stated flatly. The single exception is capped at one purchase and ninety days, and lives in a superseded document. Nobody in this sample refunds an unspent balance on request, at any point, let alone for three years. That is a real and deliberate difference, not a matter of degree.

**On expiry, three years is generous but not unique.** The field has two poles and nothing in between: four businesses expire credits at **twelve months** (Anthropic, OpenAI, ElevenLabs, Replicate) and two never expire them at all (Bunny.net, legacy Postmark). Three years is three times the finite-expiry norm, and still less generous than the never-expire pole. No business in the sample used a multi-year window — 24 or 36 months appear nowhere.

**The pairing is what has no precedent.** Every business here either keeps the money and eventually voids the credit (12-month expiry), or keeps the money and honours the credit forever (never expires). Ours does neither: it gives the money back on request for three years, and then voids what is left. It is the most customer-favourable position in the sample on both axes at once. Given a doula practice buying credits it may not spend for a season, that is defensible on its own terms — but it should be chosen, not assumed to be normal, because it is not.

## 4. The URL distribution, counted

Twenty-three sites. Every single one publishes a terms document. The path shapes:

| URL shape | Count | Sites |
| --- | --- | --- |
| Terms at the root — `/terms`, `/terms/`, `/terms-of-service`, `/terms-of-use`, `/terms-and-conditions`, `/tos/` | **14** | Practice Better, SimplePractice, Healthie, Doulado, Halaxy, Enginehire, Pirsch, Plausible, Baremetrics, Replicate, Postmark, Telnyx, ElevenLabs, Bunny.net |
| Terms under a `/legal/` prefix | **6** | Jane App, Buttondown, Fathom, Resend, Twilio, Anthropic |
| Terms under a `/policies/` prefix | **2** | Cliniko, OpenAI |
| Terms under `/docs/` | **1** | Tinylytics |
| A standalone refund page — `/refund-policy`, `/refunds`, `/legal/refunds`, `/policies/refunds` | **0** | none |
| A standalone *credit* terms page, incorporated by reference into the terms | **2** | Anthropic (`/legal/credit-terms`), OpenAI (`/policies/service-credit-terms/`) |
| A `/policies` **hub page** listing the legal documents | **1** | Cliniko (`https://www.cliniko.com/policies/`) |
| A `/legal` page that is an identity disclosure (German *Impressum*), not a policy hub | **1** | Pirsch |
| `/contact` as its own page | **5** | Practice Better, Jane App, Healthie, Fathom (`/about/contact`), Plausible |
| A `mailto:` published directly in the footer | **4** | Cliniko, Jane App, Pirsch, Baremetrics |
| A separate help centre on `help.*` / `support.*` / `/help` / `/docs` | **12** | Practice Better, SimplePractice, Jane App, Cliniko, Halaxy, Healthie, Doulado, Buttondown, Fathom, Plausible, Baremetrics, Tinylytics |
| `/pricing` carrying any refund or cancellation language | **0** | none |

**`/support` specifically.** Probed or observed on eight domains. It **404s on five** (Practice Better, Jane App, Cliniko, Doulado, Healthie), **301s to a marketing page on one** (SimplePractice → `/features/support/`), and **exists as a real page on one** ([Buttondown](https://buttondown.com/support), "Customer Support — Buttondown", footer-linked, two direct email addresses). Halaxy uses the `support.halaxy.com` subdomain for its help centre instead. **In not one of these cases does `/support` carry a refund or cancellation position.**

The headline number: **`/terms` in some form is effectively universal, and `/refunds` in any form does not exist.** Twenty-three sites, zero refund URLs.

## 5. Is the refund position consolidated, or spread? — the crux

**It is consolidated, but onto the Terms of Service, not onto a page of its own.** Of the twenty sites where a refund position could be verified at all, **eighteen state it inside a Terms of Service document**. The two exceptions, Anthropic and OpenAI, moved *credit* terms to a supplementary legal page — and both of those are incorporated by reference into the main terms, so they are appendices to a contract, not standalone policy pages.

There is a second, softer convention underneath that, and it is the one worth copying. **The position sits under a heading that names it.** Fathom's is literally headed `Refunds`. Tinylytics' is headed `Refund Policy`. Plausible's is headed `Payment, refunds, upgrading and downgrading terms`. The field's answer to "where do I find the refund rule" is not "a URL" — it is "a named section of the terms". Baremetrics is the counter-case: its refund clauses are inline with no heading, and finding them takes a text search.

So our plan of a standalone page is the unusual one, and now the reason is on the record: **the refund position is produced by the terms-of-service template, and the template emits a section, not a page.** Doulado's and SimplePractice's identical wording — word-for-word across two unrelated companies — is that template showing itself. Nobody sat down and decided the refund position belonged in the ToS. It was already there.

**One important dissent from the whole pattern.** [Buttondown](https://buttondown.com/legal/terms) has ten separate legal pages and **states no refund position anywhere at all** — not in the terms, not on `/support`, not on `/legal/support-policy`, not on `/pricing`. **Absent**, verified by fetching all four. Buttondown bills through Stripe. Whatever Stripe checks at activation, a published refund policy is evidently not enforced uniformly, so this work is about doing the thing properly rather than about clearing a gate that will otherwise stop us.

## 6. Where the customer service contact lives

**It is an email address, and it very often sits on the same page as the refund position.** **Eight sites publish a direct email address on the very same page as their refund position**: Practice Better, Jane App, Cliniko, Halaxy, Doulado, Pirsch, Fathom and Baremetrics. Jane App and Baremetrics add a phone number there; Jane App and Doulado add a postal address. SimplePractice puts only a help-centre link on its terms. So the "contact and refund policy together" instinct behind our consolidated page is not wrong — it is simply that everyone achieves it by putting the contact *into the terms document* rather than by pulling the refund rule out into a support page.

The contact surfaces observed, in order of how common they are:

- **A direct `mailto:` email address** — the majority. `support@cliniko.com`, `support@doulado.co`, `support@pirsch.io`, `support@usefathom.com`, `support@buttondown.com`, `hello@baremetrics.com`, `hello@plausible.io`, `community@halaxy.com`.
- **A separate help centre** on `help.*` or `support.*` — twelve of twenty-three. Universal among the practice-management competitors, all of them Zendesk-shaped.
- **A phone number** — Jane App (two, in the footer), Halaxy, Baremetrics.
- **A contact form as the only route** — observed nowhere. This matters, because it is the one thing Stripe's checklist names as insufficient on its own.

The weakest contact surface in the sample is **Tinylytics**, whose homepage footer has no contact or support link at all; the mailto appears only in the documentation pages' own footer.

## 7. Is there a page whose evident purpose is to satisfy a payment processor?

**No. Not one, across twenty-three sites.** This is a clean answer and it should be treated as one.

Nothing in the survey is a thin, consolidated "here is our business, here is what we sell, here is how to reach us, here is our refund position" page sitting apart from both the marketing site and the legal documents. The closest four things, and why each falls short:

- **[Cliniko `/policies/`](https://www.cliniko.com/policies/)** — a hub page titled "Policies" that links Terms, Privacy and Cookies. It is a **directory of three links**, not a content page. It carries no service description, no contact, no refund position.
- **[Buttondown `/support`](https://buttondown.com/support)** — a footer-linked page titled "Customer Support" carrying two direct email addresses. It has the **contact half** of the requirement and none of the policy half.
- **[Buttondown `/legal/support-policy`](https://buttondown.com/legal/support-policy)** — a page describing what support will and will not help with. Adjacent, but it is about the *scope of support*, not about money.
- **[SimplePractice `/features/support/`](https://www.simplepractice.com/features/support/)** — a marketing page selling the support offering. No policy, no email.

So the page we are about to build is an invention. That is not a smell — there is nothing misleading or unusual about it to a reader, and Stripe's own checklist would be satisfied by it — but we should stop describing it as following a convention. There is no convention to follow.

## 8. How the page is reached

**Footer, almost without exception.** Of the fourteen competitor and indie sites, **thirteen link their terms document from the site footer**. Reaching it any other way was not observed: no site put its terms in the main navigation, and no site made it in-product only.

The one exception is **Halaxy**, and it is a total one. Its homepage footer contains **no legal links whatsoever** — the raw HTML yields exactly two matches for any href containing `term`, `polic`, `privacy`, `legal`, `refund`, `support` or `contact`, and both are the help centre and a marketing feature page. **Absent**, verified against the raw markup. Halaxy's terms are reachable only by search engine or by going through the signup flow, and they are split by jurisdiction across four URLs. If Stripe reviewed Halaxy's public site the way it says it does, it would not find the fulfilment policy.

Support and legal are usually **two separate footer groups**, not one. SimplePractice has a "Help & Support" column and a "Legal & Policy" column. Cliniko has a "Support" column and a "Resources" column that holds Policies. Baremetrics has a "Help & Support" submenu and a "LEGAL" submenu. Buttondown is the site that mixes them, listing "Terms of service", "Privacy policy" and "Customer support" side by side in one flat footer block.

## 9. Recommendation — `/support` is fine, and the thing to change is the heading

**Keep `/support`.** The evidence does not point at a different URL, and here is the reasoning it actually supports.

**No URL is precedented for this page, so precedent cannot pick one.** Twenty-three sites, zero consolidated processor-facing pages, zero refund URLs. `/refund-policy` and `/refunds` would be inventions with no observed use anywhere. `/policies` has exactly one precedent (Cliniko) and there it names a *directory of legal documents* — a promise our page would also break, because it would be one page of positions rather than a hub of contracts. So the choice comes down to which name misleads a reader least and which name is still correct in January.

**`/support` has a real precedent for the half of the page that is permanent.** [Buttondown](https://buttondown.com/support) — a Stripe-billed indie SaaS of roughly our shape, which we already use — publishes a footer-linked page at exactly `/support`, titled "Customer Support", whose job is to give the reader direct email addresses. That is the business-name-plus-contact half of Stripe's requirement, at our chosen URL, done by a peer.

**And `/support` is the URL that gets *more* correct over time, not less.** The refund content is on this page because the real Terms of Service does not exist yet. When the lawyer-drafted terms land, the refund and cancellation position moves into them, where the entire field keeps it — and what remains at `/support` is a business description and a contact route, which is precisely Buttondown's page. A `/policies` URL would have to be restructured at that point into a hub; `/support` would not have to change at all. That asymmetry is the deciding argument.

**Both rejections in the original reasoning survive contact with the evidence.** `/about` is used for company-story pages on Practice Better, Jane App, Healthie, Fathom, Pirsch and Plausible — six observations, all marketing, none carrying policy. Reserving it for January's marketing site matches what everyone does. And naming the page `/terms-of-service` when it is not the terms of service would be the one genuinely misleading option on the table, given that all twenty-three sites use that name for a real contract.

Three things to do on the page itself, each of which the evidence supports directly.

**Give the refund position a heading that names it.** This is the convention the survey actually establishes, and it is a heading convention rather than a URL convention. Fathom heads its section `Refunds`. Tinylytics heads its `Refund Policy`. Plausible heads its `Payment, refunds, upgrading and downgrading terms`. A reader looking for the refund rule scans for that word. Put `Refunds and cancellation` on the page as a real `h2` with a stable anchor, and link the footer at that anchor as well as at the page — footer text along the lines of "Support" and "Refunds", both resolving into `/support`, costs nothing and removes the one real objection to the URL, which is that nobody would think to look under "Support" for a refund rule. Baremetrics is the cautionary case: its refund clauses have no heading and can only be found by text search.

**Publish a direct email address, not a form.** Stripe's checklist names this explicitly — "something besides contact forms" — and no site in the survey uses a form as its only contact route. An address on the page satisfies it outright.

**Say the business name and the legal entity.** Cliniko puts "Red Guava Pty. Ltd., ABN 56 147 311 466" on its terms page; Jane App puts its full postal address there; Doulado puts its Sheridan, Wyoming address. Stripe's checklist asks for the business address where one exists. This is cheap and every peer does it.

**On the refund position itself, one flag.** Section 3 establishes that "refundable within three years, at the price paid" has no precedent in the field — eight of the nine observed prepaid-credit positions refuse refunds outright, and the ninth is capped at ninety days on a first purchase under superseded terms. Two of those eight are **direct competitors**: Cliniko refuses refunds on unused SMS credits, and Halaxy refuses them on unused Halaxy Credits. Our position is not a generous version of the norm; it is a different position. That is a business decision and not a research finding, so this document does not argue against it — but the decision should be made in the knowledge that no competitor and no comparable does it, rather than in the belief that it is a customer-friendly variation on something standard.

## Every site visited

| # | Site | Group | Policy page URL | Refund position | Contact on that page | How reached |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Practice Better | competitor | `https://www.practicebetter.io/terms` | In ToS — discretionary, case by case | Email + Help Center | Footer |
| 2 | SimplePractice | competitor | `https://www.simplepractice.com/terms/` | In ToS — no refund, no proration | Help Center link | Footer ("Legal & Policy") |
| 3 | Jane App | competitor | `https://www.jane.app/legal/terms-of-use` | In ToS — non-refundable, prepaid refunded only on Jane's breach | Email, postal address, phone | Footer |
| 4 | Cliniko | competitor | `https://www.cliniko.com/policies/terms/` (hub at `/policies/`) | In ToS — no refund; **SMS credits non-refundable** | Email + legal entity | Footer ("Resources → Policies") |
| 5 | Halaxy | competitor | `https://www.halaxy.com/terms/practitioner-au` | In ToS — **Halaxy Credits non-refundable** | Email, phone, support portal | **Not linked from the footer at all** |
| 6 | Healthie | competitor | `https://www.gethealthie.com/terms` | In ToS — non-refundable, non-transferrable | Via `/contact` | Footer |
| 7 | Doulado (doula-specific) | competitor | `https://doulado.co/terms-of-service/` | In ToS — no refunds, cancel or pause any time | Email + postal address | Footer |
| 8 | Enginehire (doula vertical) | competitor | `https://enginehire.io/terms-and-conditions/` | **Not verified** — the terms URL was observed, the document was not read | Email + phone in footer | Footer |
| 9 | Buttondown | indie SaaS | `https://buttondown.com/legal/terms` | **None anywhere on the site** | `/support` page, two emails | Footer |
| 10 | Pirsch | indie SaaS | `https://pirsch.io/terms` | In ToS — EU withdrawal right; no "refund" language | Email on same page | Footer |
| 11 | Fathom Analytics | indie SaaS | `https://usefathom.com/legal/terms` | In ToS, heading `Refunds` — non-refundable | Email on same page | Footer |
| 12 | Plausible | indie SaaS (Paddle, not Stripe) | `https://plausible.io/terms` | In ToS, heading `Payment, refunds, upgrading and downgrading terms` | Link to `/contact` | Footer |
| 13 | Tinylytics | indie SaaS | `https://tinylytics.app/docs/terms` | In ToS, heading `Refund Policy` — **30-day no-questions-asked** | Mailto on doc footer only | Footer (Terms); contact absent from homepage footer |
| 14 | Baremetrics | indie SaaS | `https://baremetrics.com/terms` | In ToS, inline, no heading — no refunds; **cancellation requires a phone or video call** | Email + phone on same page | Footer ("LEGAL") |
| 15 | Anthropic | prepaid credits | `https://www.anthropic.com/legal/credit-terms` | **Standalone credit terms**, incorporated by reference — non-refundable, 1-year expiry | Not on that page | From `/legal/commercial-terms` |
| 16 | OpenAI | prepaid credits | `https://openai.com/policies/service-credit-terms/` | **Standalone credit terms** — not refundable, 1-year expiry | Not on that page | From the Services Agreement |
| 17 | ElevenLabs | prepaid credits | `https://elevenlabs.io/terms-of-use` | In ToS — non-refundable, 12-month expiry | Not on that page | Footer |
| 18 | Replicate | prepaid credits | `https://replicate.com/terms` | In ToS — non-refundable, 12-month expiry then forfeited | Not on that page | Footer |
| 19 | Bunny.net | prepaid credits | `https://bunny.net/tos/` and `https://bunny.net/docs/faq` | In ToS + FAQ — non-refundable, **credits never expire** | Support, via docs | Footer + docs |
| 20 | Postmark | prepaid credits | `https://postmarkapp.com/terms-of-service` | Legacy ToS only — refund of unused credits, first purchase, 90 days; credits never expire. **Current ToS silent on credits** | Not on that page | Footer |
| 21 | Twilio | prepaid credits | `https://www.twilio.com/en-us/legal/tos` | In ToS — non-refundable. **Expiry unverified** | Not on that page | Footer |
| 22 | Telnyx | prepaid credits | `https://telnyx.com/terms-and-conditions` | **None** — no refund, prepaid or expiry language at all | Not on that page | Footer |
| 23 | Resend | prepaid credits (not applicable) | `https://resend.com/legal/terms-of-service` | In ToS — subscription fees non-refundable; no credit mechanism | Not on that page | Footer |

## What was not established

- **Twilio's credit expiry.** Its help-centre articles are client-rendered and returned empty shells to every fetch. Third-party claims of a twelve-month expiry exist and are deliberately not repeated here as fact. A browser-driven read would settle it if it ever matters.
- **Enginehire's refund wording.** Its footer and legal URL set were read; the terms document itself was not fetched in full.
- **fal.ai, Bandwidth and Loops.so** were named as prepaid-credit candidates and never tested.
- **Halaxy's US and EU practitioner terms.** Requesting `/terms/practitioner-us` served the `practitioner-au` document, so only the Australian practitioner terms are quoted. The `consumer-us` terms were fetched and contain no refund or credit language at all.
- **SimplePractice's own help-centre articles on cancellation** returned HTTP 403 to every attempt. The refund position quoted here is taken from the terms document's raw HTML instead, which is first-party and sufficient; no secondary source was substituted for the article.
