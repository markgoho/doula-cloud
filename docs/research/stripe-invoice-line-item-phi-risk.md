# Stripe invoice line-item text — privacy exposure from a health-implying description

Research for GitHub issue #38 ("Stripe integration shape"). Narrower follow-on
to the settled finding from ticket #30/issue #29 (no clinical/care-plan
content in any Stripe field — that is not re-litigated here). Question: does
pairing a Client's name/email with a generic-but-health-implying line-item
description (e.g. "Doula services") create a meaningful privacy or
PHI-adjacent risk, even with zero clinical detail?

## Recommendation

Use **"Professional services"** (or "Consulting services") as the Stripe
invoice line-item description and statement descriptor, not "Doula services"
or "Full-spectrum doula support." No primary source says "doula services"
is contractually or legally prohibited — Stripe's Services Agreement bars
Protected Health Information itself (defined by reference to 45 CFR
160.103), not a service-category label, and a service category alone is not
PHI unless HIPAA's covered-entity/business-associate machinery applies,
which it likely does not here (see Q2). But the underlying concern — a named
person's payment record revealing pregnancy/postpartum status to anyone who
sees the Stripe dashboard, a card statement, or an email inbox (Stripe,
Doula Cloud staff with dashboard access, the client's bank, anyone who opens
the client's email) — is a real exposure surface that exists independent of
HIPAA's technical scope, and it costs nothing to close it with a generic
label. This is a recommendation based on risk-minimization judgment, not a
sourced legal requirement — treat it as inference, not fact.

On the name/email question: **do not omit or generalize the Client's name or
email.** Stripe requires a name and email to address and deliver an Invoicing
API invoice, and Doula Cloud's own billing (per `CONTEXT.md`, the domain
model's Invoice/Payment concepts) depends on this delivery working reliably.
The identity fields are unavoidable and are not the risky part on their own
— an invoice addressed to "Jane Doe" for "$1,800" reveals nothing by itself.
The risk is specifically the *pairing* of identity with a health-implying
description; removing the health-implying words closes that pairing risk
without needing to touch the name/email fields at all.

## Q1 — Stripe's own docs/policies

- Stripe's Data Processing Agreement defines "Sensitive Data" to include
  "data concerning health" (alongside genetic/biometric data, sex life,
  race/ethnicity, religion, geolocation, and CCPA sensitive personal
  information), and states Stripe may process Sensitive Data only where
  applicable to specific product use cases (e.g. facial recognition for
  Identity products). It does **not** contain field-specific language about
  Checkout/Invoicing line-item descriptions or metadata.
  [stripe.com/legal/dpa](https://stripe.com/legal/dpa)
- Stripe's Services Agreement, Section 4.5, is the operative clause: "User
  must not provide Protected Health Information to Stripe as part of Third
  Party Data. User is liable for any disclosure of Protected Health
  Information to Stripe when User provides access to the Third Party Data."
  "Protected Health Information" is defined in the agreement's Definitions
  section by reference to 45 CFR §160.103 (the same regulatory definition
  HHS uses). [stripe.com/legal/ssa](https://stripe.com/legal/ssa)
- Nothing found in Stripe's docs or legal pages specifically addresses the
  narrower case in this ticket: a service-category label that *implies* a
  health condition without containing diagnosis/treatment/clinical detail,
  tied to an identified person. Stripe's contractual bar is scoped to PHI as
  legally defined (see Q2 for what that requires), not to any text that
  merely reads as health-adjacent to a human reader. **This gap is notable
  by its absence** — Stripe gives no lower-bound guidance on descriptive
  text, so the "Professional services" recommendation above is this
  research's own risk-minimization judgment, not something Stripe directs.

## Q2 — HHS/HIPAA guidance

- HHS's Summary of the HIPAA Privacy Rule defines protected health
  information as "information, including demographic information, which
  relates to: the individual's past, present, or future physical or mental
  health or condition, the provision of health care to the individual, or
  the past, present, or future **payment for the provision of health care**
  to the individual, and that identifies the individual or for which there
  is a reasonable basis to believe [it] can be used to identify the
  individual." Common identifiers (name, address, birth date, etc.) become
  PHI when associated with that health/payment information.
  [hhs.gov/hipaa/for-professionals/privacy/laws-regulations](https://www.hhs.gov/hipaa/for-professionals/privacy/laws-regulations/index.html)
  — this directly answers the "is a payment fact itself PHI" question:
  **yes, in principle** — "payment for the provision of health care to the
  individual" is explicitly listed as PHI-qualifying content when paired
  with an identifier, independent of any diagnosis or clinical note.
- However, PHI status and HIPAA's obligations only attach when the
  information is held/transmitted by a HIPAA **covered entity** (health
  plan, health care clearinghouse, or health care provider who transmits
  health information electronically in connection with HIPAA standard
  transactions) or a **business associate** acting on a covered entity's
  behalf. [hhs.gov/hipaa/for-professionals/privacy/laws-regulations](https://www.hhs.gov/hipaa/for-professionals/privacy/laws-regulations/index.html)
- HHS's business-associate guidance carves out payment processors
  specifically: a financial institution that processes card/debit
  transactions, clears checks, or otherwise "directly facilitates or
  effects the transfer of funds for payment for health care" is providing
  "normal banking or other financial transaction services to its
  customers" and is **not** acting as a business associate for that
  activity (HIPAA Section 1179 exemption).
  [hhs.gov/hipaa/for-professionals/privacy/guidance/business-associates](https://www.hhs.gov/hipaa/for-professionals/privacy/guidance/business-associates/index.html)
  — this is directly relevant to Stripe's role: acting purely as a payment
  processor for invoices likely falls outside the business-associate
  definition regardless of what the line item says.
- **Inference (not sourced):** whether a doula *practice* itself is a HIPAA
  covered entity is a separate, harder question this research did not
  resolve with a direct HHS statement. HHS describes covered health care
  providers as those who electronically transmit health information "in
  connection with transactions for which the Secretary of HHS has adopted
  standards" (e.g. claims, eligibility, remittance) — the kind of standard
  transactions built around insurance billing.
  [hhs.gov/hipaa/for-professionals/privacy/laws-regulations](https://www.hhs.gov/hipaa/for-professionals/privacy/laws-regulations/index.html)
  Doula services are typically private-pay, not billed through HIPAA
  standard insurance transactions, which suggests (inference, not
  confirmed by a direct HHS ruling on doulas) that Doula Cloud's practice
  customers are likely **not** HIPAA covered entities in the first place —
  meaning HIPAA's PHI machinery may not contractually apply to this data at
  all, independent of the payment-processor exemption above. This
  double-exemption is why the recommendation above is framed as prudent
  risk-minimization rather than a compliance requirement: the legal case
  that "doula services" + a name is regulated PHI in this specific
  transaction chain is weak on the facts gathered here, even though the
  general PHI definition (payment + identifier) would capture it in a
  covered-entity context.

## Q3 — Comparable vertical SaaS platforms

Primary evidence here is thin, as expected.

- SimplePractice's own support article on processing online payments
  documents a **statement descriptor** field (what appears on the client's
  bank/card statement), limited to 5–22 characters, but gives no guidance on
  content — only the technical constraint and a caveat that "banks are free
  to truncate, format, or re-order this information when they show it to
  their cardholders."
  [support.simplepractice.com: Processing online payments](https://support.simplepractice.com/hc/en-us/articles/360022512232-Processing-online-payments)
- Hint (payment/booking software marketed to wellness practices) has a
  nearly identical support article on updating the Stripe statement
  descriptor, recommending only "a word or phrase that's short and
  recognizable, such as your practice's name or domain name" — i.e.
  practice branding, not a service description. No privacy rationale is
  given.
  [support.hint.com: Update Statement Descriptor in Stripe](https://support.hint.com/en/articles/2569336-update-statement-descriptor-in-stripe)
- TherapyNotes' public billing-settings help article was checked and
  contains no guidance on statement descriptor or line-item description
  text at all.
  [support.therapynotes.com: Manage Client Billing Settings](https://support.therapynotes.com/hc/en-us/articles/30661307435035-Manage-Client-Billing-Settings)
- **No vendor examined publishes an explicit statement of *why* they choose
  generic vs. specific billing text**, or any acknowledgment that a service
  name can itself be health-revealing. The closest pattern found (both
  SimplePractice and Hint) is that the *statement descriptor* field
  (bank/card statement) is conventionally set to the practice's brand name,
  not a service description — which is a data point in favor of a
  non-clinical, brand-only label, but not a sourced statement of privacy
  rationale. **This is inference, not a documented industry position.**

## Note on sourcing

hhs.gov and ecfr.gov actively block automated fetches (403 responses to
direct requests); HHS content above was retrieved via search-engine
summaries that quote and cite the specific hhs.gov page, not via direct
page fetch — flagged here for transparency. Stripe's legal pages
(stripe.com/legal/dpa, stripe.com/legal/ssa) were fetched directly.
Vendor help-center articles (SimplePractice, Hint, TherapyNotes) were
fetched directly from their own support domains, not secondary write-ups.
