# Identity Platform tier and BAA — what doula-cloud actually runs, and whether it is covered

Research for wayfinder ticket [#165](https://github.com/markgoho/doula-cloud/issues/165), part of map [#164](https://github.com/markgoho/doula-cloud/issues/164) ("Auth methods: what Doula Cloud supports, for whom, and why"). [ADR-0004](../adr/0004-bff-owned-sessions.md) §17 already recorded a partial answer from ticket [#181](https://github.com/markgoho/doula-cloud/issues/181): the `doula-cloud` project runs the upgraded `IDENTITY_PLATFORM` tier, and Identity Platform is named on Google's HIPAA covered-products list. Everything in that partial answer is reconfirmed here, live, against the project itself, rather than taken on the ADR's word. What #181 did not ask, and #164 flags as the omission, is the harder question: does appearing on a covered-products list mean anything has actually been signed? It does not, and that gap — whether a BAA is *executed* for this project's billing account, not just theoretically available — is this ticket's real contribution.

**This is a technical and compliance-surface scan, matching the style of prior BAA research in this repo (`docs/research/esignature-vendor-baa-embeddable-api.md`, `origin/research/mailgun-baa-posture` → `docs/research/mailgun-baa-posture.md`). It is not legal advice**, though the BAA-execution and legal-entity questions below are answered directly rather than deferred to counsel, per repo policy on pre-launch findings.

## Summary

| Question | Answer | Verified |
| --- | --- | --- |
| Which tier is provisioned | `IDENTITY_PLATFORM` (the upgraded, GCP-billed tier), not legacy Firebase Authentication | Live `GetConfig` call against `identitytoolkit.googleapis.com`, today |
| Does that tier appear on Google's HIPAA covered-products list | Yes, by the exact name "Identity Platform" | Live fetch of `cloud.google.com/security/compliance/hipaa`, today |
| Does "Firebase Authentication" appear on that list | No — absent entirely; the only Firebase-branded covered entry is "Cloud Storage for Firebase" | Same fetch |
| **Is a BAA actually executed for the account holding this project** | **Yes** — "Reviewed and accepted on Aug 30, 2026 by markgoho@gmail.com" | Live Cloud Console check (Legal & Compliance page), today, with screenshot |
| Does the lack of a formed legal entity block this today | No — Mark Goho, as sole proprietor, is both the accepting party and the operating entity; HIPAA's "Person" definition names a natural person first, separately from every organizational form. Re-acceptance under the LLC's name is required once the LLC forms and becomes the operating entity — not optional, and not yet due | BAA text and 45 CFR §160.103, both fetched live |
| Feature gap (MFA, multi-tenancy, SAML, OIDC, SLA) | All five: Firebase Authentication has none of them; Identity Platform has all five, including a 99.95% SLA | Google's own product-comparison page |
| Reversible upgrade | No — Firebase Authentication → Identity Platform does not require app changes, but there is no supported downgrade path back | Google's own developer-support forums (flagged as community sourcing, not formal docs) |

**Bottom line: the compliance blocker map #164 flagged is closed.** The tier is the covered one, the BAA is executed, and the not-yet-formed LLC does not stand in the way of any of it. See "What this means for map #164" below. No follow-up ticket is proposed — see that section for why.

## 1. Which tier is provisioned, verified live

`api/internal/authn/authn.go`'s use of "Identity Platform" as a name proves nothing about billing tier — Firebase Authentication and Identity Platform are the same backend behind two front doors, and the code would read identically either way, which is exactly what #164 and #165 flag. The only field that discriminates them is `subtype` on the Identity Toolkit Admin API's project config, and it is output-only — not something a docstring or a console label can spoof.

Command run against the live project today:

```sh
gcloud config get-value project   # doula-cloud
gcloud config get-value account   # markgoho@gmail.com
TOKEN=$(gcloud auth print-access-token)
curl -s -H "Authorization: Bearer $TOKEN" \
     -H "x-goog-user-project: doula-cloud" \
     "https://identitytoolkit.googleapis.com/admin/v2/projects/doula-cloud/config"
```

Result (relevant field):

```json
"subtype": "IDENTITY_PLATFORM"
```

This is the exact discriminator the API uses to distinguish `IDENTITY_PLATFORM` from `FIREBASE_AUTH`. It reproduces #181's finding (closed 2026-08-20) independently, twelve days later, against the live project rather than a prior transcript.

Corroborating, non-decisive signal: `gcloud services list --enabled --project=doula-cloud` shows `identitytoolkit.googleapis.com` enabled — but this API backs *both* tiers, so its presence alone proves nothing; it is listed here only because the ticket asked for enabled-services confirmation. Checking billable Identity Platform SKUs directly (Cloud Billing API) was not attempted: `cloudbilling.googleapis.com` is disabled on this project, and enabling a new API purely to inspect billing data would change project state for a verification task that the `subtype` field already answers authoritatively. If a second, cost-side confirmation is ever wanted, `gcloud services enable cloudbilling.googleapis.com --project=doula-cloud` followed by the Cloud Billing Catalog API would show whether Identity Platform SKUs beyond the free Spark-equivalent tier are present on this project's bill.

The full config response also surfaced two facts relevant to §5 below: `"mfa": {"state": "DISABLED"}` (the capability is provisioned but switched off) and `"multiTenant": {}` (the capability is provisioned, no tenants configured).

## 2. Covered-products-list membership — the two products checked separately

Fetched live today: [cloud.google.com/security/compliance/hipaa](https://cloud.google.com/security/compliance/hipaa).

> "The [Google Cloud BAA](https://cloud.google.com/terms/hipaa-baa) covers Google Cloud's entire infrastructure (all regions, all zones, all network paths, all points of presence), and the following products:" — followed by an alphabetical list, updated "as new products become available to the HIPAA program."

- **"Identity Platform" appears in that list by its exact name.**
- **"Firebase Authentication" does not appear anywhere on the page.** Searching every Firebase-branded entry in the covered-products list turns up exactly one: "Cloud Storage for Firebase." No authentication product branded "Firebase" is covered.

This confirms, independently of ADR-0004 §17's prior claim, that the two products do not share standing on Google's list: the upgraded tier is covered, the free tier's authentication product is not. This matches the general pattern the industry writes about Firebase/Identity Platform HIPAA posture, but the finding here rests on the primary list itself, not a secondary summary of it.

## 3. Is a BAA actually executed for this project's billing account — the question nobody had asked

Being named on the covered-products list means Google is *willing* to sign a BAA covering that product. It says nothing about whether a BAA has been *accepted* for this specific account. #181 (closed 2026-08-20) confirmed the tier and the list membership and stopped there; ADR-0004 §17's "Resolved (#181)" note reads as though the compliance question was closed, but nothing in it addresses execution. That is precisely the blind spot #164's map author flagged for this ticket, and it turned out to matter: the Console records a single acceptance, dated 2026-08-30 — ten days after #181 closed — with no earlier acceptance recorded anywhere in the account's Legal & Compliance history as viewed today.

`doula-cloud`'s billing account: `01873B-A8A1B5-E62BC2` ("3rd Time's the Charm"), account type "Direct," linked to four projects including `doula-cloud`. This billing account is not part of a Cloud Identity / Google Cloud organization — the project's own IAM & Admin Settings page states plainly: "Access Transparency is not available for projects that are not part of an organization." The account exists under the personal Google identity `markgoho@gmail.com`, consistent with the business currently operating without a formed legal entity.

The BAA acceptance record lives at `console.cloud.google.com/iam-admin/privacy?project=doula-cloud`, under "Legal & Compliance" → "Additional terms for Google Cloud Platform." Checked live today via an authenticated browser session as `markgoho@gmail.com`, the project owner:

> **Google Cloud Platform HIPAA Business Associate Addendum**
> Review the Google Cloud Platform HIPAA Business Associate Addendum
> Reviewed and accepted on Aug 30, 2026 by markgoho@gmail.com

Screenshot on file confirms this text rendered on the live Console page, with "You're currently working in Doula Cloud" visible in the header, ruling out a stale or wrong-project cache. For contrast, the "Cloud Data Processing Addendum" directly above it on the same page shows an active "Review and Accept" button with no acceptance date — that one is not yet accepted, which is unrelated to HIPAA but confirms the page correctly distinguishes accepted terms from unaccepted ones rather than showing everything as accepted by default.

**The timeline gap, stated plainly:** #181 closed 2026-08-20 having confirmed only the tier and the covered-products-list membership. The BAA itself was not clicked-to-accept until 2026-08-30 — ten days later. For those ten days, ADR-0004 §17 read as resolved while the actual paperwork protecting the project's identity data was not in force. That gap has since closed — the BAA is executed as of this research (2026-09-01) — but nothing in the repo records the execution fact or its date, so a future reader of ADR-0004 §17 alone would have no way to know whether "covered" meant "eligible" or "in force." This document, plus a short addendum to ADR-0004 §17 landed in the same change, closes that gap.

## 4. No legal entity required — what the BAA text itself says

Fetched live today: the actual BAA document at [cloud.google.com/terms/hipaa-baa](https://cloud.google.com/terms/hipaa-baa) (rendered client-side; static `curl` returns an empty shell, so this was fetched via a real browser).

> "This HIPAA Business Associate Addendum ("BAA") is entered into between Google LLC ("Google") and the customer agreeing to the terms below ("Customer")... This BAA will be effective when Customer clicks to accept this BAA (the "BAA Effective Date"). Customer must have an existing Services Agreement in place for this BAA to be valid and effective... You represent and warrant that (i) you have the full legal authority to bind Customer to this BAA, (ii) you have read and understand this BAA, and (iii) you agree, on behalf of Customer, to the terms of this BAA."

Nothing in this text requires "Customer" to be an incorporated legal entity. Customer is defined circularly and simply as whoever accepts the terms and already holds a Services Agreement (the standard Google Cloud Platform terms every account, including a free-tier individual account, accepts on sign-up) — a natural person qualifies on the same footing as a corporation. HIPAA's own definitions support this: 45 CFR §160.103, fetched live today, defines "Person" as "a natural person (meaning a human being who is born alive), trust or estate, partnership, corporation, professional association or corporation, or other entity, public or private." A natural person is named first and separately from every organizational form, and a sole proprietor doing business under a trade name is routinely treated as a covered entity or business associate in that individual capacity, not as something that first requires incorporation.

**Determination — the BAA is valid today, as accepted, by Mark Goho personally:** the business currently has no formed legal entity — a single-member NY LLC is planned, not yet filed — and this does not block or weaken the BAA accepted on 2026-08-30. Mark Goho, operating Doula Cloud as a sole proprietor, had full legal authority to bind himself as "Customer," because he *is* Customer; there was no separate entity that needed to exist first for this acceptance to be valid. The GCP account structure corroborates this reading: a standalone "Direct" billing account under a personal Google identity, outside any Cloud Identity organization, is exactly what a pre-incorporation sole proprietor's GCP presence looks like.

**What LLC formation changes, and why it is not optional:** an LLC is a distinct legal person from its member for contract purposes — that it may be a disregarded entity for federal income tax is a separate, tax-only concept and does not make the LLC the same contracting party as Mark personally. A BAA Mark accepted as an individual binds Mark, not an LLC that does not yet exist. So once the single-member NY LLC is filed and becomes the entity actually operating Doula Cloud, two things need to happen together, not just one: the GCP billing account's legal/company name should be updated to the LLC, **and** the HIPAA BAA should be re-reviewed and re-accepted under that LLC's name. Re-acceptance is the step that makes the LLC — the entity that will actually be creating, receiving, and transmitting PHI through the Covered Services — the party Google is a business associate to; skipping it would leave the LLC operating the product while the only executed BAA still names an individual who, by then, is no longer the operating entity. This is a needed step at LLC-formation time, not a hygiene nicety, though it is not a gap today: today, Mark operating as a sole proprietor is both the accepting party and the operating entity, so the BAA already covers the entity actually running the product.

## 5. Feature and pricing differences between the tiers

Source: Google's own [product-comparison page](https://docs.cloud.google.com/identity-platform/docs/product-comparison), fetched live today.

| Feature | Firebase Authentication | Identity Platform |
| --- | --- | --- |
| Multi-factor authentication | No | Yes |
| Multi-tenancy | No | Yes |
| Sign in with SAML | No | Yes |
| Sign in with OIDC | No | Yes |
| Enterprise SLA | No | Yes — 99.95% uptime |

Pricing (source: [firebase.google.com/pricing](https://firebase.google.com/pricing), fetched live for the free-tier boundaries; the per-MAU dollar figures beyond those boundaries are corroborated by a secondary aggregator and flagged as such):

- Standard sign-in (email/password, phone, federated social): no-cost up to 50,000 MAU per project per month, then Google Cloud per-MAU pricing. Secondary-sourced tier figures beyond the free line: ~$0.0055/MAU (50,001–100,000), ~$0.0046/MAU (100,001–1,000,000), ~$0.0032/MAU (1,000,001–10,000,000), ~$0.0025/MAU (10,000,000+) — not independently re-verified against Google's own pricing calculator widget, which renders as an interactive tool rather than static text.
- SAML/OIDC sign-in: no-cost up to only 50 MAU per project per month, then Google Cloud per-MAU pricing — secondary-sourced at ~$0.015/MAU beyond that.
- Phone (SMS) authentication: billed per message sent, country-dependent, fetched directly from [cloud.google.com/identity-platform/pricing](https://cloud.google.com/identity-platform/pricing) — rates run from roughly $0.01/SMS (e.g. Canada, South Korea) to $0.30–$0.52/SMS for a number of higher-cost destination countries.
- For a pilot-scale 14-doula practice with a handful of Client accounts per doula, the practice sits well inside every free tier above except SAML/OIDC, which is priced for enterprise SSO rather than a handful of Staff federating with Google — relevant if #164 eventually adopts Google sign-in for Staff at scale.

At present, `doula-cloud` is provisioned for MFA and multi-tenancy (both keys exist in the live config from §1) but neither is turned on: `mfa.state` is `DISABLED` and `multiTenant` is an empty object with no tenants configured. SAML and OIDC providers were not checked for configuration state in this pass — the question here was tier eligibility, not current provider configuration.

## 6. Reversibility

Google's own docs state upgrading requires no app changes: "You do not need to change your apps when upgrading from Firebase Authentication to Identity Platform... and your app continues to work with existing Firebase services." No formal Google documentation page addressing the reverse direction was found. Google's own developer-support forums (`groups.google.com/g/google-cicp-discussion`, Google's Developer forums) are consistent on this point: downgrading Identity Platform back to plain Firebase Authentication is not supported through any standard, self-serve means; the answer given there is to open a support ticket, without confirmation that support can actually do it. This sourcing is flagged as community/support-forum, not formal documentation, because no first-party Google docs page states it outright.

Practically, this is moot for `doula-cloud`: the project is already on the upgraded tier, the product has not launched, and there is no scenario in which downgrading serves any purpose — the tier decision is a one-way door already walked through, correctly, before this ticket ever asked the question.

## What this means for map #164

- The compliance blocker map #164 lists under "Not yet specified" — "Whether the Identity Platform tier actually in use is BAA-covered" — is answered, fully, not just partially: the tier is the upgraded one, Identity Platform is on the covered-products list, and, the piece #181 never checked, a BAA is actually executed for this project's billing account (accepted 2026-08-30). #164's own framing — "This outranks everything else here... if the tier is not covered the provider question reopens on compliance grounds" — does not fire. The provider question does not reopen.
- The lack of a formed legal entity, which #164's map itself does not mention but which is a standing fact about the business, does not create a gap today. See §4: Mark Goho, as sole proprietor, is both the accepting party and the operating entity right now. It does create a forward obligation — re-accept the BAA under the LLC's name once it forms and becomes the operating entity — which belongs with LLC-formation paperwork generally, not with this map.
- ADR-0004's identity-provider decision (keeping Identity Platform, reduced to a single `VerifyIDToken` call) now stands on a fully verified footing — tier, list membership, and execution — rather than the first two alone.
- Every other open question on map #164 (Client password vs. magic link, Staff passkeys, which federated methods, account-linking, who sends password-reset email) can proceed without re-litigating the compliance question. Federation (Google/Apple sign-in for Staff) is now confirmed technically available on the current tier at no cost within the practice's likely scale (§5) — a fact the map's federation question can use directly, though it does not decide it.
- Recommend map #164's "Not yet specified" bullet be updated to reflect this as resolved, and its "Decisions so far" section note the BAA-execution fact, once whoever owns #164 does that edit — this document does not edit #164 itself per this ticket's constraints.

## No follow-up ticket proposed

The tier in use is BAA-covered, and a BAA is executed for this project's billing account. Neither of the two conditions that would trigger a follow-up ticket (per #165's own "Done when" criteria) is present. The one loose end found — ADR-0004 §17 reading as fully resolved when it predates the actual BAA execution by ten days — is closed in this same change by appending the execution fact to ADR-0004 §17, rather than filed as a separate ticket, since it is a one-line documentation correction with no design or code implication.

## Note on sourcing

**Fetched or queried live against the project or Google's own pages, today (2026-09-01):**

- `identitytoolkit.googleapis.com` Admin v2 `GetConfig`, via `gcloud auth print-access-token` + `curl` with an `x-goog-user-project` header — the project's own tier data, §1.
- `gcloud services list --enabled --project=doula-cloud` — enabled-services corroboration, §1.
- `cloud.google.com/security/compliance/hipaa` — the HIPAA covered-products list, §2.
- Google Cloud Console, `iam-admin/privacy?project=doula-cloud`, authenticated as the project owner via a real browser session (Playwriter, not a launched/automated Chromium instance) — the BAA acceptance record and its date, §3, with a screenshot on file.
- Google Cloud Console, `billing/linkedaccount` and the linked billing account's Account Management page — billing account ID, name, "Direct" account type, and organization status, §3.
- `cloud.google.com/terms/hipaa-baa` — the BAA's own text, rendered via browser since the page is client-side rendered and a static fetch returns an empty shell, §4.
- `docs.cloud.google.com/identity-platform/docs/product-comparison` — the feature comparison table, §5.
- `firebase.google.com/pricing` and `cloud.google.com/identity-platform/pricing` — free-tier boundaries and SMS-by-country rates, §5.

**Secondary-sourced, flagged inline where used:** the specific per-MAU dollar figures beyond each free tier's boundary (§5), and the claim that downgrading Identity Platform to Firebase Authentication has no supported path (§6) — the latter from Google's own community/developer-support forums rather than a formal docs page, because no formal page addressing the reverse direction was found.

**Not verified in this pass:** current SAML/OIDC provider configuration state on the project (only tier eligibility was in scope); whether Identity Platform SKUs appear on an actual Cloud Billing invoice for this project (the `subtype` field was treated as sufficient and enabling Cloud Billing API was avoided to not alter project state, §1); the precise scope of BAA acceptance across the account holder's other GCP projects (`oregon-trail-camporee`, `doula-cooperative`, `sturgeons-law`) — Google's own support guidance says acceptance in one project covers the account, but this was not independently re-verified against each of those other projects.
