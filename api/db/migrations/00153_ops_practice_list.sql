-- +goose Up
-- #2475, ADR-0062: the Practice list and its Deleted list in ops. One
-- ops_runtime-only definer function reads every column the list prints
-- and nothing else; it never reads clients. Opening either list shows
-- Owners' contacts, so it writes an owner_contact_read row.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read'));

-- The list pages by (created_at, id) newest first, and each page's
-- Practices are summed from these three.
CREATE INDEX practices_created_at_id ON practices (created_at DESC, id DESC);
CREATE INDEX credit_ledger_practice_origin ON credit_ledger (practice_id, origin);
CREATE INDEX feedback_practice_id ON feedback (practice_id);

-- One page of the live list (p_deleted false) or the Deleted list (true),
-- after the cursor (p_after_at, p_after_id), both NULL for the first page.
-- Owners are a JSON array of name and email, by last then first name.
-- Credits held is the ledger's sum, as billing.Balance reads it, and
-- Engagements started its consumption rows, as the monthly read counts
-- them. Open Feedback is every piece sent from the Practice still on
-- file.
-- +goose StatementBegin
CREATE FUNCTION ops_practice_list(p_deleted boolean, p_after_at timestamptz, p_after_id uuid, p_limit integer)
RETURNS TABLE (
    id uuid, name text, signed_up_at timestamptz, owners jsonb, staff_count integer,
    connected boolean, card_payments_status text, payouts_status text,
    requirements_due jsonb, connect_updated_at timestamptz,
    credits_held integer, engagements_started integer,
    grant_credits integer, grant_given_by text, grant_given_at timestamptz,
    deletion_requested_at timestamptz, deletion_finalize_at timestamptz, deleted_at timestamptz,
    open_feedback integer
)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.id, p.name, p.created_at, m.owners, m.staff_count,
           p.stripe_connect_account_id IS NOT NULL, p.stripe_connect_card_payments_status,
           p.stripe_connect_payouts_status, to_jsonb(p.stripe_connect_requirements_due), p.stripe_connect_status_updated_at,
           l.held, l.started, g.quantity, o.name, g.created_at,
           p.deletion_requested_at, p.deletion_finalize_at, p.deleted_at,
           f.n
      FROM practices p
     CROSS JOIN LATERAL (
           SELECT count(*)::integer AS staff_count,
                  COALESCE(jsonb_agg(jsonb_build_object('name', btrim(s.first_name || ' ' || s.last_name), 'email', s.email)
                               ORDER BY s.last_name, s.first_name, s.id) FILTER (WHERE 'owner' = ANY (pm.roles)), '[]') AS owners
             FROM practice_memberships pm JOIN staff s ON s.id = pm.staff_id
            WHERE pm.practice_id = p.id
     ) m
     CROSS JOIN LATERAL (
           SELECT COALESCE(sum(c.quantity), 0)::integer AS held,
                  (count(*) FILTER (WHERE c.origin = 'consumption'))::integer AS started
             FROM credit_ledger c WHERE c.practice_id = p.id
     ) l
      LEFT JOIN credit_ledger g ON g.practice_id = p.id AND g.origin = 'founding_grant'
      LEFT JOIN operator_acts a ON a.id = g.operator_act_id
      LEFT JOIN operators o ON o.id = a.operator_id
     CROSS JOIN LATERAL (
           SELECT count(*)::integer AS n FROM feedback fb WHERE fb.practice_id = p.id
     ) f
     WHERE (p.deleted_at IS NOT NULL) = p_deleted
       AND (p_after_at IS NULL OR (p.created_at, p.id) < (p_after_at, p_after_id))
     ORDER BY p.created_at DESC, p.id DESC
     LIMIT p_limit
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_practice_list(boolean, timestamptz, uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_practice_list(boolean, timestamptz, uuid, integer) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_practice_list(boolean, timestamptz, uuid, integer);
DROP INDEX feedback_practice_id;
DROP INDEX credit_ledger_practice_origin;
DROP INDEX practices_created_at_id;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared')) NOT VALID;
