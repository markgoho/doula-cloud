-- +goose Up
-- #478: the Client portal's own "Your visits" read. Second RLS policy on
-- visits (00007_visit.sql), giving the Client-portal population a
-- SELECT-only view of the Visits on her own Engagement. Mirrors
-- contracts_client_visibility's EXISTS-against-engagements shape
-- (00017_contracts_client_visibility.sql), keyed on app.current_client_id
-- -- the Client-portal session variable clientauth.Middleware sets --
-- rather than app.current_practice_id, which a portal transaction never
-- sets at all.
--
-- Postgres OR's multiple permissive policies on the same table together,
-- so this adds to visits_practice_visibility rather than replacing it.
--
-- No `scheduled_at IS NOT NULL` clause here, deliberately, and it is not
-- an omission to "fix" later. 00017 put its `status IN (...)` test in the
-- policy because a Draft Contract is a confidentiality boundary: a Client
-- must not reach one whatever the Go layer does. An unscheduled Visit is
-- not that. It is a row whose only instant is `created_at` -- when a
-- Doula typed it, not when anyone visited -- so showing it would tell her
-- a visit happened that may not have. That is a truthfulness rule about
-- one screen, not a tenancy rule about the table, so it lives in the
-- handler's own WHERE (portal/visits.go) where a future Client-facing
-- reader with a different question can decide it again.
CREATE POLICY visits_client_visibility ON visits
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = visits.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- A second policy, letting a Client resolve the name of a Staff member
-- named on a Visit of her own after that person has left the Practice,
-- was written here and taken back out. It belongs to a follow-up, and
-- the reason is recorded so nobody adds it back without knowing what
-- happens.
--
-- staff_visible_to_own_client_portal_engagements (00009) reaches a Staff
-- name only through a live practice_memberships row, which removing a
-- Membership deletes -- so a departed Doula's name is not resolvable to
-- her Client, and portal/visits.go's LEFT JOIN prints "Your practice"
-- for that row rather than losing the Visit. The obvious repair is a
-- permissive `FOR SELECT` policy on `staff` reached through her own
-- Visits. Adding one makes `POST
-- /api/practices/{id}/engagements/{id}/offers` never return: reproduced
-- on `offer.e2e.ts` three runs out of three, in CI and locally, and it
-- goes green the moment the policy is dropped and red again when it is
-- restored, with every Go package test passing either way. A permissive
-- policy cannot narrow what another one already allows, so the failure
-- is something else -- and shipping a change that hangs a Practice's own
-- screen to improve a name on the Client's is the wrong trade whatever
-- the cause turns out to be.

-- The portal read written as an index: `WHERE engagement_id = $1 AND
-- scheduled_at IS NOT NULL ORDER BY scheduled_at DESC, id DESC`. The
-- partial predicate is the query's own IS NOT NULL predicate, the leading
-- column is its equality filter, and the remaining two are its ORDER BY
-- (scanned backwards, which needs no DESC in the key), so one Client's
-- page comes out of a range scan already sorted. `id` is in the key
-- because the cursor compares the pair (scheduled_at, id).
--
-- visits_scheduled_at_idx (00091) cannot serve this: it leads on
-- scheduled_at for #263's Practice-wide window, so an Engagement-scoped
-- read through it scans every scheduled Visit in the database.
CREATE INDEX visits_engagement_scheduled_at_idx ON visits (engagement_id, scheduled_at, id)
  WHERE scheduled_at IS NOT NULL;

-- +goose Down
DROP INDEX visits_engagement_scheduled_at_idx;
DROP POLICY visits_client_visibility ON visits;
