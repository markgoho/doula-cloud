# Practice deletion is a thirty-day window, then the erasure cascade

A Practice Owner who signs up, evaluates, and decides against Doula Cloud has no way to remove the Practice. [ADR-0027](0027-erasure-redacts-in-place-and-shreds-the-key.md) already answered this shape once, for a Client asking her Practice. This is the same shape one level up: a Practice asking Doula Cloud. It is a template, not an answer, because a Practice carries something a Client does not — Doula Cloud's own business records about the Practice, not only the Practice's records about other people.

Decided on [#871](https://github.com/markgoho/doula-cloud/issues/871), split from [#288](https://github.com/markgoho/doula-cloud/issues/288). GDPR and every non-US jurisdiction are out of scope, the same carve-out ADR-0027 makes.

## Deletion redacts the `practices` row; it does not scrub it

`practices` keeps its id forever, and nothing under it dangles. Unlike a Client's row, the Practice's own fields — name, address, Stripe Connect ids — are not nulled. A Practice is a business entity, not a natural person: there is no privacy-law reason to scrub what it is, only an obligation to stop what it can do. Deletion locks the row and hides it from every live session; it does not touch its content.

Vocabulary follows ADR-0027's own split: **Deletion** is this ticket's act, on a Practice. **Erasure** stays what happens to a Client, whether an Owner runs it by hand on one Client or a Practice's own deletion cascades it across every Client still on file. The two are related, never the same word for the same thing.

## A thirty-day restore window, where erasure has none

Client erasure is immediate and irreversible. Practice deletion is not, because it can cut off other people's access — Staff, Clients — not only the Owner's own. Initiating deletion starts a 30-day countdown rather than running the act on the spot.

**During the window, the Practice is fully locked.** No Staff or Client request succeeds against it — `staffauth.Middleware` and `clientauth.Middleware` both refuse before any handler runs — except two exemptions. `GET`/`DELETE /api/practices/{practiceId}/deletion` (read the window, or restore it) stay Owner-only, same as every other Owner-only gate; not the Owner who started it specifically, since restoring is seat-based. `GET /api/practices/{practiceId}/session` is open to every role, Owner or not: it now carries a `pendingDeletion` flag, and the app's `practices/[practiceId]/+layout.ts` reads it on every navigation to route an Owner to the restore screen and everyone else to that same screen's own locked notice, rather than every other route under the Practice refusing her one 403 at a time. It answers "who am I here," nothing scoped to the Practice's own data — the lockout on every other route is unchanged. Export ([#288](https://github.com/markgoho/doula-cloud/issues/288)) is the stated prerequisite in product terms, not a code dependency — whatever this act destroys, a Practice can take its data out first, but nothing enforces that a fresh export ran before deletion starts.

A reminder mails every current Owner 7 days before the deadline. Finalization runs automatically at day 30, with no second confirmation — the Owner already confirmed once, at initiation, through the same block-over-warn `ConfirmDialog` every other destructive action in this app uses.

## Finalization cascades every Client's own erasure, and forfeits the balance

At day 30, three things happen in one transaction:

1. Every Client still on file (an Owner may already have erased one by hand during the window) is erased through the exact path a single Owner-run erasure uses — `client.Erase`, ADR-0027's redact-in-place, key-shredding, Stripe-Customer-deletion act, run once per Client with `activity.SystemActor()` rather than the Staff actor a live request would carry.
2. Any unspent Credit balance is **forfeited, not refunded** — written as one `credit_ledger` row, `origin = 'forfeit'`, so the ledger still sums to zero and the act carries its own who-and-when the way every other origin does. Forfeiture happens only here, at finalization, never at initiation: nothing irreversible touches the ledger while the window is still open.
3. `practices.deleted_at` is stamped, and one `activity` row records the finalization — subject the Practice, actor Doula Cloud (ADR-0022's third actor kind, "nobody asking").

Client erasure already refuses while any of that Client's invoices is `draft` or `open`. Rather than let finalization discover that 30 days late with no Owner left to act on it, initiation itself refuses up front if any Client under the Practice carries an unsettled invoice — one whole-Practice precheck, not fifty per-Client ones.

## What survives finalization, table by table

Decision 2 asked for the same enumeration ADR-0027 gives for a Client, one level up. Every table named below is scoped to the Practice, either directly (`practice_id`) or by resolving through an Engagement or Invoice that is; what changes about each at finalization follows.

**Redacted, through the exact cascade ADR-0027 already runs per Client.** `clients`' identifying columns (`given_name` to a placeholder, `family_name`/`preferred_name`/`email`/`phone`/the five address columns/`date_of_birth` to `NULL`, `field_values` to `{}`), `contracts.merge_field_values` to `{}` on every Contract of hers, her `client_data_keys` row deleted (shredding her sealed `activity` diffs), her `portal_accounts` row and `sessions` deleted outright, and her Stripe Customer(s) deleted immediately with a Redaction Job scheduled 90 days out. `client_stripe_customers`'s own mapping rows are not scrubbed by any of this -- the same stated limitation ADR-0027 already carries for a single Client's own erasure, unchanged by running every Client's erasure through the same cascade rather than by hand.

**Kept, but locked for the 30-day window and every day after (finalization does not touch these; only the cascade above and `practices.deleted_at` itself do).** `engagements`, `visits`, `messages`, `contracts`, `invoices`, `payments`, `plan_templates`/`plan_instances`, and `practice_websites` -- every one of them still resolves by id and still holds exactly what it held. The lock itself lives on `practice_memberships`, not on `staff`: a Staff member who also belongs to another Practice keeps signing in there without incident, since `staffauth.Middleware` scopes its refusal to the one Practice pending deletion, not to the person.

## What Doula Cloud keeps regardless

The Credit ledger, the Stripe Connect account and its payouts, and `stripe_webhook_events` are Doula Cloud's own business records, not the Practice's to delete out from under. Deletion does not touch the Practice's connected Stripe account at all — it is the Practice's own, on its own Stripe login. A Practice cannot delete its way out of Doula Cloud's ledger, and the confirmation says so before an Owner commits to the act.

`activity` is the same outcome for a different reason: append-only, `GRANT SELECT, INSERT` only, and the record `CLAUDE.md`'s audit expectation exists to keep. Every `*_outbox` table — `client_erasure_outbox`, `practice_deletion_outbox` itself, and the rest — is kept regardless too, for a third reason again: none of them is scoped to a live session at all, so deletion has no mechanism that could reach them even if it wanted to.

## Cancelling a queued job by rechecking live state, not by cancelling it

Client erasure's outbox (`client_erasure_outbox`, ADR-0027) carries no cancel mechanism and no `DELETE` grant — a design accepted there because erasure itself is immediate and irreversible; nothing queued against it is ever meant to be taken back. Practice deletion's own outbox (`practice_deletion_outbox`) inherits that same shape for the same reason (Postgres's row locking gives no clean way to guarantee a cancel lands before a concurrent claim does), but now genuinely needs to be undoable: a restore during the 30-day window must stop the day-30 finalization and the day-23 reminder that were already enqueued at initiation.

The mechanism is the "skip-at-send recheck" `offer/outbox.go` already uses for a withdrawn Offer: each worker rechecks the Practice's live `deletion_requested_at`/`deleted_at` at send time, immediately before acting, and a row whose Practice is no longer pending is marked sent having done nothing. Restoring never touches the outbox table at all — it only clears the three columns on `practices` the recheck reads.

## Why two durable columns on `practices` rather than a queue-only fact

`deletion_finalize_at` on `practices` is the durable "restore closes on this date" fact; `practice_deletion_outbox`'s own `next_attempt_at` is the separately mutable retry clock a failed send's backoff schedule can move. This is the same split migration `00065` drew between `client_erasure_outbox.redactable_after` and `next_attempt_at` for exactly the same reason: a retry backoff must never quietly move a deadline an Owner has already been shown.

## Considered and rejected

**Deleting the `practices` row.** The same argument ADR-0027 makes for `clients`, one table wider: it would take fifty-odd related tables with it or leave them orphaned, and Doula Cloud's own billing record is not the Practice's to delete.

**Requiring an Owner to erase every Client by hand before deleting the Practice.** A 14-doula agency can carry hundreds of Client records; asking an Owner to run erasure that many times by hand is not a real requirement, it is a way of not building the cascade.

**No restore window, matching Client erasure's own immediacy.** Rejected specifically because this act can end other people's access, not only the actor's own — Client erasure only ever costs the Client who asked, or the Practice that ran it on her behalf.

**Refunding the unspent Credit balance.** Considered and left open at triage until [#285](https://github.com/markgoho/doula-cloud/issues/285) and [#286](https://github.com/markgoho/doula-cloud/issues/286) settled what a Credit costs and buys. Once both closed, forfeiture was chosen: a Credit is not a subscription payment owed back on cancellation, and refunding would need to reconcile against per-lot refund eligibility (`credit_ledger`'s own three-year purchased-lot window) for a Practice that, by definition, will not be back to dispute it.

**Deleting the Practice's own Stripe Connect account.** It belongs to the Practice, on the Practice's own Stripe login, not to Doula Cloud to close on its behalf.

**A second explicit confirmation at day 30.** The Owner already confirmed once, with the consequence stated in full, at initiation. A second prompt with nobody left to answer it would either block finalization forever or become a formality nobody reads — the same reasoning `staff.ts`'s `X-Confirmed` backstop settles once, at the point where a person is actually looking at the screen.
