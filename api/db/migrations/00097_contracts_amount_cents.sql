-- +goose Up
-- #967: a Contract carries a real amount in cents, snapshotted from the
-- Practice's rate card (practice_rates, 00096_practice_rates.sql) for the
-- Engagement's kind at the moment the Contract is created. Not nullable:
-- PostContractHandler refuses to create a Contract at all when the
-- Practice has no rate set for that kind (#967's AC), so every row this
-- column is ever written for already has an amount to carry. Pre-launch,
-- no production data exists to backfill, so NOT NULL needs no default.
--
-- The price is no longer a merge field a Practice's prose can carry a
-- value for in merge_field_values -- ADR-0008's amendment ("Money stops
-- being a merge field"). It lives only here, and the "price" merge-field
-- key is resolved from this column at read/render time, never stored,
-- so there is no copy for a PUT to overwrite.
ALTER TABLE contracts ADD COLUMN amount_cents bigint NOT NULL;

-- +goose Down
ALTER TABLE contracts DROP COLUMN amount_cents;
