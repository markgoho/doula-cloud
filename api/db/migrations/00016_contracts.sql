-- +goose Up
-- A "Contract" row is a Client's Draft/Sent/Signed/Voided agreement for an
-- Engagement (see CONTEXT.md). `prose` is a point-in-time COPY of the
-- Practice's Contract Template prose at the moment this row was created,
-- never a live reference to contract_templates -- editing a template
-- later must never rewrite or break an already-created Contract, the
-- same snapshot discipline as plan_instances (00012_plan_instances.sql).
-- The merge-field keys (e.g. "client_name") are not stored separately:
-- they are a pure function of `prose` (the {{...}} placeholders in it),
-- which is itself frozen at creation, so the Go layer parses them from
-- `prose` on every read/write rather than duplicating that list in a
-- second column. `merge_field_values` is a JSONB object keyed by merge
-- field key, holding what Staff filled in. status has no client-settable
-- path yet -- every row is created 'draft' and no handler in this ticket
-- accepts a status field; sending/signing/voiding land in later tickets
-- (#69 onward). UNIQUE(engagement_id) enforces "a Contract per
-- Engagement" (per the ticket's AC): GET/PUT address a Contract by
-- engagement id alone, so a second row would be unaddressable, and POST
-- relies on the resulting unique-violation to 409 rather than silently
-- creating a duplicate Draft.

CREATE TYPE contract_status AS ENUM ('draft', 'sent', 'signed', 'voided');

CREATE TABLE contracts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id uuid NOT NULL REFERENCES engagements (id),
    status contract_status NOT NULL DEFAULT 'draft',
    prose text NOT NULL,
    merge_field_values jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (engagement_id)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON contracts TO app_runtime;

ALTER TABLE contracts ENABLE ROW LEVEL SECURITY;

-- contracts has no practice_id column, so its practice-tier visibility is
-- an EXISTS subquery against engagements, the same shape as
-- plan_instances_practice_visibility in 00012_plan_instances.sql. No
-- chicken-and-egg problem here: a Contract is always created under an
-- Engagement that already exists, so a single ALL-command policy covers
-- SELECT/INSERT/UPDATE/DELETE. Staff-only for now -- no client-tier
-- policy, that lands in #69 once Send exists.
CREATE POLICY contracts_practice_visibility ON contracts
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = contracts.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
DROP TABLE contracts;
DROP TYPE contract_status;
