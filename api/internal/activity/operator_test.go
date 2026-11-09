package activity_test

import (
	"encoding/json/v2"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

const creditsRefunded = "credits_refunded"

func TestRecord_OperatorActorWrittenByOpsRuntime(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")
	at := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

	diff, _ := json.Marshal(map[string]int{"credits": 3})
	mustRecord(t, db.Ops, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   p.ID,
		Action:      creditsRefunded,
		Diff:        diff,
		Actor:       activity.OperatorActor(actID),
		CreatedAt:   at,
	})

	r := readRow(t, db, p.ID)
	if r.ActorKind != "operator" || r.ActorOperatorActID == nil || *r.ActorOperatorActID != actID || r.ActorStaffID != nil || r.ActorClientID != nil {
		t.Fatalf("row = %+v, want an operator actor keyed to %s", r, actID)
	}
	if r.Action != creditsRefunded || r.SubjectKind != "practice" || r.SubjectID != p.ID {
		t.Fatalf("row = %+v, want credits_refunded on practice/%s", r, p.ID)
	}
	if r.Diff != `{"credits": 3}` {
		t.Fatalf("diff = %s, want {\"credits\": 3}", r.Diff)
	}
	if !r.CreatedAt.Equal(at) {
		t.Fatalf("created_at = %v, want %v", r.CreatedAt, at)
	}
}

func TestRecord_OperatorActorWithoutADiffRecordsAnEmptyOne(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	mustRecord(t, db.Ops, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   p.ID,
		Action:      "founding_grant_issued",
		Actor:       activity.OperatorActor(actID),
		CreatedAt:   time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC),
	})

	if diff := readRow(t, db, p.ID).Diff; diff != `{}` {
		t.Fatalf("diff = %s, want {}", diff)
	}
}

func TestRecord_AppRuntimeCannotWriteAnOperatorRow(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	err := record(t, db.App, activity.Entry{
		PracticeID:  p.ID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   p.ID,
		Action:      creditsRefunded,
		Actor:       activity.OperatorActor(actID),
	}, activity.ScopedTo(p.ID))
	if err == nil {
		t.Fatal("app_runtime wrote an operator row, want it refused")
	}
}

func TestOperatorActActorName_NamesTheOperatorOnlyAtThePracticeTheRowIsAt(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Acted On Practice"))
	other := fixture.Practice(t, db, fixture.PracticeName("Other Practice"))
	actID := testdb.SeedOperatorActivity(t, db, p.ID, activity.SubjectPractice, p.ID, creditsRefunded, "Mark Goho", time.Now())
	strayActID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	for _, tc := range []struct {
		name       string
		practiceID string
		actID      string
		want       *string
	}{
		{"the Practice the row is at", p.ID, actID, new("Mark Goho")},
		{"another Practice", other.ID, actID, nil},
		{"an act no row at the Practice points at", p.ID, strayActID, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := db.BeginAs(t, testdb.Practice(tc.practiceID))

			got := testdb.Scalar[*string](t, tx, `SELECT operator_act_actor_name($1)`, tc.actID)

			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("name = %v, want %v", got, tc.want)
			}
		})
	}

	if _, err := db.App.ExecContext(t.Context(), `SELECT name FROM operators`); err == nil {
		t.Fatal("app_runtime read operators, want permission denied")
	}
}

func TestOperatorActorName_IsThePersonsNameWithDoulaCloud(t *testing.T) {
	if got := activity.OperatorActorName("Mark Goho"); got != "Mark Goho at DoulaCloud" {
		t.Fatalf("OperatorActorName = %q, want Mark Goho at DoulaCloud", got)
	}
}

func TestActivity_AnOperatorRowMustNameItsAct(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	staff := fixture.Staff(t, db, p)
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	for name, statement := range map[string]string{
		"operator without an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind)
			VALUES ($1, 'practice', $1, 'x', '{}', 'operator')`,
		"operator with a staff id too": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id, actor_staff_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'operator', '` + actID + `', '` + staff.ID + `')`,
		"staff with an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id, actor_staff_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'staff', '` + actID + `', '` + staff.ID + `')`,
		"system with an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'system', '` + actID + `')`,
	} {
		if _, err := db.Admin.ExecContext(t.Context(), statement, p.ID); err == nil {
			t.Errorf("%s: inserted, want the activity_actor CHECK to refuse it", name)
		}
	}
}
