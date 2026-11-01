-- +goose Up
-- #2522: an accept ADR-0015 refuses (her Portal Account already reaches
-- another Client at this Practice, #2279) leaves a fact the invite's
-- status can read, so the Practice sees "Invite refused" and not
-- "Invite pending" for an invite that can never be accepted. Set in the
-- refused accept's own kept transaction; cleared when InviteHandler
-- rotates the token, since a new invite is a new question.
ALTER TABLE client_portal_users ADD COLUMN access_refused_at timestamptz;

-- SECURITY DEFINER, mirroring portal_account_reuse_for_accept (00081):
-- the accept runs pre-account, and client_portal_users_accept_update's
-- WITH CHECK (00026) admits only an UPDATE that sets identity_uid to the
-- caller's, so no policy lets it mark the row while leaving it pending.
-- A purpose-built function rather than a wider policy: it writes this
-- one column, on the one pending row the presented token names.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION mark_portal_invite_refused(p_invite_token uuid, p_at timestamptz)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    UPDATE client_portal_users
       SET access_refused_at = p_at
     WHERE identity_uid IS NULL
       AND invite_token = p_invite_token
$$;
-- +goose StatementEnd
REVOKE EXECUTE ON FUNCTION mark_portal_invite_refused(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION mark_portal_invite_refused(uuid, timestamptz) TO app_runtime;

-- +goose Down
DROP FUNCTION mark_portal_invite_refused(uuid, timestamptz);
ALTER TABLE client_portal_users DROP COLUMN access_refused_at;
