-- +goose Up
-- Adds the ESIGN signature fields a Client fills in when signing a Sent
-- Contract (#70): the typed full legal name and attestation checkbox
-- state the request body carries, plus signed_at and signer_ip, which the
-- Go handler derives server-side from the request itself (server clock,
-- caller's remote address) -- never trusted from the request body, since
-- both are the legal record of when and from where the signature was
-- captured. signer_ip is text, not inet: the handler falls back to
-- r.RemoteAddr's raw host:port form if it can't be parsed, and text never
-- rejects that fallback the way inet would.

ALTER TABLE contracts
    ADD COLUMN signer_full_name text,
    ADD COLUMN signer_attestation boolean NOT NULL DEFAULT false,
    ADD COLUMN signed_at timestamptz,
    ADD COLUMN signer_ip text;

-- Client-tier write access for the Sign transition: a Client-portal
-- session may UPDATE only a 'sent' Contract on their own Engagement
-- (USING), and only into the 'signed' status (WITH CHECK) -- the
-- sent -> signed transition is the only write this policy admits, in
-- either direction. Adds to contracts_client_visibility
-- (00017_contracts_client_visibility.sql) and contracts_practice_visibility
-- (00016_contracts.sql) rather than replacing either, the same
-- multiple-permissive-policies shape those two already share; this is the
-- first client-tier write policy on contracts, so
-- TestRLS_ContractsClientUpdateRejected (rls_test.go) -- proving Send-era
-- Contracts staff wrote were unreachable for edit by a Client -- still
-- holds for any update that doesn't land on exactly this transition.
CREATE POLICY contracts_client_sign ON contracts
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
        status = 'signed'
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = contracts.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose Down
DROP POLICY contracts_client_sign ON contracts;
ALTER TABLE contracts
    DROP COLUMN signer_full_name,
    DROP COLUMN signer_attestation,
    DROP COLUMN signed_at,
    DROP COLUMN signer_ip;
