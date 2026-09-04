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

The erasure act itself is recorded as a plaintext `activity` row — action `erased`, subject the Client, actor the Owner, diff naming what the act covered. It is deliberately outside the sealing path: it describes the act, not her data, and it has to stay readable after her own history is shredded.

## Stripe: delete the Customer now, redact the transactions when they are old enough

A Client's Stripe Customer lives on her Practice's connected account. Erasure deletes it, which Stripe itself recommends doing first, because it stops new transactions attaching to an object that is about to be redacted. Deleting the Customer is enough to remove her name and email from Stripe; it is not enough to remove her from the charges and payment intents underneath it, which is what Redaction Jobs are for.

Stripe will not redact most transactions until 90 days after they are created. So erasure does not attempt a redaction job it knows will fail. It computes the eligibility date locally — 90 days past the newest of her invoices — and schedules the job for that date through the same outbox pattern every other deferred act in this product uses. Until then `stripeRedactionEligibleAt` is on her detail read, so the Practice can see the state rather than being told the work is finished when it is not.

Two things were verified against the Sandbox rather than read from the docs, and both shaped this:

- `DELETE /v1/customers/:id` with `Stripe-Account` works on a connected account. That leg is real.
- `POST /v1/privacy/redaction_jobs` answers *Unrecognized request URL* on this account, under both `/v1` and `/v2` and under every preview `Stripe-Version` tried. Redaction Jobs is in public preview and is not enabled on the Doula Cloud account. The call is implemented against the documented endpoint through `stripe.RawRequest`, and until the preview is enabled it will dead-letter with that error rather than silently claiming success. Enabling it is an account request, not code.

Erasure refuses, with a 409 naming the invoices, while any of her invoices is still `draft` or `open`. A non-terminal transaction cannot be redacted, and deleting the Customer under an open invoice leaves the Practice unable to collect on work it did. She is erased once the money is settled, one way or the other.

## The portal login is deleted, and so is the session holding it

Her `client_portal_users.identity_uid` names a GCP Identity Platform account whose only content is her email address. Erasure deletes that account through the Admin SDK — the first account deletion in this codebase — and clears `identity_uid` on the row so nothing points at a dead uid.

Deleting the account does not invalidate a `__session` cookie she already holds; sessions in this product are rows in Postgres, verified against Postgres. So erasure deletes her session rows in the same transaction. She cannot authenticate to the portal afterwards, and she is not still inside it.

## Considered and rejected

**Deleting the `clients` row.** It would take every Invoice, Contract and Visit with it, or leave them dangling. The Practice's financial and clinical record is not hers to delete, and it is not the product's either.

**Anonymizing rather than redacting — replacing her name with a stable pseudonym.** A pseudonym that is stable enough to be useful is stable enough to re-identify her against the free text this ADR declines to scrub. A placeholder that says nothing is honest about what happened.

**Deleting or updating `activity` rows for her.** It is the one invariant this product does not trade: an append-only log that is sometimes edited is not an audit log. Crypto-shredding exists precisely so the invariant survives the erasure.

**A Client-facing erasure request in the portal.** This is the Practice-side act. Whether a Client can ask for it herself, and what the Practice owes her in response time, is a service-design question, not this one.
