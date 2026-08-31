# What Stripe demands of a Connect platform before it moves real money

Research for [#382](https://github.com/markgoho/doula-cloud/issues/382), on the map
[#375](https://github.com/markgoho/doula-cloud/issues/375). Checked 25 August 2026.

## Method, and what is first-party

Two kinds of evidence appear below, and they are labelled.

- **Verified live** — read out of the Stripe Sandbox with the authenticated CLI and the
  Stripe MCP tools, or read off the public internet with `curl`/`dig`.
- **Read first-party** — quoted from `docs.stripe.com`, `support.stripe.com`, or the
  Stripe Services Agreement at `stripe.com/legal/ssa-services-terms`. No blog posts, no
  formation-service marketing.

The Sandbox proves the *shape* of things. It cannot prove activation, because activation
happens only in live mode and no live account exists yet — which is itself the first
finding.

## The state today, verified live

| Fact | Evidence |
| --- | --- |
| One Stripe account exists, and it is the Sandbox | `list_available_accounts_or_orgs` returns exactly one entry: `acct_1U7N3e1rKoVEA79v`, `"livemode": false`, "Doula Cloud sandbox" |
| That account has submitted nothing | `GET /v1/account` → `"details_submitted": false`, `"charges_enabled": false`, `"payouts_enabled": false`, `"business_type": null`, `"capabilities": {}` |
| It already claims a website | `business_profile.url` is `https://doula.cloud` |
| That website does not exist | `dig` resolves `doula.cloud` to `35.219.200.10`, but `curl https://doula.cloud` fails the TLS handshake (`SSL_ERROR_SYSCALL`), and `www.doula.cloud` does not resolve at all. A reviewer opening the declared URL today sees nothing. |
| Five v2 connected accounts exist, all sandbox | `GET /v2/core/accounts` — all `"livemode": false`, `applied_configurations: ["merchant"]` |

Nothing here transfers to production. A live Stripe account is a separate account, created
by registering in live mode; the Sandbox's onboarding state, connected accounts and keys do
not carry over.

## 1. What Stripe asks for at activation

Stripe frames activation as *verifying your business*, not as a form with a fixed field
list. The fields vary "by country, capabilities, business type, business structure, service
agreement type and risk level"
([Identity verification](https://docs.stripe.com/connect/identity-verification)), and Stripe
reserves the right to ask for more: "As you use more Stripe services, we might request
additional information or verification"
([Set up your account](https://docs.stripe.com/get-started/account/activate)).

For a US company Stripe says it "might need to collect" information about the business
(name, address, tax ID number), the person opening the account (name, date of birth), and
any beneficial owners (identity verification, same page). The KYC support article puts the
same three buckets more plainly: "the individual creating the Stripe account, the business
associated with the Stripe account, any individuals who ultimately own or control that
business" ([KYC obligations](https://support.stripe.com/questions/know-your-customer-obligations)).

| What Stripe asks for | Source | Satisfiable today? |
| --- | --- | --- |
| A legal entity, with a business structure Stripe recognises. A single-member New York LLC maps to `single_member_llc`, which Stripe lists explicitly for US companies | [Identity verification, US business structures](https://docs.stripe.com/connect/identity-verification) | **No.** The LLC does not exist yet. That is the map's own destination. |
| Business tax ID number — EIN for a US company | Identity verification, "name, address, tax ID number" | **No.** No EIN yet. |
| Legal business address | Same | **No** as an entity address. [#384](https://github.com/markgoho/doula-cloud/issues/384) settles which of the five addresses goes where. |
| Representative: the person opening the account — name, date of birth, home address, SSN | Same, plus the KYC article's "individual creating the Stripe account" | **Yes.** Mark is the representative, and nothing has to be created first. |
| Beneficial owners, meaning anyone who owns or controls the business | Same | **Yes, trivially.** Single-member LLC: one owner, who is also the representative. |
| A bank account for payouts. Stripe's go-live checklist puts "Review your bank account information" ahead of accepting live charges | [Account checklist](https://docs.stripe.com/get-started/account/checklist) | **No.** The business bank account is a map ticket, and banks want the EIN and the filed articles first. |
| Industry / MCC, checked against the prohibited and restricted list | [Account checklist](https://docs.stripe.com/get-started/account/checklist), [Prohibited and restricted businesses](https://stripe.com/legal/restricted-businesses) | **Yes, decidable now.** Doula Cloud is practice-management SaaS. Worth walking the list deliberately rather than assuming: several MCCs around health services and software are restricted for individual payment methods. |
| A description of the product and the business | [Set up your account](https://docs.stripe.com/get-started/account/activate): "Provide information about your business, product, and relationship to the business" | **Yes.** The description exists in this repo already; it only has to be written into the form. |
| Public business information — support email, phone, address, support URL. Stripe shows these on statements and receipts | [Set up your account](https://docs.stripe.com/get-started/account/activate) | **Partly.** `admin@doula.cloud` exists. A support phone and a postal address depend on [#384](https://github.com/markgoho/doula-cloud/issues/384). |
| A business website | [Business website FAQ](https://support.stripe.com/questions/business-website-for-account-activation-faq) | **No.** See section 2 — this is the finding the ticket asked for. |
| Two-factor authentication on the platform account | [Account checklist](https://docs.stripe.com/get-started/account/checklist) | **Yes.** Free, and it gates other things: only administrators with 2FA can add funds to a platform balance. |
| An approved **Connect platform profile**, separate from account activation | [Add funds to your platform balance](https://docs.stripe.com/connect/top-ups): "In all markets, you must complete KYC to get your platform profile approved… You can check the status in your settings after completing the platform profile" (`dashboard.stripe.com/connect/settings/profile`) | **Not yet checkable.** The profile lives in live mode behind the Dashboard, and the Sandbox does not expose its field list through the API. Treat it as a second gate after entity activation, not a formality. |

**Expected volume** was named in the ticket. No first-party page in this sweep states that
Stripe requires a volume forecast at activation for a US platform. Stripe does collect
`business_profile.annual_revenue` and `estimated_worker_count` on the account object — both
`null` in the Sandbox — and the platform profile is the plausible place a volume question
appears. Recorded as **not confirmed either way**, and cheap to answer for real once the
live Dashboard exists.

## 2. The website review requirement — the teaser does not satisfy it

This is the ticket's sharp question, and the answer is no.

Stripe's own FAQ, [Business website for account activation](https://support.stripe.com/questions/business-website-for-account-activation-faq),
requires the site to carry, at minimum:

- the business name, and
- descriptions of the goods or services offered,

and, before selling, also customer service contact details, a refund and dispute policy, a
return policy where physical goods are involved, a cancellation policy where one applies,
and promotion terms where they apply.

Two structural requirements sit alongside the content list: the page **must load**, and it
**must be accessible without a password**.

Set that against what the teaser is going to be. The teaser map's own tickets describe one
page that says the product is coming in January 2027 and collects an email address —
[#363](https://github.com/markgoho/doula-cloud/issues/363) treats a second page as a cost
worth avoiding, calling it "a second page on a site that is meant to be one page." A
coming-soon page with an email capture carries the business name and, at best, a sentence of
positioning. It carries no description of the service, no support contact, and no refund,
dispute or cancellation policy — because there is nothing to buy yet.

So the teaser as scoped satisfies "loads" and "no password", and fails the content list.

**This is a real dependency between this map and the marketing site, and it needs its own
ticket.** The shape of the work is not "build the marketing site early". It is narrower: the
site that Stripe's reviewer opens must describe what Doula Cloud sells, name who to contact
for support, and state the refund and cancellation position. That can be one extra page on
the teaser domain, or a section of the teaser itself, and it does not need a published price
to be honest about how billing and cancellation work. It does have to exist and be public
**before** the platform account is submitted for activation, which puts it in front of the
end-of-October target rather than after it.

Two escapes were checked and neither applies:

- **A substitute for a website.** The FAQ does accept a social media profile or a mobile
  application in place of a site, with a full profile URL rather than a handle. Doula Cloud
  has neither a public social profile nor a published app. The FAQ does not offer "a written
  product description" as a standalone substitute.
- **Waiting until after activation.** Stripe applies the same website standard to connected
  accounts as an ongoing requirement, not a one-time one: the Connect Dashboard carries a
  **Business website information** requirement filter for accounts that need exactly this
  ([Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts)).
  A thin site is a live risk after activation, not only a gate before it.

## 3. What Stripe requires of the platform toward its Practices

From the **Stripe Connect Terms**, the Connect section of the Stripe Services Agreement
(`stripe.com/legal/ssa-services-terms`, "Stripe Connect - Platform", last modified
18 November 2025) and the [Connected Account Agreement](https://stripe.com/connect-account/legal/full).

- **Every Practice signs its own agreement with Stripe.** A `full` service agreement
  "creates a service relationship between Stripe and the connected account holder", and it is
  what a connected account needs to hold the `card_payments` capability
  ([Service agreement types](https://docs.stripe.com/connect/service-agreement-types)). The
  `recipient` agreement is the alternative and it cannot process card payments, so it is not
  an option here.
- **Stripe collects that acceptance for us, because onboarding is Stripe-hosted.** "Stripe
  handles the service agreement acceptance if you use Stripe-hosted onboarding or Embedded
  onboarding. For API onboarding, the platform must attest that their user has seen and
  accepted the service agreement" (same page). Doula Cloud sends Owners to Stripe's own hosted
  form (ADR-0007), so no attestation flow has to be built.
- **The platform must have its own agreement with the Practice.** The Connected Account
  Agreement repeatedly points the account holder at it — "User should read User's Platform
  Provider Agreement carefully to understand the nature of the Platform Services and the
  Activity" — and makes fee disclosure the platform's job: "Stripe does not control and is not
  responsible for fees imposed by a Stripe Connect Platform, which should be made clear to
  User in User's Platform Provider Agreement." **Not satisfiable today**, and it is
  product-legal work that #375 puts explicitly out of scope. It belongs to whatever map owns
  terms of service and the pilot agreements.
- **The platform stays responsible for fraud monitoring.** "Even after Stripe verifies a
  connected account, platforms still must monitor for and prevent fraud… Don't rely on
  Stripe's verification to meet any independent legal KYC or verification requirements"
  ([Identity verification](https://docs.stripe.com/connect/identity-verification)).
- **Stripe screens each Practice** for identity, risk-based KYC and AML, sanctions, MATCH list
  membership, and prohibited business categories
  ([Risk management](https://docs.stripe.com/connect/risk-management)). A Practice can be
  rejected, and the platform is the one who has to tell them and can appeal on their behalf.

## 4. Disputes and negative balances: the configuration, and the contract behind it

ADR-0007 set `defaults.responsibilities` on every Practice to `stripe` for both collectors:
the Practice's own account is billed the processing fee and absorbs a disputed charge, and
Stripe — not Doula Cloud — carries connected-account negative balances. The docs confirm what
that setting buys, and the SSA qualifies it.

**What the docs say.** With direct charges, "negative transactions for direct charges affect
the connected account's balance", and Stripe "first attempts to offset the negative amount by
collecting funds from the account's external account". With `losses_collector` = `stripe`,
"Stripe covers losses due to your connected accounts' negative balances", Stripe holds no
reserve against the platform account, and Stripe's risk teams own connected-account payments
risk ([Risk management](https://docs.stripe.com/connect/risk-management)). Had it been
`application`, Stripe could hold reserves on the platform account and, after 180 days of a
negative connected-account balance, transfer platform funds to cover it. Choosing `stripe`
removed a balance-sheet exposure that Doula Cloud has no capital to absorb.

**What the contract says.** Stripe Connect Terms §3.1: "As between User and Stripe, User is
responsible for all Activity on its Connected Accounts, whether initiated by User or not," and
the platform is liable for Transactions, Disputes, Refunds, Reversals and resulting Merchant
Losses — "except with respect to SMR Connected Accounts to the extent stated in Section 3.2."
§7 removes the cap: "User's liability for all Connected Accounts… is not limited or excluded
in any way."

**Stripe Managed Risk (SMR) is the legal name for the `stripe` losses collector.** §3.2:
"Stripe is liable for Merchant Losses on SMR Connected Accounts up to the amount of the SMR
Risk Cap (if any)," with carve-outs for the platform's own fraud, breach, negligence, willful
misconduct or misuse. Two consequences worth carrying forward:

1. **SMR has conditions the integration must keep meeting.** §2.3: *all* connected accounts
   must be SMR accounts, the platform "must use all applicable Stripe Technology that Stripe
   requires… (e.g., Stripe-Hosted Onboarding)", and Radar is on by default and "User must not
   disable these Services." Doula Cloud satisfies all three today. Creating a Practice with
   `losses_collector = application`, or replacing hosted onboarding with a bespoke form, would
   break the arrangement rather than merely change it.
2. **"SMR Risk Cap (if any)" is defined as "Stripe's maximum liability for Merchant Losses, if
   agreed by the parties in writing."** No such writing exists. The plain reading is that an
   unagreed cap leaves Stripe's cover uncapped, but the drafting is not unambiguous and §3.1's
   residual platform liability sits behind it. **Not confirmed first-party** — this is the one
   place in this document where an hour of a lawyer's time would be worth buying, and the map
   already carries that posture for [#383](https://github.com/markgoho/doula-cloud/issues/383).

The dispute itself lands on the Practice under direct charges, which is what ADR-0007 intended
and what keeps Client funds off Doula Cloud's balance sheet.

## 5. Tax reporting: who issues what

[US tax reporting for Connect platforms](https://docs.stripe.com/connect/tax-reporting) is
explicit: "Stripe issues 1099-K forms for connected accounts where Stripe controls the pricing
or for transactions where the connected account pays fees directly to Stripe. For all other
transactions where the platform controls the pricing, the platform is responsible for filing
any relevant 1099 forms." Concretely, Stripe issues the 1099-K where `controller.fees.payer`
is `account`, and does **not** where it is `application`, `application_express` or
`application_custom`.

**Doula Cloud is on the Stripe-issues side.** ADR-0007 set the fees collector to `stripe`: the
Practice pays Stripe's processing fee directly out of its own balance, and there is no
`ApplicationFeeAmount` anywhere in `payments`. So Stripe files the 1099-K for each Practice
that crosses the threshold, and Doula Cloud files nothing for its Practices' card volume.

Thresholds Stripe states on that page for a 1099-K: US-based or a US taxpayer, **more than
20,000 USD in gross volume and more than 200 transactions** in the previous calendar year. The
1099-NEC and 1099-MISC rows on the same page (600 USD, or 10 USD in royalties) describe
platforms that *pay* their connected accounts. Doula Cloud does not: Clients pay Practices
directly and no money passes through a Doula Cloud balance.

Two caveats:

- **The SSA default is the opposite of the docs' outcome**, and the SSA governs. Connect Terms
  §4.1: "Unless Stripe notifies User otherwise, Stripe will not file any, and User assumes sole
  responsibility and liability for filing all Tax Information Reports… Notwithstanding the
  prior sentence, to the extent required by Law, Stripe will file Tax Information Reports as
  outlined in the Documentation." The documentation is the tax-reporting page above, so the
  conclusion holds — but it holds *because of* the fees configuration, and it flips if that
  configuration ever changes. §4.2 makes the platform indemnify Stripe for a failure to file.
- **Doula Cloud's own income is its own problem.** Whatever Practices pay for the software is
  the LLC's revenue, reported on Mark's return while the LLC is a disregarded entity. Stripe's
  Connect tax reporting has nothing to say about it.

The IRS's own 1099-K threshold has moved more than once in recent years. The figures above are
what Stripe publishes today. They are Stripe's statement of Stripe's behaviour, not tax advice,
and Stripe says so: "Stripe recommends that you consult a tax advisor."

## 6. How long review takes, and what that does to end-of-October

**No published SLA was found for account activation or for platform-profile approval.** This
was looked for on the activation page, the account checklist, the KYC support article, the
identity-verification doc and the Connect risk pages. None of them states a duration. The one
review SLA Stripe does publish in the docs is for Terminal *app* review — "typically… within 2
working days… Most app submissions receive a review within 5 working days" — which is a
different process and does not transfer.

What the docs do say about timing is indirect, and all of it points the same way:

- Verification is not a single pass. Stripe "might require additional information, including,
  for example, a scan of a valid government-issued ID, a proof of address document, or both"
  ([Identity verification](https://docs.stripe.com/connect/identity-verification)).
- "Faster onboarding reviews" is listed as a *benefit* of Stripe Verified, an invitation-only
  programme based on "your platform's history with Stripe, transaction volume, and risk
  profile" ([Verified for platforms](https://docs.stripe.com/verified/verified-for-platforms)).
  A brand-new platform gets the ordinary path.
- The Connect platform profile is a second approval on top of account activation, with its own
  status field ([Add funds](https://docs.stripe.com/connect/top-ups)).

**Against the end-of-October target**, the honest planning position is: budget for round trips,
not for a queue. The risk to October is not that Stripe is slow. It is that each round trip
starts only when the thing it asks for exists. Entity, then EIN, then bank, then activation is
a serial chain, and the website work in section 2 has to finish before that chain's last link,
not after it. Submitting an activation that names a URL serving nothing — the state verified
today — is a guaranteed round trip.

## 7. What could not be confirmed first-party

Stated plainly, because the rest of this document is only as good as its honesty here.

1. **The Connect platform profile's actual field list.** It lives behind the live Dashboard at
   `dashboard.stripe.com/connect/settings/profile`, and no live account exists to open it. Its
   existence and its approval requirement are first-party (the top-ups doc). Its contents are
   not.
2. **Whether Stripe asks a platform for expected volume at activation.** Not found in the docs.
   The account object carries `annual_revenue` and `estimated_worker_count`, which is
   suggestive and not proof.
3. **Any review duration.** No first-party figure exists for this process. Nothing here should
   be read as "Stripe takes N days."
4. **Whether a reviewer would accept a teaser page carrying a fuller service description.** The
   requirement list is first-party; how a human reviewer applies it to a pre-launch product is
   not documented. Section 2's recommendation is built to satisfy the written list rather than
   to guess at a reviewer's latitude.
5. **The SMR Risk Cap question** in section 4. Definitional ambiguity, not a documentation gap.
6. **MCC selection.** The prohibited-and-restricted list was located but not walked
   code-by-code against what Doula Cloud will declare.

## Sources

All first-party. Fetched 25 August 2026.

- [Set up your account](https://docs.stripe.com/get-started/account/activate)
- [Account checklist](https://docs.stripe.com/get-started/account/checklist)
- [Business website for account activation (FAQ)](https://support.stripe.com/questions/business-website-for-account-activation-faq)
- [Know Your Customer obligations](https://support.stripe.com/questions/know-your-customer-obligations)
- [Identity verification for connected accounts](https://docs.stripe.com/connect/identity-verification)
- [Service agreement types](https://docs.stripe.com/connect/service-agreement-types)
- [Risk and liability management with Connect](https://docs.stripe.com/connect/risk-management)
- [Review and take action on connected accounts](https://docs.stripe.com/connect/dashboard/review-actionable-accounts)
- [US tax reporting for Connect platforms](https://docs.stripe.com/connect/tax-reporting)
- [Add funds to your platform balance](https://docs.stripe.com/connect/top-ups)
- [Stripe Verified for platforms](https://docs.stripe.com/verified/verified-for-platforms)
- [Stripe Services Agreement — "Stripe Connect - Platform" terms](https://stripe.com/legal/ssa-services-terms)
- [Stripe Connected Account Agreement](https://stripe.com/connect-account/legal/full)
- [Prohibited and restricted businesses](https://stripe.com/legal/restricted-businesses)
- The live Stripe Sandbox `acct_1U7N3e1rKoVEA79v`, via the authenticated `stripe` CLI and the Stripe MCP tools.
