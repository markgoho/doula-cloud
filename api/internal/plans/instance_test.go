package plans_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

const nameFieldID = "name"
const jamieAnswer = "Jamie"

func TestPostInstanceHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, "not-a-uuid", carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostInstanceHandler_UnknownPlanType(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-unknown-plan-type"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, engagementID, "not_a_plan_type")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostInstanceHandler_EngagementNotFound proves an Engagement id from a
// different Practice (or one that doesn't exist) 404s rather than creating
// a Plan Instance under it.
func TestPostInstanceHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, otherEngagementID, carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostInstanceHandler_NoTemplate proves the "should not happen post-#63"
// missing-template case fails predictably with 404, not a crash or a
// silently-empty snapshot.
func TestPostInstanceHandler_NoTemplate(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-template"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostInstanceHandler_Success proves a Plan Instance is created with a
// snapshot of the Practice's current template fields and empty Answers --
// and that any Staff member (not just an Owner) can create one, unlike
// PutTemplateHandler.
func TestPostInstanceHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedPlanTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID || out.PlanType != carePlanType {
		t.Fatalf("out = %+v, want engagementId %q planType %q", out, engagementID, carePlanType)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want the template's one field", out.Fields)
	}
	if len(out.Answers) != 0 {
		t.Fatalf("answers = %+v, want empty", out.Answers)
	}
}

// TestPostInstanceHandler_Duplicate proves a second POST for the same
// Engagement + plan type 409s -- POST creates, PUT edits.
func TestPostInstanceHandler_Duplicate(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-duplicate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedPlanTemplate(t, db, practiceID, birthPlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	first := postInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first POST status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second POST status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}

func TestGetInstanceHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, "not-a-uuid", carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetInstanceHandler_UnknownPlanType(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-unknown-plan-type"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, "not_a_plan_type")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetInstanceHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-no-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, otherEngagementID, carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetInstanceHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInstanceHandler_ContractorWithoutAttachmentForbidden proves
// ADR-0008's attachment rule for Plan Instances: a contractor Doula with
// no engagement_attachments row gets the same "not found" response an
// out-of-practice Engagement gets.
func TestGetInstanceHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`,
		`{"f1":"Jamie"}`,
	)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInstanceHandler_ContractorWithGrantedAttachmentSucceeds proves
// the other half: an open, granted attachment reaches.
func TestGetInstanceHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-contractor-attached"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`,
		`{"f1":"Jamie"}`,
	)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestGetInstanceHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`,
		`{"f1":"Jamie"}`,
	)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want the stored snapshot", out.Fields)
	}
	if out.Answers["f1"] != jamieAnswer {
		t.Fatalf("answers = %+v, want f1=Jamie", out.Answers)
	}
}

func TestPutInstanceHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-invalid-engagement-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, "not-a-uuid", carePlanType, plans.PutInstanceRequest{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutInstanceHandler_UnknownPlanType(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-unknown-plan-type"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, engagementID, "not_a_plan_type", plans.PutInstanceRequest{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutInstanceHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-no-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, otherEngagementID, carePlanType, plans.PutInstanceRequest{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPutInstanceHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, engagementID, carePlanType, plans.PutInstanceRequest{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPutInstanceHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{}`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstanceRaw(t, srv, session, practiceID, engagementID, carePlanType, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutInstanceHandler_ValidationRejections table-drives validateAnswers's
// error branches: an answer for an unknown field id, an answer on a
// section_header, a non-boolean checkbox answer, a single-select value not
// among its options, a non-array multi-select answer, a multi-select entry
// not among its options, and a non-string short_text answer.
func TestPutInstanceHandler_ValidationRejections(t *testing.T) {
	const fieldsJSON = `[
		{"id":"name","type":"short_text","label":"Name","order":0},
		{"id":"heading","type":"section_header","label":"Section","order":1},
		{"id":"consent","type":"checkbox","label":"Consent","order":2},
		{"id":"location","type":"single_select","label":"Location","options":["Home","Hospital"],"order":3},
		{"id":"notify","type":"multi_select","label":"Notify","options":["Partner","Doula"],"order":4}
	]`

	cases := []struct {
		name    string
		answers plans.Answers
	}{
		{"unknown field id", plans.Answers{"nope": "x"}},
		{"answer on section_header", plans.Answers{"heading": "x"}},
		{"non-boolean checkbox", plans.Answers{"consent": "yes"}},
		{"single_select not among options", plans.Answers{"location": "Car"}},
		{"non-array multi_select", plans.Answers{"notify": "Partner"}},
		{"multi_select entry not among options", plans.Answers{"notify": []any{"Partner", "Neighbor"}}},
		{"non-string short_text", plans.Answers{nameFieldID: 5}},
	}

	db := testdb.New(t)
	const uid = "put-instance-validation"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedInstance(t, db, engagementID, carePlanType, fieldsJSON, `{}`)

			resp := putInstance(t, srv, session, practiceID, engagementID, carePlanType, plans.PutInstanceRequest{Answers: tc.answers})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutInstanceHandler_OmittedAnswersNormalizesToEmptyObject proves a PUT
// body with no "answers" key (decoding to a nil map) still round-trips as
// `{}`, not JSON null, into both the stored column and the response --
// nonEmpty's nil-normalization path.
func TestPutInstanceHandler_OmittedAnswersNormalizesToEmptyObject(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-omitted-answers"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{}`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstanceRaw(t, srv, session, practiceID, engagementID, carePlanType, []byte(`{}`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Answers == nil || len(out.Answers) != 0 {
		t.Fatalf("answers = %+v, want a non-nil empty object", out.Answers)
	}
}

// TestPutInstanceHandler_Success proves a full Answers replacement
// round-trips through GET, that the Fields snapshot is unchanged by the
// PUT, and that a second PUT with a smaller Answers map wipes the answer
// left out of it -- PUT is a full replacement, not a merge.
func TestPutInstanceHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, carePlanType,
		`[{"id":"name","type":"short_text","label":"Name","order":0},{"id":"consent","type":"checkbox","label":"Consent","order":1}]`,
		`{}`,
	)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	firstResp := putInstance(t, srv, session, practiceID, engagementID, carePlanType, plans.PutInstanceRequest{
		Answers: plans.Answers{nameFieldID: jamieAnswer, "consent": true},
	})
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first PUT status = %d, want %d", firstResp.StatusCode, http.StatusOK)
	}
	var firstOut plans.InstanceResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&firstOut); err != nil {
		t.Fatalf("decode first PUT response: %v", err)
	}
	if len(firstOut.Fields) != 2 {
		t.Fatalf("fields = %+v, want the unchanged 2-field snapshot", firstOut.Fields)
	}
	if firstOut.Answers[nameFieldID] != jamieAnswer || firstOut.Answers["consent"] != true {
		t.Fatalf("answers = %+v, want name=Jamie consent=true", firstOut.Answers)
	}

	getResp := getInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer getResp.Body.Close()
	var getOut plans.InstanceResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Answers[nameFieldID] != jamieAnswer || getOut.Answers["consent"] != true {
		t.Fatalf("GET answers after PUT = %+v, want the just-written answers", getOut.Answers)
	}

	secondResp := putInstance(t, srv, session, practiceID, engagementID, carePlanType, plans.PutInstanceRequest{
		Answers: plans.Answers{nameFieldID: jamieAnswer},
	})
	defer secondResp.Body.Close()
	var secondOut plans.InstanceResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&secondOut); err != nil {
		t.Fatalf("decode second PUT response: %v", err)
	}
	if _, stillPresent := secondOut.Answers["consent"]; stillPresent {
		t.Fatalf("answers = %+v, want consent dropped by the full-replacement PUT", secondOut.Answers)
	}
}

// TestPutInstanceHandler_RecordsWhichPlanChanged proves #2066 at the write
// side: a save records one action per plan, so the Practice's feed names
// the plan and the portal reader can leave the staff-only Care Plan out by
// action alone.
func TestPutInstanceHandler_RecordsWhichPlanChanged(t *testing.T) {
	for _, tc := range []struct {
		planType string
		want     string
	}{
		{carePlanType, "care_plan_edited"},
		{birthPlanType, "birth_plan_edited"},
	} {
		t.Run(tc.planType, func(t *testing.T) {
			db := testdb.New(t)
			uid := "put-instance-action-" + tc.planType
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedInstance(t, db, engagementID, tc.planType,
				`[{"id":"name","type":"short_text","label":"Name","order":0}]`, `{}`)

			srv, session := newPlanServer(t, db, uid)
			defer srv.Close()

			resp := putInstance(t, srv, session, practiceID, engagementID, tc.planType, plans.PutInstanceRequest{
				Answers: plans.Answers{nameFieldID: jamieAnswer},
			})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("PUT status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			var action string
			if err := db.Admin.QueryRowContext(t.Context(),
				`SELECT action FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1`,
				engagementID,
			).Scan(&action); err != nil {
				t.Fatalf("query activity: %v", err)
			}
			if action != tc.want {
				t.Fatalf("activity action = %q, want %q", action, tc.want)
			}
		})
	}
}

// TestInstance_LastEdited is #2104: a Plan that has never been saved says
// no edit, a save's own response names when and by whom, and a read of
// the other Plan does not borrow this one's edit.
func TestInstance_LastEdited(t *testing.T) {
	db := testdb.New(t)
	const uid = "instance-last-edited"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	const fields = `[{"id":"name","type":"short_text","label":"Name","order":0}]`
	seedInstance(t, db, engagementID, carePlanType, fields, `{}`)
	seedInstance(t, db, engagementID, birthPlanType, fields, `{}`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	read := func(resp *http.Response) plans.InstanceResponse {
		t.Helper()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var out plans.InstanceResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	if started := read(getInstance(t, srv, session, practiceID, engagementID, carePlanType)); started.LastEdited != nil {
		t.Fatalf("a Plan never saved: lastEdited = %+v, want none", started.LastEdited)
	}

	saved := read(putInstance(t, srv, session, practiceID, engagementID, carePlanType, plans.PutInstanceRequest{
		Answers: plans.Answers{nameFieldID: jamieAnswer},
	}))
	if saved.LastEdited == nil || saved.LastEdited.By != "Test Staff "+uid || saved.LastEdited.At.IsZero() {
		t.Fatalf("the save's response: lastEdited = %+v, want its instant, by the Staff member who saved", saved.LastEdited)
	}

	if again := read(getInstance(t, srv, session, practiceID, engagementID, carePlanType)); again.LastEdited == nil || !again.LastEdited.At.Equal(saved.LastEdited.At) {
		t.Errorf("GET after the save: lastEdited = %+v, want the save's", again.LastEdited)
	}
	if other := read(getInstance(t, srv, session, practiceID, engagementID, birthPlanType)); other.LastEdited != nil {
		t.Errorf("the Birth Plan: lastEdited = %+v, want none -- only the Care Plan was saved", other.LastEdited)
	}
}

// TestPlanTemplateEditDoesNotMutateExistingInstance is the AC's centerpiece:
// editing a Practice's Plan Template after a Plan Instance already exists
// must never change that instance's stored field snapshot.
func TestPlanTemplateEditDoesNotMutateExistingInstance(t *testing.T) {
	db := testdb.New(t)
	const uid = "snapshot-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedPlanTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	postResp := postInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST instance status = %d, want %d", postResp.StatusCode, http.StatusCreated)
	}

	putResp := putTemplate(t, srv, session, practiceID, carePlanType, plans.TemplateResponse{Fields: []plans.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
		{ID: "f2", Type: "long_text", Label: "Notes added after the plan was created"},
	}})
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT template status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	getResp := getInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer getResp.Body.Close()
	var out plans.InstanceResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("instance fields = %+v, want the original 1-field snapshot untouched by the later template edit", out.Fields)
	}
}

// TestPutInstanceHandler_AnswersChangeClearsClientAcknowledgment is
// #301's decision for the build: a Staff edit that actually changes the
// stored answers resets the Client's acknowledgment, so Staff see
// "not reviewed since last edit" until she confirms again.
func TestPutInstanceHandler_AnswersChangeClearsClientAcknowledgment(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-clears-ack"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"name","type":"short_text","label":"Name","order":0}]`, `{"name":"Jamie"}`,
	)
	setAcknowledged(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, engagementID, birthPlanType, plans.PutInstanceRequest{
		Answers: plans.Answers{nameFieldID: "Alex"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientAcknowledgedAt != nil {
		t.Fatalf("clientAcknowledgedAt = %v, want nil after an answers-changing edit", out.ClientAcknowledgedAt)
	}
}

// TestPutInstanceHandler_UnchangedAnswersKeepsClientAcknowledgment
// proves the reset is conditioned on the answers actually differing, not
// on the PUT event itself -- re-sending the same answers (a retry, or
// Staff saving without changing anything) leaves an existing
// acknowledgment in place.
func TestPutInstanceHandler_UnchangedAnswersKeepsClientAcknowledgment(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-instance-keeps-ack"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"name","type":"short_text","label":"Name","order":0}]`, `{"name":"Jamie"}`,
	)
	setAcknowledged(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := putInstance(t, srv, session, practiceID, engagementID, birthPlanType, plans.PutInstanceRequest{
		Answers: plans.Answers{nameFieldID: jamieAnswer},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientAcknowledgedAt == nil {
		t.Fatalf("clientAcknowledgedAt = nil, want the pre-existing acknowledgment kept since answers didn't change")
	}
}

// TestGetInstanceHandler_ReflectsClientAcknowledgment is the Staff
// visibility half of #301's AC4: Staff can see, on their own read, that
// the Client has reviewed her Birth Plan.
func TestGetInstanceHandler_ReflectsClientAcknowledgment(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-shows-ack"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType, `[]`, `{}`)
	setAcknowledged(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientAcknowledgedAt == nil {
		t.Fatalf("clientAcknowledgedAt = nil, want Staff's own read to show the Client's acknowledgment")
	}
}
