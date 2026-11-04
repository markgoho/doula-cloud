-- +goose Up
-- #2691: a verified Stripe event of a handled type whose content the
-- handler cannot act on is its own outcome, counted apart from "refused".
ALTER TABLE webhook_arrivals DROP CONSTRAINT webhook_arrivals_outcome_check;
ALTER TABLE webhook_arrivals ADD CONSTRAINT webhook_arrivals_outcome_check CHECK (outcome IN (
    'accepted', 'dropped', 'body_refused', 'signature_refused', 'livemode_refused', 'payload_refused', 'refused', 'failed'
));

DROP FUNCTION ops_webhook_routes(timestamptz);

-- +goose StatementBegin
CREATE FUNCTION ops_webhook_routes(p_since timestamptz)
RETURNS TABLE (route text, last_arrival_at timestamptz, signature_refused integer, livemode_refused integer,
               body_refused integer, payload_refused integer, refused integer, failed integer)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT r.route,
           max(w.arrived_at) FILTER (WHERE w.outcome IN ('accepted', 'dropped')),
           (count(w.id) FILTER (WHERE w.outcome = 'signature_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'livemode_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'body_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'payload_refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'refused' AND w.arrived_at >= p_since))::integer,
           (count(w.id) FILTER (WHERE w.outcome = 'failed' AND w.arrived_at >= p_since))::integer
      FROM (VALUES ('stripe_platform'), ('stripe_connect'), ('stripe_account'), ('mailgun')) AS r (route)
      LEFT JOIN webhook_arrivals w ON w.route = r.route
     GROUP BY r.route
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_webhook_routes(timestamptz);

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

REVOKE EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_webhook_routes(timestamptz) TO ops_runtime;

UPDATE webhook_arrivals SET outcome = 'refused' WHERE outcome = 'payload_refused';
ALTER TABLE webhook_arrivals DROP CONSTRAINT webhook_arrivals_outcome_check;
ALTER TABLE webhook_arrivals ADD CONSTRAINT webhook_arrivals_outcome_check CHECK (outcome IN (
    'accepted', 'dropped', 'body_refused', 'signature_refused', 'livemode_refused', 'refused', 'failed'
));
