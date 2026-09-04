package activitygate_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitygate"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole            = "owner"
	doulaRole            = "doula"
	employeeType         = "employee"
	contractorType       = "contractor"
	contractSignedAction = "contract_signed"
	invoicePaidAction    = "invoice_paid"
)

// seedEngagement seeds one Client and one Engagement at practiceID, the
// minimum an engagement-scoped Rule needs to decide access on.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Gate Test Client', 'gate-client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedAttachment mirrors staffauth_test's own helper of the same name.
func seedAttachment(t *testing.T, db *testdb.DB, engagementID, staffID, origin string, ended bool) {
	t.Helper()
	endedAt := "NULL"
	if ended {
		endedAt = "now()"
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at)
		 VALUES ($1, $2, $3::attachment_origin, $2, `+endedAt+`)`,
		engagementID, staffID, origin,
	); err != nil {
		t.Fatalf("seed attachment (origin=%s, ended=%v): %v", origin, ended, err)
	}
}

// resolveReader begins a tx on db.App, scopes it to practiceID (the same
// set_config staffauth.Middleware performs per request), and resolves a
// Reader for staffID -- the one way to build a Reader (ResolveReader is
// its sole constructor), so every gate test goes through the real DB. The
// tx it returns is the one the reader was resolved on and stays open for
// the rest of the test (a gate call needs the same app.current_practice_id
// scoping CanAccessEngagement/CanAccessClient's own RLS-bound queries
// rely on).
func resolveReader(t *testing.T, db *testdb.DB, practiceID, staffID string) (staffauth.Reader, *sql.Tx) {
	t.Helper()
	tx := beginScopedTx(t, db, practiceID)
	reader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, staffID)
	if err != nil {
		t.Fatalf("ResolveReader: %v", err)
	}
	return reader, tx
}

func beginScopedTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}
	return tx
}

// TestCanAccessSubject_UnregisteredKindRefused proves AC6: a subject kind
// with no registered Rule is refused, never silently allowed -- even for
// an Owner reader, who passes every registered Rule's access check. Only
// a registry miss can explain a false here.
func TestCanAccessSubject_UnregisteredKindRefused(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Unregistered Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-unregistered-owner", []string{ownerRole}, employeeType)
	reader, tx := resolveReader(t, db, practiceID, ownerID)

	got, err := activitygate.CanAccessSubject(t.Context(), tx, reader, "bogus", "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("CanAccessSubject: %v", err)
	}
	if got {
		t.Fatal("CanAccessSubject(bogus) = true, want false for an unregistered subject kind")
	}
}

// TestCanSeeAction_UnregisteredKindRefused is TestCanAccessSubject_UnregisteredKindRefused's
// row-level counterpart.
func TestCanSeeAction_UnregisteredKindRefused(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Unregistered Action Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-unregistered-action-owner", []string{ownerRole}, employeeType)
	reader, _ := resolveReader(t, db, practiceID, ownerID)

	if activitygate.CanSeeAction(reader, "bogus", "anything_happened") {
		t.Fatal("CanSeeAction(bogus) = true, want false for an unregistered subject kind")
	}
}

// TestCanAccessSubject_Engagement proves the shared gate's engagement Rule
// reproduces the ADR-0008 read table engagement.ListActivityHandler used
// to check inline: Owner reaches; a contractor with no granted attachment
// does not; a contractor with one does. Mirrors
// staffauth_test.TestReader_CanAccessEngagement's own cases, one layer up.
func TestCanAccessSubject_Engagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Engagement Access Practice")
	_, engagementID := seedEngagement(t, db, practiceID)

	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-engagement-owner", []string{ownerRole}, employeeType)
	unattachedContractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-engagement-contractor-unattached", []string{doulaRole}, contractorType)
	attachedContractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-engagement-contractor-attached", []string{doulaRole}, contractorType)
	seedAttachment(t, db, engagementID, attachedContractorID, "granted", false)

	cases := []struct {
		name    string
		staffID string
		want    bool
	}{
		{"owner reaches", ownerID, true},
		{"unattached contractor refused", unattachedContractorID, false},
		{"attached contractor reaches", attachedContractorID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, tx := resolveReader(t, db, practiceID, tc.staffID)
			got, err := activitygate.CanAccessSubject(t.Context(), tx, reader, activity.SubjectEngagement, engagementID)
			if err != nil {
				t.Fatalf("CanAccessSubject: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CanAccessSubject(engagement) = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRestrictedActions_Engagement pins the exact literal action strings
// engagement.ListActivityHandler's SQL exclusion clause is built from --
// mirrors activity_test.TestMoneyActions_ContainsExactlyTheADR0008MoneySet
// one layer up, against literals rather than recomputing from
// activity.MoneyActions() (which would just restate the production code).
func TestRestrictedActions_Engagement(t *testing.T) {
	want := []string{"contract_created", "contract_sent", contractSignedAction, "contract_voided", "invoice_raised", invoicePaidAction}
	got := activitygate.RestrictedActions(activity.SubjectEngagement)
	if len(got) != len(want) {
		t.Fatalf("RestrictedActions(engagement) = %v, want exactly %v", got, want)
	}
	gotSet := map[string]bool{}
	for _, a := range got {
		gotSet[a] = true
	}
	for _, a := range want {
		if !gotSet[a] {
			t.Errorf("RestrictedActions(engagement) missing %q", a)
		}
	}
}

// TestRestrictedActions_UnregisteredKindIsNil proves an unregistered
// subject kind has nothing to build a SQL exclusion clause from -- it
// must be refused at CanAccessSubject before RestrictedActions is ever
// consulted, never treated as unrestricted.
func TestRestrictedActions_UnregisteredKindIsNil(t *testing.T) {
	if got := activitygate.RestrictedActions("bogus"); got != nil {
		t.Fatalf("RestrictedActions(bogus) = %v, want nil", got)
	}
}

// TestCanSeeAction_Engagement proves the row-level decision an
// Owner/Admin bypasses ADR-0008's money tier and nobody else does,
// regardless of employment type -- matching
// engagement_test.TestListActivityHandler_EmployeeDoulaExcludesMoneyEntries
// and its contractor counterpart one layer up.
func TestCanSeeAction_Engagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate CanSeeAction Engagement Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-canseeaction-owner", []string{ownerRole}, employeeType)
	employeeID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-canseeaction-employee", []string{doulaRole}, employeeType)
	contractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-canseeaction-contractor", []string{doulaRole}, contractorType)

	cases := []struct {
		name    string
		staffID string
		action  string
		want    bool
	}{
		{"owner sees invoice_paid", ownerID, invoicePaidAction, true},
		{"owner sees contract_signed", ownerID, contractSignedAction, true},
		{"employee denied invoice_paid", employeeID, invoicePaidAction, false},
		{"employee denied contract_signed", employeeID, contractSignedAction, false},
		{"employee sees visit_logged", employeeID, "visit_logged", true},
		{"contractor denied invoice_paid", contractorID, invoicePaidAction, false},
		{"contractor denied contract_signed", contractorID, contractSignedAction, false},
		{"contractor sees offer_accepted", contractorID, "offer_accepted", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, _ := resolveReader(t, db, practiceID, tc.staffID)
			if got := activitygate.CanSeeAction(reader, activity.SubjectEngagement, tc.action); got != tc.want {
				t.Fatalf("CanSeeAction(engagement, %q) = %v, want %v", tc.action, got, tc.want)
			}
		})
	}
}

// TestCanAccessSubject_Client proves the shared gate's client Rule
// reproduces client.DetailHandler's own reader.CanAccessClient check
// (client/detail.go): attachment reaches through any of the Client's
// Engagements, per ADR-0017. Mirrors
// staffauth_test.TestReader_CanAccessClient one layer up.
func TestCanAccessSubject_Client(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Client Access Practice")
	clientID, engagementID := seedEngagement(t, db, practiceID)

	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-client-owner", []string{ownerRole}, employeeType)
	unattachedContractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-client-contractor-unattached", []string{doulaRole}, contractorType)
	attachedContractorID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-client-contractor-attached", []string{doulaRole}, contractorType)
	seedAttachment(t, db, engagementID, attachedContractorID, "granted", false)

	cases := []struct {
		name    string
		staffID string
		want    bool
	}{
		{"owner reaches", ownerID, true},
		{"unattached contractor refused", unattachedContractorID, false},
		{"attached contractor reaches", attachedContractorID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, tx := resolveReader(t, db, practiceID, tc.staffID)
			got, err := activitygate.CanAccessSubject(t.Context(), tx, reader, activity.SubjectClient, clientID)
			if err != nil {
				t.Fatalf("CanAccessSubject: %v", err)
			}
			if got != tc.want {
				t.Fatalf("CanAccessSubject(client) = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCanAccessSubject_ClientFieldTemplateUnregistered pins #485's AC5
// choice for client_field_template specifically, rather than relying only
// on the generic "bogus kind" pin: clientfieldtemplate.Save writes
// activity rows (template.go), but no handler reads them back, so there
// is no access rule to reuse yet -- it is deliberately absent from the
// registry, refused like any other unregistered kind, until a reader
// exists to register one.
func TestCanAccessSubject_ClientFieldTemplateUnregistered(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Client Field Template Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-cft-owner", []string{ownerRole}, employeeType)
	reader, tx := resolveReader(t, db, practiceID, ownerID)

	got, err := activitygate.CanAccessSubject(t.Context(), tx, reader, "client_field_template", practiceID)
	if err != nil {
		t.Fatalf("CanAccessSubject: %v", err)
	}
	if got {
		t.Fatal("CanAccessSubject(client_field_template) = true, want false: no reader exists yet to justify registering it")
	}
}

// TestBypasses proves Bypasses (the SQL-parameter form
// engagement.ListActivityHandler passes as its query's moneyGate
// placeholder) agrees with CanSeeAction's own Owner/Admin check.
func TestBypasses(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Gate Bypasses Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-bypasses-owner", []string{ownerRole}, employeeType)
	employeeID := testdb.SeedStaffAtPractice(t, db, practiceID, "gate-bypasses-employee", []string{doulaRole}, employeeType)

	ownerReader, _ := resolveReader(t, db, practiceID, ownerID)
	if !activitygate.Bypasses(ownerReader) {
		t.Fatal("Bypasses(owner) = false, want true")
	}
	employeeReader, _ := resolveReader(t, db, practiceID, employeeID)
	if activitygate.Bypasses(employeeReader) {
		t.Fatal("Bypasses(employee doula) = true, want false")
	}
}
