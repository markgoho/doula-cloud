-- +goose Up
-- The Clients list's open-Engagement rollup (#264) drives off
-- `WHERE e.client_id IN ($1..$n)`, one entry per Client on the page, and
-- `engagements` carried no index on `client_id` -- only its primary key
-- and `engagements_practice_idx` (00059_engagements_practice_index.sql),
-- which is practice_id-led and cannot serve a client_id predicate. So
-- every page of the Clients list sequentially scanned the Practice's
-- whole engagements table, which is the same cost 00059 was written to
-- remove for its own query.
--
-- `status` rides along because the same WHERE clause carries
-- `e.status <> 'completed'`: an index-only scan can then discard
-- completed Engagements without visiting the heap. It is deliberately
-- not a partial index `WHERE status <> 'completed'` -- the rollup is not
-- the only reader of a Client's Engagements, and a partial index serves
-- nothing else.
CREATE INDEX engagements_client_idx ON engagements (client_id, status);

-- +goose Down
DROP INDEX engagements_client_idx;
