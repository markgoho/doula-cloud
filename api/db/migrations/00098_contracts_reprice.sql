-- +goose Up
-- #968: a rate change re-prices every unsigned Contract for that kind,
-- and never a signed one. Two columns carry what that needs beyond the
-- amount_cents column #967 already added:
--
-- amount_overridden marks a Contract whose amount an Owner or Admin set
-- by hand (PutContractAmountHandler, amount.go) rather than derived from
-- the rate card. A rate change must never clobber that override --
-- #968's AC: "A Contract whose amount was overridden ... keeps the
-- override and does not re-derive." Defaults false: every existing row
-- (and every row PostContractHandler creates from the rate card) starts
-- un-overridden; PutContractAmountHandler is the only writer that ever
-- sets it true, and nothing ever sets it back to false -- a Contract is
-- Draft-only for both override and reprice, so there is no path back to
-- a rate-card-derived amount once a person has typed one in by hand.
--
-- amount_changed_at is when amount_cents last moved for any reason after
-- creation (an override, or a rate-driven reprice) -- #968's AC: "can
-- see that its price changed and when, without hunting for it." Null
-- means the Contract still carries the amount it was created with.
ALTER TABLE contracts ADD COLUMN amount_overridden boolean NOT NULL DEFAULT false;
ALTER TABLE contracts ADD COLUMN amount_changed_at timestamptz;

-- +goose Down
ALTER TABLE contracts DROP COLUMN amount_changed_at;
ALTER TABLE contracts DROP COLUMN amount_overridden;
