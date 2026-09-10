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

-- Guarded against a database that already holds rows: the migrate job
-- runs on trunk against doula-cloud-pg, where a PR's empty database
-- proves nothing (see this package's guardrail_test.go and #1021).
-- Pre-launch there is no production data, and a duplicate pair is a
-- redundant second link to the same Client from the same Portal Account
-- either way, so the earliest row survives and the rest go.
DELETE FROM client_portal_users a
 USING client_portal_users b
 WHERE a.identity_uid IS NOT NULL
   AND a.identity_uid = b.identity_uid
   AND a.client_id = b.client_id
   AND (b.created_at, b.id) < (a.created_at, a.id);

ALTER TABLE client_portal_users
    ADD CONSTRAINT client_portal_users_identity_client_key UNIQUE (identity_uid, client_id);

-- The constraint's own index leads on identity_uid, so it answers every
-- lookup 00081's single-column index was created to answer. Keeping both
-- would pay for two indexes to serve one query shape.
DROP INDEX client_portal_users_identity_uid;

-- +goose Down
CREATE INDEX client_portal_users_identity_uid ON client_portal_users (identity_uid);
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_identity_client_key;
