# The Terms hold the refund rule, a dispute goes to court, and only an Owner agrees

The Terms of Service and the Privacy Policy are live at `/terms` and `/privacy` on the marketing site. [ADR-0053](0053-a-practice-agrees-through-its-owner-and-a-change-that-matters-asks-again.md) decided how a Practice agrees to them and how a change asks again. It left the text to [Seed: the product's legal surface (#1505)](https://github.com/markgoho/doula-cloud/issues/1505). On 2026-09-28 the founder chose a spec in place of a charting session, [Publish the Terms of Service and the Privacy Policy at /terms and /privacy (#1556)](https://github.com/markgoho/doula-cloud/issues/1556), and answered the four questions that no record settled. This ADR records those four answers. The text itself, and the record behind each clause, is in [`docs/copy/terms-of-service-page.md`](../copy/terms-of-service-page.md) and [`docs/copy/privacy-policy-page.md`](../copy/privacy-policy-page.md).

## A dispute goes to court

New York law applies. A lawsuit goes to the state or federal courts for Monroe County, New York. The Terms have **no arbitration clause and no class-action waiver**, and they say in plain words that nothing in them takes away a Practice's right to go to court, on its own or together with others.

The founder decided this on 2026-09-28, on #1556, and the ticket records the decision without a stated reason. What the record around it shows: an arbitration clause is the term at issue in most of the cases the changed-terms research read (_Douglas v. Talk America_, _Harris v. Blockbuster_, _Starke v. SquareTrade_), and [ADR-0048](0048-nothing-holds-a-practice-here-but-the-next-family.md) says that a contract term whose reason is to make leaving or disputing cost a Practice more is tested against it. The courts for Monroe County are where Elephantine LLC is.

The spec beside `/terms` fails if the word "arbitration" appears on the page. A future clause that adds one is a superseding ADR, not an edit.

## A Staff member and a Client agree to nothing

Only an Owner agrees, for the Practice, at signup and after a material change (ADR-0053). A Staff member who is not an Owner and a Client never agree to either document. The Privacy Policy covers each as a person whose data is held, in a section written to each, and each is given a link to it:

| Person | Where the link is | Built by |
| --- | --- | --- |
| A Staff member | The Invitation screen | [#1557](https://github.com/markgoho/doula-cloud/issues/1557) |
| A Client | The Portal | [#1558](https://github.com/markgoho/doula-cloud/issues/1558) |

The founder decided this on 2026-09-28, on #1556. The reason on record is ADR-0053's: a doula must never lose a Client's record during a birth because a different person did not press a button. A Staff member is not a party to the Practice's agreement, and a Client is not DoulaCloud's customer at all. A notice is what each is owed, and a link is a notice that stops nobody.

## The Terms are the binding home of the refund rule

The refund rule is in three texts: the `/support` copy ([#390](https://github.com/markgoho/doula-cloud/issues/390)), `/pilot-terms` ([#444](https://github.com/markgoho/doula-cloud/issues/444)), and now the Terms. The Terms are the one a Practice agrees to, so they are the binding home. The three say the same thing, and a change to one is a change to all three. `/terms` and `/pilot-terms` say it in the same words. `/support` says it in words of its own, fixed on #390 so that each sentence answers a Stripe or New York requirement, and its copy document is final.

The Refunds section of `/terms` is word for word the Refunds section of `/pilot-terms`, and `site/src/routes/terms/terms.svelte.spec.ts` fails when the two differ. `/support` ([#358](https://github.com/markgoho/doula-cloud/issues/358)) has its own words, so no test compares it word for word; its spec asserts the three-year window and "We do not need a reason" by their words, and a reader of the three copy documents checks that they agree in substance.

The rule has no precedent in the field: eight of nine surveyed products refuse a refund on an unspent prepaid balance. It is [ADR-0042](0042-a-practice-pays-per-engagement-and-doula-cloud-is-paid-when-the-doula-is.md)'s one guarantee, and the Terms state it as it is.

## The agent writes the text, and each sentence is true today

The agent that built #1556 wrote both documents from the two research files and the ADRs, and deferred no clause to a lawyer. Each sentence describes what the code and the records show today. A sentence about a company, a cookie or a retention period was checked against the code and the ADR that owns it before it was written, and the copy document names that source. A sentence that could not be checked was not written.

Two consequences follow, and both are recorded here so that a later reader does not take them for gaps:

- **The site has no waitlist form and counts no visits today**, so the Privacy Policy says so, and says what happens when each arrives ([ADR-0014](0014-the-waitlist-lives-outside-our-stack.md), [ADR-0016](0016-teaser-analytics-are-cookieless-and-the-channel-rides-on-the-form.md)). The teaser ([#358](https://github.com/markgoho/doula-cloud/issues/358)) adds Pirsch to the list of companies in a new version of the Privacy Policy, in the same commit that adds the script. Pirsch's servers are in Germany, so that version also says where visit counts are held.
- **The Terms name no postal address and no assumed name**, because the mailbox ([#402](https://github.com/markgoho/doula-cloud/issues/402)) and the Certificate of Assumed Name ([#403](https://github.com/markgoho/doula-cloud/issues/403)) have not landed. When each lands, the Terms get a new version that is not material.

## The first version is not material

The first version of each document takes effect on 2026-09-29 and is marked not material. No Practice agreed to an earlier version, so there is nobody to email and nobody to ask again. #1548 and #1549 read the flag and do nothing for it.

## Where the version lives

The version history of each document is in two places: `site/src/lib/legal.ts`, which the pages print, and `api/internal/legal`, which the BFF reads to record an agreement (#1547), to send the email before a material change (#1548), and to ask after it (#1549). They are a mirrored pair, like the site's origin, and `site/src/lib/legal.spec.ts` reads the Go source and fails when the two histories differ in any version's date, material flag or line of what changed.

## Considered and rejected

- **Arbitration with a small-claims carve-out**, the common software-company form. Rejected for the reason above: its purpose is to keep a customer out of court.
- **A checkbox or a click-through for a Staff member or a Client.** Rejected: each would stop a person who is not a party, for an agreement that is not theirs.
- **The refund rule only on `/support`, with the Terms pointing at it.** Rejected: the Terms are what a Practice agrees to, so a rule that binds has to be in them, and a pointer to a page that can change on its own is a second document to agree to.
- **Writing the Privacy Policy for the teaser as it is planned**, naming Buttondown and Pirsch as if they were live. Rejected: a policy that describes a service the site does not run is a template, not a description.
