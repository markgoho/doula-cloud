-- +goose Up
-- #1526: the founder read page (#1499, Option 1). 00118 said the founder
-- read page "adds its own grant and policy when it is built"; this is
-- that migration. Three things: a door onto a piece of Feedback for the
-- founder's read, a function that names the sender and the Practice
-- without opening a door onto either table, and the record of each read.

-- =====================================================================
-- The door: app.feedback_reader
-- =====================================================================
--
-- staffauth.FounderOnly sets app.feedback_reader to the founder's
-- staff id, transaction-local, only after it has matched the signed-in
-- Staff member against FOUNDER_STAFF_ID and seen a second factor on her
-- session -- the shape billing/dormancy.go uses for
-- app.current_practice_id. Postgres cannot know who the founder is, so
-- that decision stays in the BFF. What Postgres can check is that the
-- setting names the Staff member this transaction resolved
-- (current_staff_id(), 00003), so a transaction cannot open the door in
-- another person's name.
--
-- NULL when the setting is absent or empty, and `NULL = x` is not true,
-- so every policy below fails closed.
CREATE FUNCTION feedback_reader() RETURNS uuid
    LANGUAGE sql STABLE
    AS $$
        SELECT NULLIF(current_setting('app.feedback_reader', true), '')::uuid
    $$;

-- SELECT is already granted to app_runtime (00120, for the issue
-- worker). This is the second policy that admits one, and the only one a
-- person's session can satisfy.
-- +goose StatementBegin
CREATE POLICY feedback_reader_select ON feedback
    FOR SELECT
    USING (feedback_reader() = (SELECT current_staff_id()));
-- +goose StatementEnd

-- What the list's join reads: the open job of one piece. 00120's two
-- indexes are both partial on status = 'pending', and a dead-lettered
-- job is exactly the row the list has to find.
CREATE INDEX feedback_issue_outbox_feedback_id ON feedback_issue_outbox (feedback_id);

-- =====================================================================
-- The sender and the Practice, with no door onto either table
-- =====================================================================
--
-- The read page shows the sender's name and address and the Practice's
-- name. staff, portal_accounts and practices each admit only the
-- caller's own row or the current Practice's, and this read has no
-- Practice. A policy on each of the three would be a door onto every
-- Staff member, every Portal Account and every Practice, evaluated on
-- every query those tables ever serve. So this is one SECURITY DEFINER
-- function in the erase_client_feedback (00121) shape: it re-checks the
-- same door, answers for one piece, and returns only the three facts the
-- page prints.
--
-- sender_name is NULL for a Portal sender: the Portal Account is a
-- login, and the sign-in address is what names it (#1526's own AC).
-- +goose StatementBegin
CREATE FUNCTION feedback_sender(p_feedback_id uuid)
RETURNS TABLE (sender_name text, sender_email text, practice_name text)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT s.name, COALESCE(s.email, pa.sign_in_address), p.name
      FROM feedback f
      LEFT JOIN staff s ON s.id = f.staff_id
      LEFT JOIN portal_accounts pa ON pa.identifier = f.portal_account
      LEFT JOIN practices p ON p.id = f.practice_id
     WHERE f.id = p_feedback_id
       AND feedback_reader() = current_staff_id();
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION feedback_sender(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION feedback_sender(uuid) TO app_runtime;

-- =====================================================================
-- feedback_reads: every open of a piece
-- =====================================================================
--
-- CONTEXT.md's Feedback entry: "every time the founder reads one is
-- recorded". One row per open of /feedback/[feedbackId]: who, and when
-- (clock.Now(ctx), #773). ON DELETE CASCADE is "deleted with its piece":
-- an Erasure (00121's two functions) and the retention DELETE both take
-- the reads with the piece, and neither has to know this table exists.
-- A cascade runs as this table's owner, so app_runtime needs no DELETE
-- grant here for the retention worker's own DELETE to carry it.
CREATE TABLE feedback_reads (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    feedback_id uuid NOT NULL REFERENCES feedback (id) ON DELETE CASCADE,
    staff_id    uuid NOT NULL REFERENCES staff (id),
    read_at     timestamptz NOT NULL
);

-- What the cascade reads.
CREATE INDEX feedback_reads_feedback_id ON feedback_reads (feedback_id);

-- INSERT only: the read page writes a row and never reads one back.
GRANT INSERT ON feedback_reads TO app_runtime;

ALTER TABLE feedback_reads ENABLE ROW LEVEL SECURITY;

-- A row names the reader herself and nobody else, behind the same door.
-- +goose StatementBegin
CREATE POLICY feedback_reads_reader_insert ON feedback_reads
    FOR INSERT
    WITH CHECK (staff_id = feedback_reader() AND staff_id = current_staff_id());
-- +goose StatementEnd

-- +goose Down
DROP TABLE feedback_reads;
REVOKE EXECUTE ON FUNCTION feedback_sender(uuid) FROM app_runtime;
DROP FUNCTION feedback_sender(uuid);
DROP INDEX feedback_issue_outbox_feedback_id;
DROP POLICY feedback_reader_select ON feedback;
DROP FUNCTION feedback_reader();
