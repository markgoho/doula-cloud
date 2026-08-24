-- +goose Up
-- Mailgun bounce/complaint webhook (#340, ADR-0010, map #213 -- #337
-- settled the two states). Record-only: no automatic remedy, no retry
-- state touched, a Staff member re-invites by hand same as today.
--
-- 'bounced' -- Mailgun accepted the send, then the address rejected it.
-- Actionable: a Staff member should re-invite.
-- 'complained' -- the recipient marked the mail as spam. The mail
-- arrived; kept distinct from 'bounced' so Staff isn't told to re-send
-- to someone who doesn't want it.
ALTER TYPE portal_invite_outbox_status ADD VALUE 'bounced';
ALTER TYPE portal_invite_outbox_status ADD VALUE 'complained';

-- Idempotency guard against a replayed Mailgun delivery, mirroring
-- stripe_webhook_events (00022): platform-level, no RLS -- the webhook
-- has no authenticated Practice session to scope RLS against.
CREATE TABLE mailgun_webhook_events (
    event_id text PRIMARY KEY,
    processed_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT ON mailgun_webhook_events TO app_runtime;

-- +goose Down
DROP TABLE mailgun_webhook_events;

-- Postgres has no ALTER TYPE ... DROP VALUE, so reversing the enum
-- addition requires rebuilding it, per 00021's precedent. Safe only
-- against a disposable test/dev database -- the USING cast fails if any
-- row already carries 'bounced' or 'complained'.
ALTER TYPE portal_invite_outbox_status RENAME TO portal_invite_outbox_status_old;
CREATE TYPE portal_invite_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');
ALTER TABLE portal_invite_outbox
    ALTER COLUMN status TYPE portal_invite_outbox_status
    USING status::text::portal_invite_outbox_status,
    ALTER COLUMN status SET DEFAULT 'pending';
DROP TYPE portal_invite_outbox_status_old;
