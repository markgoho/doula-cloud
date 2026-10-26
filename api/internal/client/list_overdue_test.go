package client_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// overdueNow is the request instant these tests pin: 5pm UTC on Dec 10,
// noon in the Practice's New York.
var overdueNow = time.Date(2026, 12, 10, 17, 0, 0, 0, time.UTC)

// lateDueAt is 2026-11-24T00:00Z, which is the evening of Nov 23 in New
// York, so at overdueNow the Invoice is 17 Practice days late -- the same
// pair payments' TestAnOverdueInvoiceReadsOverdueInEveryView counts.
var lateDueAt = time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)

// newServerAt is newServer with the request clock pinned at now, the seam
// clock.Now(ctx) reads (#773).
func newServerAt(t *testing.T, db *testdb.DB, uid string, now time.Time) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	client.Mount(g, ir, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(clock.Middleware(func() time.Time { return now })(mux)), authntest.SeedSessionAt(t, db.App, uid, now)
}

// seedClientInvoiceDue is seedClientInvoice with the due instant stated.
func seedClientInvoiceDue(t *testing.T, db *testdb.DB, practiceID, contractID string, dueAt time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at)
		 VALUES ($1, $2, gen_random_uuid()::text, 'open', 150000, gen_random_uuid()::text, $3)`,
		practiceID, contractID, dueAt,
	); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
}

func listAt(t *testing.T, srv *httptest.Server, session, practiceID string) (client.ListResponse, string) {
	t.Helper()
	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("list: status %d, read %v, body %s", resp.StatusCode, err, raw)
	}
	var list client.ListResponse
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return list, string(raw)
}

// TestListHandler_OpenEngagementsRollup_OverdueInvoiceReadsOverdue proves
// #2130: the Clients list's rollup line carries the count every other
// Staff read of an Invoice carries (#2122, ADR-0038), so an Invoice 17
// days late reads Overdue here as it does on its Engagement and on the
// Invoices list, and an open Invoice not yet due reads 0.
func TestListHandler_OpenEngagementsRollup_OverdueInvoiceReadsOverdue(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Overdue Rollup Practice")
	const ownerUID = "staff-overdue-rollup-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	lateClient, lateEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Imani Brooks", "imani@example.com", "active")
	seedClientInvoiceDue(t, db, practiceID, seedClientContract(t, db, lateEngagement, "signed"), lateDueAt)
	onTimeClient, onTimeEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "On Time Client", "on-time@example.com", "active")
	seedClientInvoiceDue(t, db, practiceID, seedClientContract(t, db, onTimeEngagement, "signed"), overdueNow.Add(14*24*time.Hour))

	srv, session := newServerAt(t, db, ownerUID, overdueNow)
	defer srv.Close()
	list, _ := listAt(t, srv, session, practiceID)

	days := map[string]*int{}
	for _, item := range list.Items {
		if len(item.OpenEngagements) != 1 {
			t.Fatalf("%s: rollup = %+v, want one line", item.Name, item.OpenEngagements)
		}
		days[item.ClientID] = item.OpenEngagements[0].DaysOverdue
	}
	if got := days[lateClient]; got == nil || *got != 17 {
		t.Fatalf("late line daysOverdue = %v, want 17", got)
	}
	if got := days[onTimeClient]; got == nil || *got != 0 {
		t.Fatalf("on-time line daysOverdue = %v, want 0", got)
	}
}

// TestListHandler_OpenEngagementsRollup_ContractorReadsNoLateness is the
// other side of the same field: lateness is a fact about the Practice's
// Invoice, which ADR-0008 keeps from a contractor Doula's rollup, so it is
// shaped away with the Invoice's status and never reaches her wire.
func TestListHandler_OpenEngagementsRollup_ContractorReadsNoLateness(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Overdue Contractor Practice")
	const contractorUID = "contractor-overdue-rollup"
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)

	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Late Client", "late@example.com", "active")
	seedGrantedAttachmentWithFee(t, db, engagementID, contractorID, 120000)
	seedClientInvoiceDue(t, db, practiceID, seedClientContract(t, db, engagementID, "signed"), lateDueAt)

	srv, session := newServerAt(t, db, contractorUID, overdueNow)
	defer srv.Close()
	list, raw := listAt(t, srv, session, practiceID)

	if len(list.Items) != 1 || len(list.Items[0].OpenEngagements) != 1 {
		t.Fatalf("list = %+v, want her one attached line", list.Items)
	}
	if line := list.Items[0].OpenEngagements[0]; line.DaysOverdue != nil || line.InvoiceStatus != nil {
		t.Fatalf("line = %+v, want no Invoice status and no lateness", line)
	}
	if strings.Contains(raw, "daysOverdue") {
		t.Fatalf("a contractor's list carries lateness: %s", raw)
	}
}
