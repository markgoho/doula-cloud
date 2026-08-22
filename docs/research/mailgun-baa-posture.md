# Mailgun and a HIPAA BAA — does Doula Cloud need one, and will Mailgun sign?

Research for wayfinder ticket
[#214](https://github.com/markgoho/doula-cloud/issues/214), part of map
[#213](https://github.com/markgoho/doula-cloud/issues/213) ("Transactional
email: the Notification capability"). It continues
[#30](https://github.com/markgoho/doula-cloud/issues/30), which found that
Doula Cloud becomes a **subcontractor business associate** the moment any
tenant practice bills insurance or Medicaid electronically, or contracts
with a covered entity.

The map's **no-content rule** (mirroring
[ADR-0002](../adr/0002-message-transport-push-triggered-fetch.md)) keeps
clinical content out of every Notification: no message bodies, no Client
names, no Engagement detail, nothing in the subject line. What the rule
cannot remove is the **association** — a recipient email address paired
with a named Practice in the `From` line, held in the vendor's delivery
logs.

The two voices are answered separately throughout, because the association
argument is different for each:

- **Platform voice** — Doula Cloud speaking as itself to its own customers.
  No Practice is named.
- **Practice voice** — Doula Cloud speaking as a named Practice to that
  Practice's Client.

**This is a technical and compliance-surface scan, not legal advice.** Same
caveat as #30. Real legal review is warranted here; see the closing
section.

## Summary

| | Platform voice | Practice voice |
| --- | --- | --- |
| Who receives it | Practice Owner / Staff — Doula Cloud's own customer | The Practice's pregnant Client |
| What the `From` line says | Doula Cloud | A named doula practice |
| Is the recipient an individual whose health care the mail relates to? | No | **Yes** |
| Does the no-content rule remove the exposure? | Yes — nothing about a Client is present | **No** — the address + named Practice pairing is the exposure |
| BAA implicated? | **No**, on the facts as scoped | **Yes, conditionally** — once the sending tenant is a covered entity or Doula Cloud is a BA for that tenant (#30's trigger) |
| Vendor conclusion | Mailgun usable with no BAA | Mailgun needs an executed BAA, or the sending identity must change |

**Mailgun's posture: it publishes a HIPAA Business Associate Addendum and
its own content tells customers to sign it.** So the answer to "will they
sign" is *yes, on their published terms*. What could **not** be confirmed
first-party is which plan is required, whether there is a cost, and whether
execution is self-serve or sales-gated — Mailgun's Help Center article on
Compliance & Security is behind a bot wall and nothing on the pricing,
legal, healthcare, or enterprise pages names a tier.

**The one hard constraint found first-party:** Mailgun's Terms of Service
§4.1 forbids sending PHI *at all* until a separate agreement is in place.
Publishing the addendum text is not the same as the addendum being in
force.

## 1. Does a BAA apply at all under the no-content rule?

### 1.1 The gate is the tenant, not the email

Nothing in this section applies unless #30's trigger has fired. HIPAA binds
covered entities and their business associates; an independent doula who
never transmits a HIPAA standard transaction electronically is not a
covered entity, and a vendor serving only such practices is not a business
associate. #30 established this and it is not re-litigated here. Everything
below is conditional on **at least one tenant Practice being a covered
entity, or Doula Cloud otherwise standing as a business associate for that
tenant**. Because the infrastructure is shared across tenants, #30's
conclusion was to treat the platform as HIPAA-capable once any tenant
crosses the line.

### 1.2 What makes the association "individually identifiable health information"

45 CFR §160.103 defines *individually identifiable health information* as
information that

> "Relates to the past, present, or future physical or mental health or
> condition of an individual; **the provision of health care to an
> individual**; or the past, present, or future payment for the provision
> of health care to an individual; and (i) That identifies the individual;
> or (ii) With respect to which there is a reasonable basis to believe the
> information can be used to identify the individual."

([law.cornell.edu/cfr/text/45/160.103](https://www.law.cornell.edu/cfr/text/45/160.103)
— eCFR and hhs.gov both refused automated fetch; see the sourcing note.)

Two things follow.

- **The clause is disjunctive.** Information need not describe a health
  condition. Relating to *the provision of health care to an individual* is
  enough on its own. This is why the appointment-reminder case is the
  standard analogue: a reminder carries no diagnosis, yet it is PHI,
  because it says this identified person is receiving care from this
  provider.
- **"Health care" is broad.** §160.103 defines it as "Care, services, or
  supplies related to the health of an individual," expressly including
  "Preventive, diagnostic, therapeutic, rehabilitative, maintenance, or
  palliative care, and **counseling, service, assessment, or procedure**
  with respect to the physical or mental condition, or functional status,
  of an individual." Doula support falls inside that wording on its face.
  Whether a *given* practice is a covered entity is still gated by §1.1.

### 1.3 Platform voice

A Platform-voice Notification is addressed to a Practice Owner or Staff
member — Doula Cloud's own paying customer — and names no Practice in the
`From` line beyond Doula Cloud itself. The recipient is not an individual
receiving health care from Doula Cloud. Examples from the map: "you are out
of credits" (MO-G9), "your payout account is not finished" (DW-G2), "a
Payment arrived", security notices.

The recipient's own address is a §164.514(b)(2) identifier, but no element
of the message relates to the provision of health care **to that
recipient**. Without the relates-to limb, the §160.103 definition is not
met, so there is no PHI and no business-associate relationship arises from
this traffic.

**Answer: no BAA implicated for Platform voice**, provided the no-content
rule holds absolutely — no Client name, no Engagement reference, no
count-of-clients detail that would make the mail about a third party's
care. A Platform-voice mail that said "Nadia Haddad's contract is unsigned"
would leave this category and land in §1.4.

### 1.4 Practice voice — the association

A Practice-voice Notification is sent to that Practice's Client, from a
named doula practice. Even with an empty body, the delivery record held by
the sending vendor pairs:

- an **identifier** — the recipient's email address. "Electronic mail
  addresses" is identifier (F) in the safe-harbour list at 45 CFR
  §164.514(b)(2)(i)
  ([law.cornell.edu/cfr/text/45/164.514](https://www.law.cornell.edu/cfr/text/45/164.514));
  and
- a fact **relating to the provision of health care to that individual** —
  that this named practice is sending them mail as a client.

That is the full §160.103 test. The no-content rule removes the *contents*
of care but not the *fact* of care. This is exactly the appointment-reminder
problem: OCR has long treated reminders as PHI notwithstanding that they
disclose nothing clinical.

Whether this makes **Mailgun** a business associate then turns on the
conduit question.

### 1.5 The conduit exception does not save an ESP that keeps logs

OCR's cloud-computing guidance and FAQ 2077 set the boundary. Both pages
refuse automated fetch (HTTP 403, as #30 also found); the passages below
are the hhs.gov text as returned by search-engine extraction of those
pages, flagged accordingly:

> "The conduit exception is limited to transmission-only services for PHI
> (whether in electronic or paper form), including any temporary storage of
> PHI incident to such transmission."

> "A conduit transports information but does not access it other than on a
> random or infrequent basis as necessary for the performance of the
> transportation service or as required by law."

> "Where a CSP provides transmission services for a covered entity or
> business associate customer, in addition to maintaining ePHI for purposes
> of processing and/or storing the information, the CSP is still a business
> associate with respect to such transmission of ePHI."

> "CSPs that provide cloud services to a covered entity or business
> associate that involve creating, receiving, or maintaining (e.g., to
> process and/or store) electronic protected health information (ePHI) meet
> the definition of a business associate, **even if the CSP cannot view the
> ePHI because it is encrypted and the CSP does not have the decryption
> key**."

Sources: [FAQ
2077](https://www.hhs.gov/hipaa/for-professionals/faq/2077/can-a-csp-be-considered-to-be-a-conduit-like-the-postal-service-and-therefore-not-a-business%20associate-that-must-comply-with-the-hipaa-rules/index.html),
[Guidance on HIPAA & Cloud
Computing](https://www.hhs.gov/hipaa/for-professionals/special-topics/health-information-technology/cloud-computing/index.html).

Mailgun is not transmission-only. Its own pricing page sells retention as a
plan feature: "1 day of log retention" on Free and Basic, "5 days of log
retention" and "1 day of message retention" on Foundation, "30 days of log
retention" and "Up to 7 days of Message Retention" on Scale
([mailgun.com/pricing](https://www.mailgun.com/pricing/)). Retained
delivery logs and retained message copies are storage held for the
customer's benefit — analytics, deliverability, debugging — not temporary
storage incident to transmission. The encryption point in the last quote
closes the remaining escape: it would not help even if the payload were
opaque to Mailgun, and here the recipient address and the `From` identity
are not opaque at all.

**Answer: for Practice voice, a BAA is implicated once #30's trigger
fires.** The no-content rule reduces the exposure to metadata but does not
take the traffic outside §160.103, and the conduit exception does not reach
a provider that retains logs and message copies.

Note that Mailgun's own addendum tries to claim the conduit ground. It
states the addendum

> "does not apply to third party conduits and providers who are involved in
> the transmission, routing, storage or receipt of email which is inherent
> in the delivery of the Mailgun Services."

([mailgun.com/legal/hipaa-baa](https://www.mailgun.com/legal/hipaa-baa/))
Read plainly this carves out Mailgun's *own* downstream providers, not
Mailgun. It is not an assertion that Mailgun is a conduit — it signs the
addendum as Business Associate. But the sentence is ambiguous enough to be
worth a lawyer's eye.

### 1.6 Does the unencrypted-email-to-individuals guidance change this?

No — and this is the most likely place for the two questions to be
conflated, so state the distinction plainly.

OCR's individual-right-of-access guidance says a covered entity may honour
a request to receive PHI by unencrypted email:

> "The covered entity must provide a brief warning to the individual that
> there is some level of risk that the individual's PHI could be read or
> otherwise accessed by a third party while in transit, and confirm that
> the individual still wants to receive their PHI by unencrypted e-mail. If
> the individual says yes, the covered entity must comply with the
> request."

> "Covered entities are responsible for adopting reasonable safeguards in
> implementing the individual's request (e.g., correctly entering the
> e-mail address), but covered entities are not responsible for a
> disclosure of PHI while in transmission to the individual based on the
> individual's access request to receive the PHI in an unsecure manner."

Sources: [Individuals' Right under HIPAA to Access their Health
Information](https://www.hhs.gov/hipaa/for-professionals/privacy/guidance/access/index.html),
[FAQ
2061](https://www.hhs.gov/hipaa/for-professionals/faq/2061/is-a-covered-entity-responsible-if-it-complies/index.html),
[FAQ
2060](https://www.hhs.gov/hipaa/for-professionals/faq/2060/do-individuals-have-the-right-under-hipaa-to-have/index.html)
— again hhs.gov text via search extraction, direct fetch 403.

Three limits, all of which matter here:

1. **It is about the covered entity's transmission-security duty, not about
   the vendor's status.** It relieves the *practice* of liability for
   interception in transit. It says nothing about whether the ESP that
   holds the delivery log is a business associate. Those are different
   questions, and only the second one governs whether a BAA is needed.
2. **It is triggered by a request from the individual.** The right of
   access is exercised by the individual asking for a copy of their
   information. A portal invite or a "something is waiting" Notification is
   product-initiated mail, not an access-request fulfilment. The safe
   harbour is not on offer by default.
3. **It covers the leg to the individual, not the leg into the vendor.**
   Disclosing the association to Mailgun happens before transmission and is
   not something the Client's acceptance of interception risk addresses.

There is a real design idea buried in it — a Client who opts in to email
Notifications, warned of the risk, is on much firmer ground than one who
never chose. That belongs to the map's open NH-G7 question, not to the BAA
question.

## 2. Will Mailgun sign a BAA?

### 2.1 Yes — there is a published addendum

Mailgun publishes a **HIPAA Business Associate Addendum** at
[mailgun.com/legal/hipaa-baa](https://www.mailgun.com/legal/hipaa-baa/),
"Last revised 11/20/2020". It is written as an addendum to the Mailgun
Terms of Service and applies "in the event and to the extent Mailgun
meets... the definition of a Business Associate" under HIPAA. It defines
PHI by direct reference to 45 CFR §160.103 — the actual regulatory
definition, as Stripe's SSA does in #30 — "limited to the information
received by Business Associate from or on behalf of you". The Business
Associate is "Mailgun Technologies, Inc., a Delaware corporation... and on
behalf of its affiliates (any entity that is owned or that is under common
control with one of its entities, including but not limited to Mailgun
Technologies SAS and Mailjet SAS)."

Mailgun's own marketing says so directly:

> "Mailgun provides HIPAA compliant email services, and you've already seen
> our business associate addendum."

> "If you need a HIPAA compliant email provider, you've found your
> solution. Create an account today, sign our BAA, and use Mailgun to
> simplify and protect your email communications."

([mailgun.com/blog/email/email-hipaa-compliance](https://www.mailgun.com/blog/email/email-hipaa-compliance/))

Mailgun's own security-and-compliance guide states: "Regulatory compliance:
Mailgun meets or exceeds GDPR and CCPA compliance to protect the privacy
and integrity of customer data. **Rights and responsibilities for HIPAA
compliance are defined in a Business Associate Addendum.**"
([GU-MG-Security-and-Compliance.pdf](https://www.mailgun.com/wp-content/uploads/2025/10/GU-MG-Security-and-Compliance.pdf),
p. 33; the HIPAA section on p. 16 says "ask to see the HIPAA Business
Associate Agreement (BAA)").

### 2.2 But the Terms of Service bar PHI until a separate agreement exists

This is the load-bearing clause and it is easy to miss. Sinch Email's Terms
of Service (last updated February 15, 2025), §4.1 "Security and Data
Privacy":

> "Customer agrees not to provide Sinch Email or use the Services in
> connection with any sensitive personal data or protected health
> information or other information that can be deemed sensitive personal
> data or protected health information **without obtaining Sinch Email's
> prior written consent and entering into a separate agreement with Sinch
> Email** governing the transmission of such information in connection with
> Customer's use and benefit of the Services."

([mailgun.com/legal/terms](https://www.mailgun.com/legal/terms/))

The Terms of Service do not otherwise mention HIPAA, PHI, or the addendum,
and do not incorporate the addendum automatically. So the sequence is:
prior written consent from Sinch, then an executed separate agreement, then
PHI may flow. Sending Practice-voice mail for a HIPAA-triggering tenant
before that is a breach of Mailgun's own terms, independent of any HIPAA
exposure.

### 2.3 What the addendum restricts and disclaims

Read against the map's design, the notable clauses of the addendum are:

- **Unsecured transmission is expressly acknowledged.** "Email sent using
  the Mailgun Services may be unsecured, may be intercepted by other users
  of the public internet, and may be stored and disclosed by third
  parties..."
- **Consent is pushed to the customer.** "You confirm that you have made
  these aspects of the Mailgun Services clear to your customers and end
  users as appropriate, and that they have provided full and adequate
  consent..." For Doula Cloud this reaches through two layers — Doula Cloud
  must have told the Practice, and the Practice must have told the Client.
  That is a product requirement, not just paperwork.
- **Encryption is the customer's job.** "You are responsible for encrypting
  any sensitive data you use in conjunction with the Mailgun Services."
  Under the no-content rule there is no body to encrypt; the exposure is
  the envelope, which the customer cannot encrypt.
- **No monitoring duty.** "Business Associate shall have no obligation to
  monitor or attempt to monitor the access to such emails."
- **Downstream carve-out.** The conduit sentence quoted in §1.5.
- **Return/destruction on request.** Mailgun will destroy PHI in its
  possession, including PHI held by subcontractors, on request.

Transport is **opportunistic TLS**, not enforced TLS: Mailgun "applies
opportunistic TLS encryption to protect messages sent from the platform in
transit" (security-and-compliance guide, p. 33). Opportunistic means it
downgrades to plaintext if the receiving server does not offer TLS. Whether
Mailgun offers a per-message *required*-TLS option was **not** confirmed
first-party in this pass — the sending-messages page fetched documents no
such parameter — and it should be confirmed before any argument relies on
enforced transport encryption.

### 2.4 Plan, cost, and feature restrictions — not confirmed first-party

Nothing on Mailgun's pricing, legal, healthcare-industry, or enterprise
pages names a plan tier for the BAA, states a fee, or lists a feature
restriction attached to it. The addendum itself names no tier. Mailgun's
Help Center article "Compliance & Security"
([help.mailgun.com/hc/en-us/articles/13402314405275](https://help.mailgun.com/hc/en-us/articles/13402314405275-Compliance-Security))
is the most likely first-party home for that answer and it is behind a
Cloudflare bot wall — HTTP 403 to both WebFetch and a browser-UA curl.
Secondary sources disagree with each other on whether HIPAA is
enterprise-only, so none are relied on here. **Treat plan and cost as
unknown and settle them with Mailgun directly.**

What *is* confirmed first-party is the plan structure the BAA would sit on
top of ([mailgun.com/pricing](https://www.mailgun.com/pricing/)):

| Plan | Price | Included volume | Log retention | Message retention | Custom sending domains |
| --- | --- | --- | --- | --- | --- |
| Free | $0/mo | 100 emails/day | 1 day | 1 day | 1 |
| Basic | from $15/mo | 10,000 emails/mo | 1 day | not stated | 1 |
| Foundation | $35/mo | 50,000 emails/mo | 5 days | 1 day | 1,000 |
| Scale | $90/mo | 100,000 emails/mo | 30 days | up to 7 days | 1,000 |

Subaccounts are not itemised on the pricing page and were not confirmed.
Two things are worth noting for the map even before the BAA question
resolves: **log retention is a compliance surface that scales with the
plan** — the more you pay, the longer the association sits in Mailgun — and
the 1-domain limit on Free and Basic forecloses any future per-tenant
sending domain, which the map already lists as beyond v1.

### 2.5 Sinch, the parent

Sinch's HIPAA statement
([sinch.com/legal/policies-statements/other-sinch-policies-statements/hipaa](https://sinch.com/legal/policies-statements/other-sinch-policies-statements/hipaa/))
says only that "Sinch has various products and solutions where the HIPAA
compliance framework has been implemented" and that "primary responsibility
for compliance with HIPAA rests with you." **It publishes no
product-by-product coverage list**, which is the single most useful thing
it could publish and the reason the Mailgun-specific addendum carries the
weight here.

Sinch's third-party HIPAA validation does **not** name email. Its own
announcement describes validation "for its security over voice, fax and
UCaaS services," assessed by BDO USA, dated June 29, 2022
([sinch.com/news/sinch-secures-hipaa-compliance-validation-security-voice-fax-and-ucaas](https://sinch.com/news/sinch-secures-hipaa-compliance-validation-security-voice-fax-and-ucaas/)).
Mailgun and email are absent from that scope. This is not evidence against
the email BAA — the addendum is Mailgun-specific and predates the
acquisition — but it means the Sinch-level validation cannot be cited as
covering this traffic.

As #30 recorded for Google, there is no official third-party HIPAA
"certification" in the first place; what exists is an executed BAA plus
correct operation.

## 3. If Mailgun will not sign — the options, not decided here

Mailgun's posture reads as *will sign*, so this section is a contingency
against §2.2's consent step being refused, priced badly, or gated behind a
tier that does not suit a pre-launch product. The options are named, not
ranked.

### 3.1 Alternative vendors that offer a BAA

- **AWS SES.** "Amazon Simple Email Service (Amazon SES)" appears on the
  AWS HIPAA-eligible services list with no footnote or caveat attached.
  The page's condition is explicit: "If you are a Covered Entity or
  Business Associate as defined by [HIPAA]... you agree not to use these
  HIPAA Eligible Services for any purpose or in any manner involving
  Protected Health Information... without first entering into an AWS
  business associate agreement."
  ([aws.amazon.com/compliance/hipaa-eligible-services-reference](https://aws.amazon.com/compliance/hipaa-eligible-services-reference/))
  Confirmed first-party. Note the platform is otherwise on Google Cloud
  (#30), so this adds a second cloud vendor and a second BAA.
- **Paubox Email API.** Purpose-built for healthcare transactional mail,
  REST API plus SMTP relay on `smtp.paubox.com:587`. "A Business Associate
  Agreement is included with every plan," including a free tier of 300
  emails per month; Paubox's own pricing FAQ states "Are there any fees for
  a business associate agreement? No. All customers receive a business
  associate agreement at no additional charge."
  ([paubox.com/products/email-api](https://www.paubox.com/products/email-api),
  [paubox.com/pricing](https://www.paubox.com/pricing)) The Email Suite
  pricing page renders its figures via JavaScript and showed placeholder
  values on fetch, so Email API pricing above the free tier is not
  confirmed here.
- **LuxSci.** Offers a published HIPAA BAA
  ([luxsci.com/company/legal/baa](https://luxsci.com/company/legal/baa/))
  and a "Secure High Volume Email" product for transactional and
  high-volume sending
  ([luxsci.com/hipaa-compliant-email.html](https://luxsci.com/hipaa-compliant-email.html)).
  Eligible plans and pricing were not confirmed first-party; the pricing
  page was not fetched in this pass.

### 3.2 Change the Practice-voice sending identity so the association never reaches the vendor

The architectural option, and the one that keeps the map's stated vendor
choice intact. If Practice-voice mail is sent from a **generic Doula Cloud
identity** — no practice name in the `From` line, display name, subject, or
envelope — then what Mailgun's logs hold is an email address plus the fact
that a doula-support platform mailed it.

Two honest caveats, both of which this ticket leaves open rather than
resolving:

- It **weakens the association, it does not obviously erase it**. §160.103's
  "reasonable basis to believe the information can be used to identify the
  individual" limb is about identifiability, and the address already
  identifies. The relates-to limb is what thins out: "Doula Cloud mailed
  this person" is a weaker statement about the provision of health care
  than "Blessingway mailed this person," but it is not nothing, and the
  same reasoning that made Doula Cloud a subcontractor BA in #30 could be
  read to cover it. This is a judgement a lawyer should make.
- It **costs the map something real**. The map's whole point in separating
  the two voices is that a Client should recognise the practice she hired,
  not a vendor she has never heard of. A portal invite from an unfamiliar
  sender is a deliverability and trust problem before it is a compliance
  one. Weighing that against the vendor exposure is the sending-identity
  ticket's job, not this one's.

A middle position exists and is worth naming: keep the practice name in the
**reply-to or the rendered body** while the envelope and `From` stay
generic. Whether that actually keeps the name out of Mailgun's retained
logs depends on what Mailgun stores — with message retention on, it stores
the message — so it must be verified against Mailgun's retention behaviour
before it is treated as a mitigation.

## 4. Legal review, and what this document is not

This is a technical and compliance-surface scan. It is not legal advice, and
the three points below are where a health-privacy attorney should be asked
rather than an engineer:

1. **Whether metadata-only association is PHI in this specific fact
   pattern.** The reasoning in §1.4 follows the regulatory text and the
   appointment-reminder analogue, but no OCR guidance found in this pass
   addresses an ESP's delivery log for a content-free notification
   directly.
2. **Whether the generic-sender mitigation in §3.2 actually works.** It is
   the kind of question that turns on a de-identification judgement, and
   getting it wrong would be discovered late.
3. **The chain of liability** — tenant Practice → Doula Cloud → Mailgun/Sinch
   → Mailgun's own subcontractors — which #30 already flagged for the
   Google Cloud leg and which this adds a branch to. Mailgun's addendum
   accepts responsibility for PHI held by its subcontractors on the
   destruction clause, but the broader flow-down was not analysed here.

Given that Medicaid doula reimbursement is expanding and #30 judged the
insurance-billing trigger plausible in this market, the question is when
legal review happens, not whether.

## Note on sourcing

**Fetched directly and quoted first-party:** `mailgun.com/legal/hipaa-baa`,
`mailgun.com/legal/terms`, `mailgun.com/pricing`,
`mailgun.com/industries/healthcare`, `mailgun.com/blog/email/email-hipaa-compliance`,
Mailgun's security-and-compliance PDF (text extracted locally),
`sinch.com/legal/policies-statements/.../hipaa`, `sinch.com/news/...`,
`aws.amazon.com/compliance/hipaa-eligible-services-reference`,
`paubox.com/pricing`, `paubox.com/products/email-api`,
`luxsci.com/hipaa-compliant-email.html`,
`law.cornell.edu/cfr/text/45/160.103`, and
`law.cornell.edu/cfr/text/45/164.514`. LuxSci's BAA URL is the link
published on the fetched LuxSci page; the BAA document itself was not
fetched.

**Blocked, and how it was handled:**

- **hhs.gov refused every automated fetch** (HTTP 403 to WebFetch and to a
  browser-UA curl), the same behaviour #30 recorded. The OCR passages in
  §1.5 and §1.6 are hhs.gov page text as returned by search-engine
  extraction of the named URLs, and are flagged as such at each use. They
  are consistent with the regulatory text quoted from Cornell LII, but a
  direct read of the hhs.gov pages is recommended before treating them as
  final.
- **eCFR** (`ecfr.gov`) redirected all requests to
  `unblock.federalregister.gov`. The §160.103 text is therefore quoted from
  the Cornell LII mirror, as #30 did.
- **Mailgun's Help Center** (`help.mailgun.com`) is behind a Cloudflare
  interstitial and returned 403 to both fetch methods. This is why the plan
  tier and cost of the BAA are marked unconfirmed rather than answered.

**Not verified in this pass, and worth closing before the sending-identity
decision:** whether the BAA requires a specific Mailgun plan and what it
costs; whether execution is self-serve or sales-gated; whether Mailgun
supports subaccounts on the relevant plan; whether Mailgun's per-message
required-TLS option exists and works as documented; exactly what a Mailgun
delivery log row retains when message retention is off; LuxSci's
BAA-eligible plans and pricing; Paubox Email API pricing above the free
tier.
