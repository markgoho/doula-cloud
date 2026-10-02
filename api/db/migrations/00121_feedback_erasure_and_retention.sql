-- +goose Up
-- #1525: what happens to a piece of Feedback over its life, as #1501
-- (Q3, Q4) decided and CONTEXT.md's Feedback entry records. A Client's
-- Erasure destroys her Feedback outright and closes its issues with the
-- `erased` label; a piece nothing destroys is deleted 24 months after it
-- was sent, and its issue stays as it is.
--
-- Three things this migration makes possible, none of which 00118 or
-- 00120 could do: a feedback row can be deleted at all, the outbox can
-- carry an act that outlives the row it was about, and each of the two
-- DELETEs has a door that admits exactly that act.

-- =====================================================================
-- feedback_issue_outbox: a second act, and a row that outlives its piece
-- =====================================================================

-- 'open' is 00120's only act. 'close_erased' closes an issue and adds
-- the `erased` label. The same enum-of-acts shape client_erasure_outbox
-- (00064) and practice_deletion_outbox already take.
CREATE TYPE feedback_issue_act AS ENUM ('open', 'close_erased');

-- DEFAULT 'open', so every row 00120 already wrote is an open job, which
-- is what it always was.
ALTER TABLE feedback_issue_outbox ADD COLUMN act feedback_issue_act NOT NULL DEFAULT 'open';

-- The trap #1525 names: the issue number lives on the feedback row the
-- Erasure deletes. So a close job carries the number itself and no
-- reference to a row that is gone by the time the worker runs.
ALTER TABLE feedback_issue_outbox ADD COLUMN issue_number integer;

-- The foreign key goes, and feedback_id stays as a plain id. Both
-- DELETEs below would otherwise be refused by any outbox row, sent or
-- pending, that names the piece. ON DELETE SET NULL was the other
-- choice, and it loses the one fact a pending open job still needs once
-- its piece is gone: the id inside the issue's hidden marker, which is
-- how the worker finds an issue GitHub already opened on an attempt that
-- failed afterward (#1500's crash window) and closes it rather than
-- leaving it open forever. The id is opaque and names nobody -- the
-- issue's own body already carries it.
ALTER TABLE feedback_issue_outbox DROP CONSTRAINT feedback_issue_outbox_feedback_id_fkey;
ALTER TABLE feedback_issue_outbox ALTER COLUMN feedback_id DROP NOT NULL;

-- One shape per act, so a close job that names a piece, or an open job
-- that names an issue, is impossible rather than merely undocumented.
-- Every row 00120 wrote passes: act defaulted to 'open' just above,
-- feedback_id was NOT NULL until the statement before this one, and
-- issue_number is new and NULL.
ALTER TABLE feedback_issue_outbox ADD CONSTRAINT feedback_issue_outbox_act_shape CHECK (
    (act = 'open' AND feedback_id IS NOT NULL AND issue_number IS NULL)
    OR
    (act = 'close_erased' AND feedback_id IS NULL AND issue_number IS NOT NULL)
);

-- At most one pending close per issue, so a repeat enqueue is a no-op
-- (ON CONFLICT DO NOTHING) rather than a second call to GitHub -- the
-- same idiom client_erasure_outbox's own partial unique index serves.
-- 00120's feedback_issue_outbox_one_pending needs no change: a close job
-- has no feedback_id, and NULLs never collide in a unique index.
CREATE UNIQUE INDEX feedback_issue_outbox_one_pending_close
    ON feedback_issue_outbox (issue_number)
    WHERE status = 'pending' AND act = 'close_erased';

-- =====================================================================
-- Erasure: two purpose-built doors, not a DELETE policy
-- =====================================================================
--
-- client.Erase runs as app_runtime inside a Staff transaction (or the
-- Practice-deletion worker's), and a Staff member may not read a piece
-- of Feedback at all (00118). A DELETE ... RETURNING issue_number under
-- a policy would need a paired SELECT policy that admits a Practice to
-- its Clients' Feedback, which is the read 00118 refuses. So each delete
-- is a SECURITY DEFINER function in the redact_absorbed_client (00112)
-- shape: it does one act, re-derives every fact it relies on, and opens
-- no door onto any row.
--
-- Each function deletes and enqueues in ONE statement. The close job is
-- built from the DELETE's own RETURNING, so the issue number it carries
-- is the one on the row at the instant it was deleted -- including a
-- number the outbox worker wrote a moment earlier in a transaction this
-- DELETE had to wait for. A read followed by a delete could miss that
-- number and leave an issue open forever.

-- A Client's own pieces: every row with her client_id. Admitted only
-- when she is a Client of the Practice in this transaction and is
-- already erased -- client.Erase calls this after redactRecord (and,
-- for a merged record, after redact_absorbed_client) has stamped
-- erased_at, so the precondition is a row this transaction wrote rather
-- than a claim the caller makes.
-- +goose StatementBegin
CREATE FUNCTION erase_client_feedback(p_client_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM clients
         WHERE id = p_client_id
           AND practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
           AND erased_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'a Client''s Feedback is destroyed only by the Erasure of that Client';
    END IF;

    WITH gone AS (
        DELETE FROM feedback WHERE client_id = p_client_id RETURNING issue_number
    )
    INSERT INTO feedback_issue_outbox (act, issue_number)
    SELECT 'close_erased', issue_number FROM gone WHERE issue_number IS NOT NULL
    ON CONFLICT (issue_number) WHERE status = 'pending' AND act = 'close_erased' DO NOTHING;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION erase_client_feedback(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_client_feedback(uuid) TO app_runtime;

-- The pieces keyed only to the Portal Account (sent from a screen with
-- no single Engagement in it) go when the Portal Account row itself
-- goes. The precondition is word for word portal_accounts_erasure_delete
-- (00108): no un-erased Client anywhere still reaches the login, and an
-- erased Client of the Practice in this transaction does. So this is
-- admitted exactly when the DELETE FROM portal_accounts that follows it
-- is, and refused exactly when that would be.
--
-- It takes every row that names the login, not only those with no
-- client_id. By this point a row with a client_id belongs to a Client
-- who is already erased, whose own erasure already took it; the wider
-- predicate costs nothing and means feedback.portal_account's foreign
-- key can never be what refuses the Portal Account's own delete.
-- +goose StatementBegin
CREATE FUNCTION erase_portal_account_feedback(p_identifier text)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF portal_account_reaches_a_live_client(p_identifier) OR NOT EXISTS (
        SELECT 1 FROM client_portal_users pu
          JOIN clients c ON c.id = pu.client_id
         WHERE pu.identity_uid = p_identifier
           AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
           AND c.erased_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'a Portal Account''s Feedback is destroyed only when the Portal Account is';
    END IF;

    WITH gone AS (
        DELETE FROM feedback WHERE portal_account = p_identifier RETURNING issue_number
    )
    INSERT INTO feedback_issue_outbox (act, issue_number)
    SELECT 'close_erased', issue_number FROM gone WHERE issue_number IS NOT NULL
    ON CONFLICT (issue_number) WHERE status = 'pending' AND act = 'close_erased' DO NOTHING;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION erase_portal_account_feedback(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_portal_account_feedback(text) TO app_runtime;

-- =====================================================================
-- Retention: the worker's own DELETE
-- =====================================================================
--
-- The feedback issue worker deletes every piece sent more than 24
-- months ago at the start of each run (#1501 Q4). No new Scheduler job:
-- the drain is the only scheduled tick (ADR-0010 as amended by #481).
-- This is a plain DELETE as app_runtime, so it needs the grant and a
-- policy -- behind the same trusted-worker door 00120's SELECT and
-- UPDATE already use. It enqueues nothing: the issue stays as it is.
GRANT DELETE ON feedback TO app_runtime;

-- +goose StatementBegin
CREATE POLICY feedback_notification_worker_delete ON feedback
    FOR DELETE
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- What the retention DELETE's WHERE clause reads.
CREATE INDEX feedback_sent_at ON feedback (sent_at);

-- +goose Down
DROP INDEX feedback_sent_at;
DROP POLICY feedback_notification_worker_delete ON feedback;
REVOKE DELETE ON feedback FROM app_runtime;

REVOKE EXECUTE ON FUNCTION erase_portal_account_feedback(text) FROM app_runtime;
DROP FUNCTION erase_portal_account_feedback(text);
REVOKE EXECUTE ON FUNCTION erase_client_feedback(uuid) FROM app_runtime;
DROP FUNCTION erase_client_feedback(uuid);

-- A close job, and an open job whose piece is gone, are rows 00120's
-- shape cannot hold.
DELETE FROM feedback_issue_outbox
 WHERE act = 'close_erased'
    OR feedback_id NOT IN (SELECT id FROM feedback);

DROP INDEX feedback_issue_outbox_one_pending_close;
ALTER TABLE feedback_issue_outbox DROP CONSTRAINT feedback_issue_outbox_act_shape;
ALTER TABLE feedback_issue_outbox ALTER COLUMN feedback_id SET NOT NULL;
ALTER TABLE feedback_issue_outbox
    ADD CONSTRAINT feedback_issue_outbox_feedback_id_fkey FOREIGN KEY (feedback_id) REFERENCES feedback (id);
ALTER TABLE feedback_issue_outbox DROP COLUMN issue_number;
ALTER TABLE feedback_issue_outbox DROP COLUMN act;
DROP TYPE feedback_issue_act;
