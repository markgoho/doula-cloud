package practicename_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicename"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	ownerRole    = "owner"
	adminRole    = "admin"
	doulaRole    = "doula"
	employeeType = "employee"
	newName      = "Willow Birth Services"
)

func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	practicename.Mount(ir)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func putRaw(t *testing.T, srv *httptest.Server, session, practiceID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/name", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putName(t *testing.T, srv *httptest.Server, session, practiceID, name string) *http.Response {
	t.Helper()
	payload, err := json.Marshal(practicename.PutRequest{Name: name})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putRaw(t, srv, session, practiceID, payload)
}

func storedName(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var name string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
		t.Fatalf("read stored name: %v", err)
	}
	return name
}

func activityCount(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND action = 'practice_name_changed'`, practiceID,
	).Scan(&n); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	return n
}

func TestPutHandler_OwnerRenamesAndItIsRecorded(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-owner"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	before := storedName(t, db, practiceID)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putName(t, srv, session, practiceID, "  "+newName+"  ")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicename.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Name != newName {
		t.Fatalf("response name = %q, want the trimmed %q", out.Name, newName)
	}
	if got := storedName(t, db, practiceID); got != newName {
		t.Fatalf("stored name = %q, want %q", got, newName)
	}

	var actor string
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity WHERE practice_id = $1 AND action = 'practice_name_changed'`, practiceID,
	).Scan(&actor, &diff); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actor != staffID {
		t.Fatalf("actor = %q, want %q", actor, staffID)
	}
	var recorded struct {
		Before string `json:"nameBefore"`
		After  string `json:"nameAfter"`
	}
	if err := json.Unmarshal(diff, &recorded); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if recorded.Before != before || recorded.After != newName {
		t.Fatalf("diff = %+v, want %q -> %q", recorded, before, newName)
	}
}

func TestPutHandler_ResendingTheSameNameRecordsNothingNew(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-idempotent"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	for range 2 {
		resp := putName(t, srv, session, practiceID, newName)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	}
	if n := activityCount(t, db, practiceID); n != 1 {
		t.Fatalf("activity rows = %d, want 1", n)
	}
}

func TestPutHandler_RefusesAnEmptyName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		t.Run("name "+name, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "name-empty"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
			before := storedName(t, db, practiceID)

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			resp := putName(t, srv, session, practiceID, name)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			var out struct {
				Details map[string]string `json:"details"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode refusal: %v", err)
			}
			if out.Details["name"] != "Enter the name of your Practice" {
				t.Fatalf("details = %v, want the name field's sentence", out.Details)
			}
			if got := storedName(t, db, practiceID); got != before {
				t.Fatalf("stored name = %q, want it untouched", got)
			}
		})
	}
}

func TestPutHandler_RefusesABodyItCannotRead(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-bad-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putRaw(t, srv, session, practiceID, []byte("{not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutHandler_OnlyAnOwnerReachesIt is the AC's refusal: the BFF, not
// the screen, turns an Admin and a Doula away, and writes nothing.
func TestPutHandler_OnlyAnOwnerReachesIt(t *testing.T) {
	for _, tc := range []struct {
		role       string
		wantStatus int
	}{
		{ownerRole, http.StatusOK},
		{adminRole, http.StatusForbidden},
		{doulaRole, http.StatusForbidden},
	} {
		t.Run(tc.role, func(t *testing.T) {
			db := testdb.New(t)
			uid := "name-role-" + tc.role
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{tc.role}, employeeType)
			before := storedName(t, db, practiceID)

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			resp := putName(t, srv, session, practiceID, newName)
			_ = resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status for %s = %d, want %d", tc.role, resp.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus != http.StatusOK {
				if got := storedName(t, db, practiceID); got != before {
					t.Fatalf("stored name = %q, want a refusal to write nothing", got)
				}
				if n := activityCount(t, db, practiceID); n != 0 {
					t.Fatalf("activity rows = %d, want 0", n)
				}
			}
		})
	}
}

func TestPutHandler_AnotherPracticesNameIsOutOfReach(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-cross-practice"
	_, _ = testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	otherID := testdb.SeedPractice(t, db, "Someone Else's Practice")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putName(t, srv, session, otherID, newName)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("status = %d, want a refusal", resp.StatusCode)
	}
	if got := storedName(t, db, otherID); got != "Someone Else's Practice" {
		t.Fatalf("other Practice's name = %q, want it untouched", got)
	}
}

// TestPutHandler_OnlyAUnsentContractTakesTheNewName is the Contract AC:
// a Draft that copied the old name moves to the new one, a Draft whose
// name Staff already typed over keeps what she wrote, and a Contract that
// was sent keeps the name its Client saw.
func TestPutHandler_OnlyAnUnsentContractTakesTheNewName(t *testing.T) {
	db := testdb.New(t)
	const uid = "name-contracts"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	old := storedName(t, db, practiceID)

	insert := func(status, mergeName string) string {
		_, engagementID := testdb.SeedEngagement(t, db, practiceID)
		var id string
		if err := db.Admin.QueryRowContext(t.Context(),
			`INSERT INTO contracts (engagement_id, status, prose, merge_field_values, amount_cents)
			 VALUES ($1, $2::contract_status, 'Between {{practice_name}}', jsonb_build_object('practice_name', $3::text), 15000)
			 RETURNING id`,
			engagementID, status, mergeName,
		).Scan(&id); err != nil {
			t.Fatalf("seed contract: %v", err)
		}
		return id
	}
	draft := insert("draft", old)
	typedOver := insert("draft", "Staff wrote this")
	sent := insert("sent", old)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putName(t, srv, session, practiceID, newName)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	for id, want := range map[string]string{draft: newName, typedOver: "Staff wrote this", sent: old} {
		var got string
		if err := db.Admin.QueryRowContext(t.Context(),
			`SELECT merge_field_values ->> 'practice_name' FROM contracts WHERE id = $1`, id,
		).Scan(&got); err != nil {
			t.Fatalf("read contract: %v", err)
		}
		if got != want {
			t.Fatalf("contract %s practice_name = %q, want %q", id, got, want)
		}
	}
}
