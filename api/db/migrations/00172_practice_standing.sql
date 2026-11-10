-- +goose Up
-- A Practice's deletion standing, named once (ADR-0031's window made
-- explicit). 00088 stored the window as four facts on practices and
-- every reader derived "pending" from them for itself -- nine readers,
-- each its own predicate, and finalize leaves deletion_requested_at set,
-- so "pending" was really "requested and not deleted", spelled several
-- ways. Three changes:
--
-- 1. The four facts may only be held together in the shapes the three
--    transitions write (practicedeletion.Transitions): all of the
--    request's three facts or none of them, and deleted_at only on top
--    of a request.
-- 2. practices.standing is the one derivation of active, pending
--    deletion and deleted, generated from those facts so no writer can
--    set it apart from them and no reader has to derive it again. Stored
--    rather than computed per reader, because a generated column is the
--    one SQL expression a Go query, a definer function and a policy can
--    all read without restating; and generated rather than written,
--    because a written enum beside the facts would be a fifth fact to
--    keep in step with the other four.
-- 3. Finalize overdue -- pending, with deletion_finalize_at already
--    past -- depends on the instant it is asked at, which a generated
--    column cannot read, so it is derived on read by
--    practice_standing_at. It is the state a finalization that keeps
--    failing (or a dead-lettered finalize row) leaves a Practice in, and
--    ops reads it on the Practice list.

CREATE TYPE practice_standing AS ENUM ('active', 'pending_deletion', 'deleted');

-- A request names who asked and when the window closes, and a restore
-- clears all three together.
ALTER TABLE practices ADD CONSTRAINT practices_deletion_request_whole CHECK (
    (deletion_requested_at IS NULL) = (deletion_requested_by IS NULL)
    AND (deletion_requested_at IS NULL) = (deletion_finalize_at IS NULL)
);

-- A Practice is only ever deleted at the end of a window someone opened.
ALTER TABLE practices ADD CONSTRAINT practices_deleted_after_request CHECK (
    deleted_at IS NULL OR deletion_requested_at IS NOT NULL
);

ALTER TABLE practices ADD COLUMN standing practice_standing NOT NULL GENERATED ALWAYS AS (
    CASE
        WHEN deleted_at IS NOT NULL THEN 'deleted'::practice_standing
        WHEN deletion_requested_at IS NOT NULL THEN 'pending_deletion'::practice_standing
        ELSE 'active'::practice_standing
    END
) STORED;

-- The standing as it reads at p_at: practices.standing, except that a
-- pending deletion whose finalize date has passed reads
-- 'finalize_overdue'. Granted to no role; ops_practice_list below calls
-- it with its owner's rights.
-- +goose StatementBegin
CREATE FUNCTION practice_standing_at(p_standing practice_standing, p_finalize_at timestamptz, p_at timestamptz)
RETURNS text
    LANGUAGE sql IMMUTABLE
    SET search_path = public, pg_temp
AS $$
    SELECT CASE
        WHEN p_standing = 'pending_deletion' AND p_finalize_at <= p_at THEN 'finalize_overdue'
        ELSE p_standing::text
    END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION practice_standing_at(practice_standing, timestamptz, timestamptz) FROM PUBLIC;

-- 00160's legal_notice_owners, restated in full: an Owner of an active
-- Practice, read from the standing.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION legal_notice_owners()
RETURNS TABLE (practice_id uuid, staff_id uuid)
    LANGUAGE sql STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT pm.practice_id, pm.staff_id
      FROM practice_memberships pm
      JOIN practices p ON p.id = pm.practice_id
      JOIN staff s ON s.id = pm.staff_id
     WHERE 'owner' = ANY(pm.roles)
       AND p.standing = 'active'
       AND s.deleted_at IS NULL
$$;
-- +goose StatementEnd

-- 00157's ops_operator_log_names, restated in full: deleted is the
-- standing.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_operator_log_names(p_practice_ids uuid[], p_staff_ids uuid[])
RETURNS TABLE (kind text, id uuid, name text, deleted boolean)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT 'practice', p.id, p.name, p.standing = 'deleted'
      FROM practices p
     WHERE p.id = ANY (p_practice_ids)
    UNION ALL
    SELECT 'staff', s.id, NULLIF(btrim(s.first_name || ' ' || s.last_name), ''), false
      FROM staff s
     WHERE s.id = ANY (p_staff_ids)
$$;
-- +goose StatementEnd

-- 00153's ops_practice_list, restated in full with the standing as it
-- reads at p_now, so a deletion past its date is a state ops sees rather
-- than a date it has to compare. The Deleted list is the deleted
-- standing.
DROP FUNCTION ops_practice_list(boolean, timestamptz, uuid, integer);

-- +goose StatementBegin
CREATE FUNCTION ops_practice_list(p_deleted boolean, p_now timestamptz, p_after_at timestamptz, p_after_id uuid, p_limit integer)
RETURNS TABLE (
    id uuid, name text, signed_up_at timestamptz, owners jsonb, staff_count integer,
    connected boolean, card_payments_status text, payouts_status text,
    requirements_due jsonb, connect_updated_at timestamptz,
    credits_held integer, engagements_started integer,
    grant_credits integer, grant_given_by text, grant_given_at timestamptz,
    standing text, deletion_requested_at timestamptz, deletion_finalize_at timestamptz, deleted_at timestamptz,
    open_feedback integer
)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT p.id, p.name, p.created_at, m.owners, m.staff_count,
           p.stripe_connect_account_id IS NOT NULL, p.stripe_connect_card_payments_status,
           p.stripe_connect_payouts_status, to_jsonb(p.stripe_connect_requirements_due), p.stripe_connect_status_updated_at,
           l.held, l.started, g.quantity, o.name, g.created_at,
           practice_standing_at(p.standing, p.deletion_finalize_at, p_now),
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
     WHERE (p.standing = 'deleted') = p_deleted
       AND (p_after_at IS NULL OR (p.created_at, p.id) < (p_after_at, p_after_id))
     ORDER BY p.created_at DESC, p.id DESC
     LIMIT p_limit
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_practice_list(boolean, timestamptz, timestamptz, uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_practice_list(boolean, timestamptz, timestamptz, uuid, integer) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_practice_list(boolean, timestamptz, timestamptz, uuid, integer);

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

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ops_operator_log_names(p_practice_ids uuid[], p_staff_ids uuid[])
RETURNS TABLE (kind text, id uuid, name text, deleted boolean)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT 'practice', p.id, p.name, p.deleted_at IS NOT NULL
      FROM practices p
     WHERE p.id = ANY (p_practice_ids)
    UNION ALL
    SELECT 'staff', s.id, NULLIF(btrim(s.first_name || ' ' || s.last_name), ''), false
      FROM staff s
     WHERE s.id = ANY (p_staff_ids)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION legal_notice_owners()
RETURNS TABLE (practice_id uuid, staff_id uuid)
    LANGUAGE sql STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT pm.practice_id, pm.staff_id
      FROM practice_memberships pm
      JOIN practices p ON p.id = pm.practice_id
      JOIN staff s ON s.id = pm.staff_id
     WHERE 'owner' = ANY(pm.roles)
       AND p.deleted_at IS NULL
       AND p.deletion_requested_at IS NULL
       AND s.deleted_at IS NULL
$$;
-- +goose StatementEnd

DROP FUNCTION practice_standing_at(practice_standing, timestamptz, timestamptz);
ALTER TABLE practices DROP COLUMN standing;
ALTER TABLE practices DROP CONSTRAINT practices_deleted_after_request;
ALTER TABLE practices DROP CONSTRAINT practices_deletion_request_whole;
DROP TYPE practice_standing;
