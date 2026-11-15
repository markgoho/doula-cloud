package sentperiod_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/sentperiod"
)

const firstOfOctober = "2026-10-01"

func TestParse(t *testing.T) {
	for name, tc := range map[string]struct {
		from, to string
		want     int
		set      bool
	}{
		"none":            {"", "", http.StatusOK, false},
		"one day only":    {firstOfOctober, "", http.StatusBadRequest, false},
		"bad from":        {"10/01/2026", "2026-10-02", http.StatusBadRequest, false},
		"bad to":          {firstOfOctober, "soon", http.StatusBadRequest, false},
		"to before from":  {"2026-10-02", firstOfOctober, http.StatusBadRequest, false},
		"a single day":    {firstOfOctober, firstOfOctober, http.StatusOK, true},
		"a longer period": {firstOfOctober, "2026-10-31", http.StatusOK, true},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			p, ok := sentperiod.Parse(w, tc.from, tc.to)
			if ok != (tc.want == http.StatusOK) || p.Set() != tc.set {
				t.Fatalf("ok = %v, set = %v, want status %d, set %v", ok, p.Set(), tc.want, tc.set)
			}
			if !ok && w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestBoundsAndClause(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	bind := func(any) string { return "$1" }
	none, _ := sentperiod.Parse(httptest.NewRecorder(), "", "")
	if start, end := none.Bounds(zone); start.Valid || end.Valid || none.Clause("c.sent_at", zone, bind) != "" {
		t.Fatal("no period should bound nothing and add no clause")
	}
	p, _ := sentperiod.Parse(httptest.NewRecorder(), "2026-10-25", "2026-10-25")
	start, end := p.Bounds(zone)
	if !start.Time.Equal(time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC)) || !end.Time.Equal(time.Date(2026, 10, 26, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("bounds = %v, %v, want the Practice's midnights", start.Time, end.Time)
	}
	if got := p.Clause("c.sent_at", zone, bind); got != ` AND c.sent_at >= $1 AND c.sent_at < $1` {
		t.Fatalf("clause = %q", got)
	}
}
