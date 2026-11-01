-- +goose Up
-- #2483, ADR-0062: clearing a Staff member's second factor moves from
-- doula-api's /api/internal to ops. The support row in staff_auth_events
-- keys the operator_acts row that cleared it in place of the free-text
-- actor_operator, app_runtime can no longer write that key, and the act's
-- reads and writes run inside ops_runtime-only definer functions.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared'));

-- No backfill: a support row written through doula-api named its operator
-- in actor_operator, and the actor key replaces that column.
ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events DROP COLUMN actor_operator;
ALTER TABLE staff_auth_events ADD COLUMN actor_operator_act_id uuid REFERENCES operator_acts (id);
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service', 'enrolled', 'removed', 'login_deleted')
        AND actor_staff_id IS NOT NULL AND actor_operator_act_id IS NULL)
    OR (reason = 'support' AND actor_operator_act_id IS NOT NULL AND actor_staff_id IS NULL)
) NOT VALID;

-- app_runtime keeps writing the rows a Staff member causes, and cannot
-- name an operator act, so it cannot write a support row at all.
REVOKE INSERT ON staff_auth_events FROM app_runtime;
GRANT INSERT (staff_id, actor_staff_id, reason, created_at) ON staff_auth_events TO app_runtime;

-- The check page's Staff member: her name and each Practice she belongs
-- to. A Staff member at no Practice is one row with no Practice.
-- +goose StatementBegin
CREATE FUNCTION ops_second_factor_staff(p_staff_id uuid)
RETURNS TABLE (staff_name text, practice_id uuid, practice_name text)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT s.name, p.id, p.name
      FROM staff s
      LEFT JOIN practice_memberships pm ON pm.staff_id = s.id
      LEFT JOIN practices p ON p.id = pm.practice_id
     WHERE s.id = p_staff_id AND s.deleted_at IS NULL
     ORDER BY p.name, p.id
$$;
-- +goose StatementEnd

-- The act's database side, keyed to the act that took it: every session
-- of the identity ends, the two notices are queued, and the support row
-- is written. Returns the identity Identity Platform clears, or NULL when
-- there is no such Staff member.
-- +goose StatementBegin
CREATE FUNCTION ops_record_second_factor_cleared(p_staff_id uuid, p_operator_act_id uuid, p_created_at timestamptz)
RETURNS text
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_identity_uid text;
BEGIN
    SELECT s.identity_uid INTO v_identity_uid FROM staff s WHERE s.id = p_staff_id AND s.deleted_at IS NULL;
    IF v_identity_uid IS NULL THEN
        RETURN NULL;
    END IF;
    DELETE FROM sessions WHERE identity_uid = v_identity_uid;
    INSERT INTO session_notice_outbox (identity_uid, kind) VALUES (v_identity_uid, 'session_revoked')
        ON CONFLICT (identity_uid) WHERE status = 'pending' AND kind = 'session_revoked' DO NOTHING;
    INSERT INTO session_notice_outbox (identity_uid, kind) VALUES (v_identity_uid, 'mfa_recovery_cleared')
        ON CONFLICT (identity_uid) WHERE status = 'pending' AND kind = 'mfa_recovery_cleared' DO NOTHING;
    INSERT INTO staff_auth_events (staff_id, actor_operator_act_id, reason, created_at)
        VALUES (p_staff_id, p_operator_act_id, 'support', p_created_at);
    RETURN v_identity_uid;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_second_factor_staff(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_record_second_factor_cleared(uuid, uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_second_factor_staff(uuid) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_record_second_factor_cleared(uuid, uuid, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_record_second_factor_cleared(uuid, uuid, timestamptz);
DROP FUNCTION ops_second_factor_staff(uuid);
REVOKE INSERT (staff_id, actor_staff_id, reason, created_at) ON staff_auth_events FROM app_runtime;
GRANT INSERT ON staff_auth_events TO app_runtime;
ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events DROP COLUMN actor_operator_act_id;
ALTER TABLE staff_auth_events ADD COLUMN actor_operator text;
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service', 'enrolled', 'removed', 'login_deleted')
        AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
    OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
) NOT VALID;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant')) NOT VALID;
