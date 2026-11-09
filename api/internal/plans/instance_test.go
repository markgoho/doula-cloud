package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestInstanceRoutes_RefuseWhatTheyCannotReach proves each of the three
// Plan Instance routes refuses an Engagement id that is not a uuid, an
// Engagement of another Practice, an unknown plan type, and a Plan that
// has nothing to act on yet: no template to snapshot for a POST, no
// instance to read or save for a GET and a PUT.
func TestInstanceRoutes_RefuseWhatTheyCannotReach(t *testing.T) {
	routes := []struct {
		method string
		opts   []reqtest.Option
	}{
		{http.MethodPost, nil},
		{http.MethodGet, nil},
		{http.MethodPut, []reqtest.Option{reqtest.JSON(plans.PutInstanceRequest{})}},
	}
	for _, route := range routes {
		t.Run(route.method, func(t *testing.T) {
			e := newEnv(t)

			e.refusesEngagementsItCannotReach(route.method, carePlanType, "", route.opts...)
			reqtest.ExpectStatus(t, route.method, e.instanceURL(notAPlanType, ""), http.StatusBadRequest, append([]reqtest.Option{e.signedIn}, route.opts...)...)
			reqtest.ExpectStatus(t, route.method, e.instanceURL(carePlanType, ""), http.StatusNotFound, append([]reqtest.Option{e.signedIn}, route.opts...)...)
		})
	}
}

// TestPostInstanceHandler_Success proves a Plan Instance is created with a
// snapshot of the Practice's current template fields and empty Answers --
// and that any Staff member (not just an Owner) can create one, unlike
// PutTemplateHandler.
func TestPostInstanceHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedTemplate(carePlanType, f1Fields)

	out := e.start(carePlanType, http.StatusCreated)

	if out.EngagementID != e.Engagement.ID || out.PlanType != carePlanType {
		t.Fatalf("out = %+v, want engagementId %q planType %q", out, e.Engagement.ID, carePlanType)
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
	e := newEnv(t)
	e.seedTemplate(birthPlanType, f1Fields)

	e.start(birthPlanType, http.StatusCreated)
	e.start(birthPlanType, http.StatusConflict)
}

// TestGetInstanceHandler_ContractorNeedsAnAttachment proves ADR-0008's
// attachment rule for Plan Instances: a contractor Doula with no
// engagement_attachments row gets the same "not found" response an
// out-of-practice Engagement gets, and an open, granted attachment reaches.
func TestGetInstanceHandler_ContractorNeedsAnAttachment(t *testing.T) {
	cases := []struct {
		name string
		opts []envOption
		want int
	}{
		{"without an attachment", nil, http.StatusNotFound},
		{"with a granted attachment", []envOption{attachedToEngagement()}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, append(tc.opts, signedInAs(fixture.Contractor()))...)
			e.seedInstance(birthPlanType, f1Fields, `{"f1":"Jamie"}`)

			reqtest.ExpectStatus(t, http.MethodGet, e.instanceURL(birthPlanType, ""), tc.want, e.signedIn)
		})
	}
}

func TestGetInstanceHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(birthPlanType, f1Fields, `{"f1":"Jamie"}`)

	out := e.instance(birthPlanType)

	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("fields = %+v, want the stored snapshot", out.Fields)
	}
	if out.Answers["f1"] != jamieAnswer {
		t.Fatalf("answers = %+v, want f1=Jamie", out.Answers)
	}
}

func TestPutInstanceHandler_InvalidBody(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(carePlanType, f1Fields, `{}`)

	reqtest.ExpectStatus(t, http.MethodPut, e.instanceURL(carePlanType, ""), http.StatusBadRequest, e.signedIn, reqtest.RawJSON([]byte("not json")))
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

	e := newEnv(t)
	e.seedInstance(carePlanType, fieldsJSON, `{}`)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodPut, e.instanceURL(carePlanType, ""), http.StatusBadRequest, e.signedIn,
				reqtest.JSON(plans.PutInstanceRequest{Answers: tc.answers}))
		})
	}
}

// TestPutInstanceHandler_OmittedAnswersNormalizesToEmptyObject proves a PUT
// body with no "answers" key (decoding to a nil map) still round-trips as
// `{}`, not JSON null, into both the stored column and the response --
// nonEmpty's nil-normalization path.
func TestPutInstanceHandler_OmittedAnswersNormalizesToEmptyObject(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(carePlanType, f1Fields, `{}`)

	out := reqtest.Expect[plans.InstanceResponse](t, http.MethodPut, e.instanceURL(carePlanType, ""), http.StatusOK, e.signedIn, reqtest.RawJSON([]byte(`{}`)))

	if out.Answers == nil || len(out.Answers) != 0 {
		t.Fatalf("answers = %+v, want a non-nil empty object", out.Answers)
	}
}

// TestPutInstanceHandler_Success proves a full Answers replacement
// round-trips through GET, that the Fields snapshot is unchanged by the
// PUT, and that a second PUT with a smaller Answers map wipes the answer
// left out of it -- PUT is a full replacement, not a merge.
func TestPutInstanceHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(carePlanType,
		`[{"id":"name","type":"short_text","label":"Name","order":0},{"id":"consent","type":"checkbox","label":"Consent","order":1}]`,
		`{}`,
	)

	firstOut := e.putAnswers(carePlanType, plans.Answers{nameFieldID: jamieAnswer, "consent": true})
	if len(firstOut.Fields) != 2 {
		t.Fatalf("fields = %+v, want the unchanged 2-field snapshot", firstOut.Fields)
	}
	if firstOut.Answers[nameFieldID] != jamieAnswer || firstOut.Answers["consent"] != true {
		t.Fatalf("answers = %+v, want name=Jamie consent=true", firstOut.Answers)
	}

	getOut := e.instance(carePlanType)
	if getOut.Answers[nameFieldID] != jamieAnswer || getOut.Answers["consent"] != true {
		t.Fatalf("GET answers after PUT = %+v, want the just-written answers", getOut.Answers)
	}

	secondOut := e.putAnswers(carePlanType, plans.Answers{nameFieldID: jamieAnswer})
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
			e := newEnv(t)
			e.seedInstance(tc.planType, nameFields, `{}`)

			e.putAnswers(tc.planType, plans.Answers{nameFieldID: jamieAnswer})

			if actions := e.activityActions(); len(actions) != 1 || actions[0] != tc.want {
				t.Fatalf("activity actions = %q, want only %q", actions, tc.want)
			}
		})
	}
}

// TestPutInstanceHandler_ASaveThatChangesNothingRecordsNothing proves #2581:
// the ledger's "edited" row follows the same answers-changed test that moves
// changed_at and clears the Client's acknowledgment, so a save that changes
// no answer leaves no row saying the plan changed.
func TestPutInstanceHandler_ASaveThatChangesNothingRecordsNothing(t *testing.T) {
	for _, planType := range []string{carePlanType, birthPlanType} {
		t.Run(planType, func(t *testing.T) {
			e := newEnv(t)
			e.seedInstance(planType, nameFields, `{}`)
			edits := func() int {
				return testdb.Scalar[int](t, e.db.Admin, `SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action LIKE '%plan_edited'`, e.Engagement.ID)
			}

			e.putAnswers(planType, plans.Answers{})
			if got := edits(); got != 0 {
				t.Fatalf("a first save of the same empty answers recorded %d edits, want 0", got)
			}
			e.putAnswers(planType, plans.Answers{nameFieldID: jamieAnswer})
			if got := edits(); got != 1 {
				t.Fatalf("a save that changed an answer recorded %d edits, want 1", got)
			}
			e.putAnswers(planType, plans.Answers{nameFieldID: jamieAnswer})
			if got := edits(); got != 1 {
				t.Fatalf("a repeat save of the same answers left %d edits, want still 1", got)
			}
		})
	}
}

// TestInstance_LastEdited is #2104: a Plan that has never been saved says
// no edit, a save's own response names when and by whom, and a read of
// the other Plan does not borrow this one's edit.
func TestInstance_LastEdited(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(carePlanType, nameFields, `{}`)
	e.seedInstance(birthPlanType, nameFields, `{}`)

	if started := e.instance(carePlanType); started.LastEdited != nil {
		t.Fatalf("a Plan never saved: lastEdited = %+v, want none", started.LastEdited)
	}

	saved := e.putAnswers(carePlanType, plans.Answers{nameFieldID: jamieAnswer})
	if saved.LastEdited == nil || saved.LastEdited.By != e.Staff.Name || saved.LastEdited.At.IsZero() {
		t.Fatalf("the save's response: lastEdited = %+v, want its instant, by the Staff member who saved", saved.LastEdited)
	}

	if again := e.instance(carePlanType); again.LastEdited == nil || !again.LastEdited.At.Equal(saved.LastEdited.At) {
		t.Errorf("GET after the save: lastEdited = %+v, want the save's", again.LastEdited)
	}
	if other := e.instance(birthPlanType); other.LastEdited != nil {
		t.Errorf("the Birth Plan: lastEdited = %+v, want none -- only the Care Plan was saved", other.LastEdited)
	}
}

// TestPostInstanceHandler_RecordsWhoStartedIt is #2133: starting a Plan
// records one action per plan, naming the Staff member who started it;
// the POST's own response, a later GET and a save's response all carry
// that start; a refused second start records nothing more; and the other
// Plan does not borrow it.
func TestPostInstanceHandler_RecordsWhoStartedIt(t *testing.T) {
	for _, tc := range []struct {
		planType, other string
		want            string
	}{
		{carePlanType, birthPlanType, "care_plan_started"},
		{birthPlanType, carePlanType, "birth_plan_started"},
	} {
		t.Run(tc.planType, func(t *testing.T) {
			e := newEnv(t)
			e.seedTemplate(tc.planType, nameFields)
			e.seedInstance(tc.other, nameFields, `{}`)

			posted := e.start(tc.planType, http.StatusCreated)
			if posted.Started == nil || posted.Started.By != e.Staff.Name || posted.Started.At.IsZero() {
				t.Fatalf("POST response: started = %+v, want its instant, by %q", posted.Started, e.Staff.Name)
			}
			if posted.LastEdited != nil {
				t.Errorf("POST response: lastEdited = %+v, want none -- starting is not a save", posted.LastEdited)
			}

			e.start(tc.planType, http.StatusConflict)

			var action, actorID string
			var rows int
			if err := e.db.Admin.QueryRowContext(t.Context(),
				`SELECT action, actor_staff_id::text, count(*) OVER () FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1`,
				e.Engagement.ID,
			).Scan(&action, &actorID, &rows); err != nil {
				t.Fatalf("query activity: %v", err)
			}
			if action != tc.want || actorID != e.Staff.ID || rows != 1 {
				t.Fatalf("activity = %q by %q (%d rows), want one %q by %q", action, actorID, rows, tc.want, e.Staff.ID)
			}

			if saved := e.putAnswers(tc.planType, plans.Answers{nameFieldID: jamieAnswer}); saved.Started == nil || !saved.Started.At.Equal(posted.Started.At) || saved.LastEdited == nil {
				t.Errorf("the save's response: started = %+v, lastEdited = %+v, want the start kept beside the save", saved.Started, saved.LastEdited)
			}

			if got := e.instance(tc.planType); got.Started == nil || !got.Started.At.Equal(posted.Started.At) {
				t.Errorf("GET: started = %+v, want the POST's", got.Started)
			}
			if other := e.instance(tc.other); other.Started != nil {
				t.Errorf("the %s: started = %+v, want none -- only the %s was started here", tc.other, other.Started, tc.planType)
			}
		})
	}
}

// TestPlanTemplateEditDoesNotMutateExistingInstance is the AC's centerpiece:
// editing a Practice's Plan Template after a Plan Instance already exists
// must never change that instance's stored field snapshot.
func TestPlanTemplateEditDoesNotMutateExistingInstance(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedTemplate(carePlanType, f1Fields)
	e.start(carePlanType, http.StatusCreated)

	e.putTemplate(carePlanType, []plans.Field{
		{ID: "f1", Type: shortTextType, Label: testFieldLabel},
		{ID: "f2", Type: "long_text", Label: "Notes added after the plan was created"},
	}, http.StatusOK)

	out := e.instance(carePlanType)
	if len(out.Fields) != 1 || out.Fields[0].ID != "f1" {
		t.Fatalf("instance fields = %+v, want the original 1-field snapshot untouched by the later template edit", out.Fields)
	}
}

// TestPutInstanceHandler_ClientAcknowledgmentFollowsTheAnswers is #301's
// decision for the build: a Staff edit that actually changes the stored
// answers resets the Client's acknowledgment, so Staff see "not reviewed
// since last edit" until she confirms again. The reset is conditioned on
// the answers actually differing, not on the PUT event itself --
// re-sending the same answers (a retry, or Staff saving without changing
// anything) leaves an existing acknowledgment in place.
func TestPutInstanceHandler_ClientAcknowledgmentFollowsTheAnswers(t *testing.T) {
	cases := []struct {
		name        string
		answer      string
		wantCleared bool
	}{
		{"answers change", "Alex", true},
		{"answers unchanged", jamieAnswer, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.seedInstance(birthPlanType, nameFields, `{"name":"Jamie"}`)
			e.setAcknowledged(birthPlanType)

			out := e.putAnswers(birthPlanType, plans.Answers{nameFieldID: tc.answer})

			if cleared := out.ClientAcknowledgedAt == nil; cleared != tc.wantCleared {
				t.Fatalf("clientAcknowledgedAt = %v, want it cleared = %v", out.ClientAcknowledgedAt, tc.wantCleared)
			}
		})
	}
}

// TestGetInstanceHandler_ReflectsClientAcknowledgment is the Staff
// visibility half of #301's AC4: Staff can see, on their own read, that
// the Client has reviewed her Birth Plan.
func TestGetInstanceHandler_ReflectsClientAcknowledgment(t *testing.T) {
	e := newEnv(t)
	e.seedInstance(birthPlanType, `[]`, `{}`)
	e.setAcknowledged(birthPlanType)

	if out := e.instance(birthPlanType); out.ClientAcknowledgedAt == nil {
		t.Fatalf("clientAcknowledgedAt = nil, want Staff's own read to show the Client's acknowledgment")
	}
}
