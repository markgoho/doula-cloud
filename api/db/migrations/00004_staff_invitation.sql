-- +goose Up
-- Staff invitation: a Practice Owner invites another person to their
-- Practice. The invited person doesn't have an Identity Platform account
-- yet, so "staff" needs a way to represent someone who has been invited
-- but hasn't signed in for the first time. That's a "pending" staff row:
-- identity_uid is NULL until the invitee accepts, at which point it's set
-- to their verified uid and the row becomes a normal, active staff row.
--
-- The practice_memberships row is created at invite time (not at accept
-- time) with zero roles, per 00002's comment -- an Owner must assign at
-- least one role before the invite is useful for anything beyond landing
-- on the Practice.
--
-- There's no email-sending infrastructure in this repo yet, so "sending
-- an invite" just means handing the Owner an accept link containing a
-- one-time, unguessable invite_token to pass along outside the app (a
-- follow-up ticket can wire up real email delivery without changing this
-- schema).

ALTER TABLE staff ALTER COLUMN identity_uid DROP NOT NULL;
ALTER TABLE staff ADD COLUMN invite_token uuid UNIQUE;

-- ---------------------------------------------------------------------
-- Inviting: an authenticated Owner, already scoped to a Practice via
-- staffauth.Middleware (so app.current_practice_id and
-- app.current_identity_uid are both set), inserts a new pending staff
-- row and a practice_memberships row for it.
--
-- staff_self_insert (00003) requires identity_uid to equal the caller's
-- own uid -- wrong for a pending row, which has no identity_uid yet.
-- staff_practice_visibility (00002) is USING-only, so Postgres reuses it
-- as the INSERT check too, but it requires a practice_memberships row
-- for the new staff id that can't exist until after this INSERT. Neither
-- existing policy fits an invite INSERT, so this adds a third one:
-- allowed only when the caller already holds a practice_memberships row
-- with the 'owner' role at the current Practice.
--
-- current_staff_id() (00003) is SECURITY DEFINER, so this subquery does
-- not re-trigger staff's own RLS policies -- the same trick 00003 uses to
-- avoid "infinite recursion detected in policy".
-- +goose StatementBegin
CREATE POLICY staff_invite_insert ON staff
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.staff_id = current_staff_id()
              AND 'owner' = ANY (pm.roles)
        )
    );
-- +goose StatementEnd

-- ---------------------------------------------------------------------
-- Accepting: the invitee isn't a Staff member of any Practice yet, so
-- this runs before a Practice is chosen -- like signup/session, only
-- app.current_identity_uid is set. A pending row is invisible to both
-- existing SELECT/UPDATE-relevant policies in that state
-- (staff_practice_visibility needs a membership that doesn't exist for
-- this caller; staff_self_visibility matches on identity_uid, which the
-- pending row doesn't have yet). These two policies are the accept
-- step's own narrow door: an UPDATE also requires SELECT permission on
-- the row being updated (Postgres combines the SELECT- and
-- UPDATE-applicable policies), so both are needed even though only the
-- UPDATE ever runs. Both admit a pending row only by presenting the
-- one-time invite_token handed out at invite time, set on the session as
-- app.invite_token. The UPDATE policy's WITH CHECK confirms the update
-- only ever sets identity_uid to the caller's own verified uid.
-- +goose StatementBegin
CREATE POLICY staff_accept_invite_select ON staff
    FOR SELECT
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    );

CREATE POLICY staff_accept_invite_update ON staff
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    )
    WITH CHECK (identity_uid = NULLIF(current_setting('app.current_identity_uid', true), ''));
-- +goose StatementEnd

-- +goose Down
DROP POLICY staff_accept_invite_update ON staff;
DROP POLICY staff_accept_invite_select ON staff;
DROP POLICY staff_invite_insert ON staff;
ALTER TABLE staff DROP COLUMN invite_token;
ALTER TABLE staff ALTER COLUMN identity_uid SET NOT NULL;
