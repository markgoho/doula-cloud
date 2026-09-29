package payments_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// These tests drive the Client-portal Invoice reads (#1011) through
// payments.Mount, the same call main.go makes, with a Client session.

// testPublishableKey is the public key the mounted routes hand a browser.
const testPublishableKey = "pk_test_fixture"

// methodCheck is the manual Payment method these tests seed and read back.
const methodCheck = "check"

// clientMoney is one seeded Client with one signed Contract.
type clientMoney struct {
	practiceID, clientID, engagementID, contractID string
}

func seedClientMoney(t *testing.T, db *testdb.DB, uid, name, email string) clientMoney {
	t.Helper()
	practiceID := testdb.SeedPractice(t, db, "Practice "+uid)
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, name, email)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(uid), clientID)
	return clientMoney{practiceID, clientID, engagementID, seedSignedContract(t, db, engagementID)}
}

func newClientMoneyServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	return newClientMoneyServerWith(t, db, payments.NewFakeClient())
}

// newClientMoneyServerWith mounts the same surface over a Stripe fake the
// test keeps hold of, to assert what did and did not reach Stripe.
func newClientMoneyServerWith(t *testing.T, db *testdb.DB, client payments.Client) *httptest.Server {
	t.Helper()
	return newClientMoneyServerKeyed(t, db, client, testPublishableKey)
}

// newClientMoneyServerKeyed is newClientMoneyServerWith over an explicit
// publishable key, for the deployment that has none.
func newClientMoneyServerKeyed(t *testing.T, db *testdb.DB, client payments.Client, publishableKey string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	payments.Mount(g, ir, client, tasknudge.NoOpEnqueuer{}, db.App, publishableKey)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func portalSession(t *testing.T, db *testdb.DB, uid string) string {
	t.Helper()
	return authntest.SeedSession(t, db.App, testdb.PortalUID(uid))
}

// getPortal issues a GET as session and returns status and body.
func getPortal(t *testing.T, srv *httptest.Server, session, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func clientInvoicesPath(engagementID, suffix string) string {
	return "/api/portal/engagements/" + engagementID + "/invoices" + suffix
}

func readClientInvoices(t *testing.T, srv *httptest.Server, session, engagementID, query string) payments.ClientInvoicesResponse {
	t.Helper()
	status, body := getPortal(t, srv, session, clientInvoicesPath(engagementID, query))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	var out payments.ClientInvoicesResponse
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func seedManualPayment(t *testing.T, db *testdb.DB, invoiceID, method, note string) (paymentID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method, note)
		 VALUES ($1, 5000, now(), 'manual', $2::payment_method, $3) RETURNING id`,
		invoiceID, method, note,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed manual payment: %v", err)
	}
	return paymentID
}

// TestClientListInvoices_OwnEngagementOnlyNoDraftAndTotals proves the
// list: her own Invoices in every non-draft status, newest first, a total
// over the open ones only, and none of another Client's or another
// Engagement's.
func TestClientListInvoices_OwnEngagementOnlyNoDraftAndTotals(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-list"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	other := seedClientMoney(t, db, "client-list-other", "Other Client", "other@example.com")
	base := time.Now().Add(-time.Hour)
	seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_draft", invoiceStatusDraft, 1000, base)
	paid := seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_paid", invoiceStatusPaid, 2000, base.Add(1*time.Minute))
	seedPayment(t, db, paid, "pi_cl_paid", 2000, time.Now())
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = now() WHERE id = $1`, paid); err != nil {
		t.Fatalf("stamp paid_at: %v", err)
	}
	seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_void", invoiceStatusVoid, 3000, base.Add(2*time.Minute))
	seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_open1", invoiceStatusOpen, 4000, base.Add(3*time.Minute))
	seedInvoice(t, db, me.practiceID, me.contractID, "in_cl_open2", invoiceStatusOpen, 5000, base.Add(4*time.Minute))
	seedInvoice(t, db, other.practiceID, other.contractID, "in_cl_theirs", invoiceStatusOpen, 9999, base)

	srv := newClientMoneyServer(t, db)
	out := readClientInvoices(t, srv, portalSession(t, db, uid), me.engagementID, "")

	if len(out.Items) != 4 {
		t.Fatalf("items = %d, want 4 (draft and another Client's Invoice excluded): %+v", len(out.Items), out.Items)
	}
	wantRefs := []string{"in_cl_open2", "in_cl_open1", "in_cl_void", "in_cl_paid"}
	for i, ref := range wantRefs {
		if out.Items[i].Reference != ref {
			t.Fatalf("item %d reference = %q, want %q (newest first)", i, out.Items[i].Reference, ref)
		}
		if out.Items[i].Status == invoiceStatusDraft {
			t.Fatalf("item %d is a draft", i)
		}
	}
	if out.TotalToPayCents != 9000 || out.TotalToPayCount != 2 {
		t.Fatalf("total = %d over %d, want 9000 over 2", out.TotalToPayCents, out.TotalToPayCount)
	}
	if out.HasMore || out.NextCursor != nil {
		t.Fatalf("hasMore = %v cursor = %v, want a single page", out.HasMore, out.NextCursor)
	}
	stripeRow := out.Items[3]
	if stripeRow.BillingMode != string(payments.BillingModeStripe) || stripeRow.PaidAt == nil || stripeRow.PaidMethod != nil {
		t.Fatalf("stripe-paid row = %+v, want stripe rail, a paidAt and no paidMethod", stripeRow)
	}
}

// TestClientListInvoices_PaginatesWithWholeEngagementTotal proves the
// total is the Engagement's, not the page's, on every page.
func TestClientListInvoices_PaginatesWithWholeEngagementTotal(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-page"
	me := seedClientMoney(t, db, uid, "Jordan Client", "jordan@example.com")
	base := time.Now().Add(-2 * time.Hour)
	const n = 32
	for i := range n {
		seedInvoice(t, db, me.practiceID, me.contractID, "in_pg_"+string(rune('a'+i)), invoiceStatusOpen, 100, base.Add(time.Duration(i)*time.Minute))
	}

	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	first := readClientInvoices(t, srv, session, me.engagementID, "")
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items hasMore %v cursor %v, want 30, true, set", len(first.Items), first.HasMore, first.NextCursor)
	}
	second := readClientInvoices(t, srv, session, me.engagementID, "?cursor="+*first.NextCursor)
	if len(second.Items) != 2 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items hasMore %v, want 2, false", len(second.Items), second.HasMore)
	}
	for name, page := range map[string]payments.ClientInvoicesResponse{"first": first, "second": second} {
		if page.TotalToPayCents != n*100 || page.TotalToPayCount != n {
			t.Fatalf("%s page total = %d over %d, want %d over %d", name, page.TotalToPayCents, page.TotalToPayCount, n*100, n)
		}
	}

	status, _ := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "?cursor=not-a-cursor"))
	if status != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want 400", status)
	}
}
