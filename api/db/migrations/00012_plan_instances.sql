-- +goose Up
-- A "Plan Instance" is a Client's filled-out Care Plan or Birth Plan for
-- an Engagement (see docs/adr/0001-practice-defined-plan-templates.md).
-- `fields` is a point-in-time COPY of the Practice's Plan Template field
-- definitions at the moment this row was created, never a live reference
-- to plan_templates -- editing a template later must never rewrite or
-- break an Engagement's already-completed plan. `answers` is a JSONB
-- object keyed by field id, holding what the Staff member filled in.

CREATE TABLE plan_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id uuid NOT NULL REFERENCES engagements (id),
    plan_type plan_type NOT NULL,
    fields jsonb NOT NULL,
    answers jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (engagement_id, plan_type)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON plan_instances TO app_runtime;

ALTER TABLE plan_instances ENABLE ROW LEVEL SECURITY;

-- plan_instances has no practice_id column, so its practice-tier
-- visibility is an EXISTS subquery against engagements, the same shape as
-- visits_practice_visibility in 00007_visit.sql. No chicken-and-egg
-- problem here (unlike clients_select in 00005_client_engagement.sql): a
-- Plan Instance is always created under an Engagement that already
-- exists, so a single ALL-command policy covers
-- SELECT/INSERT/UPDATE/DELETE without needing a separate INSERT policy.
CREATE POLICY plan_instances_practice_visibility ON plan_instances
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = plan_instances.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
DROP TABLE plan_instances;
