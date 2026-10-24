-- +goose Up
-- #1812: one append-only row for each capability_status_updated event the
-- Connect account webhook applies, so "how did this Practice come to be
-- payable?" has an answer past the last event (CLAUDE.md's audit-trail
-- expectation), and #1591 can measure how long Stripe's review takes.
--
-- practices carries only the latest state (00029). Each delivery
-- overwrites stripe_connect_status_updated_at, so the moment card_payments
-- became active is lost as soon as any later event arrives, and
-- stripe_webhook_events (00022) keeps no account, type or status to
-- rebuild it from. This table is that lost history. The webhook writes it
-- in the same transaction as the practices UPDATE, so the two cannot
-- disagree, and only for an applied event: a replay and an event for an
-- account no Practice claims write nothing here.
--
-- Two times, because they can disagree (ADR-0007). A thin event carries
-- no state, so the handler fetches the account and writes what Stripe
-- reports at the moment of processing:
--
--   stripe_event_created_at  Stripe's own `created` on the event: when
--                            Stripe noticed a change.
--   processed_at             when this row was written, from
--                            clock.Now(ctx) (#773). The statuses below
--                            are true at this instant.
--
-- A delivery retried or held back for an hour writes the status of the
-- moment it was processed beside the older created time. So
-- processed_at is the safe upper bound for "card_payments was active by
-- then", and stripe_event_created_at is the closer estimate when
-- deliveries are prompt.
--
-- The actor is Stripe in every row, so there is no actor column; the
-- event id names the delivery, as stripe_connect_status_event_id already
-- does on practices.
--
-- Platform-level and without RLS, for the reason 00022 and 00109 give:
-- the webhook runs with no Practice session, and practices itself carries
-- no RLS. GRANT SELECT, INSERT and nothing else: the history is never
-- edited. Finalizing a Practice's deletion sets practices.deleted_at
-- rather than deleting the row, so the foreign key needs no cascade.
CREATE TABLE stripe_connect_status_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    -- One applied event, one row. stripe_webhook_events already refuses a
    -- second claim of the same id; this says the same thing here.
    stripe_event_id text NOT NULL UNIQUE,
    card_payments_status text NOT NULL
        CHECK (card_payments_status IN ('active', 'pending', 'restricted', 'unsupported')),
    payouts_status text NOT NULL
        CHECK (payouts_status IN ('active', 'pending', 'restricted', 'unsupported')),
    -- The same Stripe field paths stripe_connect_requirements_due holds
    -- (00029): empty means nothing outstanding. Never Client data.
    requirements_due text[] NOT NULL DEFAULT '{}',
    stripe_event_created_at timestamptz NOT NULL,
    processed_at timestamptz NOT NULL
);

-- Serves #1591's read: one Practice's rows in the order they were applied.
CREATE INDEX stripe_connect_status_history_practice
    ON stripe_connect_status_history (practice_id, processed_at);

GRANT SELECT, INSERT ON stripe_connect_status_history TO app_runtime;

-- +goose Down
DROP TABLE stripe_connect_status_history;
