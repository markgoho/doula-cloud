-- +goose Up
-- The self-edit half of #415's work state (#437).
--
-- 00043 gave a Staff member a work state and recorded the assertion she
-- made at onboarding, and there it stopped: nothing anywhere could
-- change the value afterwards. A doula who moves from New York to New
-- Jersey had no way to say so, and from that day her Practice's sales
-- tax was quietly wrong -- nothing errored, and the apportionment
-- 00043 exists to compute simply used a stale fact.
--
-- The gap is not only that no route existed. Nothing in this schema
-- *permitted* the write either, and that is what this migration fixes:
--
--   * staff has no UPDATE policy that reaches outside a Practice
--     context. staff_self_visibility (00002, redefined 00006) is FOR
--     SELECT only, and staff_practice_visibility needs
--     app.current_practice_id, which is unset on a per-person route.
--   * staff_work_state_events' only policy (00043) admits a row through
--     an EXISTS over practice_memberships at the current Practice, so
--     its INSERT fails in that same pre-Practice window.
--
-- Both are fixed the same way staff_self_visibility fixed the read: a
-- second policy, scoped to the window before a Practice is chosen.
-- Postgres ORs policies together, so the practice-tier rules are
-- untouched and the accept-invite path (which writes its first-assertion
-- row *with* a Practice context set) keeps working unchanged.
--
-- Why the write is per-person rather than per-Practice at all: a work
-- state is a fact about a person, not about a Membership -- 00043's own
-- reasoning. A contractor doula at three Practices has one work state,
-- so a screen scoped to one of them would show a global value under a
-- Practice heading and invite her to think she was setting it there.
-- The route is /account and it sets no app.current_practice_id, which
-- is exactly why these policies have to exist.
-- ---------------------------------------------------------------------

-- Only the person herself may write where she works. #415 chose that
-- over letting an Owner correct it -- an Owner asserting where someone
-- else works is the thing an Offer's deliberately thin copy exists to
-- avoid -- and widening later is a one-line change while narrowing after
-- the fact is not.
--
-- The identity_uid match is the whole enforcement: there is no staffId
-- to pass, so an Owner or an Admin has no way to even name another
-- person's row. WITH CHECK repeats the USING expression rather than
-- being left to default, so the policy also refuses a row that would
-- hand itself to a different identity on the way out.
--
-- Row-level, not column-level: this permits her to update any column of
-- her own staff row, and the only handler that reaches it writes
-- work_state and work_state_reported_at. Stated rather than implied, so
-- a future route that writes staff through this window knows it is
-- inheriting a grant rather than being given one.
CREATE POLICY staff_self_update ON staff
    FOR UPDATE
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    )
    WITH CHECK (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    );

-- The audit row for a change, written in the same pre-Practice window as
-- the UPDATE above and in the same transaction.
--
-- Tighter than 00043's practice-tier policy on the way in, and looser on
-- the way out, which is why USING and WITH CHECK are spelled out
-- separately rather than one defaulting to the other.
--
-- WITH CHECK -- what she may write -- demands that actor_staff_id be the
-- caller as well as staff_id. 00043 left the actor unchecked because the
-- row it governs is written by a handler that has just created the
-- person; on this path the actor and the subject are the same person by
-- construction, so the boundary that can enforce it does.
--
-- USING -- what she may read -- asks only that the row be about her. A
-- row naming someone else as the actor is precisely the signal 00043
-- kept the column for, and the person it was written about is the last
-- one who should be unable to see it.
--
-- FOR ALL rather than FOR INSERT so she can read her own history at all:
-- the table has no reader yet, and the first one will be per-person
-- before it is per-Practice. The grants (00043) are SELECT and INSERT
-- only, so UPDATE and DELETE stay impossible whatever this says.
CREATE POLICY staff_work_state_events_self ON staff_work_state_events
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
    )
    WITH CHECK (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
        AND actor_staff_id = current_staff_id()
    );

-- +goose Down
DROP POLICY staff_work_state_events_self ON staff_work_state_events;
DROP POLICY staff_self_update ON staff;
