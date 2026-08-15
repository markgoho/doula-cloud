# E-signature vendors — HIPAA BAA availability and embeddable API support

Research for wayfinder ticket #44 ("Contract generation / e-signature
approach", part of wayfinder map #29). Contract is a settled domain entity
(issue #34): the service agreement between a doula practice and a client,
distinct from Invoice (Stripe billing, issue #38) and Care Plan / Birth Plan
(clinical documents, issue #43). Contracts likely carry no PHI but may carry
health-adjacent language (informed consent clauses, birth-plan references).
Standing repo policy: never send PHI or clinical/care-plan content to a
vendor without a signed BAA — the same policy already applied to Stripe and
messaging notifications.

Two questions per vendor: (1) does it offer a HIPAA BAA, at what plan tier,
and roughly what does that tier cost for a solo/small practice; (2) does it
offer a developer/embeddable API for an in-app signing flow (SvelteKit
frontend + Go backend), as opposed to only a hosted-portal/email-redirect
flow, and is API access priced separately from the base plan.

## Summary table

| Vendor | BAA — tier / cost | Embeddable API — tier / cost |
|---|---|---|
| DocuSign | Enhanced plan only; custom quote, 5-user minimum, contact sales | Embedded signing starts at Advanced tier ($480/mo) but Advanced has **no** BAA; the cheapest tier with both is Enhanced (custom) |
| Dropbox Sign | Annual Standard ($25/user/mo monthly list, ~$17.50/user/mo annual) or Premium (custom), plus an unspecified minimum contract value, plus a signed BAA | Separate API product: Essentials $75/mo or Standard $250/mo, both include embedded signing; unclear whether the HIPAA/BAA terms (tied to the "Standard or Premium" UI plans) extend to the API-only plans |
| PandaDoc | Annual Business ($49/user/mo) or Enterprise; one source specifies BAA is issued for Enterprise customers with 5+ seats | API Developer plan ~$40/mo (40 docs, then $4/doc) includes only "limited" embedded editor & signing; full embedded signing requires Enterprise |
| SignWell | **Any paid plan, on request** — Light $10/mo, Business $30/mo, or Enterprise (custom) | Standard API plan $275/mo (first 25 docs/mo free, then $0.85 down to $0.20+/doc by volume) includes full embedded signing, requesting, and templates |
| Adobe Acrobat Sign | Enterprise/Business subscription required before Adobe will discuss a BAA; no public pricing | API access described at enterprise tier; no public pricing, usage-based |
| SignNow | "Corporate" plans only; no public pricing, contact sales | Embedded signing included on **all** API plans; $2/invite at 500 invites down to $1.20/invite at 5,000; unclear if BAA extends to the API tier vs. only the UI "corporate" plan |

**Standout:** SignWell is the clear low-cost option combining both asks. Its
BAA is available on any paid plan starting at $10/month (Light), not gated
behind an annual/enterprise commitment like DocuSign, Dropbox Sign, or
PandaDoc. Its API plan ($275/mo, usage-based per document) includes full
embedded signing and lists HIPAA compliance directly on the API pricing
page. Adobe Acrobat Sign and SignNow do not stand out as low-cost — both
gate BAA behind an undisclosed-price enterprise/corporate tier.

## DocuSign

- BAA: DocuSign states it "will sign a Business Associate Addendum with
  customers who are required by law to comply with HIPAA," and publishes a
  Service Attachment / Business Associate Addendum for its Signature
  product.
  [docusign.com/company/terms-and-conditions/schedule-docusign-signature/attachment-business-associate-addendum](https://www.docusign.com/company/terms-and-conditions/schedule-docusign-signature/attachment-business-associate-addendum),
  [docusign.com/blog/electronic-signature-hipaa-forms](https://www.docusign.com/blog/electronic-signature-hipaa-forms)
  ("With the correct configuration, Docusign eSignature can be HIPAA
  compliant… A signed BAA must be completed by the vendor prior to
  providing services"). Neither page states which commercial plan tier is
  required.
- Developer API pricing (fetched directly): Starter $50/mo (40
  envelopes/mo, no embedded signing); Intermediate $300/mo (100
  envelopes/mo, no embedded signing); Advanced $480/mo (100 envelopes/mo,
  **embedded signing included**); Enhanced — custom quote, 5-user minimum.
  The Enhanced tier is explicitly where "HIPAA support through BAA" lives:
  "HIPAA support through BAA is available exclusively through Enhanced
  plans and requires contacting sales."
  [ecom.docusign.com/plans-and-pricing/developer](https://ecom.docusign.com/plans-and-pricing/developer)
- Net: the cheapest DocuSign path to an in-app embedded signing flow with a
  BAA is the custom-quoted, 5-seat-minimum Enhanced plan — likely the most
  expensive option researched here for a solo/small practice.

## Dropbox Sign (formerly HelloSign)

- BAA (standard product, fetched directly): "Dropbox Sign supports HIPAA
  compliance for customers who are on an annual Standard or Premium plan,
  have a signed Business Associate Agreement (BAA), and meet the minimum
  contract value." A minimum contract value applies but its amount is not
  disclosed on this page.
  [help.dropbox.com/security/dropbox-sign-hipaa-compliance](https://help.dropbox.com/security/dropbox-sign-hipaa-compliance)
- Standard-product pricing (fetched directly): Essentials $15/mo (no HIPAA
  mentioned), Standard $25/user/mo monthly or ~$17.50/user/mo annual
  (2-user minimum), Premium custom.
  [sign.dropbox.com/products/dropbox-sign/pricing](https://sign.dropbox.com/products/dropbox-sign/pricing)
- API product pricing (fetched directly, separate page from the UI
  product): Essentials $75/mo (50 signature requests/mo) and Standard
  $250/mo (100 signature requests/mo) both include embedded signing;
  Premium is custom; a free test/dev mode exists.
  [sign.dropbox.com/products/dropbox-sign-api/pricing](https://sign.dropbox.com/products/dropbox-sign-api/pricing)
- Gap: the HIPAA/BAA page only names the "Standard or Premium" **plan**
  without distinguishing the UI product from the API product — it is not
  confirmed here whether the BAA can be attached to an API-only
  subscription or requires the separate UI-product Standard/Premium plan
  as well.

## PandaDoc

- BAA: "PandaDoc will sign a Business Associate Agreement for Annual
  Business or Enterprise plan customers," with one source narrowing this to
  "all Enterprise customers with five or more seats."
  [pandadoc.com/blog/pandadoc-for-hipaa-compliance-updates](https://www.pandadoc.com/blog/pandadoc-for-hipaa-compliance-updates/),
  [pandadoc.com/hipaa](https://www.pandadoc.com/hipaa/)
- Base pricing: Free $0, Launch $9/user/mo, Starter $19/user/mo (annual),
  Business $49/user/mo (annual), Enterprise custom.
  [pandadoc.com/pricing](https://www.pandadoc.com/pricing/)
- API/developer pricing: an API Developer plan around $40/mo for 40
  documents sent (then $4/additional document); this tier includes only
  "limited embedded editor & signing" and "limited audit trail" — full
  embedded editor, full audit trail, custom branding, and approval
  workflows require Enterprise.
  [pandadoc.com/api](https://www.pandadoc.com/api/)
- Net: full embedded signing and BAA eligibility both land on the same
  Enterprise tier, custom-priced.
- **Sourcing caveat:** pandadoc.com consistently returned HTTP 429 to
  direct fetch attempts (pricing, /api/, /hipaa/, /developer-api/
  embedded-signing/ — all attempted, all blocked). The figures above come
  from search-engine result summaries that quote and cite these specific
  pandadoc.com pages, not from a direct page fetch.

## SignWell

- BAA (fetched directly): "SOC 2 Type II report and HIPAA BAA available on
  paid plans" — no annual commitment or enterprise-only gate stated;
  request it by emailing compliance@signwell.com per SignWell's own
  materials.
  [signwell.com/pricing](https://www.signwell.com/pricing/)
- Base pricing (fetched directly): Free $0 (3 docs/mo), Light $10/mo (1
  sender), Business $30/mo (3 senders, unlimited docs/templates),
  Enterprise custom.
  [signwell.com/pricing](https://www.signwell.com/pricing/)
- API pricing (fetched directly): free developer account (3 docs/mo, plus
  unlimited API test usage); Standard API plan $275/mo base, with the
  first 25 documents/mo free, then $0.85/document descending toward
  $0.20+/document at volume; a single "API document" can bundle multiple
  files/signers/steps for one charge. Embedded signing, embedded
  requesting, and embedded template editing are all included. The API
  pricing page itself states "SOC 2 Type II & HIPAA compliance."
  [signwell.com/api-pricing](https://www.signwell.com/api-pricing/)
- This is the standout low-cost combination identified in this research:
  BAA reachable at a $10–30/mo tier, and a usage-based embeddable API
  starting around $275/mo with no enterprise minimum-seat requirement.

## Adobe Acrobat Sign

- BAA: Adobe's own HIPAA configuration page exists at
  [helpx.adobe.com/sign/config/compliance-issues/hipaa/overview.html](https://helpx.adobe.com/sign/config/compliance-issues/hipaa/overview.html)
  but repeated direct fetches of it timed out; findings below are from
  search-engine summaries of that same page and related Adobe pages. "The
  HIPAA readiness capability is only available through an Adobe Acrobat
  Sign for enterprise or business subscription plan" — a health care
  organization must have an Enterprise Plan account before Adobe will
  discuss signing a BAA. No public pricing for this tier.
- API: described as available at the enterprise tier with HIPAA/FERPA/GLBA
  compliance features and API access for embedded signing; Adobe does not
  publicly list API pricing — usage/envelope-based, quote required.
- Net: no standout — BAA and API access both require an undisclosed-price
  enterprise contract, similar to DocuSign's Enhanced tier.

## SignNow (airSlate)

- BAA (fetched directly): "HIPAA features are only available on SignNow's
  corporate plans." Process: upgrade to a corporate plan, then contact
  support/sales to request HIPAA activation and start the BAA process. No
  pricing is disclosed on this page; it defers to SignNow's pricing page.
  [signnow.com/help/hipaa-compliance-guide](https://www.signnow.com/help/hipaa-compliance-guide)
- Pricing page: `signnow.com/pricing` redirects to a JS-rendered purchase
  page (`snseats.signnow.com/purchase/business_plans/pricing`) that did not
  yield extractable tier/price data via fetch; not confirmed from a primary
  source in this research.
- API (fetched directly): embedded signing is included on **all** API
  plans, unlike DocuSign/PandaDoc which gate it behind higher tiers.
  Pricing is per-invite: $2/invite at a 500-invite volume, down to
  $1.20/invite at 5,000 invites.
  [signnow.com/developers](https://www.signnow.com/developers)
- Net: SignNow's API is cheap and embeddable by default, but it is not
  confirmed here whether a BAA can attach to the API-plan track at all, or
  only to the separate "corporate" UI-plan track — this is the same
  UI-plan-vs-API-plan ambiguity found with Dropbox Sign, and pricing for
  the corporate/BAA-eligible tier was not found publicly.

## Note on sourcing

Direct WebFetch succeeded for: ecom.docusign.com (developer pricing),
docusign.com/blog (HIPAA statement), docusign.com (BAA addendum, found via
search but not independently re-fetched), help.dropbox.com (HIPAA page),
sign.dropbox.com (both pricing pages), signwell.com (both pricing pages),
signnow.com/help (HIPAA guide), signnow.com/developers (API page).

pandadoc.com blocked all direct fetch attempts with HTTP 429 across
multiple pages and retries; those figures rely on search-engine summaries
that quote the specific pandadoc.com page, flagged inline above.
helpx.adobe.com timed out on repeated fetch attempts; those figures also
rely on search-engine summaries. signnow.com's pricing page is
JS-rendered and did not yield data through fetch (only the HIPAA guide and
developer pages, which are static, were fetched successfully).
