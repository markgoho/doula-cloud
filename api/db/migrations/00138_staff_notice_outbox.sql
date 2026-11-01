-- +goose Up
-- staff_notice_outbox: a Platform Notification (ADR-0009) to one Staff
-- member about something another person did that concerns her, at one
-- Practice (#2267). One table for every such notice, told apart by kind,
-- so a new one is a kind and a Compose case, not a table and a worker.
-- The first kind is an Offer made to someone already at the Practice.
--
-- The row is the audit trail: what was sent (kind and subject_id), to
-- whom (recipient_staff_id, and sent_to_address at send time, since a
-- Staff address can change), when (sent_at), and who caused it
-- (triggered_by_staff_id).
CREATE TYPE staff_notice_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TYPE staff_notice_kind AS ENUM ('offer_received');

CREATE TABLE staff_notice_outbox (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id           uuid NOT NULL REFERENCES practices (id),
    recipient_staff_id    uuid NOT NULL REFERENCES staff (id),
    kind                  staff_notice_kind NOT NULL,
    -- The record the notice is about: an engagement_offers id for
    -- offer_received. Not a foreign key, because each kind points at a
    -- different table.
    subject_id            uuid NOT NULL,
    triggered_by_staff_id uuid REFERENCES staff (id),
    status                staff_notice_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count         int NOT NULL DEFAULT 0,
    next_attempt_at       timestamptz NOT NULL,
    created_at            timestamptz NOT NULL,
    sent_at               timestamptz,
    last_error            text,
    sent_to_address       text
);

-- At most one pending notice per person per record: a replayed request
-- queues nothing twice.
CREATE UNIQUE INDEX staff_notice_outbox_one_pending
    ON staff_notice_outbox (kind, subject_id, recipient_staff_id)
    WHERE status = 'pending';

CREATE INDEX staff_notice_outbox_claim
    ON staff_notice_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON staff_notice_outbox TO app_runtime;

-- No RLS on the outbox itself, platform-level like every other outbox
-- table. The worker reads staff, practice_memberships and
-- engagement_offers through 00033's and 00041's notification-worker
-- policies.

-- +goose Down
DROP TABLE staff_notice_outbox;
DROP TYPE staff_notice_kind;
DROP TYPE staff_notice_outbox_status;
