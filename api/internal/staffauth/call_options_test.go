package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	gateNotFound = "gate: engagement not found"
	ownerPath    = "/owner"
	moneyPath    = "/money"
)

const wantOwnerRefusal = "You need to be an owner of this Practice to do that."

func noContent(w http.ResponseWriter, _ *http.Request, _ staffauth.Call) {
	w.WriteHeader(http.StatusNoContent)
}

// TestHandler_RoleOptionsRefuseWithTheMountsWording covers Owner and
// NotAmbientContractor through the real Middleware: the seat each one admits,
// and the 403 sentence each writes.
func TestHandler_RoleOptionsRefuseWithTheMountsWording(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Call Options Practice")

	const (
		ownerUID      = "opt-owner"
		adminUID      = "opt-admin"
		employeeUID   = "opt-employee"
		contractorUID = "opt-contractor"
	)
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, adminUID, []string{adminRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, employeeUID, []string{doulaRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, contractorUID, []string{doulaRole}, "contractor")

	mux := http.NewServeMux()
	mw := staffauth.Middleware(db.App)
	mux.Handle("GET /practices/{practiceId}"+ownerPath, mw(staffauth.Handler(noContent, staffauth.Owner())))
	mux.Handle("GET /practices/{practiceId}"+moneyPath, mw(staffauth.Handler(noContent, staffauth.NotAmbientContractor())))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		name, uid, path string
		want            int
		wantMessage     string
	}{
		{"owner admits the owner", ownerUID, ownerPath, http.StatusNoContent, ""},
		{"owner refuses an admin", adminUID, ownerPath, http.StatusForbidden, wantOwnerRefusal},
		{"owner refuses a doula", employeeUID, ownerPath, http.StatusForbidden, wantOwnerRefusal},
		{"money admits the owner", ownerUID, moneyPath, http.StatusNoContent, ""},
		{"money admits an admin", adminUID, moneyPath, http.StatusNoContent, ""},
		{"money admits an employed doula", employeeUID, moneyPath, http.StatusNoContent, ""},
		{"money refuses a contractor", contractorUID, moneyPath, http.StatusForbidden, staffauth.MsgContractorMoneyRefused},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := reqtest.Get(t, srv.URL+"/practices/"+practiceID+tc.path, reqtest.Session(authntest.SeedSession(t, db.App, tc.uid)))
			defer func() { _ = resp.Body.Close() }()
			reqtest.RequireStatus(t, resp, tc.want)
			if tc.wantMessage != "" {
				if got := apierrtest.Decode(t, resp).Message; got != tc.wantMessage {
					t.Fatalf("message = %q, want %q", got, tc.wantMessage)
				}
			}
		})
	}
}

// TestHandler_PathValueExposesAnAcceptedValueAndRefusesTheRest covers a
// wildcard that is not a UUID: the validator decides, the refusal text is the
// handler's own, and an accepted value reads back through Call.ID.
func TestHandler_PathValueExposesAnAcceptedValueAndRefusesTheRest(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Call PathValue Practice")
	const uid = "pathvalue-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{ownerRole}, "employee")

	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/kinds/{kind}", staffauth.Middleware(db.App)(staffauth.Handler(
		func(w http.ResponseWriter, _ *http.Request, c staffauth.Call) {
			w.Header().Set("X-Kind", c.ID("kind"))
			w.WriteHeader(http.StatusNoContent)
		},
		staffauth.PathValue("kind", func(v string) bool { return v == "care" },
			func(v string) string { return "unknown kind: " + v }))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	session := reqtest.Session(authntest.SeedSession(t, db.App, uid))

	ok := reqtest.Get(t, srv.URL+"/practices/"+practiceID+"/kinds/care", session)
	defer func() { _ = ok.Body.Close() }()
	reqtest.RequireStatus(t, ok, http.StatusNoContent)
	if got := ok.Header.Get("X-Kind"); got != "care" {
		t.Fatalf("Call.ID(kind) = %q, want care", got)
	}

	bad := reqtest.Get(t, srv.URL+"/practices/"+practiceID+"/kinds/other", session)
	defer func() { _ = bad.Body.Close() }()
	reqtest.RequireStatus(t, bad, http.StatusBadRequest)
	if got := apierrtest.Decode(t, bad).Message; got != "unknown kind: other" {
		t.Fatalf("message = %q, want unknown kind: other", got)
	}
}
