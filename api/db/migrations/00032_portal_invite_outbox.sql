-- +goose Up
-- Outbox for the Client portal invite email (#219, ADR-0010, map #213).
-- InviteHandler writes a pending row in the same transaction as the
-- client_portal_users insert/rotate; a Cloud-Scheduler-triggered internal
-- endpoint (portalinvite.ProcessOutboxHandler) reads due rows afterward
-- and sends via Mailgun outside any user-facing request.
--
-- No token column here: the worker joins client_portal_users at send
-- time for the current invite_token, so a re-invite's rotation is always
-- read fresh -- there is nothing to go stale.
--
-- Platform-level, not Practice-scoped, mirroring stripe_webhook_events
-- (00022): the worker runs with no authenticated Practice session, so
-- there is no app.current_practice_id to scope RLS against. This table
-- carries none.
CREATE TYPE portal_invite_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE portal_invite_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_portal_user_id uuid NOT NULL REFERENCES client_portal_users (id),
    status portal_invite_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per portal user: a re-invite resets this row
-- (attempt_count, next_attempt_at) via ON CONFLICT rather than inserting
-- a second one, so retry state doesn't fork across two rows racing to
-- send the same invite.
CREATE UNIQUE INDEX portal_invite_outbox_one_pending
    ON portal_invite_outbox (client_portal_user_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON portal_invite_outbox TO app_runtime;

-- The worker runs with no Practice or Client session context at all --
-- unlike every other reader of client_portal_users and clients -- so the
-- existing RLS policies on those two tables (scoped to
-- app.current_practice_id, app.current_client_id, or
-- app.current_identity_uid) leave it zero visible rows. Grant it exactly
-- the columns it needs (invite_token, identity_uid, email) via a new
-- SELECT policy scoped to app.notification_worker_trusted, a session var
-- only ProcessOutboxHandler sets, and only after its own
-- X-Internal-Secret check passes -- the same "out-of-band verification
-- licenses setting the session var" shape billing's Stripe webhook uses
-- for app.current_practice_id (00022_stripe_purchase_webhook.sql /
-- PostPurchaseWebhookHandler). Postgres OR's multiple permissive SELECT
-- policies together, so this only adds visibility, never removes any.
-- +goose StatementBegin
CREATE POLICY client_portal_users_notification_worker ON client_portal_users
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');

CREATE POLICY clients_notification_worker ON clients
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY clients_notification_worker ON clients;
DROP POLICY client_portal_users_notification_worker ON client_portal_users;
DROP TABLE portal_invite_outbox;
DROP TYPE portal_invite_outbox_status;
