-- +goose Up
-- #1996, ADR-0062: the System section's reads, each a definer function
-- that names no client, so ops_runtime can run it and nothing else. The
-- Practice-facing tables are untouched: ops_runtime still holds no grant
-- on webhook_arrivals, pilot_gate, pilot_allowlist or any outbox.

-- Per webhook route: the last arrival of a request that was verified
-- (accepted or dropped), and how many requests since p_since were
-- refused, by outcome. A refused request proves nothing arrived, so it
-- never moves the last arrival: a forged request must not make a route
-- look alive. A route with no row yet still answers, with no arrival.
-- +goose StatementBegin
CREATE FUNCTION ops_webhook_routes(p_since timestamptz)
RETURNS TABLE (route text, last_arrival_at timestamptz, signature_refused integer, livemode_refused integer,
               body_refused integer, refused integer, failed integer)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT r.route,
           max(w.arrived_at) FILTER (WHERE w.outcome IN ('accepted', 'dropped')),
           (count(w.id) FILTER (WHERE w.outcome = 'signature_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'livemode_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'body_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'failed' AND w.arrived_at >= p_since))::integer
      FROM (VALUES ('stripe_platform'), ('stripe_connect'), ('stripe_account'), ('mailgun')) AS r (route)
      LEFT JOIN webhook_arrivals w ON w.route = r.route
     GROUP BY r.route
$$;
-- +goose StatementEnd

-- The last verified arrival of each event type seen on each route. Only
-- accepted and dropped rows count: a signature_refused row carries an id
-- and type parsed from a body nobody verified.
-- +goose StatementBegin
CREATE FUNCTION ops_webhook_event_types()
RETURNS TABLE (route text, event_type text, last_arrival_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT w.route, w.event_type, max(w.arrived_at)
      FROM webhook_arrivals w
     WHERE w.outcome IN ('accepted', 'dropped') AND w.event_type IS NOT NULL
     GROUP BY w.route, w.event_type
$$;
-- +goose StatementEnd

-- Per outbox: rows pending, rows dead-lettered, and the oldest pending
-- row that is due at p_now (when it was due; NULL when none is). A
-- pending row not yet due is waiting by design, as a welcome email
-- fourteen days out is. Counts only: no job content, no recipient.
-- +goose StatementBegin
CREATE FUNCTION ops_outbox_status(p_now timestamptz)
RETURNS TABLE (outbox text, pending integer, dead_lettered integer, oldest_due_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT s.outbox, s.pending::integer, s.dead_lettered::integer, s.oldest_due_at FROM (
        SELECT 'client_erasure_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM client_erasure_outbox
        UNION ALL
        SELECT 'connect_nudge_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM connect_nudge_outbox
        UNION ALL
        SELECT 'coverage_gap_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM coverage_gap_outbox
        UNION ALL
        SELECT 'engagement_offer_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_offer_outbox
        UNION ALL
        SELECT 'engagement_request_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_request_outbox
        UNION ALL
        SELECT 'feedback_issue_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM feedback_issue_outbox
        UNION ALL
        SELECT 'legal_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM legal_notice_outbox
        UNION ALL
        SELECT 'low_credit_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM low_credit_outbox
        UNION ALL
        SELECT 'payment_received_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payment_received_outbox
        UNION ALL
        SELECT 'payout_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payout_outbox
        UNION ALL
        SELECT 'portal_address_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_address_change_outbox
        UNION ALL
        SELECT 'portal_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_invite_outbox
        UNION ALL
        SELECT 'portal_magic_link_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_magic_link_outbox
        UNION ALL
        SELECT 'practice_deletion_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM practice_deletion_outbox
        UNION ALL
        SELECT 'session_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM session_notice_outbox
        UNION ALL
        SELECT 'site_build_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(created_at) FILTER (WHERE status = 'pending' AND created_at <= p_now) AS oldest_due_at
          FROM site_build_outbox
        UNION ALL
        SELECT 'staff_email_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_email_change_outbox
        UNION ALL
        SELECT 'staff_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_invite_outbox
        UNION ALL
        SELECT 'staff_mfa_recovery_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_mfa_recovery_outbox
        UNION ALL
        SELECT 'staff_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_notice_outbox
        UNION ALL
        SELECT 'staff_token_mail_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_token_mail_outbox
        UNION ALL
        SELECT 'welcome_credits_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM welcome_credits_outbox
    ) s
     ORDER BY s.outbox
$$;
-- +goose StatementEnd

-- When the pilot gate opened and the operator it is keyed to, if any.
-- NULLs in both while it is closed; NULL name for a gate opened before
-- openings were keyed to an act.
-- +goose StatementBegin
CREATE FUNCTION ops_pilot_gate()
RETURNS TABLE (opened_at timestamptz, opened_by text)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT g.opened_at, o.name
      FROM pilot_gate g
      LEFT JOIN operator_acts a ON a.id = g.operator_act_id
      LEFT JOIN operators o ON o.id = a.operator_id
$$;
-- +goose StatementEnd

-- The pilot allowlist, newest first, p_limit rows, each with the
-- operator who added it (NULL for an address added before additions were
-- keyed to an act) and the full count on every row.
-- +goose StatementBegin
CREATE FUNCTION ops_pilot_allowlist(p_limit integer)
RETURNS TABLE (address text, added_at timestamptz, added_by text, total bigint)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT l.address, l.added_at, o.name, count(*) OVER ()
      FROM pilot_allowlist l
      LEFT JOIN operator_acts a ON a.id = l.operator_act_id
      LEFT JOIN operators o ON o.id = a.operator_id
     ORDER BY l.added_at DESC, l.address
     LIMIT p_limit
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_webhook_event_types() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_outbox_status(timestamptz) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_pilot_gate() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_pilot_allowlist(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_webhook_event_types() TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_outbox_status(timestamptz) TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_pilot_gate() TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_pilot_allowlist(integer) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_pilot_allowlist(integer);
DROP FUNCTION ops_pilot_gate();
DROP FUNCTION ops_outbox_status(timestamptz);
DROP FUNCTION ops_webhook_event_types();
DROP FUNCTION ops_webhook_routes(timestamptz);
