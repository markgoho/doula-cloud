-- +goose Up
-- #967: a Contract carries a real amount in cents, snapshotted from the
-- Practice's rate card (practice_rates, 00096_practice_rates.sql) for the
-- Engagement's kind at the moment the Contract is created. Not nullable:
-- PostContractHandler refuses to create a Contract at all when the
-- Practice has no rate set for that kind (#967's AC), so every row this
-- column is ever written for already has an amount to carry.
--
-- The two-statement form is deliberate and must stay (#1021). A bare
-- `ADD COLUMN ... NOT NULL` passes every PR -- testdb builds an empty
-- database per run, so there are no rows to violate the constraint --
-- and then fails the first time trunk's migrate job applies it to
-- doula-cloud-pg, which carries rows: `column "amount_cents" of
-- relation "contracts" contains null values (SQLSTATE 23502)`. That is
-- what happened here, and it held trunk red across seven merges.
-- "Pre-launch, no production data" is true of *production* and false of
-- the shared instance every deploy migrates.
--
-- DEFAULT 0 backfills the pre-#967 rows; DROP DEFAULT then removes it so
-- no future INSERT can quietly get an amount it never asked for. Zero is
-- a marker on rows that predate the column, not a price -- #966's "no
-- rate set is not zero" invariant is what the dropped default protects
-- for every row created from here on. Those pre-existing Contracts read
-- $0.00 until re-created or overridden through PutContractAmountHandler.
--
-- The price is no longer a merge field a Practice's prose can carry a
-- value for in merge_field_values -- ADR-0008's amendment ("Money stops
-- being a merge field"). It lives only here, and the "price" merge-field
-- key is resolved from this column at read/render time, never stored,
-- so there is no copy for a PUT to overwrite.
ALTER TABLE contracts ADD COLUMN amount_cents bigint NOT NULL DEFAULT 0;
ALTER TABLE contracts ALTER COLUMN amount_cents DROP DEFAULT;

-- +goose Down
ALTER TABLE contracts DROP COLUMN amount_cents;
