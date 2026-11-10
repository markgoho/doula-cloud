package attachmentstate_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/attachmentstate"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestTransitions_AreWhatTheDeclarationSays holds each declared move to
// the states and endings that exist: a move to Ended names why, no other
// move does, and every ending but DroppedOut -- which no route writes yet
// -- has the one move that writes it.
func TestTransitions_AreWhatTheDeclarationSays(t *testing.T) {
	written := map[attachmentstate.Ending]int{}
	for _, tr := range attachmentstate.Transitions {
		if !slices.Contains(attachmentstate.States, tr.To) {
			t.Errorf("%s moves to %q, which is not a State", tr.Act, tr.To)
		}
		for _, from := range tr.From {
			if from == attachmentstate.Ended {
				t.Errorf("%s leaves Ended, which is terminal", tr.Act)
			}
		}
		if (tr.To == attachmentstate.Ended) != (tr.Ending != "") {
			t.Errorf("%s: a move to Ended names its Ending, and no other move does (To %q, Ending %q)", tr.Act, tr.To, tr.Ending)
		}
		if tr.Ending != "" {
			written[tr.Ending]++
		}
	}
	for _, e := range attachmentstate.Endings {
		want := 1
		if e == attachmentstate.DroppedOut {
			want = 0
		}
		if written[e] != want {
			t.Errorf("ending %q is written by %d transitions, want %d", e, written[e], want)
		}
	}
}

// TestNames_MatchTheEnums holds the Go names to 00173's two enums.
func TestNames_MatchTheEnums(t *testing.T) {
	db := testdb.New(t)
	for enum, want := range map[string][]string{
		"attachment_state":  toStrings(attachmentstate.States),
		"attachment_ending": toStrings(attachmentstate.Endings),
	} {
		got := testdb.Scalar[string](t, db.Admin, `SELECT array_to_string(enum_range(NULL::`+enum+`)::text[], ',')`)
		if got != strings.Join(want, ",") {
			t.Errorf("%s = %q, want %q", enum, got, strings.Join(want, ","))
		}
	}
}

func toStrings[S ~string](values []S) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

// scene is one Practice with an Owner, a Doula and an Engagement, and a
// transaction under the Practice's own row security whose context carries
// a fixed instant, so a test can tell the clock seam's instant from now().
type scene struct {
	db         *testdb.DB
	practice   fixture.PracticeRecord
	owner      fixture.StaffRecord
	doula      fixture.StaffRecord
	engagement string
	tx         *sql.Tx
	ctx        context.Context
	at         time.Time
}

func newScene(t *testing.T) scene {
	t.Helper()
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	s := scene{
		db:         db,
		practice:   p,
		owner:      fixture.Staff(t, db, p, fixture.Owner()),
		doula:      fixture.Staff(t, db, p, fixture.Doula()),
		engagement: fixture.Engagement(t, db, p).ID,
		at:         time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}
	s.ctx = clock.Into(t.Context(), func() time.Time { return s.at })
	s.tx = db.BeginAs(t, testdb.Practice(p.ID))
	t.Cleanup(func() { _ = s.tx.Rollback() })
	return s
}

// row is what one Attachment says about itself.
type row struct {
	state, origin, attachedBy string
	attachedAt                time.Time
	endedAt                   sql.NullTime
	endedBy, endedReason      sql.NullString
	fee                       sql.NullInt64
}

// rows reads every Attachment staffID holds on engagementID, oldest first.
func (s scene) rows(t *testing.T, engagementID, staffID string) []row {
	t.Helper()
	rs, err := s.tx.QueryContext(t.Context(),
		`SELECT state::text, origin::text, attached_by::text, attached_at, ended_at, ended_by::text, ended_reason::text, fee_amount_cents
		   FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2 ORDER BY attached_at`,
		engagementID, staffID)
	if err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.state, &r.origin, &r.attachedBy, &r.attachedAt, &r.endedAt, &r.endedBy, &r.endedReason, &r.fee); err != nil {
			t.Fatalf("scan attachment: %v", err)
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	return out
}

// Accrue opens her own accrued Attachment, and
// Grant upgrades it in place -- one row, granted, attached_by the grantor,
// the Offer's fee copied on -- rather than opening a second.
func TestAccrueThenGrant_UpgradesInPlace(t *testing.T) {
	s := newScene(t)
	if err := attachmentstate.Accrue(s.ctx, s.tx, s.engagement, s.doula.ID); err != nil {
		t.Fatalf("Accrue: %v", err)
	}
	got := s.rows(t, s.engagement, s.doula.ID)
	if len(got) != 1 || got[0].state != string(attachmentstate.Accrued) || got[0].attachedBy != s.doula.ID {
		t.Fatalf("after Accrue = %+v, want one accrued row by her", got)
	}

	fee, terms := int64(52000), "Two prenatal visits."
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID, &fee, &terms, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	got = s.rows(t, s.engagement, s.doula.ID)
	if len(got) != 1 || got[0].state != string(attachmentstate.Granted) || got[0].attachedBy != s.owner.ID || got[0].fee.Int64 != fee {
		t.Fatalf("after Grant = %+v, want the one row granted by the Owner with the fee", got)
	}

	// A second Accrue, and a grant carrying no fee, leave the granted row
	// exactly as it is.
	if err := attachmentstate.Accrue(s.ctx, s.tx, s.engagement, s.doula.ID); err != nil {
		t.Fatalf("Accrue again: %v", err)
	}
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, s.doula.ID, s.doula.ID, nil, nil, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant again: %v", err)
	}
	got = s.rows(t, s.engagement, s.doula.ID)
	if len(got) != 1 || got[0].state != string(attachmentstate.Granted) || got[0].attachedBy != s.owner.ID || got[0].fee.Int64 != fee {
		t.Fatalf("after a repeat = %+v, want the granted row untouched", got)
	}
}

// TakeOff ends only a granted Attachment, recording who, when (the
// clock's instant, not the database's) and why, and reports whether it
// ended one.
func TestTakeOff_EndsOnlyAGrantedAttachment(t *testing.T) {
	s := newScene(t)
	if err := attachmentstate.Accrue(s.ctx, s.tx, s.engagement, s.doula.ID); err != nil {
		t.Fatalf("Accrue: %v", err)
	}
	if ended, err := attachmentstate.TakeOff(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID); err != nil || ended {
		t.Fatalf("TakeOff on an accrued attachment = %v, %v, want false", ended, err)
	}
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID, nil, nil, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if ended, err := attachmentstate.TakeOff(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID); err != nil || !ended {
		t.Fatalf("TakeOff on a granted attachment = %v, %v, want true", ended, err)
	}
	if ended, err := attachmentstate.TakeOff(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID); err != nil || ended {
		t.Fatalf("a second TakeOff = %v, %v, want false", ended, err)
	}
	assertEnded(t, s.rows(t, s.engagement, s.doula.ID), s.owner.ID, s.at, attachmentstate.TakenOff)
}

// EndForCompletion ends every open Attachment on the Engagement, of
// either origin, and leaves one that already ended as it was.
func TestEndForCompletion_EndsEveryOpenAttachment(t *testing.T) {
	s := newScene(t)
	other := fixture.Staff(t, s.db, s.practice, fixture.Doula())
	earlier := fixture.Staff(t, s.db, s.practice, fixture.Doula())
	if err := attachmentstate.Accrue(s.ctx, s.tx, s.engagement, s.doula.ID); err != nil {
		t.Fatalf("Accrue: %v", err)
	}
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, other.ID, s.owner.ID, nil, nil, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	testdb.SeedAttachment(t, s.db, s.engagement, earlier.ID, "granted", true)

	if err := attachmentstate.EndForCompletion(s.ctx, s.tx, s.engagement, s.owner.ID); err != nil {
		t.Fatalf("EndForCompletion: %v", err)
	}
	assertEnded(t, s.rows(t, s.engagement, s.doula.ID), s.owner.ID, s.at, attachmentstate.EngagementCompleted)
	assertEnded(t, s.rows(t, s.engagement, other.ID), s.owner.ID, s.at, attachmentstate.EngagementCompleted)
	if got := s.rows(t, s.engagement, earlier.ID); len(got) != 1 || got[0].endedReason.String != string(attachmentstate.TakenOff) {
		t.Fatalf("the earlier ending = %+v, want it left as it was", got)
	}
}

// EndForMembership ends every open Attachment she holds at this Practice,
// and nothing of anyone else's or at another Practice.
func TestEndForMembership_EndsHersAtThisPracticeOnly(t *testing.T) {
	s := newScene(t)
	second := fixture.Engagement(t, s.db, s.practice).ID
	colleague := fixture.Staff(t, s.db, s.practice, fixture.Doula())
	elsewhere := fixture.Engagement(t, s.db, fixture.Practice(t, s.db)).ID
	testdb.SeedGrantedAttachment(t, s.db, elsewhere, s.doula.ID)
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, s.doula.ID, s.owner.ID, nil, nil, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if err := attachmentstate.Accrue(s.ctx, s.tx, second, s.doula.ID); err != nil {
		t.Fatalf("Accrue: %v", err)
	}
	if err := attachmentstate.Grant(s.ctx, s.tx, s.engagement, colleague.ID, s.owner.ID, nil, nil, clock.Now(s.ctx)); err != nil {
		t.Fatalf("Grant colleague: %v", err)
	}

	if err := attachmentstate.EndForMembership(s.ctx, s.tx, s.practice.ID, s.doula.ID, s.owner.ID); err != nil {
		t.Fatalf("EndForMembership: %v", err)
	}
	assertEnded(t, s.rows(t, s.engagement, s.doula.ID), s.owner.ID, s.at, attachmentstate.MembershipEnded)
	assertEnded(t, s.rows(t, second, s.doula.ID), s.owner.ID, s.at, attachmentstate.MembershipEnded)
	if got := s.rows(t, s.engagement, colleague.ID); len(got) != 1 || got[0].state != string(attachmentstate.Granted) {
		t.Fatalf("a colleague's attachment = %+v, want it still granted", got)
	}
	if state := testdb.Scalar[string](t, s.db.Admin,
		`SELECT state::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`, elsewhere, s.doula.ID,
	); state != string(attachmentstate.Granted) {
		t.Fatalf("her attachment at another Practice = %q, want it still granted", state)
	}
}

func assertEnded(t *testing.T, got []row, by string, at time.Time, why attachmentstate.Ending) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("attachments = %+v, want one", got)
	}
	r := got[0]
	if r.state != string(attachmentstate.Ended) || r.endedBy.String != by || !r.endedAt.Time.Equal(at) || r.endedReason.String != string(why) {
		t.Fatalf("attachment = %+v, want ended by %s at %s because %s", r, by, at, why)
	}
}
