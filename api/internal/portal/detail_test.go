package portal_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/reqtest"
)

func TestDetailHandler_Success(t *testing.T) {
	e := newEnv(t,
		fixture.InPractice(fixture.PracticeName("Riverside Doulas")),
		fixture.WithEngagement(fixture.DueDate("2027-06-15")),
	)

	out := e.detail()

	if out.EngagementID != e.Engagement.ID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, e.Engagement.ID)
	}
	// The portal bar's avatar needs a name of the Client's own (#452), and
	// after ADR-0017 that is a display name rather than a column.
	if out.ClientName != "Test Client" {
		t.Fatalf("clientName = %q, want %q", out.ClientName, "Test Client")
	}
	if out.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", out.PracticeName, "Riverside Doulas")
	}
	if out.PracticeTimezone != "America/New_York" {
		t.Fatalf("practiceTimezone = %q, want %q", out.PracticeTimezone, "America/New_York")
	}
	// #505: the portal reads the Engagement's due date.
	if out.DueDate == nil || *out.DueDate != "2027-06-15" {
		t.Fatalf("dueDate = %v, want %q", out.DueDate, "2027-06-15")
	}
	// #311: a birth Engagement's own read still resolves the question
	// true, the same as before this field existed.
	if !out.OffersBirthPlan {
		t.Fatalf("offersBirthPlan = %v, want true for a birth Engagement", out.OffersBirthPlan)
	}
	// #310: when the Engagement began, back on this DTO for the chrome's
	// switcher label.
	if out.CreatedAt.IsZero() {
		t.Fatalf("createdAt = %v, want a non-zero time", out.CreatedAt)
	}
}

// TestDetailHandler_NullDueDate covers #505's "show nothing" branch: a
// postpartum-only Engagement has no due date (ADR-0017), and the field
// must be absent from the JSON entirely -- `omitempty` is what the page
// relies on to distinguish "nothing to show" from a fetch that broke.
func TestDetailHandler_NullDueDate(t *testing.T) {
	e := newEnv(t)

	raw := e.detailFields()
	if _, present := raw["dueDate"]; present {
		t.Fatalf("dueDate key present in response, want omitted: %v", raw["dueDate"])
	}
}

// TestDetailHandler_PostpartumEngagementDoesNotOfferBirthPlan proves
// #311's own AC: the portal's Engagement read resolves the suppression
// question for a postpartum-only Engagement to false, and the raw kind
// never appears in the response at all -- GLOSSARY.md's Engagement entry
// gives kind no Client word, so no key on this DTO may carry it.
func TestDetailHandler_PostpartumEngagementDoesNotOfferBirthPlan(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement(fixture.Kind(fixture.KindPostpartum)))

	if e.detail().OffersBirthPlan {
		t.Fatalf("offersBirthPlan = true, want false for a postpartum Engagement")
	}
	if raw := e.detailFields(); raw["kind"] != nil {
		t.Fatalf("kind key present in response, want omitted entirely: %s", raw["kind"])
	}
}

func TestDetailHandler_NotLinkedToClient(t *testing.T) {
	e := newEnv(t)
	stranger := fixture.Client(t, e.DB, e.Practice, fixture.WithPortalLogin())

	reqtest.ExpectStatus(t, http.MethodGet, e.engagementURL(""), http.StatusForbidden, e.As(stranger))
}

// TestDetailHandler_TotalToPayCents proves the hub's one line (#1011): the
// sum of the Engagement's open Invoices, and nothing else -- not a paid,
// void or draft one.
func TestDetailHandler_TotalToPayCents(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement(fixture.DueDate("2027-06-15")))

	var contractID string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'signed', 'Test prose', 15000) RETURNING id`,
		e.Engagement.ID).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	for i, inv := range []struct {
		status string
		amount int64
	}{{"open", 4000}, {"open", 5000}, {"paid", 7000}, {"void", 8000}, {"draft", 9000}} {
		ref := "in_detail_" + string(rune('a'+i))
		e.Exec(`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, reference, due_at, paid_at)
			 VALUES ($1, $2, $3, $4::invoice_status, $5, 'usd', $3, now(), CASE WHEN $4::invoice_status = 'paid' THEN now() END)`,
			e.Practice.ID, contractID, ref, inv.status, inv.amount)
	}

	out := e.detail()
	if out.TotalToPayCents != 9000 {
		t.Fatalf("totalToPayCents = %d, want 9000", out.TotalToPayCents)
	}
	if out.OpenInvoiceCount != 2 || out.OnlyOpenInvoiceID != nil {
		t.Fatalf("open invoices = %d, only = %v, want 2 and none", out.OpenInvoiceCount, out.OnlyOpenInvoiceID)
	}

	e.Exec(`UPDATE invoices SET status = 'paid', paid_at = now() WHERE stripe_invoice_id = 'in_detail_b'`)
	var onlyID string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM invoices WHERE stripe_invoice_id = 'in_detail_a'`).Scan(&onlyID); err != nil {
		t.Fatalf("read id: %v", err)
	}
	out = e.detail()
	if out.OpenInvoiceCount != 1 || out.OnlyOpenInvoiceID == nil || *out.OnlyOpenInvoiceID != onlyID {
		t.Fatalf("open invoices = %d, only = %v, want 1 and %s", out.OpenInvoiceCount, out.OnlyOpenInvoiceID, onlyID)
	}
}

// TestDetailHandler_IsOneOfSeveral proves #2276's gate: false while her
// Client record holds one Engagement, true on each of two once a second
// is added at the same Practice.
func TestDetailHandler_IsOneOfSeveral(t *testing.T) {
	e := newEnv(t)
	read := func(engagementID string) bool {
		t.Helper()
		url := e.URL("/api/portal/engagements/" + engagementID)
		return reqtest.Expect[portal.Detail](t, http.MethodGet, url, http.StatusOK, e.AsClient).IsOneOfSeveral
	}

	if read(e.Engagement.ID) {
		t.Fatalf("isOneOfSeveral = true with one Engagement, want false")
	}

	second := e.AnotherEngagement(fixture.ForClient(e.Engagement.Client), fixture.Kind(fixture.KindPostpartum))
	for _, id := range []string{e.Engagement.ID, second.ID} {
		if !read(id) {
			t.Fatalf("isOneOfSeveral = false on %s with two Engagements, want true", id)
		}
	}
}
