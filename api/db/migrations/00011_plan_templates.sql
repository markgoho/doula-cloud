-- +goose Up
-- A "Plan Template" is a Practice's own definition of the fields on its
-- Care Plan or Birth Plan: what to ask, in what order, with what field
-- type. See docs/adr/0001-practice-defined-plan-templates.md -- Care Plan
-- and Birth Plan content varies per Practice, so this is deliberately NOT
-- a fixed set of table columns. The field list is stored as a JSONB array
-- of {id, type, label, options, order} objects. Don't "fix" this into
-- plain columns without re-reading that ADR: a filled-out plan instance
-- later stores a snapshot of the field definitions it was created
-- against, not a live reference to this table, so a flexible/JSON shape
-- here is what keeps editing a template safe for Clients who already
-- have a plan.

CREATE TYPE plan_type AS ENUM ('care_plan', 'birth_plan');

CREATE TABLE plan_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    plan_type plan_type NOT NULL,
    fields jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (practice_id, plan_type)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON plan_templates TO app_runtime;

ALTER TABLE plan_templates ENABLE ROW LEVEL SECURITY;

-- plan_templates carries practice_id directly, so its policy is a plain
-- column comparison, the same shape as engagements_practice_visibility in
-- 00005_client_engagement.sql.
CREATE POLICY plan_templates_practice_visibility ON plan_templates
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE plan_templates;
DROP TYPE plan_type;
