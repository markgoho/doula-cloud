-- +goose Up
-- Loosens contracts' UNIQUE (engagement_id) (00016_contracts.sql) so an
-- Engagement can accumulate more than one Contract row over time --
-- required for #72's Void-then-recreate flow (#66 user story 22): once
-- a Contract is voided, Staff create a fresh Draft for the same
-- Engagement via PostContractHandler's existing create flow (#68),
-- cycling through Draft -> Sent -> Signed again. The old, table-wide
-- unique constraint made that INSERT 409 forever once any row existed
-- for the Engagement, voided or not -- the very recovery path Void
-- exists for was unreachable.
--
-- A partial unique index replaces it: at most one *non-voided* row per
-- Engagement (still exactly the "one Contract in flight" invariant every
-- handler relies on), but any number of voided rows -- each one is a
-- permanent, superseded record, never reused or merged. Postgres raises
-- the same SQLSTATE 23505 on a partial-unique-index violation as on a
-- table constraint, so PostContractHandler's isUniqueViolation 409
-- handling needs no change.
ALTER TABLE contracts DROP CONSTRAINT contracts_engagement_id_key;
CREATE UNIQUE INDEX contracts_engagement_id_active_key ON contracts (engagement_id)
    WHERE status <> 'voided';

-- +goose Down
DROP INDEX contracts_engagement_id_active_key;
ALTER TABLE contracts ADD CONSTRAINT contracts_engagement_id_key UNIQUE (engagement_id);
