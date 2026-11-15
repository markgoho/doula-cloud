-- +goose Up
-- #2952: the instant the Client first read her Sent Contract on the portal,
-- so a Sent row can say whether she has opened it. Nullable: unread, or not
-- yet sent. Written once, by the Client's own first read.

ALTER TABLE contracts ADD COLUMN opened_at timestamptz;

-- The one write a Client makes to a Sent Contract that leaves it Sent
-- (contracts_client_sign admits only sent -> signed). Row security cannot
-- say which column, so the trigger below does: a Client session may set
-- opened_at once and change nothing else.
-- stepped_at is generated, so a BEFORE trigger sees it unset in NEW.
CREATE POLICY contracts_client_open ON contracts
    FOR UPDATE
    USING (
        status = 'sent'
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = contracts.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    )
    WITH CHECK (
        status = 'sent'
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = contracts.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose StatementBegin
CREATE FUNCTION contracts_client_open_only() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public
AS $$
BEGIN
    IF NULLIF(current_setting('app.current_client_id', true), '') IS NOT NULL
       AND OLD.status = 'sent' AND NEW.status = 'sent'
       AND (OLD.opened_at IS NOT NULL
            OR (to_jsonb(NEW) - 'opened_at' - 'stepped_at') IS DISTINCT FROM (to_jsonb(OLD) - 'opened_at' - 'stepped_at')) THEN
        RAISE EXCEPTION 'a Client''s only write to a sent Contract is her first open';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER contracts_client_open_only
    BEFORE UPDATE ON contracts
    FOR EACH ROW EXECUTE FUNCTION contracts_client_open_only();

-- +goose Down
DROP TRIGGER contracts_client_open_only ON contracts;
DROP FUNCTION contracts_client_open_only();
DROP POLICY contracts_client_open ON contracts;
ALTER TABLE contracts DROP COLUMN opened_at;
