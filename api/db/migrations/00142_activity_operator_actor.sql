-- +goose Up
-- #2480, ADR-0022's amendment of 2026-10-31: an operator act that touches
-- one Practice writes one activity row there, keyed to its operator_acts
-- row. operator_acts keeps no foreign key to a Practice and refuses every
-- DELETE, so Practice Deletion keeps the act this row points at.
ALTER TABLE activity ADD COLUMN actor_operator_act_id uuid REFERENCES operator_acts (id);

ALTER TABLE activity DROP CONSTRAINT activity_actor;
ALTER TABLE activity ADD CONSTRAINT activity_actor CHECK (
    (actor_kind = 'staff' AND actor_staff_id IS NOT NULL AND actor_client_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'client' AND actor_client_id IS NOT NULL AND actor_staff_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'system' AND actor_staff_id IS NULL AND actor_client_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'operator' AND actor_operator_act_id IS NOT NULL AND actor_staff_id IS NULL AND actor_client_id IS NULL)
);

CREATE INDEX activity_actor_operator_act ON activity (actor_operator_act_id)
    WHERE actor_operator_act_id IS NOT NULL;

-- ops_runtime holds no grant on activity, and activity's one policy
-- compares app.current_practice_id. The function runs as the table's
-- owner, which RLS does not bind, and writes only operator rows.
-- +goose StatementBegin
CREATE FUNCTION record_operator_activity(
    p_practice_id uuid,
    p_subject_kind text,
    p_subject_id uuid,
    p_action text,
    p_diff jsonb,
    p_operator_act_id uuid,
    p_created_at timestamptz
) RETURNS void
    LANGUAGE sql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
    INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id, created_at)
    VALUES (p_practice_id, p_subject_kind, p_subject_id, p_action, coalesce(p_diff, '{}'::jsonb), 'operator',
            p_operator_act_id, coalesce(p_created_at, now()))
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION record_operator_activity(uuid, text, uuid, text, jsonb, uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION record_operator_activity(uuid, text, uuid, text, jsonb, uuid, timestamptz) TO ops_runtime;

-- app_runtime reads no operators row. A reader of an operator row learns
-- the operator's name through this function, and only for an act an
-- activity row at the reader's own Practice points at. STRICT, so a row
-- of any other actor kind never calls it.
-- +goose StatementBegin
CREATE FUNCTION operator_act_actor_name(p_operator_act_id uuid) RETURNS text
    LANGUAGE sql SECURITY DEFINER STABLE STRICT
    SET search_path = public, pg_temp
AS $$
    SELECT o.name
      FROM operator_acts oa
      JOIN operators o ON o.id = oa.operator_id
     WHERE oa.id = p_operator_act_id
       AND EXISTS (
           SELECT 1 FROM activity a
            WHERE a.actor_operator_act_id = p_operator_act_id
              AND a.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
       )
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION operator_act_actor_name(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION operator_act_actor_name(uuid) TO app_runtime;

-- +goose Down
DROP FUNCTION operator_act_actor_name(uuid);
DROP FUNCTION record_operator_activity(uuid, text, uuid, text, jsonb, uuid, timestamptz);
DELETE FROM activity WHERE actor_kind = 'operator';
ALTER TABLE activity DROP CONSTRAINT activity_actor;
ALTER TABLE activity ADD CONSTRAINT activity_actor CHECK (
    (actor_kind = 'staff' AND actor_staff_id IS NOT NULL AND actor_client_id IS NULL)
    OR (actor_kind = 'client' AND actor_client_id IS NOT NULL AND actor_staff_id IS NULL)
    OR (actor_kind = 'system' AND actor_staff_id IS NULL AND actor_client_id IS NULL)
);
ALTER TABLE activity DROP COLUMN actor_operator_act_id;
