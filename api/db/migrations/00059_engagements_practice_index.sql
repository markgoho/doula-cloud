-- +goose Up
-- The Practice-wide "Contracts awaiting signature" roll-up (#426) reads
--   FROM contracts c JOIN engagements e ON e.id = c.engagement_id
--   WHERE e.practice_id = $1 AND c.status IN ('draft','sent')
--   ORDER BY c.created_at, c.id LIMIT 31
-- The contracts half already has its access path -- 00020's partial
-- unique index on (engagement_id) WHERE status <> 'voided' probes one
-- Engagement's live Contract directly. The engagements half had none:
-- 00005_client_engagement.sql created the table with a primary key and
-- two foreign keys and nothing else, so every reader that starts from
-- "the Engagements at this Practice" was a sequential scan of every
-- Engagement of every Practice.
--
-- practice_id alone, not a (practice_id, created_at) tuple like
-- 00056_invoices_practice_listing_index.sql: this query does not sort by
-- anything on engagements. It seeks the Practice's Engagements, probes
-- each one's live Contract, and sorts the survivors -- a set bounded by
-- the Contracts actually outstanding, which is the whole point of the
-- roll-up. A tuple index would only add bytes no plan reads.
CREATE INDEX engagements_practice_idx ON engagements (practice_id);

-- +goose Down
DROP INDEX engagements_practice_idx;
