package payments_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newPracticeInvoiceServer mounts GetPracticeInvoicesHandler alone, behind
// the same staffauth.Middleware the real route uses -- the Owner/Admin
// role gate is GatedRouter's declaration at the mount, not this handler's
// own, so it is asserted where the route table is
// (api/gate_guardrail_test.go).
func newPracticeInvoiceServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/invoices",
		staffauth.Middleware(db.App)(payments.GetPracticeInvoicesHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getPracticeInvoices(t *testing.T, srv *httptest.Server, session, practiceID, cursor string) *http.Response {
	t.Helper()
	url := srv.URL + "/practices/" + practiceID + "/invoices"
	if cursor != "" {
		url += "?cursor=" + cursor
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// readPracticeInvoices requests one page and decodes it, closing the body
// itself: the response never crosses back to the caller, which is both
// what bodyclose wants to see and one less thing for every test to
// remember. The invalid-cursor test, which asserts a status rather than a
// body, uses getPracticeInvoices directly.
func readPracticeInvoices(t *testing.T, srv *httptest.Server, session, practiceID, cursor string) payments.PracticeInvoicesResponse {
	t.Helper()
	resp := getPracticeInvoices(t, srv, session, practiceID, cursor)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.PracticeInvoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// TestGetPracticeInvoicesHandler_ListsEveryEngagementNewestFirst is gap
// RA-G7 itself: one list answering "who owes us money" without opening
// every Engagement in turn. Each row names the Client and her Engagement,
// so the list is a way in rather than a wall of amounts.
func TestGetPracticeInvoicesHandler_ListsEveryEngagementNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-list"
	practiceID := seedMember(t, db, uid)
	engagementA := seedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	engagementB := seedEngagement(t, db, practiceID, "Bea Client", "bea@example.com")
	contractA := seedContract(t, db, engagementA)
	contractB := seedContract(t, db, engagementB)
	base := time.Now().Add(-time.Hour)
	older := seedInvoice(t, db, practiceID, contractA, "in_prac_a", invoiceStatusOpen, 15000, base)
	newer := seedInvoice(t, db, practiceID, contractB, "in_prac_b", invoiceStatusOpen, 25000, base.Add(time.Minute))

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "")
	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(out.Items))
	}
	if out.Items[0].ID != newer || out.Items[1].ID != older {
		t.Fatalf("items = [%q %q], want [%q %q] (newest first)", out.Items[0].ID, out.Items[1].ID, newer, older)
	}
	if out.Items[0].ClientName != "Bea Client" || out.Items[1].ClientName != "Ada Client" {
		t.Fatalf("client names = [%q %q], want [Bea Client Ada Client]", out.Items[0].ClientName, out.Items[1].ClientName)
	}
	if out.Items[0].EngagementID != engagementB || out.Items[1].EngagementID != engagementA {
		t.Fatalf("engagement ids = [%q %q], want [%q %q]", out.Items[0].EngagementID, out.Items[1].EngagementID, engagementB, engagementA)
	}
	if out.HasMore {
		t.Fatal("hasMore = true, want false")
	}
}

// TestGetPracticeInvoicesHandler_TotalsCoverTheWholeBook proves the
// outstanding and paid totals are of every Invoice at the Practice, not
// of the page -- and that a draft, void or uncollectible Invoice counts
// as neither (a draft never reached the Client, a void was cancelled, an
// uncollectible was written off).
func TestGetPracticeInvoicesHandler_TotalsCoverTheWholeBook(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-totals"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedContract(t, db, engagementID)
	base := time.Now().Add(-time.Hour)
	seedInvoice(t, db, practiceID, contractID, "in_tot_open1", invoiceStatusOpen, 15000, base)
	seedInvoice(t, db, practiceID, contractID, "in_tot_open2", invoiceStatusOpen, 5000, base.Add(time.Second))
	seedInvoice(t, db, practiceID, contractID, "in_tot_paid", "paid", 30000, base.Add(2*time.Second))
	seedInvoice(t, db, practiceID, contractID, "in_tot_draft", "draft", 99900, base.Add(3*time.Second))
	seedInvoice(t, db, practiceID, contractID, "in_tot_void", "void", 88800, base.Add(4*time.Second))
	seedInvoice(t, db, practiceID, contractID, "in_tot_unc", "uncollectible", 77700, base.Add(5*time.Second))

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "")
	if out.OutstandingCents != 20000 {
		t.Fatalf("outstandingCents = %d, want 20000", out.OutstandingCents)
	}
	if out.OutstandingCount != 2 {
		t.Fatalf("outstandingCount = %d, want 2", out.OutstandingCount)
	}
	if out.PaidCents != 30000 {
		t.Fatalf("paidCents = %d, want 30000", out.PaidCents)
	}
}

// TestGetPracticeInvoicesHandler_ExcludesOtherPractices proves the app
// layer's own practice_id filter holds on top of the RLS policy
// 00024_invoices.sql already enforces, so a bug in either one alone
// cannot leak another Practice's book.
func TestGetPracticeInvoicesHandler_ExcludesOtherPractices(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-tenancy"
	practiceID := seedMember(t, db, uid)
	mine := seedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	mineContract := seedContract(t, db, mine)
	mineInvoice := seedInvoice(t, db, practiceID, mineContract, "in_mine", invoiceStatusOpen, 1000, time.Now())

	otherPractice := seedPractice(t, db, "Another Practice")
	theirs := seedEngagement(t, db, otherPractice, "Zoe Client", "zoe@example.com")
	theirsContract := seedContract(t, db, theirs)
	seedInvoice(t, db, otherPractice, theirsContract, "in_theirs", invoiceStatusOpen, 500000, time.Now())

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "")
	if len(out.Items) != 1 || out.Items[0].ID != mineInvoice {
		t.Fatalf("items = %+v, want only %q", out.Items, mineInvoice)
	}
	if out.OutstandingCents != 1000 {
		t.Fatalf("outstandingCents = %d, want 1000", out.OutstandingCents)
	}
}

// TestGetPracticeInvoicesHandler_EmptyBookIsAnEmptyList proves a Practice
// that has never billed gets a 200 with zero totals, not a 404.
func TestGetPracticeInvoicesHandler_EmptyBookIsAnEmptyList(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-empty"
	practiceID := seedMember(t, db, uid)

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "")
	if len(out.Items) != 0 {
		t.Fatalf("items = %d, want 0", len(out.Items))
	}
	if out.OutstandingCents != 0 || out.OutstandingCount != 0 || out.PaidCents != 0 {
		t.Fatalf("totals = (%d, %d, %d), want all zero", out.OutstandingCents, out.OutstandingCount, out.PaidCents)
	}
}

// TestGetPracticeInvoicesHandler_PaidAtRoundTrips proves a paid Invoice
// carries the date it was paid, so the list can say when rather than only
// naming the status.
func TestGetPracticeInvoicesHandler_PaidAtRoundTrips(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-paid-at"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid_at", "paid", 4200, time.Now())
	paidAt := time.Now().UTC().Truncate(time.Second)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = $1 WHERE id = $2`, paidAt, invoiceID); err != nil {
		t.Fatalf("set paid_at: %v", err)
	}

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	out := readPracticeInvoices(t, srv, session, practiceID, "")
	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	if out.Items[0].PaidAt == nil || !out.Items[0].PaidAt.Equal(paidAt) {
		t.Fatalf("paidAt = %v, want %v", out.Items[0].PaidAt, paidAt)
	}
}

// TestGetPracticeInvoicesHandler_PaginatesWithCursor proves a book larger
// than one page sets hasMore/nextCursor, that the cursor resumes exactly
// where the first page stopped, and that the totals stay whole-book on
// every page.
func TestGetPracticeInvoicesHandler_PaginatesWithCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-paginate"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Ada Client", "ada@example.com")
	contractID := seedContract(t, db, engagementID)

	const total = 31
	base := time.Now().Add(-time.Hour)
	ids := make([]string, total)
	for i := range total {
		ids[i] = seedInvoice(t, db, practiceID, contractID, "in_prac_page_"+strconv.Itoa(i), invoiceStatusOpen, int64(1000+i), base.Add(time.Duration(i)*time.Second))
	}

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	first := readPracticeInvoices(t, srv, session, practiceID, "")
	if !first.HasMore || first.NextCursor == nil || *first.NextCursor == "" {
		t.Fatalf("first page hasMore = %v, nextCursor = %v; want true and a cursor", first.HasMore, first.NextCursor)
	}
	if len(first.Items) != 30 {
		t.Fatalf("first page items = %d, want 30", len(first.Items))
	}
	if first.Items[0].ID != ids[total-1] {
		t.Fatalf("first page items[0] = %q, want %q (newest)", first.Items[0].ID, ids[total-1])
	}

	second := readPracticeInvoices(t, srv, session, practiceID, *first.NextCursor)
	if second.HasMore {
		t.Fatal("second page hasMore = true, want false")
	}
	if len(second.Items) != 1 || second.Items[0].ID != ids[0] {
		t.Fatalf("second page items = %+v, want only %q (oldest)", second.Items, ids[0])
	}
	if second.OutstandingCount != total {
		t.Fatalf("second page outstandingCount = %d, want %d", second.OutstandingCount, total)
	}
}

// TestGetPracticeInvoicesHandler_InvalidCursorReturns400 proves a
// caller-supplied cursor that cannot decode is refused rather than
// silently returning the wrong page.
func TestGetPracticeInvoicesHandler_InvalidCursorReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "practice-invoices-bad-cursor"
	practiceID := seedMember(t, db, uid)

	srv, session := newPracticeInvoiceServer(t, db, uid)
	defer srv.Close()

	cases := map[string]string{
		"not valid base64":            "!!!not-valid-base64!!!",
		"valid base64, no separator":  base64.URLEncoding.EncodeToString([]byte("nosep")),
		"valid base64, bad timestamp": base64.URLEncoding.EncodeToString([]byte("not-a-time|some-id")),
	}
	for name, cursor := range cases {
		resp := getPracticeInvoices(t, srv, session, practiceID, cursor)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want %d", name, resp.StatusCode, http.StatusBadRequest)
		}
	}
}
