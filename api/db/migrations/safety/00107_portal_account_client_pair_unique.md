# 00107_portal_account_client_pair_unique.sql

`ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_identity_client_key UNIQUE (identity_uid, client_id)` — the proof is written out at length in the migration's own SQL comments (#819); this note restates it in the form the guardrail reads, because a migration goose has already recorded cannot be edited to carry the marker itself.

## ADD CONSTRAINT ... UNIQUE / PRIMARY KEY / EXCLUDE

No existing row can be a duplicate of the pair, in two halves that cover the table's whole history.

Before `00081`, `client_portal_users.identity_uid` carried a table-wide `UNIQUE` constraint from `00006`. A constraint on `identity_uid` alone refuses every duplicate of any pair that contains it, so nothing written before `00081` can be a duplicate `(identity_uid, client_id)`.

Since `00081`, the only writer that sets `identity_uid` is `portalinvite.acceptInvite`, and it consults `portal_account_reuse_for_accept` first: a Portal Account that already reaches a Client at the invitation's Practice is refused with a 409. A duplicate pair is the same Client twice, and a Client belongs to exactly one Practice (ADR-0015), so that refusal already covers the pair.

`identity_uid` stays nullable — a pending invitation has no Portal Account yet (`00026`), and erasure clears it (`00064`) — and Postgres treats NULLs as distinct in a unique constraint, so pending and erased rows cannot collide with each other either.

The migration deliberately does not delete duplicates instead of proving their absence: `portal_invite_outbox.client_portal_user_id` (`00032`) references this table with no `ON DELETE` clause and the outbox row outlives acceptance, so a `DELETE` here would trade an unreachable unique violation for a reachable foreign-key one — green on the PR, red on trunk, which is the failure this guardrail exists to stop.
