-- +goose Up
-- Reusable Idempotency-Key storage per docs/api-design.md section 3 (#126).
-- A caller-supplied key is scoped by (practice_id, staff_id): the same key
-- value from two different Staff members, or the same Staff member at two
-- different Practices, are independent keys. created_at is enough to
-- satisfy "appropriate TTL" for v1 -- no reaper job is implemented; the
-- Go helper's lookup query treats a row past the TTL as not-found instead.
-- UPDATE is granted (not just SELECT/INSERT) because that same helper
-- refreshes an expired row in place via ON CONFLICT ... DO UPDATE, rather
-- than leaving a stale row permanently un-writable once its TTL passes.
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

-- Scoped by both session vars staffauth.Middleware sets once auth passes
-- (app.current_practice_id) and once membership is confirmed
-- (app.current_staff_id, newly added by this change) -- the same
-- fail-closed shape as every other practice-tier policy in this repo, with
-- an extra staff_id compare per this table's own scope.
-- +goose StatementBegin
CREATE POLICY idempotency_keys_own_scope ON idempotency_keys
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY idempotency_keys_own_scope ON idempotency_keys;
DROP TABLE idempotency_keys;
