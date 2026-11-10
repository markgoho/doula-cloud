-- +goose Up
-- A Client record's standing, named once (clientstanding.Standing in Go).
--
-- Until this migration nothing named it. "Is this Client live?" was asked
-- about fifteen times four different ways -- erased_at IS NULL AND
-- merged_into IS NULL, merged_into IS NULL alone, erased_at IS NULL alone,
-- and not at all -- and nothing in the schema related erased_at to
-- anything. A record is in exactly one of four standings:
--
--   live                -- neither merged nor erased: the only standing a
--                          write on or under the record is admitted in.
--   merged              -- a tombstone (ADR-0040): merged_into names the
--                          record that kept everything that follows her.
--   erased              -- redacted in place on her request (ADR-0027).
--   merged_then_erased  -- a tombstone whose survivor was erased, and so
--                          reached by that erasure (redact_absorbed_client).
--
-- There is no erased_then_merged: an erased record is never written again
-- (clients_update below), so it can never gain merged_into.
--
-- The standing is a STORED generated column rather than a Go function
-- plus a SQL expression kept beside it. Nearly every reader is SQL -- the
-- Clients list, its counts, the collision and match searches, the
-- Practice session, RLS policies and SECURITY DEFINER functions -- and a
-- generated column is the one derivation all of them read and none of
-- them can restate differently. Go names the four values and parses them;
-- it never derives them.

-- =====================================================================
-- The standing itself
-- =====================================================================

CREATE TYPE client_standing AS ENUM ('live', 'merged', 'erased', 'merged_then_erased');

-- Never NULL without saying NOT NULL: the CASE has an ELSE, so every row
-- computes one of the four values.
ALTER TABLE clients ADD COLUMN standing client_standing GENERATED ALWAYS AS (
    CASE
        WHEN merged_into IS NOT NULL AND erased_at IS NOT NULL THEN 'merged_then_erased'::client_standing
        WHEN merged_into IS NOT NULL THEN 'merged'::client_standing
        WHEN erased_at IS NOT NULL THEN 'erased'::client_standing
        ELSE 'live'::client_standing
    END
) STORED;

-- =====================================================================
-- Who erased the record, beside when
-- =====================================================================

-- erased_at said when and nothing said who. The 'erased' activity row
-- names the actor, but a Practice asking "how did this record come to
-- be?" reads the row first, and merged_at/merged_by_staff_id (00112)
-- already answer the same question for a merge in plaintext columns. The
-- kind mirrors activity's own actor_kind, because an erasure has two
-- authors: an Owner (EraseHandler) and DoulaCloud itself, finishing a
-- Practice's Deletion (ADR-0031), which names no Staff member.
ALTER TABLE clients ADD COLUMN erased_by_kind activity_actor_kind;
ALTER TABLE clients ADD COLUMN erased_by_staff_id uuid REFERENCES staff (id);

-- An erased record says who erased it, and a record that is not erased
-- carries no author of an erasure. NOT VALID: a record erased before
-- this migration has erased_at and no author column, and the 'erased'
-- activity row is still its answer. Nothing can update such a row again
-- (clients_update below admits only a live record), so the constraint
-- holds every row an erasure writes from here on. Every term is written
-- so it is never NULL, because a CHECK passes on NULL: an erasure with no
-- author at all must fail, not slip through as unknown.
ALTER TABLE clients ADD CONSTRAINT clients_erasure_is_attributed CHECK (
    (erased_at IS NULL) = (erased_by_kind IS NULL)
    AND (erased_by_kind IS NULL OR erased_by_kind IN ('staff', 'system'))
    AND (erased_by_kind IS DISTINCT FROM 'staff') = (erased_by_staff_id IS NULL)
) NOT VALID;

-- =====================================================================
-- The merge's columns, tied together on the row
-- =====================================================================

-- clients_merge_is_dated (00112) ties merged_at to merged_into. The two
-- facts left to the merge endpoint and to clients_update's WITH CHECK are
-- properties of the row alone, so they belong to the row.
ALTER TABLE clients ADD CONSTRAINT clients_merge_attribution_needs_a_merge CHECK (
    merged_into IS NOT NULL OR merged_by_staff_id IS NULL
);
ALTER TABLE clients ADD CONSTRAINT clients_never_merged_into_itself CHECK (
    merged_into <> id
);

-- =====================================================================
-- clients_update: only a live record is ever written
-- =====================================================================

-- 00080's USING carried merged_into IS NULL, so a tombstone was terminal
-- in the schema, while an erased record was terminal only because
-- EraseHandler's and EditHandler's own reads said so. USING now asks the
-- standing. It is a pre-image check, so the write that tombstones a row
-- or erases it still passes -- the row was live before it -- and every
-- write after either is refused. The two SECURITY DEFINER doors below are
-- the only writers of a record that is not live, and each answers to its
-- own precondition. WITH CHECK is unchanged.
DROP POLICY clients_update ON clients;
-- +goose StatementBegin
CREATE POLICY clients_update ON clients
    FOR UPDATE
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND standing = 'live'
    )
    WITH CHECK (
        merged_into IS NULL
        OR merged_into <> id
    );
-- +goose StatementEnd

-- =====================================================================
-- redact_client_record: the one definition of the redaction
-- =====================================================================

-- client.redactRecord wrote one column list with the Go clock's instant,
-- and redact_absorbed_client (00112) wrote a second, hand-kept copy of it
-- with now(), so under a moved clock a survivor and the record merged
-- into it were erased at two different instants. This is the column list,
-- once, and it takes the instant and the author from its caller.
--
-- SECURITY INVOKER: called by app_runtime directly (client.redactRecord),
-- it runs under clients_update, which admits only a live record at the
-- caller's own Practice, so it can write nothing a plain UPDATE could
-- not. Called from inside redact_absorbed_client it runs as that
-- function's owner, which is how a tombstone is reached at all.
--
-- 'Erased Client' is client.ErasedGivenName; client/merge_moves_test.go
-- asserts the two still agree. It returns how many rows it redacted, so
-- a caller refuses a silent zero.
-- +goose StatementBegin
CREATE FUNCTION redact_client_record(p_client uuid, p_at timestamptz, p_by_kind activity_actor_kind, p_by_staff uuid)
RETURNS integer
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = public
AS $$
DECLARE
    redacted integer;
BEGIN
    UPDATE clients SET
        given_name = 'Erased Client', family_name = NULL, preferred_name = NULL,
        email = NULL, phone = NULL,
        address_line1 = NULL, address_line2 = NULL, address_locality = NULL,
        address_region = NULL, address_postal_code = NULL, date_of_birth = NULL,
        field_values = '{}'::jsonb,
        erased_at = p_at, erased_by_kind = p_by_kind, erased_by_staff_id = p_by_staff
     WHERE id = p_client
       AND erased_at IS NULL;
    GET DIAGNOSTICS redacted = ROW_COUNT;
    RETURN redacted;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION redact_client_record(uuid, timestamptz, activity_actor_kind, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION redact_client_record(uuid, timestamptz, activity_actor_kind, uuid) TO app_runtime;

-- =====================================================================
-- redact_absorbed_client: the survivor's instant and author
-- =====================================================================

-- Restated in full from 00112. The precondition is unchanged and asked of
-- the standing: the record is merged and not yet erased, and the record
-- it was merged into is erased (or, one hop further out a chain, merged
-- and erased itself). It still takes only the record: the instant and the
-- author are the survivor's own, read from the row the erasure just
-- wrote, so a caller still chooses neither value and the two records
-- carry one erasure's instant and author rather than two.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION redact_absorbed_client(p_absorbed uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    survivor_erased_at timestamptz;
    survivor_by_kind activity_actor_kind;
    survivor_by_staff uuid;
BEGIN
    SELECT s.erased_at, s.erased_by_kind, s.erased_by_staff_id
      INTO survivor_erased_at, survivor_by_kind, survivor_by_staff
      FROM clients a
      JOIN clients s ON s.id = a.merged_into
     WHERE a.id = p_absorbed
       AND a.standing = 'merged'
       AND s.standing IN ('erased', 'merged_then_erased')
       AND a.practice_id = s.practice_id
       AND a.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'an absorbed record is redacted only once, and only after the record it was merged into has been erased';
    END IF;

    PERFORM redact_client_record(p_absorbed, survivor_erased_at, survivor_by_kind, survivor_by_staff);
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION redact_absorbed_client(p_absorbed uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM clients a
          JOIN clients s ON s.id = a.merged_into
         WHERE a.id = p_absorbed
           AND a.erased_at IS NULL
           AND s.erased_at IS NOT NULL
           AND a.practice_id = s.practice_id
           AND a.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
    ) THEN
        RAISE EXCEPTION 'an absorbed record is redacted only once, and only after the record it was merged into has been erased';
    END IF;

    UPDATE clients SET
        given_name = 'Erased Client', family_name = NULL, preferred_name = NULL,
        email = NULL, phone = NULL,
        address_line1 = NULL, address_line2 = NULL, address_locality = NULL,
        address_region = NULL, address_postal_code = NULL, date_of_birth = NULL,
        field_values = '{}'::jsonb, erased_at = now()
     WHERE id = p_absorbed;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION redact_client_record(uuid, timestamptz, activity_actor_kind, uuid) FROM app_runtime;
DROP FUNCTION redact_client_record(uuid, timestamptz, activity_actor_kind, uuid);

DROP POLICY clients_update ON clients;
-- +goose StatementBegin
CREATE POLICY clients_update ON clients
    FOR UPDATE
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND merged_into IS NULL
    )
    WITH CHECK (
        merged_into IS NULL
        OR merged_into <> id
    );
-- +goose StatementEnd

ALTER TABLE clients DROP CONSTRAINT clients_never_merged_into_itself;
ALTER TABLE clients DROP CONSTRAINT clients_merge_attribution_needs_a_merge;
ALTER TABLE clients DROP CONSTRAINT clients_erasure_is_attributed;
ALTER TABLE clients DROP COLUMN erased_by_staff_id;
ALTER TABLE clients DROP COLUMN erased_by_kind;
ALTER TABLE clients DROP COLUMN standing;
DROP TYPE client_standing;
