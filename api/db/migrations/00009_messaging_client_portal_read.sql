-- +goose Up
-- Closes an RLS gap #59 (Client-portal message thread) surfaced: neither
-- clients nor staff had a SELECT policy reachable from an
-- app.current_client_id-only context, so listMessages' sender-name JOINs
-- (00008_messaging.sql) returned no visible row for either side when run
-- through a Client-portal transaction -- a Client couldn't resolve their
-- own name after sending, and couldn't resolve a Staff sender's name when
-- reading the thread. Staff already had the mirror image (staff_practice_visibility
-- lets one Staff member see another's row; clients_select already lets Staff
-- see a Client tied to their Practice) -- #58's tests never caught this
-- because they only ever exercised Staff-authored messages read by Staff.

-- A Client may see their own clients row. Scoped to id = app.current_client_id
-- directly (clients has no practice_id column), unlike clients_select's
-- EXISTS-through-engagements shape -- a self-lookup needs no join.
CREATE POLICY clients_self_visibility ON clients
    FOR SELECT
    USING (id = NULLIF(current_setting('app.current_client_id', true), '')::uuid);

-- practice_memberships has no policy reachable from a Client-portal
-- context either (both its existing policies require app.current_practice_id
-- or app.current_client_id to be unset -- see 00003/00006), and the Staff
-- policy just below queries it inside an EXISTS, which RLS enforces even
-- there. Without this, that EXISTS always sees zero rows and the Staff
-- policy below is unreachable in practice regardless of its own condition.
CREATE POLICY practice_memberships_visible_to_own_client_portal_engagements ON practice_memberships
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.practice_id = practice_memberships.practice_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- A Client may see the Staff members of their own Engagement's Practice --
-- the same "any Staff member with access to the Practice" population
-- staffauth.Middleware and messages_practice_visibility already treat as
-- one undifferentiated group, so a Client resolving a message sender's
-- name isn't restricted to whichever Staff member happens to have sent
-- the most recent one.
CREATE POLICY staff_visible_to_own_client_portal_engagements ON staff
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM practice_memberships pm
            JOIN engagements e ON e.practice_id = pm.practice_id
            WHERE pm.staff_id = staff.id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose Down
DROP POLICY staff_visible_to_own_client_portal_engagements ON staff;
DROP POLICY practice_memberships_visible_to_own_client_portal_engagements ON practice_memberships;
DROP POLICY clients_self_visibility ON clients;
