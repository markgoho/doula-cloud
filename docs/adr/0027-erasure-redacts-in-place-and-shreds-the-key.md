# Erasure redacts in place and shreds the key

A Client asks her Practice to delete her data. US state right-to-delete law — CCPA/CPRA and the state statutes that copy it — obliges the Practice to honor that, while a separate body of law obliges the same Practice to keep the financial record of what it billed her. Doula Cloud has a third obligation of its own: `activity` is append-only, holds `GRANT SELECT, INSERT` and nothing else, and is the answer to `CLAUDE.md`'s standing audit expectation. All three have to hold at once.

Decided on [#394](https://github.com/markgoho/doula-cloud/issues/394). GDPR and every non-US jurisdiction are out of scope; no pilot Practice crosses any state's applicability threshold yet, and this is built ahead of that trigger rather than in response to it.

## Erasure is not ending

`CONTEXT.md` already records that an Attachment *ends* but is never deleted, and that ending is not erasure. Erasure is the other act, and it is the only one in the model that destroys a fact rather than closing it. Every other terminal state in this product — a completed Engagement, an ended Attachment, a voided Contract — leaves the record more complete, not less. Erasure leaves it less complete on purpose, because a person asked, and it is the only act that does.

That is why erasure is Owner-only: not Owner-or-Admin, which is the gate for most things that reshape a Practice, but Owner alone, the same seat that throws the MFA switch ([#167](https://github.com/markgoho/doula-cloud/issues/167)) and vouches for a locked-out Staff member. There is one seat that can destroy a fact, and it is the seat that owns the Practice.

## The row is redacted, never deleted

Her `clients` row keeps its id forever. Every Message, Contract, Plan Instance, Invoice, Visit and Engagement Request that names her by `client_id` keeps resolving; nothing dangles, no foreign key is dropped, and the Practice's own record of what work it did and what it billed stays whole. What changes is the content of her identifying columns: `given_name` becomes a fixed placeholder (`given_name` is `NOT NULL`, so it cannot be nulled), and `family_name`, `preferred_name`, `email`, `phone`, the five address columns and `date_of_birth` all go to `NULL`. `field_values` — the Practice-defined layer — goes to `{}`. `contracts.merge_field_values`, which is the same structured shape captured a second time at Contract-render time, goes to `{}` on every Contract under every Engagement of hers.

`erased_at` on the row is the proof the act ran, and the gate: an erased Client cannot be edited, and cannot be erased twice.

Free text is not scrubbed. A Message body, a frozen Contract's prose, a Plan Instance answer or an Engagement Request note that happens to say her name by hand stays exactly as written. This is a stated limitation, not an oversight: those are the Practice's own clinical and contractual records, they are frozen by design elsewhere in this product, and a regex sweep across them would be both unreliable and destructive of records the Practice is obliged to keep. Mailgun's send logs are likewise untouched — Mailgun self-purges within 3–30 days and exposes no per-recipient purge API to a sender.

## The audit log is shredded, not edited

`activity` never takes an `UPDATE` or a `DELETE`, before or after erasure, and its grant does not change. The problem this creates is that `client/events.go` writes her name, email, phone, address and date of birth into `activity.diff` on every create and every edit — the before-and-after of exactly the columns erasure is meant to destroy.

The mechanism is crypto-shredding. Every Client gets a random 256-bit data key at create time, held in `client_data_keys`, which is an ordinary mutable table with a `DELETE` grant. Every `subject_kind = 'client'` diff is sealed under that key with AES-256-GCM before it is inserted, and the column holds `{"v":1,"enc":"<base64 nonce||ciphertext>"}` — still `jsonb`, still one column, still append-only. Erasure deletes the key row. The ciphertext stays in `activity` untouched, the reader renders it as unreadable, and the fact that an event happened, when, and who did it all survive in plaintext — only *what changed* becomes unrecoverable.

The stated limitation: a database backup taken before erasure holds both the ciphertext and the key, so shredding is effective from the moment of erasure forward and not retroactively across the backup retention window. Wrapping the data key under a KMS-held key encryption key was considered and deferred — it does not serve this goal, since a backup that captures the key table captures the wrapped key too, and the KEK is not what erasure destroys. It becomes worth doing when the threat model is a stolen database dump rather than a right-to-delete request; that is a different ticket.

A second stated limitation, narrower: `activity` rows written before this mechanism existed are plaintext and stay that way. Sealing is a write-time act and `activity` takes no `UPDATE`, so there was never a migration that could have converted them — the read path recognizes both shapes and returns a pre-existing plaintext diff as it is. Shredding therefore covers every Client event written from #394 forward, and none written before it. Pre-launch, no such row holds a real person's data, which is the only reason this is a footnote rather than a blocker.

The erasure act itself is recorded as a plaintext `activity` row — action `erased`, subject the Client, actor the Owner, diff naming what the act covered. It is deliberately outside the sealing path: it describes the act, not her data, and it has to stay readable after her own history is shredded.

## Stripe: delete the Customer now, redact the transactions when they are old enough

A Client's Stripe Customer lives on her Practice's connected account. Erasure deletes it, which Stripe itself recommends doing first, because it stops new transactions attaching to an object that is about to be redacted. Deleting the Customer is enough to remove her name and email from Stripe; it is not enough to remove her from the charges and payment intents underneath it, which is what Redaction Jobs are for.

Stripe will not redact most transactions until 90 days after they are created. So erasure does not attempt a redaction job it knows will fail. It computes the eligibility date locally and schedules the job for that date through the same outbox pattern every other deferred act in this product uses.

The date is **per Stripe Customer, not per Client**. `payments.CreateInvoice` makes a fresh Customer for every invoice, so a Client billed twice six months apart has two Customers whose transactions come of age six months apart, and each redaction job waits only on its own. One Client-wide date — 90 days past her newest invoice overall — would hold a year-old charge hostage to a bill she paid last week, which is not what the law asks and not what Stripe requires.

What the Practice sees is the last of those dates, on her detail read as `stripeRedactionEligibleAt`: when the Stripe half of her erasure actually finishes. It is present for as long as any redaction has not succeeded — **including after one has failed**, where the date it names may already be in the past. That is deliberate. The date lives in its own `redactable_after` column rather than being read back off the outbox row's `next_attempt_at`, because retry backoff rewrites `next_attempt_at` on every failed attempt: reading it would turn "redactable after March" into a five-minute retry stamp, and dropping dead-lettered rows from the read would make a redaction that failed look like one that finished. Absent means done, and nothing else.

Two things were verified against the Sandbox rather than read from the docs, and both shaped this:

- `DELETE /v1/customers/:id` with `Stripe-Account` works on a connected account. That leg is real.
- `POST /v1/privacy/redaction_jobs` answers *Unrecognized request URL* on this account, under both `/v1` and `/v2` and under every preview `Stripe-Version` tried. Redaction Jobs is in public preview and is not enabled on the Doula Cloud account. The call is implemented against the documented endpoint through `stripe.RawRequest`, and until the preview is enabled it will dead-letter with that error rather than silently claiming success. Enabling it is an account request, not code.

Erasure refuses, with a 409 naming the invoices, while any of her invoices is still `draft` or `open`. A non-terminal transaction cannot be redacted, and deleting the Customer under an open invoice leaves the Practice unable to collect on work it did. She is erased once the money is settled, one way or the other.

## The portal login is deleted, and so is the session holding it

Her `client_portal_users.identity_uid` names a GCP Identity Platform account whose only content is her email address. Erasure deletes that account through the Admin SDK — the first account deletion in this codebase — and clears `identity_uid` on the row so nothing points at a dead uid.

Deleting the account does not invalidate a `__session` cookie she already holds; sessions in this product are rows in Postgres, verified against Postgres. So erasure deletes her session rows in the same transaction. She cannot authenticate to the portal afterwards, and she is not still inside it.

## Amendment, 2026-09-10: the login is one person's, so the last Practice out is the one that takes it ([#830](https://github.com/markgoho/doula-cloud/issues/830))

The section above was written when a Portal Account belonged to exactly one Client record, which `client_portal_users.identity_uid`'s table-wide `UNIQUE` enforced. It no longer does: [#309](https://github.com/markgoho/doula-cloud/issues/309) lifted that constraint so [ADR-0015](0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md)'s actual shape — *a Portal Account reaches many Clients, at most one per Practice* — became reachable. (The Identity Platform account that section deletes is also gone; [ADR-0026](0026-two-populations-two-sign-in-methods-and-one-token-table.md) took Identity Platform away from Clients entirely and `portal_accounts` is where the sign-in address lives now.)

Camille is a Client at Rooted Birth Collective and at Ridgeline Doulas, behind one login. She asks Rooted, and only Rooted, to erase her. Under the section above, Rooted's erasure deleted the `portal_accounts` row keyed on her identifier, the foreign key's `ON DELETE SET NULL` nulled `identity_uid` on Ridgeline's row too, and every session she held anywhere ended — including the one she had open with Ridgeline. One Practice's request reached into another Practice's relationship with her, which is exactly what ADR-0015's *no Client fact crosses a Practice* refuses.

**The rule: erasure removes the erasing Practice's link to the login always, and deletes the login itself only when no un-erased Client anywhere still reaches it.** The last Practice out takes it; anyone before that takes only its own link. Three consequences, each decided here rather than left to the caller.

**Her sign-in address stays while the login does.** It is the login's own fact — ADR-0015's *the person lives in the login* — not the erasing Practice's record of her, and she is still an active user of Doula Cloud through the other Practice. What Rooted's erasure destroys is Rooted's ability to reach her, which the cleared `identity_uid` on Rooted's own row already accomplishes.

**Her sessions survive while the login does.** A session is the person's, not a Practice's: one reaches every Client she has. `clientauth` recomputes her reachable set from `client_portal_users` on every request, so a row with no `identity_uid` is a Client she can no longer address, live cookie or not — the unlink is the enforcement, and ending every session would only be the same cross-Practice reach in a different form. When the login itself goes, the sessions go with it, unchanged from the section above.

**The other Practice is told nothing, and neither is the erasing one.** Ridgeline's audit trail has nothing to explain, because nothing about Ridgeline's relationship with her changed — and writing an entry there saying Rooted erased her would *be* the leak. The reverse holds just as hard: `ErasureResponse.portalAccountQueued` and the `erased` activity row's `portalAccount` both mean *her portal link at this Practice was removed*, true in both branches, so no Owner can read the existence of another Practice off her own erasure's result. The `erased` row carries **no session count** for the same reason — a count reads zero exactly when the login survived, and an Owner who saw her signed in an hour ago would read that zero as *somebody else serves her*. What her sessions did follows from the login's fate, and the login's fate is not this Practice's to be told.

**Two Practices erasing her at once are serialized on the login.** Without that, each transaction reads the other's Client as still un-erased, each concludes the login is still reached, and both commit — leaving a `portal_accounts` row holding her sign-in address that no Client reaches and that this ADR's own policy can never again admit to a delete. A permanent orphan of exactly the data erasure exists to scrub is a worse failure than a wait, so the act takes a transaction-scoped advisory lock on the identifier before it asks the question. An advisory lock rather than a row lock: `SELECT … FOR UPDATE` applies `portal_accounts`' `UPDATE` policy, which admits only the row whose identifier is the *caller's own* login — a Client changing her own sign-in address — so inside a Staff transaction it would match no row and lock nothing, while looking exactly like a lock that was taken. The delete's row count is checked rather than absorbed for the same reason: a login that was supposed to go and did not must fail the whole erasure, not leave her signed out with the login standing.

The liveness question crosses tenants by construction, so it cannot be asked under the erasing Practice's own row security — a sibling row is invisible to it, and a `NOT EXISTS` written inline would answer "nothing else reaches this login" every time. It is a `SECURITY DEFINER` function returning one bit, the same purpose-built shape `portal_account_reuse_for_accept` already takes, and `portal_accounts`' own `DELETE` policy carries the predicate too so the database refuses the cross-Practice delete whatever a future caller believes.

[ADR-0031](0031-practice-deletion-is-a-thirty-day-window-then-the-erasure-cascade.md)'s deletion cascade runs the same act per Client and inherits this rule unchanged. It spares a **login**, and only while an un-erased Client still reaches it; it retains no record of care and restores nothing, which is the interaction [#1094](https://github.com/markgoho/doula-cloud/issues/1094) asks about from the other side.

## Considered and rejected

**Deleting the `clients` row.** It would take every Invoice, Contract and Visit with it, or leave them dangling. The Practice's financial and clinical record is not hers to delete, and it is not the product's either.

**Anonymizing rather than redacting — replacing her name with a stable pseudonym.** A pseudonym that is stable enough to be useful is stable enough to re-identify her against the free text this ADR declines to scrub. A placeholder that says nothing is honest about what happened.

**Deleting or updating `activity` rows for her.** It is the one invariant this product does not trade: an append-only log that is sometimes edited is not an audit log. Crypto-shredding exists precisely so the invariant survives the erasure.

**Erasing the whole Portal Account whatever else reaches it, and notifying the other Practices** ([#830](https://github.com/markgoho/doula-cloud/issues/830)'s second option). It would need ADR-0015 amended to say what one Practice's erasure request costs a Practice that made no request, and the notification is itself the cross-Practice disclosure the same ADR forbids — the notice cannot be written without naming a Client relationship the recipient never had cause to hear about. There is also nothing to gain: the login holds one fact about her, her sign-in address, and she is still using it.

**A Client-facing erasure request in the portal.** This is the Practice-side act. Whether a Client can ask for it herself, and what the Practice owes her in response time, is a service-design question, not this one.
