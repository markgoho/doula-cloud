package feepayment_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/feepayment"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

const (
	feeCents     = int64(45_000)
	paidOn       = "2026-10-01"
	contractor   = "contractor"
	employee     = "employee"
	doulaRole    = "doula"
	ownerRole    = "owner"
	adminRole    = "admin"
	actionPaid   = "contractor_fee_paid"
	actionUndone = "contractor_fee_payment_reversed"

	methodBankTransfer = "bank_transfer"
	methodCash         = "cash"
	venmo              = "Venmo"
	keyReason          = "reason"
	keyMethod          = "method"
	keyPaidOn          = "paidOn"
)

type fixture struct {
	db           *testdb.DB
	srv          *httptest.Server
	practiceID   string
	engagementID string
	lenaID       string
	attachmentID string
	owner        string
	lena         string
}

// newFixture seeds a Practice with an Owner, a contractor Lena who
// accepted a $450 Offer on one Engagement, and a server mounting this
// package beside offer and engagement.
func newFixture(t *testing.T) fixture {
	t.Helper()
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "fee-owner", []string{ownerRole}, employee)
	lenaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "fee-lena", "Lena Vasquez", []string{doulaRole}, contractor)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	f := fixture{db: db, practiceID: practiceID, engagementID: engagementID, lenaID: lenaID}
	f.attachmentID = seedAcceptedOffer(t, db, engagementID, lenaID, feeCents)

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	feepayment.Mount(g, ir)
	offer.Mount(g, ir, db.App, tasknudge.NoOpEnqueuer{})
	engagement.Mount(g, ir)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	f.owner = authntest.SeedSession(t, db.App, "fee-owner")
	f.lena = authntest.SeedSession(t, db.App, "fee-lena")
	return f
}

// seedAcceptedOffer writes what offer.AcceptHandler leaves behind: an
// accepted Offer and the granted Attachment carrying its fee, in one
// statement so both read the same now(), as the accept's one transaction
// does.
func seedAcceptedOffer(t *testing.T, db *testdb.DB, engagementID, staffID string, cents int64) (attachmentID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`WITH accepted AS (
		     INSERT INTO engagement_offers
		         (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		          due_date, state, offered_by, expires_at, decided_at, decided_by)
		     VALUES ($1, $2, 'contractor', $3, 'A', 'North side', '2027-02-14', 'accepted', $2,
		             now() + interval '7 days', now(), $2))
		 INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents)
		 VALUES ($1, $2, 'granted', $2, $3) RETURNING id`,
		engagementID, staffID, cents,
	).Scan(&attachmentID); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	return attachmentID
}

type response struct {
	status int
	body   []byte
}

func do(t *testing.T, method, url, session string, body any) response {
	t.Helper()
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		payload = encoded
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("X-Confirmed", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	read, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response{status: resp.StatusCode, body: read}
}

func decode(t *testing.T, resp response, want int, out any) {
	t.Helper()
	if resp.status != want {
		t.Fatalf("status = %d, want %d: %s", resp.status, want, resp.body)
	}
	if out == nil {
		return
	}
	if err := json.Unmarshal(resp.body, out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func (f fixture) feesURL() string {
	return f.srv.URL + "/api/practices/" + f.practiceID + "/engagements/" + f.engagementID + "/doula-fees"
}

func (f fixture) paymentsURL(attachmentID string) string {
	return f.feesURL() + "/" + attachmentID + "/payments"
}

func (f fixture) list(t *testing.T) []feepayment.DoulaFee {
	t.Helper()
	var out feepayment.ListResponse
	decode(t, do(t, http.MethodGet, f.feesURL(), f.owner, nil), http.StatusOK, &out)
	return out.Items
}

func (f fixture) recordPaid(t *testing.T) feepayment.View {
	t.Helper()
	var view feepayment.View
	decode(t, do(t, http.MethodPost, f.paymentsURL(f.attachmentID), f.owner,
		map[string]string{keyMethod: methodBankTransfer, keyPaidOn: paidOn}), http.StatusCreated, &view)
	return view
}

func (f fixture) reverse(t *testing.T, paymentID string) response {
	t.Helper()
	return do(t, http.MethodPost, f.paymentsURL(f.attachmentID)+"/"+paymentID+"/reverse", f.owner,
		map[string]string{keyReason: "Recorded against the wrong birth"})
}

func (f fixture) inbox(t *testing.T, session string) offer.ListResponse {
	t.Helper()
	var out offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv.URL+"/api/practices/"+f.practiceID+"/offers", session, nil), http.StatusOK, &out)
	return out
}

func activityCount(t *testing.T, f fixture, action string) int {
	t.Helper()
	var n int
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		  WHERE subject_id = $1 AND action = $2 AND actor_kind = 'staff'
		    AND diff ->> 'attachedStaffId' = $3 AND NOT (diff ? 'amountCents')`,
		f.engagementID, action, f.lenaID,
	).Scan(&n); err != nil {
		t.Fatalf("count activity: %v", err)
	}
	return n
}

func TestRecord_TheWholeFeeIsRecordedAndListed(t *testing.T) {
	f := newFixture(t)

	fees := f.list(t)
	if len(fees) != 1 || fees[0].Paid != nil || fees[0].AmountCents != feeCents || fees[0].DoulaName != "Lena Vasquez" {
		t.Fatalf("before = %+v, want Lena's unpaid $450 fee", fees)
	}

	view := f.recordPaid(t)
	if view.AmountCents != feeCents || view.Method != methodBankTransfer || view.PaidOn != paidOn || view.Kind != "recorded" {
		t.Fatalf("recorded = %+v", view)
	}

	fees = f.list(t)
	paid := fees[0].Paid
	if paid == nil || paid.ID != view.ID || paid.AmountCents != feeCents || paid.PaidOn != paidOn || paid.RecordedBy == "" {
		t.Fatalf("after = %+v, want the recorded Fee Payment", fees[0])
	}
	if got := activityCount(t, f, actionPaid); got != 1 {
		t.Fatalf("contractor_fee_paid rows naming Lena with no amount = %d, want 1", got)
	}
}

func TestRecord_KeepsTheNote(t *testing.T) {
	f := newFixture(t)
	var view feepayment.View
	decode(t, do(t, http.MethodPost, f.paymentsURL(f.attachmentID), f.owner,
		map[string]string{keyMethod: "other", "note": venmo, keyPaidOn: paidOn}), http.StatusCreated, &view)
	if view.Note == nil || *view.Note != venmo {
		t.Fatalf("note = %v, want Venmo", view.Note)
	}
	if note := f.list(t)[0].Paid.Note; note == nil || *note != venmo {
		t.Fatalf("listed note = %v, want Venmo", note)
	}
}

func TestRecord_RefusesASecondRecord(t *testing.T) {
	f := newFixture(t)
	f.recordPaid(t)
	resp := do(t, http.MethodPost, f.paymentsURL(f.attachmentID), f.owner,
		map[string]string{keyMethod: methodCash, keyPaidOn: paidOn})
	if resp.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", resp.status, resp.body)
	}
}

func TestRecord_AnEndedAttachmentCanStillBePaid(t *testing.T) {
	f := newFixture(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE id = $1`, f.attachmentID,
	); err != nil {
		t.Fatalf("end attachment: %v", err)
	}
	if fees := f.list(t); len(fees) != 1 || fees[0].EndedOn == nil {
		t.Fatalf("fees = %+v, want the ended Attachment listed with its end", fees)
	}
	f.recordPaid(t)
}

func TestRecord_RefusesAnAttachmentWithoutAFeeOrOnAnotherEngagement(t *testing.T) {
	f := newFixture(t)
	priyaID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "fee-priya", []string{doulaRole}, employee)
	var noFee string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by)
		 VALUES ($1, $2, 'granted', $2) RETURNING id`, f.engagementID, priyaID,
	).Scan(&noFee); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	_, otherEngagement := testdb.SeedNamedEngagement(t, f.db, f.practiceID, "Other", "other@example.com")
	elsewhere := seedAcceptedOffer(t, f.db, otherEngagement, f.lenaID, feeCents)

	for name, id := range map[string]string{"no fee": noFee, "another Engagement": elsewhere} {
		resp := do(t, http.MethodPost, f.paymentsURL(id), f.owner, map[string]string{keyMethod: methodCash, keyPaidOn: paidOn})
		if resp.status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404: %s", name, resp.status, resp.body)
		}
	}
	if fees := f.list(t); len(fees) != 1 {
		t.Fatalf("fees = %+v, want only Lena's on this Engagement", fees)
	}
}

func TestRecord_RefusesWhatTheFormRefuses(t *testing.T) {
	f := newFixture(t)
	tomorrow := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)
	cases := map[string]struct {
		body  map[string]string
		field string
		want  string
	}{
		"no method":       {map[string]string{keyPaidOn: paidOn}, "method", "Select how the payment was made"},
		"other, no note":  {map[string]string{keyMethod: "other", keyPaidOn: paidOn}, "note", `Enter a note for "Other"`},
		"not a date":      {map[string]string{keyMethod: methodCash, keyPaidOn: "soon"}, keyPaidOn, "Enter the date paid as a real date, like 2027-04-23"},
		"in the future":   {map[string]string{keyMethod: methodCash, keyPaidOn: tomorrow}, keyPaidOn, "The date cannot be in the future"},
		"malformed paths": {nil, "", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.body == nil {
				resp := do(t, http.MethodPost, f.feesURL()+"/not-a-uuid/payments", f.owner, map[string]string{})
				if resp.status != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400", resp.status)
				}
				return
			}
			resp := do(t, http.MethodPost, f.paymentsURL(f.attachmentID), f.owner, tc.body)
			var refusal apierr.APIError
			decode(t, resp, http.StatusBadRequest, &refusal)
			if refusal.Details[tc.field] != tc.want {
				t.Fatalf("details[%s] = %q, want %q", tc.field, refusal.Details[tc.field], tc.want)
			}
		})
	}
}

func TestReverse_UndoesAdditivelyAndAllowsANewRecord(t *testing.T) {
	f := newFixture(t)
	view := f.recordPaid(t)

	var reversal feepayment.View
	decode(t, f.reverse(t, view.ID), http.StatusCreated, &reversal)
	if reversal.Kind != "reversal" || reversal.AmountCents != -feeCents || reversal.TargetID == nil || *reversal.TargetID != view.ID {
		t.Fatalf("reversal = %+v", reversal)
	}
	if fees := f.list(t); fees[0].Paid != nil {
		t.Fatalf("after reversal = %+v, want unpaid", fees[0])
	}
	var rows int
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM attachment_fee_payments WHERE attachment_id = $1`, f.attachmentID,
	).Scan(&rows); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want the record and its reversal both kept", rows)
	}
	if got := activityCount(t, f, actionUndone); got != 1 {
		t.Fatalf("contractor_fee_payment_reversed rows = %d, want 1", got)
	}

	if resp := f.reverse(t, view.ID); resp.status != http.StatusConflict {
		t.Fatalf("second reversal status = %d, want 409", resp.status)
	}
	if resp := f.reverse(t, reversal.ID); resp.status != http.StatusNotFound {
		t.Fatalf("reversing a reversal status = %d, want 404", resp.status)
	}
	f.recordPaid(t)
}

func TestReverse_Refusals(t *testing.T) {
	f := newFixture(t)
	view := f.recordPaid(t)
	base := f.paymentsURL(f.attachmentID)

	resp := do(t, http.MethodPost, base+"/"+view.ID+"/reverse", f.owner, map[string]string{keyReason: ""})
	var refusal apierr.APIError
	decode(t, resp, http.StatusBadRequest, &refusal)
	if refusal.Details[keyReason] == "" {
		t.Fatalf("details = %v, want a reason refusal", refusal.Details)
	}
	if resp := do(t, http.MethodPost, base+"/not-a-uuid/reverse", f.owner, map[string]string{keyReason: "x"}); resp.status != http.StatusBadRequest {
		t.Fatalf("malformed payment id status = %d, want 400", resp.status)
	}
	if resp := do(t, http.MethodPost, f.feesURL()+"/not-a-uuid/payments/"+view.ID+"/reverse", f.owner, map[string]string{keyReason: "x"}); resp.status != http.StatusBadRequest {
		t.Fatalf("malformed attachment id status = %d, want 400", resp.status)
	}
	if resp := do(t, http.MethodPost, base+"/"+view.ID+"/reverse", f.owner, "not an object"); resp.status != http.StatusBadRequest {
		t.Fatalf("bad body status = %d, want 400", resp.status)
	}
	badEngagement := f.srv.URL + "/api/practices/" + f.practiceID + "/engagements/not-a-uuid/doula-fees/" + f.attachmentID + "/payments/" + view.ID + "/reverse"
	if resp := do(t, http.MethodPost, badEngagement, f.owner, map[string]string{keyReason: "x"}); resp.status != http.StatusBadRequest {
		t.Fatalf("malformed engagement id status = %d, want 400", resp.status)
	}
	missing := f.feesURL() + "/" + f.engagementID + "/payments/" + view.ID + "/reverse"
	if resp := do(t, http.MethodPost, missing, f.owner, map[string]string{keyReason: "x"}); resp.status != http.StatusNotFound {
		t.Fatalf("unknown attachment status = %d, want 404", resp.status)
	}
}

func TestList_RefusesAMalformedEngagement(t *testing.T) {
	f := newFixture(t)
	url := f.srv.URL + "/api/practices/" + f.practiceID + "/engagements/not-a-uuid/doula-fees"
	if resp := do(t, http.MethodGet, url, f.owner, nil); resp.status != http.StatusBadRequest && resp.status != http.StatusNotFound {
		t.Fatalf("status = %d, want a refusal", resp.status)
	}
}

func TestOnlyOwnerAndAdminReadOrWrite(t *testing.T) {
	f := newFixture(t)
	view := f.recordPaid(t)
	testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "fee-admin", []string{adminRole}, employee)
	testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "fee-employee", []string{doulaRole}, employee)
	admin := authntest.SeedSession(t, f.db.App, "fee-admin")
	employeeDoula := authntest.SeedSession(t, f.db.App, "fee-employee")

	var fees feepayment.ListResponse
	decode(t, do(t, http.MethodGet, f.feesURL(), admin, nil), http.StatusOK, &fees)
	if len(fees.Items) != 1 || fees.Items[0].Paid == nil {
		t.Fatalf("admin read = %+v", fees)
	}

	for name, session := range map[string]string{"employee Doula": employeeDoula, "the contractor herself": f.lena} {
		if resp := do(t, http.MethodGet, f.feesURL(), session, nil); resp.status != http.StatusForbidden {
			t.Errorf("%s list: status = %d, want 403", name, resp.status)
		}
		if resp := do(t, http.MethodPost, f.paymentsURL(f.attachmentID), session,
			map[string]string{keyMethod: methodCash, keyPaidOn: paidOn}); resp.status != http.StatusForbidden && resp.status != http.StatusNotFound {
			t.Errorf("%s record: status = %d, want a refusal", name, resp.status)
		}
		if resp := do(t, http.MethodPost, f.paymentsURL(f.attachmentID)+"/"+view.ID+"/reverse", session,
			map[string]string{keyReason: "x"}); resp.status != http.StatusForbidden && resp.status != http.StatusNotFound {
			t.Errorf("%s reverse: status = %d, want a refusal", name, resp.status)
		}
	}
}

func TestAClientPortalSessionIsRefused(t *testing.T) {
	f := newFixture(t)
	f.recordPaid(t)
	uid := portalaccount.NewIdentifier()
	clientID := testdb.SeedNamedClient(t, f.db, f.practiceID, "Amara", "amara@example.com")
	testdb.SeedPortalUser(t, f.db, uid, clientID)
	portal := authntest.SeedSession(t, f.db.App, uid)

	resp := do(t, http.MethodGet, f.feesURL(), portal, nil)
	if resp.status < 400 || bytes.Contains(resp.body, []byte("bank_transfer")) {
		t.Fatalf("portal session read: status = %d, body = %s", resp.status, resp.body)
	}
}

func TestInbox_SheReadsHerOwnFeePaidAndNoOneElseDoes(t *testing.T) {
	f := newFixture(t)
	if offers := f.inbox(t, f.lena); len(offers.Items) != 1 || offers.Items[0].FeePaid != nil {
		t.Fatalf("before = %+v, want her accepted Offer with nothing paid", offers.Items)
	}

	view := f.recordPaid(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE id = $1`, f.attachmentID,
	); err != nil {
		t.Fatalf("end attachment: %v", err)
	}
	got := f.inbox(t, f.lena).Items[0].FeePaid
	if got == nil || got.AmountCents != feeCents || got.Method != methodBankTransfer || got.PaidOn != paidOn {
		t.Fatalf("after her Attachment ended, feePaid = %+v, want $450 by bank transfer on %s", got, paidOn)
	}

	testdb.SeedNamedStaffAtPractice(t, f.db, f.practiceID, "fee-other", "Other Doula", []string{doulaRole}, contractor)
	other := authntest.SeedSession(t, f.db.App, "fee-other")
	if offers := f.inbox(t, other); len(offers.Items) != 0 {
		t.Fatalf("another contractor's inbox = %+v, want none of Lena's", offers.Items)
	}
	var practiceSide offer.ListResponse
	decode(t, do(t, http.MethodGet, f.srv.URL+"/api/practices/"+f.practiceID+"/engagements/"+f.engagementID+"/offers", f.owner, nil),
		http.StatusOK, &practiceSide)
	if len(practiceSide.Items) != 1 || practiceSide.Items[0].FeePaid != nil {
		t.Fatalf("Practice-side offers = %+v, want no feePaid on it", practiceSide.Items)
	}

	decode(t, f.reverse(t, view.ID), http.StatusCreated, nil)
	if got := f.inbox(t, f.lena).Items[0].FeePaid; got != nil {
		t.Fatalf("after reversal, feePaid = %+v, want none", got)
	}
}

func TestInbox_EachAcceptedOfferReadsItsOwnAttachment(t *testing.T) {
	f := newFixture(t)
	f.recordPaid(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE id = $1`, f.attachmentID,
	); err != nil {
		t.Fatalf("end attachment: %v", err)
	}
	seedAcceptedOffer(t, f.db, f.engagementID, f.lenaID, 60_000)

	paid := map[int64]bool{}
	for _, item := range f.inbox(t, f.lena).Items {
		paid[*item.AmountCents] = item.FeePaid != nil
	}
	if !paid[feeCents] || paid[60_000] {
		t.Fatalf("paid by fee = %v, want only the first job's $450 paid", paid)
	}
}
