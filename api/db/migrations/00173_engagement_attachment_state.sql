-- +goose Up
-- An Attachment's state, named once (attachmentstate.State in Go).
--
-- Until this migration nothing named it. "A Doula currently on the
-- Engagement" was restated about two dozen times as origin = 'granted'
-- AND ended_at IS NULL, a handful of readers asked ended_at IS NULL
-- without the origin half, and "the Doula who held this Visit has left"
-- was written out twice in full (visit/off_engagement.go and 00165).
-- Nothing in the schema tied ended_at to ended_by, nothing said why an
-- Attachment ended, and nothing stopped an ended one being reopened or a
-- granted one going back to accrued. An Attachment is in one of three
-- states:
--
--   accrued  -- open, opened by her own work (ADR-0008's AttachingWrite
--               seam): a record of work, never a key.
--   granted  -- open, someone decided she is on it (an Offer's
--               acceptance, a Request's approval, an Owner or an Admin,
--               herself): "who is working this birth", and the only
--               state that reaches.
--   ended    -- closed, never deleted. origin still says which of the
--               two she was in when it ended.
--
-- accrued -> granted is one-way, and ended is terminal. Reopening a
-- completed Engagement opens no new Attachment (engagement/transition.go):
-- the Engagement comes back with no Doula on it until someone is put on
-- it again.
--
-- The state is a STORED generated column for 00171's reason: nearly every
-- reader is SQL, and a generated column is the one derivation all of them
-- read and none of them can restate differently.

-- =====================================================================
-- The state itself
-- =====================================================================

CREATE TYPE attachment_state AS ENUM ('accrued', 'granted', 'ended');

ALTER TABLE engagement_attachments ADD COLUMN state attachment_state GENERATED ALWAYS AS (
    CASE
        WHEN ended_at IS NOT NULL THEN 'ended'::attachment_state
        WHEN origin = 'granted' THEN 'granted'::attachment_state
        ELSE 'accrued'::attachment_state
    END
) STORED;

-- 00030's index, restated on the state, so a reader asking state =
-- 'granted' or state <> 'ended' is answered from it. The predicate is the
-- same set of rows (ended_at IS NULL is exactly state <> 'ended'), so no
-- row the old index admitted can collide under this one.
DROP INDEX engagement_attachments_one_open;
CREATE UNIQUE INDEX engagement_attachments_one_open
    ON engagement_attachments (engagement_id, staff_id)
    WHERE state <> 'ended';

-- =====================================================================
-- Why it ended, beside who and when
-- =====================================================================

-- ADR-0008 names four triggers for ending an Attachment: she drops out,
-- the Practice takes her off, the Engagement completes, or her Membership
-- ends. ended_by said who and nothing on the row said which, so "why did
-- she come off" was answerable only by matching activity rows by time.
CREATE TYPE attachment_ending AS ENUM ('dropped_out', 'taken_off', 'engagement_completed', 'membership_ended');

ALTER TABLE engagement_attachments ADD COLUMN ended_reason attachment_ending;

-- An ending names who ended it. Both writers (staffauth's two, now
-- attachmentstate's) have always set both.
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_ending_is_attributed CHECK (
    (ended_at IS NULL) = (ended_by IS NULL)
);

-- An ending names why. NOT VALID: an Attachment ended before this
-- migration has no reason, and its activity row is still its answer.
-- Nothing can write such a row again (the trigger below), so the
-- constraint holds every ending from here on.
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_ending_has_a_reason CHECK (
    (ended_at IS NULL) = (ended_reason IS NULL)
) NOT VALID;

-- =====================================================================
-- The origin's own rules, on the row
-- =====================================================================

-- ADR-0008: "attached_by equals staff_id when accrued". Accrual is her
-- own work, so nobody else can be its author.
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_accrued_is_her_own CHECK (
    origin <> 'accrued' OR attached_by = staff_id
);

-- A fee is only ever copied from an accepted Offer, which grants (#229).
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_fee_only_when_granted CHECK (
    origin = 'granted' OR (fee_amount_cents IS NULL AND fee_terms IS NULL)
);

-- =====================================================================
-- One way only
-- =====================================================================

-- A CHECK sees one row image; these two rules need both. granted never
-- goes back to accrued (accrual is a record of work, and a downgrade
-- would take away a reach someone decided to give), and an ended
-- Attachment is never reopened or ended a second time: "she was on this
-- from February to May" stays what it says. Which pair and which
-- Engagement a row is about never changes either.
-- +goose StatementBegin
CREATE FUNCTION engagement_attachments_one_way() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public
AS $$
BEGIN
    IF NEW.engagement_id <> OLD.engagement_id OR NEW.staff_id <> OLD.staff_id THEN
        RAISE EXCEPTION 'an Attachment never moves to another Engagement or another Doula';
    END IF;
    IF OLD.origin = 'granted' AND NEW.origin = 'accrued' THEN
        RAISE EXCEPTION 'a granted Attachment never goes back to accrued';
    END IF;
    IF OLD.ended_at IS NOT NULL AND (
        NEW.ended_at IS DISTINCT FROM OLD.ended_at
        OR NEW.ended_by IS DISTINCT FROM OLD.ended_by
        OR NEW.ended_reason IS DISTINCT FROM OLD.ended_reason
        OR NEW.origin IS DISTINCT FROM OLD.origin
    ) THEN
        RAISE EXCEPTION 'an ended Attachment is never reopened or ended again';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER engagement_attachments_one_way
    BEFORE UPDATE ON engagement_attachments
    FOR EACH ROW EXECUTE FUNCTION engagement_attachments_one_way();

-- =====================================================================
-- "The Doula who held this has left", once
-- =====================================================================

-- She has no open Attachment on the Engagement, of either origin, and a
-- granted one has ended. An open accrued one means she is still working
-- it; an ended accrued one alone means she was never put on it. Staff
-- reads (visit/off_engagement.go) call it under their own row security;
-- client_portal_holder_left_engagement below calls it as its owner.
-- SECURITY INVOKER, so it reads nothing its caller could not, and with no
-- SET clause, so the planner inlines it into a Staff Visit list rather
-- than calling it once a row.
-- +goose StatementBegin
CREATE FUNCTION engagement_holder_left(target_engagement uuid, target_staff uuid) RETURNS boolean
    LANGUAGE sql STABLE
    AS $$
        SELECT NOT EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff AND ea.state <> 'ended'
        )
        AND EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff
              AND ea.state = 'ended' AND ea.origin = 'granted'
        )
    $$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION engagement_holder_left(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION engagement_holder_left(uuid, uuid) TO app_runtime;

-- 00165's function, restated in full around the one definition. Its own
-- boundary is unchanged: an Engagement of the current Client's, not
-- completed.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION client_portal_holder_left_engagement(target_engagement uuid, target_staff uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = target_engagement AND e.status <> 'completed'
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
        AND engagement_holder_left(target_engagement, target_staff)
    $$;
-- +goose StatementEnd

-- 00134's function, restated in full on the state.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION engagement_doula_names(target_engagement_id uuid)
RETURNS TABLE (name text)
LANGUAGE sql SECURITY DEFINER STABLE
SET search_path = public, pg_temp
AS $$
    SELECT s.name
    FROM engagements e
    JOIN engagement_attachments ea ON ea.engagement_id = e.id
    JOIN staff s ON s.id = ea.staff_id
    WHERE e.id = target_engagement_id
      AND ea.state = 'granted'
      AND s.deleted_at IS NULL
      AND (
          e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
          OR e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
      )
    ORDER BY ea.attached_at, ea.staff_id
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION engagement_doula_names(target_engagement_id uuid)
RETURNS TABLE (name text)
LANGUAGE sql SECURITY DEFINER STABLE
SET search_path = public, pg_temp
AS $$
    SELECT s.name
    FROM engagements e
    JOIN engagement_attachments ea ON ea.engagement_id = e.id
    JOIN staff s ON s.id = ea.staff_id
    WHERE e.id = target_engagement_id
      AND ea.origin = 'granted'
      AND ea.ended_at IS NULL
      AND s.deleted_at IS NULL
      AND (
          e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
          OR e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
      )
    ORDER BY ea.attached_at, ea.staff_id
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION client_portal_holder_left_engagement(target_engagement uuid, target_staff uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = target_engagement AND e.status <> 'completed'
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
        AND NOT EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff AND ea.ended_at IS NULL
        )
        AND EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff
              AND ea.origin = 'granted' AND ea.ended_at IS NOT NULL
        )
    $$;
-- +goose StatementEnd

DROP FUNCTION engagement_holder_left(uuid, uuid);
DROP TRIGGER engagement_attachments_one_way ON engagement_attachments;
DROP FUNCTION engagement_attachments_one_way();
ALTER TABLE engagement_attachments DROP CONSTRAINT engagement_attachments_fee_only_when_granted;
ALTER TABLE engagement_attachments DROP CONSTRAINT engagement_attachments_accrued_is_her_own;
ALTER TABLE engagement_attachments DROP CONSTRAINT engagement_attachments_ending_has_a_reason;
ALTER TABLE engagement_attachments DROP CONSTRAINT engagement_attachments_ending_is_attributed;
ALTER TABLE engagement_attachments DROP COLUMN ended_reason;
DROP TYPE attachment_ending;
DROP INDEX engagement_attachments_one_open;
CREATE UNIQUE INDEX engagement_attachments_one_open
    ON engagement_attachments (engagement_id, staff_id)
    WHERE ended_at IS NULL;
ALTER TABLE engagement_attachments DROP COLUMN state;
DROP TYPE attachment_state;
