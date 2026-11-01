-- +goose Up
-- #2471: a fresh authenticator-app code stamps the operator's session,
-- and an act that needs one reads the stamp on its own request
-- (ADR-0062). NULL until the session's first step-up.
ALTER TABLE operator_sessions ADD COLUMN step_up_at timestamptz;

-- +goose Down
ALTER TABLE operator_sessions DROP COLUMN step_up_at;
