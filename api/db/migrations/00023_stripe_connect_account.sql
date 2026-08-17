-- +goose Up
-- A Practice's Stripe Connect (Standard tier) linkage, for the payments
-- feature (#78/#79): the Connect account id a Practice's Owner links via
-- hosted Account Link onboarding, plus the three capability booleans
-- mirrored from Stripe's Account object. Same "column(s) on practices"
-- pattern as billing's stripe_customer_id (00015_credit_ledger.sql) --
-- no new table for the linkage itself.
--
-- The three booleans are written only by the Connect webhook handler (a
-- later ticket, #81) in production -- this ticket's on-demand status read
-- calls Stripe's Account retrieve directly and does not write them, to
-- avoid two write paths. They default to false so a freshly-linked account
-- (before its first webhook delivery) reads as not-yet-active rather than
-- NULL.
ALTER TABLE practices ADD COLUMN stripe_connect_account_id text;
ALTER TABLE practices ADD COLUMN stripe_connect_charges_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE practices ADD COLUMN stripe_connect_payouts_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE practices ADD COLUMN stripe_connect_details_submitted boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE practices DROP COLUMN stripe_connect_details_submitted;
ALTER TABLE practices DROP COLUMN stripe_connect_payouts_enabled;
ALTER TABLE practices DROP COLUMN stripe_connect_charges_enabled;
ALTER TABLE practices DROP COLUMN stripe_connect_account_id;
