-- +goose Up
-- Email suppression (#733, map #347, ADR-0029). Keyed on the address,
-- not on identity_uid + engagement_id like notification_preferences
-- (#303): the portal invite and the Staff invitation both send before
-- any account exists, and ADR-0011 puts all eleven mail kinds on one
-- Mailgun domain and one reputation, so a suppression has to reach every
-- outbox rather than one Engagement's preferences.
--
-- Platform-level, no RLS -- mirroring mailgun_webhook_events (00037) and
-- stripe_webhook_events (00022): the Mailgun webhook that writes this
-- table has no authenticated Practice session to scope a policy against,
-- and the outbox workers that read it send on behalf of every Practice
-- at once.
CREATE TABLE email_suppressions (
    -- Stored already lower-cased, and compared lower-cased at send:
    -- Mailgun reports the address as the sender wrote it, and the local
    -- part's case must not decide whether a suppression is found.
    address text PRIMARY KEY CHECK (address = lower(address)),
    -- 'complaint' -- the recipient marked Doula Cloud's mail as spam.
    -- 'bounce'    -- a permanent SMTP-level rejection, first time seen.
    -- ADR-0029: only 'bounce' is ever clearable.
    cause text NOT NULL CHECK (cause IN ('bounce', 'complaint')),
    -- The Mailgun event that caused it, so "how did this come to be?"
    -- has an answer that reaches back into Mailgun's own logs.
    mailgun_event_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    -- Clearing is Staff-only and bounce-only (ADR-0029). Nulling the
    -- row is not enough on its own: Mailgun keeps its own bounce list
    -- and refuses the send server-side until DELETE /v3/{domain}/bounces
    -- runs too.
    cleared_at timestamptz,
    cleared_by uuid REFERENCES staff (id)
);

-- The send-time question is "is this address suppressed right now?", so
-- the partial index carries only the rows that can answer yes.
CREATE INDEX email_suppressions_active_idx ON email_suppressions (address) WHERE cleared_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON email_suppressions TO app_runtime;

-- +goose Down
DROP TABLE email_suppressions;
