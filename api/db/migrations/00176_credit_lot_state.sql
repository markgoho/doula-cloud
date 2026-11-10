-- +goose Up
-- A Credit lot's state, named once (billing's lotstate.go is the Go
-- half). A lot is a credit_ledger row with quantity > 0 -- a grant or a
-- purchase -- and every Credit that leaves it is a draw row naming it
-- through drawn_lot_id. Before this migration "how much of this lot is
-- left" was written out three times (billing's openLots, 00148's
-- ops_refund_lots, and a remaining > 0 filter in Go), the refund window
-- lived in Go alone, and a forfeit (00089) was one row naming no lot.
-- That last one meant a Practice's balance and its lots disagreed after
-- a deletion: the forfeit took the balance to zero and left every lot
-- reading open, so a refund or a consumption could still draw on them
-- and take the balance below zero.
--
-- Four changes:
--
-- 1. A forfeit is a draw like any other: one row per open lot, each
--    naming its lot and taking exactly what it holds. Every Credit that
--    leaves the ledger now says which lot it left, so the sum of the
--    lots' remainders is the balance, for every Practice, always.
--    ADR-0031 is amended to match.
-- 2. credit_lots is the one derivation of a lot's remainder, its state
--    (open or drained) and whether a refund may draw on it at a given
--    instant. billing reads it directly as app_runtime, and
--    ops_refund_lots, restated, reads it for ops_runtime.
-- 3. The refund window lives in credit_lot_refund_closes_at, because
--    ops_record_refund now decides by it too: a refund row may only be
--    written against a lot credit_lots calls refundable at the instant
--    the refund is recorded.
-- 4. A trigger refuses a draw that names something other than a lot of
--    its own Practice, or that takes the lot below zero.

CREATE TYPE credit_lot_state AS ENUM ('open', 'drained');

-- NOT VALID: every row written from here on is held to it, without
-- scanning trunk's database, which may hold a forfeit row 00089 wrote
-- with no lot. Such a row predates this migration and is the truth about
-- how it was written.
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_lot_or_draw;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
    (origin IN ('signup_bonus', 'founding_grant', 'purchase') AND quantity > 0
        AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
    OR (origin IN ('consumption', 'refund') AND quantity < 0
        AND drawn_lot_id IS NOT NULL)
    OR (origin = 'forfeit' AND quantity < 0
        AND drawn_lot_id IS NOT NULL AND stripe_refund_id IS NULL)
) NOT VALID;

-- When a purchase stops being refundable: three years after it was
-- bought (billing.RefundWindowYears, and docs/copy/support-page.md),
-- counted on the UTC calendar so the answer does not move with the
-- session's time zone.
-- +goose StatementBegin
CREATE FUNCTION credit_lot_refund_closes_at(p_bought_at timestamptz)
RETURNS timestamptz
    LANGUAGE sql IMMUTABLE
    SET search_path = public, pg_temp
AS $$
    SELECT (p_bought_at AT TIME ZONE 'UTC' + interval '3 years') AT TIME ZONE 'UTC'
$$;
-- +goose StatementEnd

-- Every lot at a Practice, oldest first, with what is left of it at
-- p_at. remaining is the lot's quantity plus every draw against it
-- (each draw is negative); refunded_count and tax_refunded count the
-- refunds alone, because a spent Credit's tax is New York's to keep and
-- a refunded one's is not. A lot is open while anything remains, and
-- refundable while it is an open purchase inside its window.
--
-- SECURITY INVOKER: app_runtime reads it under credit_ledger's own
-- practice policy, and a definer function that calls it reads as the
-- table's owner.
-- +goose StatementBegin
CREATE FUNCTION credit_lots(p_practice_id uuid, p_at timestamptz)
RETURNS TABLE (
    id uuid, origin text, quantity integer, unit_price_cents integer, tax_cents integer,
    stripe_payment_intent_id text, created_at timestamptz,
    remaining bigint, refunded_count bigint, tax_refunded bigint,
    state credit_lot_state, refundable boolean
)
    LANGUAGE sql STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT l.id, l.origin::text, l.quantity, l.unit_price_cents, l.tax_cents,
           l.stripe_payment_intent_id, l.created_at,
           d.remaining, d.refunded_count, d.tax_refunded,
           CASE WHEN d.remaining > 0 THEN 'open' ELSE 'drained' END::credit_lot_state,
           l.origin = 'purchase' AND d.remaining > 0
               AND p_at <= credit_lot_refund_closes_at(l.created_at)
      FROM credit_ledger l
     CROSS JOIN LATERAL (
          SELECT l.quantity + COALESCE(SUM(x.quantity), 0) AS remaining,
                 COALESCE(SUM(-x.quantity) FILTER (WHERE x.origin = 'refund'), 0) AS refunded_count,
                 COALESCE(SUM(-x.tax_cents) FILTER (WHERE x.origin = 'refund'), 0) AS tax_refunded
            FROM credit_ledger x
           WHERE x.drawn_lot_id = l.id
     ) d
     WHERE l.practice_id = p_practice_id AND l.quantity > 0
     ORDER BY l.created_at, l.id
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION credit_lot_refund_closes_at(timestamptz) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION credit_lots(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION credit_lot_refund_closes_at(timestamptz) TO app_runtime;
GRANT EXECUTE ON FUNCTION credit_lots(uuid, timestamptz) TO app_runtime;

-- A draw names a lot of its own Practice and leaves it at zero or more.
-- A trigger rather than a CHECK because the rule reads other rows: the
-- lot the draw names, and every other draw against it. It locks the lot
-- row first (FOR NO KEY UPDATE, which the foreign key's own KEY SHARE
-- does not conflict with), so two transactions drawing on one lot take
-- turns here, and the second reads the first's draw once it commits.
-- Every writer already locks the Practice row before it reads its lots;
-- this is the guard that does not depend on that. With every Credit that
-- leaves the ledger a draw, and no lot below zero, no balance is either.
-- SECURITY DEFINER so it sees every draw whichever role inserts; firing
-- a trigger checks no EXECUTE grant, so PUBLIC loses it.
-- +goose StatementBegin
CREATE FUNCTION credit_ledger_draw_within_lot() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_lot_practice uuid;
    v_lot_quantity integer;
    v_remaining bigint;
BEGIN
    SELECT l.practice_id, l.quantity INTO v_lot_practice, v_lot_quantity
      FROM credit_ledger l WHERE l.id = NEW.drawn_lot_id
       FOR NO KEY UPDATE;
    IF v_lot_practice IS DISTINCT FROM NEW.practice_id OR v_lot_quantity <= 0 THEN
        RAISE EXCEPTION 'credit_ledger_draw_within_lot: row % draws on %, which is not a lot at practice %',
            NEW.id, NEW.drawn_lot_id, NEW.practice_id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'credit_ledger_draw_within_lot';
    END IF;
    SELECT c.remaining INTO v_remaining
      FROM credit_lots(NEW.practice_id, NEW.created_at) c WHERE c.id = NEW.drawn_lot_id;
    IF v_remaining < 0 THEN
        RAISE EXCEPTION 'credit_ledger_draw_within_lot: row % takes lot % to %', NEW.id, NEW.drawn_lot_id, v_remaining
            USING ERRCODE = 'check_violation', CONSTRAINT = 'credit_ledger_draw_within_lot';
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION credit_ledger_draw_within_lot() FROM PUBLIC;

CREATE TRIGGER credit_ledger_draw_within_lot
    AFTER INSERT ON credit_ledger
    FOR EACH ROW WHEN (NEW.drawn_lot_id IS NOT NULL)
    EXECUTE FUNCTION credit_ledger_draw_within_lot();

-- 00148's ops_refund_lots, restated over credit_lots: the instant now
-- comes in, so the window is decided where ops_record_refund decides it.
-- It still locks the Practice row first, as ConsumeCredit does.
DROP FUNCTION ops_refund_lots(uuid);
-- +goose StatementBegin
CREATE FUNCTION ops_refund_lots(p_practice_id uuid, p_at timestamptz)
RETURNS TABLE (
    id uuid, origin text, quantity integer, unit_price_cents integer, tax_cents integer,
    stripe_payment_intent_id text, created_at timestamptz,
    remaining bigint, refunded_count bigint, tax_refunded bigint,
    state credit_lot_state, refundable boolean
)
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
BEGIN
    PERFORM 1 FROM practices p WHERE p.id = p_practice_id FOR UPDATE;
    RETURN QUERY SELECT c.* FROM credit_lots(p_practice_id, p_at) c;
END
$$;
-- +goose StatementEnd

-- 00148's ops_record_refund, restated: the lot must be refundable at the
-- instant the refund is recorded -- an open purchase of this Practice,
-- inside its window -- rather than only a purchase. The trigger above
-- refuses a refund larger than what the lot holds.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_record_refund(
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
    SELECT p_practice_id, 'refund', -p_credits, c.unit_price_cents, -p_tax_cents, p_stripe_refund_id,
           c.id, p_request_key, p_operator_act_id, p_created_at
      FROM credit_lots(p_practice_id, p_created_at) c
     WHERE c.id = p_lot_id AND c.refundable;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'lot % is not a refundable purchase at practice % at %', p_lot_id, p_practice_id, p_created_at;
    END IF;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_refund_lots(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_refund_lots(uuid, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_refund_lots(uuid, timestamptz);

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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_record_refund(
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

REVOKE EXECUTE ON FUNCTION ops_refund_lots(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_refund_lots(uuid) TO ops_runtime;

DROP TRIGGER credit_ledger_draw_within_lot ON credit_ledger;
DROP FUNCTION credit_ledger_draw_within_lot();
DROP FUNCTION credit_lots(uuid, timestamptz);
DROP FUNCTION credit_lot_refund_closes_at(timestamptz);

ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_lot_or_draw;
ALTER TABLE credit_ledger ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
    (origin IN ('signup_bonus', 'founding_grant', 'purchase') AND quantity > 0
        AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
    OR (origin IN ('consumption', 'refund') AND quantity < 0
        AND drawn_lot_id IS NOT NULL)
    OR (origin = 'forfeit' AND quantity < 0
        AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
) NOT VALID;

DROP TYPE credit_lot_state;
