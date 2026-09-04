-- +goose Up
-- #394's redaction-eligibility date needs a column of its own.
--
-- 00064 read it back off next_attempt_at, on the reasoning that the date
-- shown should be the date the worker will act on rather than a second
-- calculation that could drift. That was wrong in a way review caught:
-- next_attempt_at is exactly what drifts. outbox.MarkFailed rewrites it
-- to now + backoff on every failed attempt, so the first failure turns
-- the Practice-facing "redactable after [date]" into a retry timestamp
-- minutes away, and once the row dead-letters the pending filter drops
-- it entirely -- leaving a screen that reads as though the Stripe half
-- had finished. It has not; it failed.
--
-- Stripe's Redaction Jobs API is in public preview and not enabled on
-- this account (ADR-0027), so that failure path is not hypothetical: it
-- is what happens on the first attempt today.
--
-- redactable_after holds the fact instead: when Stripe will first allow
-- this Customer's transactions to be redacted, written once at enqueue
-- time and never rewritten by a retry. Nullable, because only
-- 'stripe_redaction_job' rows have one -- a Customer delete and an
-- Identity Platform delete wait for nothing.
ALTER TABLE client_erasure_outbox ADD COLUMN redactable_after timestamptz;

-- The read the Client detail screen makes: this Client's redaction rows
-- that have not yet succeeded. A dead-lettered row is deliberately
-- inside the index, not filtered out of it -- a redaction that failed is
-- exactly what a Practice must still be able to see.
CREATE INDEX client_erasure_outbox_redactable
    ON client_erasure_outbox (client_id, redactable_after)
    WHERE redactable_after IS NOT NULL AND status <> 'sent';

-- +goose Down
DROP INDEX client_erasure_outbox_redactable;
ALTER TABLE client_erasure_outbox DROP COLUMN redactable_after;
