-- +goose Up
-- #2482, ADR-0062: a founding grant moves from doula-api's /api/internal
-- to ops. It is sized from the roster inside an ops_runtime-only definer
-- function, its ledger row keys the operator_acts row that records who
-- gave it in place of the free-text granted_by, and app_runtime can no
-- longer write a founding grant row at all.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant'));

-- No backfill: a grant given through doula-api named its giver in
-- granted_by, and the actor key replaces that column. Every grant from
-- here on keys the act that gave it.
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_granted_by;
ALTER TABLE credit_ledger DROP COLUMN granted_by;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_founding_grant_operator_act
    CHECK (origin <> 'founding_grant' OR operator_act_id IS NOT NULL) NOT VALID;

CREATE POLICY credit_ledger_no_founding_grant_by_app_runtime ON credit_ledger
    AS RESTRICTIVE FOR INSERT TO app_runtime
    WITH CHECK (origin <> 'founding_grant');

-- The check page's Practice: its name, its roster, the grant that roster
-- sizes at three Credits a Staff member (#439), and the grant it already
-- holds, if any, with the act that gave it.
-- +goose StatementBegin
CREATE FUNCTION ops_founding_grant_practice(p_practice_id uuid)
RETURNS TABLE (practice_name text, staff_count integer, credits integer, granted_credits integer, granted_act_id uuid)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.name, r.n, r.n * 3, g.quantity, g.operator_act_id
      FROM practices p
     CROSS JOIN LATERAL (
           SELECT count(*)::integer AS n FROM practice_memberships pm WHERE pm.practice_id = p.id
     ) r
      LEFT JOIN credit_ledger g ON g.practice_id = p.id AND g.origin = 'founding_grant'
     WHERE p.id = p_practice_id
$$;
-- +goose StatementEnd

-- The grant itself, keyed to the act that gave it, written only when
-- p_credits is what the roster sizes under the Practice's lock. NULL
-- when nothing was written: the Practice already holds a grant, has no
-- Staff, or its roster changed since the act read it.
-- +goose StatementBegin
CREATE FUNCTION ops_record_founding_grant(p_practice_id uuid, p_operator_act_id uuid, p_credits integer, p_created_at timestamptz)
RETURNS integer
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_credits integer;
BEGIN
    PERFORM 1 FROM practices p WHERE p.id = p_practice_id FOR UPDATE;
    INSERT INTO credit_ledger (practice_id, origin, quantity, operator_act_id, created_at)
    SELECT p_practice_id, 'founding_grant', g.credits, p_operator_act_id, p_created_at
      FROM ops_founding_grant_practice(p_practice_id) g
     WHERE g.granted_credits IS NULL AND g.credits > 0 AND g.credits = p_credits
    RETURNING quantity INTO v_credits;
    RETURN v_credits;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_founding_grant_practice(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_record_founding_grant(uuid, uuid, integer, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_founding_grant_practice(uuid) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_record_founding_grant(uuid, uuid, integer, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_record_founding_grant(uuid, uuid, integer, timestamptz);
DROP FUNCTION ops_founding_grant_practice(uuid);
DROP POLICY credit_ledger_no_founding_grant_by_app_runtime ON credit_ledger;
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_founding_grant_operator_act;
ALTER TABLE credit_ledger ADD COLUMN granted_by text;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_granted_by
    CHECK ((origin = 'founding_grant') = (granted_by IS NOT NULL AND granted_by <> '')) NOT VALID;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund')) NOT VALID;
