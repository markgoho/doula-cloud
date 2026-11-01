-- +goose Up
-- #2477, ADR-0062: the dormant-Practices list moves from doula-api's
-- internal route to ops, as one ops_runtime-only definer function that
-- reads no clients. It shows a Practice's name, balance and last contact
-- and nobody's contact, so it writes no read row. The old read ran as
-- app_runtime under row-level security with no definer function, so
-- there is no grant to revoke.

-- One page of the Practices holding a positive Credit balance whose
-- Staff were last active on or before p_not_seen_since, or never, after
-- the cursor (p_after_at, p_after_id), both NULL for the first page,
-- newest signup first. The balance is the ledger's sum, as
-- billing.Balance reads it; last contact is the latest last_active_at of
-- anyone who is a member. The caller sets the boundary.
-- +goose StatementBegin
CREATE FUNCTION ops_dormant_practices(p_not_seen_since timestamptz, p_after_at timestamptz, p_after_id uuid, p_limit integer)
RETURNS TABLE (id uuid, name text, signed_up_at timestamptz, balance integer, last_contact_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.id, p.name, p.created_at, l.held, m.last_contact
      FROM practices p
     CROSS JOIN LATERAL (
           SELECT COALESCE(sum(c.quantity), 0)::integer AS held
             FROM credit_ledger c WHERE c.practice_id = p.id
     ) l
     CROSS JOIN LATERAL (
           SELECT max(s.last_active_at) AS last_contact
             FROM practice_memberships pm JOIN staff s ON s.id = pm.staff_id
            WHERE pm.practice_id = p.id
     ) m
     WHERE l.held > 0
       AND (m.last_contact IS NULL OR m.last_contact <= p_not_seen_since)
       AND (p_after_at IS NULL OR (p.created_at, p.id) < (p_after_at, p_after_id))
     ORDER BY p.created_at DESC, p.id DESC
     LIMIT p_limit
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_dormant_practices(timestamptz, timestamptz, uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_dormant_practices(timestamptz, timestamptz, uuid, integer) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_dormant_practices(timestamptz, timestamptz, uuid, integer);
