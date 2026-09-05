-- +goose Up
-- #617 (ADR-0026's "Clients get a magic link"): a Client signs in with a
-- BFF-minted, single-use link token rather than a password. It rides
-- #613's shared auth_tokens table (00061) as a fourth purpose rather than
-- a table of its own -- one expiry policy, one single-use rule, one place
-- to look when a link does not arrive.
--
-- Added here, used nowhere else in this same migration: Postgres forbids
-- an ALTER TYPE ... ADD VALUE from being read within the transaction that
-- added it (00071/00072's own split for exactly this reason). Nothing
-- below references this value; api/internal/authtoken.Mint/Spend read it
-- at request time, well after this migration has committed.
ALTER TYPE auth_token_purpose ADD VALUE 'client_magic_link';

-- portal_magic_link_outbox is ADR-0010's outbox for the one mail kind a
-- magic-link request queues. A table of its own rather than folding into
-- staff_token_mail_outbox (00061): that table's claim query resolves its
-- recipient live via the Identity Platform Admin SDK, which has nothing
-- to say about a Portal Account. This table's identity_uid carries a real
-- FK to portal_accounts instead -- it names nothing but a Portal Account,
-- ever, unlike sessions.identity_uid or staff_token_mail_outbox.identity_uid,
-- which share a bare text column across both populations by convention.
--
-- token is the plaintext value auth_tokens only ever stores a digest of,
-- nulled out the moment a row leaves 'pending' -- the same shape
-- staff_token_mail_outbox.token and staff_invite_outbox.invite_token use.
CREATE TYPE portal_magic_link_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE portal_magic_link_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid    text NOT NULL REFERENCES portal_accounts (identifier) ON DELETE CASCADE,
    token           text,
    status          portal_magic_link_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

-- At most one pending row per identity_uid: a re-request (she asks for a
-- second link before reading the first mail) resets this row via
-- ON CONFLICT rather than inserting a second one, mirroring
-- staff_token_mail_outbox_one_pending (00061).
CREATE UNIQUE INDEX portal_magic_link_outbox_one_pending
    ON portal_magic_link_outbox (identity_uid)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON portal_magic_link_outbox TO app_runtime;

-- No RLS: platform-level, like every other outbox table -- the worker
-- runs with no Client session context and carries no Practice to scope a
-- policy against (00061's own reasoning for staff_token_mail_outbox).

-- A magic-link request resolves an email address to its Portal Account
-- identifier before app.current_identity_uid has a value -- establishing
-- that value *is* what the request is doing -- so no session-scoped
-- policy on portal_accounts could ever admit this read, the same
-- reasoning sessions (00028) and auth_tokens (00061) are exempted from
-- RLS entirely for. portal_accounts keeps its RLS on for the erasure
-- policies 00073 already carries; this is one more permissive SELECT
-- policy alongside them (Postgres ORs multiple permissive policies for
-- the same command together). It exposes only what a caller's own
-- request already implies she knows -- the address she just typed -- and
-- the request endpoint's response is identical whether or not this
-- lookup finds a row (#617's account-enumeration AC).
CREATE POLICY portal_accounts_signin_lookup ON portal_accounts
    FOR SELECT
    USING (true);

-- +goose Down
DROP POLICY portal_accounts_signin_lookup ON portal_accounts;
DROP TABLE portal_magic_link_outbox;
DROP TYPE portal_magic_link_outbox_status;

-- auth_token_purpose's 'client_magic_link' value is left in place --
-- Postgres has no DROP VALUE, and 00071/00072 leave their own added
-- values the same way.
