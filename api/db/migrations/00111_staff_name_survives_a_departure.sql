-- +goose Up
-- #1077: a Client keeps the name of the Doula who worked a past Visit,
-- and the name of the Doula who sent a Message, after that Doula has
-- left the Practice.
--
-- The gap. staff_visible_to_own_client_portal_engagements (00009)
-- reaches a Staff row only through a live practice_memberships row, and
-- removing a Membership deletes that row. So the moment a Doula leaves,
-- every Client at the Practice loses her name -- on Visits she already
-- worked and Messages she already sent. portal/visits.go's LEFT JOIN
-- keeps the Visit and prints "Your practice" for the name;
-- message/list.go's does the same for the sender. CONTEXT.md's Visit
-- entry settles that the name is the requirement ("a Client sees the
-- visits on her own Engagement, past and scheduled, with who is
-- coming"), and CLAUDE.md's audit-trail expectation says the same about
-- a past Visit: "Maya came on 18 August" is exactly the kind of fact a
-- person must still be able to answer for.
--
-- ---------------------------------------------------------------------
-- Why this is a function and not a plain EXISTS, which is the whole
-- point of this migration and the thing to read before adding any
-- further policy to `staff`.
--
-- 00105 tried the obvious repair -- a second permissive FOR SELECT
-- policy on staff whose USING is an EXISTS against the reader's own
-- Visits -- and recorded that it makes POST
-- /api/practices/{id}/engagements/{id}/offers never return. That is
-- real, and neither half of it is about Offers.
--
-- First half: the RLS rewriter *inlines* a policy's subquery into every
-- query that reads the table. An EXISTS over visits JOIN engagements
-- drags in those two tables' own policies, which drag in
-- client_portal_users and clients, which drag in theirs. The Engagement
-- screen's Activity ledger read joins staff three times, so its plan
-- goes from 171 lines to 301 and its estimated cost crosses
-- jit_above_cost (100000) and jit_optimize_above_cost (500000).
-- Postgres then LLVM-compiles it. Measured on a freshly migrated
-- database holding the e2e suite's own handful of rows:
--
--     JIT: Functions: 1828
--          Timing: Optimization 2577 ms, Emission 3997 ms, Total 6675 ms
--     Execution Time: 6686 ms
--
-- against 1.7 ms and no JIT section at all for the same query without
-- the policy. Every node in the plan tree reports under 40 ms, so the
-- whole of it is compilation rather than row work -- which is why the
-- Go package tests, each running a statement or two per connection,
-- never showed it, and why `SET jit = off` collapses it to 2.2 ms.
-- Guarding the EXISTS on app.current_client_id being set does not help:
-- a guarded inline policy measured 7059 ms, because the plan is built
-- and compiled before any guard is evaluated.
--
-- Second half, and the reason a slow *read* stopped a *write*:
-- staffauth.Middleware stamps `UPDATE staff SET last_practice_id = ...,
-- last_active_at = ...` on the caller's own row at the start of every
-- request, so every concurrent request from one Staff member queues on
-- that single row's lock. The Engagement screen fires seven reads at
-- mount; Send Offer arrives while the ledger's transaction still holds
-- the row. pg_stat_activity during the stall has the Offer's backend in
-- wait_event_type = Lock, wait_event = transactionid, with
-- pg_blocking_pids naming the ledger's backend, and the Offer's 201
-- lands 20 ms after the ledger's 200. Driving POST .../offers directly,
-- with no ledger read in flight, returns in ~50 ms with the policy in
-- place -- the write was never the slow thing.
--
-- A SECURITY DEFINER function is what breaks the first half, and the
-- second half never fires without it. The rewriter cannot inline a
-- function body, so the policy stays one opaque call and no query that
-- reads staff grows a plan: with this migration applied, that same
-- ledger read is 171 plan lines, cost 77044, 1.3 ms, no JIT --
-- indistinguishable from a database without the policy. It is the same
-- move current_staff_id() (00003) makes for the same kind of reason.
--
-- What the function is allowed to see, said plainly. SECURITY DEFINER
-- means it runs as the migration role, which owns these tables and so
-- bypasses their RLS. Its only filter is app.current_client_id, a
-- session variable clientauth.Middleware sets and staffauth.Middleware
-- never does, and all it ever returns is a boolean about one Staff id.
-- The IS NOT NULL guard lives in the policy rather than in the function
-- so a Staff session never calls it at all: current_setting costs 1 and
-- a SQL function costs 100, so Postgres evaluates the guard first.
--
-- Each EXISTS is driven from engagements (engagements_client_idx, on
-- client_id) and joined outward, never from visits.staff_id or
-- messages.sender_id, neither of which is indexed; the two joins land on
-- visits_engagement_scheduled_at_idx (00105) and
-- messages_engagement_created_at_index (00066).
-- +goose StatementBegin
CREATE FUNCTION client_portal_sees_staff(target_staff_id uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM engagements e
            JOIN visits v ON v.engagement_id = e.id
            WHERE e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
              AND v.staff_id = target_staff_id
        ) OR EXISTS (
            SELECT 1 FROM engagements e
            JOIN messages m ON m.engagement_id = e.id
            WHERE e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
              AND m.sender_type = 'staff'
              AND m.sender_id = target_staff_id
        )
    $$;
-- +goose StatementEnd

-- Postgres ORs permissive policies together, so this adds to 00009's
-- live-Membership reach rather than replacing it: a Client still resolves
-- every Staff member at her Practice while they are there, and keeps the
-- ones who worked with her after they leave.
--
-- deleted_at IS NULL is not a tidiness clause. ADR-0033's redaction
-- overwrites staff.name with "Deleted Staff Member" (00100/00101), so a
-- Doula who deleted her own login has no name left for any policy to
-- reach -- and admitting her row here would put that internal string in
-- front of a Client, which is worse than the "Your practice" fallback
-- she reads today. Excluding the row keeps the LEFT JOIN's fallback for
-- exactly that case and changes nothing else.
CREATE POLICY staff_visible_to_own_client_portal_history ON staff
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_client_id', true), '') IS NOT NULL
        AND staff.deleted_at IS NULL
        AND client_portal_sees_staff(staff.id)
    );

-- +goose Down
DROP POLICY staff_visible_to_own_client_portal_history ON staff;
DROP FUNCTION client_portal_sees_staff(uuid);
