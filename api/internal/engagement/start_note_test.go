package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// seedStartNote writes the approved Request that started the Engagement,
// carrying note (or none for ""), decided by who.
func (e *env) seedStartNote(note string, decidedBy fixture.StaffRecord) {
	e.t.Helper()
	var text any
	if note != "" {
		text = note
	}
	e.exec(`INSERT INTO engagement_requests (practice_id, client_id, kind, note, state, requested_by, decided_by, decided_at, engagement_id)
		 SELECT e.practice_id, e.client_id, e.kind, $2, 'approved', $3, $3, now(), e.id FROM engagements e WHERE e.id = $1`,
		e.Engagement.ID, text, decidedBy.ID)
}

// #2597: the Note typed on the Request that started an Engagement is read
// back on the Engagement, with who wrote it, for an Owner or an Admin only.
func TestDetailHandler_StartNoteForOwnersAndAdminsOnly(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.StaffName("Renata Owner")), withEngagement(fixture.Status(fixture.StatusActive)))
	admin := e.staff(fixture.Admin(), fixture.StaffName("Dee Admin"))
	employee := e.staff(fixture.StaffName("Priya Employee"))
	e.seedStartNote("Heard about us from her midwife.", admin)

	for _, reader := range []fixture.StaffRecord{e.Staff, admin} {
		t.Run(reader.Name+" reads it", func(t *testing.T) {
			note := e.detailAs(reader).StartNote
			if note == nil || note.Text != "Heard about us from her midwife." || note.WrittenBy != "Dee Admin" || note.WrittenAt.IsZero() {
				t.Errorf("startNote = %+v, want Dee Admin's note", note)
			}
		})
	}
	t.Run("an employee Doula does not", func(t *testing.T) {
		if note := e.detailAs(employee).StartNote; note != nil {
			t.Errorf("startNote = %+v, want none", note)
		}
	})
}

func TestDetailHandler_NoStartNoteWhenNoneWasWritten(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	e.seedStartNote("", e.Staff)

	if note := e.detail().StartNote; note != nil {
		t.Errorf("startNote = %+v, want none", note)
	}
}
