-- +goose Up
-- #613 (map #164, #169's decision): Staff email verification, password
-- reset, and email-change notice all mint their own credential through
-- Doula Cloud's outbox rather than Identity Platform's own mailer.
-- Identity Platform remains the credential store -- it still owns
-- emailVerified and the password hash -- it just stops being the post
-- office.
--
-- auth_tokens is the one token table #613's ticket requires: a purpose
-- column and a per-purpose expiry, not one table per purpose. Today it
-- serves two purposes; #166's Client magic link and #605's MFA-recovery
-- decision add theirs later by widening auth_token_purpose, not by
-- creating a table of their own. Shaped after sessions (00028): only the
-- digest is stored, identity_uid rather than a Staff/Client foreign key
-- so the same table serves both populations, and no RLS -- like sessions,
-- this table is itself part of establishing who the caller is, so a
-- policy keyed on a session variable would run before that variable has
-- a value.
CREATE TYPE auth_token_purpose AS ENUM ('staff_email_verification', 'staff_password_reset');

CREATE TABLE auth_tokens (
    token_hash text PRIMARY KEY,
    purpose auth_token_purpose NOT NULL,
    identity_uid text NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Minting a fresh token for (identity_uid, purpose) deletes whatever
-- unspent row already exists there first (api/internal/authtoken.Mint),
-- so a re-request kills the previous link outright rather than leaving
-- two live at once -- this index is what makes that delete cheap.
CREATE INDEX auth_tokens_identity_purpose_idx
    ON auth_tokens (identity_uid, purpose)
    WHERE used_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON auth_tokens TO app_runtime;

-- staff_token_mail_outbox is ADR-0010's outbox for the two mail kinds
-- that mint an auth_tokens row: verification and reset. One table, not
-- two, because both share the same recipient resolution (the worker
-- reads the account's *current* address from Identity Platform via the
-- Admin SDK at send time -- staff.email can drift from it, #614 -- so
-- there is nothing here to join against `staff` for, and this table
-- needs no notification_worker_trusted policy on it the way
-- portal_invite_outbox needed one on client_portal_users/clients).
--
-- token is the plaintext value auth_tokens only ever stores a digest of
-- (mirroring staff_invite_outbox's invite_token, 00038): nulled out the
-- moment a row leaves 'pending', so its exposure window is "queued but
-- not yet sent".
CREATE TYPE staff_token_mail_kind AS ENUM ('email_verification', 'password_reset');
CREATE TYPE staff_token_mail_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE staff_token_mail_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL,
    kind staff_token_mail_kind NOT NULL,
    token text,
    status staff_token_mail_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per (identity_uid, kind): a re-request (the
-- signed-in "send me a fresh verification link" AC, or a repeated
-- password-reset request) resets this row via ON CONFLICT rather than
-- inserting a second one, mirroring portal_invite_outbox_one_pending
-- (00032) and staff_invite_outbox_one_pending (00038).
CREATE UNIQUE INDEX staff_token_mail_outbox_one_pending
    ON staff_token_mail_outbox (identity_uid, kind)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON staff_token_mail_outbox TO app_runtime;

-- staff_email_change_outbox notifies the *old* address after a Staff
-- email change (UpdateUser(Email(...))) -- a separate table because its
-- recipient is not the account's current address by the time the worker
-- runs; it is the one the change moved away from, so it has to be
-- captured at request time rather than resolved live like the table
-- above.
CREATE TYPE staff_email_change_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE staff_email_change_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL,
    old_email text NOT NULL,
    status staff_email_change_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

GRANT SELECT, INSERT, UPDATE ON staff_email_change_outbox TO app_runtime;

-- No RLS on either outbox table -- platform-level, like every other
-- outbox table (portal_invite_outbox, staff_invite_outbox, ...): the
-- worker runs with no Practice or Client session context, and carries
-- no tenant of its own to scope a policy against.

-- +goose Down
DROP TABLE staff_email_change_outbox;
DROP TYPE staff_email_change_outbox_status;
DROP TABLE staff_token_mail_outbox;
DROP TYPE staff_token_mail_outbox_status;
DROP TYPE staff_token_mail_kind;
DROP TABLE auth_tokens;
DROP TYPE auth_token_purpose;
