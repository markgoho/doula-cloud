-- +goose Up
-- #619 (ADR-0026): a Client changes her own sign-in address from inside
-- the portal, proved by a link sent to the *new* address, with the old
-- one still signing her in until the new one is proved.
--
-- 00073 left the door for this open in as many words -- "No UPDATE
-- grant: changing the sign-in address is #619's door" -- so this
-- migration is that grant, that policy, and the two rows the flow needs
-- between request and proof.
--
-- Added here, read nowhere else in this migration: Postgres forbids an
-- ALTER TYPE ... ADD VALUE from being read within the transaction that
-- added it (00071/00072/00074's own split for the same reason).
-- api/internal/authtoken.Mint/Spend read it at request time, well after
-- this has committed.
ALTER TYPE auth_token_purpose ADD VALUE 'client_sign_in_address_change';

-- portal_sign_in_address_changes holds the one fact auth_tokens has no
-- room for: which address this token proves. Keyed 1:1 on the
-- auth_tokens row it belongs to, exactly like
-- staff_mfa_recovery_vouches (00062) -- the pattern
-- api/internal/authtoken.Digest was exported for.
--
-- ON DELETE CASCADE off token_hash is what makes a re-request correct
-- for free: authtoken.Mint deletes the identity's prior unspent token
-- for this purpose first, so the address that token named goes with it
-- and can never be spent by a link delivered on a late outbox retry.
--
-- new_address is stored normalized (lowercased and trimmed, by
-- staffauth.NormalizeAddress at the handler), because it is what
-- portal_accounts.sign_in_address is set to when the token is spent and
-- portal_accounts_sign_in_address (00073) compares case-insensitively
-- anyway -- storing two spellings of one address would only invite them
-- to disagree.
--
-- No RLS, like auth_tokens (00061) itself: this row is read at spend
-- time, which is unauthenticated -- establishing app.current_identity_uid
-- is what spending the token *does*, so no session-scoped policy could
-- ever admit the read.
CREATE TABLE portal_sign_in_address_changes (
    token_hash  text PRIMARY KEY REFERENCES auth_tokens (token_hash) ON DELETE CASCADE,
    identifier  text NOT NULL REFERENCES portal_accounts (identifier) ON DELETE CASCADE,
    new_address text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT ON portal_sign_in_address_changes TO app_runtime;

-- The UPDATE grant 00073 withheld, plus the policy that narrows it to
-- the one row the caller has proved she holds. Mirrors
-- portal_accounts_self_insert's shape: at spend time,
-- clientauth.SpendAddressChangeHandler sets app.current_identity_uid to
-- the identifier the spent token named, immediately before this UPDATE.
--
-- USING and WITH CHECK carry the identical predicate: identifier is
-- never what this UPDATE writes, so a row cannot be moved out from under
-- the policy that admitted it.
GRANT UPDATE (sign_in_address) ON portal_accounts TO app_runtime;

CREATE POLICY portal_accounts_self_update ON portal_accounts
    FOR UPDATE
    USING (identifier = NULLIF(current_setting('app.current_identity_uid', true), ''))
    WITH CHECK (identifier = NULLIF(current_setting('app.current_identity_uid', true), ''));

-- portal_address_change_outbox is ADR-0010's outbox for the confirmation
-- mail. A table of its own rather than reusing portal_magic_link_outbox
-- (00074): that table resolves its recipient by joining
-- portal_accounts, which holds the *old* address -- the one address this
-- mail must never go to. The address it is going to lives nowhere else
-- yet, so the outbox row carries it.
CREATE TYPE portal_address_change_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE portal_address_change_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid    text NOT NULL REFERENCES portal_accounts (identifier) ON DELETE CASCADE,
    to_address      text NOT NULL,
    token           text,
    status          portal_address_change_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

-- At most one pending row per identity_uid, the same re-request rule
-- portal_magic_link_outbox_one_pending (00074) keeps: she asks twice,
-- naming a different address the second time, and the pending row is
-- reset to the fresher address and token rather than both being mailed.
CREATE UNIQUE INDEX portal_address_change_outbox_one_pending
    ON portal_address_change_outbox (identity_uid)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON portal_address_change_outbox TO app_runtime;

-- No RLS: platform-level, like every other outbox table -- the worker
-- runs with no session context at all.

-- +goose Down
DROP TABLE portal_address_change_outbox;
DROP TYPE portal_address_change_outbox_status;
DROP POLICY portal_accounts_self_update ON portal_accounts;
REVOKE UPDATE (sign_in_address) ON portal_accounts FROM app_runtime;
DROP TABLE portal_sign_in_address_changes;

-- auth_token_purpose's 'client_sign_in_address_change' value is left in
-- place -- Postgres has no DROP VALUE, and 00071/00072/00074 leave their
-- own added values the same way.
