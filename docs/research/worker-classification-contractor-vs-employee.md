# Worker classification: what makes a doula a contractor, not an employee

Research for [issue #249](https://github.com/markgoho/doula-cloud/issues/249). Question:
what do the legal tests for worker classification actually turn on, and does any of it
constrain this app's data model (ADR-0008)?

**This is not legal advice and not a classification recommendation.** No test below is
applied to any doula, Practice, or Engagement. The job here is to report what the tests
turn on and flag where the app's current design embeds an implicit legal assumption —
the "what to do about it" question is left to a lawyer and to the pilot groups, named at
the end.

Researched August 2026. Where a rule's status was in flux at that date, the flux itself
is reported, since it matters for a live answer more than the historical baseline does.

## Summary

There is no single "contractor test." There are at least four overlapping ones — IRS,
federal DOL/FLSA, and state law (which can be stricter than both) — and a worker can
pass one and fail another for the same set of facts, because each test exists to answer
a different question (who withholds payroll tax, who gets minimum wage/overtime, who
gets state wage-and-hour protection). The sharpest tension for this app: every test,
federal or state, weighs **behavioral control** — who directs how the work gets done —
more heavily than anything else, and in a stricter state test (California's or
Massachusetts's ABC test) a contractor who is *even partly* controlled or *dependent on
one company for the item itself* can fail outright, on that one prong, regardless of
what the parties call the relationship or wrote in a contract. ADR-0008's model gives an
Owner an instant, one-click switch between two labels and encodes a real behavioral-
control difference (ambient reach vs. attachment-gated reach) as the switch's downstream
effect — which is close to the axis every test actually measures, not incidental to it.
Separately, federal enforcement policy for the DOL/FLSA test is itself unsettled as of
this research date (see below), so "current federal rule" is a moving target for anyone
relying on it in 2026.

## 1. The federal control test — IRS common-law rules and Form SS-8

The IRS does not use a single formula. It groups evidence into three categories and asks
whether, taken together, they show the business has the *right* to direct and control
the worker — actual, exercised control is not required, only the retained right to
exercise it.

- **Behavioral control** — facts showing whether the business has a right to direct and
  control *what* work is done and *how* it is done, through instructions, training, or
  other means. The business need not actually direct the work; retaining the right is
  enough.
- **Financial control** — facts showing whether the business controls the financial and
  business aspects of the job: the worker's unreimbursed expenses, the worker's
  investment in her own tools/facilities, whether she offers her services to the broader
  market (not just this one payer), how she is paid, and whether she can realize a
  profit or suffer a loss on the job.
- **Relationship of the parties** — written contracts describing the intended
  relationship, whether the business provides employee-type benefits, the permanency of
  the relationship, and whether the work is a key aspect of the business's regular
  operations.

No single fact is dispositive; the IRS instructs weighing the whole picture.
[IRS: Employee (common-law employee)](https://www.irs.gov/businesses/small-businesses-self-employed/employee-common-law-employee),
[IRS: Independent contractor (self-employed) or employee?](https://www.irs.gov/businesses/small-businesses-self-employed/independent-contractor-self-employed-or-employee),
[IRS Topic no. 762](https://www.irs.gov/taxtopics/tc762).

**Form SS-8** is the mechanism for getting an actual IRS determination: either the
worker or the firm can file it, submitting the facts of the relationship, and the IRS
issues a determination letter classifying the worker as employee or independent
contractor for federal employment-tax and withholding purposes. The determination is
binding on the IRS based on the facts submitted. Filing discloses the submitted facts to
the other party by default (the filer can decline to have it shared, in which case IRS
will not process it as a full determination request in some circumstances).
[IRS: About Form SS-8](https://www.irs.gov/forms-pubs/about-form-ss-8),
[Form SS-8 (PDF)](https://www.irs.gov/pub/irs-pdf/fss8.pdf).

## 2. The federal DOL/FLSA rule — status as of August 2026

This is the part most likely to be stale in anyone's training data, so it is reported in
full with dates.

**The 2024 rule.** On January 10, 2024 the Department of Labor published a final rule
(effective **March 11, 2024**) rescinding the 2021 independent-contractor rule and
replacing it with a **six-factor "economic reality" test**, applied as a
totality-of-the-circumstances analysis with no factor given predetermined weight:

1. Opportunity for profit or loss depending on managerial skill.
2. Investments by the worker and by the potential employer.
3. Degree of permanence of the working relationship.
4. Nature and degree of the potential employer's control over the work.
5. Extent to which the work performed is an integral part of the potential employer's
   business.
6. Skill and initiative the worker brings to the work.

[DOL: Final Rule — Employee or Independent Contractor Classification Under the FLSA
(RIN 1235-AA43)](https://www.dol.gov/agencies/whd/flsa/misclassification/rulemaking),
[DOL news release, Jan. 9, 2024](https://www.dol.gov/newsroom/releases/whd/whd20240109-1).

**Its status is currently split between litigation and enforcement, and both changed
again in 2025–2026:**

- The 2024 rule was never vacated by a court and, as of this research, **remains the
  operative regulatory text and remains in effect for private FLSA litigation** — a
  worker or a plaintiff's lawyer can still argue it in a lawsuit.
- Separately, DOL's **own enforcement arm stopped applying it**. Per Field Assistance
  Bulletin 2025-1, WHD field investigators were instructed, for matters not yet resolved
  as of May 1, 2025, to apply the pre-2024 framework instead — the **2008 DOL Fact Sheet
  #13** and Opinion Letter FLSA2019-6 — not the 2024 six-factor rule.
- On **February 26, 2026**, DOL published a **Notice of Proposed Rulemaking** proposing
  to formally **rescind** the 2024 rule and replace it with a narrower **two-"core-
  factor" test** (control, and opportunity for profit or loss; three secondary factors
  considered only if those two conflict) — closely resembling the 2021 rule the 2024
  rule had itself rescinded. Comment period closes **April 28, 2026**; Federal Register
  citation 91 Fed. Reg. 9932 (Feb. 27, 2026).
  [DOL: NPRM Q&A](https://www.dol.gov/agencies/whd/flsa/misclassification/2026rulemaking/faqs),
  [DOL news release, Feb. 26, 2026](https://www.dol.gov/newsroom/releases/whd/whd20260226).

**Net effect for 2026: there is no single settled federal FLSA test right now.** The
regulatory text on the books is the 2024 six-factor rule; the enforcing agency is using
the older 2008 multi-factor framework instead; and a further rulemaking that would
replace both with a two-factor test is mid-comment-period and not final. A plan built
today on "the current DOL rule says X" needs to name which of these three it means, and
revisit this after the NPRM resolves.

*Direct WebFetch of dol.gov pages returned HTTP 403 during this research (their server
blocks the fetch tool); the content above was retrieved via search-engine indexing of
the same dol.gov URLs, cross-checked against law-firm summaries citing the same
documents (Jackson Lewis, Mayer Brown — see sourcing note).*

## 3. State tests stricter than federal — the ABC test

Some states use a test that is **structurally stricter** than the IRS or DOL tests: a
worker is presumed an **employee**, and the hiring business carries the burden of
proving **all three** of the following are true. Failing even one prong — regardless of
how the other two come out, and regardless of contract language — makes the worker an
employee for that state's purposes. This is qualitatively different from the federal
tests, which balance factors against each other; the ABC test is conjunctive, not
balanced.

**California** (Labor Code §2775, codifying *Dynamex Operations West, Inc. v. Superior
Court* (2018) via AB 5, amended by AB 2257):

- **(A)** The person is free from the control and direction of the hiring entity in
  connection with the performance of the work, both under the contract and in fact.
- **(B)** The person performs work that is outside the usual course of the hiring
  entity's business.
- **(C)** The person is customarily engaged in an independently established trade,
  occupation, or business of the same nature as the work performed.

[DIR: Independent contractors FAQ](https://www.dir.ca.gov/dlse/faq_independentcontractor.htm),
[EDD: Employee or Independent Contractor](https://edd.ca.gov/en/payroll_taxes/employment-status/).
Where a worker's occupation does not qualify for one of California's statutory
exemptions, the older multi-factor *Borello* test applies instead of the ABC test — but
those exemptions are a fixed, named list. Checked directly against the statute text
(Labor Code §2783, referral-agency/professional-services exemptions): the enumerated
exemptions cover categories like licensed healthcare providers, licensed professionals
(lawyers, architects, engineers, accountants), insurance and securities professionals,
direct salespersons, manufactured-housing salespersons, commercial fishers, newspaper
carriers, and a short list of specific service trades. **Doulas, birth workers, or
postpartum support workers do not appear in that list** — meaning if a California
Practice were in scope, the full three-prong ABC test, not the softer *Borello* test,
would presumptively apply to a contractor doula relationship, absent some other
exemption not found in this pass.
[Cal. Labor Code §2783 (leginfo.legislature.ca.gov)](https://leginfo.legislature.ca.gov/faces/codes_displaySection.xhtml?lawCode=LAB&sectionNum=2783.).

**Massachusetts** (M.G.L. c. 149, §148B): the same three-prong structure, same
conjunctive rule — the hiring entity must prove all three or the worker is an employee.

- **Prong 1** — the individual is free from control and direction in connection with the
  performance of the service, both under the contract and in fact.
- **Prong 2** — the service is performed outside the usual course of the business of the
  employer.
- **Prong 3** — the individual is customarily engaged in an independently established
  trade, occupation, profession, or business of the same nature as the service
  performed.

The independent-contractor law **presumes employee status**; the burden to prove
otherwise, on every prong, sits with the employer.
[Mass.gov: Massachusetts law about independent contractors](https://www.mass.gov/info-details/massachusetts-law-about-independent-contractors),
[Attorney General's Advisory 2008/1 on M.G.L. c. 149, §148B (PDF)](https://www.mass.gov/doc/an-advisory-from-the-attorney-generals-fair-labor-division-on-mgl-c-149-s-148b-20081/download).

**Why this is "stricter," precisely:** the federal tests (IRS, DOL) weigh several
factors against each other — no single factor is fatal to contractor status, and the
overall picture can favor "contractor" even with some employee-like facts present. The
ABC test does not weigh; it is a checklist where any single failed prong ends the
analysis in "employee," no matter how strongly the other two prongs point the other way.
A relationship that reads as a contractor relationship under the IRS's "totality of
factors" balancing can still fail an ABC test on one prong alone.

**Pilot jurisdiction.** Neither `CONTEXT.md` nor `docs/personas/README.md` (nor any
persona file checked) names a state, region, or jurisdiction for the pilot Practices.
**The pilot's home state is not determinable from this repo's docs.** This matters
directly: whether an ABC-test state's stricter rule even applies to a given pilot
Practice is unknown until that fact is known, and it is not a fact this research can
supply.

## 4. Doula-specific practice — what's actually documented

**DONA International** (the largest doula certifying body) was searched directly for a
position statement on doula employment classification. **None was found.** Its public
materials describe doulas practicing "in hospitals, in doula agencies, in nonprofits and
as independent business owners" without taking a position on which legal classification
applies to agency-affiliated doulas. This is reported as a negative finding, not a
settled "DONA has no opinion" — only that a targeted search did not surface one.
[DONA International](https://www.dona.org/) (general site, no classification position
paper located).

**Whether a contractor doula invoices the Client or the agency: not clearly settled by
public sources.** Several doula-training-org contract templates were found
(illustrative only — none of these are legal authorities, and each is a commercial
product, not primary law):

- A "Sub-Contractor Agreement" product from ProDoula (a doula training/certification
  organization), described as being for agencies "looking to bring on independent
  contractors." The product's public listing page does not state, in the text available,
  whether the contractor bills the client directly or is paid by the agency.
  [ProDoula: Sub-Contractor Agreement](https://www.prodoula.com/shop/contracts/sub-contractor-agreement/)
  — cited as illustrative of what the doula-training-org market sells, not as evidence
  of a standard practice.
- Multiple independent "Doula Agency Contract" / "Postpartum Doula Agency Independent
  Contractor Agreement" templates are sold on Etsy and by individual doula-business
  consultants (e.g. douladarcy.com), aimed at agencies defining "payment structure and
  administrative expectations" between the agency and its contractor doulas. These
  confirm the *existence* of an agency-pays-contractor pattern as a commercial template
  category, but none of the pages fetched state definitively, in a way suitable to cite
  as settled practice, whether the standard structure is agency-bills-client/
  agency-pays-doula (the shape ADR-0008's model assumes, since no Client-invoicing
  mechanism exists for a doula) versus doula-bills-client directly.

**Honest gap:** this research did not find a citable, authoritative source settling
"does an independent-contractor doula ordinarily invoice the agency or the client."
The commercial contract-template market's existence is weak evidence that
agency-administers-billing is a common pattern (that is what "administrative
expectations" language in template marketing implies), but it is not confirmation, and
none of it is a legal or standards-body source.

## 5. Where this app's model makes an implicit legal claim

ADR-0008 (`docs/adr/0008-employment-type-gates-the-practice-attachment-gates-the-engagement.md`)
already flags worker classification as an open axis it does not resolve
("**Worker classification** — whether the law treats a contractor Doula as one, and who
she invoices — raised on #230, researched on #249. Nothing in this model waits on it").
The three specific design points below are assessed against what the research above
turns on — again, not a verdict on any real classification, only whether the research
suggests a concern worth carrying forward.

**a. The Owner-flippable `employment_type` dropdown — instant reclassification by admin
choice.**
`CONTEXT.md`'s Employment type entry states an Owner may change it and "the change takes
effect at once." Every test surveyed above treats the *actual, ongoing relationship* —
not a label an employer assigns — as the classification. The IRS control test, DOL's
economic-reality test, and both ABC tests all look at what happens in fact, not what a
dropdown says. A person's legal status under any of these tests does not change because
an Owner selects a different value in a UI; if the underlying facts of the relationship
haven't changed, the label was wrong either before or after the flip, not both times
correctly. **Research flags a concern**: the schema treats `employment_type` as a
freely-assignable business attribute, which is a reasonable *data model* (it does need
to store the Practice's current position), but the ADR's own language ("takes effect at
once") reads as if flipping the value changes the legal fact, and no test surveyed here
works that way. This is a modeling-language concern, not necessarily a schema defect —
storing an assignable field is not itself wrong, but nothing in the current design
distinguishes "the Practice's current classification decision" from "the classification
that would actually hold up," and no test above supports treating those as the same
thing.

**b. The ambient-vs-attached control difference — this maps directly onto behavioral
control.**
This is the sharpest finding of this research. Every test surveyed (IRS behavioral
control, all six DOL/FLSA factors including "nature and degree of control," and prong A
of both state ABC tests) treats *the hiring party's actual, retained authority to direct
and reach the work* as central — often the single heaviest-weighted or, for the ABC
test, an outright disqualifying factor if not satisfied. ADR-0008's model is not
adjacent to that axis; it *is* that axis, expressed as data: an `employee` Doula has
ambient read/write reach over every Engagement at the Practice; a `contractor` reaches
only what she is explicitly attached to via an accepted Offer. That the schema enforces
this control boundary in the product (RLS, `Reader`-gated views, `GatedRouter`) is
notable precisely because it means the app's access-control model and the legal
control test are measuring close to the same thing — a contractor with genuinely
attachment-gated reach is *consistent* with what "free from control" (ABC prong A) or
low "behavioral control" (IRS) would look for; a contractor who somehow acquired ambient
reach would be evidence pointing the other way. **Research flags this as directly
relevant, not as a defect** — the design choice happens to track a legally load-bearing
distinction, which is worth knowing explicitly rather than as an accident of the access
model.

**c. The Offer's imposed `amount_cents` + free-text `terms` — a company dictating price
and terms to a "contractor."**
`engagement_offers` lets the Practice set the fee and write free-text terms for a
specific job; the contractor can only accept or decline (ADR-0008 explicitly rejected
"countering an Offer's fee" as a supported mechanism — "a counter is haggling"). Several
tests weigh *how payment is set* as evidence of financial control or independence: the
IRS's financial-control category asks how the worker is paid and whether she can realize
a profit or loss through her own judgment; DOL's factor 1 ("opportunity for profit or
loss depending on managerial skill") and factor 2 ("investments by the worker") look at
whether the worker has room to negotiate or structure her own economics; a genuinely
independent contractor "in business for herself" ordinarily sets or negotiates her own
price, rather than accepting a company-issued, non-negotiable rate card per job.
**Research flags a concern**: a fixed, company-set, non-negotiable fee per job — with
"counter-offering" explicitly designed out of the system — is a fact pattern that reads
more like an employer setting pay than like two businesses negotiating a price, under
the financial-control/opportunity-for-profit-or-loss factors in the tests above. This
does not by itself decide a classification (no single factor is dispositive under IRS or
DOL, and even under the ABC test this touches only how prong A's "free from control...
in fact" gets evaluated, not an automatic fail), but it is a fact the design creates on
purpose, and it points toward "more like an employee's pay" rather than toward "more
like an independent contractor's" on this factor.

## 6. Questions for a lawyer

Short and distinct, not the pilot-facing list below:

1. Given the actual mechanics of ADR-0008's model — ambient vs. attachment-gated reach,
   a Practice-set non-negotiable fee per Offer, an Owner-flippable classification label
   — does the *product's own behavior*, independent of what any real doula's outside
   business looks like, create exposure under the IRS common-law test, the DOL/FLSA
   test (whichever version is operative when this is reviewed), or a state ABC test, if
   a pilot Practice operates in an ABC-test state?
2. Is Form SS-8 ever an appropriate step for a pilot Practice to take proactively
   (getting a binding IRS determination before launch), or is it better reserved for a
   contractor or agency who wants a decision after a dispute arises?
3. Does building software that gives an Owner an instant, one-click reclassification
   switch — with no required review, no effective-date logic beyond "at once," and no
   record of *why* the classification was chosen — create its own legal risk
   independent of any individual classification decision (e.g., as evidence of
   sham-classification practice, if ever examined)?
4. Given the DOL/FLSA rule's unsettled 2025–2026 status (2024 rule on the books but not
   enforced by DOL; NPRM pending as of this research), what test should the product's
   design assumptions target for a January 2027 launch — and does that answer change if
   the NPRM finalizes before then?

## Questions for the pilot groups

**These belong on [issue #243](https://github.com/markgoho/doula-cloud/issues/243)
("Questions for the pilot groups"), which already carries a "Contractor classification"
section with its own question list — not on this research file or on #249.** This
research was not able to read #243's existing question list directly (out of scope for
a primary-source legal research pass), so rather than risk duplicating it, only genuinely
new questions this research specifically surfaces are listed below, for #243 to absorb
or discard as already covered:

1. **What state(s) do the pilot Practices actually operate in?** This research
   confirms the pilot's home state is not named anywhere in this repo's docs, and it is
   the single fact that determines whether an ABC-test state's stricter, conjunctive
   rule applies at all, versus only the federal balancing tests.
2. **When a contractor doula is offered and accepts a job, does she separately invoice
   anyone — the agency, the Client, or neither — today, outside Doula Cloud?** This
   research did not find a settled public answer to whether contractor doulas
   ordinarily bill the agency or the client; the pilot groups are the only source that
   can settle it for this product's real users, and the schema currently has no
   payment-to-doula or doula-invoices-Client mechanism at all (per #230), so the answer
   changes what's missing.
3. **Do any pilot Practices' contractor doulas ever negotiate or counter a job's fee**,
   or is a take-it-or-leave-it offered rate (what ADR-0008's model implements) how it
   already works in practice? This bears on the financial-control factor described in
   §5(c) above and is a fact only the pilot groups can supply.

## Note on sourcing

Primary sources used directly: IRS.gov (common-law employee guidance, Form SS-8 and its
instructions, Topic 762), DOL.gov (2024 final rule page, 2026 NPRM FAQ page — both
retrieved via search-engine indexing after direct WebFetch was blocked with HTTP 403 by
dol.gov's server), California DIR and EDD guidance pages, California Labor Code §2775
and §2783 fetched directly from leginfo.legislature.ca.gov, and Massachusetts's own
Mass.gov page and Attorney General's Fair Labor Division Advisory 2008/1 on M.G.L. c.
149, §148B.

Secondary sources used only to confirm or date-check facts already anchored to a primary
source above, never as the sole basis for a claim: Jackson Lewis and Mayer Brown law-firm
client alerts (used to confirm the 2026 NPRM's exact Federal Register citation, comment-
period end date, and the enforcement-posture detail — DOL field staff using the 2008
Fact Sheet #13 framework — since dol.gov itself could not be fetched directly).

Not used as evidence anywhere in this file: SEO listicles, aggregator "best practices"
roundups, and general blog posts about worker classification. Doula contract-template
marketplace listings (ProDoula, Etsy sellers, individual doula-business consultants) are
cited only in §4, explicitly labeled illustrative/commercial, never as legal or
authoritative sources.

This file does not contain a recommendation section. Per the originating issue, no
classification is recommended for any doula, and no change to ADR-0008's model is
proposed here — §5 reports where the research suggests a concern worth carrying forward,
not what to do about it.
