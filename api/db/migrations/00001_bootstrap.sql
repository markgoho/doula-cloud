-- +goose Up
CREATE TABLE goose_bootstrap_check (
	id serial PRIMARY KEY
);

-- +goose Down
DROP TABLE goose_bootstrap_check;
