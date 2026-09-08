package visit_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// assigneesURL is the one place the endpoint's path is spelled, so a
// case reads as "this Practice, this Engagement" rather than as string
// concatenation.
func assigneesURL(baseURL, practiceID, engagementID string) string {
	return baseURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/visit-assignees"
}

// getAssignees calls the read under session and reports the status, with
// the list decoded when there was one. Both halves live in one function
// because the body has to be closed either way.
func getAssignees(t *testing.T, session, url string) (int, map[string]visit.Assignee) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		// coverage:ignore reason: request construction failure, not exercised by these tests
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// coverage:ignore reason: transport failure against an in-process server, not exercised by these tests
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	var out visit.AssigneesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		// coverage:ignore reason: decode failure of a response this handler always writes as JSON
		t.Fatalf("decode response: %v", err)
	}
	// Keyed by staff id so a case can name the one row it is about rather
	// than counting positions.
	byID := map[string]visit.Assignee{}
	for _, row := range out.Items {
		byID[row.StaffID] = row
	}
	return resp.StatusCode, byID
}

// decodeAssignees is getAssignees where the case expects a 200 and cares
// only about the rows.
func decodeAssignees(t *testing.T, session, url string) map[string]visit.Assignee {
	t.Helper()
	status, rows := getAssignees(t, session, url)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	return rows
}

// TestVisitAssignees_MarksWhoMayBeNamedOnThisEngagement is #911's whole
// point in one case: the same Practice roster answers differently per
// Engagement, and nobody is dropped from it. The employee is nameable
// because an Admin may attach her directly; the unattached contractor is
// not, and says why; the contractor holding an open granted attachment
// is; and the contractor whose attachment was *ended* -- what Engagement
// completion does -- is not, which is exactly the row a screen deriving
// eligibility from the Offers list would have got wrong.
func TestVisitAssignees_MarksWhoMayBeNamedOnThisEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "assignees-owner", []string{ownerRole}, "employee")
	employeeID := testdb.SeedStaffAtPractice(t, db, practiceID, "assignees-employee", []string{doulaRole}, "employee")
	unattachedID := testdb.SeedContractorAtPractice(t, db, practiceID, "assignees-unattached")
	attachedID := testdb.SeedContractorAtPractice(t, db, practiceID, "assignees-attached")
	endedID := testdb.SeedContractorAtPractice(t, db, practiceID, "assignees-ended")
	accruedID := testdb.SeedContractorAtPractice(t, db, practiceID, "assignees-accrued")
	bookkeeperID := testdb.SeedStaffAtPractice(t, db, practiceID, "assignees-bookkeeper", []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedGrantedAttachment(t, db, engagementID, attachedID)
	testdb.SeedAttachment(t, db, engagementID, endedID, grantedOrigin, true)
	testdb.SeedAttachment(t, db, engagementID, accruedID, "accrued", false)

	srv, session := newServer(t, db, "assignees-owner")
	defer srv.Close()

	rows := decodeAssignees(t, session, assigneesURL(srv.URL, practiceID, engagementID))

	for _, want := range []struct {
		label    string
		staffID  string
		nameable bool
		reason   string
	}{
		{"an employee Doula", employeeID, true, ""},
		{"a contractor with no attachment", unattachedID, false, visit.ReasonContractorWithoutAcceptedOffer},
		{"a contractor with an open granted attachment", attachedID, true, ""},
		{"a contractor whose granted attachment was ended", endedID, false, visit.ReasonContractorWithoutAcceptedOffer},
		{"a contractor with only an accrued attachment", accruedID, false, visit.ReasonContractorWithoutAcceptedOffer},
	} {
		row, listed := rows[want.staffID]
		if !listed {
			t.Fatalf("%s: not listed at all -- #911 marks her, it does not drop her", want.label)
		}
		if row.Nameable != want.nameable || row.Reason != want.reason {
			t.Errorf("%s: nameable = %v, reason = %q; want %v, %q",
				want.label, row.Nameable, row.Reason, want.nameable, want.reason)
		}
		if row.Name == "" {
			t.Errorf("%s: name is empty -- the picker offers a person by name", want.label)
		}
	}

	// The Owner here holds no Doula role, so she is not somebody a Visit
	// can be put on, and neither is the bookkeeper.
	for label, staffID := range map[string]string{"the Owner, who is not a Doula": ownerID, "the bookkeeper": bookkeeperID} {
		if _, listed := rows[staffID]; listed {
			t.Errorf("%s is in the picker; only Doula-role Memberships belong there", label)
		}
	}
}

// TestVisitAssignees_CallerIsNameableAsHerself covers the row the
// named-colleague rule must not be applied to. assignee (roles.go) takes
// the self path whenever the requested id equals the caller's own, so a
// contractor Doula who owns the Practice is accepted by the write when
// she names herself with no attachment at all -- and the read has to say
// so, or the screen would block a write the BFF allows.
func TestVisitAssignees_CallerIsNameableAsHerself(t *testing.T) {
	db := testdb.New(t)
	practiceID, callerID := testdb.SeedStaffAtNewPractice(t, db, "assignees-self",
		[]string{ownerRole, doulaRole}, "contractor")
	otherID := testdb.SeedContractorAtPractice(t, db, practiceID, "assignees-self-other")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, "assignees-self")
	defer srv.Close()

	rows := decodeAssignees(t, session, assigneesURL(srv.URL, practiceID, engagementID))

	if self := rows[callerID]; !self.Nameable || self.Reason != "" {
		t.Errorf("the caller's own row: nameable = %v, reason = %q; want true, \"\" -- she needs no attachment to log her own Visit",
			self.Nameable, self.Reason)
	}
	// The same employment type, the same absent attachment, a different
	// person: the rule that applies to her is the stricter one.
	if other := rows[otherID]; other.Nameable {
		t.Error("another unattached contractor is nameable; only the caller's own row takes the self rule")
	}
}

// TestVisitAssignees_RefusesAnEngagementAtAnotherPractice proves the
// Engagement is confirmed to be this Practice's before any roster is
// answered -- otherwise a guessed id would answer 200 with a whole
// Practice's Doulas and their attachment state.
func TestVisitAssignees_RefusesAnEngagementAtAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "assignees-outsider", []string{ownerRole, doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Another Practice")
	_, foreignEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newServer(t, db, "assignees-outsider")
	defer srv.Close()

	if status, _ := getAssignees(t, session, assigneesURL(srv.URL, practiceID, foreignEngagementID)); status != http.StatusNotFound {
		t.Errorf("another Practice's Engagement: status = %d, want %d", status, http.StatusNotFound)
	}
	if status, _ := getAssignees(t, session, assigneesURL(srv.URL, practiceID, notAUUID)); status != http.StatusBadRequest {
		t.Errorf("a malformed engagement id: status = %d, want %d", status, http.StatusBadRequest)
	}
}

// TestVisitAssignees_RefusesAPlainDoula holds the endpoint to the
// audience of the roster read the pickers used before it: naming a
// colleague is scheduling (ADR-0006, ADR-0008), and a plain Doula has no
// roster to pick one from.
func TestVisitAssignees_RefusesAPlainDoula(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "assignees-doula", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, "assignees-doula")
	defer srv.Close()

	if status, _ := getAssignees(t, session, assigneesURL(srv.URL, practiceID, engagementID)); status != http.StatusForbidden {
		t.Errorf("a plain Doula: status = %d, want %d", status, http.StatusForbidden)
	}
}
