# Refunding unspent Credits: what happens to the New York sales tax

Question: a New York single-member LLC sells prepaid **Credits** to business customers, charges and collects New York sales tax at the moment of purchase, and is about to publish a policy that **unspent Credits are refundable at any time, with no time limit**. When a purchase is refunded — in the same quarter or three years later, in full or in part — what happens to the tax?

Facts held fixed, established elsewhere and not re-argued here: the taxable event is the **purchase** of the Credits, not their redemption; a licence to remotely access prewritten software is taxable in New York under [Tax Bulletin ST-128](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/computer_software.htm); Stripe Tax computes and collects the tax into the seller's ordinary Stripe balance and **never remits it**; the seller files Form ST-100 quarterly and remits by hand.

**This is not tax advice.** It is a record of what the taxing authority and Stripe themselves publish, with every claim traced to the document that owns it. Researched **28 August 2026**.

Sources are first-party throughout: the NY Department of Taxation and Finance (tax.ny.gov), the New York Tax Law as published by the New York State Senate (nysenate.gov), and docs.stripe.com. **One sourcing gap, flagged in full below**: the text of 20 NYCRR Part 534 is not published on tax.ny.gov, so the regulation is cited only through the statute it implements and through the Department's own advisory opinion that quotes it. **One claim is verified by running it**, not by reading it: the Stripe partial-reversal arithmetic, executed against the live Stripe sandbox on 28 August 2026 and reproduced below.

## How to read the labels

Every claim below carries one of these:

- **Quoted** — reproduced verbatim from the named first-party document.
- **Assembled** — no single publication says it; it follows from two or more quoted provisions read together, and the reasoning is shown.
- **Run** — verified by executing it against a live system, with the transcript.
- **Checked absent** — searched for and not found; what was searched is named.

## The four findings that matter

1. **The seller cannot keep the tax.** No New York publication says in terms "if you refund the price you must refund the tax" — that is a **checked absence** — but there is no lawful outcome in which the seller keeps it. Tax Law §1132(a)(1) makes the money the customer's, held "as trustee for and on account of the state"; §1139(a) bars the seller from recovering it from the State unless it first repaid the customer; and the Department's advisory opinion TSB-A-09(29)S says the customer "is entitled to a refund of sales tax only to the extent the original transaction is undone" — that is, entitled to it, to that extent. Refund the price, refund the tax on it.
2. **Recovering it from the State is a credit *and* an application — not a choice between them.** For a refund in a **later** quarter the current Form ST-100 instructions require all three: the credit on the Step 3 jurisdiction line, **Form ST-100-ATT (Quarterly Schedule CW)** filed with the return, and **Form AU-11** mailed separately with substantiation. Schedule CW line 12 is literally *"Refund issued to a customer for sale reported in a prior period."* For a refund in the **same** quarter as the sale there is no credit at all — the receipt is simply left out of the return.
3. **The recovery dies three years after the tax was payable.** For a sale in the 1 March – 31 May quarter, the ST-100 is due 20 June and the recovery closes on 20 June three years later. A no-time-limit refund promise therefore guarantees a window — opening between **three years and 20 days** and **three years, three months and 20 days** after each sale, depending where in the quarter it fell — in which the seller must hand back tax it can never get back. **This is a decision, not a finding**; it is set out in full in section 4.
4. **Stripe's location reports will silently misfile a cross-quarter refund.** Stripe puts a reversal in the location report for the period of the *original* transaction, "even if the refund occurred much later", and "doesn't allow the reassigning of refunds to alternate periods". New York wants the credit on the return for the quarter the refund was **given**. Fill the ST-100 from the summarized or itemized export, never from the location report, the moment any cross-quarter refund exists.

## 1. Must the tax be refunded to the customer?

**Yes. Assembled, not quoted** — and the assembly is worth showing, because the first place anyone looks does not contain the sentence.

**Checked absent.** Searched tax.ny.gov for a publication stating that a vendor must return sales tax when it returns the purchase price: Tax Bulletin ST-810 (*Sales Tax Credits*), Tax Bulletin ST-350 (*How to Apply for a Refund of Sales and Use Tax*), Tax Bulletin ST-770 (*Recordkeeping Requirements for Sales Tax Vendors*), the Form AU-11 instructions, and the Form ST-100 instructions. **None of them contains such a rule.** Every one of them is written from the other end — it assumes the tax has already been returned to the customer, and tells the vendor how to get it back from the State. That assumption is the rule, and three provisions make it binding.

**First: the money was never the seller's.** Tax Law §1132(a)(1):

> Every person required to collect the tax shall collect the tax from the customer when collecting the price, amusement charge or rent to which it applies. If the customer is given any sales slip, invoice, receipt or other statement or memorandum of the price, amusement charge or rent paid or payable, **the tax shall be stated, charged and shown separately** on the first of such documents given to him. **The tax shall be paid to the person required to collect it as trustee for and on account of the state.**

— [NY Tax Law §1132(a)(1)](https://www.nysenate.gov/legislation/laws/TAX/1132), emphasis added. The seller is a trustee of that amount. It is the State's money or the customer's money; on no reading is it the seller's.

**Second: New York will not give it back unless the customer got it first.** Tax Law §1139(a):

> **No refund or credit shall be made to any person of tax which he collected from a customer until he shall first establish to the satisfaction of the tax commission**, under such regulations as it may prescribe, **that he has repaid such tax to the customer.**

— [NY Tax Law §1139(a)](https://www.nysenate.gov/legislation/laws/TAX/1139), emphasis added.

The same rule appears on the face of Form AU-11 itself, as a certification the seller signs knowing that "willfully providing false or fraudulent information with this document with the intent to evade tax may constitute a felony":

> certify that all of the tax for which this claim is filed has been paid, and no portion has been previously credited or refunded to the applicant by any person required to collect tax; or, **if the claim for credit or refund is made by a person required to collect tax, that the amount claimed has previously been refunded to the appropriate purchaser**

— [Form AU-11 (12/10)](https://www.tax.ny.gov/pdf/current_forms/st/au11_fill_in.pdf), Certification, emphasis added.

**Third: the Department calls it the customer's entitlement.** TSB-A-09(29)S — an advisory opinion of the Department's Office of Counsel, published by the Department as its own view of the law:

> Since the sales tax initially collected is based upon the amount of the original transaction that gave the customer the use and /or possession of the merchandise, **the customer is entitled to a refund of sales tax only to the extent the original transaction is undone.**

— [TSB-A-09(29)S](https://tax.ny.gov/pdf/advisory_opinions/sales/a09_29s.pdf), 15 July 2009, emphasis added. Read the sentence forwards rather than backwards: to the extent the transaction *is* undone, the customer is entitled.

**Fourth, and this closes the loop: the customer has its own route to the money.** §1139(a)(i) permits an application for refund "in the case of tax paid by the applicant to a person required to collect tax, within three years after the date when the tax was payable by such person to the tax commission". A customer whose purchase price came back but whose tax did not can apply to the Department directly. The seller would then face a demand it cannot satisfy out of a credit it can no longer claim.

**Obligation or choice?** Two different things are being asked, and they have different answers.

- **Whether to refund the purchase price at all** is a matter of contract, not tax law. New York does not compel any seller to accept a return. TSB-A-09(29)S is the proof: the Department examined a retailer whose policy refunded 100%, then 75%, then 50%, then nothing, and did not question the policy — only the tax arithmetic that followed from it. **The refund policy is the seller's to write.**
- **Whether to refund the tax on whatever price is refunded** is not a choice. Once the price goes back, the tax on the refunded portion is tax on a receipt that no longer exists; §1132(a)(1) says the seller holds it as trustee; §1139(a) says the seller cannot recover it without having returned it; TSB-A-09(29)S says the customer is entitled to it. **Keeping it is not one of the available outcomes.**

## 2. How the seller recovers the tax from New York State

Two entirely different mechanisms, and which one applies turns on a single fact: whether the refund falls in the same **sales tax quarter** as the sale. New York's quarters are offset from calendar quarters — 1 March–31 May, 1 June–31 August, 1 September–30 November, 1 December–28/29 February — with the ST-100 due 20 days after each quarter ends ([Tax Bulletin ST-275](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/filing_requirements_for_sales_and_use_tax_returns.htm)).

### Same quarter as the sale: no credit, no form — leave it out

The delegating statute is Tax Law §1132(e):

> The commissioner may provide, by regulation, for the **exclusion from taxable receipts** [...] of amounts representing sales where **the contract of sale has been cancelled**, the property returned or the receipt, charge or rent has been ascertained to be uncollectible or, in case the tax has been paid upon such receipt [...] **for refund of or credit for the tax so paid**. Where the commissioner provides for a credit for the tax so paid, he or she shall require an application for credit to be filed, but he or she may also allow the applicant to immediately take the credit on the return which is due coincident with or immediately subsequent to the time the applicant files his or her application for credit.

— [NY Tax Law §1132(e)](https://www.nysenate.gov/legislation/laws/TAX/1132), emphasis added.

The regulation made under it, 20 NYCRR §534.6, provides that where the contract of sale is cancelled or the property returned **within the reporting period in which the sale was made**, the vendor may exclude those receipts from the sales and use tax return altogether. Nothing is claimed back, because nothing was ever reported. A Credit purchase refunded in full three weeks later, inside the same quarter, simply nets out of gross sales and taxable sales before the ST-100 is prepared.

**Sourcing gap, stated plainly.** The Department does not publish the text of 20 NYCRR Part 534 on tax.ny.gov — searched, and the site's "Regulations and regulatory actions" route points off-site to the State's contracted publisher, which is not reachable from here. The same-period exclusion is therefore stated above from the delegating statute §1132(e), which contains it in terms ("exclusion from taxable receipts [...] where the contract of sale has been cancelled"), and from the Department's own citation of §534.6 in TSB-A-09(29)S. **No wording of §534.6 is quoted anywhere in this document, because none was obtained from a first-party source.**

### A later quarter: three filings, not one

This is the case the refund policy actually creates, and the current form instructions are stricter than the bulletin most people would read first.

**The credit itself** goes on the jurisdiction line of the return, as a negative:

> **Credits against sales and services.** Credits that can be identified by locality must be reported on the appropriate line in Step 3. If the result is a negative number, show the negative using a minus sign (-). Examples of such credits include: [...] **tax paid on canceled sales, returned merchandise, and bad debts**; [...]

> This result may be a **net credit**, which you should show as negative using a minus sign (-).

— [Instructions for Form ST-100](https://www.tax.ny.gov/forms/current-forms/st/st100i.htm), Step 3, emphasis added. A quarter whose refunds exceed its sales produces a negative return, and the instructions contemplate that on their face.

Note that the amount entered is the **receipt**, not the tax: the credit is taken by reducing Column C (taxable sales) for the jurisdiction, and the tax falls out of the Column E rate. That matters, because the rate applied is the one printed on the line — so the credit must be booked against **the jurisdiction the original sale was sourced to**, which under ST-128 is where the purchaser uses or directs the use of the software, not where the seller sits.

**Then Schedule CW, filed with the return.** From the same instructions:

> **Are you claiming any credits?** As a registered vendor, you can claim a credit for sales tax you overpaid, paid by mistake, or **collected but then repaid to your customers**. You can apply the credit to reduce the tax you owe on your sales tax return. If you are claiming credits on this return or any schedules: Mark an X in the box and enter the amount of credits claimed. **Complete and submit Form ST-100-ATT, Quarterly Schedule CW, with this return. Complete Form AU-11, Application for Credit or Refund of Sales or Use Tax, and mail it to the address in the instructions with documentation to substantiate your claim.**

— emphasis added. Schedule CW carries a line written for exactly this situation:

> **11** Bad debt under Tax Law § 1132(e)
> **12** **Refund issued to a customer for sale reported in a prior period**
> **13** Materials stored in bulk or fabricated in New York State [...]

— [Form ST-100-ATT, Quarterly Schedule CW (6/26)](https://www.tax.ny.gov/pdf/current_forms/st/st100att.pdf), "Other types of credits". The form's own header repeats the point: *"Note: If you must complete this form, you must also complete Form AU-11 [...] and mail it to the address in the instructions with documentation to substantiate your claim."* Its Credit summary asks for **"the total amount of taxable receipts"** — again the receipt, not the tax.

**And AU-11, sent separately.** Not to the address the return goes to:

> Note that the mailing address for Form AU-11 is different from the mailing address for your paper sales tax return. You must send Form AU-11 (including Form POA-1, if required) to the above address for your credit to be processed in a timely manner.

— [Tax Bulletin ST-810](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/sales_tax_credits.htm). The address is NYS TAX DEPARTMENT, TDAB - SALES TAX REFUNDS, W A HARRIMAN CAMPUS, ALBANY NY 12227. A registered business may instead file AU-11 through Sales Tax Web File ([TB-ST-350](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/how_to_apply_for_a_refund_of_sales_and_use_tax.htm)).

### Is AU-11 the right form? Yes — and it is not optional

The question was worth asking, because two first-party documents pull in different directions and only one of them is current.

- **TB-ST-810** (issued 13 June 2014; page last updated 24 February 2026) reads as though AU-11 were the exception: *"In general, you may use Web File to claim a credit on your sales tax return"*, with AU-11 appearing under "Special circumstances" — *"If you cannot assign credits to a specific jurisdiction, you cannot claim the credit on your Web File or paper return. Apply for a refund using Form AU-11."*
- **The current Form ST-100 instructions and the current Schedule CW (6/26)** require AU-11 whenever *any* Step 3 credit is claimed, assignable to a jurisdiction or not.

**Conclusion: follow the form instructions.** They are the operative, currently-dated document, and the bulletin carries the Department's own standing caveat that "subsequent changes in the Tax Law or its interpretation may affect the accuracy of a Tax Bulletin". The later-quarter answer is therefore **all three**: credit on the ST-100 jurisdiction line, Schedule CW attached, AU-11 filed with substantiation.

**AU-11 also buys a cash refund instead of a credit, and the seller chooses.** The form has separate boxes for *Refund claimed* and *Credit claimed*, and one claim can be split between them: *"If you want to apply part of your claim as a credit on a sales tax return and you're requesting the balance as a refund, state these amounts separately in the applicable boxes."* ([Form AU-11-I (12/10)](https://www.tax.ny.gov/pdf/current_forms/st/au11i.pdf)). The cash route matters if refunds ever exceed collections for long enough that carrying a credit stops making sense — for instance after the business winds down, when there is no future ST-100 to absorb it.

**No interest, in practice, either way.** *"Generally, even if otherwise eligible, you won't receive interest if we process your claim for credit or refund within three months of the date we receive it in processible form"* (AU-11-I, "Interest"), and the Department is required by law to process a properly completed claim within six months (TB-ST-350).

## 3. The deadline, and the window the no-time-limit policy opens

### The limit

**Quoted, three times, in three different words for the same rule.** The Department's own statement of it, which applies to the credit-on-a-return route as much as to AU-11:

> **Deadline for filing.** You must claim your credit **within three years from the date the sales tax return was due, or two years from the date the tax was paid to the Tax Department, whichever is later.** For sales tax, the quarterly reporting periods end the last day of February, May, August, and November, and the annual reporting period ends the last day of February.

— [Tax Bulletin ST-810](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/sales_tax_credits.htm), emphasis added.

The same limit for the refund route:

> **Timeliness.** You must submit your application within three years from the date the tax was due to the Tax Department, or two years from the date you paid the tax, whichever is later.

— [Tax Bulletin ST-350](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/how_to_apply_for_a_refund_of_sales_and_use_tax.htm); and on the form itself, *"Generally, you must submit your application within three years from the date the tax was payable to the Tax Department, or two years from the date the tax was paid, whichever is later"* ([Form AU-11-I](https://www.tax.ny.gov/pdf/current_forms/st/au11i.pdf), "When to file your application").

**The statute of limitations, named**: Tax Law §1139, subdivisions (a) and (c). §1139(a)(ii) sets the limit for a vendor's own overpayment at "**within three years after the date when such amount was payable under this article**". §1139(c) states the standard formulation:

> Claim for credit or refund of an overpayment of sales tax shall be filed by the taxpayer **within three years from the time the return was filed or two years from the time the tax was paid, whichever of such periods expires the later**, or if no return was filed, within two years from the time the tax was paid.

— [NY Tax Law §1139(c)](https://www.nysenate.gov/legislation/laws/TAX/1139), emphasis added.

The statute measures from when the return was **filed**; the Department's publications measure from when it was **due**. For a return filed on time these are the same date, and for a seller who files on time — which is the only plan worth having, given the $50 minimum penalty on a late return — the distinction never arises. **Three years from the ST-100 due date is the operative deadline.** The two-year-from-payment limb never extends anything for a seller who pays with the return, because payment and the due date coincide.

### The window, computed

The clock starts at the ST-100 due date for the quarter the sale fell in, so the interval between the *sale* and the deadline depends on where in the quarter the sale landed.

| Sale date | Quarter | ST-100 due | Recovery closes | Gap from sale |
| --- | --- | --- | --- | --- |
| 1 March 2027 | 1 Mar – 31 May 2027 | 20 June 2027 | **20 June 2030** | 3 years, 3 months, 20 days |
| 15 April 2027 | 1 Mar – 31 May 2027 | 20 June 2027 | **20 June 2030** | 3 years, 2 months, 5 days |
| 31 May 2027 | 1 Mar – 31 May 2027 | 20 June 2027 | **20 June 2030** | 3 years, 20 days |
| 1 December 2027 | 1 Dec 2027 – 29 Feb 2028 | 20 March 2028 | **20 March 2031** | 3 years, 3 months, 19 days |

So: **the window in which the seller must refund tax it can no longer recover opens between three years and 20 days, and three years, three months and 20 days, after the sale** — and never closes, because the published policy never closes.

### What that means for the policy, and what must be decided

The mechanics are settled; this part is not a finding but a choice, and it has to be made before the policy is published.

Inside the window, a refund costs the seller the price (which it always did) **plus the tax**, out of pocket, with no route to recover it. On a $200 purchase in Monroe County at 8% that is $16 — small per refund, and entirely a function of how many three-year-old Credit balances exist. There is no partial mitigation: §1139(a) is a bar on the State paying, not a discount.

**Three ways to close it. Pick one:**

1. **Put a time limit on the refund promise that tracks the recovery window.** "Unspent Credits are refundable within three years of purchase" is the honest version of a promise the seller can actually keep whole. Simple, and it makes the tax question disappear.
2. **Keep the unlimited promise and absorb the tax knowingly.** Defensible — the amount is small and the goodwill is real — but it must be a decision recorded somewhere, not a surprise discovered in year four. If this is the choice, the refund policy should still not *promise* the tax back beyond the window without the seller having priced that in.
3. **Keep the unlimited promise but refund only the price after the window closes**, with the policy saying so in terms. This is the worst of the three. It leaves the customer holding a §1139(a)(i) claim against the Department, and a policy that reads "we refund your money but keep your sales tax" is a support conversation nobody wants.

**Recommendation: option 1**, with option 2 as a deliberate, documented fallback if an unlimited promise is judged worth the cost. **The decision that must be made before publishing: does the refund promise carry a time limit, and if not, is the seller accepting that refunds after roughly three years cost it the sales tax as well as the price?**

## 4. Partial refunds — 15 of 20 Credits

**Proportional. Quoted, and with the Department's own worked example.**

TSB-A-09(29)S answers this exact question. A retailer refunded less than the full purchase price on a return, and asked whether the customer got all the tax back or a proportionate share:

> We conclude that **the customer is entitled to only a partial refund of the sales tax, based upon a percentage of the purchase price refunded.** The New York State sales tax is a **transaction tax**. Since the sales tax initially collected is based upon the purchase price in the original transaction that gave the customer the use and/or possession of the merchandise, the customer is entitled to a refund of sales tax only to the extent the original transaction is undone. **If Petitioner retains a percentage of the original sales price, the sales tax collected on that retained amount (receipt) must be remitted to the State, and is not subject to a refund.**

The Department then works the arithmetic itself, at the same 8% combined rate that applies in Monroe County:

> [A]ssume that a customer purchases an item of merchandise from Petitioner for $100 where the receipt would be subject to a combined State and local sales tax rate of 8 %. In this example, the total tax that should be collected on the original sales transaction is $8.00. If the customer returns the merchandise 130 days from its purchase, **Petitioner will refund the customer $50.00 (50% of the purchase price based on its return policy), plus the amount of tax attributable to that $50.00, which is $4.00, for a total refund of $54.00.** The amount of tax attributable to the retained amount ($50.00) of the purchase price, which is $4.00, is not subject to a refund or credit. Petitioner can claim a refund or credit for the $4.00 in sales tax that it has refunded to its customer. It should be noted that **Petitioner may file the claim for refund or credit in a case only when the tax has actually been refunded to the customer.** See 20 NYCRR §534.6(a)(2). **Petitioner should maintain adequate documentation to support its refund claim(s).**

— [TSB-A-09(29)S](https://tax.ny.gov/pdf/advisory_opinions/sales/a09_29s.pdf), emphasis added.

**Applied to the Credits case.** 20 Credits bought for $200 in Monroe County: tax $16, charged $216. The customer spends 5 and asks for the 15 unspent back.

| | Amount |
| --- | --- |
| Original price | $200.00 |
| Tax collected at 8% | $16.00 |
| Fraction refunded (15 / 20) | 75% |
| Price refunded | $150.00 |
| **Tax refunded** | **$12.00** |
| **Total paid back to the customer** | **$162.00** |
| Tax on the 5 spent Credits, kept and remitted | $4.00 |
| Credit claimed on the ST-100 (Column C receipt) | $150.00 |

**Nothing about a partial refund changes the mechanism** — only the amounts. Same-quarter, exclude the $150 receipt from the return; later quarter, the $150 goes on the jurisdiction line as a negative in Column C, on Schedule CW line 12, and on AU-11. Nothing about a partial refund changes the deadline either; it runs from the original sale's quarter, not from the refund.

**One trap worth naming.** Because the refund is proportional to the *receipt*, a Credit product where Credits are consumed at different values — or where a discount, promotion or bonus Credit means the price per Credit is not uniform — cannot compute the refundable fraction by counting Credits. The fraction that matters is the fraction of the **dollars paid** that is being handed back. For a flat-price Credit that is the same number; the moment it is not, the count stops being the right divisor.

## 5. Documentation, and whether the customer's own position matters

### What the seller must keep

**Quoted.** The general vendor duty, Tax Bulletin ST-770:

> You must keep records of every sale, the amount of the sale, and the sales tax on the sale. You must retain a true copy of each: sales slip, invoice, receipt, contract, statement, or other memorandum of sale [...]

> All of your records must be dated and kept in good order. Your records must provide **sufficient detail to independently determine the taxable status of each sale and the amount of tax due and collected.**

> You must always **separately state the amount of sales tax due on the invoice or receipt** that you give your customer.

> **How long must I keep these records?** You must keep all of your records for a minimum of **three years from the due date of the return to which those records relate, or the date the return is filed, if later.** [...] The Tax Department may require you to keep records for a longer period of time, such as when the records are the subject of an audit, court case, or other proceeding.

> If you maintain records in an electronic format, all the requirements for paper records also apply to records created and stored electronically. Records that are maintained in an electronic format must be made available to the Tax Department in an electronically readable form.

— [Tax Bulletin ST-770](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/record-keeping_requirements_for_sales_tax_vendors.htm), emphasis added. Note the retention period is the *same three years* as the recovery deadline — so the records supporting a claim survive exactly as long as the right to make it, and no longer.

**What specifically substantiates a refund credit.** TB-ST-810's list of documents, of which the first is the one that matters here:

> copies of the receipts or invoices showing **the credit issued with the amount of the sales tax that was returned to the customer** after you remitted the tax to the Tax Department;
> [...] copies of the original invoices showing the amount of the sales, the sales tax collected, **the customer name and address**

> If your documents are voluminous, you may submit a summary in table form or a schedule.

— [Tax Bulletin ST-810](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/sales_tax_credits.htm), emphasis added.

The AU-11 instructions raise the bar further on what "returned to the customer" has to look like. In the closest analogous case they cover — a credit claimed after a customer produces an exemption certificate late — the Department wants **proof of payment, not just a credit memo**:

> If your customer originally paid you sales or use tax and subsequently submitted an exemption certificate requesting a credit or refund of the sales or use tax, you must include with your claim **proof that you repaid the tax to the customer, such as a copy of the canceled check.**

> All documentation must **clearly identify the purchaser.** Cash receipts, register tapes, or other forms of receipts or invoices that don't identify the purchaser may not be accepted.

> If the invoices and credit memoranda are voluminous, submit a schedule or a summary in table form. It should contain information concerning these documents, such as **invoice number, date of invoice, name of purchaser or supplier, item sold or purchased, amount of invoice excluding tax, amount of tax billed, taxing jurisdiction where sale or purchase was made, and the reason the claimant is entitled to a credit or refund.**

— [Form AU-11-I (12/10)](https://www.tax.ny.gov/pdf/current_forms/st/au11i.pdf), emphasis added.

**That last quotation is a schema.** It is the Department telling the seller exactly which columns a refund record needs: invoice number, invoice date, purchaser name, item, amount excluding tax, tax billed, taxing jurisdiction, reason. A Credits refund record built to those eight fields — plus the Stripe refund ID and the tax reversal ID, so the claim reconciles to the Stripe export — satisfies both the substantiation requirement and the summary-in-table-form allowance in one artefact.

**Assembled, the minimum per refund:**

- The original invoice or receipt, showing the price, the separately stated tax, the customer's name and address, and the jurisdiction the sale was sourced to.
- The credit memo or refund receipt, showing the price refunded **and the tax refunded** as separate amounts.
- Proof the money actually moved — the Stripe refund object is the analogue of AU-11-I's "canceled check".
- The Stripe tax reversal transaction ID, tying the refund to the figure the ST-100 credit is built from.
- The arithmetic for a partial refund: fraction of dollars refunded, and the tax computed on that fraction.

Kept three years from the due date of the return on which the credit was claimed, at least; keeping them for three years from the *original* sale's return as well is the safer reading, since that is the return the Department would test the credit against.

### Does the customer's own treatment change anything?

**Checked absent, and the absence is the answer.** Searched TB-ST-810, TB-ST-350, TB-ST-770, the AU-11 form and its instructions, and the ST-100 instructions for any requirement that a vendor obtain a statement, certificate or confirmation from the customer about how the customer treated the tax — whether it claimed a credit of its own, paid use tax, or deducted the purchase. **There is no such requirement in any of them.** The only affirmative documentary duty pointed at the customer is that the paperwork must identify the purchaser, and that the seller must be able to prove it repaid the purchaser.

**But it matters in exactly one way, and the AU-11 certification is where.** The seller certifies:

> certify that **no amount claimed has previously been subject to a credit or refund**

— [Form AU-11](https://www.tax.ny.gov/pdf/current_forms/st/au11_fill_in.pdf), Certification.

The risk that certification exists to catch is real here, because **the customer has a parallel claim of its own.** §1139(a)(i) lets a person who paid tax to a vendor apply to the Department directly, within three years of when the vendor's tax was payable. If a customer had already applied and been paid, and the seller then refunded the tax and claimed a credit, the State would have paid twice and the seller's certification would be false. In ordinary practice this does not happen — a customer who is getting a refund from the seller has no reason to go to Albany — but it is the one fact about the customer's own position that the seller is signing a statement about.

**Two things that specifically do *not* matter:**

- **Use tax.** A New York purchaser owes use tax only where New York sales tax was not collected on a taxable purchase. Here the seller collects sales tax at purchase, so the customer never had a use tax liability on this transaction, and there is nothing on the customer's side to unwind.
- **Whether the customer deducted the purchase, or is a business rather than a consumer.** New York's credit turns on whether the receipt was returned and the tax repaid to the purchaser — TSB-A-09(29)S frames it entirely in terms of "the extent the original transaction is undone". Nothing in the sources conditions it on what the purchaser did with the expense. **Checked absent**: no such condition appears in §1132(e), §1139, TB-ST-810, TB-ST-350, or AU-11 and its instructions.

**One thing that would matter but does not arise here.** A customer that had given the seller a **resale or exemption certificate** would have been charged no tax in the first place, so there would be nothing to refund. AU-11-I's separate rules for exemption-certificate claims — original invoice, certificate, credit memoranda, canceled check — belong to a different situation than an unspent-Credit refund, and are cited above only for the standard of proof they set for "repaid the customer".

## 6. The Stripe side

### The objects

A completed sale in Stripe Tax is a **tax transaction** (`tax.transaction`, id `tax_...`, `type: transaction`), created either automatically by a Checkout Session or a finalized invoice, or explicitly from a **tax calculation** (`tax.calculation`, id `taxcalc_...`) with [`POST /v1/tax/transactions/create_from_calculation`](https://docs.stripe.com/api/tax/transactions/create_from_calculation). A refund is a second tax transaction with `type: reversal`, pointing back at the first:

> After creating a tax transaction to record a sale to your customer, you might need to record refunds. These are also represented as tax transactions, with `type=reversal`. Reversal transactions offset an earlier transaction by having amounts with opposite signs. For example, a transaction that recorded a sale for 50 USD might later have a full reversal of -50 USD.

— [Collect tax on off-Stripe payments](https://docs.stripe.com/tax/off-stripe), "Record refunds". The call is [`POST /v1/tax/transactions/create_reversal`](https://docs.stripe.com/api/tax/transactions/create_reversal), taking `original_transaction`, a unique `reference`, and `mode` of either `full` or `partial`.

### Automatic or explicit? It depends on how the sale was taken

**Quoted, and this is the decisive list.** The Stripe Tax reporting page enumerates exactly which operations move the reported tax balance:

> The following operations *decrease* the balance of total tax reported:
> - Voiding an invoice.
> - Marking an invoice as uncollectible.
> - Creating a credit note.
> - **Creating a refund of a charge associated with an invoice or a Checkout Session.**
> - **Creating a reversal of a tax transaction using the Stripe Tax API.**

— [Tax reporting](https://docs.stripe.com/tax/reports), emphasis added.

So there are two regimes, and the seller is in one of them depending on how Credits are sold:

- **Checkout Session, Payment Link, or invoice with `automatic_tax[enabled]=true`** — refunding the charge is enough. The reversal is created for you and the reports move. The Refunds page confirms the boundary: *"If you're using Stripe Tax APIs to record sales, you must [record refunds](https://docs.stripe.com/tax/payment-intent/custom#reversals)"* ([Refund and cancel payments](https://docs.stripe.com/refunds)) — the "if" is doing the work.
- **PaymentIntent with the Stripe Tax API, or the standalone Tax API for an off-Stripe sale** — nothing is automatic. *"When you issue a refund, you must create a reversal tax transaction with a unique `reference`."* Refund without the reversal call and the sale stays on the books at full tax.

**This is an integration-shape decision, not a compliance detail.** The regime the Credits purchase flow lands in decides whether refund-time tax handling is free or is code the seller has to write and keep correct.

### Partial refunds — supported, and the arithmetic matches New York's

`mode=partial` takes either explicit per-line-item amounts or a single flat after-tax figure:

> Alternatively, you can create a reversal with `mode=partial` by specifying a flat after-tax amount to refund. **The amount distributes across each line item and shipping cost proportionally**, depending on the remaining amount left to refund on each.

> You can create up to **30 partial reversals** for each sale. **Reversing more than the amount of tax you collected returns an error.**

— [Collect tax on off-Stripe payments](https://docs.stripe.com/tax/off-stripe), emphasis added. Tax transactions are immutable; a partial reversal is undone only by creating a **full** reversal of the partial reversal, and a full reversal of a sale does not cancel earlier partials — *"you need to fully reverse any previous partial reversals for the same transaction to avoid duplicate refunds."*

**Run — verified against the live Stripe sandbox, 28 August 2026.** Rather than reason from the docs, the 20-Credit case was executed end to end. A calculation for a $200 line item (quantity 20, `tax_behavior: exclusive`, tax code `txcd_10103001`) delivered to 100 State St, Rochester, NY 14614 returned `amount_total: 21600` — that is $200 plus **$16.00 tax, an 8% rate**, matching Monroe County in Publication 718. A transaction was created from it (`tax_1U9R9V1rKoVEA79vzWq3Enie`), then a flat-amount partial reversal for 15 of the 20 Credits:

```
stripe post /v1/tax/transactions/create_reversal \
  -d "original_transaction=tax_1U9R9V1rKoVEA79vzWq3Enie" \
  -d "reference=doula-credits-order-001-refund-15" \
  -d "mode=partial" \
  -d "flat_amount=-16200"
```

The reversal line item came back as:

```
{'reference': 'refund_credits20', 'amount': -15000, 'amount_tax': -1200,
 'quantity': 20, 'tax_behavior': 'exclusive'}
```

**$150.00 of price and $12.00 of tax** — precisely the 75% proportional split TSB-A-09(29)S requires, arrived at by Stripe without being told the fraction. **Stripe's flat-amount distribution and New York's apportionment rule agree.** Passing the after-tax total the customer is actually owed ($162.00) is therefore both the simplest call to make and the one that produces the number the ST-100 credit needs.

Two observations from the run that the docs do not spell out:

- **`quantity` on the reversal line stayed at 20**, not 15. The units figure in an itemized export (`quantity_decimal`) is not a reliable record of how many Credits were refunded; the money is.
- The reversal's line item `reference` was auto-derived (`refund_credits20`) from the original line reference. Only the transaction-level `reference` was supplied and it must be unique — that is the string the tax export is reconciled by, so it should carry the Stripe refund ID or the order ID plus a suffix.

**One caution Stripe states in its own words**, relevant if the seller ever refunds tax and price in a ratio other than the original one:

> If you refund a tax amount such that the total tax is no longer proportional to the subtotal, **your tax reporting can be unreliable.** It won't automatically adjust the taxable and nontaxable amounts, and won't reflect the reason for the tax reversal [...] We recommend not refunding partial line item tax amounts. Instead, fully reverse the transaction and create a new one with appropriate inputs for an accurate tax calculation.

— [Tax reporting](https://docs.stripe.com/tax/reports). New York requires the proportional split anyway, so following TSB-A-09(29)S keeps the seller inside Stripe's supported case automatically.

### A refund outside Stripe's awareness overstates the report — silently

**Yes, and there is no warning.** The decisive list quoted above is closed: the only things that *decrease* reported tax are voiding or writing off an invoice, a credit note, refunding a charge tied to an invoice or Checkout Session, and an explicit tax-transaction reversal. Anything else leaves the tax on the books.

That means each of the following produces a Stripe tax report that claims the seller owes New York more than it does:

- Paying the customer back **outside Stripe** — a bank transfer, a cheque, a credit applied elsewhere.
- Refunding a **PaymentIntent-based** charge without calling `create_reversal`.
- Refunding a charge whose sale was recorded through the **standalone Tax API**, again without the reversal.

And one where the report is wrong in the other direction, stated by Stripe explicitly:

> **Disputes** that are upheld by the cardholder's bank. Stripe Tax doesn't decrease the balance of the collected total tax. For example, for a disputed transaction with an amount of 100 USD and exclusive tax of 10 USD, Tax reports still reflect the total tax collected as 10 USD.

> Refunds of **uncaptured amounts** of a payment [...] When the capture amount is lower than the original amount, Stripe Tax doesn't reduce the total balance of the collected tax.

— [Tax reporting](https://docs.stripe.com/tax/reports). A chargeback undoes the sale in the real world and does not undo it in the tax report; that is a manual adjustment, and the same New York credit machinery applies to it.

**No exception surfaces, no report flags it.** The overstatement is invisible unless someone reconciles refunds against reversals. The operational rule that follows: **every Credits refund is issued as a Stripe refund against the original charge, never by any other means** — and where the integration is API-based, the reversal call is made in the same code path as the refund, not as a follow-up someone remembers.

### The location-report trap

This is the finding most likely to cause an actual filing error, and it is a straight collision between how Stripe files a refund and how New York wants it filed.

> **Refunds.** Location reports include refunds associated with an original transaction **in the same period as the original transaction, even if the refund occurred much later.** This can affect the aggregated amounts in a report. **Stripe doesn't allow the reassigning of refunds to alternate periods.**

— [Tax reporting](https://docs.stripe.com/tax/reports), emphasis added.

New York wants the opposite. A refund given in a later quarter is a credit **on the return for the quarter in which the refund was given** — that is what Schedule CW line 12 says in terms ("Refund issued to a customer for sale reported in a prior period") and what §1132(e)'s exclusion route implies by carving out only the same-period case.

**So a sale in Q1 refunded in Q3 will:**

- **retroactively change the Q1 location report**, for a quarter whose ST-100 has already been filed and paid; and
- **not appear in the Q3 location report at all**, which is the return where New York actually wants the credit.

A seller who fills in each ST-100 from the location report will therefore under-claim in the quarter the refund happened, and — if anyone ever re-runs an old period to check their work — find a Q1 report that no longer matches the Q1 return they filed.

**The exports do not have this problem.** In the itemized export a reversal is its own row with `transaction_type: reversal`, its own `transaction_date` ("The time at which the tax liability is assumed **or reduced**") and a `reversal_original_tax_transaction_id` pointing at the sale it undoes. The summarized export aggregates by the date range requested, with `total_sales_refunded` and `total_tax_refunded` carrying the reversals and `filing_tax_payable` giving "the net tax liability (tax collected minus tax refunded)".

**Conclusion: build the ST-100 from the summarized export for the New York quarter, and use the itemized export for the sub-state jurisdiction breakdown the ST-100's jurisdiction lines need.** Stripe says the same about sub-state detail — *"Use itemized exports for US states that require sub-state level reporting."* **Do not use the location report** as the source for a return once any cross-quarter refund exists. Two further reasons to distrust it here: Stripe's own caveat that *"Stripe Tax currently doesn't support use cases beyond your transaction data, such as credits, prepayments, discounts, and so on. As a result, the final numbers for your business's filing might vary"* — and prepayments are precisely what Credits are — and the fact that location reports cannot be downloaded, only viewed, so there is nothing to keep for the three-year record.

**One timing note.** *"Completed transactions can take up to 24 hours to appear in reports."* The ST-100 is due 20 days after quarter end; pulling the export on day one of the new quarter risks missing the last day's activity.

## 7. The practical consequence

**What the policy should say.** State the refund in tax-aware terms rather than leaving it to be discovered: unspent Credits are refunded at the price paid **plus the New York sales tax charged on the refunded amount**, and where only part of a balance is refunded, the tax comes back in the same proportion as the money — 15 of 20 Credits means 75% of the price and 75% of the tax. Give the promise a **three-year limit from the date of purchase**, or, if it is to stay unlimited, take that decision knowing that a refund given after roughly three years costs the seller the sales tax as well as the price with no way to recover it from New York. Say nothing that implies the tax is the seller's to keep, because it is not; §1132(a)(1) makes it the customer's money held in trust.

**What must happen each time a refund is issued.** Compute the refundable fraction from dollars paid, not Credits counted, and refund the price and the tax in that proportion. Issue it as a Stripe refund **against the original charge** — never a bank transfer, never a manual payment — and if the sale was taken through the Tax API rather than Checkout or an invoice, create the tax transaction reversal in the same code path, passing the after-tax total as `flat_amount` so Stripe's proportional split lands on New York's answer. Record, against the refund: the original invoice with its separately stated tax and sourcing jurisdiction, the credit memo showing price and tax refunded separately, the Stripe refund ID as proof the money moved, and the reversal transaction ID; keep it three years. Then, at quarter end, if the refund fell in the **same** quarter as the sale, simply leave the receipt out of the ST-100. If it fell in a **later** quarter, put the refunded receipt on the ST-100 jurisdiction line as a negative in Column C, file **Schedule CW** with line 12 completed, and file **AU-11** with the substantiation — all within three years of the *original* sale's ST-100 due date. Build every one of those numbers from the summarized and itemized Stripe exports, never the location report, because Stripe files a late refund back into the quarter of the sale and New York wants it in the quarter of the refund.

## What this does not answer

- **New York abandoned property law and escheatment on unredeemed Credits.** Deliberately out of scope; it is a separate question owned elsewhere and nothing here should be read as touching it.
- **Sales tax on refunds in states other than New York.** Every state sets its own rule on both the refund of tax and the recovery deadline, and the proportional answer here is New York's, evidenced by a New York advisory opinion.
- **Whether the Credits price is published tax-inclusive or tax-exclusive.** It changes what "the price paid" means in a refund and therefore what the customer sees. Everything above is written for the tax-exclusive case, which is what Stripe Tax with `tax_behavior: exclusive` produces.
- **Which Stripe product tax code is correct for the Credits product.** Not researched, but the sandbox run turned up a warning worth recording: `txcd_10502000` returned `taxability_reason: product_exempt` and **zero tax** for the Rochester address, while `txcd_10103001` returned the expected 8%. The tax code is not a cosmetic field, and a wrong one produces a confidently-computed $0 of New York tax on a taxable sale. It should be settled deliberately against ST-128 before launch.
- **Whether a refund can be given at all under the contract**, and how partial-refund arithmetic interacts with promotional or bonus Credits priced at something other than the flat rate. Flagged in section 4; not resolved.

## Sources

All retrieved 28 August 2026.

**New York Tax Law** (New York State Senate)
- [Tax Law §1132](https://www.nysenate.gov/legislation/laws/TAX/1132) — §1132(a)(1) separately stated tax, collected "as trustee for and on account of the state"; §1132(e) exclusion from taxable receipts on cancelled sales
- [Tax Law §1139](https://www.nysenate.gov/legislation/laws/TAX/1139) — §1139(a) no refund or credit to the collector until the customer has been repaid, and the customer's own three-year application right; §1139(c) the three-year / two-year limitation

**NY Department of Taxation and Finance**
- [Tax Bulletin ST-810, *Sales Tax Credits*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/sales_tax_credits.htm) — when a credit may be claimed, documentation, deadline, AU-11 mailing address
- [Tax Bulletin ST-350, *How to Apply for a Refund of Sales and Use Tax*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/how_to_apply_for_a_refund_of_sales_and_use_tax.htm) — eligibility including "collected, reported, and remitted sales tax but then repaid it to your customers", timeliness, six-month processing
- [Tax Bulletin ST-770, *Recordkeeping Requirements for Sales Tax Vendors*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/record-keeping_requirements_for_sales_tax_vendors.htm) — what to keep, separately stated tax, three-year retention, electronic records
- [Tax Bulletin ST-275, *Filing Requirements for Sales and Use Tax Returns*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/filing_requirements_for_sales_and_use_tax_returns.htm) — the offset quarters and the 20-day due date
- [Tax Bulletin ST-128, *Computer Software*](https://www.tax.ny.gov/pubs_and_bulls/tg_bulletins/st/computer_software.htm) — taxability of remotely accessed prewritten software and the situs rule (background, established elsewhere)
- [Instructions for Form ST-100](https://www.tax.ny.gov/forms/current-forms/st/st100i.htm) — Step 3 credits by locality, "tax paid on canceled sales, returned merchandise, and bad debts", net credit, and the Schedule CW plus AU-11 requirement
- [Form ST-100-ATT, Quarterly Schedule CW, Credit Worksheet (6/26)](https://www.tax.ny.gov/pdf/current_forms/st/st100att.pdf) — line 12, "Refund issued to a customer for sale reported in a prior period"
- [Form AU-11, *Application for Credit or Refund of Sales or Use Tax* (12/10)](https://www.tax.ny.gov/pdf/current_forms/st/au11_fill_in.pdf) — refund and credit boxes, the certification
- [Instructions for Form AU-11 (AU-11-I) (12/10)](https://www.tax.ny.gov/pdf/current_forms/st/au11i.pdf) — proof of repayment, documentation schema, splitting a claim, when to file, interest
- [TSB-A-09(29)S](https://tax.ny.gov/pdf/advisory_opinions/sales/a09_29s.pdf) — advisory opinion, 15 July 2009: partial refunds are proportional, with the Department's worked 8% example, and its citation of 20 NYCRR §534.6(a)(2)

**Not obtained** — 20 NYCRR Part 534. Searched tax.ny.gov; the Department does not publish the regulation text, and its off-site route to the State's contracted publisher is not reachable from here. Part 534 is relied on in this document only through Tax Law §1132(e), which delegates it, and through the Department's own citation of §534.6(a)(2) in TSB-A-09(29)S. No wording of it is quoted.

**Stripe**
- [Tax reporting](https://docs.stripe.com/tax/reports) — the lists of operations that increase and decrease reported tax, the location-report refund rule, partial-refund unreliability, disputes and uncaptured amounts, itemized and summarized export column references
- [Collect tax on off-Stripe payments](https://docs.stripe.com/tax/off-stripe) — recording refunds, full and partial reversals, flat-amount distribution, the 30-reversal limit, undoing a partial refund
- [Custom Stripe Tax API](https://docs.stripe.com/tax/payment-intent/custom) — the same reversal mechanics for PaymentIntent integrations
- [Refund and cancel payments](https://docs.stripe.com/refunds) — "If you're using Stripe Tax APIs to record sales, you must record refunds"
- [Create a reversal Transaction](https://docs.stripe.com/api/tax/transactions/create_reversal) and [Create a transaction from a calculation](https://docs.stripe.com/api/tax/transactions/create_from_calculation) — API reference

**Run, not read** — Stripe sandbox, 28 August 2026: calculation `taxcalc_1U9R9V1rKoVEA79v7qkEMwVO`, transaction `tax_1U9R9V1rKoVEA79vzWq3Enie`, partial reversal `tax_1U9R9l1rKoVEA79vAx1Sqdob`. Confirms the 8% Monroe County rate on a NY-delivered SaaS line item and the 75% proportional split of a flat-amount partial reversal into `amount: -15000` and `amount_tax: -1200`.
