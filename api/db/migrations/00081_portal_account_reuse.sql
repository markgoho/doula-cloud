-- +goose Up
-- #309: a person who already holds a Portal Account can accept a second
-- Practice's invitation and reach a new Client through the same
-- identity, per ADR-0015 ("a Portal Account reaches many Clients, at
-- most one per Practice"). Two things stood in the way.

-- 1. client_portal_users.identity_uid still carried the table-wide
-- UNIQUE constraint 00006 gave it, back when identity_uid held a
-- caller's raw Identity Platform uid and one row per uid was the whole
-- model. #616 turned identity_uid into a foreign key into
-- portal_accounts (many client_portal_users rows can name one Portal
-- Account) but never lifted this constraint, so a second row for the
-- same Portal Account still failed on it before ADR-0015's own,
-- narrower rule ever got a chance to run.
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_identity_uid_key;

-- That constraint's own index was every WHERE identity_uid = ... lookup's
-- index too (clientauth.Middleware runs one on every Client-portal
-- request). Dropping the constraint drops the index with it, so it is
-- replaced here rather than left implicit -- a plain, non-unique index
-- now that more than one row can share a value.
CREATE INDEX client_portal_users_identity_uid ON client_portal_users (identity_uid);

-- 2. Nothing enforces the narrower rule ADR-0015 actually states: one
-- row per (Portal Account, Practice). client_portal_users carries no
-- practice_id of its own (a Client belongs to one Practice, reached
-- through client_id), so the invariant crosses two tables and cannot be
-- a plain UNIQUE index the way client_portal_users_one_pending_per_client
-- (00026) is. portalinvite.acceptInvite enforces it itself, inside the
-- same transaction as the attach, by asking this function the one
-- question it needs answered: does this sign-in address's Portal
-- Account already reach a Client at this Practice?
--
-- SECURITY DEFINER, mirroring push_subscriptions_for_message_recipient
-- (00067): acceptInvite runs pre-account, with only app.invite_token and
-- a freshly cleared app.current_client_id in scope, so no existing
-- SELECT policy on client_portal_users or clients admits the
-- cross-Practice, cross-Client read this question needs. A
-- purpose-built function, not a new general-purpose RLS policy: it
-- answers exactly this one question and nothing else, rather than
-- opening a door onto every row either table holds.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION portal_account_reuse_for_accept(p_sign_in_address text, p_practice_id uuid)
RETURNS TABLE (identifier text, conflicting_client_id uuid)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT pa.identifier,
           (SELECT pu.client_id
              FROM client_portal_users pu
              JOIN clients c ON c.id = pu.client_id
             WHERE pu.identity_uid = pa.identifier
               AND c.practice_id = p_practice_id
             LIMIT 1)
      FROM portal_accounts pa
     WHERE lower(pa.sign_in_address) = lower(p_sign_in_address)
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION portal_account_reuse_for_accept(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION portal_account_reuse_for_accept(text, uuid) TO app_runtime;

-- +goose Down
REVOKE EXECUTE ON FUNCTION portal_account_reuse_for_accept(text, uuid) FROM PUBLIC;
DROP FUNCTION portal_account_reuse_for_accept(text, uuid);
DROP INDEX client_portal_users_identity_uid;
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_identity_uid_key UNIQUE (identity_uid);
