-- +goose Up
-- Outbox for the "a Payment arrived" Platform Notification (#344,
-- ADR-0010, map #213). Filed by #334's ship/file grill as the
-- lowest-justified of four candidates -- pure convenience, since Staff
-- can already see a Payment in the app -- so unlike low_credit_outbox
-- (00033) and payout_outbox (00034), the recipient here is not "whoever
-- can act on this" (there is no responsive action) but "whoever can
-- already read this money": ADR-0006/ADR-0008's read table gives
-- Contract money and Invoice history to Owner and Admin alike, and
-- GetInvoicesHandler is gated ownerAndAdmin in main.go, so this
-- notification goes to both roles, not Owner-only like its two
-- siblings.
--
-- One row per Payment, not per episode: practice_id is copied onto the
-- row at queue time, inside handleInvoicePaid's own tx (the only place
-- app.current_practice_id is set for a webhook-driven write), because
-- the worker itself runs with no Practice session and cannot re-read the
-- payments/invoices tables to resolve it later -- mirroring
-- payout_outbox's own practice_id column for the same reason. The email
-- body carries no payment detail (ADR-0009: link-only, matching both
-- shipped sibling notifications), so amount_cents/paid_at are not stored
-- here at all -- payment_id's FK already gives full traceability back to
-- the Payment for anyone who needs it. A payment that already arrived
-- cannot un-arrive, so -- unlike payout_outbox's live requirements
-- recheck -- there is nothing to verify again at send time. payment_id
-- is UNIQUE, not a partial index keyed to "one pending per Practice":
-- two payments landing before a Scheduler tick must both be mailed, and
-- claimEvent's stripe_webhook_events claim already makes a replayed
-- invoice.paid event a no-op before the INSERT this table's row rides on
-- is ever reached, so no further dedup is needed here.
--
-- next_attempt_at defaults to now(), like low_credit_outbox and unlike
-- payout_outbox's 48-hour grace window: that window exists for a
-- mid-onboarding nag problem this notification has no analogue of.
CREATE TYPE payment_received_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE payment_received_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id uuid NOT NULL REFERENCES payments (id),
    practice_id uuid NOT NULL REFERENCES practices (id),
    status payment_received_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text,
    UNIQUE (payment_id)
);

GRANT SELECT, INSERT, UPDATE ON payment_received_outbox TO app_runtime;

-- No new RLS: platform-level like low_credit_outbox and payout_outbox,
-- and the worker reuses 00033's app.notification_worker_trusted
-- policies on staff/practice_memberships (table-generic) to resolve
-- Owners and Admins. This table has no policy of its own for the same
-- reason those two don't.

-- +goose Down
DROP TABLE payment_received_outbox;
DROP TYPE payment_received_outbox_status;
