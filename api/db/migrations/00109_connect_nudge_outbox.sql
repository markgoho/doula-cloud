-- +goose Up
-- Outbox for the "your Practice still has to connect Stripe" Platform
-- Notification (#917, ADR-0010, ADR-0035). Shaped after payout_outbox
-- (00034): keyed to practice_id, recipient is every Owner of the
-- Practice, resolved at send time via staff/practice_memberships, never
-- stored on the row.
--
-- Two things differ from 00034, and both are what #917 is about.
--
-- The trigger is a person, not a webhook. 00034 fires on the
-- stripe_connect_requirements_due empty -> non-empty transition, which
-- can only happen once a Stripe account exists. A Practice that has
-- never started onboarding has no account, so no capability_status_
-- updated event is ever delivered for it and 00034 can never fire --
-- the exact state an Admin can see and nobody is told about. Nothing in
-- this repo fires on a clock (ADR-0033), so the act that queues a row
-- here is a Staff member pressing the control on the Payments settings
-- screen.
--
-- And the row records who pressed it. requested_by_staff_id is
-- CLAUDE.md's audit-trail expectation answered on the row itself,
-- alongside the activity entry the handler writes; no other Notification
-- outbox in this schema carries an actor, because no other one has a
-- person behind it.
--
-- next_attempt_at is immediately due, unlike 00034's 48-hour grace
-- window. That window exists because 00034 fires the instant an Owner
-- has anything outstanding, including mid-form; there is no such risk
-- here, because a colleague only presses this after deciding the wait
-- has gone on long enough. ADR-0013's nudge carries it from there.
CREATE TYPE connect_nudge_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE connect_nudge_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    requested_by_staff_id uuid NOT NULL REFERENCES staff (id),
    status connect_nudge_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text,
    -- Whom it went to, written by the worker at send time. #917's audit
    -- criterion is "who sent it, to whom, and when", and the first and
    -- last are answerable from the columns above; this is the second.
    --
    -- Recipients are still resolved at send time and never chosen at
    -- queue time -- that is #343's rule, reused unchanged -- so this is a
    -- record of what the resolution produced, not an instruction to the
    -- worker. It matters because the resolution is against a roster that
    -- moves: an Owner who leaves the Practice next month is nowhere in a
    -- re-run of that query, and "every Owner at the time" would then be
    -- unanswerable. Staff ids rather than addresses: the ledger already
    -- resolves a staff id to a name, and an address on an append-only row
    -- would outlive the person changing it.
    notified_owner_staff_ids uuid[]
);

-- At most one pending row per Practice, guarding the race between two
-- Admins pressing the control in the same second. This is the
-- concurrency half of the bound only; the durable half is the cooldown
-- the handler enforces against created_at (see the index below).
CREATE UNIQUE INDEX connect_nudge_outbox_one_pending
    ON connect_nudge_outbox (practice_id)
    WHERE status = 'pending';

-- Serves the cooldown check: the most recent row for a Practice,
-- whatever became of it. A dead-lettered or failed nudge still counts
-- against the cooldown -- the bound is on how often a Practice's Owners
-- can be asked, and a retry schedule that is still running is not a
-- reason to queue a second ask.
CREATE INDEX connect_nudge_outbox_practice_recent
    ON connect_nudge_outbox (practice_id, created_at DESC);

GRANT SELECT, INSERT, UPDATE ON connect_nudge_outbox TO app_runtime;

-- No new RLS, for the reason 00034 gives: this table is platform-level
-- like payout_outbox and low_credit_outbox, and the worker reuses
-- 00033's table-generic app.notification_worker_trusted policies on
-- staff/practice_memberships to resolve Owners. practices itself carries
-- no RLS, so the send-time recheck needs no policy either.

-- +goose Down
DROP TABLE connect_nudge_outbox;
DROP TYPE connect_nudge_outbox_status;
