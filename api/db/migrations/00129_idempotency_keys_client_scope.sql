-- +goose Up
-- #1924: idempotency_keys takes a Client-scoped key path beside the Staff
-- one, so a Client-portal POST can be wrapped in idempotency.WrapClient the
-- way a Staff POST is wrapped in idempotency.Wrap.
--
-- 00027 scoped a key by (practice_id, staff_id), both NOT NULL and both in
-- the primary key, and its policy compared against app.current_staff_id --
-- none of which clientauth.Middleware sets. A Client caller has a Client id
-- (app.current_client_id) and, through her clients row, one Practice.
--
-- The table is dropped and created again rather than altered: a primary key
-- cannot hold a nullable column, and every row in it is a response cached
-- for 48 hours at most (idempotency.ttl), so nothing in it outlives a
-- retry window. Pre-launch, there is no live data to keep. A table this
-- migration creates also holds no rows for any later statement in it to
-- trip over (rowsafety.go's createTable).
--
-- One table with two caller columns, exactly one of them set, rather than
-- a second table: the Go helper keeps one lookup and one save statement for
-- both populations, and UNIQUE NULLS NOT DISTINCT over all four scope
-- columns is the one arbiter save's ON CONFLICT names -- a Staff row has
-- client_id NULL and a Client row has staff_id NULL, and NULLS NOT DISTINCT
-- makes two rows with the same key in the same scope collide.
DROP POLICY idempotency_keys_own_scope ON idempotency_keys;
DROP TABLE idempotency_keys;

CREATE TABLE idempotency_keys (
    key text NOT NULL,
    practice_id uuid NOT NULL REFERENCES practices (id),
    staff_id uuid REFERENCES staff (id),
    client_id uuid REFERENCES clients (id),
    status_code int NOT NULL,
    response_body bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_one_caller CHECK (num_nonnulls(staff_id, client_id) = 1),
    CONSTRAINT idempotency_keys_scope UNIQUE NULLS NOT DISTINCT (key, practice_id, staff_id, client_id)
);

GRANT SELECT, INSERT, UPDATE ON idempotency_keys TO app_runtime;

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;

-- Two branches, one per population, each NULL (so false) under the other's
-- session: staffauth.Middleware sets app.current_practice_id and
-- app.current_staff_id and never app.current_client_id, and
-- clientauth.Middleware sets app.current_client_id and neither of the
-- others. With no session variable set, both are NULL and nothing is
-- visible -- the same fail-closed shape as every other policy here.
--
-- The Client branch scopes a key to her own Client and to that Client's own
-- Practice: clientauth.Middleware sets no Practice id, so the Practice is
-- read from her own clients row, which clients_self_visibility (00009) lets
-- her session read. A row naming her Client id under another Practice is
-- refused on insert and invisible on read.
-- +goose StatementBegin
CREATE POLICY idempotency_keys_own_scope ON idempotency_keys
    USING (
        (
            staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
            AND practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
        OR (
            client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
            AND practice_id = (SELECT c.practice_id FROM clients c WHERE c.id = idempotency_keys.client_id)
        )
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY idempotency_keys_own_scope ON idempotency_keys;
DROP TABLE idempotency_keys;

CREATE TABLE idempotency_keys (
    key text NOT NULL,
    practice_id uuid NOT NULL REFERENCES practices (id),
    staff_id uuid NOT NULL REFERENCES staff (id),
    status_code int NOT NULL,
    response_body bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (key, practice_id, staff_id)
);

GRANT SELECT, INSERT, UPDATE ON idempotency_keys TO app_runtime;

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY idempotency_keys_own_scope ON idempotency_keys
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
    );
-- +goose StatementEnd
