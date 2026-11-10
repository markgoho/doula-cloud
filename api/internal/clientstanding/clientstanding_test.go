package clientstanding_test

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestStandings_MatchTheEnum holds the Go names to client_standing's own
// labels (00171), in order, so neither side can gain a standing the other
// does not know.
func TestStandings_MatchTheEnum(t *testing.T) {
	db := testdb.New(t)
	labels := testdb.Scalar[string](t, db.Admin, `SELECT array_to_string(enum_range(NULL::client_standing), ',')`)
	got := make([]string, len(clientstanding.Standings))
	for i, s := range clientstanding.Standings {
		got[i] = string(s)
	}
	if strings.Join(got, ",") != labels {
		t.Fatalf("Standings = %v, want the enum's %s", got, labels)
	}
}

// scene is a Practice with a Client in each of the four standings, each
// with an Engagement, reached through a transaction that sees every row.
type scene struct {
	tx      *sql.Tx
	clients map[clientstanding.Standing]fixture.EngagementRecord
}

func newScene(t *testing.T) scene {
	t.Helper()
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	survivor := fixture.Client(t, db, p)
	s := scene{clients: map[clientstanding.Standing]fixture.EngagementRecord{}}
	for _, standing := range clientstanding.Standings {
		s.clients[standing] = fixture.Engagement(t, db, p)
	}
	fixture.EraseClient(t, db, s.clients[clientstanding.Erased].Client.ID)
	fixture.MergeClient(t, db, s.clients[clientstanding.Merged].Client.ID, survivor.ID)
	mergedThenErased := s.clients[clientstanding.MergedThenErased].Client.ID
	fixture.MergeClient(t, db, mergedThenErased, survivor.ID)
	fixture.EraseClient(t, db, mergedThenErased)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	s.tx = tx
	return s
}

// TestOf_ReadsTheDerivedStanding proves the generated column derives each
// standing from merged_into and erased_at, merged-then-erased included,
// and that Of and OfEngagement read the same answer.
func TestOf_ReadsTheDerivedStanding(t *testing.T) {
	s := newScene(t)
	for want, e := range s.clients {
		got, err := clientstanding.Of(t.Context(), s.tx, e.Client.ID)
		if err != nil || got != want {
			t.Errorf("Of(%s) = %q, %v, want %q", want, got, err, want)
		}
		got, err = clientstanding.OfEngagement(t.Context(), s.tx, e.ID)
		if err != nil || got != want {
			t.Errorf("OfEngagement(%s) = %q, %v, want %q", want, got, err, want)
		}
	}
}

// TestOf_NotFound proves a record the caller cannot see reads as
// ErrNotFound, not as a standing.
func TestOf_NotFound(t *testing.T) {
	s := newScene(t)
	const nobody = "00000000-0000-0000-0000-000000000000"
	if _, err := clientstanding.Of(t.Context(), s.tx, nobody); !errors.Is(err, clientstanding.ErrNotFound) {
		t.Fatalf("Of(nobody) err = %v, want ErrNotFound", err)
	}
}

func TestStanding_IsTombstoneAndIsErased(t *testing.T) {
	cases := []struct {
		s                 clientstanding.Standing
		tombstone, erased bool
	}{
		{clientstanding.Live, false, false},
		{clientstanding.Merged, true, false},
		{clientstanding.Erased, false, true},
		{clientstanding.MergedThenErased, true, true},
	}
	for _, tc := range cases {
		if tc.s.IsTombstone() != tc.tombstone || tc.s.IsErased() != tc.erased {
			t.Errorf("%s: IsTombstone = %v, IsErased = %v, want %v, %v", tc.s, tc.s.IsTombstone(), tc.s.IsErased(), tc.tombstone, tc.erased)
		}
	}
}

// TestAct_CheckSpeaksToThePerson is the refusal a screen prints word for
// word (docs/api-design.md section 7 rule 4): a capital, "you", a full
// stop, the standing the record is actually in, and no pronoun.
func TestAct_CheckSpeaksToThePerson(t *testing.T) {
	cases := []struct {
		act  clientstanding.Act
		s    clientstanding.Standing
		want string
	}{
		{clientstanding.ActAddVisit, clientstanding.Erased, "This Client's details have been erased, so you cannot add a Visit."},
		{clientstanding.ActApproveRequest, clientstanding.Merged, "This Client record has been merged into another, so you cannot approve this request."},
		{clientstanding.ActEdit, clientstanding.MergedThenErased, "This Client record has been merged into another, so you cannot change the details on this record."},
		{clientstanding.ActErase, clientstanding.Standing("archived"), "This Client record is archived, so you cannot erase this record."},
	}
	for _, tc := range cases {
		if ok, refusal := tc.act.Check(tc.s); ok || refusal != tc.want {
			t.Errorf("%q.Check(%q) = %v, %q, want false, %q", tc.act.Does, tc.s, ok, refusal, tc.want)
		}
	}
	if ok, refusal := clientstanding.ActAddVisit.Check(clientstanding.Live); !ok || refusal != "" {
		t.Errorf("Check(live) = %v, %q, want true and no refusal", ok, refusal)
	}
}

// TestActs_EveryOneRefusesEveryStandingButLive walks the registry: an Act
// that admitted a record in another standing would be a write the gate
// does not guard.
func TestActs_EveryOneRefusesEveryStandingButLive(t *testing.T) {
	seen := map[string]bool{}
	for _, act := range clientstanding.Acts {
		if act.Does == "" || seen[act.Does] {
			t.Errorf("Act %+v is unnamed or named twice", act)
		}
		seen[act.Does] = true
		for _, s := range clientstanding.Standings {
			if ok, _ := act.Check(s); ok != (s == clientstanding.Live) {
				t.Errorf("%q.Check(%q) = %v, want %v", act.Does, s, ok, s == clientstanding.Live)
			}
		}
	}
}

// TestRequire_WritesTheRefusalOrPasses proves the HTTP helpers: a live
// record passes and writes nothing, a record in any other standing is
// refused with 409 CONFLICT and the Act's own sentence, and a record the
// caller cannot see passes to the route's own 404.
func TestRequire_WritesTheRefusalOrPasses(t *testing.T) {
	s := newScene(t)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	act := clientstanding.ActSendMessage

	for standing, e := range s.clients {
		for name, require := range map[string]func(w http.ResponseWriter) bool{
			"Require":              func(w http.ResponseWriter) bool { return act.Require(w, r, s.tx, e.Client.ID) },
			"RequireForEngagement": func(w http.ResponseWriter) bool { return act.RequireForEngagement(w, r, s.tx, e.ID) },
		} {
			w := httptest.NewRecorder()
			ok := require(w)
			if standing == clientstanding.Live {
				if !ok || w.Body.Len() != 0 {
					t.Errorf("%s(live) = %v with body %q, want a pass that writes nothing", name, ok, w.Body.String())
				}
				continue
			}
			_, want := act.Check(standing)
			got := apierrtest.Decode(t, w.Result())
			if ok || w.Code != http.StatusConflict || got.Code != apierr.CodeConflict || got.Message != want {
				t.Errorf("%s(%s) = %v, %d %+v, want false, 409 CONFLICT %q", name, standing, ok, w.Code, got, want)
			}
		}
	}

	w := httptest.NewRecorder()
	if !act.Require(w, r, s.tx, "00000000-0000-0000-0000-000000000000") || w.Body.Len() != 0 {
		t.Errorf("Require(unseen) wrote %q, want a pass to the route's own 404", w.Body.String())
	}
}

// TestRequireErr_ReportsARefusal is Require below the HTTP layer.
func TestRequireErr_ReportsARefusal(t *testing.T) {
	s := newScene(t)
	act := clientstanding.ActApproveRequest
	if err := act.RequireErr(t.Context(), s.tx, s.clients[clientstanding.Live].Client.ID); err != nil {
		t.Errorf("RequireErr(live) = %v, want nil", err)
	}
	if err := act.RequireErr(t.Context(), s.tx, "00000000-0000-0000-0000-000000000000"); err != nil {
		t.Errorf("RequireErr(unseen) = %v, want nil", err)
	}
	err := act.RequireErr(t.Context(), s.tx, s.clients[clientstanding.Erased].Client.ID)
	var refusal clientstanding.RefusalError
	_, want := act.Check(clientstanding.Erased)
	if !errors.As(err, &refusal) || refusal.Error() != want {
		t.Errorf("RequireErr(erased) = %v, want the RefusalError %q", err, want)
	}
}
