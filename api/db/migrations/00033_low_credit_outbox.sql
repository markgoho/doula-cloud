-- +goose Up
-- Outbox for the "Practice is out of Credits" Platform Notification
-- (#342, ADR-0010, map #213). Unlike portal_invite_outbox (00032), this
-- is keyed to practice_id, not to a single Client-facing row: the
-- recipient is every Owner of the Practice, resolved at send time via
-- staff/practice_memberships, never stored on the row.
--
-- One row per "ran out of Credits" episode, not one per failed create
-- attempt: engagement.CreateHandler only queues a new row when no
-- existing row for the Practice was created after its most recent
-- 'purchase' credit_ledger entry (or ever, if none exists) -- so an
-- Owner is notified once per wall hit, not once per subsequent retry,
-- and is notified again after buying more Credits and running out a
-- second time.
CREATE TYPE low_credit_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE low_credit_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    status low_credit_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per Practice, guarding the race between two
-- concurrent CreateHandler requests that both observe a zero balance
-- before either commits: engagement.QueueOutOfCreditsNotification's
-- INSERT carries ON CONFLICT DO NOTHING against this index.
CREATE UNIQUE INDEX low_credit_outbox_one_pending
    ON low_credit_outbox (practice_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON low_credit_outbox TO app_runtime;

-- Platform-level like portal_invite_outbox: the worker runs with no
-- Practice session, so this table carries no RLS of its own. It does
-- need staff/practice_memberships visibility, though, to resolve which
-- Staff hold the owner role -- reusing 00032's
-- app.notification_worker_trusted session var (the same trusted-worker
-- context, applied to two more tables) rather than minting a new one.
-- +goose StatementBegin
CREATE POLICY staff_notification_worker ON staff
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');

CREATE POLICY practice_memberships_notification_worker ON practice_memberships
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY practice_memberships_notification_worker ON practice_memberships;
DROP POLICY staff_notification_worker ON staff;
DROP TABLE low_credit_outbox;
DROP TYPE low_credit_outbox_status;
