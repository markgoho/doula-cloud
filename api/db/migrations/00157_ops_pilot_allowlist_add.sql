-- +goose Up
-- #2484, ADR-0062: adding an address to the pilot allowlist moves from
-- doula-api's /api/internal to ops. The row keys the operator_acts row
-- that records who added it in place of the free-text added_by, and
-- app_runtime no longer reads or writes the list: signup asks
-- pilot_admits, which is a definer function.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added'));

-- No backfill: an address added through doula-api named its adder in
-- added_by, and the actor key replaces that column. Every address from
-- here on keys the act that added it.
ALTER TABLE pilot_allowlist DROP CONSTRAINT pilot_allowlist_added_by_named;
ALTER TABLE pilot_allowlist DROP COLUMN added_by;
ALTER TABLE pilot_allowlist ADD COLUMN operator_act_id uuid REFERENCES operator_acts (id);
ALTER TABLE pilot_allowlist ADD CONSTRAINT pilot_allowlist_operator_act
    CHECK (operator_act_id IS NOT NULL) NOT VALID;

DROP POLICY pilot_allowlist_operator_select ON pilot_allowlist;
DROP POLICY pilot_allowlist_operator_insert ON pilot_allowlist;
REVOKE SELECT, INSERT ON pilot_allowlist FROM app_runtime;

-- When an address was added, or no row when it is not on the list.
-- p_address is already in staffauth.NormalizeAddress's spelling.
-- +goose StatementBegin
CREATE FUNCTION ops_pilot_address_added_at(p_address text)
RETURNS TABLE (added_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT a.added_at FROM pilot_allowlist a WHERE a.address = p_address
$$;
-- +goose StatementEnd

-- The add itself, keyed to the act that made it. False when the address
-- was already on the list, which keeps its first record.
-- +goose StatementBegin
CREATE FUNCTION ops_add_pilot_address(p_address text, p_operator_act_id uuid, p_added_at timestamptz)
RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
BEGIN
    INSERT INTO pilot_allowlist (address, operator_act_id, added_at)
    VALUES (p_address, p_operator_act_id, p_added_at)
    ON CONFLICT (address) DO NOTHING;
    RETURN FOUND;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_pilot_address_added_at(text) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_add_pilot_address(text, uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_pilot_address_added_at(text) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_add_pilot_address(text, uuid, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_add_pilot_address(text, uuid, timestamptz);
DROP FUNCTION ops_pilot_address_added_at(text);
GRANT SELECT, INSERT ON pilot_allowlist TO app_runtime;
-- +goose StatementBegin
CREATE POLICY pilot_allowlist_operator_select ON pilot_allowlist
    FOR SELECT
    USING (current_setting('app.pilot_operator_trusted', true) = 'true');
-- +goose StatementEnd
-- +goose StatementBegin
CREATE POLICY pilot_allowlist_operator_insert ON pilot_allowlist
    FOR INSERT
    WITH CHECK (current_setting('app.pilot_operator_trusted', true) = 'true');
-- +goose StatementEnd
ALTER TABLE pilot_allowlist DROP CONSTRAINT pilot_allowlist_operator_act;
ALTER TABLE pilot_allowlist DROP COLUMN operator_act_id;
ALTER TABLE pilot_allowlist ADD COLUMN added_by text;
ALTER TABLE pilot_allowlist ADD CONSTRAINT pilot_allowlist_added_by_named
    CHECK (btrim(added_by) <> '') NOT VALID;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read')) NOT VALID;
