# `/terms` — the Terms of Service a Practice agrees to

The copy below is **final and verbatim**. It was written on [Publish the Terms of Service and the Privacy Policy at /terms and /privacy (#1556)](https://github.com/markgoho/doula-cloud/issues/1556), from the two research files ADR-0053 names and the ADRs each clause cites below. The decisions no record held before 2026-09-28 are [ADR-0054](../adr/0054-the-terms-hold-the-refund-rule-a-dispute-goes-to-court-and-only-an-owner-agrees.md)'s.

Built as `site/src/routes/terms/+page.svelte` on `site/src/lib/components/ReadingPage.svelte`. The spec beside it asserts the sentences that do legal work by their words.

Do not reword a clause without reading its row under [Why each clause says what it says](#why-each-clause-says-what-it-says). Several read like ordinary prose while they do legal work, and a sentence removed because it looked redundant can be the one a court reads.

## Where it lives, and how it is served

- URL: `https://doula.cloud/terms`
- **Indexed.** No `noindex`, a canonical address, and an entry in `sitemap.xml`. A Practice has to be able to find what she agreed to.
- Linked from the site footer on every page of the site, and from the agreement sentence on `/signup` ([#1547](https://github.com/markgoho/doula-cloud/issues/1547)), which opens it in a new tab.
- Reachable with no password, no cookie and no JavaScript.

## Constraints the copy is written under

- **Each sentence is true of the product as built today.** A sentence that could not be checked against the code or an ADR was not written (ADR-0054).
- **A new version is three edits in one commit:** this document and the page, an entry in `site/src/lib/legal.ts`, and the same entry in `api/internal/legal/legal.go`. `site/src/lib/legal.spec.ts` fails when the last two differ.
- **The Refunds section is `/pilot-terms`' word for word, and `/support`'s in substance.** Change all three or none (ADR-0054).
- **The word "arbitration" is not on the page.** The spec fails if it is (ADR-0054).
- **Connect Terms §3.4(b)** forbids holding DoulaCloud out as a payment facilitator, intermediary or aggregator. Those words are not on the page either.
- **No capitals-only paragraph.** The warranty and liability clauses are in plain sentences.
- **The product name is a token.** Every mention goes through `PRODUCT_NAME` in `site/src/lib/product.ts`, so [#1463](https://github.com/markgoho/doula-cloud/issues/1463) stays a one-edit change.

---

# Terms of Service

These terms are the agreement between your practice and Elephantine LLC, a New York limited liability company that operates DoulaCloud. In them, “we” and “us” mean Elephantine LLC, and “you” means your practice.

Your practice agrees to these terms when its Owner creates it, by pressing Create Practice under the sentence that links them. The Owner agrees for the practice, and must have the authority to do so.

This version takes effect on October 3, 2026. The history of every version is at the end of this page.

## What DoulaCloud is

DoulaCloud is software a doula practice uses to run its business: its clients, engagements, visits, plans, contracts, invoices and messages. It is offered to practices in the United States.

## What a Credit costs

One Credit starts one engagement — a single client relationship centered on one baby, from intake through the end of care. Birth and postpartum engagements cost the same.

A Credit costs **$20.00**, plus sales tax where it applies. That is the one price, and every feature is included in it.

There is no subscription, no minimum and no recurring charge. You buy Credits when you want them and are charged nothing in between. Credits do not expire, and the oldest are spent first.

If the price of a Credit ever rises, we email each Owner at least 60 days before, with the reason. A rise never changes a Credit you already bought: it keeps the price you paid for it, and until the new price starts you can buy as many Credits as you like at the old one.

When you buy Credits, Stripe takes the payment for us. We never see or store your card number.

## Refunds

Credits given free of charge are not refundable.

Purchased Credits that have not been spent can be refunded within three years of the date they were bought, at the price paid for them and together with any sales tax charged on them, to the original payment method. A Credit already used to start an engagement has been spent, and is not refundable. To ask for a refund, email us. We do not need a reason.

## Your practice is the merchant

DoulaCloud is not a payment service. When your practice invoices a client, your practice is the merchant: it holds its own agreement with Stripe, the money is paid into its own account, and it is responsible for the care it provides and for what it charges. We never receive or hold your practice's money, and we take no part of any payment a client makes to you.

## Stripe

Payment processing services for your practice are provided by Stripe and are subject to the [Stripe Connected Account Agreement](https://stripe.com/connect-account/legal/full), which includes the Stripe Terms of Service. That agreement is between your practice and Stripe. Stripe asks your Owner to accept it on Stripe's own pages, when your practice is connected to Stripe, and Stripe can change it from time to time.

You agree to give accurate and complete information about your practice, and you authorize us to share it, and information about the payments your practice takes through DoulaCloud, with Stripe. Stripe tells us whether your practice's account can take payments.

Stripe processes your data as [Stripe's Privacy Policy](https://stripe.com/privacy) explains.

## Leaving

You can leave at any time, and you take everything with you.

**Export.** An Owner can download everything your practice has put into DoulaCloud as one archive, in formats you can open without DoulaCloud. Take it before you start deletion: the archive cannot be downloaded while deletion is under way.

**Deletion.** An Owner can delete your practice from its settings, with no call and no reason asked. Deletion starts a 30-day window. During it, the practice is locked for everyone, and an Owner can restore it. We email each Owner 7 days before the window ends. When it ends, every client record is erased and the practice is closed.

**Credits when you leave.** Ask us for a refund of your unspent purchased Credits before you start deletion, as Refunds above says. A Credit still unspent when deletion finishes ends with the practice.

**What stays after deletion.** We keep our own records of your practice: its name and details, the Credits it bought and spent, the record of what was done and by whom, with each client's details made unreadable, and the record of when your practice agreed to these terms and to the Privacy Policy. We keep them for our accounts and taxes, and to answer a question or a dispute about your practice after it has gone. Your practice's Stripe account is its own, and deletion does not close it.

Nothing in these terms is there to make leaving cost you more.

## What your practice is responsible for

Your practice is responsible for the care it gives, for what it charges its clients, and for what it tells its clients about the records it keeps on them. DoulaCloud gives no medical advice, and it is not responsible for the care your practice gives.

Your practice decides what it records about its clients and who on its staff can see it. It must have the right to put that information into DoulaCloud, and it must answer a client who asks to see, correct or erase their record.

Each person at your practice must keep their sign-in details private. Tell us at once if you think someone else has used them.

## What DoulaCloud must not be used for

Do not use DoulaCloud to:

- break the law, or help someone else break it;
- store information about a person that your practice has no right to hold;
- send messages a person has not agreed to receive, or harass anyone;
- try to reach data that is not your practice's, or test, break or overload the security of DoulaCloud;
- resell DoulaCloud, or copy it.

If a practice does any of these, we can suspend or close its access. Unless the law or someone's safety stops us, we tell its Owners first and give them time to download their export.

## HIPAA and insurance billing

DoulaCloud is not a business associate under HIPAA, and it signs no business associate agreement today.

A practice that sends a claim electronically to Medicaid or to an insurer can become a covered entity under HIPAA. Tell us before your practice sends its first electronic claim, so that we can tell you what DoulaCloud can and cannot do for a covered entity. Until then, do not use DoulaCloud as if it were your business associate.

## When these terms change

We decide for each new version whether the change is material. A material change is a change to what your practice pays, what it gets, or what happens to its data. A corrected typing error is not.

For a material change, we email each Owner at least 30 days before the new version takes effect, and say what changes. After that date, the first Owner who signs in is asked to agree to the new version. Your practice works as usual until then.

A change never applies to a dispute, a claim or a purchase that came before it.

If you do not want to agree to a change, you can leave before it takes effect, with your export and a refund of your unspent purchased Credits.

A change that is not material, such as a clearer sentence or a new postal address, takes effect on the date of its version. The history below records every version.

## Warranty

We work to keep DoulaCloud running and your data safe, and we fix what goes wrong. We cannot promise that it will never be unavailable or never have an error. DoulaCloud is provided as it is, and we give no warranty beyond what these terms say, except where the law does not allow us to leave one out.

Keep your own copy of what matters to your practice. The export is there for that.

## Liability

Neither your practice nor we are responsible to the other for a loss that could not reasonably have been foreseen, or for lost profits or lost business.

Our total liability to your practice for all claims about DoulaCloud is limited to the amount your practice paid us in the 12 months before the claim arose, or $100, whichever is more.

These limits do not apply to a refund we owe you, or where the law does not allow them.

## Disputes

These terms are governed by the law of the State of New York.

A lawsuit about these terms or about DoulaCloud goes to the state or federal courts for Monroe County, New York, and your practice and we both agree to those courts.

Nothing in these terms takes away your right to go to court, on your own or together with others. Before either of us goes to court, we ask that you write to us first, so that we can try to put it right.

## Everything else

These terms and the [Privacy Policy](/privacy) are the whole agreement between your practice and us about DoulaCloud. If a court finds that part of them cannot be enforced, the rest stays in force. If we do not enforce a term at once, we have not given it up.

## Contact us

Email [hello@doula.cloud](mailto:hello@doula.cloud). That address reaches a person.

## Version history

1. October 3, 2026. Not a material change. Writes the name of the product as one word, DoulaCloud.
2. September 29, 2026. Not a material change. First version.

---

## Why each clause says what it says

One row per section, in page order. "Checked" names the code a sentence was read against, where one exists.

| Section | What the clause does | The record or source |
| --- | --- | --- |
| Opening | Names the parties. The agreement is with the Practice, made by its Owner, and not with each Staff member. "Doing business as" is left out because the assumed name is not filed. | ADR-0053 (a Practice agrees through its Owner); ADR-0054 (only an Owner agrees); #403 open. The entity is on the public register, and no formation document is in the repository. |
| Opening | States the date the version takes effect, and points at the history. | ADR-0053 (a version is a date). Printed from `site/src/lib/legal.ts`, the mirror of `api/internal/legal`. |
| What DoulaCloud is | Says what is being agreed to, and limits the offer to the United States. | ADR-0027, ADR-0031 and ADR-0033 put every non-US jurisdiction out of scope. |
| What a Credit costs | The fee disclosure the Stripe Connected Account Agreement asks for: "fees imposed by a Stripe Connect Platform … should be made clear to User in User's Platform Provider Agreement". One price, every feature, no subscription, no minimum, no recurring charge. | `signup-agreement-requirements.md` §1, quoting `stripe-platform-production-requirements.md` §3; ADR-0042 (one price, every feature). Checked: `docs/environment.md` "One credit costs $20.00", and the Stripe Price that is its only authority. |
| What a Credit costs | Credits do not expire, and the oldest are spent first. | `CONTEXT.md` **Credit**. |
| What a Credit costs | A rise is emailed 60 days ahead with its reason, never touches a bought Credit, and leaves the old price open until it starts. The 60 days are longer than the 30 days a material change gets, and the longer promise stands. | ADR-0042, "A price rise never touches a bought Credit" and its consequences. |
| What a Credit costs | Stripe takes the payment for Credits, and no card number reaches DoulaCloud. | Checked: the Credit purchase is a Stripe Checkout Session (`api/internal/billing`), and `docs/environment.md` "No PHI reaches Stripe". |
| Refunds | The refund rule, word for word as on `/pilot-terms` and the same in substance as the `/support` copy, under a heading that names it. The Terms are its binding home. | ADR-0042 ("The refund of an unspent Credit is the only guarantee"); #390; #444; ADR-0054. `terms.svelte.spec.ts` holds the two pages together. |
| Your practice is the merchant | The Practice is the merchant, and DoulaCloud holds no money. Written so as never to call DoulaCloud a payment facilitator, intermediary or aggregator. | Connect Terms §3.4(b), as `pilot-terms-page.md` applies it. Checked: `docs/environment.md` "DoulaCloud takes no per-transaction cut: there is no `ApplicationFeeAmount` anywhere". |
| Stripe | Names and links the Stripe Connected Account Agreement, says Stripe captures its acceptance, carries Stripe's model consent to share information, and links Stripe's Privacy Policy. | `signup-agreement-requirements.md` §1: Stripe's hosted onboarding captures the acceptance (verified in the Sandbox on #421), and "You must disclose that to your accounts by providing them with a link to that policy". ADR-0007 (hosted onboarding). |
| Leaving | Export as one archive readable without DoulaCloud, taken before deletion because the export route is not one of the deletion window's exemptions. | ADR-0048 (a Practice can leave at any time with everything she put in, in a form she can read without DoulaCloud; the restore screen says the archive comes first). Checked: `api/internal/export` writes one archive with a README. |
| Leaving | Deletion is the Owner's, from settings, with no call: a 30-day locked window an Owner can restore, a reminder 7 days before the end, then every Client erased. | ADR-0031. ADR-0048 refuses a call before deletion. |
| Leaving | Unspent Credits are forfeited when deletion finishes, so the Terms tell her to ask for the refund first. Without this sentence the Terms would promise a refund on leaving that the product does not give. | ADR-0031, "Finalization cascades every Client's own erasure, and forfeits the balance". |
| Leaving | What stays after deletion, and why. | ADR-0031 (the `practices` row is locked, not scrubbed; the ledger's forfeit row); ADR-0022 and ADR-0027 (the activity log stays, a Client's diffs shredded); ADR-0053 (the agreement row survives deletion). |
| Leaving | "Nothing in these terms is there to make leaving cost you more." | ADR-0048, which tests any contract term whose reason is to make leaving cost more. |
| What your practice is responsible for | The line between the tool and the care; the Practice's duty to its Clients about their records; keeping sign-in details private. | The Practice decides about a Client's record (ADR-0027: erasure is an Owner's act on a Client's request). |
| What DoulaCloud must not be used for | A short list, and the right to close a Practice that breaks it, with notice and time to export unless the law or safety forbids. | The time to export follows ADR-0048. |
| HIPAA and insurance billing | DoulaCloud is not a business associate and signs no BAA. A Practice tells DoulaCloud before its first electronic claim, which is the event that can make it a covered entity. Claims no BAA with any vendor. | ADR-0012; the resolution of #30 (45 CFR §160.103); `signup-agreement-requirements.md` §3. #546 is open, so no vendor BAA is named. |
| When these terms change | DoulaCloud decides materiality; the definition of material; 30 days' email notice; the first Owner at next sign-in is asked; the Practice works as usual. | ADR-0053. `changed-terms-notice-and-assent.md` §1 (_Douglas_: posting alone does not bind), Determination (direct notice, 30 days). |
| When these terms change | No retroactive effect on a dispute, a claim or a purchase; she can leave first with her export and her refund. No "at any time", and no duty to check the page. | `changed-terms-notice-and-assent.md` §4 (_Harris v. Blockbuster_: a change power with no savings clause, no notice period and no way to leave is illusory); §6 (Stripe's "check the page regularly" sentence is what _Douglas_ rejects). |
| Warranty | Plain words, no capitals-only paragraph, and the export named as her backup. | Spec #1556, "Warranty and liability, in plain words". |
| Liability | Excludes unforeseeable and consequential loss; caps liability at 12 months' fees or $100; never caps a refund owed. The $100 floor keeps the cap from being zero for a pilot Practice that has paid nothing yet. | Spec #1556. The refund carve-out keeps ADR-0042's guarantee whole. |
| Disputes | New York law; the state or federal courts for Monroe County; no clause that takes away a court or a claim brought together with others, said in words that never use the word "arbitration". | ADR-0054, decided by the founder on 2026-09-28. The spec fails if "arbitration" appears. |
| Everything else | Whole agreement with the Privacy Policy; severability; no waiver by delay. | Ordinary contract terms, kept short. |
| Contact us | An address that reaches a person. No postal address yet. | #402 open; when it lands, a new version that is not material adds the address. |
| Version history | Every version, its material flag and one line of what changed. | ADR-0053; `site/src/lib/legal.ts`. |
