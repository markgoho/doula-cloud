-- +goose Up
-- #819: client_portal_users.identity_uid carried a table-wide UNIQUE
-- constraint from 00006, which said one Portal Account reaches exactly
-- one Client record. ADR-0015 says the opposite -- "a Portal Account
-- reaches many Clients, at most one per Practice" -- and ADR-0026 rests
-- its whole argument for portal_accounts on that shape. #309 (00081)
-- settled the disagreement in the ADR's favor and dropped the
-- constraint, leaving a plain index in its place so the
-- WHERE identity_uid = ... lookups kept an index to use.
--
-- What #309 did not leave behind is any constraint holding the rule that
-- replaced it. ADR-0015's own rule is narrower than "anything goes": at
-- most one Client per Practice, and therefore at most one link row per
-- (Portal Account, Client), since a Client belongs to exactly one
-- Practice. Today that is enforced only in Go, inside
-- portalinvite.acceptInvite's reuse check -- a refusal the database
-- would still admit if any other writer ever reached this table. The
-- pair is what UNIQUE can express here, so this is the constraint
-- 00006's leftover is actually replaced by.
--
-- identity_uid stays nullable (a pending invitation has no Portal
-- Account yet, 00026; erasure clears it, 00064) and Postgres treats
-- NULLs as distinct by default, so pending and erased rows are untouched
-- and client_portal_users_one_pending_per_client (00026) keeps its own
-- job unchanged.

-- No existing row can violate this, which is the question the migrate
-- job on trunk asks and a PR's empty database cannot answer (see this
-- package's guardrail_test.go and #1021). The proof is in two halves.
-- Before 00081 the table-wide UNIQUE on identity_uid alone refused every
-- duplicate, the pair included, so nothing older than that migration can
-- be one. Since 00081 the only writer that sets identity_uid is
-- portalinvite.acceptInvite, and it asks portal_account_reuse_for_accept
-- first: a Portal Account that already reaches a Client at the
-- invitation's Practice is refused with a 409. A duplicate pair is the
-- same Client twice, and a Client belongs to one Practice, so that
-- refusal covers it.
--
-- Deleting duplicates instead of proving their absence is what this
-- deliberately does not do: portal_invite_outbox.client_portal_user_id
-- (00032) references this table with no ON DELETE clause, and the outbox
-- row outlives acceptance, so a DELETE here would trade an unreachable
-- constraint violation for a reachable foreign-key one -- green on the
-- PR and red on trunk, the exact failure this comment exists to rule
-- out.

ALTER TABLE client_portal_users
    ADD CONSTRAINT client_portal_users_identity_client_key UNIQUE (identity_uid, client_id);

-- The constraint's own index leads on identity_uid, so it answers every
-- lookup 00081's single-column index was created to answer. Keeping both
-- would pay for two indexes to serve one query shape.
DROP INDEX client_portal_users_identity_uid;

-- +goose Down
CREATE INDEX client_portal_users_identity_uid ON client_portal_users (identity_uid);
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_identity_client_key;
