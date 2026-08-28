# What Stripe actually checks when it reviews a business website

Research on the mechanics of Stripe's business-website review, for a US platform about to submit a live account for activation while its public site is a pre-launch teaser. Checked 28 August 2026.

## Method, and what is first-party

Every claim below is labelled by how it was verified.

- **Read first-party** — quoted from `docs.stripe.com`, `support.stripe.com`, the Stripe API reference, or the Stripe Services Agreement at `stripe.com/legal/ssa`. Support-centre articles were read by extracting the `QuestionPage` payload out of the page HTML, so the text below is the article's own source Markdown, not a rendering.
- **Checked absence** — a thing Stripe does not say, with the pages that were actually read named. An absence is only recorded where the page list makes the search auditable.
- **Inference** — reasoning from what Stripe says, marked as such and never stated as Stripe's own position.

No blog posts, no integration guides, no forum threads. Nothing in this document required an API write, and the Sandbox was not touched.

## The seven answers, in one table

| Question | Answer | Confidence |
| --- | --- | --- |
| 1. Must the declared URL be a homepage? | No stated requirement either way. Stripe describes the field only as "the business's publicly available website" and never mentions the apex, the root, or a homepage. | Checked absence, with the adjacent social-profile rule pointing the other way |
| 2. Where is `business_profile.url` shown? | Stripe states that customers see the business name **and website URL** on card statements or in Stripe-sent email receipts. It is a public field, unlike `product_description`. | Read first-party |
| 3. Must the content be reachable from the homepage? | No. Stripe anchors every requirement to the **declared URL**, and the one place it discusses linking says the linked content only has to be "easily accessible from" that declared destination. | Read first-party, plus a checked absence on any homepage/navigation wording |
| 4. Does `noindex` conflict with anything Stripe requires? | No. Stripe's structural requirements are: loads, reachable, not password protected, not geo-blocked. Search-engine indexability appears nowhere, including in the enumerated failure codes. | Checked absence over the full `invalid_url_*` error enum |
| 5. How is the review performed? | Both. Something fetches the URL remotely and can fail it on specific missing policies; a human contacts the account holder for ownership verification and for "recommendations." Stripe says it "regularly checks" the site, so review is ongoing, not one-shot. | Read first-party for the fact; **inference** for the automated/human split |
| 6. Any guidance for a business not yet selling? | Yes, and it is the FAQ's own structure: two content tiers, the second one required "by the time you start selling." An under-construction site is a named, enumerated failure reason. | Read first-party |
| 7. Is an invite-only pilot offer a "promotion"? | Stripe never defines the term. Its own gloss is "any promotion, discount, or trial," and its instruction is to make the terms visible **at the point the customer agrees to participate**. | Read first-party for the gloss; the pilot-specific reading is **inference** |

## The source text everything else is measured against

Two pages carry almost all of Stripe's stated website policy, and both are short enough to quote in full where it matters.

**[Business website for account activation FAQ](https://support.stripe.com/questions/business-website-for-account-activation-faq)** — the activation-side rule:

> If you use a website, social media profile or mobile app to promote your business or sell products, Stripe needs to review it to understand your business and verify what you are selling in order to comply with financial regulations.
>
> You are required to maintain a functioning website with the appropriate information at all times. Stripe regularly checks that the provided website is compliant and if it detects an issue, you will need to update it or provide a new website that satisfies the requirements.

Its content list comes in two tiers. Tier one — "Your business website, social media profile, or mobile application must include" — is your business name and descriptions of the goods or services offered, and "These must align with the information you've provided to Stripe." Tier two is introduced with a deadline rather than an immediate obligation:

> Stripe and our financial partners require the following additional information on your website, social media profile, or mobile application **by the time you start selling** your goods or services. Add it as soon as possible to avoid business disruptions. These requirements can come into effect more quickly if you operate in an industry with elevated financial risk.

and lists customer service contact details, return policy and process (physical goods), refund and dispute policy, cancelation policy (if applicable), legal or export restrictions (if applicable), and terms and conditions of any promotions.

Under "Common reasons that the business website or social media profile you provided was not sufficient", the FAQ names four: the webpage "must be accessible without a password"; "Under construction or incomplete", where "Your account may not be able to be activated until your website contains the necessary information about your business. Once your website is complete with all products you are selling listed, update the URL in your Dashboard"; an incomplete social media profile; and region blocking, where the fix is "Temporarily remove the blocker until your website has been verified."

**[Website checklist](https://docs.stripe.com/get-started/checklist/website)** — the docs-side companion, framed as card-network compliance rather than activation gating. It adds purchase currency, privacy policy, business address, PCI/security statements, and card logos to the FAQ's list, and it is the page that says what Stripe does when a site falls short: "If we review your website and find that it isn't clear what you're selling, we may contact you with recommendations for improving the description," and "If your website doesn't include sufficient information about your fulfillment policies, we might request that you add it."

**The `requirements.errors[].code` enum on the Account object** ([API reference](https://docs.stripe.com/api/accounts/object#account_object-requirements-errors-code)) is the machine-readable form of the same policy, and it is the most precise statement Stripe publishes of what the website review can fail on:

| Code | Stripe's description |
| --- | --- |
| `invalid_url_denylisted` | "Generic business URLs aren't supported." |
| `invalid_url_format` | "URL must be formatted as https://example.com." |
| `invalid_url_web_presence_detected` | "Because you use a website, app, social media page, or online profile to sell products or services, you must provide a URL for your business." |
| `invalid_url_website_business_information_mismatch` | "The business information on your website must match the details you provided to Stripe." |
| `invalid_url_website_empty` | "Your provided website appears to be empty." |
| `invalid_url_website_inaccessible` | "This URL couldn't be reached. Make sure it's available and entered correctly or provide another." |
| `invalid_url_website_inaccessible_geoblocked` | "Your provided website appears to be geographically blocked." |
| `invalid_url_website_inaccessible_password_protected` | "Your provided website appears to be password protected." |
| `invalid_url_website_incomplete` | "Your website seems to be missing some required information." |
| `invalid_url_website_incomplete_cancellation_policy` | "…appears to have an incomplete cancellation policy." |
| `invalid_url_website_incomplete_customer_service_details` | "…appears to have incomplete customer service details." |
| `invalid_url_website_incomplete_legal_restrictions` | "…appears to have incomplete legal restrictions." |
| `invalid_url_website_incomplete_refund_policy` | "…appears to have an incomplete refund policy." |
| `invalid_url_website_incomplete_return_policy` | "…appears to have an incomplete refund policy." (Stripe's text; presumably a copy error for *return*) |
| `invalid_url_website_incomplete_terms_and_conditions` | "…appears to have incomplete terms and conditions." |
| `invalid_url_website_incomplete_under_construction` | "Your provided website appears to be incomplete or under construction." |
| `invalid_url_website_other` | "We weren't able to verify your business using the URL you provided." |

That is the whole family. There is no code for an unindexed page, an unlinked page, a non-root URL, or a page missing from a sitemap.

## 1. The declared URL does not have to be a homepage

**What Stripe says the field is.** The API reference defines it in six words: `business_profile.url` is "The business's publicly available website" ([Account object](https://docs.stripe.com/api/accounts/object#account_object-business_profile-url)). The Dashboard equivalent is a text box labelled **Business Website** under Settings → Business → Business Details ([How to update your business website URL in the Dashboard](https://support.stripe.com/questions/how-to-update-your-business-website-url-in-the-dashboard)). The `invalid_url_format` error states the only format rule Stripe publishes: "URL must be formatted as https://example.com." That is a scheme-and-host example inside an error message, not a constraint on path.

**Checked absence: Stripe never distinguishes apex from deep page for a website.** The words *homepage*, *home page*, *root*, *apex*, and *landing page* appear nowhere in the Business website FAQ, the Website checklist, [Do I have to have a business website to sign up for Stripe?](https://support.stripe.com/questions/do-i-have-to-have-a-business-website-to-sign-up-for-stripe), [Set up your account](https://docs.stripe.com/get-started/account/activate), [Website ownership verification during Stripe account application](https://support.stripe.com/questions/website-ownership-verification-during-stripe-account-application), the Account object reference including the full `requirements.errors` enum, [Identity verification](https://docs.stripe.com/connect/identity-verification), [Risk management for platforms](https://docs.stripe.com/connect/risk-management), or [Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts). The last two contain the string "website" zero times.

**Two adjacent facts point toward a deep page rather than away from it**, and neither is a statement about websites, so neither is treated here as the answer.

- For the social-profile substitute, a deep URL is *mandatory* and the root is explicitly rejected: "You must give a full URL to your specific profile for example facebook.com/profile/yourbusiness. Just your social media handle or just facebook.com for example is insufficient" (Business website FAQ). Stripe's concern there is that the URL resolves to the page carrying the information, not to the top of the property.
- The under-construction remedy is phrased as re-pointing the field, not as fixing the front door: "Once your website is complete with all products you are selling listed, **update the URL in your Dashboard**." The field is treated as something you aim at whatever page now satisfies the requirement.

**Conclusion.** A deep URL such as `https://example.com/what-we-do` satisfies everything Stripe states about the field. Nothing in this sweep requires the apex, and nothing forbids it either. The one live risk is `invalid_url_denylisted` — "Generic business URLs aren't supported" — which Stripe does not define; on its plain reading it targets placeholder or shared-host URLs rather than a first-party path on the business's own domain, and that reading is an **inference**.

## 2. `business_profile.url` is a public, customer-facing field

**Read first-party.** [Set up your account](https://docs.stripe.com/get-started/account/activate), under the heading "Public business information", is unambiguous:

> Your customers see the following details on either their card statements or in [email receipts](https://docs.stripe.com/receipts) sent by Stripe.
>
> - Business name and website URL
> - Support email address, phone number, and address
> - Support site URL
> - Statement descriptor text

So the declared website URL is not an internal underwriting note. It is grouped with the support fields and the statement descriptor as something a paying customer reads.

The API reference draws the public/internal line explicitly, field by field:

| Field | Stripe's description | Audience |
| --- | --- | --- |
| `business_profile.url` | "The business's publicly available website." | Public |
| `business_profile.name` | "The customer-facing business name." | Public |
| `business_profile.support_url` | "A publicly available website for handling support issues." | Public |
| `business_profile.support_email` | "A publicly available email address for sending support issues to." | Public |
| `business_profile.support_phone` | "A publicly available phone number to call with support issues." | Public |
| `business_profile.support_address` | "A publicly available mailing address for sending support issues to." | Public |
| `business_profile.product_description` | "**Internal-only** description of the product sold or service provided by the business. It's used by Stripe for risk and underwriting purposes." | Stripe only |

`product_description` is the only one of the seven Stripe marks internal. Every other field in the group, `url` included, Stripe describes as publicly available.

**The statement descriptor is a separate field with separate rules.** It lives at `settings.payments.statement_descriptor` (and `settings.card_payments.statement_descriptor_prefix`), is 5–22 Latin characters, must "reflect your Doing Business As (DBA) name", and must contain "more than a single common term or common website URL" — a bare URL is allowed only "if it provides a clear and accurate description of a transaction on a customer's statement" ([Statement descriptors](https://docs.stripe.com/get-started/account/statement-descriptors)). It is not derived from `business_profile.url`.

**A tension worth recording.** [Receipts and paid invoices](https://docs.stripe.com/receipts) has its own "Support requirements" section — "For compliance reasons, some contact information is always required on receipts: Legal business name, Customer support address, Customer support email, Privacy policy URL" — and the business website URL is **not** on that list. The two pages disagree about whether the website URL is on every receipt or merely among the details a customer may see. The activation page is the one that names it as customer-visible; the receipts page names what is mandatory. Both are quoted here rather than reconciled.

**Checked absence: Checkout and hosted onboarding.** No page in this sweep states that `business_profile.url` is rendered on a Stripe Checkout page or inside Stripe-hosted Connect onboarding. [Stripe-hosted onboarding](https://docs.stripe.com/connect/hosted-onboarding) says the form's appearance is customised from the platform's Connect settings — "your brand's name, color, and icon" — and mentions `business_profile.url` only as something the platform may *prefill on the connected account*: "If you onboard an account and your platform provides it with a URL, prefill the account's `business_profile.url`. If the business doesn't have a URL, you can prefill its `business_profile.product_description` instead." Pages read for this: the hosted-onboarding doc, the receipts doc, the activation doc, and the Account object reference.

**One further public surface, and it is opt-in and separate.** [Stripe profiles](https://docs.stripe.com/get-started/account/profile) is a distinct product where a business publishes "a description, business website, business address, and business email" to Dashboard search, the Stripe Directory, Stripe-generated invoices, and `profile.stripe.com/@handle`. It is created deliberately, and it carries a **Private Stripe profile** toggle. It is not `business_profile.url` and it is not created by activating an account.

## 3. The requirement attaches to the declared URL, not to a path from the homepage

This is the crux, and Stripe's own grammar settles most of it.

**Read first-party.** Every requirement in the FAQ is written against the thing you declared. "Your business **website, social media profile, or mobile application** must include: Your business name, Descriptions of the goods or services offered." "**The webpage** must be accessible without a password." "Your provided **website** appears to be empty." Nowhere is the object of the sentence a site, a domain, or a homepage; it is the property whose URL you gave Stripe.

**The single piece of navigation wording Stripe publishes says the opposite of a homepage rule.** From [Do I have to have a business website to sign up for Stripe?](https://support.stripe.com/questions/do-i-have-to-have-a-business-website-to-sign-up-for-stripe):

> If the social media profile can't show all the required information, due to character limits or other functionality, you can link to this information as long as it's easily accessible from the social media profile.

Stripe permits the required content to live at some *other* URL entirely, provided it is easily reachable **from the declared destination**. The anchor of reachability is the URL you gave Stripe. There is no third point in that sentence — no site root that also has to reach it.

**Checked absence: crawling, navigation, sitemaps, site structure.** The words *crawl*, *crawler*, *sitemap*, *robots*, *navigation*, *menu*, *link from your homepage*, and *site structure* appear in none of: the Business website FAQ, the Website checklist, the "Do I have to have a business website" article, the website ownership verification article, [Set up your account](https://docs.stripe.com/get-started/account/activate), [Receipts and paid invoices](https://docs.stripe.com/receipts), the Account object API reference including the whole `requirements.errors` enum, [Stripe-hosted onboarding](https://docs.stripe.com/connect/hosted-onboarding), [Identity verification](https://docs.stripe.com/connect/identity-verification), [Risk management for platforms](https://docs.stripe.com/connect/risk-management), [Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts), or the Stripe Services Agreement at `stripe.com/legal/ssa` (searched in full text for "website"). The `requirements.errors` enum has seventeen `invalid_url_*` codes and not one of them concerns reachability by link.

**Conclusion.** An unlinked page whose URL is declared to Stripe satisfies everything Stripe states. The declared URL is the review's entry point, and Stripe's only linking rule runs outward from it, not inward to it.

Two caveats that are real and are not about navigation.

- `invalid_url_website_business_information_mismatch` — "The business information on your website must match the details you provided to Stripe" — and the FAQ's "These must align with the information you've provided to Stripe." If the apex teaser and the declared page describe the business differently, that is a mismatch risk on content, independent of linking. Keeping the two consistent is cheap.
- Website ownership verification, if triggered, asks for proof of control over "the website (domain/url)". One of its three paths is "an email address that matches your website domain", another is "add your email address to your website and then send us a direct URL where we can view this edit" ([Website ownership verification](https://support.stripe.com/questions/website-ownership-verification-during-stripe-account-application)). Note that Stripe's own remedy there is again a **direct URL** to a specific page. Ownership is checked at domain level, so the domain has to be genuinely yours; the *content* requirement stays attached to the declared page.

## 4. `noindex` conflicts with nothing Stripe requires

**Stripe's structural requirements, in full.** The page must load and be reachable (`invalid_url_website_inaccessible`), must not be password protected ("The webpage must be accessible without a password"; `invalid_url_website_inaccessible_password_protected`), must not be geo-blocked ("Temporarily remove the blocker until your website has been verified"; `invalid_url_website_inaccessible_geoblocked`), must not be empty (`invalid_url_website_empty`), must be a well-formed https URL (`invalid_url_format`), and must not be a generic URL (`invalid_url_denylisted`). That is the complete structural list Stripe publishes.

**Checked absence: search-engine indexability is not among Stripe's requirements anywhere.** The terms *index*, *indexed*, *indexable*, *noindex*, *robots*, *robots.txt*, *X-Robots-Tag*, *SEO*, *search engine*, *Google*, and *discoverable* appear in none of the pages listed in section 3, and specifically not in the Business website FAQ, not in the Website checklist, and not in the seventeen-member `invalid_url_*` error family. Stripe enumerates its failure modes in that enum to a fine grain — down to which individual policy is missing — and an indexing failure mode is not in it.

**Connect side, checked separately.** The Connect-side obligation is the same standard applied to connected accounts. [Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts) exposes it as a Dashboard filter — "use the **Requirements** filter and select the appropriate requirements, such as **Additional business information** or **Business website information**" — and the page says nothing about indexing, because it says nothing about website content at all beyond that filter name. [Risk management for platforms](https://docs.stripe.com/connect/risk-management) and [Identity verification](https://docs.stripe.com/connect/identity-verification) were also read in full and contain the string "website" zero times each.

**Conclusion.** A page serving `X-Robots-Tag: noindex` or a `<meta name="robots" content="noindex">` still loads, still returns 200 to any client, and is still password-free and un-geo-blocked. It meets every structural requirement Stripe states. Keeping the pilot out of Google is orthogonal to Stripe's review.

**One thing `noindex` must not become.** A `robots.txt` `Disallow` is a different mechanism from a `noindex` tag and is equally irrelevant to Stripe, but any measure that turns into an actual *fetch* barrier is not. Anything that returns a 401/403, sits behind a login, requires a shared password, requires an invite token in the path to render content, or serves different content by IP or geography lands squarely on `invalid_url_website_inaccessible`, `..._password_protected`, or `..._geoblocked`. The line Stripe draws is: anyone with the URL can load it. Unlisted is fine; gated is not.

## 5. The review is continuous, and it is both machine and human

**It is ongoing, and the obligation is on the account holder — including a platform's own account.** The Business website FAQ states it as a standing duty, not an activation hurdle:

> You are required to maintain a functioning website with the appropriate information at all times. Stripe regularly checks that the provided website is compliant and if it detects an issue, you will need to update it or provide a new website that satisfies the requirements.

That sentence is addressed to whoever holds the Stripe account. A platform holds a Stripe account, so the platform's own declared website is subject to the same recurring check as any connected account's. Nothing in the FAQ carves platforms out, and the Connect-side material does not replace this obligation — it adds the platform's duty to act on the *same* requirement when it lands on a connected account, surfaced as the **Business website information** requirement filter in the Connect Dashboard ([Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts)).

**What is checked.** The `requirements.errors` enum is the answer, and it is granular: not merely "site is bad" but which specific policy is absent — cancellation policy, customer service details, legal restrictions, refund policy, return policy, terms and conditions — plus mismatch against the details given to Stripe, plus emptiness, inaccessibility, password protection, geo-blocking, and under-construction state. Whatever performs the check is able to distinguish a missing cancellation policy from a missing refund policy on a live page.

**Speed, first-party.** [New Stripe account approval process](https://support.stripe.com/questions/new-stripe-account-approval-process): "In most cases, the account approval process is nearly instantaneous and most users will be able to accept payments right away. If we need more information about a business or expect a longer delay in processing an account approval, we will reach out to you immediately." Restricted-list businesses "commonly experience longer processing delays."

**Documented human touchpoints.** Three, all first-party:

1. The Website checklist's own language — "If we review your website and find that it isn't clear what you're selling, we may contact you with recommendations for improving the description", and "If your website doesn't include sufficient information about your fulfillment policies, we might request that you add it. If you aren't sure what information you need to add, or you want us to review updates that you've made in response to our request, contact Stripe Support."
2. Website ownership verification, which is a form-and-email exchange whose free-text branch — "If you can't make edits to your website, you'll need to enter an explanation into the form" — has to be read by a person. Its consequence is stated plainly: "If you cannot verify website ownership, Stripe may pause your payouts until the issue is resolved."
3. The approval-process article's "we will reach out to you immediately."

**Inference, labelled.** Stripe never says "a crawler fetches your URL." Three things make an automated remote fetch the only coherent reading: near-instant approval in most cases; an error taxonomy fine-grained enough to be produced by content analysis; and the geo-blocking instruction, which only makes sense if the fetch originates from wherever Stripe's infrastructure sits rather than from a reviewer sitting in the merchant's own market. **Checked absence** on the mechanism itself: neither the Business website FAQ, the Website checklist, the ownership-verification article, the approval-process article, the Account object reference, nor the Connect review page states whether the check is automated, human, or both, and none of them names a re-check cadence or a list of triggers beyond "regularly" and "if it detects an issue."

**Practical consequence.** Passing activation is not the end state. If the declared URL later stops carrying the required content — the pilot page is taken down, or the domain is reworked for the January 2027 launch and the path changes — the account can pick up a `business_profile.url` requirement after the fact, and on the ownership-verification path the stated consequence is paused payouts. The declared URL is a live dependency for as long as the account exists.

## 6. Stripe does address a business that is not yet selling

**This is a positive finding, not an absence.** The FAQ's two-tier structure *is* Stripe's accommodation for a business before it starts selling. Tier one — business name, and descriptions of the goods or services — is required to activate. Tier two — customer service contact details, return, refund and dispute, cancellation, legal restrictions, promotion terms — is required "by the time you start selling your goods or services", with "Add it as soon as possible to avoid business disruptions" and an explicit acceleration clause: "These requirements can come into effect more quickly if you operate in an industry with elevated financial risk."

So a business that has not started selling can, on Stripe's own wording, activate on tier one alone. That is what the deadline phrasing means and it is the only reading the sentence supports.

**What is not permitted is a page that reads as unfinished.** Stripe names it twice. As a common failure reason: "Under construction or incomplete — Your account may not be able to be activated until your website contains the necessary information about your business." And as an enumerated error: `invalid_url_website_incomplete_under_construction`, "Your provided website appears to be incomplete or under construction." A bare "we ship in January 2027, leave your email" page is precisely the shape that code describes.

The resolution the FAQ gives is not "wait until you launch." It is: "Once your website is complete with all products you are selling listed, **update the URL in your Dashboard**." Stripe's own remedy for an under-construction site is to point the field at a page that carries the information.

**Inference, labelled.** A page whose content is complete — business name, a real description of the service, and the tier-two policies — does not become "under construction" because it also says the product opens to general availability in January 2027. The failure code describes a page missing information, not a page describing a future launch date. That reading is consistent with everything quoted above but Stripe does not state it.

**Checked absence: no dedicated pre-launch guidance exists.** The terms *pre-launch*, *prelaunch*, *coming soon*, *beta*, *invite-only*, *private pilot*, *waitlist*, and *early access* were searched for across the Business website FAQ, the Website checklist, [Set up your account](https://docs.stripe.com/get-started/account/activate), [Do I have to have a business website](https://support.stripe.com/questions/do-i-have-to-have-a-business-website-to-sign-up-for-stripe), [New Stripe account approval process](https://support.stripe.com/questions/new-stripe-account-approval-process), and [Accepting payments for pre-orders](https://support.stripe.com/questions/accepting-payments-for-pre-orders) — the last of which turns out to be entirely about SetupIntents and manual capture, with nothing to say about website content. Stripe publishes no article aimed at a business activating before it launches. The tiered FAQ is all there is, and it is enough.

## 7. "Promotion" is undefined, and the rule is about disclosure at the point of agreement

**What Stripe actually says.** The FAQ lists "Terms and conditions of any promotions" among the before-selling items and defines nothing. The Website checklist is the only page that expands it:

> **The terms of any promotions you're offering** — Clearly disclose the conditions of any promotion, discount, or trial that you offer to customers. Display a link or disclaimer text so that it's visible when customers agree to participate. Transparency around these conditions can help avoid confusion and disputes.

Three things are load-bearing there. Stripe's gloss is "any promotion, discount, or trial" — a *trial* counts, which is broader than a discount code. The framing is card-network dispute prevention, and the checklist page as a whole is explicitly grounded in "the rules published by the card networks". And the required placement is not "somewhere on your homepage" but "visible **when customers agree to participate**" — at the point of acceptance.

**Checked absence: Stripe defines "promotion" nowhere in this context.** The Stripe Services Agreement at `stripe.com/legal/ssa` was searched in full text; "promotion" occurs there only in the trademark clause ("promotional activities to which the parties agree in writing") and in the definition of "Third-Party Service", neither of which is about merchant website content. [Promotion Codes](https://support.stripe.com/questions/promotion-codes) and the Billing coupons docs describe the `PromotionCode` API object, which is a discount mechanism inside Stripe, not the website-content term. The Business website FAQ, the Website checklist, and the Account object error enum's `invalid_url_website_incomplete_terms_and_conditions` all use the word without defining it. There is no first-party page that says which offers are in scope and which are not.

**Inference, labelled as such — this is the reading, not Stripe's.** An invite-only pilot offered by email, on terms different from the eventual general-availability terms, is an offer with conditions attached: who is eligible, what it costs during the pilot, how long the pilot pricing lasts, what happens at the end of it, and how a participant leaves. That is materially what Stripe's "promotion, discount, or trial" describes, and it is exactly the situation the card-network rationale is aimed at — a customer who agrees to something whose conditions were never written down is the customer who disputes the charge later. The narrow reading, that a promotion means a public discount code or a public sale, is not supported by the word "trial" sitting in Stripe's own list.

**What that implies, and it is cheap.** Stripe's placement rule points at the pilot offer itself, not at the public site: the terms must be visible where the pilot participant agrees. Putting the pilot's conditions in front of the invitee at the point they accept satisfies the rule as written. Publishing them on the declared URL as well costs nothing and removes the argument entirely. Neither requires the pilot to be advertised to the teaser's waitlist audience, because neither requires the terms to be *promoted* — only disclosed to the people the offer is made to.

## The pages that were read

Recorded so every checked absence above is auditable.

Stripe support centre: [Business website for account activation FAQ](https://support.stripe.com/questions/business-website-for-account-activation-faq) · [Do I have to have a business website to sign up for Stripe?](https://support.stripe.com/questions/do-i-have-to-have-a-business-website-to-sign-up-for-stripe) (also served at `information-required-on-your-business-website-to-use-stripe`) · [Website ownership verification during Stripe account application](https://support.stripe.com/questions/website-ownership-verification-during-stripe-account-application) (also served at `domain-verification-during-stripe-account-application`) · [How to update your business website URL in the Dashboard](https://support.stripe.com/questions/how-to-update-your-business-website-url-in-the-dashboard) · [New Stripe account approval process](https://support.stripe.com/questions/new-stripe-account-approval-process) · [Accepting payments for pre-orders](https://support.stripe.com/questions/accepting-payments-for-pre-orders)

Stripe docs: [Website checklist](https://docs.stripe.com/get-started/checklist/website) · [Set up your account](https://docs.stripe.com/get-started/account/activate) · [Statement descriptors](https://docs.stripe.com/get-started/account/statement-descriptors) · [Receipts and paid invoices](https://docs.stripe.com/receipts) · [Stripe profiles](https://docs.stripe.com/get-started/account/profile) · [Stripe-hosted onboarding](https://docs.stripe.com/connect/hosted-onboarding) · [Identity verification](https://docs.stripe.com/connect/identity-verification) · [Risk management for platforms](https://docs.stripe.com/connect/risk-management) · [Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts)

Stripe API reference: [The Account object](https://docs.stripe.com/api/accounts/object), read in the expanded forms `object.md?query=business_profile` and `object.md?query=requirements` so that every nested field description and the complete `requirements.errors[].code` enum were in view.

Stripe legal: [Stripe Services Agreement](https://stripe.com/legal/ssa), full text searched for "website" and "promotion".

## What follows for the plan

Stated plainly, because these were established rather than guessed.

1. **Declare the deep page.** Put the required content on a dedicated page and set `business_profile.url` — the Dashboard's **Business Website** box — to that page's URL. Nothing Stripe publishes prefers the apex, and Stripe's own remedy for a thin site is to re-point this field.
2. **The page does not have to be linked from the teaser.** The declared URL is the review's entry point. Stripe's only linking rule runs outward from the declared URL, and Stripe never asks for a path inward to it.
3. **`noindex` is safe. A gate is not.** Serve the page 200 to anyone with the URL, no password, no geo-block, no invite token needed to render it. Suppressing search indexing touches none of Stripe's stated requirements.
4. **Put tier-one content on it now and tier-two content on it before the first charge.** Business name and a real description of the service are the activation gate. Customer service contact, refund and dispute, and cancellation policies are due by the time selling starts — and since the pilot is the first selling, "before the pilot's first charge" is the honest deadline, not "before general availability."
5. **Do not let it read as under construction.** `invalid_url_website_incomplete_under_construction` is a named failure code. A future launch date stated on a page that is otherwise complete is a different thing from a placeholder, but the page has to carry actual content for that distinction to hold.
6. **Keep the teaser and the declared page consistent.** `invalid_url_website_business_information_mismatch` and the FAQ's "must align with the information you've provided to Stripe" are about content agreement, and they apply across whatever a reviewer can see on the domain as well as against the Dashboard details.
7. **Write the pilot's terms down and show them at acceptance.** Treat the invite-only pilot as a promotion for disclosure purposes. This is the one recommendation resting on an inference rather than a Stripe statement, and the cost of following it anyway is one page of text.
8. **Treat the declared URL as permanent infrastructure.** Stripe "regularly checks" it, and payouts are the stated lever when it cannot verify. If the January 2027 site restructures the domain, the declared URL has to keep resolving to compliant content or be updated in the Dashboard at the same time.
