-- +goose Up
-- #830: one Practice's Client erasure was deleting the whole Portal
-- Account, and so reaching into every other Practice that still holds a
-- Client behind the same login.
--
-- 00081 (#309) dropped client_portal_users' table-wide UNIQUE on
-- identity_uid, so ADR-0015's real shape -- "a Portal Account reaches
-- many Clients, at most one per Practice" -- became reachable. That made
-- a pre-existing assumption in client.enqueuePortalErasure wrong: its
-- `DELETE FROM portal_accounts WHERE identifier = $1` is keyed on the
-- login alone, and the FK's ON DELETE SET NULL (00073) then clears
-- identity_uid on every sibling client_portal_users row, at Practices
-- that never asked for anything. ADR-0015 forbids exactly that: "No
-- Client fact crosses a Practice."
--
-- The rule this migration makes enforceable: the Portal Account row is
-- deleted only when no un-erased Client anywhere still reaches it. The
-- last Practice out takes the login with it; anyone before that takes
-- only its own link.

-- The question erasure has to ask before it deletes. It crosses tenants
-- by construction -- the whole point is a row at a Practice other than
-- the one running the erasure -- so no SELECT policy on
-- client_portal_users or clients admits it, and a plain NOT EXISTS
-- written inline would read zero rows and answer "nothing else reaches
-- it" every time.
--
-- SECURITY DEFINER, the same purpose-built shape
-- portal_account_reuse_for_accept (00081) and
-- push_subscriptions_for_message_recipient (00067) already take: it
-- answers this one question, returns one bit, and opens no door onto any
-- row of either table.
--
-- "A live Client", not "another live Client": the caller runs this after
-- client.redactRecord has already stamped clients.erased_at on the
-- Client being erased, so her own row is excluded by the erased_at test
-- rather than by an id parameter this function would otherwise have to
-- be trusted to pass correctly.
--
-- The one bit this returns is not the erasing Practice's to see. It says
-- a named woman is a Client somewhere else, which is the fact ADR-0015
-- keeps inside the portal -- so nothing may carry it into an API
-- response or an activity diff.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION portal_account_reaches_a_live_client(p_identifier text)
RETURNS boolean
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT EXISTS (
        SELECT 1
          FROM client_portal_users pu
          JOIN clients c ON c.id = pu.client_id
         WHERE pu.identity_uid = p_identifier
           AND c.erased_at IS NULL
    )
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION portal_account_reaches_a_live_client(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION portal_account_reaches_a_live_client(text) TO app_runtime;

-- The same predicate on erasure's own door, so the database refuses the
-- cross-Practice delete whatever the caller believes. client.Erase asks
-- the function first and does not issue the DELETE at all when a live
-- Client still reaches the account; this is the boundary that can
-- actually enforce it if a future caller forgets to ask.
--
-- The paired SELECT policy (00073) is left exactly as it was: it exists
-- so the DELETE's own USING clause can find the row it is allowed to
-- remove, and narrowing the read too would only make a refused delete
-- harder to explain, not safer.
DROP POLICY portal_accounts_erasure_delete ON portal_accounts;

CREATE POLICY portal_accounts_erasure_delete ON portal_accounts
    FOR DELETE
    USING (
        NOT portal_account_reaches_a_live_client(portal_accounts.identifier)
        AND EXISTS (
            SELECT 1 FROM client_portal_users pu
            JOIN clients c ON c.id = pu.client_id
            WHERE pu.identity_uid = portal_accounts.identifier
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND c.erased_at IS NOT NULL
        )
    );

-- +goose Down
DROP POLICY portal_accounts_erasure_delete ON portal_accounts;

CREATE POLICY portal_accounts_erasure_delete ON portal_accounts
    FOR DELETE
    USING (
        EXISTS (
            SELECT 1 FROM client_portal_users pu
            JOIN clients c ON c.id = pu.client_id
            WHERE pu.identity_uid = portal_accounts.identifier
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND c.erased_at IS NOT NULL
        )
    );

REVOKE EXECUTE ON FUNCTION portal_account_reaches_a_live_client(text) FROM app_runtime;
DROP FUNCTION portal_account_reaches_a_live_client(text);
