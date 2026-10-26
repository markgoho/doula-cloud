-- +goose Up
-- #1987: an idempotency_keys row is deleted, twice over. Until now nothing
-- deleted one: idempotency.ttl only made an old row read as not found, so
-- a stored response -- a portal Message's own text under her client_id,
-- or a Staff response that carries a Client's fields -- stayed for good,
-- and a Client's Erasure did not reach the table at all.
--
-- Two doors, both SECURITY DEFINER in the erase_client_feedback (00121)
-- shape, rather than a DELETE grant and policies. Both deletes reach rows
-- no single caller's policy branch can see: the sweep every Practice's,
-- the Erasure every Staff member's at her Practice and every row of hers.
-- A plain DELETE would need a SELECT policy to match (a DELETE's WHERE
-- reads the row), and that would let whoever holds the door read every
-- stored response body. A function deletes and returns a count, and lets
-- nobody read anything.

-- What the sweep's WHERE clause reads.
CREATE INDEX idempotency_keys_created_at ON idempotency_keys (created_at);

-- The sweep: every row created before p_expired_before, at every
-- Practice. The caller passes the cutoff (clock.Now minus idempotency.ttl)
-- so the TTL lives in one place, in Go, and a simulation run that moves
-- the clock moves the sweep with it.
--
-- Admitted only behind the trusted-worker door the outbox drain sets
-- (outbox.NotificationDoor) -- the one scheduled tick (ADR-0010 as
-- amended by #481). A Staff or a Client request never sets it, so neither
-- can empty another Practice's keys.
-- +goose StatementBegin
CREATE FUNCTION sweep_expired_idempotency_keys(p_expired_before timestamptz)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    n bigint;
BEGIN
    IF current_setting('app.notification_worker_trusted', true) IS DISTINCT FROM 'true' THEN
        RAISE EXCEPTION 'expired idempotency keys are deleted only by the scheduled sweep';
    END IF;

    DELETE FROM idempotency_keys WHERE created_at < p_expired_before;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION sweep_expired_idempotency_keys(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION sweep_expired_idempotency_keys(timestamptz) TO app_runtime;

-- The Erasure: every row of hers, and every Staff row at her Practice.
--
-- A Staff row names no Client. idempotency.Wrap is one decorator over
-- every Staff write, it never learns which Client a request was about,
-- and the body it stores is whatever the handler wrote -- a Client's
-- name inside an Engagement, an Invoice, a list. The request path is no
-- better a guide: a write keyed by a Contract or an Invoice id names no
-- Client either. The one fact a Staff row carries that reliably bounds
-- where her data can be is its Practice, so every Staff row there goes.
-- The cost is that a Staff retry in flight at the moment of an Erasure
-- runs again rather than replaying, inside a 48-hour window and only at
-- an act an Owner takes rarely.
--
-- Another Client's portal row at the same Practice stays: the portal
-- shows a Client only her own records, so it cannot hold her.
--
-- Admitted only when she is a Client of the Practice in this transaction
-- and is already erased -- the precondition erase_client_feedback states,
-- for the same reason: client.Erase calls this after redactRecord (or
-- redact_absorbed_client) has stamped erased_at, so the precondition is a
-- row this transaction wrote rather than a claim the caller makes.
-- +goose StatementBegin
CREATE FUNCTION erase_client_idempotency_keys(p_client_id uuid)
RETURNS bigint
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_practice_id uuid;
    n bigint;
BEGIN
    SELECT practice_id INTO v_practice_id FROM clients
     WHERE id = p_client_id
       AND practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
       AND erased_at IS NOT NULL;
    IF v_practice_id IS NULL THEN
        RAISE EXCEPTION 'a Client''s idempotency keys are deleted only by the Erasure of that Client';
    END IF;

    DELETE FROM idempotency_keys
     WHERE client_id = p_client_id
        OR (practice_id = v_practice_id AND staff_id IS NOT NULL);
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION erase_client_idempotency_keys(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION erase_client_idempotency_keys(uuid) TO app_runtime;

-- +goose Down
REVOKE EXECUTE ON FUNCTION erase_client_idempotency_keys(uuid) FROM app_runtime;
DROP FUNCTION erase_client_idempotency_keys(uuid);
REVOKE EXECUTE ON FUNCTION sweep_expired_idempotency_keys(timestamptz) FROM app_runtime;
DROP FUNCTION sweep_expired_idempotency_keys(timestamptz);
DROP INDEX idempotency_keys_created_at;
