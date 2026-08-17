-- +goose Up
-- Idempotency guard for the credit-purchase webhook (#77): Stripe retries
-- webhook deliveries until it gets a 2xx, and the same Checkout Session
-- completion can be delivered more than once. Recording each processed
-- Stripe event id here -- and relying on the PRIMARY KEY via
-- INSERT ... ON CONFLICT DO NOTHING -- lets the webhook handler tell
-- "already credited" apart from "credit it now" even under concurrent
-- duplicate deliveries, without a SELECT-then-INSERT race.
-- Platform-level, not Practice-scoped: the webhook has no authenticated
-- Practice session to scope RLS against, so this table carries none.
CREATE TABLE stripe_webhook_events (
    event_id text PRIMARY KEY,
    processed_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT ON stripe_webhook_events TO app_runtime;

-- +goose Down
DROP TABLE stripe_webhook_events;
