-- +goose Up
-- #1790: until January, only an address the founder added can create a
-- Practice. Two tables and one function: the list of addresses the
-- founder let into the pilot, the gate that says whether the list still
-- applies, and the one question signup asks of both.
--
-- Neither table is tenant data. Both are global, written only by an
-- operator endpoint behind the internal guard (ADR-0037), and neither is
-- ever read by a Practice.

-- =====================================================================
-- pilot_allowlist: who the founder let in, and when
-- =====================================================================
--
-- address is stored in the spelling staffauth.NormalizeAddress produces
-- (trimmed, lowercased) -- the same spelling staff.email holds -- so the
-- match is case-insensitive by construction. The CHECK is that rule said
-- in SQL, so a row written past the BFF cannot hold a second spelling of
-- one address. added_at comes from clock.Now(ctx) (#773), never now().
CREATE TABLE pilot_allowlist (
    address  text PRIMARY KEY
             CONSTRAINT pilot_allowlist_address_is_normalized
             CHECK (address <> '' AND address = lower(btrim(address))),
    added_by text NOT NULL
             CONSTRAINT pilot_allowlist_added_by_named CHECK (btrim(added_by) <> ''),
    added_at timestamptz NOT NULL
);

-- =====================================================================
-- pilot_gate: one row, closed until the founder opens it
-- =====================================================================
--
-- ADR-0043: a date is a trigger only when someone set it. The gate opens
-- by one deliberate act, and opened_by/opened_at are the record of who
-- did it and when -- both or neither. There is no way back: no endpoint
-- clears them.
CREATE TABLE pilot_gate (
    singleton boolean PRIMARY KEY DEFAULT true
              CONSTRAINT pilot_gate_one_row CHECK (singleton),
    opened_by text
              CONSTRAINT pilot_gate_opened_by_named CHECK (btrim(opened_by) <> ''),
    opened_at timestamptz,
    CONSTRAINT pilot_gate_opening_is_recorded CHECK ((opened_by IS NULL) = (opened_at IS NULL))
);

-- Seeded closed.
INSERT INTO pilot_gate DEFAULT VALUES;

-- =====================================================================
-- The operator's door: app.pilot_operator_trusted
-- =====================================================================
--
-- Its own setting name rather than app.notification_worker_trusted, for
-- 00049's reason: the workers that open that door have no business
-- letting anybody into the pilot. One door, one pair of endpoints.
GRANT SELECT, INSERT ON pilot_allowlist TO app_runtime;
GRANT SELECT, UPDATE ON pilot_gate TO app_runtime;

ALTER TABLE pilot_allowlist ENABLE ROW LEVEL SECURITY;
ALTER TABLE pilot_gate ENABLE ROW LEVEL SECURITY;

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

-- =====================================================================
-- Signup's question, with no door onto the list
-- =====================================================================
--
-- Signup runs for any verified identity, before any Practice exists. A
-- SELECT policy it could satisfy would let any signed-in caller read
-- every pilot address. So signup asks one yes-or-no question of one
-- address, in 00122's feedback_sender shape: is the gate open, or is
-- this address on the list?
-- +goose StatementBegin
CREATE FUNCTION pilot_admits(p_address text) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT EXISTS (SELECT 1 FROM pilot_gate WHERE opened_at IS NOT NULL)
        OR EXISTS (SELECT 1 FROM pilot_allowlist WHERE address = p_address);
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION pilot_admits(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION pilot_admits(text) TO app_runtime;

-- +goose Down
REVOKE EXECUTE ON FUNCTION pilot_admits(text) FROM app_runtime;
DROP FUNCTION pilot_admits(text);
DROP TABLE pilot_gate;
DROP TABLE pilot_allowlist;
