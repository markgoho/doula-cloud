-- +goose Up
-- client_portal_users links a verified Identity Platform uid to the
-- Client it belongs to -- the Client-portal population's equivalent of
-- staff.identity_uid, pulled into its own link table (rather than a
-- column on clients) because a future multi-login Client (a partner or
-- support person, per CONTEXT.md) will need more than one identity_uid
-- per Client. v1 only ever creates one row per Client.

CREATE TABLE client_portal_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL UNIQUE,
    client_id uuid NOT NULL REFERENCES clients (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON client_portal_users TO app_runtime;

ALTER TABLE client_portal_users ENABLE ROW LEVEL SECURITY;

-- Right after login, before the BFF has resolved which Engagement (if
-- any) the caller is scoped to, the caller still needs to look
-- themselves up by their verified identity-provider uid -- the same
-- shape as staff_self_visibility in 00002_practice_staff_tenancy.sql.
-- Guarded against *both* other-tier session variables (not just
-- app.current_client_id) for the same reason the widened policies below
-- are: a person could in principle hold both a staff row and a
-- client_portal_users row under the same identity_uid.
CREATE POLICY client_portal_users_self_visibility ON client_portal_users
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    );

-- Client-tier visibility on engagements, the second of the two RLS tiers
-- named in #47/CONTEXT.md (the first, practice-tier, is
-- engagements_practice_visibility in 00005_client_engagement.sql).
-- Postgres OR's multiple permissive policies on the same table together,
-- so a Staff request (app.current_practice_id set, app.current_client_id
-- never set) and a Client-portal request (the reverse) each only ever
-- match their own policy. FOR SELECT only: nothing in this ticket lets a
-- Client-portal caller create or modify an Engagement.
CREATE POLICY engagements_client_visibility ON engagements
    FOR SELECT
    USING (client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid);

-- ---------------------------------------------------------------------
-- Widening two pre-existing self-visibility policies.
--
-- staff_self_visibility (00002) and practice_memberships_self_visibility
-- (00003) each only guarded against app.current_practice_id being set,
-- which was a complete guard back when app.current_client_id didn't
-- exist as a session variable. Now that this migration introduces it, a
-- person who happens to hold both a staff row and a client_portal_users
-- row under the same identity_uid (e.g. a Practice Owner who is also a
-- Client elsewhere) would otherwise have their own staff row and
-- memberships leak into a Client-portal request, since
-- app.current_practice_id is never set there either. Widen both to also
-- require app.current_client_id be unset -- the RLS backstop only holds
-- if every self-visibility policy accounts for every population, not
-- just the one it was written against.
DROP POLICY staff_self_visibility ON staff;
CREATE POLICY staff_self_visibility ON staff
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    );

DROP POLICY practice_memberships_self_visibility ON practice_memberships;
CREATE POLICY practice_memberships_self_visibility ON practice_memberships
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND staff_id = current_staff_id()
    );

-- +goose Down
DROP POLICY practice_memberships_self_visibility ON practice_memberships;
CREATE POLICY practice_memberships_self_visibility ON practice_memberships
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
    );

DROP POLICY staff_self_visibility ON staff;
CREATE POLICY staff_self_visibility ON staff
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    );

DROP POLICY engagements_client_visibility ON engagements;
DROP POLICY client_portal_users_self_visibility ON client_portal_users;
DROP TABLE client_portal_users;
