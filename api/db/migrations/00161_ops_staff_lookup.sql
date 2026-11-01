-- +goose Up
-- #2483, ADR-0062: a two-factor reset starts from the sign-in email the
-- person gives on the call. The lookup shows her contact, so it writes a
-- staff_lookup read row, and it runs inside an ops_runtime-only definer
-- function that reads staff, practice_memberships and practices, never
-- clients.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened', 'legal_version_published', 'staff_lookup'));

-- Each live Staff member whose sign-in email is p_email, ignoring case,
-- once for each Practice she belongs to, or once with no Practice.
-- +goose StatementBegin
CREATE FUNCTION ops_find_staff(p_email text)
RETURNS TABLE (staff_id uuid, staff_name text, staff_email text, practice_name text)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT s.id, s.name, s.email, p.name
      FROM staff s
      LEFT JOIN practice_memberships pm ON pm.staff_id = s.id
      LEFT JOIN practices p ON p.id = pm.practice_id
     WHERE lower(s.email) = lower(btrim(p_email)) AND s.deleted_at IS NULL
     ORDER BY s.name, s.id, p.name
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_find_staff(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_find_staff(text) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_find_staff(text);
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened', 'legal_version_published')) NOT VALID;
