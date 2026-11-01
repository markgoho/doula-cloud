-- +goose Up
-- The Operator log pages operator_acts by (created_at, id) newest first,
-- read under ops_runtime's own grant. The rows hold only ids, so the log
-- names the Practices and Staff members they act on through this one
-- function: a name and, for a Practice, whether its Deletion finalized.
-- Names only, never a contact.
CREATE INDEX operator_acts_created_at_id_idx ON operator_acts (created_at DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION ops_operator_log_names(p_practice_ids uuid[], p_staff_ids uuid[])
RETURNS TABLE (kind text, id uuid, name text, deleted boolean)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT 'practice', p.id, p.name, p.deleted_at IS NOT NULL
      FROM practices p
     WHERE p.id = ANY (p_practice_ids)
    UNION ALL
    SELECT 'staff', s.id, NULLIF(btrim(s.first_name || ' ' || s.last_name), ''), false
      FROM staff s
     WHERE s.id = ANY (p_staff_ids)
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_operator_log_names(uuid[], uuid[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_operator_log_names(uuid[], uuid[]) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_operator_log_names(uuid[], uuid[]);
DROP INDEX operator_acts_created_at_id_idx;
