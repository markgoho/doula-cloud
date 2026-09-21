-- +goose Up
-- #1256: a `removed` Membership row on the practice-wide feed names the
-- person who left, instead of reading `activity.DepartedStaffName` ("a
-- former colleague") for every roster row about her -- 'removed' itself
-- and every 'joined'/'roles_changed' row still sitting further down the
-- same feed from when she was here.
--
-- The gap. staff_practice_visibility (00002) reaches a staff row only
-- through a *live* practice_memberships row. A 'removed' event's subject
-- is, by construction, someone whose row that policy just stopped
-- admitting -- activityfeed.resolveSubjectName's own doc comment records
-- the degradation this migration closes.
--
-- ---------------------------------------------------------------------
-- Route taken, and why the other was not.
--
-- Two routes were open. This migration takes the first: a fourth policy
-- on staff, SECURITY DEFINER-shaped exactly as 00111 shaped its own, so
-- this doesn't repeat 00105's mistake -- a plain EXISTS policy that the
-- planner inlines into every query touching staff, crossing
-- jit_above_cost and turning a 1.7 ms read into 6.7 s of LLVM (00111's
-- own doc comment has the numbers). A SECURITY DEFINER function stays an
-- opaque call the rewriter cannot inline, which is what keeps this
-- migration from repeating that failure.
--
-- The second route -- writing the departed person's name into
-- staffauth.RecordMembershipEvent's diff at write time, and reading it
-- back in the feed's query -- was not taken because it cannot honor
-- ADR-0033. `activity` is INSERT-only (ADR-0022, no DELETE grant), and
-- ADR-0033's own "No crypto-shredding" section says plainly why a
-- Membership event builds no shredding analog: "There is nothing sealed
-- and nothing to shred." A name copied into a diff at write time would
-- be exactly that -- something sealed with nothing to shred -- and would
-- outlive a later login deletion, handing back her real name forever
-- after ADR-0033 promised the opposite. A policy that reads the current
-- state of `staff` at query time does not have this problem: once
-- redactStaffRow (logindeletion.go) stamps deleted_at, this policy's own
-- guard stops admitting the row, and DepartedStaffName is what a reader
-- sees again -- AC4.
--
-- ---------------------------------------------------------------------
-- Scope, said plainly, because this is table-wide and not
-- query-scoped -- no RLS policy can be.
--
-- staff_was_ever_a_member_at_practice answers "did this person ever
-- appear as a Membership event's subject at this Practice?", not "is she
-- a member now" (00002 already answers that) and not "has she ever
-- worked at any Practice" (unscoped, which this deliberately is not).
-- The activity table is the only place that question has a durable
-- answer once practice_memberships' own row for her is gone -- every
-- Membership start records a 'joined' event (signup.go, accept.go), so
-- anyone this policy needs to admit has one.
--
-- Because this policy applies to every SELECT against staff for the
-- app_runtime role, not only the practice-wide feed's own subj join, it
-- also resolves the same degradation everywhere else a Staff-side reader
-- hits it -- the Engagement ledger's actor name, the Membership
-- history's actor name (#872), and the two gaps #1322 named in
-- message.ListHandler and visit.listVisits. That is not a second
-- migration's worth of work pulled forward; RLS cannot be scoped to one
-- caller, so a correct policy for this ticket's own read is, by
-- construction, a correct policy for every other Staff-side read of the
-- same table, and #1322's own two fixes (message.go, visit.go) already
-- fall back to DepartedStaffName exactly where this policy now finds a
-- real name to give them instead.
--
-- Client-portal sessions never set app.current_practice_id
-- (clientauth.Middleware only ever sets app.current_client_id and
-- app.current_identity_uid), so this policy's guard on
-- app.current_practice_id being set already excludes them without a
-- separate Staff-only term -- the same guard 00002's own
-- staff_practice_visibility policy relies on for the same reason.
-- +goose StatementBegin
CREATE FUNCTION staff_was_ever_a_member_at_practice(target_staff_id uuid, target_practice_id uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM activity
            WHERE practice_id = target_practice_id
              AND subject_kind = 'membership'
              AND subject_id = target_staff_id
        )
    $$;
-- +goose StatementEnd

-- deleted_at IS NULL excludes a redacted row (ADR-0033) the same way
-- 00111's own policy does, and for the same reason: her deleted_at
-- stamp means DepartedStaffName is the word a Staff reader gets, not
-- "Deleted Staff Member" -- an internal sentinel that was never meant to
-- reach a reader.
--
-- The NOT EXISTS guard measures its own cost: without it,
-- listPracticeActivityQuery's actor join re-evaluates
-- staff_was_ever_a_member_at_practice for every one of a fixture's 5,000
-- rows (they share one live actor, so 00002's own staff_practice_
-- visibility policy already admits every one of them) and execution time
-- moves from 10.6 ms to 54.8 ms -- no JIT, but a real 5x this migration
-- does not need to spend. 00002's own policy is a plain column
-- comparison, unable to short-circuit an OR'd sibling policy it knows
-- nothing about, so the short-circuit has to be authored inside this
-- policy's own USING clause instead: a currently-live Membership is
-- common (most actor rows are a live Owner or Admin) and cheap to rule
-- out first, an indexed lookup on practice_memberships' own (practice_id,
-- staff_id) unique key, not a cascading EXISTS through visits or
-- engagements the way 00105's own failed attempt was -- so inlining it
-- costs nothing like that one did. Re-measured with the guard in place,
-- on TestPracticeQueryPlanAtScale's own all-live-actor fixture where the
-- guard rules out every row before the function is ever called: 19 ms,
-- repeatable across runs, against a 10.6 ms pre-migration baseline and no
-- JIT section -- the added cost is the extra OR branch's own planning,
-- not a per-row function call.
-- TestPracticeQueryPlanWithDepartedMembershipSubjects measures the
-- guard's own worst case instead -- 5,000 rows where every subject
-- really is departed, so the guard is false and the function runs every
-- time: 68 ms, still no JIT. Both live in perf_test.go, and both assert
-- the JIT section's absence rather than a wall-clock number, which is
-- the one thing 00111's own history says actually matters.
CREATE POLICY staff_visible_to_own_practice_membership_history ON staff
    FOR SELECT
    USING (
        staff.deleted_at IS NULL
        AND NULLIF(current_setting('app.current_practice_id', true), '') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = staff.id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
        AND staff_was_ever_a_member_at_practice(staff.id, NULLIF(current_setting('app.current_practice_id', true), '')::uuid)
    );

-- +goose Down
DROP POLICY staff_visible_to_own_practice_membership_history ON staff;
DROP FUNCTION staff_was_ever_a_member_at_practice(uuid, uuid);
