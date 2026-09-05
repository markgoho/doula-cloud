# No third-party open or click tracking in Notification email

[#734](https://github.com/markgoho/doula-cloud/issues/734) (map [#347](https://github.com/markgoho/doula-cloud/issues/347)) closed the question [#218](https://github.com/markgoho/doula-cloud/issues/218) left open when it provisioned `mg.doula.cloud`: the tracking CNAME was never installed, deliberately, because nobody had decided whether open and click tracking were worth a tracking pixel in mail sent to a Client in a Practice's voice.

**The tracking CNAME is not installed, and no Doula Cloud email carries open or click tracking, in either voice.** This is a standing rule over every mail kind on the shared domain ([ADR-0011](0011-notification-sending-identity-is-one-shared-domain.md)), not a fact about one provisioning task — today's eleven kinds and any kind written later.

**Click tracking's cost is the link itself.** A Notification is content-free and carries exactly one link into the product ([ADR-0009](0009-notification-is-one-term-two-voices-keyed-by-recipient.md)); that link *is* the message. Mailgun's own documentation says click tracking overwrites links so that they point at Mailgun's servers first. Turning it on would route every Client's portal invite through a third party, and record that Client's IP address, user agent and click time there. `mg.doula.cloud`'s `web_scheme` is also `http`, so the rewritten link would be plain HTTP until somebody changed it — a downgrade on the one link a Client is asked to trust.

**Open tracking would buy nothing it is even able to measure.** Mailgun's open tracking is a transparent `.png` in the HTML part, and its documentation is explicit that "text only emails will not track opens". `mail.Message` has a `Text` field and no HTML field, and `MailgunSender.Send` posts only `text`. An open number therefore costs an HTML body for all eleven mail kinds *before* the first data point, and even then it measures image loading, not reading.

**Nothing wants the data, and the data would not survive being wanted.** No feature in the code or the issue tracker asks for an open or click count; [#346](https://github.com/markgoho/doula-cloud/issues/346)'s Staff-visible column reports `bounced`/`complained`, which comes from the webhook ([#340](https://github.com/markgoho/doula-cloud/issues/340)), not from tracking. Mailgun's pricing page holds logs for 1 day on the Free and Basic plans, 5 on Foundation and 30 on Scale; on this account the events expire before anyone could read a trend from them, so any lasting number needs a store built for it as well.

**If the underlying question is ever asked, the answer is first-party.** "Did the Client act on the invite?" is a reasonable Staff question one day. It does not need Mailgun: the invite link points at a page Doula Cloud owns, and a hit on that page answers the same question with no pixel, no rewriting and no third party. That route is open; the third-party one is closed. It is the same posture [ADR-0016](0016-teaser-analytics-are-cookieless-and-the-channel-rides-on-the-form.md) took for the teaser, where measurement was wanted and was still built cookieless.

## Considered options

- **Install the CNAME and enable clicks only.** The cheapest way to get a real number, and rejected on what it costs to get it: every Client-facing link rewritten through a third party, personal data recorded there, an HTTP downgrade to fix first, a privacy disclosure to write with no page yet to put it on ([#363](https://github.com/markgoho/doula-cloud/issues/363) owns `/privacy`), and 1-day retention at the end of it.
- **Install the CNAME, enable both, and give all eleven kinds an HTML part.** The only shape in which open tracking works at all. Rejected as the largest build and the largest privacy cost, for a number no feature has asked for.
- **Leave it off, and record it only as a closing note on [#218](https://github.com/markgoho/doula-cloud/issues/218).** The same outcome, and the reason it was rejected is scope: a note on a provisioning ticket binds nothing, and a later session adding a twelfth mail kind would find no rule.

## Consequences

- **Delivery telemetry stays limited to what the webhook reports** — accepted, delivered, `bounced`, `complained`. There is no measure of whether a Notification was read or acted on, and none is planned.
- **The guard stays in two places, on purpose.** `mg.doula.cloud`'s domain settings have `open`, `click` and `unsubscribe` all inactive, and `MailgunSender.Send` sets `o:tracking`, `o:tracking-clicks` and `o:tracking-opens` to `no` on every message. Either alone would do; both mean a change in the Mailgun dashboard cannot silently turn tracking on.
- **Nothing to disclose.** There is no third-party tracking in Doula Cloud email, so `/privacy` gains no processor and no line from this decision — unlike [ADR-0016](0016-teaser-analytics-are-cookieless-and-the-channel-rides-on-the-form.md), which did.
- **This is fully reversible**, and cheaply: a DNS record and a per-message flag. What is not reversible is a Client's click that was already routed through a third party, which is why the default is off rather than on.

