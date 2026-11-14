package oncall_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

const (
	camilleBoyd  = "Camille Boyd"
	fieldStaffID = "staffId"
)

func (e *env) previewURL(staffID string, from, to time.Time) string {
	q := url.Values{fieldStaffID: {staffID}, fieldStartsAt: {from.Format(time.RFC3339)}, fieldEndsAt: {to.Format(time.RFC3339)}}
	return e.PracticeURL("/coverage-gaps/preview?" + q.Encode())
}

func (e *env) bulkURL() string { return e.PracticeURL("/coverage-gaps") }

// secondBirth is another live birth with the same window, the Doula on it.
func (e *env) secondBirth(who fixture.StaffRecord) fixture.EngagementRecord {
	e.T.Helper()
	return e.AnotherEngagement(
		fixture.NewClient(fixture.ClientName(camilleBoyd)),
		fixture.Status(fixture.StatusActive), fixture.DueDate(birthDue), fixture.WithDoula(who))
}

func (e *env) preview(who reqtest.Option, staffID string) oncall.GapPreview {
	e.T.Helper()
	return reqtest.Expect[oncall.GapPreview](e.T, http.MethodGet, e.previewURL(staffID, gapStart, gapEnd), http.StatusOK, who)
}

func bulkBody(staffID string, births ...oncall.BulkGapBirth) oncall.BulkGapRequest {
	reason := " Sister's wedding "
	return oncall.BulkGapRequest{StaffID: staffID, StartsAt: &gapStart, EndsAt: &gapEnd, Reason: &reason, Births: births}
}

func birthOf(id string, covering *string) oncall.BulkGapBirth {
	return oncall.BulkGapBirth{EngagementID: id, CoveringStaffID: covering}
}

func (e *env) record(who reqtest.Option, key string, body oncall.BulkGapRequest) oncall.BulkGapResponse {
	e.T.Helper()
	return reqtest.Expect[oncall.BulkGapResponse](e.T, http.MethodPost, e.bulkURL(), http.StatusCreated,
		who, reqtest.Header("Idempotency-Key", key), reqtest.JSON(body))
}

// expectBulkRefusal posts body under key and wants a 400 naming want under field.
func (e *env) expectBulkRefusal(t *testing.T, key string, body oncall.BulkGapRequest, field, want string) {
	t.Helper()
	got := reqtest.Expect[refusal](t, http.MethodPost, e.bulkURL(), http.StatusBadRequest,
		e.SignedIn, reqtest.Header("Idempotency-Key", key), reqtest.JSON(body))
	if msg, _ := got.Details[field].(string); msg != want {
		t.Fatalf("details = %v, want %q under %q", got.Details, want, field)
	}
}

func (e *env) gapRows() int {
	e.T.Helper()
	return e.count(`SELECT count(*) FROM engagement_coverage_gaps`)
}

func TestBulkGap_ADoulaRecordsOnEveryBirthShesOnCall(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()
	camille := e.secondBirth(e.Doula)
	me := e.As(e.Doula)

	got := e.preview(me, e.Doula.ID)
	if len(got.Births) != 2 || got.Births[0].ClientName != adaWhitfield || got.Births[1].ClientName != camilleBoyd {
		t.Fatalf("preview = %+v, want Ada then Camille", got.Births)
	}
	if opts := got.Births[0].CoverOptions; len(opts) != 1 || opts[0].StaffID != backup.ID {
		t.Errorf("Ada's cover options = %+v, want the backup", opts)
	}
	if opts := got.Births[1].CoverOptions; opts == nil || len(opts) != 0 {
		t.Errorf("Camille's cover options = %+v, want an empty list", opts)
	}
	if got.Births[0].ExistingGap != nil || !got.Births[0].StartsAt.Equal(gapStart) || !got.Births[0].EndsAt.Equal(gapEnd) {
		t.Errorf("Ada = %+v, want the whole time and no gap yet", got.Births[0])
	}

	out := e.record(me, "k-1", bulkBody(e.Doula.ID, birthOf(e.Birth.ID, &backup.ID), birthOf(camille.ID, nil)))

	if len(out.Gaps) != 2 || out.Gaps[0].EngagementID != e.Birth.ID || out.Gaps[1].EngagementID != camille.ID {
		t.Fatalf("gaps = %+v, want one on each birth in the request's order", out.Gaps)
	}
	if out.Gaps[0].CoveringStaffID == nil || *out.Gaps[0].CoveringStaffID != backup.ID || out.Gaps[1].CoveringStaffID != nil {
		t.Errorf("covering = %v and %v, want the backup then nobody", out.Gaps[0].CoveringStaffID, out.Gaps[1].CoveringStaffID)
	}
	if r := out.Gaps[0].Reason; r == nil || strings.HasPrefix(*r, " ") {
		t.Errorf("reason = %v, want it trimmed", r)
	}
	for _, g := range out.Gaps {
		actors := testdb.Column[string](t, e.DB.Admin,
			`SELECT actor_staff_id FROM activity WHERE subject_id = $1 AND action = 'coverage_gap_created'`, g.EngagementID)
		if len(actors) != 1 || actors[0] != e.Doula.ID {
			t.Errorf("created entries on %s = %v, want one naming the Doula", g.EngagementID, actors)
		}
		by := testdb.Scalar[string](t, e.DB.Admin, `SELECT created_by::text FROM engagement_coverage_gaps WHERE id = $1`, g.ID)
		if by != e.Doula.ID {
			t.Errorf("created_by = %s, want the Doula", by)
		}
	}
	if n := e.pendingNotices(out.Gaps[0].ID); n != 0 {
		t.Errorf("the covered gap queued %d notices, want none", n)
	}
	if n := e.pendingNotices(out.Gaps[1].ID); n != 1 {
		t.Errorf("the uncovered gap queued %d notices, want 1", n)
	}
	if calls := e.enq.Calls(); len(calls) != 1 || calls[0] != tasknudge.CoverageGap {
		t.Errorf("nudges = %v, want one %s", calls, tasknudge.CoverageGap)
	}
}

func TestBulkGap_OneNoticeForEachBirthLeftWithNobodyCovering(t *testing.T) {
	e := newEnv(t)
	camille := e.secondBirth(e.Doula)

	out := e.record(e.SignedIn, "k-1", bulkBody(e.Doula.ID, birthOf(e.Birth.ID, nil), birthOf(camille.ID, nil)))

	if n := e.count(`SELECT count(*) FROM coverage_gap_outbox WHERE status = 'pending'`); n != 2 || len(out.Gaps) != 2 {
		t.Fatalf("pending notices = %d for %d gaps, want one each", n, len(out.Gaps))
	}
	if calls := e.enq.Calls(); len(calls) != 1 {
		t.Errorf("nudges = %d, want the worker nudged once", len(calls))
	}
}

func TestBulkGap_AnOwnerRecordsForANamedDoulaAndIsTheActor(t *testing.T) {
	e := newEnv(t)

	out := e.record(e.SignedIn, "k-1", bulkBody(e.Doula.ID, birthOf(e.Birth.ID, nil)))

	if out.Gaps[0].StaffID != e.Doula.ID {
		t.Fatalf("gap is for %s, want the named Doula", out.Gaps[0].StaffID)
	}
	if got := e.activityActors("coverage_gap_created"); len(got) != 1 || got[0] != e.Staff.ID {
		t.Errorf("created entries = %v, want one naming the Owner", got)
	}
}

func TestBulkGap_ADoulaIsRefusedForAnother(t *testing.T) {
	e := newEnv(t)
	colleague := e.backup()

	reqtest.ExpectStatus(t, http.MethodGet, e.previewURL(e.Doula.ID, gapStart, gapEnd), http.StatusForbidden, e.As(colleague))
	reqtest.ExpectStatus(t, http.MethodPost, e.bulkURL(), http.StatusForbidden,
		e.As(colleague), reqtest.Header("Idempotency-Key", "k-1"), reqtest.JSON(bulkBody(e.Doula.ID, birthOf(e.Birth.ID, nil))))
	if n := e.gapRows(); n != 0 {
		t.Fatalf("%d gaps written by a refused request", n)
	}
}

func TestBulkGap_AContractorSeesOnlyHerOwnBirths(t *testing.T) {
	e := newEnv(t)
	contractor := fixture.Staff(t, e.DB, e.Practice, fixture.StaffName("Lena Vasquez"), fixture.Contractor())
	camille := e.secondBirth(e.Doula)
	testdb.SeedGrantedAttachment(t, e.DB, camille.ID, contractor.ID)
	me := e.As(contractor)

	got := e.preview(me, contractor.ID)
	if len(got.Births) != 1 || got.Births[0].EngagementID != camille.ID {
		t.Fatalf("preview = %+v, want only Camille's birth", got.Births)
	}
	if opts := got.Births[0].CoverOptions; len(opts) != 1 || opts[0].StaffID != e.Doula.ID {
		t.Errorf("cover options = %+v, want the Doula on her birth", opts)
	}

	reqtest.ExpectStatus(t, http.MethodPost, e.bulkURL(), http.StatusConflict,
		me, reqtest.Header("Idempotency-Key", "k-1"), reqtest.JSON(bulkBody(contractor.ID, birthOf(e.Birth.ID, nil))))
	e.record(me, "k-2", bulkBody(contractor.ID, birthOf(camille.ID, nil)))
}

func TestBulkGap_IsCutToHerOwnDays(t *testing.T) {
	e := newEnv(t)
	backup := e.backup()
	from, to := "2026-10-18", "2026-10-17"
	e.narrow(e.Doula.ID, oncall.NarrowingRequest{To: &to})
	e.narrow(backup.ID, oncall.NarrowingRequest{From: &from})

	mine := e.preview(e.SignedIn, e.Doula.ID)
	if len(mine.Births) != 1 || !mine.Births[0].StartsAt.Equal(gapStart) {
		t.Fatalf("her preview = %+v, want the birth from the start of the time", mine.Births)
	}
	endOfHerDays := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	if !mine.Births[0].EndsAt.Equal(endOfHerDays) {
		t.Fatalf("her time ends %s, want her last day's end %s", mine.Births[0].EndsAt, endOfHerDays)
	}
	theirs := e.preview(e.SignedIn, backup.ID)
	if len(theirs.Births) != 1 || !theirs.Births[0].StartsAt.Equal(endOfHerDays) || !theirs.Births[0].EndsAt.Equal(gapEnd) {
		t.Fatalf("the backup's preview = %+v, want her days only", theirs.Births)
	}

	out := e.record(e.SignedIn, "k-1", bulkBody(e.Doula.ID, birthOf(e.Birth.ID, &backup.ID)))
	if !out.Gaps[0].StartsAt.Equal(gapStart) || !out.Gaps[0].EndsAt.Equal(endOfHerDays) {
		t.Fatalf("gap = %s to %s, want it cut to her days", out.Gaps[0].StartsAt, out.Gaps[0].EndsAt)
	}
}

func TestBulkGap_ABirthWhoseDaysMissTheTimeIsNotHit(t *testing.T) {
	e := newEnv(t)
	e.attachNewBirth("No Due Date", e.Doula)
	after := "2026-12-02"
	e.narrow(e.Doula.ID, oncall.NarrowingRequest{From: &after})

	if got := e.preview(e.SignedIn, e.Doula.ID); len(got.Births) != 0 {
		t.Fatalf("preview = %+v, want no birth: one has no days and the other starts later", got.Births)
	}
	long := gapEnd.AddDate(0, 3, 0)
	got := reqtest.Expect[oncall.GapPreview](t, http.MethodGet, e.previewURL(e.Doula.ID, long, long.Add(time.Hour)), http.StatusOK, e.SignedIn)
	if len(got.Births) != 0 {
		t.Fatalf("preview = %+v, want no birth for a time after every window", got.Births)
	}
}

func TestBulkGap_ABirthWithAGapOfHersAlreadyOverItIsLeftAsItIs(t *testing.T) {
	e := newEnv(t)
	camille := e.secondBirth(e.Doula)
	existing := e.recordGap(e.SignedIn, gapBody(e.Doula.ID, nil))
	e.forgetNudges()

	got := e.preview(e.SignedIn, e.Doula.ID)
	byName := map[string]oncall.GapPreviewBirth{}
	for _, b := range got.Births {
		byName[b.ClientName] = b
	}
	if g := byName[adaWhitfield].ExistingGap; g == nil || g.ID != existing.ID {
		t.Fatalf("Ada's existing gap = %+v, want the recorded one", g)
	}
	if byName[camilleBoyd].ExistingGap != nil {
		t.Fatalf("Camille has an existing gap, want none")
	}

	reqtest.ExpectStatus(t, http.MethodPost, e.bulkURL(), http.StatusConflict,
		e.SignedIn, reqtest.Header("Idempotency-Key", "k-1"),
		reqtest.JSON(bulkBody(e.Doula.ID, birthOf(e.Birth.ID, nil), birthOf(camille.ID, nil))))

	out := e.record(e.SignedIn, "k-2", bulkBody(e.Doula.ID, birthOf(camille.ID, nil)))
	if len(out.Gaps) != 1 || out.Gaps[0].EngagementID != camille.ID {
		t.Fatalf("gaps = %+v, want one on Camille's birth only", out.Gaps)
	}
	if n := e.count(`SELECT count(*) FROM engagement_coverage_gaps WHERE engagement_id = $1`, e.Birth.ID); n != 1 {
		t.Fatalf("Ada's birth holds %d gaps, want the one it had", n)
	}
	if e.nudges() != 1 {
		t.Errorf("nudges = %d, want one for the new gap", e.nudges())
	}
}

func TestBulkGap_ASetThatChangedSinceItWasCheckedIsRefusedWithNothingWritten(t *testing.T) {
	e := newEnv(t)
	camille := e.secondBirth(e.Doula)
	stranger := e.secondBirth(e.stranger())

	tests := []struct {
		name   string
		births []oncall.BulkGapBirth
	}{
		{"a birth was left out", []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil)}},
		{"a birth the time does not meet was added", []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(camille.ID, nil), birthOf(stranger.ID, nil)}},
		{"a birth is named twice", []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(e.Birth.ID, nil)}},
		{"a birth is not one of hers", []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(stranger.ID, nil)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reqtest.Expect[refusal](t, http.MethodPost, e.bulkURL(), http.StatusConflict,
				e.SignedIn, reqtest.Header("Idempotency-Key", "k-"+tt.name), reqtest.JSON(bulkBody(e.Doula.ID, tt.births...)))
			if got.Message != oncall.MsgBirthsChanged {
				t.Fatalf("message = %q, want %q", got.Message, oncall.MsgBirthsChanged)
			}
		})
	}
	if n := e.gapRows(); n != 0 {
		t.Fatalf("%d gaps written by refused requests", n)
	}
}

func TestBulkGap_ReplayingTheSameKeyWritesNothingMore(t *testing.T) {
	e := newEnv(t)
	camille := e.secondBirth(e.Doula)
	body := bulkBody(e.Doula.ID, birthOf(e.Birth.ID, nil), birthOf(camille.ID, nil))

	first := e.record(e.SignedIn, "same-key", body)
	again := e.record(e.SignedIn, "same-key", body)

	if first.Gaps[0].ID != again.Gaps[0].ID || first.Gaps[1].ID != again.Gaps[1].ID {
		t.Fatalf("replay answered %+v, want the first response %+v", again, first)
	}
	if n := e.gapRows(); n != 2 {
		t.Fatalf("%d gaps after a replay, want 2", n)
	}
	if n := e.count(`SELECT count(*) FROM activity WHERE action = 'coverage_gap_created'`); n != 2 {
		t.Fatalf("%d ledger entries after a replay, want 2", n)
	}
}

func TestBulkGap_RefusesWhatItShould(t *testing.T) {
	e := newEnv(t)
	camille := e.secondBirth(e.Doula)
	stranger := e.stranger()
	early := gapStart.Add(-time.Hour)
	long := "x"
	for len([]rune(long)) <= 500 {
		long += long
	}
	hit := []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(camille.ID, nil)}
	elsewhere := []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(camille.ID, &stranger.ID)}
	same := []oncall.BulkGapBirth{birthOf(e.Birth.ID, nil), birthOf(camille.ID, &e.Doula.ID)}

	tests := []struct {
		name  string
		body  oncall.BulkGapRequest
		field string
		want  string
	}{
		{"nobody is named", bulkBody("", hit...), fieldStaffID, oncall.MsgStaffIDNeeded},
		{"no start", oncall.BulkGapRequest{StaffID: e.Doula.ID, EndsAt: &gapEnd, Births: hit}, fieldStartsAt, oncall.MsgStartsAtRequired},
		{"no end", oncall.BulkGapRequest{StaffID: e.Doula.ID, StartsAt: &gapStart, Births: hit}, fieldEndsAt, oncall.MsgEndsAtRequired},
		{"it runs backward", oncall.BulkGapRequest{StaffID: e.Doula.ID, StartsAt: &gapStart, EndsAt: &early, Births: hit}, fieldEndsAt, oncall.MsgEndsBeforeStarts},
		{"no birth is chosen", bulkBody(e.Doula.ID), "births", oncall.MsgNoBirthsChosen},
		{"a cover is not on that birth", bulkBody(e.Doula.ID, elsewhere...), "births[1].coveringStaffId", oncall.MsgNotOnCall},
		{"a cover is the same Doula", bulkBody(e.Doula.ID, same...), "births[1].coveringStaffId", oncall.MsgCoverIsTheSame},
		{"the reason is too long", oncall.BulkGapRequest{StaffID: e.Doula.ID, StartsAt: &gapStart, EndsAt: &gapEnd, Reason: &long, Births: hit}, "reason", oncall.MsgReasonTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.expectBulkRefusal(t, "k-"+tt.name, tt.body, tt.field, tt.want)
		})
	}

	t.Run("no Idempotency-Key", func(t *testing.T) {
		got := reqtest.Expect[refusal](t, http.MethodPost, e.bulkURL(), http.StatusBadRequest, e.SignedIn, reqtest.JSON(bulkBody(e.Doula.ID, hit...)))
		if got.Message != oncall.MsgIdempotencyKeyNeeded {
			t.Fatalf("message = %q", got.Message)
		}
	})
	t.Run("a body that is not JSON", func(t *testing.T) {
		reqtest.ExpectStatus(t, http.MethodPost, e.bulkURL(), http.StatusBadRequest,
			e.SignedIn, reqtest.Header("Idempotency-Key", "k-junk"), reqtest.RawJSON([]byte("{")))
	})
	t.Run("a preview with no times", func(t *testing.T) {
		got := reqtest.Expect[refusal](t, http.MethodGet, e.PracticeURL("/coverage-gaps/preview?staffId="+e.Doula.ID), http.StatusBadRequest, e.SignedIn)
		if got.Details[fieldStartsAt] != oncall.MsgStartsAtRequired || got.Details[fieldEndsAt] != oncall.MsgEndsAtRequired {
			t.Fatalf("details = %v", got.Details)
		}
	})
	if n := e.gapRows(); n != 0 {
		t.Fatalf("%d gaps written by refused requests; a refusal on one birth must write on none", n)
	}
}
