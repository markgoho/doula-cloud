-- +goose Up
-- 00015_credit_ledger.sql left credit_ledger_origin with only the two
-- grant origins it needed at the time (signup_bonus, purchase) --
-- consumed_engagement_id/consumed_at were added as columns for "future
-- consumption rows (a later ticket)" but no origin value existed yet for
-- them. This is that later ticket (#76): a consumption row's origin is
-- 'consumption', quantity -1, consumed_engagement_id/consumed_at set.
ALTER TYPE credit_ledger_origin ADD VALUE 'consumption';

-- +goose Down
-- Postgres has no ALTER TYPE ... DROP VALUE, so reversing this requires
-- rebuilding the enum. Safe only because down-migrations run against a
-- disposable test/dev database, never a database with real consumption
-- rows -- if one existed, the USING cast below would fail on it.
ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'purchase');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;
