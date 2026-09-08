-- +goose Up
-- A Practice-scoped rate card (#966): the flat amount in cents an Owner
-- or Admin charges for each Engagement kind (engagement_kind,
-- 00042_client_intake_schema.sql). Scope is deliberately one flat
-- amount per kind -- tiers, hourly postpartum, per-visit and insurance
-- shapes are #248 and #335, and the one-rate-per-kind assumption is
-- provisional pending #243.
--
-- Sparse rather than one row per Practice with two nullable columns: no
-- row for a (practice_id, kind) pair reads as "no rate set", the same
-- shape contract_templates already uses for "no template written yet",
-- so "a Practice with no rate set is a valid state" (#966's AC) needs no
-- special-cased NULL handling on top of the ordinary missing-row case.
CREATE TABLE practice_rates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    kind engagement_kind NOT NULL,
    amount_cents bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (practice_id, kind)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON practice_rates TO app_runtime;

ALTER TABLE practice_rates ENABLE ROW LEVEL SECURITY;

-- practice_rates carries practice_id directly, so its policy is the same
-- plain column comparison contract_templates_practice_visibility already
-- uses (00014_contract_templates.sql).
CREATE POLICY practice_rates_practice_visibility ON practice_rates
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE practice_rates;
