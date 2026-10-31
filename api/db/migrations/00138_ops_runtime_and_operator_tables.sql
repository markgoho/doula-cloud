-- +goose Up
-- ADR-0062: doula-ops connects as ops_runtime, a role doula-api never
-- holds. It has no table grant but SELECT and INSERT on operator_acts,
-- the operator's own append-only record. operator_acts keeps no foreign
-- key to a Practice, a Staff member or a Feedback piece, so a Practice's
-- Deletion never reaches it.

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ops_runtime') THEN
        CREATE ROLE ops_runtime NOLOGIN;
    END IF;
END
$$;
-- +goose StatementEnd

CREATE TABLE operators (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL UNIQUE,
    name text NOT NULL,
    email text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE operator_sessions (
    token_hash text PRIMARY KEY,
    operator_id uuid NOT NULL REFERENCES operators (id),
    idle_expires_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX operator_sessions_expires_at_idx ON operator_sessions (expires_at);

CREATE TABLE operator_acts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operator_id uuid NOT NULL REFERENCES operators (id),
    action text NOT NULL CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused')),
    subject_kind text CHECK (subject_kind IN ('practice', 'staff', 'feedback')),
    subject_id uuid,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    step_up_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((subject_kind IS NULL) = (subject_id IS NULL))
);

CREATE INDEX operator_acts_created_at_idx ON operator_acts (created_at DESC);

GRANT SELECT, INSERT ON operator_acts TO ops_runtime;

-- No grant reaches the table's owner, so a trigger refuses every change
-- to a written row whoever asks, the owner and a superuser included.
-- +goose StatementBegin
CREATE FUNCTION operator_acts_refuse_change() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    RAISE EXCEPTION 'operator_acts is append-only: % is refused', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END
$$;
-- +goose StatementEnd

CREATE TRIGGER operator_acts_refuse_update_delete
    BEFORE UPDATE OR DELETE ON operator_acts
    FOR EACH ROW EXECUTE FUNCTION operator_acts_refuse_change();

CREATE TRIGGER operator_acts_refuse_truncate
    BEFORE TRUNCATE ON operator_acts
    FOR EACH STATEMENT EXECUTE FUNCTION operator_acts_refuse_change();

-- Four definer functions were left executable by PUBLIC, so every role,
-- ops_runtime among them, could call them. They are RLS predicates, read
-- under app_runtime's policies; site_builder keeps what it held.
REVOKE EXECUTE ON FUNCTION current_staff_id() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION staff_is_sole_owner(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION client_portal_sees_staff(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION staff_was_ever_a_member_at_practice(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION current_staff_id() TO app_runtime, site_builder;
GRANT EXECUTE ON FUNCTION staff_is_sole_owner(uuid) TO app_runtime, site_builder;
GRANT EXECUTE ON FUNCTION client_portal_sees_staff(uuid) TO app_runtime, site_builder;
GRANT EXECUTE ON FUNCTION staff_was_ever_a_member_at_practice(uuid, uuid) TO app_runtime, site_builder;

-- +goose Down
GRANT EXECUTE ON FUNCTION current_staff_id() TO PUBLIC;
GRANT EXECUTE ON FUNCTION staff_is_sole_owner(uuid) TO PUBLIC;
GRANT EXECUTE ON FUNCTION client_portal_sees_staff(uuid) TO PUBLIC;
GRANT EXECUTE ON FUNCTION staff_was_ever_a_member_at_practice(uuid, uuid) TO PUBLIC;
DROP TABLE operator_acts;
DROP FUNCTION operator_acts_refuse_change();
DROP TABLE operator_sessions;
DROP TABLE operators;
