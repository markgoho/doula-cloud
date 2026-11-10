-- +goose Up
-- Portal access gets one named state. Until now a client_portal_users
-- row's stage was read off four nullable columns (identity_uid,
-- invite_token, invite_token_expires_at, access_refused_at) with no
-- CHECK between them, so "accepted and refused" was representable and
-- the winner was whichever `if` a reader asked first; an invite whose
-- token a merge or an erasure had nulled still read "Invite sent"; and a
-- revocation left no record of who or when beyond an outbox last_error
-- string.
--
-- The stored states are the ones a writer moves a row into. "Expired" is
-- not stored: it is a function of time, an 'invited' row whose
-- invite_token_expires_at has passed, and portalinvite.Derive is the one
-- place that reads it (with the instant from the clock seam). The legal
-- moves are declared once, as portalinvite.Transitions; the CHECKs below
-- refuse every column combination a state does not own.
--
--   invited  -- a live invitation: a token and its expiry
--   refused  -- the same live invitation, whose accept ADR-0015 refused
--               because her login already reaches another Client here
--               (#2522); re-inviting or a later accept can still move it
--   accepted -- her Portal Account reaches this Client
--   revoked  -- the invitation was stopped before it was accepted: the
--               contact address changed, the record was merged into
--               another, or the Client was erased. Terminal: a new
--               invitation is a new row, so this one keeps its record.
--   unlinked -- an accepted row whose Portal Account no longer reaches
--               this Client, because the Client was erased. Terminal.
CREATE TYPE portal_access_state AS ENUM ('invited', 'refused', 'accepted', 'revoked', 'unlinked');

-- Why a revocation happened, in the record itself rather than only in a
-- dead-lettered outbox row's last_error (portalinvite.RevokePending).
CREATE TYPE portal_access_revoke_reason AS ENUM ('email_changed', 'record_merged', 'client_erased');

ALTER TABLE client_portal_users ADD COLUMN state portal_access_state;
ALTER TABLE client_portal_users ADD COLUMN revoked_at timestamptz;
-- NULL on a revoked row means DoulaCloud did it with nobody asking: a
-- Practice deletion erases every Client under activity.SystemActor.
ALTER TABLE client_portal_users ADD COLUMN revoked_by_staff_id uuid REFERENCES staff (id);
ALTER TABLE client_portal_users ADD COLUMN revoke_reason portal_access_revoke_reason;
ALTER TABLE client_portal_users ADD COLUMN unlinked_at timestamptz;

-- Every row already written takes the state its columns describe.
-- Pre-launch, so these are walk and test leftovers (CLAUDE.md); the
-- statement exists so the migration applies wherever rows exist, and
-- clears the columns a state does not own so the CHECKs below hold.
-- A token-less, identity-less row was either an invitation a merge or
-- an erasure revoked, or a login an erasure (or 00073's reconciliation)
-- took away; only a merged record's row can be told apart, so every
-- other one reads as unlinked, dated from the erasure where there was
-- one. An invited row with no expiry could never be accepted
-- (portalinvite.acceptInvite), so it is given one already in the past.
UPDATE client_portal_users pu
   SET state = s.state,
       invite_token = CASE WHEN s.state IN ('invited', 'refused') THEN pu.invite_token END,
       invite_token_expires_at = CASE WHEN s.state IN ('invited', 'refused')
                                      THEN COALESCE(pu.invite_token_expires_at, pu.created_at) END,
       access_refused_at = CASE WHEN s.state = 'refused' THEN pu.access_refused_at END,
       revoked_at = CASE WHEN s.state = 'revoked' THEN COALESCE(s.merged_at, pu.created_at) END,
       revoked_by_staff_id = CASE WHEN s.state = 'revoked' THEN s.merged_by_staff_id END,
       revoke_reason = CASE WHEN s.state = 'revoked' THEN 'record_merged'::portal_access_revoke_reason END,
       unlinked_at = CASE WHEN s.state = 'unlinked' THEN COALESCE(s.erased_at, pu.created_at) END
  FROM (
      SELECT p.id, c.merged_at, c.merged_by_staff_id, c.erased_at,
             CASE
                 WHEN p.identity_uid IS NOT NULL THEN 'accepted'
                 WHEN p.invite_token IS NOT NULL AND p.access_refused_at IS NOT NULL THEN 'refused'
                 WHEN p.invite_token IS NOT NULL THEN 'invited'
                 WHEN c.merged_into IS NOT NULL THEN 'revoked'
                 ELSE 'unlinked'
             END::portal_access_state AS state
        FROM client_portal_users p
        JOIN clients c ON c.id = p.client_id
  ) s
 WHERE s.id = pu.id;

ALTER TABLE client_portal_users ALTER COLUMN state SET NOT NULL;

-- Each state owns exactly its columns. Written one fact per constraint,
-- the shape engagement_requests' CHECKs (00042) take, so a refusal names
-- the fact that was wrong.
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_accepted_has_identity
    CHECK ((state = 'accepted') = (identity_uid IS NOT NULL));
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_open_invite_has_token
    CHECK ((state IN ('invited', 'refused')) = (invite_token IS NOT NULL));
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_open_invite_has_expiry
    CHECK ((state IN ('invited', 'refused')) = (invite_token_expires_at IS NOT NULL));
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_refusal
    CHECK ((state = 'refused') = (access_refused_at IS NOT NULL));
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_revocation
    CHECK (
        (state = 'revoked') = (revoked_at IS NOT NULL)
        AND (state = 'revoked') = (revoke_reason IS NOT NULL)
        AND (state = 'revoked' OR revoked_by_staff_id IS NULL)
    );
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_unlink
    CHECK ((state = 'unlinked') = (unlinked_at IS NOT NULL));

-- At most one live invitation per Client. 00026's index keyed "pending"
-- on identity_uid IS NULL, which also caught revoked and unlinked rows:
-- erasing a Client a merge had left holding two rows -- the survivor's
-- own invitation and the absorbed record's accepted login -- nulled
-- identity_uid on both and the erasure failed on this index. Revoked and
-- unlinked rows are history, and a Client may hold any number of them.
DROP INDEX client_portal_users_one_pending_per_client;
CREATE UNIQUE INDEX client_portal_users_one_open_invite_per_client
    ON client_portal_users (client_id)
    WHERE state IN ('invited', 'refused');

-- accepted -> unlinked has one cause that no Go code can name: deleting a
-- Portal Account, whose FK (00073) sets identity_uid NULL on every row
-- that names it as the DELETE's own side effect. So the database writes
-- this one transition itself, for that cascade and for erasure's own
-- `SET identity_uid = NULL`, and portalinvite.TransitionUnlink declares
-- it. now() rather than an instant from Go because the cascade gives Go
-- no seam to pass one through; within an erasure it is that
-- transaction's own start.
-- +goose StatementBegin
CREATE FUNCTION client_portal_users_unlink() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public
AS $$
BEGIN
    IF OLD.state = 'accepted' AND NEW.state = 'accepted' AND NEW.identity_uid IS NULL THEN
        NEW.state := 'unlinked';
        NEW.unlinked_at := now();
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER client_portal_users_unlink
    BEFORE UPDATE OF identity_uid ON client_portal_users
    FOR EACH ROW EXECUTE FUNCTION client_portal_users_unlink();

-- The Staff doors (00042) restated against the state rather than
-- identity_uid. Inviting inserts only a live invitation.
DROP POLICY client_portal_users_invite_insert ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_insert ON client_portal_users
    FOR INSERT
    WITH CHECK (
        state = 'invited'
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

-- Re-inviting rotates a live invitation; revoking stops one. Both start
-- from a live invitation and neither can reach an accepted row, which
-- stays immutable through the Staff door as 00026 intended.
DROP POLICY client_portal_users_invite_update ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_update ON client_portal_users
    FOR UPDATE
    USING (
        state IN ('invited', 'refused')
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    )
    WITH CHECK (state IN ('invited', 'revoked'));
-- +goose StatementEnd

-- 00140's function, now moving the row into 'refused' rather than only
-- stamping a column beside an unchanged stage. It starts from a live
-- invitation, as portalinvite.TransitionRefuse declares.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION mark_portal_invite_refused(p_invite_token uuid, p_at timestamptz)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    UPDATE client_portal_users
       SET state = 'refused',
           access_refused_at = p_at
     WHERE state IN ('invited', 'refused')
       AND invite_token = p_invite_token
$$;
-- +goose StatementEnd

-- +goose Down
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

DROP POLICY client_portal_users_invite_update ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_update ON client_portal_users
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    )
    WITH CHECK (identity_uid IS NULL);
-- +goose StatementEnd

DROP POLICY client_portal_users_invite_insert ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_insert ON client_portal_users
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

DROP TRIGGER client_portal_users_unlink ON client_portal_users;
DROP FUNCTION client_portal_users_unlink();

DROP INDEX client_portal_users_one_open_invite_per_client;
CREATE UNIQUE INDEX client_portal_users_one_pending_per_client
    ON client_portal_users (client_id)
    WHERE identity_uid IS NULL;

ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_unlink;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_revocation;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_refusal;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_open_invite_has_expiry;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_open_invite_has_token;
ALTER TABLE client_portal_users DROP CONSTRAINT client_portal_users_accepted_has_identity;

ALTER TABLE client_portal_users DROP COLUMN unlinked_at;
ALTER TABLE client_portal_users DROP COLUMN revoke_reason;
ALTER TABLE client_portal_users DROP COLUMN revoked_by_staff_id;
ALTER TABLE client_portal_users DROP COLUMN revoked_at;
ALTER TABLE client_portal_users DROP COLUMN state;

DROP TYPE portal_access_revoke_reason;
DROP TYPE portal_access_state;
