# How a doula claim reaches New York Medicaid, and what the route costs to run

Research for [#803](https://github.com/markgoho/doula-cloud/issues/803). Gathered 2026-09-05 from eMedNY's own manuals and forms, NY DOH, the federal eCFR, and named vendors' own published pages. Every load-bearing fact carries a URL. Anything I could not settle from a primary source is in "Open questions and quote-only facts" at the end, and anything that conflicts with a figure this map has been quoting is in "Contradictions".

Two conventions used throughout: **Confirmed** means a primary source says it, and the source is cited. **Inference** means I reasoned to it from a cited source and it is not itself stated anywhere I found.

## Sources leaned on most

- eMedNY Doula Services Policy Manual, document header "March 2026", Document Version 6/2/2025, Effective date March 1, 2024 — https://www.emedny.org/ProviderManuals/Doula/PDFS/Doula_Policy_Guidelines.pdf (cited below as **Doula Policy Manual**)
- eMedNY Doula Services Professional Billing Guidelines, Version 2024-01, 11/22/2024 — https://www.emedny.org/ProviderManuals/Doula/PDFS/Doula_Billing_Guidelines.pdf (cited as **Doula Billing Guidelines**)
- eMedNY Doula Fee Schedule (xlsx) — https://www.emedny.org/ProviderManuals/Doula/PDFS/Doula_Fee_Schedule.xlsx
- eMedNY Trading Partner Information Companion Guide, Version 3.1.2, July 7, 2026 — https://www.emedny.org/HIPAA/5010/transactions/eMedNY_Trading_Partner_Information_CG.pdf (cited as **TPI CG**)
- eMedNY Information for All Providers — General Billing — https://www.emedny.org/ProviderManuals/AllProviders/PDFS/Information_for_All_Providers-General_Billing.pdf (cited as **General Billing**)

## (a) May a software vendor submit claims on a provider's behalf?

**Yes — confirmed — but only if the vendor itself enrolls in the New York State Medicaid Program as a service bureau / billing agency, holds its own ETIN and Trading Partner Agreement, and collects a notarized certification from every provider it submits for.**

The eMedNY definition of a trading partner names software vendors explicitly: "An Electronic Data Interchange (EDI) Trading Partner is any entity (provider, billing service, software vendor, employer group, financial institution, etc.) that transmits electronic data to or receives electronic data from another entity" (TPI CG §2.2.2). So the category exists and Doula Cloud fits it.

The gate is enrollment, and it is not waived for third parties. "NYS DOH requires any entity that wishes to exchange electronic data with NYS Medicaid to be enrolled in the NYS Medicaid Program." And, flagged in the guide as an "Important Note": "Clearinghouses, Service Bureaus or any entity that intends to exchange electronic data with NYS Medicaid must be enrolled in the NYS Medicaid Program" (TPI CG §2.2.1). Successful enrollment is required before proceeding with EDI.

There are two ETIN application forms, and eMedNY distinguishes them exactly on the third-party question. The Service Bureau / Billing Agency ETIN Application "is used only by entities who submit transactions on behalf of an enrolled NYS Medicaid provider" (TPI CG §2.2.2). The form itself repeats the rule and adds the prerequisite: "This ETIN Application form is to be used only by persons who are self-employed or employed by a service bureau/billing agency and submit transactions (claims) on behalf of an enrolled NYS Medicaid provider. Service Bureaus/Billing Agencies must be enrolled in the NYS Medicaid Program prior to submitting this ETIN Application." It also draws the employee line: "If the person applying for the ETIN is an employee of the enrolled provider, a Provider ETIN Application should be submitted." Source: https://www.emedny.org/info/ProviderEnrollment/ProviderMaintForms/403101_ETIN_SBaP_ETIN_Service_Bureau_application.pdf

So the full checklist for Doula Cloud sitting in the claims path as the submitter, all confirmed:

1. **Enroll as a service bureau in NYS Medicaid.** eMedNY defines a service bureau as "An entity which submits claims, and/or verifies patient eligibility for providers enrolled in the Medicaid Program." Enrollment runs through the Provider Services Portal and requires a Service Bureau Application Agreement, an ETIN Certification Statement, a Prior Conduct Questionnaire where disclosure questions are answered affirmatively, an IRS assignment letter showing the FEIN and applicant name (a W-9 is not accepted), and a signed regulatory-compliance form 504.9 for service bureaus. Source: https://www.emedny.org/info/ProviderEnrollment/svcbur/index.aspx
2. **Pay the application fee.** eMedNY: "The application fee for 2026 is $750." It is collected under 42 CFR 455.460, is waived where the applicant is already enrolled in Medicare or another state's Medicaid/CHIP, and a hardship waiver may be requested. Source: https://www.emedny.org/info/ProviderEnrollment/ffs.aspx
3. **Obtain a Service Bureau ETIN.** "NYS DOH requires any entity that plans to exchange electronic data with NYS Medicaid obtain an Electronic Transmitter Identification Number (ETIN)... An ETIN is used to identify a submitter." (TPI CG §2.2.2)
4. **Collect a notarized Certification Statement per provider, per ETIN, every year.** "A notarized Certification Statement must be submitted for each enrolled Provider ID and ETIN combination... NYS DOH requires re-certification annually." (TPI CG §2.2.2). eMedNY's ETIN page adds that the signature "must be an original, wet signature", that certifications must be renewed yearly, and that "In order to have access to any electronic transaction with eMedNY, the ETIN has to be up to date and active" — https://www.emedny.org/info/etin/. The statement itself has a field for the billing service name and is signed by the **provider**, who certifies she has reviewed the claims and that everything transmitted is true, accurate and complete, with the six-year record-retention undertaking. So the vendor transmits, but the provider stays the certifying party. Text: https://www.emedny.org/info/ProviderEnrollment/ProviderMaintForms/403101_ETIN_SBaP_ETIN_Service_Bureau_application.pdf
5. **Have a Trading Partner Agreement on file.** "All Trading Partners must have a Trading Partner Agreement (TPA) on file. You may do so only after successful enrollment into the NYS Medicaid Program and upon receiving an ETIN." (TPI CG §2.2.2). Form: https://www.emedny.org/info/ProviderEnrollment/ProviderMaintForms/801101_TRDPRTaGR_Trading_Partner_agreement.pdf
6. **Test in the Provider Testing Environment before going live.** The PTE "is designed to enable NYS Medicaid trading partners to test batch and real-time EDI transactions using the same validation, adjudication logic, and methods as the eMedNY production environment", with ISA15 set to "T"; setting it wrong sends the transaction to production (TPI CG §2.3.1, §2.3.3).

**Who holds the Trading Partner Agreement, then?** Any of the three. A provider can be her own trading partner; a group can; a clearinghouse or service bureau can. They are not exclusive — a provider may be enrolled with an ETIN of her own *and* appear on a service bureau's ETIN via a Certification Statement, and the TPI CG notes that a provider submitting under multiple ETINs receives a separate 835 and a separate check for each (§7.2.1).

**The money never moves through the vendor.** The Doula Policy Manual §13.4 says Medicaid payment "may be made directly to: The individual doula; or, The group which employs the doula; or, The group in which the doula is a member." A service bureau is not on that list. This is the fact that keeps the percentage-pricing question in #547 clean of the factoring prohibition — but not of the business-agent rule, which is next.

**The federal rule on paying a billing agent (relevant to #547's percentage question).** 42 CFR 447.10(f), "Business agents": "Payment may be made to a business agent, such as a billing service or an accounting firm, that furnishes statements and receives payments in the name of the provider, if the agent's compensation for this service is—(1) Related to the cost of processing the billing; (2) Not related on a percentage or other basis to the amount that is billed or collected; and (3) Not dependent upon the collection of the payment." §447.10(h) separately bars payment "to or through a factor". Source: https://www.govinfo.gov/content/pkg/CFR-2023-title42-vol4/xml/CFR-2023-title42-vol4-sec447-10.xml

New York restates the percentage concern at the group level too. Doula Policy Manual §13.3.1: "Federal and State anti-kickback provisions provide for administrative and criminal penalties for improper compensation arrangements. Improper arrangements usually involve compensation paid on a percentage basis."

**Inference, labeled as such:** the safest reading is that a percentage-of-collections charge for claims submission is off the table for Doula Cloud, and a flat fee related to the cost of processing is what 447.10(f) contemplates. Two caveats I did not resolve from a source. First, 447.10(f) is written for an agent that "receives payments in the name of the provider"; a submit-only vendor that never touches the money is arguably outside its literal terms — but the anti-kickback exposure the Doula Policy Manual warns about does not depend on who holds the funds. Second, #547 already settled that claims billing carries no charge of its own, so the question is currently moot; it becomes live only if that changes.

## (b) What each route costs, and whether that cost is per claim or per provider per month

The short answer, and it is the good one for this product: **the cheapest credible routes are free or near-free at fourteen doulas' volume, and the one published cost structure that scales per provider is avoidable.** Nothing here forces a monthly minimum across fourteen enrolled doulas.

**ePACES is free. Confirmed.** "NYS Medicaid provides a HIPAA-compliant direct data entry web-based application that is customized for specific transactions, including the 837I. ePACES, which is provided free of charge, is ideal for providers with small-to-medium claim volumes" (TPI CG §4.3.1). Requirements are an ETIN and Certification Statement, a browser, and an email address — no fee anywhere in the list.

**Eligibility verification (270/271) is free through ePACES. Confirmed.** ePACES carries "270/271 - Eligibility Benefit Inquiry and Response" alongside 276/277, 278 and the 837s, and the application as a whole is "provided free of charge" (TPI CG §4.3.1). eMedNY charges nothing for the transaction itself on any of its own access methods that I found; the cost of 270/271 only appears once a commercial clearinghouse sits in the middle.

**Enrolling as a doula costs nothing.** "There is no application fee for individual doulas to enroll with New York State Medicaid" and "There is no application fee for doula-only groups to enroll with New York State Medicaid" (Doula Policy Manual §18.1, §18.2). The $750 fee at https://www.emedny.org/info/ProviderEnrollment/ffs.aspx attaches to a service bureau enrollment, which is Doula Cloud's cost, not the Practice's.

**Clearinghouse pricing — real published numbers.**

*Claim.MD* publishes a full price list, and it is priced **per tax ID and per transaction, explicitly not per provider**. Verbatim from https://www.claim.md/pricing.html:

| Plan | Monthly | Included | Overage |
| --- | --- | --- | --- |
| Basic | $30.00/month | "Pay only for what you use" | $0.30/claim, $0.30/eligibility |
| Small Volume | $60.00/month | 100 claims, ERA, and eligibility per month | $0.30/claim, $0.50/eligibility |
| Unlimited | $120.00/month | Unlimited claims, unlimited ERA, 1,000 eligibility per month | $0.50/claim beyond |

Setup fee on every plan: "none". Attachments $0.60 each; paper/faxed claims $1.00 each for five pages. One tax ID is included, with tiered pricing for additional tax IDs. The page states no per-provider fee and no NPI limit. Claim.MD lists New York Medicaid (payer ID NYMCD) as a supported payer for Professional/1500 claims, institutional claims, secondary claims, eligibility/benefits and ERA — https://www.claim.md/payer/NYMCD/NY%20Medicaid.html

*Office Ally* publishes both a price page and a contractual Data Sheet (revised 06/01/2026). The Data Sheet is the authoritative text — https://cms.officeally.com/products/service-center-data-sheet:

- "Par Claims (Electronic): There is no cost for submitting Par claims."
- "Non-Par Claims (Electronic): The Non-Par Claim processing fee is $44.95 and is calculated and charged per unique Tax ID + Rendering NPI combination if any Non-Par claims are submitted within the month."
- Eligibility 270/271: "$10.00 for 1-100 transactions and any additional transactions will be $0.10 per transaction within a given month."
- Attachments: "$0.55 per attachment". Printed-and-mailed claims: "$0.75 per claim".
- Office Ally reserves the right to move an account to Enterprise pricing based on transaction volume, "number of Rendering NPIs submitting under a UserID", or invoice size.

The Service Center product itself is listed at "$0 / Start for Free*" with "Transactional fees may apply" on the pricing page — https://cms.officeally.com/products/pricing

**This is the per-provider-per-month shape the ticket was worried about, and whether it bites depends on one flag I could not confirm.** If New York Medicaid is classified Non-Par on Office Ally's 837 claims payer list, then a fourteen-doula group billing under a group NPI with each doula as the rendering provider produces fourteen unique Tax ID + Rendering NPI combinations, and fourteen × $44.95 = **$629.30 per month** — against roughly $333 a month of Credit for the whole agency. If NY Medicaid is Par, the same claims cost **$0**. Office Ally's payer list renders client-side and I could not read the flag from the page source; see Open questions. **Do not build on the $629.30 figure — it is arithmetic on an unconfirmed premise, shown only because the shape matters.**

**Cost structure, characterized as the ticket asks:**

- eMedNY direct (ePACES, eXchange, FTP, FTS/SOAP, CORE Web Services): **no fee at all**, on any axis. Confirmed for ePACES by the "free of charge" language; for the other access methods, no fee appears anywhere in the TPI CG and none is charged at enrollment. The cost is labor and compliance, not dollars.
- Claim.MD: **per account and per transaction**, never per provider. Confirmed from the vendor's own page.
- Office Ally: **free per claim for Par payers; per Tax ID + Rendering NPI per month for Non-Par payers**, i.e. per-provider-per-month in the specific case that matters here. Confirmed from the vendor's own Data Sheet; the Par/Non-Par classification for NYMCD on the claims list is not confirmed.
- Waystar, Availity Essentials Pro, Optum/Change Healthcare, TriZetto, Inovalon: **quote-only**. None of them publishes a price for 837P to a state Medicaid program. To get a number, someone has to ask their sales teams directly. I did not estimate one.
- Outside billing service (a human billing company): **quote-only, and typically a percentage of collections** — which is exactly the arrangement 42 CFR 447.10(f) constrains. No named vendor publishes a rate for New York doula claims. The party to ask is a New York medical billing company, or the pilot agency owner about what she pays today (#802).
- Service bureau enrollment for Doula Cloud: **$750 one-time**, plus annual re-certification paperwork per provider.

**The conclusion the ticket was reaching for:** there is no route where fourteen enrolled doulas force a per-head monthly floor that the $20.00 Credit cannot absorb, *provided* the route is eMedNY-direct or Claim.MD rather than Office Ally against a Non-Par NY Medicaid. Claim.MD's Unlimited plan at $120/month, flat, covers the whole agency's claims and 1,000 eligibility checks with no per-provider component. eMedNY-direct costs nothing at all. **Inference:** the cost question does not, by itself, force Doula Cloud out of the claims path.

## (c) Is a doula enrolled individually, or can an agency enroll as a group?

**Both, and they are not alternatives — the group route requires the individual enrollment underneath it. Confirmed, and stated as a rule rather than inferred from other provider types.**

Doula Policy Manual §4: "An individual doula or a doula-only group can enroll in the New York State Medicaid program as a doula services provider and bill Medicaid directly for doula services. A Medicaid-enrolled doula provider can also affiliate with a multi-professional group. Doula services may be provided through a Medicaid-enrolled individual provider, a doula-only group or a multi-professional group. **The individual doula in a doula-only group or multi-professional group must be enrolled as a Medicaid provider.** The New York State Medicaid-enrolled doula does not require supervision."

§13.3.1 says it again without ambiguity: "All doulas must be enrolled in Medicaid as a fee-for-service/billing provider to bill Medicaid for their services, **regardless of whether payment is made to the individual doula, the doula group or the multi-professional group.**"

So for a fourteen-doula agency: fourteen individual enrollments plus one group enrollment. Fifteen enrollment records, not one.

**What each requires.**

*Individual enrollment* (Doula Policy Manual §6.2, §18.1): an individual (Type 1) NPI, obtained before applying; age 18 or over; current Adult and Infant CPR certification; current doula-specific liability insurance; completion of the electronic New York State Medicaid Fee-for-Service Doula Directory form; familiarity with HIPAA; and qualification under either the Training Pathway (24 hours of training in the listed competencies — 20 core, 4 broader — plus a training certificate and an attestation covering the training and support at a minimum of three births) or the Work Experience Pathway (attestation of support at 30 births or 1,000 hours within the last ten years, an attestation to prenatal/labor/postpartum skills, and three client or professional recommendations on DOH template forms). No application fee. Enrollment must be revalidated every five years, with six hours of continuing education, maintained CPR, and maintained liability coverage (§6.3, §18.4). Instructions: https://www.emedny.org/info/ProviderEnrollment/doula/

*Group enrollment* (Doula Policy Manual §18.2): a **group** NPI obtained before applying, and enrollment as either a doula-only group (https://www.emedny.org/info/ProviderEnrollment/doulagroup/) or a multi-professional group (https://www.emedny.org/info/ProviderEnrollment/practGroups/). No application fee for doula-only groups. A doula-only group contains only enrolled perinatal doulas; if a licensed professional affiliates, the group converts automatically to a multi-professional group, and vice versa (§7).

*What the group owes on an ongoing basis* (§7): it must "immediately notify the Medicaid Program" of the addition or deletion of group members, of any change in ownership including agents, managing employees or employees with a controlling interest, and of any change or addition of address or service location. A departing doula must also notify the program herself; if she does not, "the individual's liability for group activity will continue."

*Who bills* (§13.1): for an individual doula, "the enrolled doula is the billing provider." For a group, "the Medicaid-enrolled doula-only group or multi-professional group may serve as the billing provider when the doula providing the services is enrolled in Medicaid and affiliated with that group", and the claim must identify both "the Medicaid provider number of the doula who provided the services" and "the Medicaid provider number of the group". "In this case, payment will be made to the group provider number. Use of any other provider number is prohibited."

*Liability under a group number* (§13.3.2): "Any individual doula in a group, or their designated agent (including billing agents), may certify a Medicaid claim for payment where the group number is used on the Medicaid claim." When a group number is used, "All members in the group are liable for overpayments" and all members are subject to administrative sanctions. Note the phrase "including billing agents" — it is the manual's own acknowledgment that a third party may sit in the certification path for a group claim.

*The gap worth knowing about:* §13.3.1 says "Members of the group will either be owners, members or managing employees", and §13.4 lists payment to "the group which employs the doula". A 1099 contractor doula fits none of those words cleanly. See Open questions.

## (d) The route space into eMedNY

Four routes, evenly surveyed. All of them ultimately require the same two things — a Medicaid enrollment and an ETIN with a current notarized Certification Statement — because "In order to have access to any electronic transaction with eMedNY, the ETIN has to be up to date and active" (https://www.emedny.org/info/etin/). What differs is who does the transmitting and who bears the technical burden.

**1. ePACES — the free web portal.** Direct data entry in the browser. "NYS Medicaid provides a HIPAA-compliant direct data entry web-based application... ePACES, which is provided free of charge, is ideal for providers with small-to-medium claim volumes." Requirements: an ETIN and Certification Statement, a browser supporting 128-bit encryption and cookies, a 56K-or-better connection, and an email address. It carries 270/271, 276/277, 278, and 837 Dental, Professional and Institutional, "also features real time claim submission of the 837 Professional transaction", and returns adjudication information shortly after submission (TPI CG §4.3.1). Note for testing: ePACES is not supported in the Provider Testing Environment (TPI CG §2.3.3 access exceptions). **Open to:** a solo doula, trivially. **Open to a fourteen-doula agency:** yes, but every claim is typed by hand by a person, which is the labor cost the product would be replacing.

**2. eMedNY eXchange — the "build the file, hand it over" route.** "eMedNY eXchange is a web-based access method used to exchange transaction files and works similarly to a typical ftp interface. Users are assigned a directory and can upload and download transaction files." It is reachable only through the eMedNY.org website, and it "is accessed using the login and password established during the ePACES enrollment process. At least one login attempt into ePACES must be successful before eXchange may be accessed" (TPI CG §4.3.2). **This is the route the ticket body sketches** — Doula Cloud builds a valid 837P file and the Practice uploads it under her own ETIN, so the vendor holds no eMedNY relationship, needs no service bureau enrollment, pays no $750, and signs no TPA. The 835 comes back to the same directory (§7.2.1: the 835 "is available via the eMedNY eXchange, FTP, or SOAP (batch)"). **Open to:** both. The friction is one manual upload and one manual download per cycle.

**3. Direct EDI as a trading partner — automated file exchange.** Three machine-facing options, all requiring an ETIN and Certification Statement first.
   - *FTP*: "File Transfer Protocol (FTP) is the standard process for batch authorization transmissions... FTP is strictly a dial-up connection." Access via Security Packet B (TPI CG §4.3.3). The dial-up characterization makes this the least attractive path.
   - *eMedNY File Transfer Service using SOAP*: "allows trading partners to submit files via the internet using Service Oriented Architecture (SOA) with the Simple Object Access Protocol (SOAP). It is most suitable for users who prefer to develop an automated, systemic approach to file submission." Enrollment produces an eMedNY SOAP Certificate and a SOAP Administrator; minimum requirements are an ETIN and Certification Statement obtained beforehand, plus either being a Primary ePACES Administrator or having existing FTP access (TPI CG §4.3.4).
   - *CORE Web Services*: HTTPS with either HTTP MIME Multipart or SOAP + WSDL, for real-time and batch 270/271 and 276/277, and it "can also be used to retrieve certain eMedNY generated files. These include the x12 remittance advice (835/820)" (TPI CG §4.3.5).
   
   **Open to:** realistically, not a solo doula on her own — this is software work, an annual notarized form, and a testing cycle. It is open to a fourteen-doula agency only if someone builds and operates it, which in this product means Doula Cloud enrolling as a service bureau per (a). This is the route that puts Doula Cloud in the claims path.

**4. A clearinghouse.** The clearinghouse is itself a trading partner and must be enrolled in NYS Medicaid (TPI CG §2.2.1). The provider signs up with the vendor, completes the vendor's payer enrollment for NY Medicaid, and the vendor handles the ETIN-side mechanics and the X12 envelope. Named vendors carrying NY Medicaid professional claims with published prices: Claim.MD (payer ID NYMCD, professional claims, secondary claims, eligibility and ERA — https://www.claim.md/payer/NYMCD/NY%20Medicaid.html) and Office Ally (payer ID NYMCD). **Open to:** both a solo doula and an agency; this is the ordinary path for a small practice and the one that requires the least of the submitter technically. It is also the one that introduces a second PHI processor and therefore a second BAA — see (k).

**5. An outside billing service.** A human billing company takes the encounter data and does everything above on the provider's behalf, under its own Service Bureau ETIN with the doula's notarized Certification Statement on file (this is precisely the case the Service Bureau ETIN Application form describes). **Open to:** both. Cost is quote-only, and if it is a percentage of collections the 42 CFR 447.10(f) constraint in (a) applies to it directly.

**Which route for whom, as a judgment (inference):** a solo doula's realistic options are ePACES or a clearinghouse. A fourteen-doula agency's realistic options are a clearinghouse, an outside billing service, or eXchange upload of files Doula Cloud generates. Direct EDI only makes sense if the vendor operates it for many Practices and amortizes the service bureau enrollment across them.

## (e) Medicaid Managed Care

**The 1 April 2025 date is confirmed, and the consequence is bigger than the ticket assumes: since that date eMedNY fee-for-service is the minority path, and each MMC plan is its own route with its own contract and its own billing instructions.**

Doula Policy Manual §4: "Doula providers are required to bill for doula services to FFS through eMedNY through March 31, 2025. Effective April 1, 2025, doulas services are added to the MMC benefit package. Doula providers are required to bill the individual's MMC plan for covered doula services on and after April 1, 2025, and will continue to bill FFS only when the Medicaid member is enrolled in FFS." §15 repeats it: "Effective April 1, 2025, covered doula services are added to the MMC benefit package, and are reimbursable by MMC plans."

The carve-out period was 1 March 2024 through 31 March 2025, during which "All covered doula services are to be billed to FFS, even for members who are enrolled in a managed care plan" (§15).

**What that means for a route design.** From §15 and §18.3, all confirmed:

- For a member who was **already receiving services before 1 April 2025**, the plan must cover the services and continue the FFS equivalent until 12 months after the pregnancy ends, "even if the doula, doula-only group, or multi-professional group is not contracted with the MMC plan as of April 1, 2025", and "The plan will reimburse no less than the FFS equivalent". Continuity of care, not an ongoing rule.
- For a member who was **not** receiving services before that date, the plan reimburses only if all three hold: the doula is enrolled as an FFS provider, the doula or group "has contracted with the individual MMC plan in which the MMC member is enrolled", and the doula or group is billing that plan.
- "For services provided to Medicaid members enrolled in MMC, providers must contact the member's MMC plan for billing instructions that apply on and after April 1, 2025" (§18.3). Plan contact information is in the eMedNY Information for All Providers — Managed Care Information document: https://www.emedny.org/ProviderManuals/AllProviders/PDFS/Information_for_All_Providers_Managed_Care_Information.pdf

**Inference, and it is the load-bearing one in this section:** the route is not "eMedNY". It is "eMedNY, plus one contracted relationship per MMC plan the Practice's clients are enrolled in." Each plan sets its own billing instructions and its own negotiated rate — the FFS fee schedule binds a plan only for the pre-April-2025 continuity-of-care cohort. The software therefore needs to know, per client, which payer the claim goes to, and needs to hold plan-specific payer identifiers and rates rather than a single fee schedule. The clearinghouse route becomes more attractive here than it looks from the FFS-only view, because a clearinghouse is one connection to many plans. **MMC rates are quote-only, per plan, per contract.**

Also confirmed and relevant to eligibility checking: FFS-vs-MMC enrollment is exactly what a 270/271 eligibility inquiry answers, so the eligibility transaction is not optional nicety here — it is what tells the software where the claim goes.

## (f) The fee schedule

**Confirmed, with one thing the map has been missing: there is an NYC differential, and it is material.**

From the eMedNY Doula Fee Schedule (https://www.emedny.org/ProviderManuals/Doula/PDFS/Doula_Fee_Schedule.xlsx), verbatim:

| HCPCS | Diagnosis | Service | Per pregnancy allowance | Reimbursement rate |
| --- | --- | --- | --- | --- |
| T1032 | Z32.2 (prenatal/pregnancy) or Z32.3 (postpartum) | Perinatal service: prenatal or postpartum doula support (minimum of 30 minutes) | Up to and including 8 times | NYC: $93.75 per visit; Rest of State: $84.37 per visit |
| T1033 | Z32.2 | Labor and delivery: in-person doula support during labor and birth (no time minimum, must be present for the birth) | Up to and including 1 time | NYC: $750.00; Rest of State: $675.00 |

So per pregnancy at the maximum: **Rest of State $1,349.96** (8 × $84.37 = $674.96, plus $675.00) and **NYC $1,500.00** (8 × $93.75 = $750.00, plus $750.00). The map's "$675 once, up to eight at $84.37, roughly $1,350 per pregnancy for Rest-of-State" is correct to the dollar. The map does not appear to carry the NYC figures; it should, and the software must treat the rate as location-dependent rather than a constant.

**Effective date confirmed:** "The FFS doula services fee schedule is effective March 1, 2024" (Doula Policy Manual §13.6), and the manual's own Document Control Properties give an Effective date of March 1, 2024. **No rate change since:** the current fee schedule file on eMedNY's Doula provider manual page carries these amounts as of 2026-09-05, and the manual's most recent revision (header "March 2026", Document Version 6/2/2025) still points to the 1 March 2024 effective date. I found no Medicaid Update announcing a doula rate change.

**The nine-claims-per-pregnancy shape is confirmed:** "The Medicaid-enrolled doula services provider may be reimbursed for up to eight perinatal visits and one labor and delivery encounter per pregnancy" (§13.1). Additional constraints that shape the claim ledger, all from §13.2: each perinatal visit must be at least 30 minutes of direct interaction; perinatal visits may be in person or via telehealth; the labor and delivery encounter must be in person except in extenuating circumstances such as illness, emergency or precipitous birth; "A licensed perinatal services provider must be in attendance for the doula to be reimbursed"; no reimbursement for visits that are not kept; and multiple visits are not allowed on the same day except a perinatal visit and a labor-and-delivery encounter in either order.

Also confirmed and worth carrying: unused perinatal visits do **not** carry over to a subsequent pregnancy within 12 months, and "Documentation will be required to support reimbursement for support for an additional pregnancy within the 12 months following a prior pregnancy" (§4). Coverage runs during pregnancy and up to 12 months after the pregnancy ends, regardless of outcome.

Separately billable, and not currently on the map: **language interpretation, T1013**, billed by the doula's billing provider and paid to a third-party vendor. "One Unit: Includes a minimum of eight up to 22 minutes of medical language interpreter services. Two Units: Includes 23 or more minutes." The doula cannot bill for interpretation she provides herself, and the need must be documented in the record (Doula Policy Manual §16, §17). No rate is published on the doula fee schedule for T1013 — quote-only.

## (g) What an 837P doula claim must carry

Doula Billing Guidelines §2.1: "Doula Services providers who choose to submit their Medicaid claims electronically are required to use the HIPAA 837 Professional (837P) transaction." Paper is the eMedNY-150003 form. The billing guidelines are written against the paper form but map every field to an 837P loop and segment, and say so explicitly: the instructions "are also intended as a guideline for electronic billers to find out what information they need to provide in their claims" (§2.3).

**What the software must hold, mapped to loops. Confirmed from the Doula Billing Guidelines field instructions.**

Member (subscriber) identity — Loop 2010BA:
- Member first and last name — NM1 (from the Common Benefit ID Card).
- Date of birth — DMG02, format MMDDYYYY.
- Sex — DMG03.
- Medicaid Member ID — NM109. Eight characters in the format AANNNNNA (alpha, alpha, five numerics, alpha). This is a validatable format and the software should validate it.

Claim header — Loop 2300:
- Patient account number — CLM01, up to 20 alphanumeric characters, "will be returned on the Remittance Advice". This is the software's own claim correlation key; hold it deliberately.
- Place of service — CLM05-1, a two-digit code (the guidelines give 12, patient's home, as the example). Doula services may be provided "in the hospital, clinic, or community settings" (Policy Manual §13.4), so this varies per encounter.
- Related-causes indicators — CLM11, for worker's compensation, crime victim, auto no-fault, other liability. Usually blank for doula work.
- Certification / provider signature — CLM06.
- Diagnosis — HI, carrying ICD-10-CM Z32.2 (prenatal/pregnancy) or Z32.3 (postpartum). The guidelines: "Include ICD-10-CM diagnosis code Z32.2 or Z32.3 as appropriate."

Service line — Loop 2400, one per encounter:
- Date of service — DTP03 where DTP01 = 472. "A service date must be entered for each procedure code listed."
- Procedure code — SV101-2 (T1032 or T1033; T1013 for interpretation).
- Modifiers — SV101-3 through -7, up to four, if the procedure code requires them. No modifiers are published on the current doula fee schedule.
- Diagnosis pointer — SV107.
- Units — SV104. "Enter '1' as the number of units." A per-line unit of 1 per encounter, not a minutes count.
- Charge amount — SV102. "This field must contain the Amount Charged... may not exceed the provider's customary charge for the procedure" and "must never be left blank or contain zeroes." So the software needs a customary-charge value per code, distinct from the Medicaid fee.
- Emergency indicator — SV109.

Billing and rendering provider — Loops 2010AA and 2310B:
- Billing provider NPI — the guidelines give "Loop 2010AA NM109 OR Loop 2310B NM109" for "the provider's 10-digit National Provider Identifier (NPI)" (Field 25A).
- Billing provider name and correspondence address, five-digit ZIP or ZIP+4 — 2010AA NM1, N3, N4.
- **Inference (labeled):** for a group claim, the Policy Manual §13.4 requires both "the group Medicaid provider identification number (PID)... and the Medicaid PID of the individual doula who provided the service", plus "the place of actual service... if the group is enrolled with multiple service locations". In 837P terms that is the group NPI in 2010AA (billing provider) and the individual doula's NPI in 2310B (rendering provider). The Billing Guidelines do not say this — they say "leave blank" for the group field, because they are written for a solo doula filling in a paper form. See Contradictions.

Coordination of benefits — the Payment Source Code (Field 23B) has no 837P mapping in the guidelines ("No Map"), but the underlying data must be held because Medicaid is payer of last resort: whether the member has Medicare Part B and whether Medicare approved or denied, whether the member has other insurance and what it paid, and any member participation (spend-down) amount. "It is the responsibility of the provider to determine whether Medicare covers the service being billed for... the provider must first submit a claim to Medicare, as Medicaid is always the payer of last resort." Proof of another carrier's denial "must be maintained in the member's billing record."

Timeliness — a delay reason code where the claim is over 90 days old, and the date signed. See (i).

**Left blank for doula claims. Confirmed, and this is the useful part for a PHI boundary.** The billing guidelines instruct "leave this field blank" for: referring physician name, address and identification number (Loop 2310A NM1 / NM109); facility name and address (2310C); service provider name and ID (2310B, in the solo case); sterilization/abortion code; possible disability; EPSDT; prior approval number (2300 REF where REF01 = G1); service authorization exception code; NDC and NDC unit/quantity/cost (Loop 2410 LIN03, SV103, SV104, SV102); locator code; and the consecutive billing section.

**The consequence for the PHI boundary, an inference worth carrying forward:** the recommending practitioner never appears on the claim. The standing order or individual recommendation required by (h) is documentation retained by the doula, not a claim field. So the claim payload is narrower than the clinical record — member demographics and ID, dates, place of service, two procedure codes, one of two diagnosis codes, a unit count of 1, a charge, and provider identifiers. There is no free text, no clinical narrative, and no attachment in the ordinary doula case.

## (h) What New York requires before a doula claim will pay

**Four things: a recommendation (now satisfied by a statewide standing order), active enrollment, a licensed practitioner in attendance for the birth encounter, and documentation kept on file. All confirmed.**

**1. A recommendation from a health practitioner — but a standing order now covers it.** Doula Policy Manual §12.1: "Doula services are a preventative health service, and as such, must be recommended by a physician or other licensed practitioner of the healing arts acting within their scope of practice under State law to be eligible for Medicaid reimbursement."

Before 10 June 2024 that meant an individual written recommendation, which "the doula must obtain a written record of" and "maintain... in their documentation records for the Medicaid member in compliance with HIPAA standards", containing the member's first and last name, the member's date of birth, the practitioner's first and last name, the practitioner's license number, the date of the recommendation, and the practitioner's signature (§12.2). The DOH form is Appendix A of the manual, but its use is not required as long as those six items are present.

On and after 10 June 2024 (§12.3): "A standing order for doula services has been issued as of June 10, 2024 by the New York State Commissioner of Health. This standing order fulfills the federal requirements in section 440.130(c) of title 42 of the Code of Federal Regulations... As such, Medicaid members do not need an individualized recommendation from a healthcare provider for doula services to be covered." Standing order: https://www.health.ny.gov/health_care/medicaid/program/doula/docs/2024-06_doula_standing_order.pdf — the doula "may continue to obtain a written record" of an individual recommendation *or* rely on the standing order.

**2. Active enrollment.** "Doula services will only be reimbursed when provided by doulas that have enrolled as New York State Medicaid providers" (§15). Enrollment must be current — revalidation every five years, and providers "risk termination from the New York State Medicaid program if they do not revalidate timely" (§18.4).

**3. A licensed perinatal services provider in attendance at the birth** for T1033 to be reimbursable (§13.2.2), and the doula must be present for the birth (fee schedule: "must be present for the birth").

**4. Documentation retained.** §12.4: "Services must be documented in the record maintained by the doula services provider for the Medicaid member. The Department conducts audits of persons who submit claims for payment under the Medicaid Program, and the Department may seek recovery or restitution if payments were improperly claimed, regardless of whether unacceptable practices have occurred." The record must include "Date, time, and duration/time of service provided to Medicaid members" and "Information on the nature of the service provided and that supports the length of time spent with the individual on the date of service." The Certification Statement adds the retention period: records "will be kept for a period of six years from the date of payment", furnished on request to the local Department of Social Services, the State Department of Health, OMIG, the Medicaid Fraud Control Unit, or HHS.

**What that adds up to for the software:** the duration of every perinatal visit must be recorded and must support the 30-minute minimum; the recommendation or the reliance on the standing order must be recorded per client; and everything must survive six years past payment. That six-year floor is a retention requirement the product has to design to, not a preference.

## (i) The failure path: denial, resubmission, appeal

**How a denial comes back. Confirmed.** The remittance advice is the channel, in 835, PDF or paper form. It contains "A listing of all claims... that have entered the computerized processing system during the corresponding cycle", "The status of each claim (denied, paid or pended) after processing", "The eMedNY edits (errors) that resulted in a claim denied or pended", subtotals and grand totals, and "Other pertinent financial information such as recoupment, negative balances, etc." (Doula Billing Guidelines §3; TPI CG §7.2).

Mechanics of the 835, from TPI CG §7.2.1:
- "The electronic HIPAA 835 transaction (Remittance Advice) is available via the eMedNY eXchange, FTP, or SOAP (batch)." Requesting it requires the Electronic Remittance Request Form from https://www.emedny.org/info/ProviderEnrollment/allforms.aspx
- Remittances are produced weekly: eMedNY generates them "after the completion of the Financial Cycle over the weekend. Electronic Remittance Advices are available to each submitter on Monday."
- "Providers who submit claims under multiple ETINs will receive a separate 835 for each ETIN and a separate check or EFT for each 835."
- An 835 holds at most 10,000 claim lines; overflow generates a separate 835 and a separate check.
- **Pended claims do not appear in the 835.** "Pending claims do not appear in the 835 transaction; they are listed in the Pended Claims Report file, which will be sent along with the 835 transaction for any processing cycle that produces pended claims." So a claim that vanishes from the 835 is not necessarily lost — the software must read the Pended Claims Report too.
- A Default ETIN must be designated to receive adjudication information for Medicare crossover claims, paper claims, and State-submitted adjustments and voids; only one Default ETIN is allowed per provider.

**Denial codes come in two vocabularies.** eMedNY's own numeric **edit** codes appear on paper and PDF remittances; standard X12 **Claim Adjustment Reason Codes (CARC)** appear on the 835. General Billing gives the worked example for a stale claim: "On Paper Remits: Edit 01292; Date of Service Two Years Prior to Date Received. On the 835 Electronic Remittance: Claim Adjustment Reason Code (CARC) 29; The time limit for filing has expired." eMedNY publishes a searchable edit knowledge base, linked from the Doula Policy Manual §1 as "Search Tool for Denied Claims: eMedNYHIPAASupport - EEKB Search Tool". **Inference:** the software needs to map both vocabularies, because which one it sees depends on the remittance format chosen.

Before adjudication there are two earlier rejection surfaces, from TPI CG §4 and §8: file-level validation (X12 and HIPAA syntax), answered by a **999** functional acknowledgment, and a claim-level acknowledgment, the **277CA**. A claim can therefore fail at three points — 999, 277CA, or 835 — and the software has to reconcile all three against the same claim.

**Resubmission. Confirmed, and the deadlines are tight.** From General Billing:
- Original submission: within **90 days** of the date of service. "All such claims submitted after 90 days must be submitted with a HIPAA delay reason code."
- Correction after a rejection or denial: "it must be corrected and resubmitted within **60 days** of the date of notification to the provider. In addition, paid claims requiring correction or resubmission must be submitted as adjustments to the paid claim within 60 days of the date of notification. In most cases adjustments, rather than voids, must be billed to correct a paid claim."
- The hard stop: "Claims not correctly resubmitted within 60 days, or those continuing to not be payable after the second resubmission, are neither valid nor enforceable." **Two corrective attempts, then the claim is dead.** This is the single most consequential operational fact in this section for a product that wants to keep a Practice out of trouble.
- Outer bound: "All claims must be finally submitted to the fiscal agent and be payable within two years" of the date of service.
- Delay reason codes are numeric and must be included with any claim aged over 90 days or any adjustment made over 60 days out. Delay reason **9** is the one for the ordinary case: "Original Claim Rejected or Denied Due to a Reason Unrelated to the Billing Limitation... valid for resubmitted claims when the original claim was not denied or rejected for any timeliness edits. The corrected claim must be submitted within 60 days of the date of notification. This delay reason is invalid for adjustments." Delay reason **11** covers a paid claim needing correction through adjustment or void for a reason not otherwise listed. Delay reason **10** covers a retroactive authorization granted after administrative appeals, fair hearings or litigation.
- Voiding and resubmitting resets the clock the wrong way: "When a provider voids a previously paid claim and now wishes to resubmit, the resubmission is treated as a new claim and will be subjected to the criteria above for the submission of claim(s) over two years old."

**Appeal, mechanically.** For a claim denied on age, the route is a written Two Year Claim Review request submitted directly to the Department, with supporting documentation — "remittance statements, notice of eligibility, fair hearing decision, court order" — and "documenting the edit 01292 denial must accompany your written request" (General Billing). For a substantive denial, there is no claim-level appeal tribunal in the manuals I read: the mechanism is correction and resubmission within the 60-day window. Fair hearings, referenced in delay reasons 8 and 10, are the **member's** remedy on an eligibility or coverage determination, not the provider's on a claim. Provider-side disputes over enrollment status or sanctions run through OMIG and 18 NYCRR Part 515 (Doula Policy Manual §13.5). **Inference:** a doula's practical recourse on a denial is fix-and-resubmit, twice, inside 60 days — so the product's job is to catch the error early rather than to model an appeal.

## (j) Balance billing a Medicaid member

**Confirmed against a New York source, in New York's own words.**

Doula Policy Manual §13.2.3: "Medicaid providers are not allowed to balance bill Medicaid members; reimbursement received through Medicaid is considered payment in full for services rendered. By enrolling in the Medicaid program, a provider agrees to accept payment under the Medicaid program as payment in full for services rendered."

Three more constraints in the same section, all confirmed:
- No upgrade-and-charge-the-difference arrangement: "A provider may not make a private pay agreement with a beneficiary to accept a Medicaid fee for a particular covered service and then provide a different upgraded service... and agree to charge the beneficiary only the difference in fee between two services, in addition to billing Medicaid for the covered service."
- "It is an unacceptable practice to knowingly demand or collect any reimbursement in addition to claims made under the Medicaid program, except where permitted by law."
- "Transportation costs associated with providing Medicaid covered doula services may not be charged to the Medicaid member."

The Certification Statement the provider signs annually says the same thing from the other side: "payment of fees made in accordance with established schedules is accepted as payment in full."

**The denied-claim case specifically.** The manual states the rule as a general bar tied to enrollment and to payment in full; it does not use the words "denied claim". **Inference, labeled:** a denied covered service does not create a member liability, because the bar is on collecting "any reimbursement in addition to claims made under the Medicaid program" and on the enrollment agreement itself, neither of which is conditioned on the claim having been paid. The provider's recourse on a denial is the 60-day resubmission path in (i), not the member. I did not find a New York source that says this in terms, and if the product is going to hard-block collecting from a client after a denial — which it should — the block should be written against the general bar, which is unambiguous.

Note the boundary: services that are **not** covered doula services (the §9 exclusion list — group classes, childcare, placenta encapsulation, photography, birthing ceremonies, and so on) are outside the Medicaid program entirely and are not what this rule addresses. A private-pay arrangement for a genuinely non-covered service is a different question, and §13.2.3's upgrade prohibition constrains it.

## (k) Services outside the Google Cloud BAA

**One category, and it is unavoidable if the route is a clearinghouse: the clearinghouse itself.** A clearinghouse creates, receives, maintains and transmits PHI on the Practice's behalf, which is a business associate relationship on its face.

**Office Ally publicly publishes a BAA and executes it with every user, at any scale.** From https://cms.officeally.com/baa: "This Business Associate Agreement ('Agreement') by and between you (hereinafter known as 'Covered Entity') and Office Ally, Inc., a Covered Entity (a Health Care Clearinghouse) under HIPAA, providing Business Associate services (hereinafter known as 'Business Associate'), is effective as of the date on which Covered Entity acknowledges and agrees to the Business Associate's User Agreement through a separate form or online enrollment process ('Effective Date')." It binds on enrollment, with no negotiation and no scale threshold, and it uses the HIPAA Rules definitions verbatim. Office Ally also states in the Service Center Data Sheet that it will "manage security controls using industry standards and HIPAA best practices". This is the strongest available answer to "is a BAA obtainable at a small vendor's scale" — yes, at Office Ally, by clicking through enrollment.

**Claim.MD: quote-only.** Claim.MD publishes a licence agreement at https://www.claim.md/hipaa.html which returned 403 to automated fetching, so I could not read it and will not characterize it. Claim.MD does publish that it maintains HITRUST CSF r2 certification (first certified 2023, maintained as of June 2026) — https://www.claim.md/news/claimmd-maintains-hitrust-r2-certification-and-strongly-encourages-vendors-to-adopt-similar-security-practices-to-strengthen-protection-of-sensitive-claims-data — which is evidence of a security program, not a BAA. Whether Claim.MD will sign a BAA, and on what terms, has to be asked of Claim.MD directly.

**Other named vendors:** Waystar, Availity, Optum/Change Healthcare and TriZetto all operate as HIPAA business associates or clearinghouses as a matter of course, but none of them publishes a standing BAA text I could fetch. Quote-only.

**Not a new BAA:** eMedNY itself. New York State DOH is the payer — a covered entity in its own right, not the Practice's business associate — and the instrument governing the relationship is the Trading Partner Agreement, not a BAA. The TPI CG treats the exchange as one between covered entities and requires trading partners to protect PHI in testing ("Testers are responsible for the preservation, privacy, and security of data in their possession", §2.3). **Inference:** no BAA is needed with eMedNY, but the TPA is a distinct instrument the product would have to execute if it takes the direct-EDI route in (a).

**Also outside the Google Cloud BAA, and worth naming now:** a notary is required annually for every Certification Statement, with an "original, wet signature" (https://www.emedny.org/info/etin/). If the product ever wants to automate that, remote online notarization is a third-party vendor touching provider identity documents. And if the outside-billing-service route is chosen, that billing company is a business associate too.

**What stays inside the existing boundary:** if the route is "generate an 837P file and hand it to the Practice for eXchange upload", no new BAA is needed at all. The file is built in Google Cloud and handed to the covered entity herself. That is the compliance argument for the eXchange route, and it is a real one.

## Open questions and quote-only facts

Everything I could not settle from a primary source, with who would have to be asked.

1. **Is New York Medicaid (NYMCD) Par or Non-Par on Office Ally's 837 claims payer list?** This decides whether Office Ally is free or costs $44.95 per month per Tax ID + Rendering NPI for this workload — the difference between $0 and roughly $629 a month for fourteen doulas. Office Ally's payer list renders client-side and I could not read the flag from page source; a general search suggested Non-Par on the 270/271 and 276/277 lists, but I am not carrying an aggregated snippet as a finding. **Ask:** Office Ally sales or support, or read the flag from https://cms.officeally.com/all-payers-list-2-0-claims-837 in a real browser. This is the single highest-value unanswered question in this document.
2. **Under a service bureau's ETIN, does each of fourteen rendering doulas need her own notarized Certification Statement, or does the group's statement suffice?** The TPI CG says a statement is required "for each enrolled Provider ID and ETIN combination", and eMedNY's ETIN page says "The billing provider's NPI, the group NPI, and the rendering provider's NPI" must all be linked to the ETIN — which reads as fifteen notarized statements a year, but does not say so. **Ask:** eMedNY Provider Enrollment, (800) 343-9000.
3. **Can a 1099 contractor doula be a member of a doula-only group for billing purposes?** The manual's categories are "owners, members or managing employees" (§13.3.1) and payment goes to "the group which employs the doula" (§13.4). Contractor doulas are in the pilot. **Ask:** NYS DOH Bureau of Maternal and Child Health, MaternalAndChild.HealthPolicy@health.ny.gov (the contact the manual itself gives).
4. **MMC plan rates and billing instructions, per plan.** The manual sends providers to each plan: "providers must contact the member's MMC plan for billing instructions". Rates are contractually negotiated except for the continuity-of-care cohort. **Ask:** each MMC plan the Practice contracts with, using the directory at https://www.emedny.org/ProviderManuals/AllProviders/PDFS/Information_for_All_Providers_Managed_Care_Information.pdf
5. **Clearinghouse pricing for Waystar, Availity, Optum/Change Healthcare, TriZetto, Inovalon.** None publishes a price for 837P to a state Medicaid program. **Ask:** each vendor's sales team. I did not estimate.
6. **Outside billing service rates for New York doula claims.** No published number exists. Typically a percentage of collections, which 42 CFR 447.10(f) constrains. **Ask:** a New York medical billing company, or the pilot agency owner (#802).
7. **Whether Claim.MD will sign a BAA, and on what terms.** Their licence agreement page 403s to automated fetching. **Ask:** Claim.MD directly.
8. **The T1013 language-interpretation rate.** Not published on the doula fee schedule. **Ask:** eMedNY, or check the professional fee schedule of the billing provider type.
9. **Whether any modifiers apply to T1032 or T1033.** The Billing Guidelines point to "Modifier values and their definitions... on the web page for this manual under Procedure Codes and Fee Schedule", and the current fee schedule publishes none. Reading that as "no modifiers" is an inference; **ask:** eMedNY billing support, (800) 343-9000.
10. **Whether the Doula Cloud LLC's non-existence blocks a service bureau enrollment today.** The enrollment requires an IRS assignment letter showing the FEIN and applicant name, and a W-9 is not accepted. That is an entity-formation dependency, not a research question, but it belongs on the list.
11. **What a Practice's existing route actually is, what it costs her, her denial rate, and whether her doulas are individually enrolled.** Private to that agency and still owned by #802.

## Contradictions

Everything the map has been quoting checks out. The conflicts below are between sources, or between a source and an assumption the map carries implicitly.

1. **The nine-claims-per-pregnancy shape: confirmed, no conflict.** Up to eight T1032 plus one T1033 per pregnancy (Doula Policy Manual §13.1, and the fee schedule's "Per Pregnancy Allowance" column).
2. **Roughly $1,350 per pregnancy Rest-of-State: confirmed, no conflict** — $1,349.96 exactly. **But the map is incomplete.** The fee schedule also publishes an NYC rate the map does not carry: $93.75 per T1032 visit and $750.00 for T1033, i.e. $1,500.00 per pregnancy. Rate is location-dependent and the software must not hardcode one.
3. **1 March 2024 for fee-for-service: confirmed, no conflict** (Doula Policy Manual §13.6 and Document Control Properties).
4. **1 April 2025 for Managed Care: confirmed, no conflict** — but the map appears to treat this as an addition to an eMedNY-centered route, when the manual makes it a **replacement** for most members. Since that date, an MMC member's claim goes to her plan, not to eMedNY, and only FFS-enrolled members' claims go to eMedNY. Reimbursement at the FFS rate is guaranteed only for members who were already receiving services before 1 April 2025. Treating "the route into eMedNY" as the whole route understates the work.
5. **"$606 per pregnancy" appears in secondary summaries of the doula benefit.** It is a pilot-era or otherwise stale figure and conflicts with the current eMedNY fee schedule. Ignore it; the fee schedule file is the source of truth.
6. **The Doula Billing Guidelines say to leave the Medicaid Group Identification Number blank; the Doula Policy Manual requires it on every group claim.** Billing Guidelines Field 25B: "837P Ref: Loop 2010AA NM109 — Leave this field blank." Doula Policy Manual §13.4: when billing for a group, "The group Medicaid provider identification number (PID)... and the Medicaid PID of the individual doula who provided the service" must both be entered. **Resolution (inference):** the Billing Guidelines are written for a solo doula filling in the paper eMedNY-150003 form, and the "leave blank" instructions for the group and rendering-provider fields apply to that case only. A group claim populates 2010AA with the group and 2310B with the rendering doula. This should be verified before any 837P generator is built — the Billing Guidelines are the document a developer would reach for, and read literally they produce an invalid group claim.
7. **The Doula Policy Manual's header says "March 2026" while its Document Control Properties say "Document Version 6/2/2025".** Both appear in the same file, fetched 2026-09-05. Read the March 2026 header as the publication cycle and 6/2/2025 as the last substantive revision. The effective date of the benefit, 1 March 2024, is unaffected.
8. **"eMedNY charges nothing" needs one asterisk.** It is free for a provider. It is not free for a **software vendor** that wants to sit in the claims path: the service bureau enrollment carries a $750 application fee for 2026 and the associated compliance paperwork.

