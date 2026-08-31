# What a single-member New York LLC owes, to whom, and when

Research for [issue #385](https://github.com/markgoho/doula-cloud/issues/385), on the map [The business behind Doula Cloud](https://github.com/markgoho/doula-cloud/issues/375). Question: **what does a single-member New York LLC actually owe, to whom, and when?**

Facts held fixed, from the map: the owner lives and works in **Monroe County** (Rochester), not New York City; the LLC is **single-member** and a **disregarded entity** for federal tax by default; formation is targeted for **October 2026** and the tax year is assumed to be the **calendar year**; the business sells **multi-tenant SaaS** to doula Practices and runs a **Stripe Connect** platform.

**This is not tax advice.** It is a record of what the taxing authorities themselves publish, with each claim traced to the page or form instruction that owns it. Researched **25 August 2026**; amounts and dates are those in force on that date, and the dollar figures in particular are the kind that move.

Sources are first-party throughout: the NY Department of Taxation and Finance (tax.ny.gov), the NY Department of State (dos.ny.gov), and the IRS (irs.gov). **One exception, flagged**: the NYC Unincorporated Business Tax is not a New York State tax and tax.ny.gov does not state its rules — it points the reader at the NYC Department of Finance instead. That section is therefore sourced to nyc.gov, which is the government that owns the tax. There is no secondary source anywhere in this document.

## The three findings that matter

1. **Form IT-204-LL applies.** A single-member LLC that is a disregarded entity **must file** it if the LLC has any income, gain, loss, **or deduction** from New York sources. The word "deduction" is the sting: a pre-revenue LLC with nothing but expenses still owes the filing. The fee is **$25** and it is due the **15th day of the third month** after the tax year closes — **15 March 2027 for tax year 2026**, with no extension available. Nobody sends a bill for it.
2. **New York taxes this product's sales.** Prewritten software is taxable as tangible personal property "regardless of how the software is conveyed", **including by remote access**. Doula Cloud is prewritten software delivered by remote access. That makes a New York Practice's subscription a taxable sale, requires a **Certificate of Authority before the first sale**, and makes the published price a tax-exclusive-or-tax-inclusive decision. Raised against [#285](https://github.com/markgoho/doula-cloud/issues/285).
3. **NYC UBT does not reach Monroe County**, and neither does the MCTMT. Both are geographic, both are settled below, and neither needs re-raising.

## 1. Form IT-204-LL — the New York LLC filing fee

**It applies.** This was the question the ticket said not to assume either way, and the form's own instructions answer it directly.

> **Who must file.** You must file Form IT-204-LL, Partnership, Limited Liability Company, and Limited Liability Partnership Filing Fee Payment Form, if you are a:
> - limited liability company (LLC) that is a **disregarded entity for federal income tax purposes that has income, gain, loss, or deduction from New York State sources**; or [...]

— [Instructions for Form IT-204-LL (IT-204-LL-I), 2025](https://www.tax.ny.gov/pdf/current_forms/it/it204lli.pdf), "Who must file", emphasis added.

**What triggers it.** Not revenue. The trigger is *any* income, gain, loss, **or deduction** from New York sources. The instructions confirm the negative case too:

> **Do not file** Form IT-204-LL if you are: [...] a partnership, LLC, or LLP with **no** income, gain, loss, or deduction from New York sources regardless of whether or not you are formed under the laws of New York State or are **dormant**; or [...] an LLC or LLP that has elected to be treated as a corporation for federal income tax purposes.

So a genuinely dormant LLC — formed, and then nothing at all — does not file. An LLC that pays for a domain, a Stripe account and a bank account in the same year it was formed has **deductions from New York sources** and does. Treat the first return as due, not as optional.

The same instructions define what makes the source New York:

> A partnership carries on a business, trade, profession, or occupation within New York State if it maintains or operates an office, shop, store, warehouse, factory, agency, or other place in New York State where its affairs are systematically and regularly carried on, or it performs a series of acts or transactions in New York [State].

An LLC run from a desk in Rochester meets this.

**How much.** $25.

> If your LLC is treated as a disregarded entity for federal income tax purposes and has any income, gain, loss, or deduction from New York sources, the filing fee is **$25**.

The fee is otherwise a sliding scale keyed to New York source gross income, but for a disregarded entity it is flat $25 — the scale does not apply.

**When.**

> You must file Form IT-204-LL and pay the filing fee in full **on or before the 15th day of the third month following the close of your calendar or fiscal tax year**. When the due date falls on a Saturday, Sunday, or legal holiday, you must file and pay no later than the next business day.
>
> **There is no extension of time to file Form IT-204-LL or pay the annual fee.** If you fail to timely file Form IT-204-LL, or fail to pay the full amount of the filing fee by the due date, you may be subject to penalties and interest.

For a calendar-year LLC that is **15 March**, and there is no extension to hide behind.

**Identification number.** A disregarded SMLLC with no EIN may file under the owner's Social Security number — but the LLC will have an EIN anyway for the bank account and Stripe, and the instructions say to use it once it exists:

> However, if your LLC is considered a disregarded entity for federal income tax purposes and has not been previously assigned an employer identification number or a New York State temporary number, then enter your Social Security number. If you were issued a temporary number but now have an employer identification number, enter it here [...]

## 2. Federal — what the IRS wants

**The LLC does not file its own income tax return.** The IRS ignores it:

> If a single-member LLC does not elect to be treated as a corporation, the LLC is a "disregarded entity," and the LLC's activities should be reflected on its owner's federal tax return. [...] If the owner is an individual, the activities of the LLC will generally be reflected on: Form 1040 or 1040-SR Schedule C, Profit or Loss from Business (Sole Proprietorship) [...]

— [IRS, Single member limited liability companies](https://www.irs.gov/businesses/small-businesses-self-employed/single-member-limited-liability-companies).

So the recurring federal filing is **Form 1040 with Schedule C**, plus **Schedule SE** for self-employment tax on the net profit, filed on the owner's personal return and on the personal return's calendar — **15 April** for the prior calendar year.

**The LLC is *not* disregarded for employment and excise tax.** The same page:

> For wages paid after January 1, 2009, the single-member LLC is required to use its name and employer identification number (EIN) for reporting and payment of employment taxes.

Nothing switches this on while there are no employees. It is the line that moves when the first hire happens, which the map already parks under "Not yet specified".

**EIN.** Strictly, not required yet:

> [A] single-member LLC that is a disregarded entity that does not have employees and does not have an excise tax liability does not need an EIN. It should use the name and TIN of the single member owner for federal tax purposes.

The map already treats the EIN as in scope for other reasons — a bank account and Stripe both ask for one — so this changes nothing about the plan. It only means the EIN is a banking and platform requirement, not an IRS one, at this stage.

**Quarterly estimated payments.** There is no employer withholding on Schedule C profit, so the tax is paid in four instalments during the year.

Who has to:

> Individuals, including sole proprietors, partners, and S corporation shareholders, generally have to make estimated tax payments if they expect to owe tax of **$1,000 or more** when their return is filed.

— [IRS, Estimated taxes](https://www.irs.gov/businesses/small-businesses-self-employed/estimated-taxes).

The dates, for the 2026 tax year, from the form itself:

| Payment | Due |
| --- | --- |
| 1st payment | April 15, 2026 |
| 2nd payment | June 15, 2026 |
| 3rd payment | Sept. 15, 2026 |
| 4th payment | Jan. 15, 2027 * |

> \* You don't have to make the payment due January 15, 2027, if you file your 2026 tax return by February 1, 2027, and pay the entire balance due with your return.

— [Form 1040-ES (2026)](https://www.irs.gov/pub/irs-pdf/f1040es.pdf), "Payment Due Dates". The pattern repeats every year: 15 April, 15 June, 15 September, and 15 January of the following year, shifting to the next business day when it lands on a weekend or holiday.

**When the first payment actually falls due, here.** It depends on income that does not exist yet, so state the rule rather than a date:

- If the business earns nothing in 2026 (formation year, launch in January 2027), there is no 2026 estimated-tax obligation, and the first payment that could be required is **15 April 2027**, for the first quarter of 2027.
- If 2026 somehow produces net self-employment income large enough to push the owner's total expected tax $1,000 over withholding, the obligation attaches to whichever 2026 quarter the income lands in, and the last chance to cure it is **15 January 2027**.
- The safe harbour is the escape hatch either way: penalties are generally avoided if the owner pays "at least 90% of the tax for the current year, or 100% of the tax shown on the return for the prior year, whichever is smaller" (IRS, *Estimated taxes*). With W-2 withholding already in the picture on a personal return, increasing withholding is an alternative to writing quarterly cheques.

## 3. New York State — personal income tax and the MCTMT

Disregarded federally means disregarded by New York too: the LLC's profit lands on the owner's **Form IT-201** resident return, on the same 15 April calendar as the federal return.

**Estimated tax.** New York runs its own quarterly instalments, with a much lower entry threshold than the IRS:

> **Who must make estimated income tax payments** – Generally you must pay estimated income tax if you expect to owe, after subtracting your withholding [...] and credits, at least **$300** of either New York State, New York City, or Yonkers tax for 2026.

— [Instructions for Form IT-2105 (IT-2105-I)](https://www.tax.ny.gov/pdf/current_forms/it/it2105i.pdf).

$300, not $1,000. It is quite possible to owe New York estimated payments in a year where no federal ones are required.

The due dates match the federal ones — for 2026: **15 April 2026, 15 June 2026, 15 September 2026, 15 January 2027** ([Estimated tax payment due dates](https://www.tax.ny.gov/pit/estimated_tax/estimated_tax_payment_due_dates.htm)).

**MCTMT — does not apply.** The Metropolitan Commuter Transportation Mobility Tax is charged on self-employment earnings allocated to the Metropolitan Commuter Transportation District. [tax.ny.gov's MCTMT page](https://www.tax.ny.gov/bus/mctmt/default.htm) lists the district as two zones: **Zone 1** — New York (Manhattan), Bronx, Kings (Brooklyn), Queens and Richmond (Staten Island); **Zone 2** — Rockland, Nassau, Suffolk, Orange, Putnam, Dutchess and Westchester. **Monroe County is in neither.** The IT-2105 instructions confirm the tax attaches only to "net earnings from self-employment allocated to Zone 1 [or] Zone 2", and only above $150,000.

Recorded so it is not re-raised: **a Rochester-run LLC owes no MCTMT.** It would start to matter only if the business established a place in one of those twelve counties.

## 4. New York City Unincorporated Business Tax — does not reach Monroe County

**Source note.** UBT is a New York City tax, not a New York State one. The IT-204-LL instructions explicitly hand the question over — "For information regarding the tax treatment of an LLC or LLP for purposes of the [...] New York City Unincorporated Business Tax, visit the NYC Department of Finance website at www.nyc.gov/finance" — so the citation below is nyc.gov, the government that imposes it, reached through the state's own pointer.

> **WHO MUST FILE.** For tax years beginning in 2009 or later, any individual or unincorporated entity that **carries on or liquidates a trade, business, profession or occupation wholly or partly within New York City** and has a total gross income from all business regardless of where carried on of more than **$95,000** (prior to any deduction for cost of goods sold or services performed) must file an Unincorporated Business Tax Return.

— [Instructions for Form NYC-202 (2025)](https://www.nyc.gov/assets/finance/downloads/pdf/25pdf/business_tax_forms/nyc-202-instr_2025.pdf), emphasis added.

The test has two limbs joined by "and", and the **first limb is geographic**. A business carried on wholly in Monroe County is not carried on "wholly or partly within New York City", so the $95,000 income limb is never reached. The same instructions confirm that a single-member LLC disregarded federally is the entity that would file on Form NYC-202 if the geographic limb were met — the LLC form is not the escape; the geography is.

**Recorded so it is not re-raised: the NYC UBT does not apply to this LLC.** Two things would change that, and only two: carrying on the trade or business partly within the five boroughs (an office, or a place where the affairs are systematically and regularly carried on there — not a customer who happens to live there), or moving. Selling SaaS to a Practice located in Manhattan does **not**, by itself, make the business carried on within New York City. Note that this is the opposite of the sales-tax answer below, where the customer's location is exactly what matters — the two taxes ask different questions and it is worth not confusing them.

## 5. Sales tax on the product — New York taxes SaaS

**It bites.** This is the finding with a product consequence, not just a filing one.

### The rule

> Prewritten computer software is taxable as tangible personal property, whether it is sold as part of a package or as a separate component, **regardless of how the software is conveyed to the purchaser**. Therefore, prewritten computer software is taxable whether sold: on a disk or other physical medium; by electronic transmission; **or by remote access**.

> **Remotely accessed software.** A sale of computer software includes any transfer of title or possession or both, including a **license to use**. When a purchaser remotely accesses software over the Internet, the seller has transferred possession of the software because the purchaser gains constructive possession of the software and the right to use or control the software. Accordingly, **the sale to a purchaser in New York of a license to remotely access software is subject to state and local sales tax.**

— [Tax Bulletin ST-128, *Computer Software*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/computer_software.htm), emphasis added.

New York does not use the word "SaaS". It does not need to: it treats the licence to remotely access prewritten software as a sale of tangible personal property, and that is precisely what a Doula Cloud subscription is. **Prewritten** means "not designed and developed to the specifications of a particular purchaser" — a multi-tenant product sold to many Practices is prewritten by definition. The custom-software exemption is not available and cannot be engineered into: the bulletin closes that door explicitly, noting the exemption does not apply where "the sale is prewritten software that is available to be sold to customers in the ordinary course of business".

### Which customers

The tax follows the **user**, not the seller and not the server:

> The situs of the sale for purposes of determining the proper local tax rate and jurisdiction is **the location from which the purchaser uses or directs the use of the software**, not the location of the code embodying the software. Therefore, if a purchaser has employees who use the software located both in and outside of New York State, the seller of the software should collect tax based on the portion of the receipt attributable to the users located in New York.

So: a Practice whose doulas work in New York is a taxable sale at that Practice's local combined rate. A Practice entirely outside New York is not a New York taxable sale — but that is where multi-state economic nexus starts, which the map already parks under "Not yet specified". And a Practice with doulas in and out of New York requires the receipt to be **apportioned**, which is a billing-data problem, not just a tax-return problem.

Rates are combined state-plus-local and destination-based. Monroe County is **8%**, reporting code 2611 ([Publication 718, *New York State Sales and Use Tax Rates by Jurisdiction*](https://www.tax.ny.gov/pdf/publications/sales/pub718.pdf), effective 1 March 2025). Other New York jurisdictions differ, and the Department warns against deriving the rate from a ZIP code — "the use of ZIP codes for tax collection results in a high degree of inaccurate tax reporting" — pointing instead at its Jurisdiction and Rate Lookup Service.

### What that obliges, before the first sale

**A Certificate of Authority, applied for at least 20 days ahead.**

> You must register with the Tax Department and obtain a Certificate of Authority if you will be making sales in New York State that are subject to sales tax. [...] If you are required to register for sales tax, you must apply for your certificate **at least 20 days before you begin operating your business** or before purchasing assets of another business.

— [Tax Bulletin ST-175, *Do I Need to Register for Sales Tax?*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/do_i_need_to_register_for_sales_tax.htm).

The registration limb that applies here is the first one in that bulletin's list: "you maintain a place of business in the state, such as a store, office, or warehouse, and sell taxable tangible personal property or services to persons within the state". A Rochester office selling remote-access software to New York Practices is squarely inside it. Note also that "how often you sell or how much you charge [...] does not usually determine whether you need to register" — there is no small-seller floor to sit under.

Operating without one is expensive: up to **$10,000**, "imposed at the rate of up to $500 for the first day business is conducted without a valid Certificate of Authority, plus up to $200 per day for each day after" ([Tax Bulletin ST-360, *How to Register for New York State Sales Tax*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/how_to_register_for_nys_sales_tax.htm)).

**Then a return every quarter, whether or not anything sold.**

> Most vendors file quarterly when they first register to collect sales tax.

> **Even if your business did not make any taxable sales or purchases during the reporting period, you must file your sales and use tax return by the due date.**

> Quarterly returns are due no later than **20 days after the end of the quarter** to which they relate.

— [Tax Bulletin ST-275, *Filing Requirements for Sales and Use Tax Returns*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/filing_requirements_for_sales_and_use_tax_returns.htm).

The quarters are **1 March–31 May, 1 June–31 August, 1 September–30 November, and 1 December–28/29 February** — deliberately offset from calendar quarters, so the returns fall due on **20 June, 20 September, 20 December and 20 March**. A new registrant is a quarterly filer by default. Annual filing (Form ST-101, due 20 March) becomes available only once total tax due for four consecutive quarters is $3,000 or less, and the Department has notified the change; the reclassification is not something to assume in year one.

The zero return is the trap: **the minimum penalty for filing late is $50 "even if no tax is due for the reporting period."** A quarter with no sales still needs a filing.

### Why this is issue #285's problem, not only a filing item

Charging a New York Practice $X means either the Practice pays $X plus 8%-ish sales tax, or $X is treated as tax-inclusive and the business remits the tax out of it — a real haircut on revenue. That decision has to be made **before a price is published**, because changing a published price from tax-exclusive to tax-inclusive later is a price rise, and the reverse is a revenue cut. Raised against [#285](https://github.com/markgoho/doula-cloud/issues/285).

## 6. New York Department of State — the biennial statement

> Domestic and foreign business corporations are required by Section 408 of the Business Corporation Law, and limited liability companies are required by Section 301(e) of the Limited Liability Company Law, **to file a Biennial Statement every two years** [...] The filing period for a business corporation or LLC is the calendar month in which its original Certificate of Incorporation, Articles of Organization, or Application for Authority was filed with the New York Department of State.

— [NY Department of State, *Biennial Statements for Business Corporations and Limited Liability Companies*](https://dos.ny.gov/biennial-statements-business-corporations-and-limited-liability-companies).

- **Fee: $9.** Filed online through the [e-Statement Filing Service](https://filing.dos.ny.gov/eBiennialWeb/), which needs the exact entity name and the DOS ID number.
- **Due month: the anniversary month of formation**, every second year. For an LLC formed in October 2026, the first statement is due in **October 2028** — two years after formation, not one, which is exactly why it is easy to forget.
- **The reminder is conditional.** The Department "will send an email notice at the beginning of the calendar month in which the Biennial Statement is due" — *if an email address has been provided*. Provide one at formation, and do not rely on it.
- **Missing it does not dissolve the LLC**, but the entity shows as **past due** in DOS records, and "any Certificate of Status or status letter obtained from the New York Department of State will reflect that the corporation or LLC is past due" — which is the document a bank, an insurer or an enterprise customer asks for.

**Publication is separate and one-time**, already settled on the map: LLC Law §206 notice in two Monroe County newspapers for six consecutive weeks within 120 days of formation, then a $50 Certificate of Publication. It is in the calendar below as a deadline, but it is not a recurring obligation and is not re-researched here.

## 7. The calendar

Built on the map's assumptions: **Articles of Organization filed October 2026**, calendar tax year, launch and first revenue January 2027, no employees. Change the formation month and the publication deadline and the biennial statement move with it; nothing else does.

Every date below shifts to the next business day when it lands on a Saturday, Sunday or legal holiday — that rule is stated in the IT-204-LL instructions, Form 1040-ES and the IT-2105 instructions alike. Weekend landings in the first two years are called out inline.

### One-time, on the way in

| Date | What | Who | Cost |
| --- | --- | --- | --- |
| **October 2026** | Articles of Organization | NY Department of State | $200 |
| October 2026 | EIN — not required by the IRS at this stage, but required by the bank and by Stripe | IRS | free |
| **by ~mid-December 2026** | Apply for a **Certificate of Authority** — at least **20 days** before the first taxable sale. For a January 2027 launch, applying by **15 December 2026** leaves no margin; earlier is better, since the certificate arrives by post | NY Tax Department | free |
| **within 120 days of formation — by ~early February 2027** | LLC Law §206 publication: two Monroe County newspapers, six consecutive weeks. Six weeks of running time inside a 120-day window means starting by **December 2026**, not February | county-designated newspapers | ~$250–650 (map) |
| after publication | Certificate of Publication | NY Department of State | $50 |

### Recurring, in order of first occurrence

| Date | Obligation | Form | Amount | Notes |
| --- | --- | --- | --- | --- |
| **Fri 15 Jan 2027** | Q4-2026 federal + NY estimated tax | 1040-ES / IT-2105 | as computed | **Conditional.** Only if 2026 produced enough net self-employment income to cross $1,000 federal or $300 New York. In a formation-only year, expect nil |
| **Mon 15 Mar 2027** | **LLC filing fee, tax year 2026** | **IT-204-LL** | **$25** | **Almost certainly due.** 2026 deductions alone trigger it. No extension exists |
| Sat 20 Mar 2027 → **Mon 22 Mar 2027** | Sales tax, quarter 1 Dec 2026 – 28 Feb 2027 | ST-100 | tax collected | First return if registered in December. **File even if zero.** $50 minimum late penalty |
| **Thu 15 Apr 2027** | 2026 personal returns carrying the LLC's 2026 activity | 1040 + Sch. C + Sch. SE; IT-201 | as computed | |
| Thu 15 Apr 2027 | Q1-2027 federal + NY estimated tax | 1040-ES / IT-2105 | as computed | First payment that is realistically due, once January revenue starts |
| Tue 15 Jun 2027 | Q2-2027 estimated tax | 1040-ES / IT-2105 | | |
| Sun 20 Jun 2027 → **Mon 21 Jun 2027** | Sales tax, 1 Mar – 31 May 2027 | ST-100 | | |
| Wed 15 Sep 2027 | Q3-2027 estimated tax | 1040-ES / IT-2105 | | |
| Mon 20 Sep 2027 | Sales tax, 1 Jun – 31 Aug 2027 | ST-100 | | |
| Mon 20 Dec 2027 | Sales tax, 1 Sep – 30 Nov 2027 | ST-100 | | |
| Sat 15 Jan 2028 → **Tue 18 Jan 2028** | Q4-2027 estimated tax | 1040-ES / IT-2105 | | 17 Jan 2028 is a federal holiday |
| **Wed 15 Mar 2028** | LLC filing fee, tax year 2027 | IT-204-LL | $25 | |
| Mon 20 Mar 2028 | Sales tax, 1 Dec 2027 – 29 Feb 2028 | ST-100 | | |
| Sat 15 Apr 2028 → **next business day** | 2027 personal returns; Q1-2028 estimated tax | 1040 + Sch. C/SE; IT-201; 1040-ES / IT-2105 | | Check the exact day nearer the time: Emancipation Day (16 April, a District of Columbia holiday) can push the federal deadline past the following Monday |
| **October 2028** | **First Biennial Statement** | DOS e-Statement | **$9** | Anniversary month of formation, every two years. Email reminder only if an address is on file |

### The steady-state year, once it is running

- **Four times** — 15 April, 15 June, 15 September, 15 January: federal and New York estimated income tax.
- **Four times** — 20 March, 20 June, 20 September, 20 December: sales tax return, **even in a quarter with no sales**.
- **Once, 15 March**: Form IT-204-LL and $25.
- **Once, 15 April**: Form 1040 with Schedule C and Schedule SE, and Form IT-201.
- **Every second year, in the formation anniversary month**: the $9 Biennial Statement.

### The ones nobody sends a bill for

Worth naming, since that is the class of obligation this ticket exists to surface. **IT-204-LL** — $25, no bill, no extension, and it is due in the very first spring after formation whether or not a dollar came in. **The zero-dollar ST-100** — no revenue does not mean no return, and the minimum late penalty is $50 a quarter. **The Biennial Statement** — $9, two years after formation, with a reminder email that only arrives if an address was supplied at formation.

## 8. What this does not answer

- **Multi-state sales tax.** Every state decides SaaS for itself and economic-nexus thresholds turn on where Practices actually are. Already parked on the map under "Not yet specified", and the finding here — that the home state taxes the product — makes it more likely to matter, not less.
- **Whether Stripe Tax should compute and collect the sales tax**, and what that means for Stripe Connect. This is a real, ticketable question that this research surfaced and did not resolve: someone must calculate a destination-based rate per Practice, apportion a receipt where a Practice's doulas straddle the state line, and remit. Doing it by hand does not scale past a handful of Practices.
- **Whether the S-corp election ever pays.** Map-settled as a later-year CPA question.
- **What switches on at the first hire.** Payroll withholding, the LLC's own employment-tax identity, workers' compensation, unemployment insurance. Map-settled as out of scope while it is one person.
- **The Stripe Connect platform's own information-reporting position** — 1099-K issuance for connected accounts. Not part of this question, and not researched.

## Sources

All retrieved 25 August 2026.

**NY Department of Taxation and Finance**
- [Instructions for Form IT-204-LL (IT-204-LL-I), 2025](https://www.tax.ny.gov/pdf/current_forms/it/it204lli.pdf) — who must file, $25 disregarded-entity fee, due date, no extension
- [Instructions for Form IT-2105 (IT-2105-I), 2026](https://www.tax.ny.gov/pdf/current_forms/it/it2105i.pdf) — $300 estimated-tax threshold, MCTMT zones and $150,000 threshold
- [Estimated tax payment due dates](https://www.tax.ny.gov/pit/estimated_tax/estimated_tax_payment_due_dates.htm)
- [Metropolitan commuter transportation mobility tax (MCTMT)](https://www.tax.ny.gov/bus/mctmt/default.htm) — MCTD zone composition
- [Tax Bulletin ST-128, *Computer Software*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/computer_software.htm) — prewritten software, remote access, situs of the sale
- [Tax Bulletin ST-175, *Do I Need to Register for Sales Tax?*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/do_i_need_to_register_for_sales_tax.htm) — 20-day rule, registration limbs
- [Tax Bulletin ST-360, *How to Register for New York State Sales Tax*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/how_to_register_for_nys_sales_tax.htm) — penalty for operating without a Certificate of Authority
- [Tax Bulletin ST-275, *Filing Requirements for Sales and Use Tax Returns*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/filing_requirements_for_sales_and_use_tax_returns.htm) — quarters, 20-day due date, zero returns, $50 minimum penalty
- [Publication 718, *Sales and Use Tax Rates by Jurisdiction*](https://www.tax.ny.gov/pdf/publications/sales/pub718.pdf) — Monroe County 8%, code 2611

**IRS**
- [Single member limited liability companies](https://www.irs.gov/businesses/small-businesses-self-employed/single-member-limited-liability-companies) — disregarded-entity treatment, Schedule C, employment tax, EIN
- [Estimated taxes](https://www.irs.gov/businesses/small-businesses-self-employed/estimated-taxes) — $1,000 threshold, 90%/100% safe harbour
- [Form 1040-ES (2026)](https://www.irs.gov/pub/irs-pdf/f1040es.pdf) — the four payment due dates

**NY Department of State**
- [Biennial Statements for Business Corporations and Limited Liability Companies](https://dos.ny.gov/biennial-statements-business-corporations-and-limited-liability-companies) — $9, anniversary month, past-due consequence
- [e-Statement Filing Service](https://filing.dos.ny.gov/eBiennialWeb/)

**NYC Department of Finance** — used only for the UBT, which New York State does not administer and which the IT-204-LL instructions refer out to nyc.gov
- [Instructions for Form NYC-202 (2025)](https://www.nyc.gov/assets/finance/downloads/pdf/25pdf/business_tax_forms/nyc-202-instr_2025.pdf) — who must file, geographic limb, $95,000 income limb
