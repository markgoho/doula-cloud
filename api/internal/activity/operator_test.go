package activity_test

import (
	"encoding/json"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

const creditsRefunded = "credits_refunded"

func TestRecord_OperatorActorWrittenByOpsRuntime(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")
	at := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

	tx, err := db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	diff, _ := json.Marshal(map[string]int{"credits": 3})
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      creditsRefunded,
		Diff:        diff,
		Actor:       activity.OperatorActor(actID),
		CreatedAt:   at,
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var (
		actorKind, action, subjectKind, subjectID, gotActID string
		gotDiff                                             []byte
		createdAt                                           time.Time
		staffID, clientID                                   *string
	)
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_kind::text, action, subject_kind, subject_id, actor_operator_act_id, diff, created_at, actor_staff_id, actor_client_id
		   FROM activity WHERE practice_id = $1`, practiceID,
	).Scan(&actorKind, &action, &subjectKind, &subjectID, &gotActID, &gotDiff, &createdAt, &staffID, &clientID); err != nil {
		t.Fatalf("read activity row: %v", err)
	}
	if actorKind != "operator" || gotActID != actID || staffID != nil || clientID != nil {
		t.Fatalf("actor = (%s, act %s, staff %v, client %v), want operator keyed to %s", actorKind, gotActID, staffID, clientID, actID)
	}
	if action != creditsRefunded || subjectKind != "practice" || subjectID != practiceID {
		t.Fatalf("row = (%s, %s/%s), want credits_refunded on practice/%s", action, subjectKind, subjectID, practiceID)
	}
	if string(gotDiff) != `{"credits": 3}` {
		t.Fatalf("diff = %s, want {\"credits\": 3}", gotDiff)
	}
	if !createdAt.Equal(at) {
		t.Fatalf("created_at = %v, want %v", createdAt, at)
	}
}

func TestRecord_OperatorActorWithoutADiffRecordsAnEmptyOne(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	tx, err := db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      "founding_grant_issued",
		Actor:       activity.OperatorActor(actID),
		CreatedAt:   time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT diff FROM activity WHERE practice_id = $1`, practiceID).Scan(&diff); err != nil {
		t.Fatalf("read diff: %v", err)
	}
	if string(diff) != `{}` {
		t.Fatalf("diff = %s, want {}", diff)
	}
}

func TestRecord_AppRuntimeCannotWriteAnOperatorRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	err = activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      creditsRefunded,
		Actor:       activity.OperatorActor(actID),
	}, activity.ScopedTo(practiceID))
	if err == nil {
		t.Fatal("app_runtime wrote an operator row, want it refused")
	}
}

func TestOperatorActActorName_NamesTheOperatorOnlyAtThePracticeTheRowIsAt(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Acted On Practice")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	actID := testdb.SeedOperatorActivity(t, db, practiceID, activity.SubjectPractice, practiceID, creditsRefunded, "Mark Goho", time.Now())
	strayActID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	for _, tc := range []struct {
		name       string
		practiceID string
		actID      string
		want       *string
	}{
		{"the Practice the row is at", practiceID, actID, new("Mark Goho")},
		{"another Practice", otherPracticeID, actID, nil},
		{"an act no row at the Practice points at", practiceID, strayActID, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.App.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, tc.practiceID); err != nil {
				t.Fatalf("scope: %v", err)
			}
			var got *string
			if err := tx.QueryRowContext(t.Context(), `SELECT operator_act_actor_name($1)`, tc.actID).Scan(&got); err != nil {
				t.Fatalf("operator_act_actor_name: %v", err)
			}
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
	practiceID := testdb.SeedPractice(t, db, "Test Practice")
	staffID := testdb.SeedStaff(t, db, "operator-check-staff")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	for name, statement := range map[string]string{
		"operator without an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind)
			VALUES ($1, 'practice', $1, 'x', '{}', 'operator')`,
		"operator with a staff id too": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id, actor_staff_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'operator', '` + actID + `', '` + staffID + `')`,
		"staff with an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id, actor_staff_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'staff', '` + actID + `', '` + staffID + `')`,
		"system with an act": `INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_operator_act_id)
			VALUES ($1, 'practice', $1, 'x', '{}', 'system', '` + actID + `')`,
	} {
		if _, err := db.Admin.ExecContext(t.Context(), statement, practiceID); err == nil {
			t.Errorf("%s: inserted, want the activity_actor CHECK to refuse it", name)
		}
	}
}
