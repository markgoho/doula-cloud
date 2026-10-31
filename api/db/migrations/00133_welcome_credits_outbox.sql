-- +goose Up
-- The one email to a Practice that signed up and started no Engagement,
-- fourteen days after signup (#1381). Decided on #1372 (Q3, Q10, P3) and
-- recorded in ADR-0043's third section, "A Practice that starts nothing
-- gets one message, on a date her signup fixed": her signup is the act,
-- and the date is its consequence.
--
-- Shaped after practice_deletion_outbox (00088): the row is written in
-- the signup transaction with next_attempt_at fourteen days out, and the
-- every-five-minute drain fires it. Nothing scans for quiet Practices
-- (ADR-0038). At send the worker rechecks the Practice live and skips,
-- marking the row sent with nothing mailed, when there is anything to
-- skip for (ADR-0031's skip-at-send recheck).
CREATE TYPE welcome_credits_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE welcome_credits_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- One row per Practice, ever: the message is one message and not a
    -- sequence.
    practice_id uuid NOT NULL UNIQUE REFERENCES practices (id),
    status welcome_credits_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    -- When the worker acted: mailed, or skipped.
    sent_at timestamptz,
    last_error text,
    -- Why a row was marked sent with nothing mailed. NULL on a row that
    -- mailed.
    skip_reason text CHECK (skip_reason IN (
        'engagement_started',
        'founding_grant',
        'practice_deleting',
        'owners_suppressed',
        'no_owners'
    )),
    -- Whom it reached, written at send time as connect_nudge_outbox
    -- (00109) does: a record of what the resolution produced, never an
    -- instruction to the worker.
    notified_owner_staff_ids uuid[]
);

CREATE INDEX welcome_credits_outbox_claim
    ON welcome_credits_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON welcome_credits_outbox TO app_runtime;

-- No RLS, platform-level like every other outbox table.

-- +goose Down
DROP TABLE welcome_credits_outbox;
DROP TYPE welcome_credits_outbox_status;
