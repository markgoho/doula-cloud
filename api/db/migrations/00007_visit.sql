-- +goose Up
-- A "Visit" is a scheduled meeting between a Doula and a Client within an
-- Engagement -- it may represent the birth itself (see CONTEXT.md).
-- Scheduling fields (date/time, location) are deliberately out of scope
-- for this ticket and deferred to a later one; today a Visit only tracks
-- which Engagement it belongs to and which Doula is assigned to it.

CREATE TABLE visits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id uuid NOT NULL REFERENCES engagements (id),
    staff_id uuid NOT NULL REFERENCES staff (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON visits TO app_runtime;

ALTER TABLE visits ENABLE ROW LEVEL SECURITY;

-- visits has no practice_id column, so its practice-tier visibility is an
-- EXISTS subquery against engagements, the same shape as clients_select in
-- 00005_client_engagement.sql. Unlike clients, there is no chicken-and-egg
-- problem here: a Visit is always created under an Engagement that already
-- exists (and whose practice_id is already known), so a single ALL-command
-- policy covers SELECT/INSERT/UPDATE/DELETE without needing a separate
-- INSERT policy.
CREATE POLICY visits_practice_visibility ON visits
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = visits.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
DROP TABLE visits;
