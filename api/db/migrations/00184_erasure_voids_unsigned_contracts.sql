-- +goose Up
-- #2990, ADR-0027's amendment of 2026-11-15: erasing a Client voids every
-- Contract of hers that is not signed, in the transaction that redacts her.
--
-- voided_by_erasure says how a Contract came to be voided, so no reader
-- words an erasure as a Staff void. voided_by stays NULL on such a row: who
-- erased her is the 'contract_voided_by_erasure' Activity row's actor and the
-- erased_by columns on her record.
ALTER TABLE contracts ADD COLUMN voided_by_erasure boolean NOT NULL DEFAULT false;

ALTER TABLE contracts ADD CONSTRAINT contracts_voided_by_erasure_is_voided CHECK (
    NOT voided_by_erasure OR status = 'voided'
);

-- A Draft was never sent, so a Draft voided by erasure has no sent_at.
-- Sent and signed Contracts still carry one, and a Draft still has none.
ALTER TABLE contracts DROP CONSTRAINT contracts_sent_at_matches_status;

ALTER TABLE contracts ADD CONSTRAINT contracts_sent_at_matches_status CHECK (
    CASE status
        WHEN 'draft' THEN sent_at IS NULL
        WHEN 'voided' THEN voided_by_erasure OR sent_at IS NOT NULL
        ELSE sent_at IS NOT NULL
    END
);

-- Restated in full from 00171 with the same signature and return type, so
-- its grants and redact_absorbed_client's call survive. Still the one
-- definition of the redaction: the BFF's erasure and the seed both reach
-- the void through it, and a record merged into a survivor is voided at the
-- survivor's own instant.
--
-- The void and its Activity row take p_at and p_by_*, never now(), for the
-- same reason the redaction does. Activity's subject is the Engagement, like
-- every other Contract act.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION redact_client_record(p_client uuid, p_at timestamptz, p_by_kind activity_actor_kind, p_by_staff uuid)
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

    IF redacted = 1 THEN
        WITH voided AS (
            UPDATE contracts c
               SET status = 'voided', voided_at = p_at, voided_by_erasure = true
              FROM engagements e
             WHERE e.id = c.engagement_id
               AND e.client_id = p_client
               AND c.status IN ('draft', 'sent')
            RETURNING e.practice_id, c.engagement_id
        )
        INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
        SELECT practice_id, 'engagement', engagement_id, 'contract_voided_by_erasure', '{}'::jsonb, p_by_kind, p_by_staff, p_at
          FROM voided;
    END IF;
    RETURN redacted;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION redact_client_record(p_client uuid, p_at timestamptz, p_by_kind activity_actor_kind, p_by_staff uuid)
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

ALTER TABLE contracts DROP CONSTRAINT contracts_sent_at_matches_status;
ALTER TABLE contracts ADD CONSTRAINT contracts_sent_at_matches_status CHECK (
    (status = 'draft') = (sent_at IS NULL)
);
ALTER TABLE contracts DROP CONSTRAINT contracts_voided_by_erasure_is_voided;
ALTER TABLE contracts DROP COLUMN voided_by_erasure;
