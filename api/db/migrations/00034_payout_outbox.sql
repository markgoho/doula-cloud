-- +goose Up
-- Outbox for the "your Practice's payout account needs more information"
-- Platform Notification (#343, ADR-0010, map #213). Mirrors
-- low_credit_outbox (00033) in shape: keyed to practice_id, recipient is
-- every Owner of the Practice, resolved at send time via
-- staff/practice_memberships, never stored on the row.
--
-- Trigger and cadence, decided here: PostAccountWebhookHandler's
-- handleCapabilityStatusUpdated queues one row per "requirements due"
-- episode -- the transition from an empty stripe_connect_requirements_due
-- to a non-empty one, not every webhook delivery while it stays
-- non-empty. Keyed to requirements_due rather than capability status
-- because that is the Owner-actionable condition: connect.go's own
-- ConnectStatus doc already treats StatusPending ("Stripe reviewing what
-- was supplied") as nothing left for the Owner to do, while outstanding
-- requirements are exactly "the Owner has something to give Stripe". One
-- email per episode, no re-reminding while requirements stay
-- outstanding; re-armed only if requirements clear and then reappear.
--
-- next_attempt_at defaults 48 hours out, not immediately due like
-- low_credit_outbox's: this fires the moment an Owner has anything
-- outstanding at all, which includes the instant they start onboarding a
-- brand-new account, and mailing them mid-form would be the noisiest
-- possible reading of "notify". The grace window gives a normal Owner
-- time to finish what is usually a short Stripe form before being
-- nagged. billing.Worker's counterpart recheck at send time (against the
-- live stripe_connect_requirements_due column, not a snapshot) skips the
-- mail entirely if the Owner already finished within the window.
CREATE TYPE payout_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE payout_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    status payout_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now() + interval '48 hours',
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per Practice, guarding both the race between
-- two concurrent webhook deliveries and a second episode queued while an
-- earlier one is still waiting out its grace window.
CREATE UNIQUE INDEX payout_outbox_one_pending
    ON payout_outbox (practice_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON payout_outbox TO app_runtime;

-- No new RLS: this table is platform-level like low_credit_outbox and
-- portal_invite_outbox, and the worker reuses 00033's
-- app.notification_worker_trusted policies on staff/practice_memberships
-- (table-generic, not tied to a specific outbox table) to resolve
-- Owners. practices itself carries no RLS, so the send-time requirements
-- recheck needs no policy either.

-- +goose Down
DROP TABLE payout_outbox;
DROP TYPE payout_outbox_status;
