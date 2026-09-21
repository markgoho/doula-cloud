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
-- also admits the same row everywhere else a Staff-side reader hits it
-- by construction -- the Engagement ledger's actor name, the Membership
-- history's actor name (#872), and message.ListHandler's and
-- visit.listVisits' own unresolvedStaffSenderName fallback (#1322,
-- already closed on its own DepartedStaffName-shaped fix). That is not a
-- second migration's worth of work pulled forward; RLS cannot be scoped
-- to one caller, so a correct policy for this ticket's own read is,
-- mechanically, a correct policy for every other Staff-side read of the
-- same table too. Said as mechanism rather than as a tested claim: this
-- PR touches no code in message/ or visit/, and their own #1322 fixture
-- (testdb.RemoveMembership, a bare DELETE) never writes the 'removed'
-- activity row this policy's own function reads, so their existing
-- tests still exercise the "genuinely no audit trail" case and pass
-- unchanged rather than proving the wider claim. Production's own
-- RemoveMembershipHandler and logindeletion.go always write that row
-- first, so the mechanism holds there; it is recorded here rather than
-- left to be rediscovered, not verified by a new test in this PR.
--
-- Client-portal sessions never set app.current_practice_id
-- (clientauth.Middleware only ever sets app.current_client_id and
-- app.current_identity_uid), so this policy's guard on
-- app.current_practice_id being set already excludes them without a
-- separate Staff-only term -- the same guard 00002's own
-- staff_practice_visibility policy relies on for the same reason.
-- The live-membership check below lives inside this function, not in the
-- policy's own USING clause, and that placement is load-bearing, not
-- style. SECURITY DEFINER makes the function run as the table owner,
-- which bypasses RLS on every table *it* queries -- so a NOT EXISTS
-- against practice_memberships in here pays only that table's own index
-- lookup. The same NOT EXISTS written directly into the policy's USING
-- clause is a different query entirely: the rewriter inlines it, which
-- drags in practice_memberships' OWN policies -- including
-- practice_memberships_visible_to_own_client_portal_engagements (00009),
-- itself an inlined EXISTS through engagements -- and that cascade
-- doubles the already-near-jit_above_cost estimated cost of
-- engagement.listEngagementActivity's own three staff joins (99,757 of a
-- 100,000 budget, measured with that version of this migration applied,
-- against a 77,043 pre-migration baseline -- 22,714 of 22,957 available
-- points spent, a margin no other change could safely share). Measured
-- with the guard moved in here instead: see this policy's own doc
-- comment below for the number, and
-- engagement/activity_jit_internal_test.go's own
-- TestListEngagementActivityQuery_StaysOffTheJITCliff for the canary
-- that would have caught the first version outright had its own
-- threshold been any narrower.
-- +goose StatementBegin
CREATE FUNCTION staff_was_ever_a_member_at_practice(target_staff_id uuid, target_practice_id uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT NOT EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = target_staff_id
              AND pm.practice_id = target_practice_id
        )
        AND EXISTS (
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
-- This USING clause is one opaque function call plus two cheap guards --
-- current_setting and a column read -- the same shape 00111's own policy
-- has, and for the same reason: anything more than that here is visible
-- to the rewriter and eligible for inlining. An earlier version of this
-- migration put the "already a live member" short-circuit here instead,
-- as its own NOT EXISTS against practice_memberships. That query reads
-- fine on its own, but practice_memberships carries a policy of its own
-- that is not SECURITY DEFINER --
-- practice_memberships_visible_to_own_client_portal_engagements (00009),
-- an inlined EXISTS through engagements -- and the rewriter pulled that
-- whole cascade into this one too. Measured against
-- engagement.listEngagementActivity, the one reader in this codebase
-- that already joins staff three times (engagement/activity_jit_internal
-- _test.go's own TestListEngagementActivityQuery_StaysOffTheJITCliff):
-- 99,757 of a 100,000 jit_above_cost budget, against a 77,043
-- pre-migration baseline -- the guard priced almost every point of
-- margin that query had left, on a canary built for exactly this
-- failure mode. The short-circuit moved inside
-- staff_was_ever_a_member_at_practice instead (see its own doc comment):
-- SECURITY DEFINER means that NOT EXISTS runs as the table owner and
-- never touches practice_memberships' own policies at all, so nothing
-- about this policy's shape gives the rewriter anything to pull in.
-- Re-measured with the guard moved: the same canary reads 77,223 -- 180
-- points over the unmodified baseline for one more opaque call in the
-- plan, not a cascade.
--
-- Wall-clock, on TestPracticeQueryPlanAtScale's own all-live-actor
-- fixture (perf_test.go): 67 ms against a 10.6 ms pre-migration
-- baseline, no JIT -- every one of the 5,000 rows now calls the function
-- once (it short-circuits its own internal AND on the live-membership
-- half, but still pays one SPI round trip per row, which the plain
-- EXISTS this replaced never did).
-- TestPracticeQueryPlanWithDepartedMembershipSubjects measures this
-- policy's own worst case instead -- 5,000 rows where every subject
-- really is departed, so the function's internal AND runs both halves
-- every time: 140 ms, still no JIT. Both
-- numbers are slower than the version this migration rejected above, and
-- that trade is deliberate: the estimated-cost budget the JIT canary
-- guards is a hard ceiling this codebase cannot safely spend from, where
-- a few dozen milliseconds of actual execution time on a fixture four
-- orders of magnitude past a pilot Practice's real Membership-event
-- volume is not. Both perf tests assert the JIT section's absence rather
-- than only a wall-clock number, which is the one thing 00111's own
-- history says actually matters, and the Engagement ledger's own canary
-- is what caught the version of this migration that would have shipped
-- otherwise.
CREATE POLICY staff_visible_to_own_practice_membership_history ON staff
    FOR SELECT
    USING (
        staff.deleted_at IS NULL
        AND NULLIF(current_setting('app.current_practice_id', true), '') IS NOT NULL
        AND staff_was_ever_a_member_at_practice(staff.id, NULLIF(current_setting('app.current_practice_id', true), '')::uuid)
    );

-- +goose Down
DROP POLICY staff_visible_to_own_practice_membership_history ON staff;
DROP FUNCTION staff_was_ever_a_member_at_practice(uuid, uuid);
