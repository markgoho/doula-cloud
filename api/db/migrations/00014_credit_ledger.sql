-- +goose Up
-- A Practice's billing credit balance is never stored as a mutable
-- counter -- it is always the sum of its credit_ledger rows. Each row
-- records where credits came from (origin) and a signed quantity: +N for
-- a grant (signup_bonus, purchase, with room for future origins such as
-- referral_bonus), or -1 for a consumption. consumed_engagement_id and
-- consumed_at are for future consumption rows (a later ticket) -- this
-- migration only establishes the table and the signup_bonus grant.
-- credit_ledger is append-only: app_runtime is granted SELECT and INSERT
-- only, no UPDATE or DELETE, so the database itself enforces that history
-- is never rewritten.

CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'purchase');

CREATE TABLE credit_ledger (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    origin credit_ledger_origin NOT NULL,
    quantity integer NOT NULL,
    consumed_engagement_id uuid REFERENCES engagements (id),
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Populated lazily on a Practice's first credit purchase (a later ticket),
-- which also owns any uniqueness constraint that purchase flow needs.
ALTER TABLE practices ADD COLUMN stripe_customer_id text;

GRANT SELECT, INSERT ON credit_ledger TO app_runtime;

ALTER TABLE credit_ledger ENABLE ROW LEVEL SECURITY;

-- Same shape as practice_memberships_practice_visibility in
-- 00002_practice_staff_tenancy.sql: a direct practice_id column matched
-- against the app.current_practice_id session variable, fail-closed when
-- unset.
CREATE POLICY credit_ledger_practice_visibility ON credit_ledger
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
ALTER TABLE practices DROP COLUMN stripe_customer_id;
DROP TABLE credit_ledger;
DROP TYPE credit_ledger_origin;
