-- +goose Up
-- #2476, ADR-0046, ADR-0062: the monthly Credits read moves from
-- doula-api's internal route to ops. Two ops_runtime-only definer
-- functions, neither reading clients. The read shows a Practice's name
-- and two counts and nobody's contact, so it writes no read row.

-- Every Practice that existed in [p_start, p_end), by name: Credits
-- bought is the sum of its purchase lots in the month, Credits spent the
-- count of its consumption rows, and bought_before whether it bought
-- before the month began. A signup bonus, a founding grant, a refund
-- and a forfeit count as neither. The caller sets the boundaries.
-- +goose StatementBegin
CREATE FUNCTION ops_monthly_credits(p_start timestamptz, p_end timestamptz)
RETURNS TABLE (id uuid, name text, credits_bought integer, credits_spent integer, bought_before boolean)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.id, p.name, l.bought, l.spent, l.before
      FROM practices p
     CROSS JOIN LATERAL (
           SELECT COALESCE(sum(c.quantity) FILTER (WHERE c.origin = 'purchase' AND c.created_at >= p_start), 0)::integer AS bought,
                  (count(*) FILTER (WHERE c.origin = 'consumption' AND c.created_at >= p_start))::integer AS spent,
                  COALESCE(bool_or(c.origin = 'purchase' AND c.created_at < p_start), false) AS before
             FROM credit_ledger c
            WHERE c.practice_id = p.id AND c.origin IN ('purchase', 'consumption') AND c.created_at < p_end
     ) l
     WHERE p.created_at < p_end
       AND (p.deleted_at IS NULL OR p.deleted_at >= p_start)
     ORDER BY p.name, p.id
$$;
-- +goose StatementEnd

-- When the first Practice signed up, NULL before any has: the oldest
-- month the read offers.
-- +goose StatementBegin
CREATE FUNCTION ops_first_signup_at()
RETURNS timestamptz
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT min(created_at) FROM practices
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_monthly_credits(timestamptz, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_monthly_credits(timestamptz, timestamptz) TO ops_runtime;
REVOKE EXECUTE ON FUNCTION ops_first_signup_at() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_first_signup_at() TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_first_signup_at();
DROP FUNCTION ops_monthly_credits(timestamptz, timestamptz);
