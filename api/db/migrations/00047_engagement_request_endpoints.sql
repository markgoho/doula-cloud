-- +goose Up
-- #398's RLS half of the Engagement Request endpoints (ADR-0017). 00042
-- gave engagement_requests one blanket practice_visibility policy
-- covering every command. That is still correct for SELECT and UPDATE,
-- but INSERT needs the same contractor refusal clients_insert already
-- enforces: "a contractor originates nothing" (ADR-0017's write table --
-- a contractor Doula requests no Engagement at a Practice she contracts
-- for). Approve, refuse and withdraw stay endpoint-enforced only,
-- mirroring engagement_offers' accept/decline/withdraw, which rely on
-- the same single blanket policy shape without a role split in RLS.
DROP POLICY engagement_requests_practice_visibility ON engagement_requests;

-- +goose StatementBegin
CREATE POLICY engagement_requests_select ON engagement_requests
    FOR SELECT
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- Same WITH CHECK shape as clients_insert (00042): the practice_id being
-- written must match the caller's session, and her own Membership there
-- must not be a contractor Doula's -- an Owner or Admin who happens to
-- hold a contractor employment_type (ADR-0017's solo Practice) stays
-- admitted, since the check is role-gated, not bare employment_type.
-- +goose StatementBegin
CREATE POLICY engagement_requests_insert ON engagement_requests
    FOR INSERT
    WITH CHECK (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND (
                  pm.employment_type <> 'contractor'
                  OR pm.roles && ARRAY['owner', 'admin']::practice_role[]
              )
        )
    );
-- +goose StatementEnd

-- approve/refuse/withdraw are all UPDATEs; who may call each is enforced
-- at the endpoint (staffauth.RequireOwnerOrAdmin, or "the requester
-- herself" for withdraw), the same split engagement_offers draws between
-- its blanket RLS policy and AcceptHandler/DeclineHandler/WithdrawHandler.
-- +goose StatementBegin
CREATE POLICY engagement_requests_update ON engagement_requests
    FOR UPDATE
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- +goose Down
DROP POLICY engagement_requests_update ON engagement_requests;
DROP POLICY engagement_requests_insert ON engagement_requests;
DROP POLICY engagement_requests_select ON engagement_requests;

-- +goose StatementBegin
CREATE POLICY engagement_requests_practice_visibility ON engagement_requests
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd
