-- +goose Up
-- Outbox for the Staff invitation email (RA-G1, #339, ADR-0010, map
-- #213). practice_invitations (00030) already exists as schema-only --
-- no handler writes it yet (#316 builds InviteHandler/accept). Once it
-- does, InviteHandler queues a pending row here in the same transaction
-- as the practice_invitations insert/rotate, mirroring
-- portalinvite.queueOutboxSend's shape.
--
-- Unlike portal_invite_outbox, this table DOES carry the token: 00030
-- deliberately stores only token_digest on practice_invitations (a
-- leaked read of that table hands nobody a usable credential), so
-- there is nowhere else for the worker to read a live plaintext token
-- from at send time. invite_token here is that token, nulled out the
-- moment a row leaves 'pending' -- sent or dead-lettered alike -- keeping
-- its exposure window to "queued but not yet resolved" -- staffinvite.
-- Queue overwrites it (via ON CONFLICT) on every re-invite/rotation, so
-- the worker never mails a stale one even though it isn't reading
-- practice_invitations for it.
CREATE TYPE staff_invite_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE staff_invite_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invitation_id uuid NOT NULL REFERENCES practice_invitations (id),
    invite_token uuid,
    status staff_invite_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per Invitation: a re-invite refreshes this row
-- (invite_token, attempt_count, next_attempt_at) via ON CONFLICT rather
-- than inserting a second one, mirroring portal_invite_outbox_one_pending
-- (00032).
CREATE UNIQUE INDEX staff_invite_outbox_one_pending
    ON staff_invite_outbox (invitation_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON staff_invite_outbox TO app_runtime;

-- No RLS -- platform-level like portal_invite_outbox/low_credit_outbox/
-- payout_outbox/payment_received_outbox/session_notice_outbox: the
-- worker runs with no Practice or Client session context.

-- The worker's own trusted-session context has no visibility into
-- practice_invitations (00030's practice_invitations_practice_visibility
-- is practice-tier only), so it needs the same
-- app.notification_worker_trusted door 00032/00033 already opened on
-- other tables, applied here to read the recipient's address and current
-- status (to skip mailing an already-accepted/revoked/expired
-- Invitation) at send time.
-- +goose StatementBegin
CREATE POLICY practice_invitations_notification_worker ON practice_invitations
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY practice_invitations_notification_worker ON practice_invitations;
DROP TABLE staff_invite_outbox;
DROP TYPE staff_invite_outbox_status;
