package practicedeletion_test

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/practicedeletion"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestTransitions_TableIsComplete holds practicedeletion.Transitions to
// the three moves ADR-0031 draws: request, restore, finalize, each from
// and to the standing its doc comment names.
func TestTransitions_TableIsComplete(t *testing.T) {
	want := map[string][2]practicedeletion.Standing{
		"delete it":             {practicedeletion.StandingActive, practicedeletion.StandingPendingDeletion},
		"restore it":            {practicedeletion.StandingPendingDeletion, practicedeletion.StandingActive},
		"finalize its deletion": {practicedeletion.StandingPendingDeletion, practicedeletion.StandingDeleted},
	}
	if len(practicedeletion.Transitions) != len(want) {
		t.Fatalf("len(Transitions) = %d, want %d", len(practicedeletion.Transitions), len(want))
	}
	for _, tr := range practicedeletion.Transitions {
		moves, known := want[tr.Act]
		if !known {
			t.Errorf("Transitions has unexpected entry %+v", tr)
			continue
		}
		if tr.Requires != moves[0] || tr.Becomes != moves[1] {
			t.Errorf("Transition %q moves %s -> %s, want %s -> %s", tr.Act, tr.Requires, tr.Becomes, moves[0], moves[1])
		}
	}
}

// TestTransition_CheckSpeaksToThePerson proves each refusal names the
// standing the Practice is in, in a sentence an error summary prints as
// it is.
func TestTransition_CheckSpeaksToThePerson(t *testing.T) {
	if ok, refusal := practicedeletion.TransitionRestore.Check(practicedeletion.StandingPendingDeletion); !ok || refusal != "" {
		t.Fatalf("Check(pending) = (%v, %q), want (true, \"\")", ok, refusal)
	}
	cases := []struct {
		transition practicedeletion.Transition
		current    practicedeletion.Standing
		want       string
	}{
		{practicedeletion.TransitionRestore, practicedeletion.StandingActive, "This Practice is not set to be deleted, so you cannot restore it."},
		{practicedeletion.TransitionRequest, practicedeletion.StandingPendingDeletion, "This Practice is set to be deleted, so you cannot delete it."},
		{practicedeletion.TransitionRequest, practicedeletion.StandingFinalizeOverdue, "This Practice is set to be deleted, so you cannot delete it."},
		{practicedeletion.TransitionRestore, practicedeletion.StandingDeleted, "This Practice has been deleted, so you cannot restore it."},
		{practicedeletion.TransitionFinalize, practicedeletion.Standing("archived"), "This Practice is archived, so you cannot finalize its deletion."},
	}
	for _, tc := range cases {
		if _, refusal := tc.transition.Check(tc.current); refusal != tc.want {
			t.Errorf("Check(%q) = %q, want %q", tc.current, refusal, tc.want)
		}
	}
}

// writesPractice is a statement in this package that changes the
// practices row, up to the end of its backquoted SQL.
var writesPractice = regexp.MustCompile("UPDATE practices SET[^`]*")

// TestTransitions_EveryWriterIsGuarded is the source-scan guardrail
// contracts' lifecycle_test.go keeps for a Contract: every statement in
// this package that writes the practices row carries a standing guard,
// and every file that holds one consults a Transition's Check -- so a
// writer that stops asking which standing the row is in is caught the
// moment it stops, not when two requests race.
func TestTransitions_EveryWriterIsGuarded(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		// coverage:ignore reason: this package's own directory is always readable while its tests run
		t.Fatalf("read package directory: %v", err)
	}
	writers := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// #nosec G304 -- name comes from ReadDir on this package's own directory
		src, err := os.ReadFile(name)
		if err != nil {
			// coverage:ignore reason: a file ReadDir listed a moment ago
			t.Fatalf("read %s: %v", name, err)
		}
		statements := writesPractice.FindAllString(string(src), -1)
		for _, statement := range statements {
			writers++
			if !strings.Contains(statement, "AND standing = $") {
				t.Errorf("%s writes practices without a standing guard: %q", name, statement)
			}
		}
		if len(statements) > 0 && !strings.Contains(string(src), "Transition") {
			t.Errorf("%s writes practices but never names a Transition", name)
		}
	}
	// request, restore and finalize
	if writers != 3 {
		t.Errorf("found %d practices writers in this package, want 3 -- a new one needs a Transition, and this count", writers)
	}
}

// derivesStanding is a reader asking the deletion facts what a Practice's
// standing is, rather than reading practices.standing.
var derivesStanding = regexp.MustCompile(`deletion_requested_at\s+IS|\bp\.deleted_at\s+IS|practices\.deleted_at\s+IS`)

// writesDeletionFacts is a write to the deletion facts.
var writesDeletionFacts = regexp.MustCompile(`UPDATE practices\b[^` + "`" + `]*\b(deletion_requested_at|deletion_requested_by|deletion_finalize_at|deleted_at)\s*=`)

// TestStanding_IsTheOneDerivation walks every hand-written Go file under
// api/internal and api/cmd: none derives a Practice's standing from its
// deletion facts, and none outside this package writes them. Each reads
// practices.standing, and each write goes through a Transition here --
// but for testdb's fixture, which seeds a standing for other packages'
// tests the way every fixture there bypasses the handler it stands in for.
func TestStanding_IsTheOneDerivation(t *testing.T) {
	mayWrite := map[string]bool{}
	for _, dir := range []string{".", filepath.Join("..", "testdb")} {
		abs, err := filepath.Abs(dir)
		if err != nil {
			// coverage:ignore reason: a path inside this module always resolves while tests run
			t.Fatalf("resolve %s: %v", dir, err)
		}
		mayWrite[abs] = true
	}
	scanned := 0
	for _, root := range []string{"..", filepath.Join("..", "..", "cmd")} {
		tree := os.DirFS(root)
		err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// coverage:ignore reason: a directory WalkDir just listed
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := fs.ReadFile(tree, path)
			if err != nil {
				// coverage:ignore reason: a file WalkDir listed a moment ago
				return fmt.Errorf("read %s: %w", path, err)
			}
			scanned++
			if m := derivesStanding.FindString(string(src)); m != "" {
				t.Errorf("%s derives a Practice's standing (%q) -- read practices.standing instead", path, m)
			}
			abs, _ := filepath.Abs(filepath.Join(root, filepath.Dir(path)))
			if m := writesDeletionFacts.FindString(string(src)); m != "" && !mayWrite[abs] {
				t.Errorf("%s writes a Practice's deletion facts (%q) -- only practicedeletion's Transitions may", path, m)
			}
			return nil
		})
		if err != nil {
			// coverage:ignore reason: the source tree is readable while tests run
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d files, want the whole of api/internal and api/cmd", scanned)
	}
}

// TestStanding_SchemaRefusesWhatNoTransitionWrites proves 00172's CHECKs:
// a request missing one of its three facts, and a deletion with no
// request under it, are refused at the database, whoever writes them.
func TestStanding_SchemaRefusesWhatNoTransitionWrites(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := newOwnedPractice(t, db, "owner-schema-refusals")
	for name, statement := range map[string]string{
		"a request with no requester":     `UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now() WHERE id = $1 AND $2::uuid IS NOT NULL`,
		"a request with no finalize date": `UPDATE practices SET deletion_requested_at = now(), deletion_requested_by = $2 WHERE id = $1`,
		"a requester with no request":     `UPDATE practices SET deletion_requested_by = $2 WHERE id = $1`,
		"a deletion with no request":      `UPDATE practices SET deleted_at = now() WHERE id = $1 AND $2::uuid IS NOT NULL`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := db.Admin.ExecContext(t.Context(), statement, practiceID, staffID)
			if err == nil || !strings.Contains(err.Error(), "practices_deletion") && !strings.Contains(err.Error(), "practices_deleted_after_request") {
				t.Fatalf("update = %v, want a CHECK refusal", err)
			}
		})
	}
}

// standingAt reads practice_standing_at for practiceID at at -- the one
// derivation of finalize_overdue.
func standingAt(t *testing.T, db *testdb.DB, practiceID string, at time.Time) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin,
		`SELECT practice_standing_at(standing, deletion_finalize_at, $2) FROM practices WHERE id = $1`, practiceID, at)
}

// TestStanding_ReadsEachStanding walks one Practice through every
// standing the column and practice_standing_at can report.
func TestStanding_ReadsEachStanding(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := newOwnedPractice(t, db, "owner-standings")
	finalizeAt := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)

	if got := standingAt(t, db, practiceID, finalizeAt); got != string(practicedeletion.StandingActive) {
		t.Errorf("a Practice nobody asked to delete reads %s", got)
	}
	testdb.SeedPracticeDeletion(t, db, practiceID, finalizeAt.Add(-practicedeletion.RestoreWindow), finalizeAt, nil)
	if got := standingAt(t, db, practiceID, finalizeAt.Add(-time.Second)); got != string(practicedeletion.StandingPendingDeletion) {
		t.Errorf("a Practice inside its window reads %s", got)
	}
	if got := standingAt(t, db, practiceID, finalizeAt); got != string(practicedeletion.StandingFinalizeOverdue) {
		t.Errorf("a Practice still pending at its finalize date reads %s", got)
	}
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practices SET deleted_at = $2 WHERE id = $1`, practiceID, finalizeAt); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if got := standingAt(t, db, practiceID, finalizeAt.AddDate(1, 0, 0)); got != string(practicedeletion.StandingDeleted) {
		t.Errorf("a finalized Practice reads %s", got)
	}
}

// TestRestoreHandler_RefusesWithTheStandingNamed proves the restore's
// refusal is the Transition's own sentence.
func TestRestoreHandler_RefusesWithTheStandingNamed(t *testing.T) {
	e := newEnv(t, fixture.Owner())

	resp := e.call(http.MethodDelete, false)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusConflict)
	if got := apierrtest.Decode(t, resp).Message; got != "This Practice is not set to be deleted, so you cannot restore it." {
		t.Fatalf("message = %q", got)
	}
}

// TestRestoreHandler_RestoresADeletionPastItsDate proves an overdue
// Practice is still pending to every gate: its Owner can restore it.
func TestRestoreHandler_RestoresADeletionPastItsDate(t *testing.T) {
	e := newEnv(t, fixture.Owner())
	past := time.Now().Add(-time.Hour)
	testdb.SeedPracticeDeletion(t, e.DB, e.Practice.ID, past.Add(-practicedeletion.RestoreWindow), past, nil)

	e.restore()

	if got := standingAt(t, e.DB, e.Practice.ID, time.Now()); got != string(practicedeletion.StandingActive) {
		t.Fatalf("standing after restoring an overdue deletion = %s, want active", got)
	}
}

// TestWorker_AFinalizeThatFailsLeavesThePracticeOverdue reproduces what
// the finalize outbox row's failure path does. A finalization fails on a
// database error, which aborts the worker's transaction, so MarkFailed's
// own write never lands and the pass rolls back: the row stays pending
// at the same attempt, retried every pass, and is never dead-lettered.
// Either way -- retrying for ever or dead-lettered by some other route --
// the Practice is left pending past its date with nothing to show it
// but practice_standing_at, which reads finalize_overdue.
func TestWorker_AFinalizeThatFailsLeavesThePracticeOverdue(t *testing.T) {
	db := testdb.New(t)
	practiceID, _ := newOwnedPractice(t, db, "owner-finalize-fails")
	due := time.Now().Add(-time.Minute)
	markPendingDeletion(t, db, practiceID, due)
	rowID := seedOutboxRow(t, db, practiceID, "finalize", due)
	for _, statement := range []string{
		`CREATE FUNCTION refuse_practice_finalize() RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN RAISE EXCEPTION 'finalize refused for this test'; END $$`,
		`CREATE TRIGGER refuse_practice_finalize BEFORE UPDATE OF deleted_at ON practices
		 FOR EACH ROW EXECUTE FUNCTION refuse_practice_finalize()`,
	} {
		if _, err := db.Admin.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("install failing finalize: %v", err)
		}
	}

	tx := db.BeginAs(t, testdb.NotificationWorker())
	err := newTestWorker(&mail.FakeSender{}).ProcessPending(t.Context(), tx)
	_ = tx.Rollback()
	if err == nil {
		t.Fatal("ProcessPending = nil, want the finalize failure")
	}

	if status, attempts := testdb.Scalar[string](t, db.Admin, `SELECT status::text FROM practice_deletion_outbox WHERE id = $1`, rowID),
		testdb.Scalar[int](t, db.Admin, `SELECT attempt_count FROM practice_deletion_outbox WHERE id = $1`, rowID); status != "pending" || attempts != 0 {
		t.Errorf("finalize row = (%s, %d attempts), want (pending, 0) -- the aborted pass records nothing", status, attempts)
	}
	if got := standingAt(t, db, practiceID, time.Now()); got != string(practicedeletion.StandingFinalizeOverdue) {
		t.Errorf("standing after a failed finalize = %s, want finalize_overdue", got)
	}
}
