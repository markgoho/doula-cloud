package contracts_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2003: a Contract never renders a merge
// field with an empty value. A contractor may not read the Practice's
// price (ADR-0008, as amended on #282), so the line that states it is
// left out of the document she reads, at the BFF, on every Staff-side
// Contract response -- not only the GET.

// pricedProse is the default Template's shape (staffauth/signup.go): the
// price on a line of its own, between two paragraphs.
const pricedProse = "Care for {{client_name}}.\n\nPrice: {{price}}\n\nBy signing below, both parties agree."

// assertNoPrice fails when any part of out still names the price.
func assertNoPrice(t *testing.T, out contracts.ContractResponse) {
	t.Helper()
	if strings.Contains(out.Prose, "Price") || strings.Contains(out.Prose, "{{price}}") {
		t.Fatalf("prose = %q, want the price line left out for a contractor", out.Prose)
	}
	if out.Prose != "Care for {{client_name}}.\n\nBy signing below, both parties agree." {
		t.Fatalf("prose = %q, want the two paragraphs around it kept, one blank line apart", out.Prose)
	}
	if slices.Contains(out.MergeFields, priceKey) {
		t.Fatalf("mergeFields = %v, want no price field for a contractor", out.MergeFields)
	}
	if _, present := out.Values[priceKey]; present {
		t.Fatalf("values = %+v, want no price value for a contractor", out.Values)
	}
}

func TestGetContractHandler_ContractorReadsNoPriceLine(t *testing.T) {
	db := testdb.New(t)
	const uid = "withheld-price-get-contractor"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, pricedProse)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer func() { _ = resp.Body.Close() }()
	assertNoPrice(t, decodeContractResponse(t, resp, http.StatusOK))
}

// The write responses are Contract-shaped too: a contractor who saves a
// draft's merge fields must not read the price in what comes back.
func TestPutContractHandler_ContractorReadsNoPriceLine(t *testing.T) {
	db := testdb.New(t)
	const uid = "withheld-price-put-contractor"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, pricedProse)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContract(t, srv, session, practiceID, engagementID, contracts.MergeFieldValues{clientNameKey: jamieName})
	defer func() { _ = resp.Body.Close() }()
	out := decodeContractResponse(t, resp, http.StatusOK)
	assertNoPrice(t, out)
	if out.Values[clientNameKey] != jamieName {
		t.Fatalf("values[client_name] = %q, want %q: scope still reaches her", out.Values[clientNameKey], jamieName)
	}
}

// An employed Doula is inside the business (#282): she reads the price
// line, filled.
func TestGetContractHandler_EmployeeReadsThePriceLine(t *testing.T) {
	db := testdb.New(t)
	const uid = "withheld-price-get-employee"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, pricedProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContract(t, srv, session, practiceID, engagementID)
	defer func() { _ = resp.Body.Close() }()
	out := decodeContractResponse(t, resp, http.StatusOK)
	if out.Prose != pricedProse {
		t.Fatalf("prose = %q, want it whole for an employee", out.Prose)
	}
	if !slices.Contains(out.MergeFields, priceKey) || out.Values[priceKey] != testRateAmountDollars {
		t.Fatalf("mergeFields = %v values = %+v, want price filled with %q", out.MergeFields, out.Values, testRateAmountDollars)
	}
}
