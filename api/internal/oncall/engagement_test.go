package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

func panelURL(srvURL, practiceID, engagementID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/on-call"
}

func narrowURL(srvURL, practiceID, engagementID, staffID string) string {
	return srvURL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/attachments/" + staffID + "/on-call"
}

func TestNarrowing_APrimaryAndABackupEachCarryTheirOwnDays(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "narrow-pair")
	backupID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "narrow-pair-backup", backupName, []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, backupID)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	to, from := octTwentyTwo, octTwentyThree
	doJSON[oncall.NarrowingRequest](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, f.doulaID), oncall.NarrowingRequest{To: &to}, http.StatusOK)
	doJSON[oncall.NarrowingRequest](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, backupID), oncall.NarrowingRequest{From: &from}, http.StatusOK)

	panel := getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK)
	days := map[string][2]string{}
	for _, d := range panel.Doulas {
		days[d.StaffID] = [2]string{*d.OnCallFrom, *d.OnCallTo}
	}
	if days[f.doulaID] != [2]string{windowStart, octTwentyTwo} || days[backupID] != [2]string{octTwentyThree, windowEnd} {
		t.Fatalf("on-call days = %v, want the primary to the 22nd and the backup from the 23rd", days)
	}
	if n := len(activityActors(t, db, f.engagementID, "on_call_narrowing_changed")); n != 2 {
		t.Fatalf("narrowing entries = %d, want 2", n)
	}

	// Clearing puts her back on the whole window.
	doJSON[oncall.NarrowingRequest](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, f.doulaID)+"?clear", oncall.NarrowingRequest{}, http.StatusOK)
	panel = getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK)
	for _, d := range panel.Doulas {
		if d.StaffID == f.doulaID && (d.From != nil || *d.OnCallTo != windowEnd) {
			t.Fatalf("after clearing, primary = %+v, want the whole window", d)
		}
	}
}

func TestNarrowing_RefusesWhatItShould(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "narrow-refuse")
	strangerID := testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, "narrow-refuse-stranger", "Not On It", []string{doulaRole}, employeeType)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	from, to, junk := "2026-10-25", octTwenty, "next week"
	body := doJSON[map[string]any](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, f.doulaID), oncall.NarrowingRequest{From: &from, To: &to}, http.StatusBadRequest)
	if details, _ := body["details"].(map[string]any); details["to"] != oncall.MsgNarrowOrder {
		t.Errorf("details = %v, want the order refused", details)
	}
	body = doJSON[map[string]any](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, f.doulaID)+"?junk", oncall.NarrowingRequest{From: &junk}, http.StatusBadRequest)
	if details, _ := body["details"].(map[string]any); details["from"] != oncall.MsgNarrowDate {
		t.Errorf("details = %v, want the date refused", details)
	}
	doJSON[map[string]any](t, session, http.MethodPut, narrowURL(srv.URL, f.practiceID, f.engagementID, strangerID), oncall.NarrowingRequest{}, http.StatusNotFound)

	doulaSrv, doulaSession := newServer(t, db, "narrow-refuse-doula")
	defer doulaSrv.Close()
	doJSON[map[string]any](t, doulaSession, http.MethodPut, narrowURL(doulaSrv.URL, f.practiceID, f.engagementID, f.doulaID), oncall.NarrowingRequest{}, http.StatusForbidden)
}

func TestPanel_AContractorReadsOnlyABirthSheIsOn(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "panel-contractor")
	const uid = "panel-contractor-contractor"
	contractorID := testdb.SeedContractorAtPractice(t, db, f.practiceID, uid)
	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, panelURL(srv.URL, f.practiceID, f.engagementID))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("an unattached contractor got %d, want 404", resp.StatusCode)
	}

	testdb.SeedGrantedAttachment(t, db, f.engagementID, contractorID)
	panel := getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK)
	if panel.Window == nil || len(panel.Doulas) != 2 {
		t.Fatalf("panel = %+v, want the window and both Doulas on her own birth", panel)
	}
}

func TestPanel_StatesWhyThereIsNoWindow(t *testing.T) {
	db := testdb.New(t)
	f := newSoloFixture(t, db, "panel-reasons")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	exec(t, db, `UPDATE engagements SET due_date = NULL WHERE id = $1`, f.engagementID)
	if panel := getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK); panel.NoWindowReason != oncall.NoWindowNoDueDate || panel.Window != nil {
		t.Fatalf("panel = %+v, want no window for want of a due date", panel)
	}

	exec(t, db, `UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, f.engagementID)
	if panel := getJSON[oncall.EngagementOnCall](t, session, panelURL(srv.URL, f.practiceID, f.engagementID), http.StatusOK); panel.NoWindowReason != oncall.NoWindowNobodyAttached {
		t.Fatalf("panel = %+v, want no window because nobody is on it", panel)
	}

	resp := authedGet(t, session, panelURL(srv.URL, f.practiceID, "00000000-0000-0000-0000-000000000000"))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("an unknown Engagement got %d, want 404", resp.StatusCode)
	}
}
