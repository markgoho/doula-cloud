-- +goose Up
-- ADR-0026's "The Portal Account becomes a table" (#616). Clients stop
-- having Identity Platform accounts, so Doula Cloud must hold the
-- sign-in address itself -- it is what a magic-link request (#617) will
-- be looked up by. It does not belong on client_portal_users:
-- ADR-0015 already makes the Portal Account one identity across several
-- client_portal_users rows (a person can be a Client at more than one
-- Practice), so an address repeated per row would store one fact many
-- times. portal_accounts is that identity's own table.
--
-- identifier stays `text`, minted by api/internal/portalaccount, rather
-- than `uuid`: the ADR's prefix -- the sanctioned way to tell a Portal
-- Account's identity_uid apart from an Identity Platform one -- is not a
-- valid uuid literal.
CREATE TABLE portal_accounts (
    identifier      text PRIMARY KEY,
    sign_in_address text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- Case-insensitive, matching practice_invitations_one_pending's shape
-- (00039) -- a sign-in address is looked up the same way an invited
-- address is.
CREATE UNIQUE INDEX portal_accounts_sign_in_address ON portal_accounts (lower(sign_in_address));

-- No UPDATE grant: changing the sign-in address is #619's door
-- ("A Client changes her own sign-in address, proved by a link to the
-- new one"), not this table's default access.
GRANT SELECT, INSERT, DELETE ON portal_accounts TO app_runtime;

ALTER TABLE portal_accounts ENABLE ROW LEVEL SECURITY;

-- Mirrors staff_self_insert (00003_staff_signup.sql): the only writer is
-- portalinvite.acceptInvite, which sets app.current_identity_uid to the
-- identifier it just minted before this INSERT.
CREATE POLICY portal_accounts_self_insert ON portal_accounts
    FOR INSERT
    WITH CHECK (identifier = NULLIF(current_setting('app.current_identity_uid', true), ''));

-- Erasure's own door, the same shape as client_portal_users_erasure_update
-- (00064_client_erasure.sql): admits a DELETE only for the Portal Account
-- behind an already-erased Client's row. Reached through
-- client_portal_users rather than a column of its own -- portal_accounts
-- carries no practice_id or client_id, because ADR-0015 makes one Portal
-- Account able to reach Clients at more than one Practice, so it cannot
-- be scoped to a single one.
--
-- Paired with a SELECT policy carrying the identical USING, not left as
-- DELETE alone: a table with row security enabled and no SELECT (or ALL)
-- policy admits no row to any read at all, including the implicit lookup
-- a DELETE's own USING clause needs to find the row it is allowed to
-- remove -- confirmed empirically (client.enqueuePortalErasure's DELETE
-- silently matched zero rows, no error, until this policy was added).
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

CREATE POLICY portal_accounts_erasure_select ON portal_accounts
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM client_portal_users pu
            JOIN clients c ON c.id = pu.client_id
            WHERE pu.identity_uid = portal_accounts.identifier
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND c.erased_at IS NOT NULL
        )
    );

-- client_portal_users.identity_uid now names a Portal Account rather
-- than an Identity Platform uid directly -- the FK makes that real
-- rather than a convention nobody enforces. FK checks run as the
-- constraint's own privilege, bypassing RLS, so this needs no SELECT
-- policy on portal_accounts.
--
-- ON DELETE SET NULL, not the default NO ACTION: erasure
-- (client.enqueuePortalErasure) deletes the portal_accounts row directly,
-- and portal_accounts_erasure_delete's own USING clause is the reason
-- the order has to be delete-then-clear rather than the reverse --
-- it finds the row to delete by joining back through
-- client_portal_users.identity_uid, so that column must still hold the
-- identifier at DELETE time. This cascade is what clears it immediately
-- after, in the same statement, rather than erase.go needing a second,
-- separately-ordered UPDATE to avoid violating the FK the other way.
ALTER TABLE client_portal_users
    ADD CONSTRAINT client_portal_users_identity_uid_fkey
    FOREIGN KEY (identity_uid) REFERENCES portal_accounts (identifier) ON DELETE SET NULL;

-- The invitation has never had an expiry (#616's own finding). 7 days,
-- re-sendable by the doula (portalinvite.invite already rotates
-- invite_token on a re-invite; this rides the same UPDATE). Nullable
-- like invite_token itself: a pending row with no invite_token has no
-- expiry either, and an already-accepted row has neither.
ALTER TABLE client_portal_users ADD COLUMN invite_token_expires_at timestamptz;

-- +goose Down
ALTER TABLE client_portal_users DROP COLUMN invite_token_expires_at;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_identity_uid_fkey;
DROP POLICY portal_accounts_erasure_select ON portal_accounts;
DROP POLICY portal_accounts_erasure_delete ON portal_accounts;
DROP POLICY portal_accounts_self_insert ON portal_accounts;
DROP TABLE portal_accounts;
