-- +goose Up
-- #2610: doula-ops admits only a login already on the operators list, so
-- ops_runtime no longer inserts into operators. A login is added by the
-- founder, as the database owner, with the INSERT docs/infrastructure.md
-- gives ("Adding an operator").
REVOKE INSERT ON operators FROM ops_runtime;

-- +goose Down
GRANT INSERT ON operators TO ops_runtime;
