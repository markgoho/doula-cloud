-- +goose Up
-- ADR-0062's two direct grants beside operator_acts: doula-ops records
-- an operator at first sign-in and keeps the operator's sessions. Neither
-- table holds a Practice. operators takes no UPDATE or DELETE: the first
-- sign-in's name and email stay what they were.
GRANT SELECT, INSERT ON operators TO ops_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON operator_sessions TO ops_runtime;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON operator_sessions FROM ops_runtime;
REVOKE SELECT, INSERT ON operators FROM ops_runtime;
