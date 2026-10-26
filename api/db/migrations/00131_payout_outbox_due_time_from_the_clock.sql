-- +goose Up
-- #1992: payout_outbox's first due time is written by the one insert that
-- queues a row, payments.QueuePayoutIncompleteNotification, as
-- clock.Now(ctx) plus the 48-hour grace window, rather than by 00034's
-- DEFAULT now() + interval '48 hours'. A business rule reads the instant
-- through api/internal/clock, so a moved clock moves the grace window.
--
-- The DEFAULT is dropped rather than left beside the insert, so a later
-- insert that leaves next_attempt_at out is refused by NOT NULL instead
-- of quietly taking the database's clock.
ALTER TABLE payout_outbox ALTER COLUMN next_attempt_at DROP DEFAULT;

-- +goose Down
ALTER TABLE payout_outbox ALTER COLUMN next_attempt_at SET DEFAULT now() + interval '48 hours';
