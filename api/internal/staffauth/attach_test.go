package staffauth_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	outcomeOK      = "ok"
	outcomeRefused = "refused"

	originAccrued = "accrued"
	originGranted = "granted"
)

// attachEnv is a Practice with an Engagement and one write endpoint
// mounted behind the seam, so a test can drive a real request through
// staffauth.Middleware and then read what the seam left behind. Nobody is
// signed in: a test names the Staff member who writes.
type attachEnv struct {
	*fixture.Scene
}

// newAttachEnv mounts a stub Engagement-scoped write behind
// AttachingWrite. The stub answers whatever status the request's
// X-Test-Status header asks for, which is how the tests below tell a
// write that happened from one that was refused without needing a real
// handler's failure mode.
func newAttachEnv(t *testing.T) attachEnv {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, _ *idempotency.Router, _ *sql.DB) {
		g.GatedWrite("POST /api/practices/{practiceId}/engagements/{engagementId}/writes", staffauth.AnyStaff,
			staffauth.AttachingWrite(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Test-Status") == outcomeRefused {
					http.Error(w, outcomeRefused, http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
			})))
	}
	return attachEnv{fixture.NewScene(t, mount, fixture.NobodySignedIn(), fixture.WithEngagement())}
}

// write drives one request through the seam as who, asking the stub for
// the given outcome ("ok" or "refused"), and returns the seam's status.
func (e attachEnv) write(who fixture.StaffRecord, outcome string) int {
	e.T.Helper()
	resp := reqtest.Post(e.T, e.PracticeURL("/engagements/"+e.Engagement.ID+"/writes"), e.As(who), reqtest.Header("X-Test-Status", outcome))
	_ = resp.Body.Close()
	return resp.StatusCode
}

// attachmentFor reads the open attachment for staffID, if there is one.
func (e attachEnv) attachmentFor(staffID string) (origin, attachedBy string, found bool) {
	e.T.Helper()
	err := e.DB.Admin.QueryRowContext(e.T.Context(),
		`SELECT origin::text, attached_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		e.Engagement.ID, staffID,
	).Scan(&origin, &attachedBy)
	if err != nil {
		return "", "", false
	}
	return origin, attachedBy, true
}

// A Doula writing under an Engagement accrues an attachment to it, with
// attached_by equal to her own staff id -- ADR-0008's write-side seam.
func TestAttachingWrite_AccruesTheActingDoula(t *testing.T) {
	e := newAttachEnv(t)
	doula := e.Colleague()

	e.write(doula, outcomeOK)

	origin, attachedBy, found := e.attachmentFor(doula.ID)
	if !found {
		t.Fatal("the seam attached nobody")
	}
	if origin != originAccrued {
		t.Fatalf("origin = %q, want accrued -- the seam must never mint granted", origin)
	}
	if attachedBy != doula.ID {
		t.Fatalf("attached_by = %q, want the actor herself", attachedBy)
	}
}

// An Owner or Admin acting on an Engagement is never attached by it,
// even when she also holds the Doula role: attachment answers "who is on
// this birth", and running the Practice is not being on it.
func TestAttachingWrite_NeverAttachesAnOwnerOrAdmin(t *testing.T) {
	e := newAttachEnv(t)
	owner := e.Colleague(fixture.Owner(), fixture.Doula())
	admin := e.Colleague(fixture.Admin(), fixture.Doula())

	e.write(owner, outcomeOK)
	e.write(admin, outcomeOK)

	if _, _, found := e.attachmentFor(owner.ID); found {
		t.Fatal("the seam attached an Owner")
	}
	if _, _, found := e.attachmentFor(admin.ID); found {
		t.Fatal("the seam attached an Admin")
	}
}

// A write that was refused attaches nothing: accrual follows work that
// actually happened.
func TestAttachingWrite_AttachesNothingOnARefusedWrite(t *testing.T) {
	e := newAttachEnv(t)
	doula := e.Colleague()

	e.write(doula, outcomeRefused)

	if _, _, found := e.attachmentFor(doula.ID); found {
		t.Fatal("a refused write still attached the caller")
	}
}

// TestAttachingWrite_ReachByEmploymentAndAttachment runs one write per
// kind of Staff member the seam decides about.
//
// A contractor Doula with no attachment at all is refused before the
// wrapped handler ever runs, and the refusal reads as the same 404 a
// contractor's read gets -- #350, the write-side mirror of #231/#315, so
// she can't tell "doesn't exist" from "not attached". An accrued-only
// attachment is a record of work, never a key (#228): a contractor who has
// merely written here before still can't write again without a granted
// attachment. A contractor who holds an open, granted attachment -- an
// accepted Offer -- writes through the seam like anyone else, but once
// that attachment has ended she is refused again: attachment gates the
// Engagement live, not once. An employee Doula keeps the ambient write
// grant #227 decided -- no attachment needed at all.
func TestAttachingWrite_ReachByEmploymentAndAttachment(t *testing.T) {
	tests := []struct {
		name       string
		contractor bool
		seeded     string
		ended      bool
		want       int
		wantOpen   bool
	}{
		{"an unattached contractor", true, "", false, http.StatusNotFound, false},
		{"a contractor with only an accrued attachment", true, originAccrued, false, http.StatusNotFound, true},
		{"a contractor with a granted attachment", true, originGranted, false, http.StatusCreated, true},
		{"a contractor whose granted attachment has ended", true, originGranted, true, http.StatusNotFound, false},
		{"an employee Doula with no attachment", false, "", false, http.StatusCreated, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newAttachEnv(t)
			var opts []fixture.StaffOption
			if tt.contractor {
				opts = append(opts, fixture.Contractor())
			}
			who := e.Colleague(opts...)
			if tt.seeded != "" {
				testdb.SeedAttachment(t, e.DB, e.Engagement.ID, who.ID, tt.seeded, tt.ended)
			}

			if got := e.write(who, outcomeOK); got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
			if _, _, found := e.attachmentFor(who.ID); found != tt.wantOpen {
				t.Fatalf("open attachment = %v, want %v", found, tt.wantOpen)
			}
		})
	}
}

// A malformed engagementId is left for the wrapped handler's own
// staffauth.ParseUUID to reject -- the seam must not turn a typo'd path
// segment into a 500 by handing it straight to a UUID column.
func TestAttachingWrite_MalformedEngagementIDReachesTheHandler(t *testing.T) {
	mount := func(g *staffauth.GatedRouter, _ *idempotency.Router, _ *sql.DB) {
		g.GatedWrite("POST /api/practices/{practiceId}/engagements/{engagementId}/writes", staffauth.AnyStaff,
			staffauth.AttachingWrite(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "invalid engagement id", http.StatusBadRequest)
			})))
	}
	s := fixture.NewScene(t, mount)

	reqtest.ExpectStatus(t, http.MethodPost, s.PracticeURL("/engagements/not-a-uuid/writes"), http.StatusBadRequest, s.SignedIn)
}

// Origin travels accrued -> granted and never the reverse: a second
// write after an Offer's acceptance leaves the granted row, and its
// copied fee, exactly as they are.
func TestAttachingWrite_NeverDowngradesAGrantedAttachment(t *testing.T) {
	e := newAttachEnv(t)
	doula := e.Colleague()
	e.Exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000)`,
		e.Engagement.ID, doula.ID)

	e.write(doula, outcomeOK)

	var origin string
	var fee *int64
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, fee_amount_cents FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2`, e.Engagement.ID, doula.ID,
	).Scan(&origin, &fee); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if origin != originGranted || fee == nil || *fee != 45000 {
		t.Fatalf("attachment = %s/%v, want the granted row and its fee untouched", origin, fee)
	}
}

// Grant upgrades an open accrued attachment in place rather than
// inserting a second one -- one row per (Engagement, Doula) while open.
func TestGrant_UpgradesAnAccruedAttachmentInPlace(t *testing.T) {
	e := newAttachEnv(t)
	doula := e.Colleague()
	e.write(doula, outcomeOK)

	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
	fee := int64(52000)
	terms := "Two prenatal visits."
	if err := staffauth.Grant(t.Context(), tx, e.Engagement.ID, doula.ID, doula.ID, &fee, &terms); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	var count int
	var origin string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), max(origin::text) FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		e.Engagement.ID, doula.ID,
	).Scan(&count, &origin); err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	if count != 1 || origin != originGranted {
		t.Fatalf("attachments = %d rows at origin %q, want one granted row", count, origin)
	}
}

// newWrittenAttachEnv is an attachEnv in which a Doula has written once,
// so she holds an accrued attachment, beside an Owner, with a transaction
// scoped to the Practice.
func newWrittenAttachEnv(t *testing.T) (e attachEnv, doula, owner fixture.StaffRecord, tx *sql.Tx) {
	t.Helper()
	e = newAttachEnv(t)
	doula = e.Colleague()
	owner = e.Colleague(fixture.Owner())
	e.write(doula, outcomeOK)
	return e, doula, owner, e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
}

// endedBy is who ended the doula's attachment, read inside tx.
func endedBy(t *testing.T, tx testdb.Querier, e attachEnv, doula fixture.StaffRecord) string {
	t.Helper()
	return testdb.Scalar[string](t, tx, `SELECT ended_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NOT NULL`, e.Engagement.ID, doula.ID)
}

// EndGranted ends one person's granted attachment and names who ended it
// (#2596), leaves an accrued one alone, and says whether it ended any.
func TestEndGranted_EndsOnlyAGrantedAttachment(t *testing.T) {
	e, doula, owner, tx := newWrittenAttachEnv(t)

	if ended, err := staffauth.EndGranted(t.Context(), tx, e.Engagement.ID, doula.ID, owner.ID); err != nil || ended {
		t.Fatalf("EndGranted on an accrued attachment = %v, %v, want false", ended, err)
	}
	if err := staffauth.Grant(t.Context(), tx, e.Engagement.ID, doula.ID, owner.ID, nil, nil); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if ended, err := staffauth.EndGranted(t.Context(), tx, e.Engagement.ID, doula.ID, owner.ID); err != nil || !ended {
		t.Fatalf("EndGranted on a granted attachment = %v, %v, want true", ended, err)
	}

	if got := endedBy(t, tx, e, doula); got != owner.ID {
		t.Fatalf("ended_by = %q, want the person who ended it", got)
	}
}

// EndAttachments closes every open attachment and names who closed them,
// and leaves an already-ended one alone.
func TestEndAttachments_ClosesOpenRowsOnly(t *testing.T) {
	e, doula, owner, tx := newWrittenAttachEnv(t)
	if err := staffauth.EndAttachments(t.Context(), tx, e.Engagement.ID, owner.ID); err != nil {
		t.Fatalf("EndAttachments: %v", err)
	}

	if got := endedBy(t, tx, e, doula); got != owner.ID {
		t.Fatalf("ended_by = %q, want the person who ended it", got)
	}
}
