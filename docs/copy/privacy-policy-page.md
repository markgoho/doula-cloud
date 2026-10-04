# `/privacy` — the Privacy Policy

The copy below is **final and verbatim**. It was written on [Publish the Terms of Service and the Privacy Policy at /terms and /privacy (#1556)](https://github.com/markgoho/doula-cloud/issues/1556), with [ADR-0054](../adr/0054-the-terms-hold-the-refund-rule-a-dispute-goes-to-court-and-only-an-owner-agrees.md) for the decisions no record held before 2026-09-28.

Built as `site/src/routes/privacy/+page.svelte` on `site/src/lib/components/ReadingPage.svelte`. The spec beside it asserts the sentences that do legal work by their words.

**Every claim about a company, a cookie or how long something is kept was checked against the code and the ADR that owns it before it was written.** The table under [Where each claim was checked](#where-each-claim-was-checked) names the source of each. A change to the product that makes a sentence false is a new version of this policy, in the same commit as the change.

## Where it lives, and how it is served

- URL: `https://doula.cloud/privacy`
- **Indexed and conspicuously posted.** No `noindex`, a canonical address, an entry in `sitemap.xml`, and a link from the site footer on every page. CalOPPA (Cal. Bus. & Prof. Code §22575) asks a site that collects information about a California resident to post its policy conspicuously; a Client can live in California (`signup-agreement-requirements.md` §3).
- Linked from the agreement sentence on `/signup` ([#1547](https://github.com/markgoho/doula-cloud/issues/1547)), from the Invitation screen ([#1557](https://github.com/markgoho/doula-cloud/issues/1557)) and from the Portal ([#1558](https://github.com/markgoho/doula-cloud/issues/1558)).
- Reachable with no password, no cookie and no JavaScript.

## Constraints the copy is written under

- **Each sentence is true of the product as built today** (ADR-0054). What the site does not run yet, a waitlist and a visit counter, the policy says it does not run.
- **A new version is three edits in one commit:** this document and the page, an entry in `site/src/lib/legal.ts`, and the same entry in `api/internal/legal/legal.go`.
- **A company is named before it receives data.** The teaser ([#358](https://github.com/markgoho/doula-cloud/issues/358)) adds Buttondown's form and Pirsch's script; the commit that adds them adds a version of this policy that names them, and says Pirsch holds its counts in Germany (ADR-0016).
- **No GDPR statement.** ADR-0027 puts every non-US jurisdiction out of scope.
- **No claim of a BAA with any vendor** while [#546](https://github.com/markgoho/doula-cloud/issues/546) is open.
- **The Client section is written to the Client**, in the second person, because she is the reader least likely to know that a company other than her doula holds her record.
- **The product name is a token**, through `PRODUCT_NAME`.
- **The version of October 2, 2026 is not a material change** ([#1595](https://github.com/markgoho/doula-cloud/issues/1595)). It adds GitHub and says how long Feedback is kept, which the policy's own rule ("A change to what happens to anyone's data is material") would make material for a Practice that had agreed to the version before it. No Practice had: DoulaCloud has not launched, and no Practice had agreed to the version of September 29 on the deployed app. A material change exists to email an Owner and ask her to agree again (ADR-0053), and with no Owner who agreed to anything, the version changes nothing anybody agreed to, which is the same reason `api/internal/legal`'s test gives for a first version. The version also takes effect the day it is published, because the page prints only the version in force and has no way to show one that is still pending. After launch, a version like this one is material.

---

# Privacy Policy

This policy says what Elephantine LLC, which operates DoulaCloud, holds about each person who uses DoulaCloud or visits this site: where it comes from, what it is used for, who else receives it, how long it is kept, and how you see it, correct it or have it erased. “We” and “us” mean Elephantine LLC.

This version takes effect on October 3, 2026. The history of every version is at the end of this page.

## Who decides about your data

A practice's records about its clients belong to the practice. The practice decides what it records about a client, who on its staff sees it, and when it is erased. We hold those records for the practice, and use them only to run DoulaCloud for it.

For the account of each person who owns or works at a practice, and for a visitor to this site, we decide, and this policy says how.

## What we never do

- We do not sell anyone's data.
- We do not use it for advertising, and DoulaCloud shows no advertisements.
- No email we send is tracked for opens or clicks. A message from your doula is only a message.
- We do not study what a named person does in DoulaCloud. The record of who changed what is there for the practice to read, and the only use we measure is how many Credits each practice bought and spent.

## If you own a practice

When you create a practice, and while you run it, we hold:

- your name and email address, and the password and any second sign-in factor you set. Google Identity Platform checks your password for us, and we do not store it;
- your practice's name, its time zone, and what else you enter about it;
- the Credits your practice buys and spends. Stripe takes the payment, and we never see your card number;
- your practice's Stripe account number, and whether Stripe says it can take payments;
- your practice's public page, if you publish one. It is public by design, and shows what you put on it, including the name and email address you give as its contact;
- the date, the time and the IP address when your practice agrees to our [Terms of Service](/terms) and to this policy;
- what you tell us through Feedback, with the name and version of your browser;
- a record of each change you make, with who made it and when.

We use it to run DoulaCloud for your practice, to charge for Credits, to email you about your practice's account, and to answer you.

## If you work at a practice

When a practice invites you to work with it, we hold your name and email address, your password and second sign-in factor as the section above says, the practices you work at and your role at each, and what you do at each: the visits you are given, the messages you write, and the changes you make, with when you made them. We also hold what you tell us through Feedback, with the name and version of your browser.

You are one person with one login, even if you work at more than one practice. Each practice sees your name, your email address and the work you do for it. It does not see the other practices you work at, or what you do there.

You can delete your login yourself. It is deleted at once, and your place at every practice ends. Your name and email address are replaced, and the work you did stays in each practice's records under “Deleted Staff Member”. If you are the last Owner of a practice, another person must become an Owner, or the practice must be deleted, first.

The Feedback you sent us stays when your login is deleted, and when a practice you work at is deleted. Once your login is deleted, your Feedback no longer names you. It is deleted 24 months after you sent it.

## If you are a client of a practice

Your doula's practice uses DoulaCloud to keep its records about you. This section is written to you.

Your practice decides what it records. That can include your name, contact details, address and date of birth; what it records about your pregnancy, birth and care; your visits; your plans, such as a birth plan; the contracts you sign; your invoices and payments; and the messages and files you and your doula send each other. When you sign a contract, we record the time and the IP address it was signed from.

You sign in to your practice's portal with a link we email to you. There is no password.

When you pay an invoice, the payment form comes from Stripe, and your card details go straight to Stripe. We never see or store a card number. Stripe receives your name and email address with each invoice, and collects information about your device to prevent fraud.

**To see, correct or erase your record, ask your practice.** It is your practice's record, and your practice decides. If your practice does not answer you, write to us at [hello@doula.cloud](mailto:hello@doula.cloud). We cannot change a practice's record ourselves, but we will send your request to your practice and ask it to answer you.

When your practice erases your record, your name, contact details, address, date of birth and the details it recorded about you in its own fields are removed, Stripe's customer record of you is deleted, and the history of changes to those details becomes unreadable. What was written in messages, contracts and notes stays, because those are your practice's own records of the care it gave and what it charged.

You can tell us what you think of DoulaCloud through Feedback in the portal. It comes to us, never to your practice. We keep what you send, with the name and version of your browser, and erase it when your practice erases your record or is deleted. If more than one practice keeps a record of you, Feedback you sent from a portal page that is not about one practice is erased when the last of them erases your record.

## If you visit this site

This site sets no cookies and runs no scripts. It does not count visits today. If it starts to, it will count them with a service that sets no cookie and cannot identify you, and this policy will name that service first.

Firebase Hosting, a Google service, delivers each page, and receives your IP address to do so.

This site has no waitlist form today. If you join the waitlist once it has one, your email address and first name are kept by Buttondown, the service that runs the list. After you confirm your address, the list writes to you once, when DoulaCloud opens in January 2027.

DoulaCloud does not follow you across other sites, so it treats every visit the same whether or not your browser sends a Do Not Track signal.

## Who else receives data

These companies receive data to run DoulaCloud, and each receives only what its job needs:

- **Google Cloud** runs DoulaCloud and stores all of its data, in the United States: the database, files such as contract documents and message attachments, and the jobs that send its email.
- **Google Identity Platform** checks the sign-in of each person who owns or works at a practice: the email address, the password and the second sign-in factor.
- **Firebase Hosting**, a Google service, delivers the DoulaCloud app and this site, and receives the IP address of each request.
- **Mailgun** sends DoulaCloud's email. It receives each address and message, and deletes its logs of them within 30 days.
- **Stripe** takes payments: a practice's payment for Credits, and a client's payment of an invoice. It receives what each payment needs, including a client's name and email address with an invoice, and it holds each practice's own Stripe account. Stripe processes data as [Stripe's Privacy Policy](https://stripe.com/privacy) explains.
- **Your browser's push service** — Google, Apple or Mozilla, depending on your browser — carries the signal that tells your device a new message is waiting. The signal says only that something is waiting, never what it is.
- **GitHub** keeps a note of each piece of Feedback in a private repository, so that we can sort it. The note holds only:
  - the kind of Feedback;
  - the kind of page it was sent from, never that page's address;
  - the version of the app, the width of the screen, and the name and version of the browser;
  - the time it was sent;
  - the role of the person who sent it, such as Owner or Client;
  - a link that only we can open.

  It never holds the words a person typed, a person's name, or a practice's name. The note names nobody, so it stays after the Feedback is erased or deleted.

A company is added to this list, in a new version of this policy, before it receives anyone's data.

## Cookies and storage on your device

DoulaCloud sets one cookie, `__session`, which keeps you signed in. It is used for nothing else, and it ends when you sign out or your session expires.

When a person who owns or works at a practice signs in, Google's sign-in library keeps a record of the sign-in in the browser's own storage until they sign out. The app also keeps a form you have not sent yet, such as a new client's intake, in the browser tab's own storage until the tab is closed.

This site sets no cookies.

## How long we keep data

- We keep a practice's records for as long as the practice uses DoulaCloud.
- A client's record is erased at once when the practice erases it, as described above.
- A practice's records are erased at the end of its 30-day deletion window, as the [Terms of Service](/terms) describe.
- A login is deleted at once when its owner deletes it, as described above.
- The database is backed up each day, and each backup is kept for 7 days. Something erased can stay in a backup for up to 7 days after it was erased, and is then gone.
- Mailgun deletes its logs of each email within 30 days.
- Feedback is deleted 24 months after it was sent, unless it is erased sooner as described above. GitHub's note of it names nobody, and stays.

Some records stay after a practice or a login is deleted: the practice's own name and details, the Credits it bought and spent, the record of what was done and by whom, with each client's details made unreadable, and the record of when an Owner agreed to the Terms of Service and to this policy. We keep them for our accounts and taxes, and because the record of an agreement is needed most after the person who agreed has gone.

## How we protect data

We keep a program of safeguards, as New York's SHIELD Act asks. Among them:

- every connection to DoulaCloud and to this site is encrypted;
- Google Cloud encrypts the data it stores;
- the database itself keeps each practice's records apart from every other practice's;
- each person who owns or works at a practice can add a second sign-in factor;
- the history of changes to a client's details is locked with a key of its own, which is destroyed when the record is erased.

## If there is a breach

If someone gets access to data they should not have, we tell each practice whose data is affected, without unreasonable delay, so that it can tell its clients. We also tell the people and the authorities that the law says we must.

## Children

DoulaCloud is for adults, and a child cannot have an account. A practice's record can name a baby. That information comes from the baby's parent or from the practice, and the parent can ask the practice about it as the section for clients describes.

## Where this policy applies

DoulaCloud is offered in the United States, and its data is stored there. This policy is written for the laws of the United States.

## When this policy changes

We decide for each new version whether the change is material, as the [Terms of Service](/terms) describe. A change to what happens to anyone's data is material. For a material change, we email each Owner at least 30 days before the new version takes effect.

A material change to how we use data we already hold applies to that data only after an Owner of the practice has agreed to it. Until then, that data keeps the promises this policy made when it was collected.

Every version is posted on this page, and the history below says what changed.

## Contact us

Email [hello@doula.cloud](mailto:hello@doula.cloud). That address reaches a person.

## Version history

1. October 3, 2026. Not a material change. Writes the name of the product as one word, DoulaCloud.
2. October 2, 2026. Not a material change. Says how long Feedback is kept, what erases it, and that GitHub keeps a note of each piece.
3. September 29, 2026. Not a material change. First version.

---

## Where each claim was checked

One row per claim that names a company, a cookie, a store on the device, or how long something is kept, and one per clause that does legal work. "Checked" is the code the sentence was read against on 2026-09-29, or on 2026-10-02 for the rows about Feedback, which [#1595](https://github.com/markgoho/doula-cloud/issues/1595) added.

| Section | Claim | Checked against |
| --- | --- | --- |
| Who decides about your data | The Practice decides about a Client's record, and DoulaCloud holds it for her; DoulaCloud decides about account and visitor data. | ADR-0027 (erasure is the Practice's act); ADR-0033 (a login is the person's own). |
| What we never do | No sale, no advertising. | Nothing in `api/` or `app/` sends data to an advertiser; ADR-0046. |
| What we never do | No open or click tracking in any email. | ADR-0030. Checked: `mail.MailgunSender.Send` posts only a `text` part, so there is nothing a tracking pixel could ride in, and the tracking CNAME is not installed. |
| What we never do | No study of a named person's use; the only measure is Credits bought and spent. | ADR-0046, "the app counts nothing about anyone". |
| If you own a practice | Identity Platform checks the password, and DoulaCloud does not store it. | ADR-0026. Checked: the `staff` table holds `identity_uid`, `name` and `email`, and no password column exists. |
| If you own a practice | Stripe takes the Credit payment; no card number reaches DoulaCloud. | `docs/environment.md` "No PHI reaches Stripe": the Credit flow sends a Practice id and a count. |
| If you own a practice | The Practice Page is public and carries a named contact. | `GLOSSARY.md` **Practice Page**; `site/src/routes/p/[slug]/+page.svelte`. |
| If you own a practice | The date, time and IP address of the agreement. | ADR-0053: one `activity` row with both versions and the IP address from `clientip.From`. Built by #1547. |
| If you own a practice | Feedback keeps the browser's name and major version, never the raw header. | `api/internal/feedback/browser.go`, `ParseBrowser`. |
| If you own a practice | A record of each change, with who and when. | ADR-0022; `CLAUDE.md`'s audit-trail expectation. |
| If you work at a practice | One person, one login across Practices; each Practice sees only its own work with her. | ADR-0008; ADR-0015; ADR-0033. The database refuses a read across Practices: row-level security on every Practice table, proved by `api/internal/rlsguardrail`. |
| If you work at a practice | Login deletion is immediate, replaces the name and email, keeps the work under "Deleted Staff Member", and refuses the last Owner. | ADR-0033. Checked: `api/internal/staffauth/logindeletion.go`. |
| If you work at a practice | A Staff member can send Feedback, kept with the browser's name and version. | `staffauth.FeedbackHandler`, mounted in `api/internal/staffauth/mount.go`; `GLOSSARY.md` **Feedback** ("any Staff member at any role"); `api/internal/feedback/browser.go`, `ParseBrowser`. |
| If you work at a practice | A Staff member's Feedback stays after her Login deletion and after a Practice's Deletion, names nobody once her login is deleted, and is deleted 24 months after it was sent. | `GLOSSARY.md` **Feedback** ("A Staff member's Feedback survives her Login deletion and then names nobody"), decided on #1501 Q3. Checked: `api/internal/staffauth/logindeletion.go` touches no `feedback` row, and the `staff` row the piece points to has its name and email replaced; `api/internal/practicedeletion/outbox_test.go`, `TestWorker_FinalizeDestroysClientFeedbackAndLeavesStaffFeedback` ("Staff items survive it"); `feedback.RetentionMonths`. |
| If you are a client of a practice | What a Practice can record about her. | `GLOSSARY.md` **Client**, **Engagement**, **Visit**, **Birth Plan**, **Message**; ADR-0017 (the Practice-defined layer). |
| If you are a client of a practice | A signed Contract records the time and the IP address. | ADR-0053 names `clientip.From` as the function a Contract signature already uses. |
| If you are a client of a practice | The portal signs in by an emailed link, with no password. | ADR-0026, "Magic link, minted by the BFF". |
| If you are a client of a practice | The payment form is Stripe's; Stripe receives her name and email with an invoice and collects device information against fraud. | `docs/environment.md` "No PHI reaches Stripe": "The invoice flow sends a Client's name and email". The portal Payment Element loads Stripe.js (#1020), which collects device signals for Stripe's fraud checks. |
| If you are a client of a practice | Ask the Practice first; DoulaCloud forwards a request the Practice leaves unanswered. | Spec #1556, "The Client section is written to the Client". |
| If you are a client of a practice | What erasure removes, that Stripe's Customer is deleted, that her history of changes becomes unreadable, and that free text stays. | ADR-0027 (redact in place; crypto-shredding; free text is not scrubbed; the Stripe Customer is deleted). The Medicaid hold in ADR-0031's amendment is not built (#335), so the policy does not describe it; the version that builds it adds it. |
| If you are a client of a practice | A Client can send Feedback from the portal; it goes to DoulaCloud, never to her Practice. | `clientauth.FeedbackHandler`, mounted as `POST /api/portal/feedback` in `api/internal/clientauth/mount.go` ("any signed-in Client sends a piece of Feedback from any portal screen"); `PortalFeedback` in `app/src/routes/portal/(authenticated)/+layout.svelte`. `GLOSSARY.md` **Feedback**: "no Staff member reads it and a Practice's export leaves it out". |
| If you are a client of a practice | Her Feedback is erased when her record is erased, and when her Practice is deleted; a piece sent from a portal page with no single Engagement goes when the last Practice with a record of her erases it. | `client.Erase` calls `eraseFeedback` (`erase_client_feedback`, 00121) for the record and for each record merged into it (`api/internal/client/erase.go`); the Portal Account branch calls `erase_portal_account_feedback` only when `portal_account_reaches_a_live_client` is false. Practice Deletion reaches both through the same cascade: `api/internal/practicedeletion/outbox_test.go`, `TestWorker_FinalizeDestroysClientFeedbackAndLeavesStaffFeedback`. Decided on #1525 and #1501 Q3. |
| If you visit this site | The site sets no cookies, runs no scripts, and counts no visits today. | `site/src/routes/+layout.ts` (`csr = false`); no analytics tag anywhere under `site/src`. ADR-0016 names Pirsch for when it does, cookieless. |
| If you visit this site | Firebase Hosting delivers each page and receives the IP address. | `firebase.json` hosting targets. |
| If you visit this site | No waitlist form today; Buttondown keeps the list once there is one, and the list writes once, in January. | ADR-0014 (Buttondown; one confirmation, one January broadcast). No form exists under `site/src`. |
| If you visit this site | Do Not Track. | CalOPPA §22575(b)(5) asks a policy to say how the site responds to a Do Not Track signal. |
| Who else receives data | Google Cloud runs the service and stores all data in the United States. | `terraform/cloud_sql.tf` (`region = "us-central1"`); `docs/environment.md` (the attachments bucket in `us-central1`; the Cloud Tasks queue). |
| Who else receives data | Google Identity Platform checks each Staff sign-in. | ADR-0026. |
| Who else receives data | Firebase Hosting delivers the app and the site. | `firebase.json`. |
| Who else receives data | Mailgun sends every email and deletes its logs within 30 days. | ADR-0012; ADR-0030 (logs held 1 to 30 days by plan); ADR-0027 ("Mailgun self-purges within 3–30 days"). |
| Who else receives data | Stripe takes both kinds of payment and holds each Practice's own account. | ADR-0007; ADR-0032; `docs/environment.md` "Stripe: two surfaces, one key". |
| Who else receives data | The browser's push service carries a signal with no content. | ADR-0002 (a content-free Web Push wakes the service worker, which fetches the content from the BFF). |
| Who else receives data | GitHub keeps a note of each piece of Feedback in a private repository: its kind, the route pattern, the app build, the screen width, the browser, the time sent, the role, and a link only we can open; never the words typed, the full address, the sender or the Practice. The note stays after the piece is erased or deleted. | The outbox that files it (#1524) is built: `api/internal/feedback/issue_outbox.go`, `issueTitle` (kind and route pattern only) and `issueBody` (the field list #1501 Q1 settled; `issueClaimQuery` never selects the free text). The route pattern is SvelteKit's `route.id`, which the app sends and which never holds a resolved id; the BFF forwards it unchecked today, and [#1697](https://github.com/markgoho/doula-cloud/issues/1697) makes it refuse one that is not a pattern. The link opens the founder read page, `/feedback/<id>`, which only the founder can read (00122, `feedback_reader()`). An Erasure closes the issue and labels it `erased` (`erasedLabel`, #1525); retention leaves it untouched (`deleteExpired`). This row replaced, in the version of October 2, 2026, the first version's row that said GitHub was not on the list because the outbox was not built. |
| Cookies and storage on your device | One cookie, `__session`, ending at sign-out or expiry. | ADR-0004; ADR-0026 ("the same `__session` cookie"); `api/internal/authn/authn.go` (the cookie's `MaxAge` is the session's lifetime). |
| Cookies and storage on your device | Google's sign-in library keeps the sign-in in browser storage until sign-out. | `app/src/lib/firebase.ts` calls `getAuth`, whose default persistence is the browser's local storage; `signOut` clears it. |
| Cookies and storage on your device | An unsent form stays in the tab's storage until the tab closes. | `app/src/lib/intakeDraft.svelte.ts` and `app/src/lib/engagementRequest.ts` use `sessionStorage`. |
| How long we keep data | Daily backups, each kept 7 days. | `terraform/cloud_sql.tf`: `backup_configuration` enabled, `retained_backups = 7`. ADR-0027 states that shredding is not retroactive across the backup window. |
| How long we keep data | Feedback is deleted 24 months after it was sent, unless erased sooner; GitHub's note stays. | `feedback.RetentionMonths = 24` and `IssueWorker.deleteExpired` in `api/internal/feedback/issue_outbox.go`, which compares against `sent_at` and queues nothing for the issue. Decided on #1501 Q4; built by #1525. |
| How long we keep data | What stays after deletion, and why. | ADR-0031; ADR-0033; ADR-0053 (the agreement row survives both). |
| How we protect data | A safeguards program. | New York General Business Law §899-bb (SHIELD Act), `signup-agreement-requirements.md` §3. |
| How we protect data | Encrypted connections; encrypted storage; each Practice kept apart by the database; a second factor; a Client's history locked with its own key. | HTTPS on Firebase Hosting and Cloud Run; Google Cloud's default encryption at rest; `api/internal/rlsguardrail`; ADR-0026 (TOTP); ADR-0027 (`client_data_keys`). |
| If there is a breach | The Practice is told, so that it can tell its Clients; the law's other notices are given. | New York General Business Law §899-aa: a business that holds data it does not own tells the owner. |
| Children | For adults; a record can name a baby. | ADR-0015 (the baby on an Engagement). |
| Where this policy applies | United States only, with no GDPR statement. | ADR-0027, ADR-0031 and ADR-0033 put every non-US jurisdiction out of scope. |
| When this policy changes | 30 days' notice of a material change; a material change to data already held applies only after an Owner agrees. | ADR-0053; `changed-terms-notice-and-assent.md` §7 (FTC, _Gateway Learning_, and its 2024 restatement). |
| Contact us | An address that reaches a person. | Spec #1556. |
