package exporttest_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/csvexport/exporttest"
	"doula-cloud/api/internal/fixture"
)

func ownerScene(t *testing.T, refuse bool, opts ...fixture.SceneOption) *fixture.Scene {
	t.Helper()
	return exporttest.NewScene(t, refuse, append([]fixture.SceneOption{fixture.SignedInAs(fixture.Owner())}, opts...)...)
}

func TestRead_ReadsTheFileHeaderFirst(t *testing.T) {
	s := ownerScene(t, false)

	rows := exporttest.Read(t, func() *http.Response { return exporttest.Send(s) })

	if len(rows) != 2 || rows[0][0] != "a" || rows[1][1] != "2" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestRequireFileAndRequireRecorded_ReadTheDownloadAndItsOneRow(t *testing.T) {
	now := time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)
	s := ownerScene(t, false, fixture.At(now))

	resp := exporttest.Send(s)
	defer func() { _ = resp.Body.Close() }()

	exporttest.RequireFile(t, resp, "test-2026-11-14.csv", "Test-Count", "1")
	exporttest.RequireRecorded(t, s.DB, s.Practice.ID, s.Staff.ID, exporttest.Action, now, map[string]any{"zone": "America/New_York"}, "quinn")
}

func TestNewScene_ARefusingBuildAnswersItsOwn400(t *testing.T) {
	s := ownerScene(t, true)

	resp := exporttest.Send(s)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRequireOwnerAndAdminOnly_AsksEachRoleOnce(t *testing.T) {
	exporttest.RequireOwnerAndAdminOnly(t, func(t *testing.T, role fixture.RoleCase) (int, int) {
		s := exporttest.NewScene(t, false, fixture.SignedInAs(role.Opts...))
		resp := exporttest.Send(s)
		_ = resp.Body.Close()
		return resp.StatusCode, len(exporttest.Diffs(t, s.DB, s.Practice.ID, exporttest.Action))
	})
}

func TestRequireRateLimited_TheThirtyFirstIsRefused(t *testing.T) {
	s := ownerScene(t, false)

	exporttest.RequireRateLimited(t, func() *http.Response {
		return exporttest.Send(s)
	}, func() int { return len(exporttest.Diffs(t, s.DB, s.Practice.ID, exporttest.Action)) })
}
