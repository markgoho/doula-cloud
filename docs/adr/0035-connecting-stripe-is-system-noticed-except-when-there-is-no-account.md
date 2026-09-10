# An unfinished Stripe account is system-noticed; an absent one is a colleague's ask

[#917](https://github.com/markgoho/doula-cloud/issues/917) came out of [#270](https://github.com/markgoho/doula-cloud/issues/270)'s grilling with one question left open and deliberately unanswered: when Dee Whitlock, an Admin at a fourteen-doula agency, opens the Payments settings screen and reads that Stripe is not connected, is telling Renata about it *a control she presses* or *a condition the product notices on its own*? The ticket asked for both readings to be weighed, and named [#343](https://github.com/markgoho/doula-cloud/issues/343)'s payout outbox as the precedent to look at first.

Looking at it settles most of the question, because **the system-noticed half is already built** and #917 was written without it in view.

## What already notices

`PostAccountWebhookHandler.handleCapabilityStatusUpdated` queues a `payout_outbox` row on every `stripe_connect_requirements_due` empty → non-empty transition, and `payments.Worker` mails every Owner the Practice currently has — one email per episode, after a 48-hour grace window, skipped entirely if the Owner finished inside that window, re-armed only when requirements clear and reappear (`00034_payout_outbox.sql`). That is a Platform Notification in ADR-0009's voice, with ADR-0011's shared sending identity, resolving recipients at send time and storing none of them on the row.

It covers `onboarding_incomplete`, `pending`, and `payouts_restricted` — every state in which Stripe has actually asked the Practice for something. In those states an Admin's "nudge" would be a second copy of an email the Owners already have, sent by a person who cannot see that the first one went. So the answer there is not a control at all: it is telling her the truth, which is that the Owners have already been emailed.

## What cannot notice, and why no sweep fixes it

`not_connected` is the one status the webhook can never reach. There is no Stripe account, so no `capability_status_updated` event is ever delivered for that Practice, and `payout_outbox` has nothing to fire on. The one act that touches the condition is raising a Stripe-rail Invoice, which `createStripeInvoice` refuses with `errClientsCannotPay` — and that refusal reaches the Doula who tried, never an Owner.

The obvious repair is a scheduled sweep that looks for Practices sitting unconnected and mails their Owners. [ADR-0033](0033-overdue-is-derived-and-notifies-nobody.md) already refused exactly this shape and its reasoning carries here unchanged: *"every outbox is nudged by an act through `tasknudge`, and a due date passing is not an act"*. A Practice not having connected Stripe is not an act either. Building the first time-based sweep in the product for this would also make the product the author of a chase nobody asked for, which is the half of ADR-0033 that is about judgment rather than mechanism.

## The decision

**A Staff member's act, offered only in the state the system cannot notice.**

- In `not_connected`, a reader who may read Connect status and may not act on it is offered one control. Pressing it queues a Platform Notification to every current Owner through ADR-0010's outbox, nudged by ADR-0013.
- In every other unconnected status, there is no control, and the screen says the Owners have already been emailed.

This is not a departure from [ADR-0028](0028-the-shell-has-no-notification-bell.md) or ADR-0033. Both refuse **the product chasing on its own initiative** — a bell, a badge that follows a person around, a timer nobody can see. A single bounded email that a colleague deliberately chose to send is a Staff act with a name attached to it, and #343 already established that a Practice's Owners receive Stripe reminders in Platform voice. Nothing durable and per-recipient is created, so ADR-0028's "there is no feed to read" still holds and no read scope is opened.

### Recipients are not re-decided

Every Owner the Practice currently has, resolved at send time from `staff`/`practice_memberships`, never stored on the row. That is #343's recorded rule and `payments.ownerEmails` is its implementation; #917's "all of them, or the one who created the account, or the sender's choice" is a question the precedent has already answered, and a second answer would mean two rules for who counts as an Owner of a Practice. A Practice with no Owners left is marked sent with nothing mailed, exactly as `SendAll` already handles for the payout mail.

### The bound is a cooldown, not an episode

#343 bounds repeats with "one per episode", where an episode begins and ends with a webhook. This has no webhook, so it has no episode to re-arm against, and the honest translation of "one per episode" would be "once, ever" — which turns a second ask a fortnight later, an entirely reasonable thing for a colleague to want, into a permanent refusal.

So the bound is **one nudge per Practice per week, counted across every sender**, refused at the act with a sentence that names the rule. Across senders rather than per sender, because what the bound protects is the Owner's inbox and eleven Admins asking once each is eleven emails. Evaluated against Postgres's clock at the moment of the press, so it is still a bound arrived at by an act — no timer, no sweep, nothing that fires on its own. A pending row is additionally held to one per Practice by a partial unique index, which is the concurrency guard rather than the rule.

Seven days is a judgment and is recorded as one: long enough that a second ask reads as *this is still not done* rather than as nagging, short enough that a Practice that cannot invoice at all is not stuck for a month.

### The email names nobody, and the ledger names everybody

ADR-0009's content rule as corrected by ADR-0011 is unconditional: no Practice name, no Client detail, nothing identifying, in `From`, subject, or body. The **sender** is covered by that rule too, so the mail says "someone at your Practice" and stops. Who asked is not lost — it is written twice, on `connect_nudge_outbox.requested_by_staff_id` where the worker can see it, and as an `activity` row against the Practice with the Staff member as actor, where the Practice reads its own history behind its own permission gate. That is CLAUDE.md's audit-trail expectation answered where it belongs rather than in an email that cannot hold it.

That entry's action is `stripe_connect_nudge_requested`, not `..._sent`. The transaction it is written in proves an ask, not a delivery — ADR-0010's outbox is exactly the gap between those two — and a row that later dead-letters, or that the worker marks sent with nothing mailed because the Practice connected first, must not leave a ledger entry claiming an email went out.

**Whom it went to is recorded too**, on `connect_nudge_outbox.notified_owner_staff_ids`, written by the worker at the moment it resolves the roster. Recipients are still resolved at send time and never chosen at queue time — that is #343's rule, unchanged — so this records what the resolution produced rather than instructing it. It is not redundant with the rule itself, because the rule is evaluated against a roster that moves: an Owner who leaves next month is nowhere in a re-run of that query, and "every Owner at the time" would then be unanswerable. Staff ids rather than addresses, because the ledger already resolves an id to a name and an address on an append-only row would outlive the person changing it. A Practice with no Owners records an empty array, which is a different fact from a row no attempt has reached yet.

### The send rechecks the condition

A retry after a Mailgun failure can be up to a day later (ADR-0010's backoff). `ConnectNudgeWorker` re-reads `practices.stripe_connect_account_id` at send time and marks the row sent with nothing mailed if the Practice connected in the meantime — the same shape `requirementsStillOutstanding` gives #343's own worker, so no Owner is ever told to do something already done.

## Considered and rejected

- **Copy naming the Owners, and no send at all.** The Payments screen could name each Owner and her email address — the staff roster is behind the same `OwnerAndAdmin` gate as Connect status, so nothing new would be exposed. It is the cheapest thing that could work and the Rule of Least Power favors it. Rejected because it makes the bottleneck legible without removing it: Dee still has to leave the product, compose the message herself, and remember to. #917 calls that the defect, not the fix.
- **A control in every unconnected status.** Rejected: #343 owns the states a webhook can reach, and a second sender there is duplicate mail an Admin has no way to know she is sending.
- **A scheduled sweep over unconnected Practices.** Rejected on ADR-0033's own reasoning, quoted above.
- **Naming the sender in the email.** Rejected: ADR-0009's content rule carries no exception for a colleague's name, and the audit trail is a better home for the fact.
- **Refusing an Owner at the boundary.** Rejected as a gate shape that earns nothing. An Owner nudging herself is a pointless errand, not a permission violation; the screen never offers her the control, and the boundary already enforces the two things that matter — that the Practice really is unconnected, and that nobody asked this week.
