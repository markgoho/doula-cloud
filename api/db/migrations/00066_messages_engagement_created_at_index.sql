-- +goose Up
-- #455's "waiting on a reply" roll-up needs "the latest Message on this
-- Engagement, newest first" -- a LATERAL subquery per Engagement
-- (api/internal/message/awaiting.go). messages carries no index at all
-- today (00008_messaging.sql never added one, and neither has anything
-- since), so that per-Engagement lookup and message.listMessages' own
-- `WHERE engagement_id = $1 ORDER BY created_at DESC, id DESC` both sequential-
-- scan the whole table. id DESC is the tiebreak both queries already order
-- by (two Messages can share a created_at within one transaction, the same
-- reasoning as activity_subject, 00058), so it belongs in the index too.
CREATE INDEX messages_engagement_created_at
    ON messages (engagement_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX messages_engagement_created_at;
