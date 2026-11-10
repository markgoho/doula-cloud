package portalinvite_test

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

// TestTransitions_EveryWriterNamesOne is the guardrail the declaration
// exists for, in the shape contracts' lifecycle_test.go takes: a
// non-test file that writes a client_portal_users row must name a
// portalinvite Transition, so a new writer cannot quietly restate its
// own "pending" guard the way accept.go, merge.go and 00140 each once
// did. It walks this package and the client package, the two that write
// the row.
func TestTransitions_EveryWriterNamesOne(t *testing.T) {
	writes := []string{"UPDATE client_portal_users", "INSERT INTO client_portal_users"}
	scanned := 0
	for _, dir := range []string{".", filepath.Join("..", "client")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			// #nosec G304 -- path is built from ReadDir over two fixed
			// package directories, not from anything a caller supplies
			src, err := os.ReadFile(path)
			if err != nil {
				// coverage:ignore reason: a file ReadDir listed a moment ago
				t.Fatalf("read %s: %v", path, err)
			}
			for _, write := range writes {
				if !strings.Contains(string(src), write) {
					continue
				}
				scanned++
				if !strings.Contains(string(src), "Transition") {
					t.Errorf("%s writes %q but names no portalinvite Transition -- a writer of portal access must go through the declaration", path, write)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no file writes client_portal_users -- did invite.go, accept.go or lifecycle.go move?")
	}
}

// TestTransitions_TableIsComplete pins the declared moves, so a change to
// one is a change to this test too.
func TestTransitions_TableIsComplete(t *testing.T) {
	open := []portalinvite.State{portalinvite.StateInvited, portalinvite.StateRefused}
	want := map[string]struct {
		from []portalinvite.State
		to   portalinvite.State
	}{
		"invite her":        {nil, portalinvite.StateInvited},
		"send a new invite": {open, portalinvite.StateInvited},
		"accept it":         {open, portalinvite.StateAccepted},
		"refuse it":         {open, portalinvite.StateRefused},
		"withdraw it":       {open, portalinvite.StateRevoked},
		"unlink it":         {[]portalinvite.State{portalinvite.StateAccepted}, portalinvite.StateUnlinked},
	}
	if len(portalinvite.Transitions) != len(want) {
		t.Fatalf("len(Transitions) = %d, want %d", len(portalinvite.Transitions), len(want))
	}
	for _, tr := range portalinvite.Transitions {
		w, known := want[tr.Act]
		if !known {
			t.Errorf("unexpected Transition %+v", tr)
			continue
		}
		if tr.To != w.to || strings.Join(tr.FromStates(), ",") != joinStates(w.from) {
			t.Errorf("Transition %q = %v -> %q, want %v -> %q", tr.Act, tr.From, tr.To, w.from, w.to)
		}
		for _, s := range w.from {
			if !tr.Allows(s) {
				t.Errorf("Transition %q does not allow %q", tr.Act, s)
			}
		}
		if tr.Allows(portalinvite.StateExpired) {
			t.Errorf("Transition %q allows the derived state expired, which no row is ever stored in", tr.Act)
		}
	}
}

func joinStates(states []portalinvite.State) string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = string(s)
	}
	return strings.Join(out, ",")
}

// TestDerive is the one derivation's whole table: expiry is a function
// of the instant passed in, a delivery is carried only beside a live
// invitation, and no row means never invited.
func TestDerive(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	stored := func(s portalinvite.State) sql.NullString { return sql.NullString{String: string(s), Valid: true} }
	at := func(d time.Duration) sql.NullTime { return sql.NullTime{Time: now.Add(d), Valid: true} }
	sent := sql.NullString{String: testOutboxStatusSent, Valid: true}

	if got := portalinvite.Derive(sql.NullString{}, sql.NullTime{}, sql.NullString{}, now); got != nil {
		t.Fatalf("Derive(no row) = %+v, want nil", got)
	}
	for _, tc := range []struct {
		name     string
		stored   portalinvite.State
		expires  sql.NullTime
		want     portalinvite.State
		delivery string
	}{
		{"a live invitation", portalinvite.StateInvited, at(time.Hour), portalinvite.StateInvited, testOutboxStatusSent},
		{"an invitation at its expiry", portalinvite.StateInvited, at(0), portalinvite.StateExpired, ""},
		{"an invitation past its expiry", portalinvite.StateInvited, at(-time.Hour), portalinvite.StateExpired, ""},
		{"a refused invitation past its expiry", portalinvite.StateRefused, at(-time.Hour), portalinvite.StateRefused, ""},
		{"accepted", portalinvite.StateAccepted, sql.NullTime{}, portalinvite.StateAccepted, ""},
		{"revoked", portalinvite.StateRevoked, sql.NullTime{}, portalinvite.StateRevoked, ""},
		{"unlinked", portalinvite.StateUnlinked, sql.NullTime{}, portalinvite.StateUnlinked, ""},
	} {
		got := portalinvite.Derive(stored(tc.stored), tc.expires, sent, now)
		if got == nil || got.State != tc.want || got.Delivery != tc.delivery {
			t.Errorf("%s: Derive = %+v, want %q with delivery %q", tc.name, got, tc.want, tc.delivery)
		}
	}
}

// TestAccess_Fields proves the wire shape: nil for never invited, and a
// delivery only beside a live invitation that has one.
func TestAccess_Fields(t *testing.T) {
	var none *portalinvite.Access
	if s, d := none.Fields(); s != nil || d != nil {
		t.Fatalf("nil Access fields = %v, %v, want nil, nil", s, d)
	}
	if s, d := (&portalinvite.Access{State: portalinvite.StateAccepted}).Fields(); s == nil || *s != portalinvite.StateAccepted || d != nil {
		t.Fatalf("accepted fields = %v, %v, want accepted, nil", s, d)
	}
	if s, d := (&portalinvite.Access{State: portalinvite.StateInvited, Delivery: statusBounced}).Fields(); s == nil || *s != portalinvite.StateInvited || d == nil || *d != statusBounced {
		t.Fatalf("invited fields = %v, %v, want invited, bounced", s, d)
	}
}

// TestAccess_CanReachPortal is the question a Contract send asks: only
// a Client who signs in, or holds an invitation that can still be
// accepted and whose send did not fail, can open what is sent to her.
func TestAccess_CanReachPortal(t *testing.T) {
	var none *portalinvite.Access
	if none.CanReachPortal() {
		t.Fatal("never invited can reach the portal, want false")
	}
	for _, tc := range []struct {
		access portalinvite.Access
		want   bool
	}{
		{portalinvite.Access{State: portalinvite.StateAccepted}, true},
		{portalinvite.Access{State: portalinvite.StateInvited, Delivery: "pending"}, true},
		{portalinvite.Access{State: portalinvite.StateInvited, Delivery: testOutboxStatusSent}, true},
		{portalinvite.Access{State: portalinvite.StateInvited, Delivery: "complained"}, true},
		{portalinvite.Access{State: portalinvite.StateInvited}, true},
		{portalinvite.Access{State: portalinvite.StateInvited, Delivery: statusBounced}, false},
		{portalinvite.Access{State: portalinvite.StateInvited, Delivery: "dead_lettered"}, false},
		{portalinvite.Access{State: portalinvite.StateExpired}, false},
		{portalinvite.Access{State: portalinvite.StateRefused}, false},
		{portalinvite.Access{State: portalinvite.StateRevoked}, false},
		{portalinvite.Access{State: portalinvite.StateUnlinked}, false},
	} {
		if got := tc.access.CanReachPortal(); got != tc.want {
			t.Errorf("%+v CanReachPortal = %v, want %v", tc.access, got, tc.want)
		}
	}
}

// TestRevoke_RecordsWhoWhenAndWhy drives Revoke directly for the one
// actor no handler test reaches: DoulaCloud acting with nobody asking (a
// Practice deletion), which records no Staff member.
func TestRevoke_RecordsWhoWhenAndWhy(t *testing.T) {
	db := testdb.New(t)
	clientID, _ := seedPendingPortalInvite(t, db)
	practiceID := testdb.PracticeOfClient(t, db, clientID)
	at := time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)

	tx := db.BeginAs(t, testdb.Practice(practiceID))
	revoked, err := portalinvite.Revoke(t.Context(), tx, clientID, "", portalinvite.RevokedClientErased, at)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked != 1 {
		t.Fatalf("revoked = %d, want 1", revoked)
	}
	again, err := portalinvite.Revoke(t.Context(), tx, clientID, "", portalinvite.RevokedClientErased, at)
	if err != nil || again != 0 {
		t.Fatalf("second Revoke = %d, %v; want 0, nil -- a revoked invitation is terminal", again, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var state, reason string
	var by sql.NullString
	var revokedAt time.Time
	var hasToken bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT state, revoke_reason, revoked_by_staff_id::text, revoked_at, invite_token IS NOT NULL FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&state, &reason, &by, &revokedAt, &hasToken); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if state != "revoked" || reason != "client_erased" || by.Valid || !revokedAt.Equal(at) || hasToken {
		t.Fatalf("row = (%s, %s, %v, %v, token %v), want (revoked, client_erased, no Staff member, %v, no token)", state, reason, by, revokedAt, hasToken, at)
	}
}

// TestAcceptInviteHandler_AcceptAfterRefusalClearsIt proves accepted and
// refused can no longer stand together: once the refusal's cause is gone
// (her login no longer reaches another Client here), the same link is
// accepted and the refusal is cleared rather than left for a reader's
// `if` order to outrank.
func TestAcceptInviteHandler_AcceptAfterRefusalClearsIt(t *testing.T) {
	s := newAcceptScene(t)
	testdb.SeedPortalAccount(t, s.DB, "portal_existing-account", invitedAddress)
	clientID, inviteToken := seedPendingPortalInvite(t, s.DB)
	otherClientID, _ := testdb.SeedNamedEngagement(t, s.DB, testdb.PracticeOfClient(t, s.DB, clientID), "Other Record", "other@example.com")
	testdb.AttachPortalUser(t, s.DB, "portal_existing-account", otherClientID)

	acceptInvite(t, s, inviteToken, http.StatusConflict)
	if state := testdb.Scalar[string](t, s.DB.Admin, `SELECT state FROM client_portal_users WHERE client_id = $1`, clientID); state != "refused" {
		t.Fatalf("state = %q, want refused", state)
	}

	s.Exec(`DELETE FROM client_portal_users WHERE client_id = $1`, otherClientID)
	acceptInvite(t, s, inviteToken, http.StatusOK)

	var state string
	var refusedAt sql.NullTime
	if err := s.DB.Admin.QueryRowContext(t.Context(),
		`SELECT state, access_refused_at FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&state, &refusedAt); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if state != "accepted" || refusedAt.Valid {
		t.Fatalf("row = (%s, refused at %v), want accepted with the refusal cleared", state, refusedAt)
	}
}

// TestAcceptInviteHandler_RefusedAndExpiredIsGone proves expiry is asked
// of every live token, a refused one included: Derive still names such a
// row "refused" for the Practice, but the link itself is past its time.
func TestAcceptInviteHandler_RefusedAndExpiredIsGone(t *testing.T) {
	s := newAcceptScene(t)
	clientID, inviteToken := seedPendingPortalInviteExpiringAt(t, s.DB, time.Now().Add(-time.Minute))
	s.Exec(`UPDATE client_portal_users SET state = 'refused', access_refused_at = now() WHERE client_id = $1`, clientID)

	acceptInvite(t, s, inviteToken, http.StatusGone)
}
