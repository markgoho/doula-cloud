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

// The two roles this package's tests seed a membership with, named once
// for goconst.
const (
	testOwnerRole = "owner"
	testAdminRole = "admin"
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
	if got := feedback.BuildRolesLiteral([]string{testOwnerRole, testAdminRole}); got == nil || *got != "{owner,admin}" {
		t.Errorf("BuildRolesLiteral([owner admin]) = %v, want \"{owner,admin}\"", got)
	}
}

// --- Insert (00118_feedback.sql) ---------------------------------------

// TestInsert_AdmitsTheCallersOwnSenderAndRefusesAnotherSender holds
// feedback_sender_insert to both its branches: a Staff sender whose own
// identity_uid resolves to staff_id (the current_staff_id() half) and a
// Portal sender whose portal_account matches app.current_identity_uid
// directly are admitted, and a row naming a different Staff member or
// Portal Account than the caller's own is refused.
func TestInsert_AdmitsTheCallersOwnSenderAndRefusesAnotherSender(t *testing.T) {
	cases := []struct {
		name string
		// seed returns the caller's identity and the sender the row names.
		seed   func(*testing.T, *testdb.DB) (identity, sender string)
		portal bool
		want   bool
	}{
		{"own Staff member", func(t *testing.T, db *testdb.DB) (string, string) {
			return "feedback-insert-staff-uid", testdb.SeedStaff(t, db, "feedback-insert-staff-uid")
		}, false, true},
		{"own Portal Account", func(t *testing.T, db *testdb.DB) (string, string) {
			identifier := portalaccount.NewIdentifier()
			testdb.SeedPortalAccount(t, db, identifier, identifier+"@example.com")
			return identifier, identifier
		}, true, true},
		{"another Staff member", func(t *testing.T, db *testdb.DB) (string, string) {
			testdb.SeedStaff(t, db, "feedback-caller-uid")
			return "feedback-caller-uid", testdb.SeedStaff(t, db, "feedback-foreign-uid")
		}, false, false},
		{"another Portal Account", func(t *testing.T, db *testdb.DB) (string, string) {
			caller, foreign := portalaccount.NewIdentifier(), portalaccount.NewIdentifier()
			testdb.SeedPortalAccount(t, db, caller, caller+"@example.com")
			testdb.SeedPortalAccount(t, db, foreign, foreign+"@example.com")
			return caller, foreign
		}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			identity, sender := tc.seed(t, db)
			tx := db.BeginAs(t, testdb.Identity(identity))

			id := uuid.NewString()
			row := feedback.Row{
				ID: id, Kind: feedback.KindIdeaOrRequest, Text: "",
				PageURL: testAccountPage, RouteID: testAccountPage, AppBuild: testAppBuild, ScreenWidth: 390,
				Browser: feedback.UnknownBrowser, SentAt: time.Now(),
			}
			column := "staff_id"
			if tc.portal {
				row.PortalAccount, column = &sender, "portal_account"
			} else {
				row.StaffID = &sender
			}
			err := feedback.Insert(t.Context(), tx, row)
			if !tc.want {
				if err == nil {
					t.Fatal("Insert succeeded -- a caller may not send a piece of Feedback naming a different sender")
				}
				return
			}
			if err != nil {
				t.Fatalf("Insert: %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			if count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM feedback WHERE id = $1 AND `+column+`::text = $2`, id, sender); count != 1 {
				t.Fatalf("rows for the inserted id = %d, want 1", count)
			}
		})
	}
}

// TestGrant_FeedbackSelectIsRLSGatedToTheTrustedWorker proves 00120's own
// grant: SELECT on feedback exists for app_runtime (#1524's outbox needs
// it to build an issue), but feedback_notification_worker_select (00120)
// admits no row at all outside the trusted-worker door -- so neither a
// Staff member nor a Client can read a piece of Feedback back through an
// ordinary session (#1523's own AC), even though the privilege check
// itself now passes. TestProcessIssueOutboxHandler_RunsBehindTheDoor
// (issue_outbox_test.go) proves the other side: the same SELECT succeeds
// and returns rows once the door is open.
func TestGrant_FeedbackSelectIsRLSGatedToTheTrustedWorker(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "feedback-select-gated")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, sent_at)
		 VALUES ($1, 'not_working', '', '/account', '/account', 'abc1234', 390, 'Unknown browser', $2, now())`,
		uuid.NewString(), staffID,
	); err != nil {
		t.Fatalf("seed feedback row: %v", err)
	}

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback`).Scan(&count); err != nil {
		t.Fatalf("SELECT on feedback failed: %v -- app_runtime must hold the grant #1524 added", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 -- an ordinary session with the trusted-worker door closed must see no row", count)
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
