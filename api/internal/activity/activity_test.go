package activity_test

import (
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

func TestRecord_StaffActor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	staff := fixture.Staff(t, db, p)

	diff, _ := json.Marshal(map[string]string{"note": "hello"})
	mustRecord(t, db.Admin, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: "membership",
		SubjectID:   staff.ID,
		Action:      "joined",
		Diff:        diff,
		Actor:       activity.StaffActor(staff.ID),
	})

	r := readRow(t, db, p.ID)
	if r.ActorKind != "staff" || r.ActorStaffID == nil || *r.ActorStaffID != staff.ID || r.ActorClientID != nil {
		t.Fatalf("row = %+v, want a staff actor naming %q", r, staff.ID)
	}
	if r.SubjectKind != "membership" || r.SubjectID != staff.ID || r.Action != "joined" {
		t.Fatalf("row = %+v, want subject membership/%q joined", r, staff.ID)
	}
}

func TestRecordPracticeChange_NamesThePracticeAsSubjectAndTheStaffMemberAsActor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	staff := fixture.Staff(t, db, p)

	if err := inTx(t, db.Admin, func(tx *sql.Tx) error {
		return activity.RecordPracticeChange(t.Context(), tx, p.ID, staff.ID, "name_changed", map[string]string{"nameAfter": "New"})
	}); err != nil {
		t.Fatalf("RecordPracticeChange: %v", err)
	}

	got := readRow(t, db, p.ID)
	if got.SubjectKind != activity.SubjectPractice || got.SubjectID != p.ID || got.Action != "name_changed" ||
		got.ActorKind != "staff" || got.ActorStaffID == nil || *got.ActorStaffID != staff.ID {
		t.Fatalf("row = %+v, want the Practice as subject and the Staff member as actor", got)
	}
	if got.Diff != `{"nameAfter": "New"}` {
		t.Fatalf("diff = %s", got.Diff)
	}
}

func TestRecord_ClientActor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	client := fixture.Client(t, db, p)

	mustRecord(t, db.Admin, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: "contract",
		SubjectID:   client.ID,
		Action:      "signed",
		Diff:        jsontext.Value(`{}`),
		Actor:       activity.ClientActor(client.ID),
	})

	r := readRow(t, db, p.ID)
	if r.ActorKind != "client" || r.ActorClientID == nil || *r.ActorClientID != client.ID || r.ActorStaffID != nil {
		t.Fatalf("row = %+v, want a client actor naming %q", r, client.ID)
	}
}

func TestRecord_SystemActor(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	staff := fixture.Staff(t, db, p)

	mustRecord(t, db.Admin, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: "offer",
		SubjectID:   staff.ID,
		Action:      "superseded",
		Diff:        nil,
		Actor:       activity.SystemActor(),
	})

	r := readRow(t, db, p.ID)
	if r.ActorKind != "system" || r.ActorStaffID != nil || r.ActorClientID != nil {
		t.Fatalf("row = %+v, want a system actor naming nobody", r)
	}
	if r.Diff != "{}" {
		t.Fatalf("diff = %q, want a nil Diff to fall back to an empty object", r.Diff)
	}
}

// TestRecord_CreatedAtIsTheCallersInstant: a write site that names the
// instant (#1547's terms_accepted, read through clock.Now) gets that
// instant in created_at, not the database's now().
func TestRecord_CreatedAtIsTheCallersInstant(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	staff := fixture.Staff(t, db, p)
	at := time.Date(2026, 10, 23, 9, 15, 0, 0, time.UTC)

	mustRecord(t, db.Admin, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   p.ID,
		Action:      "terms_accepted",
		Actor:       activity.StaffActor(staff.ID),
		CreatedAt:   at,
	})

	if got := readRow(t, db, p.ID).CreatedAt; !got.Equal(at) {
		t.Fatalf("created_at = %s, want %s", got, at)
	}
}

// TestMoneyActions_ContainsExactlyTheADR0008MoneySet pins the money-tier
// action set #476's read filter builds its SQL exclusion clause from --
// a drift here silently widens or narrows what an employed Doula or a
// contractor can read on her Engagement ledger.
func TestMoneyActions_ContainsExactlyTheADR0008MoneySet(t *testing.T) {
	requireExactly(t, "MoneyActions()", activity.MoneyActions(), []activity.EngagementAction{
		activity.ActionContractPriced,
		activity.ActionContractAmountOverridden,
		activity.ActionContractAmountRepriced,
		activity.ActionInvoiceRaised,
		activity.ActionInvoicePaid,
		activity.ActionInvoicePaymentFailed,
		activity.ActionInvoicePaymentProcessing,
		activity.ActionPaymentRecorded,
		activity.ActionInvoiceVoided,
		activity.ActionInvoiceWrittenOff,
		activity.ActionInvoiceNotSent,
		activity.ActionPaymentReversed,
		activity.ActionPaymentRefunded,
		activity.ActionContractorFeePaid,
		activity.ActionContractorFeePaymentReversed,
	})
}

// TestMoneyActions_Sorted proves the deterministic ordering
// engagement.ListActivityHandler's SQL-literal clause relies on.
func TestMoneyActions_Sorted(t *testing.T) {
	requireStrictlySorted(t, "MoneyActions()", activity.MoneyActions())
}

// TestStaffingActions_ContainsExactlyTheRosterSet pins the set GLOSSARY.md's
// Activity entry keeps off a Client's own portal ledger -- the whole of
// "never who inside the Practice did what", not the roster alone. An
// Offer is which Doula was asked, accepted or bumped, and a Visit
// reassignment is which Doula covers it, both Practice-roster facts
// rather than facts about her; a void request and its refusal (#1096) are
// the Practice deliberating with itself about her Contract, with nothing
// of hers changed either way. The three facts #1423 moved out of
// engagement_events are here too, because ADR-0015 kept that audit off
// the portal. Money actions are deliberately absent from
// this set (GLOSSARY.md: "her money" stays on her own ledger) -- a drift
// here would either leak the Practice's own facts to a Client or hide a
// fact she is owed.
func TestStaffingActions_ContainsExactlyTheRosterSet(t *testing.T) {
	requireExactly(t, "StaffingActions()", activity.StaffingActions(), []activity.EngagementAction{
		activity.ActionOfferSent,
		activity.ActionOfferAccepted,
		activity.ActionOfferDeclined,
		activity.ActionOfferSuperseded,
		activity.ActionOfferWithdrawn,
		activity.ActionVisitReassigned,
		activity.ActionContractVoidRequested,
		activity.ActionContractVoidDeclined,
		activity.ActionCoverageGapCreated,
		activity.ActionCoverageGapUpdated,
		activity.ActionCoverageGapCleared,
		activity.ActionOnCallNarrowingChanged,
		activity.ActionOnCallRuleChanged,
		// #1596: who the Practice put on the birth.
		activity.ActionDoulaAttached,
		// #2596: and who it took off.
		activity.ActionDoulaTakenOff,
		// #1423: engagement_events' three facts, folded in, stay staff-only.
		activity.ActionEngagementReopened,
		activity.ActionKindChanged,
		activity.ActionBirthOutcomeRecorded,
		// #2013: the due date is one more of ADR-0015's mutable facts.
		activity.ActionDueDateChanged,
		// #2066: the Care Plan is staff-only, never shown in the portal.
		activity.ActionCarePlanEdited,
		// #2133: and so is its start.
		activity.ActionCarePlanStarted,
		// #2271: what the Practice paid its contractor Doula.
		activity.ActionContractorFeePaid,
		activity.ActionContractorFeePaymentReversed,
		// #2279: a refused accept changes nothing of hers.
		activity.ActionPortalInviteRefused,
		// #2520: a Draft she has not been sent.
		activity.ActionContractEdited,
		// An Invoice draft Stripe would not finalize never reached her.
		activity.ActionInvoiceNotSent,
	})
}

// TestStaffingActions_Sorted proves the deterministic ordering a caller
// building a SQL exclusion clause relies on, the same way MoneyActions'
// own ordering test does.
func TestStaffingActions_Sorted(t *testing.T) {
	requireStrictlySorted(t, "StaffingActions()", activity.StaffingActions())
}

// TestBirthPlanActions_NamesEveryBirthPlanAction proves #2219's set holds
// every action whose name carries the Birth Plan, sorted, and none of
// them a staffing action: the portal reader adds the set only where a
// Birth Plan is not offered, so one already excluded everywhere would
// mean the Birth Plan was never on her ledger to begin with.
func TestBirthPlanActions_NamesEveryBirthPlanAction(t *testing.T) {
	got := activity.BirthPlanActions()
	want := []activity.EngagementAction{
		activity.ActionBirthPlanAcknowledged,
		activity.ActionBirthPlanEdited,
		activity.ActionBirthPlanStarted,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("BirthPlanActions() = %v, want %v", got, want)
	}
	for _, a := range got {
		if slices.Contains(activity.StaffingActions(), a) {
			t.Errorf("%q is a staffing action; it would never reach her ledger at all", a)
		}
	}
}

// TestRecord_ScopedToLetsAWriteOutsideStaffauthMiddlewarePassRLS proves
// the landmine ADR-0022 names for a write site with no per-request
// app.current_practice_id (a Client-portal or webhook path): a Record
// call carrying activity.ScopedTo passes activity's own RLS policy,
// where a plain Record call from the same session does not.
func TestRecord_ScopedToLetsAWriteOutsideStaffauthMiddlewarePassRLS(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	client := fixture.Client(t, db, p)
	entry := activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   client.ID,
		Action:      "contract_signed",
		Actor:       activity.ClientActor(client.ID),
	}

	// Each write is its own transaction: a failed statement aborts the
	// rest of a Postgres transaction (SQLSTATE 25P02), so the RLS failure
	// and the ScopedTo success cannot share one.
	if err := record(t, db.App, entry); err == nil {
		t.Fatal("expected Record to fail RLS with no app.current_practice_id set, got no error")
	}
	if err := record(t, db.App, entry, activity.ScopedTo(p.ID)); err != nil {
		t.Fatalf("Record with ScopedTo: %v", err)
	}
}
