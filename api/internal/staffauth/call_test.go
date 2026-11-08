package staffauth_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

type callEcho struct {
	PracticeID   string `json:"practiceId"`
	StaffID      string `json:"staffId"`
	EngagementID string `json:"engagementId"`
	ClientID     string `json:"clientId"`
	StaffPathID  string `json:"staffPathId"`
	HasTx        bool   `json:"hasTx"`
	IsOwner      bool   `json:"isOwner"`
}

func echo(w http.ResponseWriter, _ *http.Request, c staffauth.Call) {
	out := callEcho{
		PracticeID: c.PracticeID, StaffID: c.StaffID,
		EngagementID: c.EngagementID, ClientID: c.ClientID,
		HasTx: c.Tx != nil, IsOwner: c.Reader.Has(ownerRole),
	}
	_ = json.MarshalWrite(w, out)
}

func TestHandler_HandsOverWhatMiddlewareResolvedAndGatesTheDeclaredIDs(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Call Practice")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)
	otherPracticeID := testdb.SeedPractice(t, db, "Call Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	const (
		ownerUID      = "call-owner"
		attachedUID   = "call-contractor-attached"
		unattachedUID = "call-contractor-unattached"
	)
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")
	attachedID := testdb.SeedStaffAtPractice(t, db, practiceID, attachedUID, []string{doulaRole}, "contractor")
	testdb.SeedStaffAtPractice(t, db, practiceID, unattachedUID, []string{doulaRole}, "contractor")
	testdb.SeedAttachment(t, db, engagementID, attachedID, "granted", false)

	mux := http.NewServeMux()
	mw := staffauth.Middleware(db.App)
	base := "GET /practices/{practiceId}"
	mux.Handle(base+"/bare", mw(staffauth.Handler(echo)))
	mux.Handle(base+"/engagements/{engagementId}/id", mw(staffauth.Handler(echo, staffauth.EngagementID())))
	mux.Handle(base+"/engagements/{engagementId}/at-practice", mw(staffauth.Handler(echo, staffauth.Engagement())))
	mux.Handle(base+"/engagements/{engagementId}/reach", mw(staffauth.Handler(echo, staffauth.EngagementReach(gateNotFound))))
	mux.Handle(base+"/clients/{clientId}", mw(staffauth.Handler(echo, staffauth.Client())))
	mux.Handle(base+"/engagements/{engagementId}/staff/{staffId}", mw(staffauth.Handler(
		func(w http.ResponseWriter, _ *http.Request, c staffauth.Call) {
			_ = json.MarshalWrite(w, callEcho{EngagementID: c.EngagementID, StaffPathID: c.ID("staffId")})
		},
		staffauth.EngagementID(), staffauth.PathID("staff", "staffId"))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	get := func(t *testing.T, uid, path string) (int, callEcho, string) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, callEcho{}, apierrtest.Decode(t, resp).Message
		}
		var got callEcho
		if err := json.UnmarshalRead(resp.Body, &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp.StatusCode, got, ""
	}
	enc := "/engagements/" + engagementID

	t.Run("bare handler gets tx, practice, actor and reader", func(t *testing.T) {
		status, got, _ := get(t, ownerUID, "/bare")
		if status != http.StatusOK || !got.HasTx || !got.IsOwner || got.PracticeID != practiceID || got.StaffID != ownerID {
			t.Fatalf("status %d, call %+v; want 200, a tx, owner %s at %s", status, got, ownerID, practiceID)
		}
	})
	t.Run("EngagementID parses and sets the id", func(t *testing.T) {
		status, got, _ := get(t, ownerUID, enc+"/id")
		if status != http.StatusOK || got.EngagementID != engagementID {
			t.Fatalf("status %d, engagement %q; want 200, %q", status, got.EngagementID, engagementID)
		}
	})
	t.Run("EngagementID leaves a foreign Engagement to the handler", func(t *testing.T) {
		if status, _, _ := get(t, ownerUID, "/engagements/"+otherEngagementID+"/id"); status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
	})
	t.Run("a malformed id is the 400 ParseUUID writes", func(t *testing.T) {
		status, _, msg := get(t, ownerUID, "/engagements/not-a-uuid/id")
		if status != http.StatusBadRequest || msg != "invalid engagement id" {
			t.Fatalf("status %d, error %q; want 400, invalid engagement id", status, msg)
		}
	})
	t.Run("Engagement refuses one at another Practice", func(t *testing.T) {
		if status, _, _ := get(t, ownerUID, "/engagements/"+otherEngagementID+"/at-practice"); status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
		if status, got, _ := get(t, unattachedUID, enc+"/at-practice"); status != http.StatusOK || got.EngagementID != engagementID {
			t.Fatalf("status %d, call %+v; want 200 for an unattached contractor", status, got)
		}
	})
	t.Run("EngagementReach narrows to an attached contractor", func(t *testing.T) {
		if status, got, _ := get(t, attachedUID, enc+"/reach"); status != http.StatusOK || got.EngagementID != engagementID {
			t.Fatalf("attached: status %d, call %+v; want 200", status, got)
		}
		status, _, msg := get(t, unattachedUID, enc+"/reach")
		if status != http.StatusNotFound || msg != gateNotFound {
			t.Fatalf("unattached: status %d, error %q; want 404 %q", status, msg, gateNotFound)
		}
		if status, _, _ := get(t, ownerUID, "/engagements/"+otherEngagementID+"/reach"); status != http.StatusNotFound {
			t.Fatalf("other Practice: status = %d, want 404", status)
		}
	})
	t.Run("Client gates on the Reader's reach", func(t *testing.T) {
		if status, got, _ := get(t, attachedUID, "/clients/"+clientID); status != http.StatusOK || got.ClientID != clientID {
			t.Fatalf("attached: status %d, call %+v; want 200", status, got)
		}
		status, _, msg := get(t, unattachedUID, "/clients/"+clientID)
		if status != http.StatusNotFound || msg != staffauth.MsgClientNotFound {
			t.Fatalf("unattached: status %d, error %q; want 404 %q", status, msg, staffauth.MsgClientNotFound)
		}
		status, _, msg = get(t, ownerUID, "/clients/not-a-uuid")
		if status != http.StatusBadRequest || msg != "invalid client id" {
			t.Fatalf("malformed: status %d, error %q; want 400 invalid client id", status, msg)
		}
	})
	t.Run("PathID exposes a declared wildcard and refuses a malformed one", func(t *testing.T) {
		status, got, _ := get(t, ownerUID, enc+"/staff/"+ownerID)
		if status != http.StatusOK || got.StaffPathID != ownerID || got.EngagementID != engagementID {
			t.Fatalf("status %d, call %+v; want 200 with staff %s", status, got, ownerID)
		}
		status, _, msg := get(t, ownerUID, enc+"/staff/nope")
		if status != http.StatusBadRequest || msg != "invalid staff id" {
			t.Fatalf("status %d, error %q; want 400 invalid staff id", status, msg)
		}
	})
}

func TestHandler_AnswersInternalErrorWhenMiddlewareNeverRan(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	staffauth.Handler(func(http.ResponseWriter, *http.Request, staffauth.Call) {
		t.Fatal("handler ran without Middleware")
	}).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCall_IDPanicsOnAnUndeclaredWildcard(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a wildcard no PathID declared")
		}
	}()
	staffauth.Call{}.ID("staffId")
}
