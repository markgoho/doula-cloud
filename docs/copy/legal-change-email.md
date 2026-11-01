# The email before a material change to the Terms or the Privacy Policy

The copy below is the email each Owner of each Practice gets when DoulaCloud publishes a material version of the Terms of Service or the Privacy Policy, 30 days or more before the version takes effect, or 60 days or more when the version raises the price of a Credit. It was written on [A material change to the Terms or the Privacy Policy sends each Owner an email 30 days before it starts (#1548)](https://github.com/markgoho/doula-cloud/issues/1548), from [ADR-0053](../adr/0053-a-practice-agrees-through-its-owner-and-a-change-that-matters-asks-again.md) and the "When these terms change" section of [`terms-of-service-page.md`](terms-of-service-page.md). `api/internal/legalnotice/copy.go` holds it, and `TestCopy_MatchesTheCopyDocument` in `outbox_test.go` beside it fails when the two differ.

## Where it comes from

- The operator publishes the version in ops (#2486). A material version queues one row in `legal_notice_outbox` for each Owner of each Practice that is not deleted and not in its deletion window, the rule `legal_notice_owners()` (migration 00159) holds.
- One email for each Owner of each Practice. An Owner of two Practices gets two emails, one for each Practice, because each Practice agrees on its own (ADR-0053).
- Platform voice (ADR-0011): from `notifications@`, and a reply goes to `support@`, a monitored inbox. That is why "reply to this email" is how a refund is asked for: the Terms say "To ask for a refund, email us."

## Constraints the copy is written under

- **Transactional, and nothing else.** It is not a series (ADR-0043) and it carries no tracking (ADR-0030). No link in it carries a UTM tag: the rule that every doula.cloud link carries one is for outreach, and this email is a notice the Terms promise.
- **It says what she can do.** ADR-0053 rejects a screen with no email before it, because she must have time to export and leave. So the email names export, deletion and the refund of unspent purchased Credits (ADR-0048, ADR-0042), and says to ask for the refund before deletion starts, as the Terms' Leaving section does.
- **Voice** (`docs/design/brief.md`). "You" and "your" for the Owner who reads it, no pronoun for anyone else, and "practice" lowercase in a running sentence. The Practice's own name is printed, because an Owner of two Practices must know which one the email is about.
- **The one line of what changed is the version's own record**, `Version.Change` in `api/internal/legal`, the same line the page at `/terms` or `/privacy` prints. The email never writes a second summary.
- **The reason for a rise in the price of a Credit is the version's own record too**, `Version.PriceRise` in `api/internal/legal`, written in the same commit as the new text. The Terms' Pricing section promises it: "If the price of a Credit ever rises, we email each Owner at least 60 days before, with the reason." A version with a reason is a price rise, so the publish act refuses it fewer than 60 days before it takes effect, not 30 (#1817). A version without one prints no reason line.
- **Dates** are written as "November 30, 2026", from the version's date. A version is a date, not an instant, so no time zone is printed.

## The copy

`{Document}` is "Terms of Service" or "Privacy Policy". `{Date}` is the date the version takes effect. `{Practice}` is the Practice's name. `{Change}` is the version's one line. `{Reason}` is the reason the price of a Credit rises, and the line "Why the price rises: {Reason}" and the blank line after it are in the email only for a version that raises the price. `{DocumentURL}` is the new version's own page, `https://doula.cloud/terms/{version}` or `https://doula.cloud/privacy/{version}`, where `{version}` is the date it takes effect, for example `2026-12-01` (#1816): until that date `/terms` and `/privacy` show the version in force. `{SettingsURL}` and `{DeleteURL}` are the Practice's settings screen and its deletion screen in the app.

**Subject:** DoulaCloud: a new version of the {Document} starts on {Date}

**Body**, plain text, exactly as sent:

```text
Hello,

A new version of the DoulaCloud {Document} starts on {Date}. This email is about your practice, {Practice}.

What changes: {Change}

Why the price rises: {Reason}

Read the new version:
{DocumentURL}

From {Date}, the new version applies to {Practice}. The first Owner of {Practice} who signs in after that date is asked to agree.

If you do not agree, you can leave before {Date}:

1. Export your practice's data: {SettingsURL}
2. Ask for a refund of your unspent purchased Credits: reply to this email. We do not need a reason.
3. Delete your practice: {DeleteURL}

Ask for the refund before you start deletion. A Credit that is still unspent when deletion finishes ends with the practice.

If you have questions, reply to this email.
```

## Checked against the Voice rule

| Rule | How the copy meets it |
| --- | --- |
| 1, no pronoun for a third person | No "she", "her", "he", "they" for anybody. The other Owners are "the first Owner of {Practice}". |
| 4, "you" and "your" | Every line that speaks to the reader says "you" or "your". |
| 5, a label and one short line | The subject names the document and the date, and nothing else. |
| 6, said aloud | Each line is a sentence one person says to another. "Ask for the refund before you start deletion" is the Terms' own instruction. |
| 8, capitals | "Owner" and "Credits" are the names of things in the product; "practice" in a running sentence is lowercase, as the Terms write it. |
