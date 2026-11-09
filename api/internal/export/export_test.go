package export_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

const (
	doulaRole = fixture.RoleDoula
	adminRole = fixture.RoleAdmin
)

// TestHandler_OwnerGetsHerWholeArchive is the central acceptance
// criterion: an Owner downloads a ZIP of UTF-8 CSVs whose rows carry the
// ids needed to join them back together.
func TestHandler_OwnerGetsHerWholeArchive(t *testing.T) {
	e := newEnv(t, fixture.WithClient(fixture.ClientName("Ada Lovelace"), fixture.ClientEmail("ada@example.com")))
	clientID := e.Engagement.Client.ID

	resp := reqtest.Do(t, http.MethodGet, e.exportURL(), e.SignedIn)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want application/zip", got)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), ".zip") {
		t.Fatalf("Content-Disposition = %q, want a .zip filename", resp.Header.Get("Content-Disposition"))
	}

	files := readZIP(t, reqtest.ReadBody(t, resp))
	for _, want := range []string{"README.txt", "practice.csv", "client.csv", "engagement.csv", "activity.csv"} {
		if _, ok := files[want]; !ok {
			t.Fatalf("archive missing %s; have %v", want, keys(files))
		}
	}

	client := onlyRow(t, "client.csv", parseCSV(t, files["client.csv"]))
	if got := client.get("id"); got != clientID {
		t.Fatalf("client.csv id = %q, want %q", got, clientID)
	}

	engagement := onlyRow(t, "engagement.csv", parseCSV(t, files["engagement.csv"]))
	if got := engagement.get("client_id"); got != clientID {
		t.Fatalf("engagement.csv client_id = %q, want %q (the join key back to client.csv)", got, clientID)
	}

	// Exactly one Activity row, subject the Practice, actor the Owner.
	activity := onlyRow(t, "activity.csv", parseCSV(t, files["activity.csv"]))
	if activity.get("subject_kind") != "practice" || activity.get("subject_id") != e.Practice.ID {
		t.Fatalf("activity row = %v, want subject practice/%s", activity.cells, e.Practice.ID)
	}
	if got := activity.get("action"); got != "practice_data_exported" {
		t.Fatalf("activity action = %q, want practice_data_exported", got)
	}
}

// TestHandler_RefusesEveryRoleButOwner is the second acceptance
// criterion: a non-Owner Staff member of the same Practice is refused at
// the API with a coded error. Table-driven over both non-Owner roles,
// the same shape client.TestEraseHandler_RefusesEveryRoleButOwner uses
// for the same reasoning -- Admin is the near-miss role most other
// Practice-wide reads admit (staffauth.OwnerAndAdmin), so it gets its
// own row rather than trusting doulaRole alone to stand in for it.
func TestHandler_RefusesEveryRoleButOwner(t *testing.T) {
	for _, role := range []string{doulaRole, adminRole} {
		t.Run(role, func(t *testing.T) {
			e := newEnv(t, fixture.SignedInAs(fixture.Roles(role)))

			resp := reqtest.Do(t, http.MethodGet, e.exportURL(), e.SignedIn)
			defer resp.Body.Close()
			reqtest.RequireStatus(t, resp, http.StatusForbidden)
			if apiErr := apierrtest.Decode(t, resp); apiErr.Code != apierr.CodeForbidden {
				t.Fatalf("code = %q, want %q", apiErr.Code, apierr.CodeForbidden)
			}
		})
	}
}

// TestHandler_NeverCrossesIntoAnotherPractice is the third acceptance
// criterion: a Staff member of a different Practice is refused, and no
// row belonging to another Practice appears in any file.
func TestHandler_NeverCrossesIntoAnotherPractice(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement())
	otherPractice := fixture.Practice(t, e.DB)
	fixture.Client(t, e.DB, otherPractice, fixture.ClientName("Someone Else"), fixture.ClientEmail("else@example.com"))
	otherOwner := fixture.Staff(t, e.DB, otherPractice, fixture.Owner())

	// The other Practice's Owner cannot reach this Practice's export at all.
	reqtest.ExpectStatus(t, http.MethodGet, e.exportURL(), http.StatusForbidden, e.As(otherOwner))

	// This Practice's own export names only her own Client, never the
	// other Practice's.
	clientCSV := e.archive()["client.csv"]
	if strings.Contains(clientCSV, "Someone Else") || strings.Contains(clientCSV, "else@example.com") {
		t.Fatalf("client.csv leaked another Practice's Client: %s", clientCSV)
	}
}
