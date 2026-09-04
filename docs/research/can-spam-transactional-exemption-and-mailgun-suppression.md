# CAN-SPAM's transactional exemption, and Mailgun's own suppression list

Research for GitHub issue [#731](https://github.com/markgoho/doula-cloud/issues/731) on map [#347](https://github.com/markgoho/doula-cloud/issues/347). Scoped to CAN-SPAM only (US-based, no stated international expansion — CASL/GDPR are out of scope).

## Bottom line

1. CAN-SPAM's opt-out mechanism, physical-address, and accurate-subject-line requirements apply only to a "commercial electronic mail message." A "transactional or relationship message" is a separate statutory category that these three requirements do not reach at all — this is not a conditional exemption a message can lose, it is a different bucket. The one requirement that reaches both buckets is accurate header/routing information (15 U.S.C. § 7704(a)(1)), which is unconditional regardless of category.
2. All six of Doula Cloud's Notification email types (all built, none merely filed — see the correction below) read as transactional or relationship content under the FTC's own multi-factor test — none reads as commercial.
3. Mailgun enforces its bounce/complaint/unsubscribe suppression list server-side, per sending domain, independent of the sending application. `api/internal/mail/mailgun.go`'s hand-rolled client sets no parameter that bypasses this (no such parameter exists in Mailgun's API) — every send made through it is already subject to Mailgun's suppression check. The sharper finding is on the read side: Mailgun's own docs confirm a suppressed send is dropped and reported as a "failed" event with severity "permanent" — the exact same shape as a genuine hard bounce — and #340's webhook handler (`bounce_webhook.go`) cannot tell the two apart, because it never parses the payload's `reason` field. A Client's spam complaint on the portal invite therefore gets recorded as an ordinary `bounced` row, indistinguishable from a typo'd address.

## 1. CAN-SPAM's transactional/relationship exemption

### Statutory definitions

**"Commercial electronic mail message"** — 15 U.S.C. § 7702(2)(A): "any electronic mail message the primary purpose of which is the commercial advertisement or promotion of a commercial product or service." § 7702(2)(B) excludes a transactional or relationship message from this definition outright — the two categories are mutually exclusive by statute, not overlapping with one taking precedence.

**"Transactional or relationship message"** — 15 U.S.C. § 7702(17)(A), quoted verbatim: a message whose primary purpose is —

- (i) "to facilitate, complete, or confirm a commercial transaction that the recipient has previously agreed to enter into with the sender;"
- (ii) "to provide warranty information, product recall information, or safety or security information with respect to a commercial product or service used or purchased by the recipient";
- (iii) "notification concerning a change in the terms or features of; notification of a change in the recipient's standing or status with respect to; or at regular periodic intervals, account balance information or other type of account statement with respect to, a subscription, membership, account, loan, or comparable ongoing commercial relationship";
- (iv) "to provide information directly related to an employment relationship or related benefit plan in which the recipient is currently involved, participating, or enrolled"; or
- (v) "to deliver goods or services, including product updates or upgrades, that the recipient is entitled to receive under the terms of a transaction that the recipient has previously agreed to enter into with the sender".

§ 7702(17)(B) lets the FTC amend this list by rule; the FTC has not narrowed it in a way relevant here (per its own compliance guide, cited below).

### What survives the exemption vs. what it removes

Read directly against 15 U.S.C. § 7704(a):

| Requirement | Statutory text scope | Applies to transactional/relationship mail? |
|---|---|---|
| No materially false/misleading header info — § 7704(a)(1) | "a commercial electronic mail message, **or a transactional or relationship message**" | **Yes — unconditionally.** This is the one rule that names transactional/relationship mail explicitly and holds it to the same standard as commercial mail. |
| No deceptive subject heading — § 7704(a)(2) | "a commercial electronic mail message" | No — text does not reach transactional/relationship mail. |
| Functioning opt-out mechanism — § 7704(a)(3) | "commercial electronic mail message" | No. |
| Clear identification as an advertisement/solicitation — § 7704(a)(4) | "commercial electronic mail message" | No. |
| Valid physical postal address — § 7704(a)(5) | "any commercial electronic mail message" | No. |

So the framing in the ticket — "does the exemption remove the unsubscribe requirement entirely" — understates it: opt-out, "ADV" labeling, and the physical-address line were never statutory requirements for this category to begin with, because § 7704(a)(3)–(5) are written to reach only "commercial electronic mail message," a term § 7702(2)(B) defines to exclude transactional/relationship mail by definition. The FTC's own compliance guide (below) describes this the same way: transactional/relationship mail "may not contain false or misleading routing information, but is otherwise exempt from most provisions of the CAN-SPAM Act."

Source: [15 U.S.C. § 7702](https://www.law.cornell.edu/uscode/text/15/7702) (Cornell LII, mirroring the U.S. Code), [15 U.S.C. § 7704](https://www.law.cornell.edu/uscode/text/15/7704), [FTC, "CAN-SPAM Act: A Compliance Guide for Business"](https://www.ftc.gov/business-guidance/resources/can-spam-act-compliance-guide-business).

### The multi-factor test for mixed content

A message can carry both commercial and transactional/relationship content. The FTC's implementing rule, 16 CFR § 316.3(a)(2), treats such a mixed message as commercial (losing the exemption) if either: a recipient reasonably reading the subject line would conclude the message advertises or promotes a commercial product or service, or the transactional/relationship content does not appear, in whole or substantial part, at the beginning of the message body.

For a message that mixes commercial content with *other* (non-transactional) content, § 316.3(a)(3)(ii) uses a broader reasonable-recipient test on the body, with factors including where commercial content is placed, what proportion of the message it occupies, and how much visual emphasis (color, graphics, type size) it gets.

Source: [16 CFR § 316.3](https://www.law.cornell.edu/cfr/text/16/316.3) (Cornell LII, mirroring the Code of Federal Regulations).

### Doula Cloud's six email types against this test

Every type is a Notification under ADR-0009 — link-only, content-free bodies naming no Client, Engagement, or Practice — which already keeps them out of any "advertising placement/emphasis" problem the multi-factor test is built to catch. Five recipients are a Staff member or Owner (Platform voice, per ADR-0009: Doula Cloud speaking as itself to its own customer); the Client portal invite alone goes to a Client (Practice voice: Doula Cloud speaking as the Practice). Subject and body text below are quoted verbatim from the committed source.

| Type | Voice | Status | Subject | § 7702(17)(A) category | Read |
|---|---|---|---|---|---|
| Client portal invite | Practice | Built (`api/internal/portalinvite/outbox.go:17`) | "You've been invited to view your care details online" | (v), plausibly (i) — delivers access to a service the Client's Engagement already entitles them to | Transactional. No commercial content of any kind in the body. |
| Staff invitation | Platform | Built (`api/internal/staffinvite/outbox.go:24`) | "You've been invited to join a practice on Doula Cloud" | (iv), plausibly (i)/(v) — the pilot includes contractor doulas (not only employees), so "employment relationship" is a fit but not the only one | Transactional. Same shape as the portal invite. |
| Out of Credits | Platform | Built (`api/internal/billing/outbox.go:18`) | "Doula Cloud: your Practice is out of Credits" | (iii) — status change on an ongoing account | Transactional. Body states the account condition and links to Billing to restore service; this is an account-status notice with a single remedial CTA, not a promotion for a different product — squarely the "change in ... standing or status" category, not the "advertisement placed at the top" pattern § 316.3(a)(2) polices. |
| Payout account incomplete | Platform | Built (`api/internal/payments/outbox.go:19`) | "Doula Cloud: your Practice's payout account needs more information" | (iii) — status of an ongoing account (the Practice's Stripe-connected payout account) | Transactional, same reasoning as Out of Credits. |
| Payment arrived | Platform | Built (`api/internal/payments/payment_outbox.go:21`) | "Doula Cloud: a Payment arrived" | (i)/(iii) — confirms a transaction/account event already part of the relationship | Transactional. Pure confirmation, no promotional content. |
| Security notices (sign-in, session revoked, MFA reset) | Platform | Built (`api/internal/sessionnotice/outbox.go:39,47,55`) | "Doula Cloud: new sign-in to your account" / "...your sessions were signed out" / "...your two-factor authentication was reset" | (ii) — security information about an account the recipient has | Transactional. Textbook fit for the (ii) category by name. |

**Correction to the ticket's framing:** the ticket (and map #347) describe Out of Credits (#342), payout account (#343), payment arrived (#344), and security notices (#345) as "filed but not built." All four are closed and shipped — `billing.lowCreditText`, `payments.payoutText`, `payments.paymentReceivedText`, and `sessionnotice.newSignInText`/`sessionRevokedText`/`mfaRecoveryClearedText` are live code with wired outbox workers and `tasknudge` registrations (`api/internal/tasknudge/tasknudge.go:25-33`). The assessment above is against the actual shipped copy, not the tickets' original sketches — worth flagging on map #347 since other open tickets may still be reasoning from the stale "filed, not built" state.

None of the six reads closer to commercial under the FTC's test: none advertises or promotes a product or service distinct from restoring/continuing the service the recipient already has, none places promotional content ahead of the transactional content (there is no promotional content to place), and every body is the fixed, content-free copy ADR-0009 already requires.

## 2. Mailgun's own suppression list

### What Mailgun maintains, and how it behaves

Mailgun's Help Center documents three suppression categories, populated automatically from delivery events on a domain, independent of any sending application: **bounces** (hard/permanent delivery failures such as a non-existent mailbox; soft bounces like a full mailbox are not added), **complaints** (addresses of recipients who marked a message as spam), and **unsubscribes** (addresses of recipients who used an unsubscribe mechanism).

Directly on the blocking behavior: "we internally block sending to these addresses so they don't harm your ability to keep sending." Suppressions are organized **per sending domain** ("since they're based on the proven interaction with a specific send"), not account-wide. This means the block is enforced by Mailgun itself at send time, before the message leaves Mailgun's infrastructure — it does not depend on the sending application checking anything first.

Confirming exactly what happens on the wire: Mailgun's Tracking Failures documentation states that "when the address is on one of the 'Do Not Send' lists because the recipient has previously bounced, unsubscribed, or reported spam, Mailgun will drop the message and stop trying to send it," and posts the resulting event to the account's `permanent_fail` webhook URLs — the same severity classification ("permanent") a real hard bounce gets.

Source: [Mailgun Help Center, "Suppressions (Bounces, Complaints, Unsubscribes) & Allowlists"](https://help.mailgun.com/hc/en-us/articles/360012287493-Suppressions-Bounces-Complaints-Unsubscribes-Allowlists), [Mailgun docs, "Tracking Failures"](https://documentation.mailgun.com/docs/mailgun/user-manual/tracking-messages/tracking-failures).

Management is exposed both in the dashboard and via API — list, add, delete a single address, or clear a whole category — under Mailgun's `bounces`, `complaints`, and `unsubscribes` API resources (e.g., `GET/POST/DELETE /v3/{domain}/bounces`, `/v3/{domain}/complaints`, `/v3/{domain}/unsubscribes`). Source: [Mailgun API reference, Complaints](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/complaints), [Add unsubscribes](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/unsubscribe/post-v3--domainid--unsubscribes), [Clear all bounces](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/bounces/delete-v3--domainid--bounces).

**One domain, one suppression list, all six email types.** ADR-0011 puts every Notification — Practice voice and Platform voice alike — on one shared `MAILGUN_DOMAIN`. Since suppression is per-domain, not per-notification-type, a Client who marks a portal invite as spam suppresses that address for *every* future Doula Cloud send to it: a later Staff invitation, a payout notice, a security notice, all Platform voice, all blocked the same way. There is no way to scope suppression to only the notification type that triggered it without a second sending domain — which #216/ADR-0011's own history (map #213) already treats as a bigger decision than this ticket reopens.

### No bypass parameter exists

Mailgun's `POST /v3/{domain}/messages` reference documents every `o:`-prefixed send option (`o:tag`, `o:dkim`, `o:deliverytime`, `o:testmode`, `o:tracking`/`o:tracking-clicks`/`o:tracking-opens`, `o:require-tls`, `o:skip-verification`, `o:sending-ip`, `o:suppress-headers`, and others). None of them controls or bypasses suppression-list checking — `o:suppress-headers` in particular is easy to mistake for this, but it only removes specific `X-Mailgun-*` headers from the outgoing message, unrelated to the bounce/complaint/unsubscribe lists. There is no documented way to force a send through to a suppressed address short of removing that address from suppression first (via the bounces/complaints/unsubscribes API or dashboard). Source: [Mailgun API reference, Send a message](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/messages/post-v3--domain-name--messages).

### Whether `api/internal/mail/mailgun.go` bypasses or honors this

It honors it — trivially, by never trying to do anything else. `MailgunSender.Send` (`api/internal/mail/mailgun.go:46-77`) builds its form body with exactly eight `form.Set` calls (`api/internal/mail/mailgun.go:47-55`): `from`, `to`, `subject`, `text`, `h:Reply-To`, `o:tracking`, `o:tracking-clicks`, `o:tracking-opens`. None of these touch suppression, and since no suppression-bypass parameter exists in Mailgun's API at all, there is no parameter this code could have set to skip the check even if it tried. Every send this client makes is subject to Mailgun's normal per-domain suppression enforcement, unmodified.

**What the code does with a suppressed send once Mailgun reports it.** `Send` treats any HTTP status `>= 300` on the initial `/messages` POST as a synchronous failure (`api/internal/mail/mailgun.go:72-75`); Mailgun's docs describe the suppression drop as happening asynchronously (the message is accepted, then dropped, then reported via the `permanent_fail` webhook), so the caller's immediate `Send` call most likely returns success either way — this repo's tests don't exercise that distinction and Mailgun's docs don't spell out the initial HTTP status for this case, so it's not confirmed first-party here. What *is* confirmed is what happens downstream: #340's webhook handler (`api/internal/portalinvite/bounce_webhook.go:24-36`) parses only `id`, `event`, `recipient`, and `severity` out of the event payload — it never reads a `reason` field. Mailgun's own webhook-payload documentation shows `reason` is a real field on a `failed` event (e.g., `"reason": "bounce"` for a genuine hard bounce, `"reason": "generic"` for a temporary failure), but this repo's struct at lines 30-35 drops it entirely. So even though Mailgun's suppression-caused drop and a fresh hard bounce may carry different `reason` values, `bounce_webhook.go:70` matches on `event == "failed" && severity == "permanent"` alone and writes the same `bounced` status for both — a suppression-caused drop (a Client's earlier spam complaint suppressing the address) is indistinguishable in `portal_invite_outbox` from a typo'd or dead address. That distinction, and whether it's worth carrying, is a question for #732, not answered here.

The other five outbox tables (`billing`, `payments` ×2, `sessionnotice`, `staffinvite`) have no equivalent webhook handler at all — #340/#340's bounce webhook only updates `portal_invite_outbox`. For those five, a suppression-caused drop is invisible end to end: Mailgun refuses to deliver, nothing in this codebase records that refusal anywhere, and the outbox row is left exactly as if delivery had succeeded.

## Sources consulted (primary)

- [15 U.S.C. § 7702](https://www.law.cornell.edu/uscode/text/15/7702) — CAN-SPAM definitions
- [15 U.S.C. § 7704](https://www.law.cornell.edu/uscode/text/15/7704) — CAN-SPAM requirements
- [16 CFR § 316.3](https://www.law.cornell.edu/cfr/text/16/316.3) — FTC primary-purpose rule
- [FTC, CAN-SPAM Act: A Compliance Guide for Business](https://www.ftc.gov/business-guidance/resources/can-spam-act-compliance-guide-business)
- [Mailgun Help Center: Suppressions (Bounces, Complaints, Unsubscribes) & Allowlists](https://help.mailgun.com/hc/en-us/articles/360012287493-Suppressions-Bounces-Complaints-Unsubscribes-Allowlists)
- [Mailgun docs: Tracking Failures](https://documentation.mailgun.com/docs/mailgun/user-manual/tracking-messages/tracking-failures)
- [Mailgun docs: Webhook Payloads](https://documentation.mailgun.com/docs/mailgun/user-manual/webhooks/webhook-payloads)
- [Mailgun API reference: Send a message](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/messages/post-v3--domain-name--messages)
- [Mailgun API reference: Complaints](https://documentation.mailgun.com/docs/mailgun/api-reference/send/mailgun/complaints)
- `api/internal/mail/mailgun.go`, `api/internal/mail/mail.go`, `api/internal/portalinvite/bounce_webhook.go`, `api/internal/portalinvite/outbox.go`, `api/internal/staffinvite/outbox.go`, `api/internal/billing/outbox.go`, `api/internal/payments/outbox.go`, `api/internal/payments/payment_outbox.go`, `api/internal/sessionnotice/outbox.go`, `api/internal/tasknudge/tasknudge.go`, `docs/adr/0009-notification-is-one-term-two-voices-keyed-by-recipient.md` (this repo, read directly)
