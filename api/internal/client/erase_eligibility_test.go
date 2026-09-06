package client_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// #691: the eligibility precheck an Owner reads before ever reaching the
// erasure confirmation. It answers the same question EraseHandler's own
// 409 would otherwise be the only way to learn.

func eraseEligibility(t *testing.T, session, baseURL, practiceID, clientID string) *http.Response {
	t.Helper()
	return authedGet(t, session, baseURL+"/practices/"+practiceID+"/clients/"+clientID+"/erasure")
}

func decodeEligibility(t *testing.T, resp *http.Response) client.EraseEligibility {
	t.Helper()
	defer resp.Body.Close()
	var out client.EraseEligibility
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode eligibility: %v", err)
	}
	return out
}

// TestEraseEligibilityHandler_ClearWithNoInvoices -- a Client with no
// billing history at all is immediately eligible: not erased, and no
// invoice stands in the way.
func TestEraseEligibilityHandler_ClearWithNoInvoices(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-eligibility-clear"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := eraseEligibility(t, session, srv.URL, practiceID, clientID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	out := decodeEligibility(t, resp)
	if out.ErasedAt != nil {
		t.Fatalf("erasedAt = %v, want absent for a Client who was never erased", out.ErasedAt)
	}
	if out.UnsettledInvoices == nil || len(out.UnsettledInvoices) != 0 {
		t.Fatalf("unsettledInvoices = %+v, want an empty slice", out.UnsettledInvoices)
	}
}

// TestEraseEligibilityHandler_NamesUnsettledInvoicesOnly -- the same
// fact EraseHandler's own 409 checks: draft and open invoices block, a
// paid one does not, and every name shown is one the eventual 409 would
// have named too.
func TestEraseEligibilityHandler_NamesUnsettledInvoicesOnly(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-eligibility-invoices"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_open", "open", time.Hour)
	seedInvoicedClient(t, db, practiceID, clientID, "cus_paid", "paid", time.Hour)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := eraseEligibility(t, session, srv.URL, practiceID, clientID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	out := decodeEligibility(t, resp)
	if len(out.UnsettledInvoices) != 1 {
		t.Fatalf("unsettledInvoices = %+v, want exactly the one open invoice", out.UnsettledInvoices)
	}
	got := out.UnsettledInvoices[0]
	if got.Status != "open" {
		t.Fatalf("status = %q, want %q", got.Status, "open")
	}
	if got.AmountCents != 150000 {
		t.Fatalf("amountCents = %d, want %d", got.AmountCents, 150000)
	}
	if got.Currency != "usd" {
		t.Fatalf("currency = %q, want %q", got.Currency, "usd")
	}
	if got.InvoiceID == "" {
		t.Fatal("invoiceId is blank")
	}

	// The same fact, confirmed against the actual 409: attempting the
	// erasure now refuses on exactly the invoice the precheck named.
	postResp := postErasure(t, session, srv, practiceID, clientID)
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusConflict {
		t.Fatalf("erasure status = %d, want %d given the precheck found an unsettled invoice", postResp.StatusCode, http.StatusConflict)
	}
}

// TestEraseEligibilityHandler_AlreadyErased -- once erased, the precheck
// says so and stops naming invoices; the act cannot run again regardless.
func TestEraseEligibilityHandler_AlreadyErased(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-eligibility-erased"
	practiceID, staffID := seedOwner(t, db, uid)
	clientID := seedFullClient(t, db, practiceID, staffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	first := postErasure(t, session, srv, practiceID, clientID)
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("seeding erasure: status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	resp := eraseEligibility(t, session, srv.URL, practiceID, clientID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	out := decodeEligibility(t, resp)
	if out.ErasedAt == nil {
		t.Fatal("erasedAt is absent for a Client who was already erased")
	}
	if len(out.UnsettledInvoices) != 0 {
		t.Fatalf("unsettledInvoices = %+v, want none once erasedAt is set", out.UnsettledInvoices)
	}
}

// TestEraseEligibilityHandler_RefusesEveryRoleButOwner -- the same seat
// as the act it precedes, per ADR-0027 and #394.
func TestEraseEligibilityHandler_RefusesEveryRoleButOwner(t *testing.T) {
	for name, roles := range map[string][]string{
		adminRole: {adminRole},
		doulaRole: {doulaRole},
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "eligibility-role-" + name
			practiceID := testdb.SeedPractice(t, db, "Erasure Practice")
			staffID := testdb.SeedStaffAtPractice(t, db, practiceID, uid, roles, "employee")
			clientID := seedFullClient(t, db, practiceID, staffID)

			srv, session := newErasureServer(t, db, uid)
			defer srv.Close()

			resp := eraseEligibility(t, session, srv.URL, practiceID, clientID)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d -- only an Owner may precheck an erasure", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}

// TestEraseEligibilityHandler_RefusesAnUnknownClient mirrors
// EraseHandler's own refusal: a Client from another Practice reads as
// not found, never forbidden.
func TestEraseEligibilityHandler_RefusesAnUnknownClient(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-eligibility-other-practice"
	practiceID, _ := seedOwner(t, db, uid)
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	otherStaffID := testdb.SeedStaffAtPractice(t, db, otherPracticeID, "other-owner-eligibility", []string{ownerRole}, "employee")
	otherClientID := seedFullClient(t, db, otherPracticeID, otherStaffID)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := eraseEligibility(t, session, srv.URL, practiceID, otherClientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestEraseEligibilityHandler_RefusesAMalformedClientID(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-eligibility-bad-id"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newErasureServer(t, db, uid)
	defer srv.Close()

	resp := eraseEligibility(t, session, srv.URL, practiceID, "not-a-uuid")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
