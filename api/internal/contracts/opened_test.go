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

func (e *env) openURL() string { return e.portalURL("/open") }

func (e *env) openedCount(engagementID string) (count int) {
	e.T.Helper()
	if err := e.DB.Admin.QueryRowContext(e.T.Context(),
		`SELECT count(*) FROM contracts WHERE engagement_id = $1 AND opened_at IS NOT NULL`, engagementID,
	).Scan(&count); err != nil {
		e.T.Fatalf("count opened: %v", err)
	}
	return count
}

// TestClientGetContractHandler_RecordsNothing is #2974: a GET is preloaded
// and prefetched, so reading the Contract leaves it unopened.
func TestClientGetContractHandler_RecordsNothing(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)

	if got := e.contract(); got.OpenedAt != nil {
		t.Fatalf("openedAt = %v after reads, want none", got.OpenedAt)
	}
	if n, _ := e.openedEntries(); n != 0 {
		t.Fatalf("reads recorded %d opens, want 0", n)
	}
}

// TestClientPostOpenContractHandler_RecordsTheFirstOpenOnly is #2952 and
// #2974: the Client's explicit open is stamped with the clock's instant and
// recorded once, as her act; a repeat answers the same and moves neither.
func TestClientPostOpenContractHandler_RecordsTheFirstOpenOnly(t *testing.T) {
	first := time.Date(2026, 11, 12, 15, 4, 5, 0, time.UTC)
	e := newEnv(t, withPortalLogin(), atInstant(first))
	e.seedContract(statusSent)

	reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), http.StatusNoContent, e.AsClient)
	e.MoveClockTo(first.Add(time.Minute))
	reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), http.StatusNoContent, e.AsClient)

	got := e.contract()
	if got.OpenedAt == nil || !got.OpenedAt.Equal(first) {
		t.Fatalf("openedAt = %v, want the first open, %v", got.OpenedAt, first)
	}
	if n, kind := e.openedEntries(); n != 1 || kind != "client" {
		t.Fatalf("opens = %d by %q, want one, by the Client", n, kind)
	}
}

// TestClientPostOpenContractHandler_Refusals proves only the Client, on her
// own Sent Contract, opens it.
func TestClientPostOpenContractHandler_Refusals(t *testing.T) {
	// jscpd:ignore-start -- the foreign-Engagement refusal each portal route's test makes
	t.Run("another Client's Engagement", func(t *testing.T) {
		e := newEnv(t, withPortalLogin())
		other := e.AnotherEngagement()
		seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)

		reqtest.ExpectStatus(t, http.MethodPost, e.portalURLOf(other.ID, "/open"), http.StatusForbidden, e.AsClient)

		if n := e.openedCount(other.ID); n != 0 {
			t.Fatalf("a foreign Client opened %d Contracts, want 0", n)
		}
	})
	// jscpd:ignore-end
	t.Run("no session", func(t *testing.T) {
		e := newEnv(t, withPortalLogin())
		e.seedContract(statusSent)

		reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), http.StatusUnauthorized)
	})
	t.Run("Staff", func(t *testing.T) {
		e := newEnv(t, withPortalLogin())
		e.seedContract(statusSent)

		reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), http.StatusUnauthorized, e.SignedIn)

		if n := e.openedCount(e.Engagement.ID); n != 0 {
			t.Fatalf("Staff opened %d Contracts, want 0", n)
		}
	})
	for _, c := range []struct {
		status string
		want   int
	}{
		{statusDraft, http.StatusNotFound},
		{statusSigned, http.StatusConflict},
		{statusVoided, http.StatusConflict},
	} {
		t.Run(c.status, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			e.seedContract(c.status)

			reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), c.want, e.AsClient)

			if n, _ := e.openedEntries(); n != 0 {
				t.Fatalf("a %s Contract recorded %d opens, want 0", c.status, n)
			}
		})
	}
	t.Run("no Contract yet", func(t *testing.T) {
		e := newEnv(t, withPortalLogin())

		reqtest.ExpectStatus(t, http.MethodPost, e.openURL(), http.StatusNotFound, e.AsClient)
	})
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
