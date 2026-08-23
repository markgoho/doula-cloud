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
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, birthPlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

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
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID, _ := seedContractorAtPractice(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID, staffID := seedContractorAtPractice(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`,
		`{"f1":"Jamie"}`,
	)
	seedGrantedAttachment(t, db, engagementID, staffID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
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

// TestPlanTemplateEditDoesNotMutateExistingInstance is the AC's centerpiece:
// editing a Practice's Plan Template after a Plan Instance already exists
// must never change that instance's stored field snapshot.
func TestPlanTemplateEditDoesNotMutateExistingInstance(t *testing.T) {
	db := testdb.New(t)
	const uid = "snapshot-owner"
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

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
