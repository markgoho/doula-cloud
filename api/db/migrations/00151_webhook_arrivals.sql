-- +goose Up
-- ADR-0062, #2473: one row per request to each of doula-api's four
-- webhook routes, refused ones included, written by app_runtime after the
-- handler returns. No payload is stored. Platform-level like
-- stripe_webhook_events, so no RLS. app_runtime may only insert; the
-- operator's reads (#1996, #2487) are ops_runtime definer functions.
CREATE TABLE webhook_arrivals (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    route text NOT NULL CHECK (route IN ('stripe_platform', 'stripe_connect', 'stripe_account', 'mailgun')),
    outcome text NOT NULL CHECK (outcome IN (
        'accepted', 'dropped', 'body_refused', 'signature_refused', 'livemode_refused', 'refused', 'failed'
    )),
    event_id text CHECK (char_length(event_id) <= 255),
    event_type text CHECK (char_length(event_type) <= 255),
    arrived_at timestamptz NOT NULL
);

CREATE INDEX webhook_arrivals_route_arrived_at_idx ON webhook_arrivals (route, arrived_at DESC);
CREATE INDEX webhook_arrivals_event_type_arrived_at_idx ON webhook_arrivals (event_type, arrived_at DESC);
CREATE INDEX webhook_arrivals_arrived_at_idx ON webhook_arrivals (arrived_at);

GRANT INSERT ON webhook_arrivals TO app_runtime;

-- The 90-day sweep, in sweep_expired_idempotency_keys's (00130) shape:
-- behind the drain's worker door, so app_runtime holds no DELETE.
-- +goose StatementBegin
CREATE FUNCTION sweep_webhook_arrivals(p_arrived_before timestamptz)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    n bigint;
BEGIN
    IF current_setting('app.notification_worker_trusted', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'webhook arrivals are deleted only by the scheduled sweep';
    END IF;

    DELETE FROM webhook_arrivals WHERE arrived_at < p_arrived_before;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION sweep_webhook_arrivals(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION sweep_webhook_arrivals(timestamptz) TO app_runtime;

-- +goose Down
DROP FUNCTION sweep_webhook_arrivals(timestamptz);
DROP TABLE webhook_arrivals;
