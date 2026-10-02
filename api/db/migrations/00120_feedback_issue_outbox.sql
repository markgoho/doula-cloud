-- +goose Up
-- #1524's outbox: one row per piece of Feedback that still needs an
-- issue opened in the private GitHub repo (#1519, #1500's resolution
-- for the shape, #1501 Q1 for which facts cross). 00118 already gave
-- feedback its own nullable issue_number column for the worker to fill
-- in; this migration is what lets it.
--
-- Same row-for-row shape as client_erasure_outbox (00064) and
-- low_credit_outbox (00033): id, a status enum, the backoff bookkeeping,
-- and nothing about the act itself beyond feedback_id, since everything
-- the worker needs to build an issue lives on the feedback row it points
-- at.
CREATE TYPE feedback_issue_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE feedback_issue_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    feedback_id     uuid NOT NULL REFERENCES feedback (id),
    status          feedback_issue_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

-- At most one pending row per piece of Feedback: both send handlers
-- enqueue exactly once, in the same transaction as the Feedback insert,
-- and nothing else ever writes here.
CREATE UNIQUE INDEX feedback_issue_outbox_one_pending
    ON feedback_issue_outbox (feedback_id)
    WHERE status = 'pending';

CREATE INDEX feedback_issue_outbox_claim
    ON feedback_issue_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON feedback_issue_outbox TO app_runtime;

-- No RLS on this table -- platform-level like client_erasure_outbox and
-- low_credit_outbox: the worker runs with no Practice or Client session
-- context.

-- =====================================================================
-- feedback: the worker's own read, and its one write
-- =====================================================================
--
-- 00118 granted app_runtime INSERT only on feedback -- "neither a Staff
-- member nor a Client may read a piece of Feedback back" -- because the
-- only two surfaces that would ever touch this table again were the
-- outbox worker (here) and the founder read page (#1499), and neither
-- had a grant yet. This is the first of those two: SELECT, so the
-- worker can build an issue's title and body off the row (kind,
-- route_id, app_build, screen_width, browser, sent_at, roles,
-- staff_id/portal_account -- never text, which the worker's own claim
-- query does not even select), and UPDATE on issue_number alone, the
-- one column an outside act may ever change on a piece of Feedback.
--
-- Both are gated by the same trusted-worker door 00033 minted
-- (app.notification_worker_trusted), reused rather than a new session
-- variable: a piece of Feedback is not mail, but the door means exactly
-- the same thing here -- no Staff or Client session is behind this
-- read, the outbox is reading and writing in the caller's place.
GRANT SELECT ON feedback TO app_runtime;
GRANT UPDATE (issue_number) ON feedback TO app_runtime;

-- +goose StatementBegin
CREATE POLICY feedback_notification_worker_select ON feedback
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');

CREATE POLICY feedback_notification_worker_update ON feedback
    FOR UPDATE
    USING (current_setting('app.notification_worker_trusted', true) = 'true')
    WITH CHECK (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY feedback_notification_worker_update ON feedback;
DROP POLICY feedback_notification_worker_select ON feedback;
REVOKE UPDATE (issue_number) ON feedback FROM app_runtime;
REVOKE SELECT ON feedback FROM app_runtime;

DROP TABLE feedback_issue_outbox;
DROP TYPE feedback_issue_outbox_status;
