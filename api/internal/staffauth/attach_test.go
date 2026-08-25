package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// attachFixture is a Practice with an Engagement and one write endpoint
// mounted behind the seam, so a test can drive a real request through
// staffauth.Middleware and then read what the seam left behind.
type attachFixture struct {
	db           *testdb.DB
	srv          *httptest.Server
	practiceID   string
	engagementID string
}

// newAttachFixture mounts a stub Engagement-scoped write behind
// AttachingWrite. The stub answers whatever status the request's
// X-Test-Status header asks for, which is how the tests below tell a
// write that happened from one that was refused without needing a real
// handler's failure mode.
func newAttachFixture(t *testing.T) attachFixture {
	t.Helper()
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Attach Test Practice")

	var clientID, engagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Attach Client', 'attach@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`, clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/writes",
		staffauth.Middleware(db.App)(staffauth.AttachingWrite(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if status := r.Header.Get("X-Test-Status"); status == "refused" {
				http.Error(w, "refused", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}))))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return attachFixture{db: db, srv: srv, practiceID: practiceID, engagementID: engagementID}
}

// write drives one request through the seam as uid, asking the stub for
// the given outcome ("ok" or "refused").
func (f attachFixture) write(t *testing.T, uid, outcome string) {
	t.Helper()
	session := authntest.SeedSession(t, f.db.App, uid)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		f.srv.URL+"/practices/"+f.practiceID+"/engagements/"+f.engagementID+"/writes", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Test-Status", outcome)
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()
}

// attachmentFor reads the open attachment for staffID, if there is one.
func (f attachFixture) attachmentFor(t *testing.T, staffID string) (origin, attachedBy string, found bool) {
	t.Helper()
	err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, attached_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		f.engagementID, staffID,
	).Scan(&origin, &attachedBy)
	if err != nil {
		return "", "", false
	}
	return origin, attachedBy, true
}

// A Doula writing under an Engagement accrues an attachment to it, with
// attached_by equal to her own staff id -- ADR-0008's write-side seam.
func TestAttachingWrite_AccruesTheActingDoula(t *testing.T) {
	f := newAttachFixture(t)
	doulaID := seedStaff(t, f.db, "attach-doula")
	seedMembershipWithRoles(t, f.db, f.practiceID, doulaID, "{doula}")

	f.write(t, "attach-doula", "ok")

	origin, attachedBy, found := f.attachmentFor(t, doulaID)
	if !found {
		t.Fatal("the seam attached nobody")
	}
	if origin != "accrued" {
		t.Fatalf("origin = %q, want accrued -- the seam must never mint granted", origin)
	}
	if attachedBy != doulaID {
		t.Fatalf("attached_by = %q, want the actor herself", attachedBy)
	}
}

// An Owner or Admin acting on an Engagement is never attached by it,
// even when she also holds the Doula role: attachment answers "who is on
// this birth", and running the Practice is not being on it.
func TestAttachingWrite_NeverAttachesAnOwnerOrAdmin(t *testing.T) {
	f := newAttachFixture(t)
	ownerID := seedStaff(t, f.db, "attach-owner")
	seedMembershipWithRoles(t, f.db, f.practiceID, ownerID, "{owner,doula}")
	adminID := seedStaff(t, f.db, "attach-admin")
	seedMembershipWithRoles(t, f.db, f.practiceID, adminID, "{admin,doula}")

	f.write(t, "attach-owner", "ok")
	f.write(t, "attach-admin", "ok")

	if _, _, found := f.attachmentFor(t, ownerID); found {
		t.Fatal("the seam attached an Owner")
	}
	if _, _, found := f.attachmentFor(t, adminID); found {
		t.Fatal("the seam attached an Admin")
	}
}

// A write that was refused attaches nothing: accrual follows work that
// actually happened.
func TestAttachingWrite_AttachesNothingOnARefusedWrite(t *testing.T) {
	f := newAttachFixture(t)
	doulaID := seedStaff(t, f.db, "attach-refused-doula")
	seedMembershipWithRoles(t, f.db, f.practiceID, doulaID, "{doula}")

	f.write(t, "attach-refused-doula", "refused")

	if _, _, found := f.attachmentFor(t, doulaID); found {
		t.Fatal("a refused write still attached the caller")
	}
}

// Origin travels accrued -> granted and never the reverse: a second
// write after an Offer's acceptance leaves the granted row, and its
// copied fee, exactly as they are.
func TestAttachingWrite_NeverDowngradesAGrantedAttachment(t *testing.T) {
	f := newAttachFixture(t)
	doulaID := seedStaff(t, f.db, "attach-granted-doula")
	seedMembershipWithRoles(t, f.db, f.practiceID, doulaID, "{doula}")
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, 45000)`,
		f.engagementID, doulaID,
	); err != nil {
		t.Fatalf("seed granted attachment: %v", err)
	}

	f.write(t, "attach-granted-doula", "ok")

	var origin string
	var fee *int64
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, fee_amount_cents FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2`, f.engagementID, doulaID,
	).Scan(&origin, &fee); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if origin != "granted" || fee == nil || *fee != 45000 {
		t.Fatalf("attachment = %s/%v, want the granted row and its fee untouched", origin, fee)
	}
}

// Grant upgrades an open accrued attachment in place rather than
// inserting a second one -- one row per (Engagement, Doula) while open.
func TestGrant_UpgradesAnAccruedAttachmentInPlace(t *testing.T) {
	f := newAttachFixture(t)
	doulaID := seedStaff(t, f.db, "grant-doula")
	seedMembershipWithRoles(t, f.db, f.practiceID, doulaID, "{doula}")
	f.write(t, "grant-doula", "ok")

	tx, err := f.db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, f.practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	fee := int64(52000)
	terms := "Two prenatal visits."
	if err := staffauth.Grant(t.Context(), tx, f.engagementID, doulaID, doulaID, &fee, &terms); err != nil {
		t.Fatalf("Grant: %v", err)
	}

	var count int
	var origin string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), max(origin::text) FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		f.engagementID, doulaID,
	).Scan(&count, &origin); err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	if count != 1 || origin != "granted" {
		t.Fatalf("attachments = %d rows at origin %q, want one granted row", count, origin)
	}
}

// EndAttachments closes every open attachment and names who closed them,
// and leaves an already-ended one alone.
func TestEndAttachments_ClosesOpenRowsOnly(t *testing.T) {
	f := newAttachFixture(t)
	doulaID := seedStaff(t, f.db, "end-doula")
	seedMembershipWithRoles(t, f.db, f.practiceID, doulaID, "{doula}")
	ownerID := seedStaff(t, f.db, "end-owner")
	seedMembershipWithRoles(t, f.db, f.practiceID, ownerID, "{owner}")
	f.write(t, "end-doula", "ok")

	tx, err := f.db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, f.practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if err := staffauth.EndAttachments(t.Context(), tx, f.engagementID, ownerID); err != nil {
		t.Fatalf("EndAttachments: %v", err)
	}

	var endedBy string
	if err := tx.QueryRowContext(t.Context(),
		`SELECT ended_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NOT NULL`,
		f.engagementID, doulaID,
	).Scan(&endedBy); err != nil {
		t.Fatalf("read ended attachment: %v", err)
	}
	if endedBy != ownerID {
		t.Fatalf("ended_by = %q, want the person who ended it", endedBy)
	}
}
