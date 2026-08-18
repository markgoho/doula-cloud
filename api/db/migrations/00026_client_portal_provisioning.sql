-- +goose Up
-- Client-portal provisioning: lets a Staff member invite a Client to the
-- portal and have it land as a client_portal_users row once the Client
-- verifies their identity. Mirrors 00004_staff_invitation.sql's
-- pending-row shape, applied to client_portal_users instead of staff.
--
-- Unlike staff (one row per invite, id generated up front so a pending row
-- can be targeted before it has an identity_uid), client_portal_users rows
-- are keyed by client_id: a Client may have at most one non-accepted row
-- at a time, enforced below by a partial unique index rather than an
-- application-level check.

ALTER TABLE client_portal_users ALTER COLUMN identity_uid DROP NOT NULL;
ALTER TABLE client_portal_users ADD COLUMN invite_token uuid UNIQUE;

CREATE UNIQUE INDEX client_portal_users_one_pending_per_client
    ON client_portal_users (client_id)
    WHERE identity_uid IS NULL;

-- ---------------------------------------------------------------------
-- Inviting: a Staff caller scoped to a Practice (app.current_practice_id
-- set by staffauth.Middleware, any role -- per #90's scope decision, invite
-- gating is practice-tier, same as clients_insert in 00005) may insert a
-- pending row for a client_id that has an Engagement at that Practice.
-- Same EXISTS shape as clients_select (00005).
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_insert ON client_portal_users
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

-- Staff SELECT: client_portal_users_self_visibility (00006) only admits
-- rows when *no* practice/client session var is set, so a Staff caller
-- under staffauth.Middleware currently sees zero rows in this table. That
-- would silently break duplicate-invite detection in the invite handler
-- and INSERT ... RETURNING (Postgres applies SELECT policies to the
-- returned row too). Same EXISTS shape as the insert policy above, widened
-- to any row (not just pending) so Staff can also detect an
-- already-accepted row and return 409.
-- +goose StatementBegin
CREATE POLICY client_portal_users_practice_visibility ON client_portal_users
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

-- Re-inviting: rotating invite_token on an existing pending row is an
-- UPDATE, which needs its own door -- neither the insert policy (INSERT
-- only) nor the accept-update policy below (keyed on app.invite_token,
-- which a Staff caller never has) covers it. Restricted to pending rows
-- both before and after the update, so a Staff caller can rotate a token
-- but can never use this door to touch an already-accepted row.
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_update ON client_portal_users
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    )
    WITH CHECK (identity_uid IS NULL);
-- +goose StatementEnd

-- ---------------------------------------------------------------------
-- Accepting: the invitee isn't linked to a Client yet, so this runs before
-- any Practice/Client is chosen -- only app.current_identity_uid is set.
-- Same two-policy shape as staff_accept_invite_select/_update (00004): a
-- pending row is invisible to every other policy in that state, so these
-- are the accept step's own narrow door, gated on the one-time
-- invite_token presented as app.invite_token. The UPDATE policy's WITH
-- CHECK confirms the update only ever sets identity_uid to the caller's
-- own verified uid.
-- +goose StatementBegin
CREATE POLICY client_portal_users_accept_select ON client_portal_users
    FOR SELECT
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    );

CREATE POLICY client_portal_users_accept_update ON client_portal_users
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    )
    WITH CHECK (identity_uid = NULLIF(current_setting('app.current_identity_uid', true), ''));
-- +goose StatementEnd

-- +goose Down
DROP POLICY client_portal_users_accept_update ON client_portal_users;
DROP POLICY client_portal_users_accept_select ON client_portal_users;
DROP POLICY client_portal_users_invite_update ON client_portal_users;
DROP POLICY client_portal_users_practice_visibility ON client_portal_users;
DROP POLICY client_portal_users_invite_insert ON client_portal_users;
DROP INDEX client_portal_users_one_pending_per_client;
ALTER TABLE client_portal_users DROP COLUMN invite_token;
ALTER TABLE client_portal_users ALTER COLUMN identity_uid SET NOT NULL;
