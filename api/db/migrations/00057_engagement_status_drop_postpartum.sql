-- +goose Up
-- #477. engagement_status carries a stale fourth value: 'postpartum'
-- duplicates the Engagement kind axis (birth/postpartum, what the Practice
-- sold -- engagement_kind, added in 00042_client_intake_schema.sql, its own
-- column and its own enum). CONTEXT.md's Engagement entry and ADR-0005's
-- Client-facing labels only ever describe three phases: intake, active,
-- completed. No code writes 'active' or 'postpartum' to this column today,
-- so 'postpartum' as a status value is pure drift. Any row that has it
-- means care is underway but not complete -- exactly what 'active' means --
-- so the USING clause maps it there.
ALTER TYPE engagement_status RENAME TO engagement_status_old;
CREATE TYPE engagement_status AS ENUM ('intake', 'active', 'completed');
ALTER TABLE engagements
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE engagement_status
        USING (CASE status::text WHEN 'postpartum' THEN 'active' ELSE status::text END)::engagement_status,
    ALTER COLUMN status SET DEFAULT 'intake';
DROP TYPE engagement_status_old;

-- +goose Down
-- Restores the four-value type shape only -- it does not resurrect which
-- rows were originally 'postpartum' before the up migration ran.
ALTER TYPE engagement_status RENAME TO engagement_status_old;
CREATE TYPE engagement_status AS ENUM ('intake', 'active', 'postpartum', 'completed');
ALTER TABLE engagements
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE engagement_status
        USING status::text::engagement_status,
    ALTER COLUMN status SET DEFAULT 'intake';
DROP TYPE engagement_status_old;
