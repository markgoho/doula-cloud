-- +goose Up
-- #2481, ADR-0062: a refund moves from doula-api's /api/internal to ops.
-- Its SQL becomes ops_runtime-only definer functions, its ledger row
-- keys the operator_acts row that records who took it, and app_runtime
-- can no longer write a refund row at all.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund'));

ALTER TABLE credit_ledger ADD COLUMN operator_act_id uuid REFERENCES operator_acts (id);

-- NOT VALID: a refund issued through doula-api before this records no
-- actor, and that is the truth about it. Every row from here on names one.
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_refund_operator_act
    CHECK (origin <> 'refund' OR operator_act_id IS NOT NULL) NOT VALID;

-- app_runtime keeps INSERT on credit_ledger for purchases, consumption and
-- forfeits, and loses it for a refund. The definer functions below run as
-- the table's owner, which RLS does not bind.
CREATE POLICY credit_ledger_no_refund_by_app_runtime ON credit_ledger
    AS RESTRICTIVE FOR INSERT TO app_runtime
    WITH CHECK (origin <> 'refund');

-- The Practice and its Owners, for the check page and for the act's
-- check that the requesting Owner is one. One row per Owner; a Practice
-- with none still answers one row with a NULL Owner.
-- +goose StatementBegin
CREATE FUNCTION ops_refund_practice(p_practice_id uuid)
RETURNS TABLE (practice_name text, owner_staff_id uuid, owner_name text)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.name, s.id, btrim(s.first_name || ' ' || s.last_name)
      FROM practices p
      LEFT JOIN practice_memberships pm ON pm.practice_id = p.id AND 'owner' = ANY (pm.roles)
      LEFT JOIN staff s ON s.id = pm.staff_id
     WHERE p.id = p_practice_id
     ORDER BY s.last_name, s.first_name, s.id
$$;
-- +goose StatementEnd

-- The Practice's open lots, oldest first, as billing's openLots reads
-- them, after locking the Practice row as ConsumeCredit does, so two
-- refunds cannot both read one lot as unspent.
-- +goose StatementBegin
CREATE FUNCTION ops_refund_lots(p_practice_id uuid)
RETURNS TABLE (
    id uuid, origin text, quantity integer, unit_price_cents integer, tax_cents integer,
    stripe_payment_intent_id text, created_at timestamptz,
    remaining bigint, refunded_count bigint, tax_refunded bigint
)
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
BEGIN
    PERFORM 1 FROM practices p WHERE p.id = p_practice_id FOR UPDATE;
    RETURN QUERY
    SELECT l.id, l.origin::text, l.quantity, l.unit_price_cents, l.tax_cents,
           l.stripe_payment_intent_id, l.created_at,
           l.quantity + COALESCE((
               SELECT SUM(d.quantity) FROM credit_ledger d WHERE d.drawn_lot_id = l.id
           ), 0),
           COALESCE((
               SELECT SUM(-d.quantity) FROM credit_ledger d
                WHERE d.drawn_lot_id = l.id AND d.origin = 'refund'
           ), 0),
           COALESCE((
               SELECT SUM(-d.tax_cents) FROM credit_ledger d
                WHERE d.drawn_lot_id = l.id AND d.origin = 'refund'
           ), 0)
      FROM credit_ledger l
     WHERE l.practice_id = p_practice_id AND l.quantity > 0
     ORDER BY l.created_at, l.id;
END
$$;
-- +goose StatementEnd

-- The refund already issued under a request key at this Practice, so a
-- retried request is answered with it.
-- +goose StatementBegin
CREATE FUNCTION ops_refund_by_request_key(p_practice_id uuid, p_request_key text)
RETURNS TABLE (credits integer, amount_cents integer, tax_cents integer, stripe_refund_id text, payment_intent_id text)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT -r.quantity, -r.quantity * r.unit_price_cents - r.tax_cents, -r.tax_cents,
           r.stripe_refund_id, l.stripe_payment_intent_id
      FROM credit_ledger r
      JOIN credit_ledger l ON l.id = r.drawn_lot_id
     WHERE r.refund_request_key = p_request_key AND r.practice_id = p_practice_id
$$;
-- +goose StatementEnd

-- The refund row, drawn against one of the Practice's own purchase lots
-- and keyed to the act that took it.
-- +goose StatementBegin
CREATE FUNCTION ops_record_refund(
    p_practice_id uuid,
    p_lot_id uuid,
    p_credits integer,
    p_tax_cents integer,
    p_stripe_refund_id text,
    p_request_key text,
    p_operator_act_id uuid,
    p_created_at timestamptz
) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
BEGIN
    INSERT INTO credit_ledger
        (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_refund_id,
         drawn_lot_id, refund_request_key, operator_act_id, created_at)
    SELECT p_practice_id, 'refund', -p_credits, l.unit_price_cents, -p_tax_cents, p_stripe_refund_id,
           l.id, p_request_key, p_operator_act_id, p_created_at
      FROM credit_ledger l
     WHERE l.id = p_lot_id AND l.practice_id = p_practice_id AND l.origin = 'purchase';
    IF NOT FOUND THEN
        RAISE EXCEPTION 'lot % is not a purchase at practice %', p_lot_id, p_practice_id;
    END IF;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_refund_practice(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_refund_lots(uuid) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_refund_by_request_key(uuid, text) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_record_refund(uuid, uuid, integer, integer, text, text, uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_refund_practice(uuid) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_refund_lots(uuid) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_refund_by_request_key(uuid, text) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_record_refund(uuid, uuid, integer, integer, text, text, uuid, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_record_refund(uuid, uuid, integer, integer, text, text, uuid, timestamptz);
DROP FUNCTION ops_refund_by_request_key(uuid, text);
DROP FUNCTION ops_refund_lots(uuid);
DROP FUNCTION ops_refund_practice(uuid);
DROP POLICY credit_ledger_no_refund_by_app_runtime ON credit_ledger;
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_refund_operator_act;
ALTER TABLE credit_ledger DROP COLUMN operator_act_id;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused')) NOT VALID;
