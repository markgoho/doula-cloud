-- +goose Up
-- #2485, ADR-0062: opening the pilot gate moves from doula-api's
-- /api/internal to ops. The gate keys the operator_acts row that records
-- who opened it in place of the free-text opened_by, and app_runtime no
-- longer reads or writes the gate: signup asks pilot_admits, which is a
-- definer function.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened'));

-- No backfill: a gate opened through doula-api named its opener in
-- opened_by, and the actor key replaces that column, so such a gate is
-- open with no act on record. An act keys only an open gate; the one
-- writer, ops_open_pilot_gate, always keys the act that opened it.
ALTER TABLE pilot_gate DROP CONSTRAINT pilot_gate_opening_is_recorded;
ALTER TABLE pilot_gate DROP CONSTRAINT pilot_gate_opened_by_named;
ALTER TABLE pilot_gate DROP COLUMN opened_by;
ALTER TABLE pilot_gate ADD COLUMN operator_act_id uuid REFERENCES operator_acts (id);
ALTER TABLE pilot_gate ADD CONSTRAINT pilot_gate_act_opened_it
    CHECK (operator_act_id IS NULL OR opened_at IS NOT NULL);

DROP POLICY pilot_gate_operator_select ON pilot_gate;
DROP POLICY pilot_gate_operator_update ON pilot_gate;
REVOKE SELECT, UPDATE ON pilot_gate FROM app_runtime;

-- When the gate was opened, or NULL while it is closed.
-- +goose StatementBegin
CREATE FUNCTION ops_pilot_gate_opened_at()
RETURNS timestamptz
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT g.opened_at FROM pilot_gate g
$$;
-- +goose StatementEnd

-- The opening itself, keyed to the act that made it. False when the gate
-- was already open, which keeps its first record.
-- +goose StatementBegin
CREATE FUNCTION ops_open_pilot_gate(p_operator_act_id uuid, p_opened_at timestamptz)
RETURNS boolean
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
BEGIN
    UPDATE pilot_gate SET operator_act_id = p_operator_act_id, opened_at = p_opened_at
     WHERE opened_at IS NULL;
    RETURN FOUND;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_pilot_gate_opened_at() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_open_pilot_gate(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_pilot_gate_opened_at() TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_open_pilot_gate(uuid, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_open_pilot_gate(uuid, timestamptz);
DROP FUNCTION ops_pilot_gate_opened_at();
GRANT SELECT, UPDATE ON pilot_gate TO app_runtime;
-- +goose StatementBegin
CREATE POLICY pilot_gate_operator_select ON pilot_gate
    FOR SELECT
    USING (current_setting('app.pilot_operator_trusted', true) = 'true');
-- +goose StatementEnd
-- +goose StatementBegin
CREATE POLICY pilot_gate_operator_update ON pilot_gate
    FOR UPDATE
    USING (current_setting('app.pilot_operator_trusted', true) = 'true')
    WITH CHECK (current_setting('app.pilot_operator_trusted', true) = 'true');
-- +goose StatementEnd
ALTER TABLE pilot_gate DROP CONSTRAINT pilot_gate_act_opened_it;
ALTER TABLE pilot_gate DROP COLUMN operator_act_id;
ALTER TABLE pilot_gate ADD COLUMN opened_by text;
ALTER TABLE pilot_gate ADD CONSTRAINT pilot_gate_opened_by_named
    CHECK (btrim(opened_by) <> '') NOT VALID;
ALTER TABLE pilot_gate ADD CONSTRAINT pilot_gate_opening_is_recorded
    CHECK ((opened_by IS NULL) = (opened_at IS NULL)) NOT VALID;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added')) NOT VALID;
