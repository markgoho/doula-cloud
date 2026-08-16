-- +goose Up
-- A "Contract Template" is a Practice's own boilerplate for the legal
-- agreement it signs with a Client for an Engagement -- legal prose
-- carrying merge-field placeholders (client name, price, engagement
-- dates, scope of service) that a later ticket fills in per Engagement.
-- Unlike plan_templates (00011_plan_templates.sql), this content is not a
-- structured field list: it is prose text, so it is stored as a single
-- text column rather than JSONB.

CREATE TABLE contract_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    prose text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (practice_id)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON contract_templates TO app_runtime;

ALTER TABLE contract_templates ENABLE ROW LEVEL SECURITY;

-- contract_templates carries practice_id directly, so its policy is a
-- plain column comparison, the same shape as
-- plan_templates_practice_visibility in 00011_plan_templates.sql.
CREATE POLICY contract_templates_practice_visibility ON contract_templates
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE contract_templates;
