package oncall_test

import "testing"

// #2755: the panel says who holds the birth tonight, by the roster's own
// rule.
func TestPanel_TonightNamesWhoHoldsTheBirth(t *testing.T) {
	e := newEnv(t, dueOn(octEighth))

	t.Run("a plain holder", func(t *testing.T) {
		got := e.panel().Tonight
		if got == nil || got.IsNobody || len(got.Holders) != 1 || got.Holders[0].StaffID != e.Doula.ID || got.Holders[0].CoveringFor != nil {
			t.Errorf("tonight = %+v, want the Doula alone", got)
		}
	})

	t.Run("a covering Doula", func(t *testing.T) {
		backup := e.backup()
		e.gapOn(e.Birth.ID, e.Doula.ID, "2026-10-01T22:00:00Z", "2026-10-02T10:00:00Z", backup.ID)
		got := e.panel().Tonight
		if got == nil || len(got.Holders) != 1 || got.Holders[0].StaffID != backup.ID || got.Holders[0].CoveringFor == nil {
			t.Fatalf("tonight = %+v, want the backup alone, covering", got)
		}
	})

	t.Run("nobody: a gap with nobody covering", func(t *testing.T) {
		e.Exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id, ended_reason = 'taken_off' WHERE staff_id <> $1 AND engagement_id = $2`, e.Doula.ID, e.Birth.ID)
		e.Exec(`UPDATE engagement_coverage_gaps SET covering_staff_id = NULL`)
		got := e.panel().Tonight
		if got == nil || !got.IsNobody || len(got.Holders) != 0 {
			t.Errorf("tonight = %+v, want nobody", got)
		}
	})
}

func TestPanel_HasNoTonightOutsideTheBirthsDays(t *testing.T) {
	e := newEnv(t)

	if got := e.panel().Tonight; got != nil {
		t.Errorf("tonight = %+v, want none: the days start on the 9th", got)
	}
}
