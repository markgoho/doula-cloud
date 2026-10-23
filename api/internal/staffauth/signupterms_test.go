package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// signedUpAt is the instant the fixed clock below reports, so a test can
// tell the server's clock.Now(ctx) from the database's own now().
var signedUpAt = time.Date(2026, 10, 23, 14, 30, 0, 0, time.UTC)

// newClockedSignupServer is newSignupServer behind clock.Middleware, with
// a fixed instant.
func newClockedSignupServer(verifier authntest.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	return httptest.NewServer(clock.Middleware(func() time.Time { return signedUpAt })(mux))
}

type termsRow struct {
	subjectKind, subjectID, actorKind, actorStaffID string
	diff                                            map[string]string
	createdAt                                       time.Time
}

func termsAcceptedRows(t *testing.T, db *testdb.DB) []termsRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT subject_kind, subject_id::text, actor_kind::text, coalesce(actor_staff_id::text, ''), diff::text, created_at
		   FROM activity WHERE action = 'terms_accepted'`)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer rows.Close()
	var out []termsRow
	for rows.Next() {
		var r termsRow
		var diff string
		if err := rows.Scan(&r.subjectKind, &r.subjectID, &r.actorKind, &r.actorStaffID, &diff, &r.createdAt); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		if err := json.Unmarshal([]byte(diff), &r.diff); err != nil {
			t.Fatalf("diff %s: %v", diff, err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

// #1547 (ADR-0053): the transaction that creates the Practice records that
// its Owner agreed, to which version of each document, when, and from
// which address. The versions are legal's, never the request's.
func TestSignupHandler_RecordsTheOwnersAgreement(t *testing.T) {
	db := testdb.New(t)
	srv := newClockedSignupServer(authntest.Verifier{UID: "agreeing-owner", Email: jamieEmail}, db)
	defer srv.Close()

	// A body that names versions of its own: the handler reads none of it.
	var body struct {
		staffauth.SignupRequest
		TermsVersion   string `json:"termsVersion"`
		PrivacyVersion string `json:"privacyVersion"`
	}
	body.SignupRequest = staffauth.SignupRequest{WorkState: "NY", Timezone: signupZone, FirstName: jamieName, LastName: ownerLastName}
	body.TermsVersion, body.PrivacyVersion = "1999-01-01", "1999-01-01"
	resp := postSignup(t, srv, "tok", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	rows := termsAcceptedRows(t, db)
	if len(rows) != 1 {
		t.Fatalf("terms_accepted rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.subjectKind != "practice" || r.subjectID != out.PracticeID {
		t.Fatalf("subject = %s/%s, want practice/%s", r.subjectKind, r.subjectID, out.PracticeID)
	}
	if r.actorKind != "staff" || r.actorStaffID != out.StaffID {
		t.Fatalf("actor = %s/%s, want staff/%s", r.actorKind, r.actorStaffID, out.StaffID)
	}
	want := map[string]string{
		"termsVersion":   legal.Terms.Current().Effective,
		"privacyVersion": legal.Privacy.Current().Effective,
		"ipAddress":      "127.0.0.1",
	}
	if len(r.diff) != len(want) {
		t.Fatalf("diff = %v, want %v", r.diff, want)
	}
	for k, v := range want {
		if r.diff[k] != v {
			t.Fatalf("diff[%q] = %q, want %q (diff %v)", k, r.diff[k], v, r.diff)
		}
	}
	if !r.createdAt.Equal(signedUpAt) {
		t.Fatalf("created_at = %s, want the server's clock, %s", r.createdAt, signedUpAt)
	}
}

// A signup that fails writes no agreement: the row is in the transaction
// that creates the Practice, so a refusal or a failed commit leaves none.
func TestSignupHandler_AFailedSignupRecordsNoAgreement(t *testing.T) {
	t.Run("the commit fails", func(t *testing.T) {
		db := testdb.New(t)
		srv := newSignupServer(authntest.Verifier{UID: "unminted-owner", Email: jamieEmail}, db)
		defer srv.Close()
		if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
			t.Fatalf("drop sessions: %v", err)
		}
		resp := postSignup(t, srv, "tok", staffauth.SignupRequest{WorkState: "NY", FirstName: jamieName, LastName: ownerLastName})
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
		}
		if rows := termsAcceptedRows(t, db); len(rows) != 0 {
			t.Fatalf("terms_accepted rows = %d, want 0", len(rows))
		}
	})

	t.Run("she already belongs to a Practice", func(t *testing.T) {
		db := testdb.New(t)
		srv := newSignupServer(authntest.Verifier{UID: "twice-owner", Email: jamieEmail}, db)
		defer srv.Close()
		body := staffauth.SignupRequest{WorkState: "NY", FirstName: jamieName, LastName: ownerLastName}
		first := postSignup(t, srv, "tok", body)
		_ = first.Body.Close()
		second := postSignup(t, srv, "tok", body)
		_ = second.Body.Close()
		if second.StatusCode != http.StatusConflict {
			t.Fatalf("second status = %d, want %d", second.StatusCode, http.StatusConflict)
		}
		if rows := termsAcceptedRows(t, db); len(rows) != 1 {
			t.Fatalf("terms_accepted rows = %d, want only the first signup's", len(rows))
		}
	})
}
