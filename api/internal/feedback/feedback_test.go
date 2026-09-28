package feedback_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// testAccountPage and testPortalPage are the page/route values every
// fixture below sends -- what the page was is never the point of these
// tests, only that Insert carries it through.
const (
	testAccountPage = "/account"
	testPortalPage  = "/portal"
	testAppBuild    = "abc1234"
)

func TestValidKind(t *testing.T) {
	cases := map[string]bool{
		feedback.KindNotWorking:    true,
		feedback.KindIdeaOrRequest: true,
		feedback.KindSomethingElse: true,
		"":                         false,
		"not_a_kind":               false,
	}
	for kind, want := range cases {
		if got := feedback.ValidKind(kind); got != want {
			t.Errorf("ValidKind(%q) = %v, want %v", kind, got, want)
		}
	}
}

func TestValidText(t *testing.T) {
	atLimit := strings.Repeat("a", feedback.MaxTextRunes)
	overLimit := strings.Repeat("a", feedback.MaxTextRunes+1)
	// A multi-byte rune ("é" is two UTF-8 bytes) counted MaxTextRunes
	// times must still pass -- ValidText counts characters, not bytes.
	multiByteAtLimit := strings.Repeat("é", feedback.MaxTextRunes)

	if !feedback.ValidText("") {
		t.Error(`ValidText("") = false, want true -- empty free text is allowed`)
	}
	if !feedback.ValidText(atLimit) {
		t.Errorf("ValidText(%d chars) = false, want true -- exactly at the limit", feedback.MaxTextRunes)
	}
	if feedback.ValidText(overLimit) {
		t.Errorf("ValidText(%d chars) = true, want false -- one over the limit", feedback.MaxTextRunes+1)
	}
	if !feedback.ValidText(multiByteAtLimit) {
		t.Error("ValidText of a multi-byte string at the rune limit = false, want true -- counted in characters, not bytes")
	}
}

func TestBuildRolesLiteral(t *testing.T) {
	if got := feedback.BuildRolesLiteral(nil); got != nil {
		t.Errorf("BuildRolesLiteral(nil) = %v, want nil", got)
	}
	if got := feedback.BuildRolesLiteral([]string{}); got == nil || *got != "{}" {
		t.Errorf("BuildRolesLiteral([]string{}) = %v, want \"{}\"", got)
	}
	if got := feedback.BuildRolesLiteral([]string{"owner", "admin"}); got == nil || *got != "{owner,admin}" {
		t.Errorf("BuildRolesLiteral([owner admin]) = %v, want \"{owner,admin}\"", got)
	}
}

// --- Insert (00118_feedback.sql) ---------------------------------------

// TestInsert_StaffSenderSucceeds proves the happy path Insert exists for:
// a Staff sender whose own identity_uid resolves to staff_id, admitted by
// feedback_sender_insert's current_staff_id() half.
func TestInsert_StaffSenderSucceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "feedback-insert-staff-uid"
	staffID := testdb.SeedStaff(t, db, identityUID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	id := uuid.NewString()
	row := feedback.Row{
		ID: id, Kind: feedback.KindIdeaOrRequest, Text: "it would help if",
		PageURL: testAccountPage, RouteID: testAccountPage, AppBuild: testAppBuild, ScreenWidth: 390,
		Browser: "Safari 18", StaffID: &staffID, SentAt: time.Now(),
	}
	if err := feedback.Insert(t.Context(), tx, row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback WHERE id = $1 AND staff_id = $2`, id, staffID).Scan(&count); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for the inserted id = %d, want 1", count)
	}
}

// TestInsert_PortalSenderSucceeds is the Portal half: portal_account
// matching app.current_identity_uid directly, feedback_sender_insert's
// other branch.
func TestInsert_PortalSenderSucceeds(t *testing.T) {
	db := testdb.New(t)
	identifier := portalaccount.NewIdentifier()
	testdb.SeedPortalAccount(t, db, identifier, identifier+"@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, identifier); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	id := uuid.NewString()
	row := feedback.Row{
		ID: id, Kind: feedback.KindSomethingElse, Text: "",
		PageURL: testPortalPage, RouteID: testPortalPage, AppBuild: testAppBuild, ScreenWidth: 320,
		Browser: feedback.UnknownBrowser, PortalAccount: &identifier, SentAt: time.Now(),
	}
	if err := feedback.Insert(t.Context(), tx, row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback WHERE id = $1 AND portal_account = $2`, id, identifier).Scan(&count); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for the inserted id = %d, want 1", count)
	}
}

// TestRLS_FeedbackRefusesAForeignStaffSender is feedback_sender_insert's
// own boundary: a row naming a staff_id other than the caller's own is
// refused, the same shape TestRLS_WorkStateEventRefusesAForeignActor
// proves for staff_work_state_events.
func TestRLS_FeedbackRefusesAForeignStaffSender(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, "feedback-caller-uid")
	foreignID := testdb.SeedStaff(t, db, "feedback-foreign-uid")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "feedback-caller-uid"); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	row := feedback.Row{
		ID: uuid.NewString(), Kind: feedback.KindNotWorking, Text: "",
		PageURL: testAccountPage, RouteID: testAccountPage, AppBuild: testAppBuild, ScreenWidth: 390,
		Browser: feedback.UnknownBrowser, StaffID: &foreignID, SentAt: time.Now(),
	}
	if err := feedback.Insert(t.Context(), tx, row); err == nil {
		t.Fatal("Insert succeeded -- a caller may not send a piece of Feedback naming a different Staff member as sender")
	}
}

// TestRLS_FeedbackRefusesAForeignPortalSender is the Portal half of the
// same proof.
func TestRLS_FeedbackRefusesAForeignPortalSender(t *testing.T) {
	db := testdb.New(t)
	caller := portalaccount.NewIdentifier()
	foreign := portalaccount.NewIdentifier()
	testdb.SeedPortalAccount(t, db, caller, caller+"@example.com")
	testdb.SeedPortalAccount(t, db, foreign, foreign+"@example.com")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, caller); err != nil {
		t.Fatalf("set identity: %v", err)
	}

	row := feedback.Row{
		ID: uuid.NewString(), Kind: feedback.KindNotWorking, Text: "",
		PageURL: testPortalPage, RouteID: testPortalPage, AppBuild: testAppBuild, ScreenWidth: 390,
		Browser: feedback.UnknownBrowser, PortalAccount: &foreign, SentAt: time.Now(),
	}
	if err := feedback.Insert(t.Context(), tx, row); err == nil {
		t.Fatal("Insert succeeded -- a Portal Account may not send a piece of Feedback naming a different one as sender")
	}
}

// TestGrant_FeedbackHasNoSelect proves 00118's own grant: app_runtime
// holds INSERT and nothing else, so neither a Staff member nor a Client
// can ever read a piece of Feedback back through this table (#1523's own
// AC) -- refused at the privilege check, before RLS is even reached.
func TestGrant_FeedbackHasNoSelect(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.App.ExecContext(t.Context(), `SELECT count(*) FROM feedback`); err == nil {
		t.Fatal("SELECT on feedback succeeded -- app_runtime must hold no SELECT grant on this table")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected a permission-denied error, got: %v", err)
	}
}

// TestCheck_FeedbackSenderRejectsBothSendersOrNeither is feedback_sender
// itself (00118): a row naming both a Staff member and a Portal Account,
// or naming neither, violates the CHECK regardless of who is inserting
// it -- proved through db.Admin, which bypasses RLS but never a CHECK
// constraint, the same distinction TestGrant_ActivityIsAppendOnly's own
// package comment draws.
func TestCheck_FeedbackSenderRejectsBothSendersOrNeither(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-check-staff-uid")
	identifier := portalaccount.NewIdentifier()
	testdb.SeedPortalAccount(t, db, identifier, identifier+"@example.com")

	insert := `INSERT INTO feedback
	    (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, portal_account, sent_at)
	 VALUES ($1, 'not_working', '', '/x', '/x', 'abc1234', 390, 'Unknown browser', $2, $3, now())`

	if _, err := db.Admin.ExecContext(t.Context(), insert, uuid.NewString(), staffID, identifier); err == nil {
		t.Fatal("insert naming both a Staff member and a Portal Account succeeded -- feedback_sender must reject it")
	}
	if _, err := db.Admin.ExecContext(t.Context(), insert, uuid.NewString(), nil, nil); err == nil {
		t.Fatal("insert naming no sender at all succeeded -- feedback_sender must reject it")
	}
}
