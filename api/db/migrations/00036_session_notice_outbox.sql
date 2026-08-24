-- +goose Up
-- Outbox for the two security-notice Platform Notifications ADR-0004
-- orphaned when session ownership moved off Identity Platform: "new
-- sign-in" and "session revoked" (#345, ADR-0010, map #213). One table
-- for both, not one per kind like low_credit_outbox/payout_outbox/
-- payment_received_outbox -- #345 bundled them into a single ticket
-- precisely because they share the same trigger surface (the `sessions`
-- table) and the same recipient shape: a single named Staff member,
-- resolved from identity_uid at send time via `staff`, never a Practice
-- or an Owner set. Kept distinct from `sessions` itself, whose rows are
-- swept or deleted the moment they stop mattering to auth -- an outbox
-- row has to outlive that.
CREATE TYPE session_notice_kind AS ENUM ('new_signin', 'session_revoked');
CREATE TYPE session_notice_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE session_notice_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL,
    kind session_notice_kind NOT NULL,
    status session_notice_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- sessionnotice.QueueNewSignInIfDue's idle-window dedupe (skip a new row
-- if one was already queued for this identity+kind within the window)
-- scans by identity_uid and kind, most-recent first.
CREATE INDEX session_notice_outbox_identity_kind_idx
    ON session_notice_outbox (identity_uid, kind, created_at DESC);

-- At most one pending session_revoked row per identity, guarding the
-- same concurrent-double-insert race low_credit_outbox_one_pending
-- guards for Credits: two Owner clicks of "end sessions" landing before
-- either's queue insert commits must not queue two identical notices.
-- new_signin carries no equivalent constraint -- sessionnotice.
-- QueueNewSignInIfDue's own idle-window dedupe already governs when a
-- row is due, and that window is time-based, so a pending row can
-- legitimately still exist (worker outage, mid-backoff) well after a
-- fresh notice becomes due again; a matching unique index would wrongly
-- block that fresh row rather than the narrow race it exists to guard.
CREATE UNIQUE INDEX session_notice_outbox_revoked_one_pending
    ON session_notice_outbox (identity_uid)
    WHERE status = 'pending' AND kind = 'session_revoked';

GRANT SELECT, INSERT, UPDATE ON session_notice_outbox TO app_runtime;

-- No RLS -- platform-level like sessions/low_credit_outbox/
-- payout_outbox/payment_received_outbox: the worker runs with no
-- Practice or Client session, and the queuing call from
-- session.CreateHandler runs before any session exists at all. Reuses
-- 00033's `staff_notification_worker` policy (already a blanket SELECT
-- permit under app.notification_worker_trusted) to resolve the target
-- Staff member's email at send time and, from CreateHandler, to check
-- whether a signing-in identity_uid is Staff at all -- no new policy
-- needed on `staff` for either read.

-- +goose Down
DROP TABLE session_notice_outbox;
DROP TYPE session_notice_outbox_status;
DROP TYPE session_notice_kind;
