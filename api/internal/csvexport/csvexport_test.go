package csvexport_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/csvexport/exporttest"
	"doula-cloud/api/internal/fixture"
)

func TestHandler_WritesTheFileThenItsOneActivityRowAtTheRequestsInstant(t *testing.T) {
	now := time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)
	s := exporttest.NewScene(t, false, fixture.SignedInAs(fixture.Owner()), fixture.At(now))

	resp := exporttest.Send(s)
	defer func() { _ = resp.Body.Close() }()

	exporttest.RequireFile(t, resp, "test-2026-11-14.csv", "Test-Count", "1")
	exporttest.RequireRecorded(t, s.DB, s.Practice.ID, s.Staff.ID, exporttest.Action, now, map[string]any{"zone": "America/New_York"})
}

func TestHandler_ARefusalWritesNoRow(t *testing.T) {
	s := exporttest.NewScene(t, true, fixture.SignedInAs(fixture.Owner()))

	resp := exporttest.Send(s)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want the build's own 400", resp.StatusCode)
	}
	if got := exporttest.Diffs(t, s.DB, s.Practice.ID, exporttest.Action); len(got) != 0 {
		t.Fatalf("activity = %v, want none", got)
	}
}

func TestHandler_ThirtyOneDownloadsAnHourAreRefusedTheLast(t *testing.T) {
	s := exporttest.NewScene(t, false, fixture.SignedInAs(fixture.Owner()))

	exporttest.RequireRateLimited(t, func() *http.Response {
		return exporttest.Send(s)
	}, func() int { return len(exporttest.Diffs(t, s.DB, s.Practice.ID, exporttest.Action)) })
}
