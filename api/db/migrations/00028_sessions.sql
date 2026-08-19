-- +goose Up
-- Browser sessions move from Identity Platform to Postgres (#163, ADR-0004).
-- A session is an opaque random token held in the __session cookie; this
-- table is the only record of it, so verification is a local read inside
-- the transaction authn.Begin already opens, and renewal is an UPDATE that
-- needs no Bearer ID token.
--
-- Only the SHA-256 of the token is stored, never the token itself: the
-- token carries enough entropy that a plain digest is the right primitive
-- (no salt, no password hash), and it means a leaked read of this table
-- hands nobody a usable session.
--
-- identity_uid, not a staff_id: this table serves both populations, so it
-- matches staff.identity_uid and client_portal_users.identity_uid rather
-- than referencing either table. Its index is what keeps a future "end
-- every session for this person" (#154) a single cheap DELETE.
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    identity_uid text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_identity_uid_idx ON sessions (identity_uid);

-- The expiry sweep runs on every mint; without this index it is a full
-- scan of every live session each time somebody signs in.
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON sessions TO app_runtime;

-- No row-level security, deliberately -- the same call this table's own
-- lookup makes is what *establishes* the caller's identity, so it runs
-- before app.current_identity_uid (or any other session variable) has a
-- value. Any policy keyed on those variables would match zero rows and
-- 401 every request. RLS in this schema enforces tenant isolation over
-- tenant data; this is BFF infrastructure with no tenant, the same
-- reading as stripe_webhook_events in 00022. Storing only the digest is
-- what keeps that safe.

-- +goose Down
DROP TABLE sessions;
