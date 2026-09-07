-- #871: a Practice's day-30 deletion finalization forfeits any unspent
-- Credit balance rather than refunding it (decision 4, settled in the
-- ticket's comment thread). Forfeiture is written as one credit_ledger
-- row -- the same append-only, sum-to-balance shape every other origin
-- already uses -- so the ledger still sums to zero for a fully forfeited
-- Practice and the act carries its own who/when the way every other
-- origin does.
--
-- No NO TRANSACTION directive, matching 00021's own bare ADD VALUE: the
-- restriction is on *using* a value added this transaction, not on
-- adding it, and this file does neither in its Up section -- the value
-- is spent later, by application code in its own separate transaction.

-- +goose Up
ALTER TYPE credit_ledger_origin ADD VALUE 'forfeit';

-- +goose Down
-- Postgres has no ALTER TYPE ... DROP VALUE, so reversing this requires
-- rebuilding the enum, the same shape 00021/00052/00055 use for their
-- own additions. Safe only because down-migrations run against a
-- disposable test/dev database, never one with real forfeit rows -- if
-- one existed, the USING cast below would fail on it.
ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'founding_grant', 'purchase', 'consumption', 'refund');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;
