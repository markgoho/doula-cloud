package contracts_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

func (e *env) openedEntries() (count int, actorKind string) {
	e.T.Helper()
	if err := e.DB.Admin.QueryRowContext(e.T.Context(),
		`SELECT count(*), COALESCE(max(actor_kind::text), '') FROM activity WHERE subject_id = $1 AND action = $2`,
		e.Engagement.ID, string(activity.ActionContractOpened),
	).Scan(&count, &actorKind); err != nil {
		e.T.Fatalf("count opens: %v", err)
	}
	return count, actorKind
}

// TestClientGetContractHandler_RecordsTheFirstOpenOnly is #2952: the
// Client's first read of her Sent Contract is stamped on it with the
// clock's instant and recorded once, as her act; a second read moves
// neither.
func TestClientGetContractHandler_RecordsTheFirstOpenOnly(t *testing.T) {
	first := time.Date(2026, 11, 12, 15, 4, 5, 0, time.UTC)
	e := newEnv(t, withPortalLogin(), atInstant(first))
	e.seedContract(statusSent)

	if got := e.contract(); got.OpenedAt != nil {
		t.Fatalf("before any read: openedAt = %v, want none", got.OpenedAt)
	}
	if n, _ := e.openedEntries(); n != 0 {
		t.Fatalf("a Staff read recorded %d opens, want 0", n)
	}

	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
	e.MoveClockTo(first.Add(time.Minute))
	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)

	got := e.contract()
	if got.OpenedAt == nil || !got.OpenedAt.Equal(first) {
		t.Fatalf("openedAt = %v, want the first read, %v", got.OpenedAt, first)
	}
	if n, kind := e.openedEntries(); n != 1 || kind != "client" {
		t.Fatalf("opens = %d by %q, want one, by the Client", n, kind)
	}
}

// TestListContracts_SaysWhenASentContractWasOpened proves the list's Sent row
// carries the Practice-zone day it was opened, and a Sent row not yet
// opened, or any other row, carries none.
func TestListContracts_SaysWhenASentContractWasOpened(t *testing.T) {
	sentAt := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, e.Practice.ID)
	opened, _ := e.seedListed(statusSent, sentAt)
	e.seedListed(statusSent, sentAt)
	e.Exec(`UPDATE contracts SET opened_at = $2 WHERE engagement_id = $1`, opened.ID, time.Date(2026, 11, 12, 3, 30, 0, 0, time.UTC))

	var days []string
	for _, item := range e.list("?view=sent").Items {
		days = append(days, item.OpenedOn)
	}
	if len(days) != 2 || (days[0] != "2026-11-11" || days[1] != "") && (days[0] != "" || days[1] != "2026-11-11") {
		t.Fatalf("openedOn = %q, want one 2026-11-11 (the Practice's day, not UTC's) and one empty", days)
	}
}
