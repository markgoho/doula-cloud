# A Practice agrees through its Owner, and a change that matters asks again

A Practice agrees to the Terms of Service and the Privacy Policy when its Owner creates it. The form is one sentence directly above the button, with no checkbox. The record is one `activity` row. When a document changes in a way that matters, each Owner gets an email 30 days before the change starts, and after that date the first Owner who signs in is asked one time, on one screen. No other Staff member and no Client is ever stopped. Decided on [What a Practice agrees to at signup, and where the record is kept (#1494)](https://github.com/markgoho/doula-cloud/issues/1494), 2026-09-27, a ticket of [Practice onboarding: the route an Owner walks from signup to her first Engagement (#1487)](https://github.com/markgoho/doula-cloud/issues/1487).

The evidence is two research files, each on its own branch: `docs/research/signup-agreement-requirements.md` on `research/signup-agreement-requirements` ([#1490](https://github.com/markgoho/doula-cloud/issues/1490)) and `docs/research/changed-terms-notice-and-assent.md` on `research/changed-terms-notice-and-assent`.

## What she agrees to, and when

| Moment | What is agreed | Who captures it |
| --- | --- | --- |
| Signup | The Terms of Service and the Privacy Policy | DoulaCloud |
| Connecting Stripe | Stripe's Connected Account Agreement | Stripe, in its own flow |
| Buying a Credit | Nothing new | Nobody |

The Terms must disclose the price of a Credit, because Stripe's Connected Account Agreement obliges the platform to make its fees clear in its own terms. The price is thus agreed at signup, and a second agreement at purchase adds no term.

There is no marketing opt-in on the form. ADR-0043 refuses a series and ADR-0044 gives the site one ask.

## The form at signup

The sentence is: "By creating your Practice, you agree to the Terms of Service and the Privacy Policy." Each document name is a link that opens in a new tab, so that she does not lose the form. The sentence sits after the role notice and directly above the button ([#1493](https://github.com/markgoho/doula-cloud/issues/1493)).

The button stays **Create Practice**. The button carries `aria-describedby` pointing at the sentence, so a person who moves to the button with the Tab key hears the label and then the sentence.

There is no checkbox. The courts enforce a sentence that is joined to the button in place and in time (_Meyer v. Uber_), and GOV.UK's Check answers pattern uses a declaration sentence and no checkbox. This is not a departure from `docs/design/govuk-alignment.md`.

## The record

One `activity` row (ADR-0022), written in the transaction that creates the Practice:

| Column | Value |
| --- | --- |
| `subject_kind`, `subject_id` | The Practice |
| `action` | `terms_accepted` |
| `actor_kind`, `actor_staff_id` | `staff`, the Owner |
| `created_at` | The server's instant |
| `diff` | The version of the Terms, the version of the Privacy Policy, and the IP address the request came from |

A **version** is the date a document takes effect, for example `2026-10-15`. Each document has its own version. The IP address is read by `clientip.From`, the function that a Contract signature already uses.

An agreement to a later version writes the same row with the new version.

The row is plain text and stays for the life of the log. It survives the deletion of a Practice (ADR-0031) and the deletion of the Owner's login (ADR-0033), because the record of an agreement is needed most after the person who agreed has left. ADR-0033 is amended to name this one exception to "nothing to shred".

## When a document changes

DoulaCloud decides for each new version whether the change is **material**. A material change is a change to what a Practice pays, what she gets, or what occurs with her data. A corrected typing error is not material.

| | A change that is not material | A material change |
| --- | --- | --- |
| The document | Gets a new date | Gets a new date |
| Email | None | To each Owner, 30 days or more before the date |
| Screen | None | After the date, at an Owner's next sign-in |
| Record | None | The email in the outbox (ADR-0010); one `activity` row when she agrees |

### The screen

The screen has the sentence, the two links, one line that says what changed, and the button **Agree and continue**. It also has a link to export, a link to deletion, and a link to sign out. It has no "Not now".

The screen does not stand in front of export, deletion, or the refund of an unspent Credit. ADR-0048 refuses a gated export and refuses anything in the way of deletion.

In a Practice with more than one Owner, the first Owner who agrees settles it for the Practice. The other Owners do not see the screen.

### Only an Owner is asked

A Staff member who is not an Owner never sees the screen, and a Client never sees it. Between the date and the Owner's agreement, the Practice works as usual. A doula must never lose a Client's record during a birth because a different person did not press a button.

For that interval the agreement rests on the email and on the Practice's continued use. The research found this to be the minimum mechanism that binds, and found that it fails in court when the sender cannot prove the notice reached the person. The screen exists to replace that weaker proof with a record as soon as an Owner signs in.

### The version in force, and a signup during the 30 days

**Amended, 2026-10-23 ([#1816](https://github.com/markgoho/doula-cloud/issues/1816)):** a material version is in the history 30 days or more before it starts, so the newest version is not always the one in force.

- **The version in force** at an instant is the newest version that has started. A version starts at midnight UTC on its date, the same reading that gives the email its 30 days. In the United States that is the evening before the date. The BFF reads the instant through `clock.Now(ctx)` (`legal.Document.InForce`, `legal.Document.Coming`). The site reads the moment of its build (`versionInForce` in `site/src/lib/legal.ts`).
- **The text of each version has its own page**, at `/terms/<date>` and `/privacy/<date>`. `/terms` and `/privacy` show the version in force, and above its text a notice for each newer version: the date it takes effect, what changes, and a link to its page. The email links the new version's own page, not `/terms`. A text is kept once it is written, and the version history links each kept text, so the version a Practice agreed to stays one press away. A version's own page links `/terms` or `/privacy` as the place the version in force always is.
- **A Practice that signs up during the 30 days agrees to the version in force and to the coming version.** The sentence at signup links `/terms` and `/privacy`, and those pages show the coming version with its date and its change above the text in force. The email went out before this Practice existed, so it gets no email. The record shows what it agreed to instead, and #1549 does not show it the screen after the date. Told at signup with no record was rejected, because then the screen after the date would be the first record of its agreement. That is the weaker proof this ADR replaces with a record.
- **The `terms_accepted` diff** keeps `termsVersion` and `privacyVersion` as the versions in force. It adds `termsComing` and `privacyComing`, each a list of the dates of the versions that had not started, oldest first. Each key is left out when no version is coming. A Practice has agreed to a version when a `terms_accepted` row names it in either place. That is the rule the screen after a material change (#1549) reads. **Amended, 2026-10-24 ([#1549](https://github.com/markgoho/doula-cloud/issues/1549)):** a row that names a later version as the version in force has agreed to this one too, because the later text holds every change before it. Without this, a Practice that signs up after a material version and a later correction would be asked about the material version, which its row does not name. A later version in the coming list does not count, because that row was written before this version started. The screen asks only about the newest material version that was published, so a material version nobody was emailed about never hides an older one that was.
- **The site has no clock.** Every page is HTML built ahead of time and ships no JavaScript, so the version in force is decided at build time. The site's deploy workflow also runs each day soon after midnight UTC, so `/terms` moves to a new version within a day of its start. Every dated sentence on these pages says when a version takes effect, never that it is or is not in force yet. A page built the day before is thus still true. The browser's clock was rejected. It needs JavaScript for something the build can do, the reader's clock can be wrong, and a page with no script would still show the build's choice.

## The documents come first

The two documents do not exist. [Seed: the product's legal surface (#1505)](https://github.com/markgoho/doula-cloud/issues/1505) owns the text. No Practice signs up, a pilot Practice included, until both documents are live at `/terms` and `/privacy` on the marketing site. The sentence is not built to link a page that is not there.

## Considered and rejected

- **A checkbox.** It adds nothing in law and GOV.UK does not use one for a declaration.
- **The button label "Accept and create Practice".** #1490 proposed it to put the agreement into the control's name for a screen reader. `aria-describedby` does that job in HTML with no change to the label, and the sentence says "By creating your Practice", which is the act the label names.
- **A table of its own for agreements.** ADR-0022 exists so that a new record type does not make a new event table.
- **Posting a new version with no notice.** It does not bind her (_Douglas v. Talk America_).
- **Email only.** It depends on proof of delivery, and it is not sufficient for a privacy change that applies to data already collected: the FTC asks for affirmative consent there.
- **The screen with no email before it.** She learns of the change at the moment she must agree, with no time to export and leave.
- **Locking the Practice until an Owner agrees.** It stops a doula's work for an act that is not hers.
- **A "Not now" link.** It turns the screen back into email only.
