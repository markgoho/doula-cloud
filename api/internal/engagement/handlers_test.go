package engagement_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// testBrokeClientName is shared across this file's zero-balance tests
// per golangci-lint's goconst check.
const testBrokeClientName = "Broke Client"

// portalOutboxStatusPending mirrors testBrokeClientName's role, for
// TestListHandler_PortalInviteStatus's repeated "pending" literal.
const portalOutboxStatusPending = "pending"

func authedGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
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

func authedPost(t *testing.T, session, url string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestListHandler_ReturnsOnlyClientsAtCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-listing"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	otherPracticeID := seedStaffWithMembership(t, db, "staff-elsewhere")
	seedClientEngagement(t, db, practiceID, "Jamie Client", "jamie@example.com", "intake")
	seedClientEngagement(t, db, otherPracticeID, "Other Client", "other@example.com", "intake")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Jamie Client" {
		t.Fatalf("list = %+v, want only Jamie Client", list)
	}
}

// TestListHandler_VisibleToAnyStaffAtSamePractice proves the ticket's "any
// Staff member at a Practice sees all Clients and Engagements there, not
// just ones assigned to them" requirement: a Staff member who neither
// created nor was assigned the Client still sees it, because there is no
// staff_id column anywhere in the schema to restrict on.
func TestListHandler_VisibleToAnyStaffAtSamePractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-creator")
	seedStaffAtPractice(t, db, practiceID, "staff-bystander")
	seedClientEngagement(t, db, practiceID, "Shared Client", "shared@example.com", "intake")

	srv, session := newServer(t, db, "staff-bystander")
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Shared Client" {
		t.Fatalf("list = %+v, want the bystander Staff member to see Shared Client too", list)
	}
}

// TestListHandler_ContractorSeesOnlyAttachedEngagements proves ADR-0008's
// attachment-narrowed column: a contractor Doula's list is scoped to the
// Engagements she holds an open, granted attachment on, not every Client
// at the Practice the way an employee's is.
func TestListHandler_ContractorSeesOnlyAttachedEngagements(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-listing"
	practiceID := seedStaffWithMembership(t, db, "staff-owner-of-practice")
	staffID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, attachedEngagementID := seedClientEngagement(t, db, practiceID, "Attached Client", "attached@example.com", "intake")
	seedClientEngagement(t, db, practiceID, "Unattached Client", "unattached@example.com", "intake")
	seedGrantedAttachment(t, db, attachedEngagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Attached Client" {
		t.Fatalf("list = %+v, want only Attached Client", list)
	}
}

// TestListHandler_PortalInviteStatus covers #346: the Staff Clients list
// surfaces each Client's portal invite state. "accepted" comes from
// client_portal_users.identity_uid, not the outbox row -- accept.go never
// touches portal_invite_outbox, so without this precedence an
// active-for-months Client would still read as whatever their original
// send landed on.
func TestListHandler_PortalInviteStatus(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-portal-status")
	now := time.Now()

	seedClientEngagement(t, db, practiceID, "Never Invited", "never@example.com", "intake")

	pendingClientID, _ := seedClientEngagement(t, db, practiceID, "Pending Client", "pending@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, pendingClientID, false), portalOutboxStatusPending, now)

	sentClientID, _ := seedClientEngagement(t, db, practiceID, "Sent Client", "sent@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, sentClientID, false), "sent", now)

	bouncedClientID, _ := seedClientEngagement(t, db, practiceID, "Bounced Client", "bounced@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, bouncedClientID, false), "bounced", now)

	complainedClientID, _ := seedClientEngagement(t, db, practiceID, "Complained Client", "complained@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, complainedClientID, false), "complained", now)

	deadLetteredClientID, _ := seedClientEngagement(t, db, practiceID, "Dead Lettered Client", "dead@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, deadLetteredClientID, false), "dead_lettered", now)

	acceptedClientID, _ := seedClientEngagement(t, db, practiceID, "Accepted Client", "accepted@example.com", "intake")
	seedOutboxRow(t, db, seedPortalUser(t, db, acceptedClientID, true), "sent", now)

	noOutboxClientID, _ := seedClientEngagement(t, db, practiceID, "No Outbox Client", "nooutbox@example.com", "intake")
	seedPortalUser(t, db, noOutboxClientID, false)

	latestClientID, _ := seedClientEngagement(t, db, practiceID, "Latest Wins Client", "latest@example.com", "intake")
	latestPU := seedPortalUser(t, db, latestClientID, false)
	seedOutboxRow(t, db, latestPU, "bounced", now.Add(-time.Hour))
	seedOutboxRow(t, db, latestPU, portalOutboxStatusPending, now)

	srv, session := newServer(t, db, "staff-portal-status")
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	statusByName := make(map[string]*string, len(list))
	for _, ce := range list {
		statusByName[ce.Name] = ce.PortalInviteStatus
	}

	for _, name := range []string{"Never Invited", "No Outbox Client"} {
		if got := statusByName[name]; got != nil {
			t.Errorf("%s portalInviteStatus = %v, want nil", name, *got)
		}
	}

	wantStatus := map[string]string{
		"Pending Client":       portalOutboxStatusPending,
		"Sent Client":          "sent",
		"Bounced Client":       "bounced",
		"Complained Client":    "complained",
		"Dead Lettered Client": "dead_lettered",
		"Accepted Client":      "accepted",
		"Latest Wins Client":   portalOutboxStatusPending,
	}
	for name, want := range wantStatus {
		got := statusByName[name]
		if got == nil || *got != want {
			t.Errorf("%s portalInviteStatus = %v, want %q", name, got, want)
		}
	}
}

func TestCreateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-creating"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	seedSignupBonus(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "New Client", Email: "new@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out engagement.CreateClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientID == "" || out.EngagementID == "" || out.Status != "intake" {
		t.Fatalf("unexpected response: %+v", out)
	}

	listResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/clients")
	defer listResp.Body.Close()
	var list []engagement.ClientEngagement
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].ClientID != out.ClientID {
		t.Fatalf("expected the created client to appear in the list, got %+v", list)
	}

	var consumptionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE practice_id = $1 AND origin = 'consumption'`,
		practiceID,
	).Scan(&consumptionCount); err != nil {
		t.Fatalf("count consumption rows: %v", err)
	}
	if consumptionCount != 1 {
		t.Fatalf("consumption row count = %d, want exactly 1", consumptionCount)
	}

	var origin string
	var quantity int
	var consumedEngagementID string
	err = db.Admin.QueryRowContext(t.Context(),
		`SELECT origin, quantity, consumed_engagement_id FROM credit_ledger WHERE practice_id = $1 AND origin = 'consumption'`,
		practiceID,
	).Scan(&origin, &quantity, &consumedEngagementID)
	if err != nil {
		t.Fatalf("query consumption row: %v", err)
	}
	if origin != "consumption" || quantity != -1 || consumedEngagementID != out.EngagementID {
		t.Fatalf("consumption row = (%q, %d, %q), want (\"consumption\", -1, %q)", origin, quantity, consumedEngagementID, out.EngagementID)
	}
}

// TestCreateHandler_NoCreditsReturnsPaymentRequired proves a Practice with
// a zero balance is rejected with 402, that the Client/Engagement rows
// written earlier in the same handler are rolled back, not left orphaned,
// and that a pending low_credit_outbox row for the Practice survives that
// same rollback (#342) -- proving it does not ride the rolled-back tx.
func TestCreateHandler_NoCreditsReturnsPaymentRequired(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-broke"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: testBrokeClientName, Email: "broke@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusPaymentRequired)
	}

	var clientCount, engagementCount, ledgerCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&clientCount); err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE practice_id = $1`, practiceID).Scan(&engagementCount); err != nil {
		t.Fatalf("count engagements: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count credit_ledger: %v", err)
	}
	if clientCount != 0 || engagementCount != 0 || ledgerCount != 0 {
		t.Fatalf("rows survived rollback: clients=%d engagements=%d credit_ledger=%d, want all 0", clientCount, engagementCount, ledgerCount)
	}

	var outboxCount int
	var outboxStatus string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(status) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&outboxCount, &outboxStatus); err != nil {
		t.Fatalf("count low_credit_outbox: %v", err)
	}
	if outboxCount != 1 || outboxStatus != "pending" {
		t.Fatalf("low_credit_outbox rows = %d (status %q), want 1 pending row", outboxCount, outboxStatus)
	}
}

// TestCreateHandler_NoCreditsQueuesNotificationOnlyOncePerEpisode proves
// the dedupe rule: a second failed create attempt for the same Practice,
// still out of Credits, does not queue a second low_credit_outbox row.
func TestCreateHandler_NoCreditsQueuesNotificationOnlyOncePerEpisode(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-broke-twice"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	for i, email := range []string{"first@example.com", "second@example.com"} {
		body, err := json.Marshal(engagement.CreateClientRequest{Name: testBrokeClientName, Email: email})
		if err != nil {
			t.Fatalf("marshal body %d: %v", i, err)
		}
		resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusPaymentRequired {
			t.Fatalf("attempt %d status = %d, want %d", i, resp.StatusCode, http.StatusPaymentRequired)
		}
	}

	var outboxCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count low_credit_outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("low_credit_outbox rows = %d, want exactly 1 across both failed attempts", outboxCount)
	}
}

// TestCreateHandler_NoCreditsRequeuesNotificationAfterPurchase proves the
// other half of the dedupe rule: once the Practice buys more Credits, a
// later zero-balance episode queues a new row rather than staying
// suppressed by the first, already-sent notification.
func TestCreateHandler_NoCreditsRequeuesNotificationAfterPurchase(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-broke-again"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstBody, err := json.Marshal(engagement.CreateClientRequest{Name: testBrokeClientName, Email: "first@example.com"})
	if err != nil {
		t.Fatalf("marshal first body: %v", err)
	}
	firstResp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", firstBody)
	defer firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("first attempt status = %d, want %d", firstResp.StatusCode, http.StatusPaymentRequired)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE low_credit_outbox SET status = 'sent' WHERE practice_id = $1`, practiceID,
	); err != nil {
		t.Fatalf("mark first outbox row sent: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'purchase', 1)`, practiceID,
	); err != nil {
		t.Fatalf("seed purchase: %v", err)
	}
	// Spends the purchased credit directly (not through a real create
	// call) so the Practice is back at a zero balance for the second
	// attempt below -- this test is only proving the dedupe window
	// re-arms on a purchase, not exercising consumption itself.
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'consumption', -1)`, practiceID,
	); err != nil {
		t.Fatalf("seed consumption: %v", err)
	}

	secondBody, err := json.Marshal(engagement.CreateClientRequest{Name: "Broke Client Two", Email: "second@example.com"})
	if err != nil {
		t.Fatalf("marshal second body: %v", err)
	}
	secondResp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", secondBody)
	defer secondResp.Body.Close()
	if secondResp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("second attempt status = %d, want %d", secondResp.StatusCode, http.StatusPaymentRequired)
	}

	var outboxCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM low_credit_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count low_credit_outbox: %v", err)
	}
	if outboxCount != 2 {
		t.Fatalf("low_credit_outbox rows = %d, want 2 (one per episode, re-armed by the purchase)", outboxCount)
	}
}

// TestCreateHandler_NegativeBalanceReturnsPaymentRequired proves the
// "zero-or-negative balance" half of the AC that a zero balance alone
// doesn't exercise: a Practice whose ledger nets negative (e.g. a refunded
// purchase clawed back credits already spent) is blocked the same way.
func TestCreateHandler_NegativeBalanceReturnsPaymentRequired(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-negative-balance"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', -1)`,
		practiceID,
	); err != nil {
		t.Fatalf("seed negative balance: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "Negative Client", Email: "negative@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusPaymentRequired)
	}

	var clientCount, engagementCount, ledgerCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&clientCount); err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE practice_id = $1`, practiceID).Scan(&engagementCount); err != nil {
		t.Fatalf("count engagements: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count credit_ledger: %v", err)
	}
	if clientCount != 0 || engagementCount != 0 || ledgerCount != 1 {
		t.Fatalf("rows after rollback: clients=%d engagements=%d credit_ledger=%d, want 0, 0, 1 (only the seeded negative row)", clientCount, engagementCount, ledgerCount)
	}
}

// TestCreateHandler_SequenceExhaustsFreeCreditsThenFails proves a fresh
// Practice's 3 signup-bonus credits allow exactly 3 Engagement creations,
// and that a 4th fails with 402 and writes no new rows.
func TestCreateHandler_SequenceExhaustsFreeCreditsThenFails(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-sequence"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	seedSignupBonus(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	for i := range 3 {
		body, err := json.Marshal(engagement.CreateClientRequest{Name: "Client", Email: fmt.Sprintf("client%d@example.com", i)})
		if err != nil {
			t.Fatalf("marshal body %d: %v", i, err)
		}
		resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %d status = %d, want %d", i+1, resp.StatusCode, http.StatusCreated)
		}
	}

	var engagementCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE practice_id = $1`, practiceID).Scan(&engagementCount); err != nil {
		t.Fatalf("count engagements: %v", err)
	}
	if engagementCount != 3 {
		t.Fatalf("engagement count after 3 creates = %d, want 3", engagementCount)
	}

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "Fourth Client", Email: "fourth@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("4th create status = %d, want %d", resp.StatusCode, http.StatusPaymentRequired)
	}

	var clientCount, ledgerCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE practice_id = $1`, practiceID).Scan(&engagementCount); err != nil {
		t.Fatalf("count engagements after 4th: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&clientCount); err != nil {
		t.Fatalf("count clients after 4th: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM credit_ledger WHERE practice_id = $1`, practiceID).Scan(&ledgerCount); err != nil {
		t.Fatalf("count credit_ledger after 4th: %v", err)
	}
	if engagementCount != 3 || clientCount != 3 || ledgerCount != 4 {
		t.Fatalf("row counts after failed 4th create: engagements=%d clients=%d credit_ledger=%d, want 3, 3, 4 (bonus + 3 consumptions, no new rows)", engagementCount, clientCount, ledgerCount)
	}
}

func TestCreateHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-missing-fields"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	body, err := json.Marshal(engagement.CreateClientRequest{Name: "", Email: "new@example.com"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestCreateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-invalid-body"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedPost(t, session, srv.URL+"/practices/"+practiceID+"/clients", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestDetailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-viewing"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Detail Client", "detail@example.com", "active")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if d.EngagementID != engagementID || d.ClientName != "Detail Client" || d.Status != "active" {
		t.Fatalf("unexpected detail: %+v", d)
	}
}

func TestDetailHandler_NotFoundAtWrongPractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-wrong-practice"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	otherPracticeID := seedStaffWithMembership(t, db, "staff-owns-engagement")
	_, engagementID := seedClientEngagement(t, db, otherPracticeID, "Elsewhere Client", "elsewhere@example.com", "intake")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_ContractorWithoutAttachmentForbidden proves the
// per-Engagement half of ADR-0008's attachment rule: same "not found"
// response an out-of-practice Engagement gets, so a contractor can't
// distinguish "doesn't exist" from "not attached" (#230).
func TestDetailHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-unattached-detail"
	practiceID := seedStaffWithMembership(t, db, "staff-owner-of-practice-2")
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Unattached Detail Client", "unattached-detail@example.com", "intake")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestDetailHandler_ContractorWithGrantedAttachmentSucceeds proves the
// other half: an open, granted attachment reaches.
func TestDetailHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-attached-detail"
	practiceID := seedStaffWithMembership(t, db, "staff-owner-of-practice-3")
	staffID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Attached Detail Client", "attached-detail@example.com", "active")
	seedGrantedAttachment(t, db, engagementID, staffID)

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestDetailHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-bad-id"
	practiceID := seedStaffWithMembership(t, db, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/engagements/not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
