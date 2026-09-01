-- +goose Up
-- #476's Engagement activity read is cursor-paginated per
-- docs/api-design.md section 4: WHERE practice_id = $1 AND subject_kind
-- = 'engagement' AND subject_id = $2 AND (created_at, id) < ($3, $4)
-- ORDER BY created_at DESC, id DESC. 00051_activity_log.sql's
-- activity_subject index ends at created_at, one column short of the
-- cursor's own tuple -- two rows written in the same transaction (this
-- ticket writes several: e.g. Contract voided alongside the Offer it
-- reopens) share a created_at, and without id in the index Postgres
-- still has to sort those ties instead of walking the index in cursor
-- order. Replaces rather than adds to 00051's index -- keeping both
-- would just make every INSERT maintain two near-identical indexes.
DROP INDEX activity_subject;

CREATE INDEX activity_subject
    ON activity (practice_id, subject_kind, subject_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX activity_subject;

CREATE INDEX activity_subject ON activity (practice_id, subject_kind, subject_id, created_at);
