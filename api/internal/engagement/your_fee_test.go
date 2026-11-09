package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2003: a contractor Doula reads her own
// agreed fee on the Engagement, and nobody else's (ADR-0008's money row,
// as amended on #282: "her own agreed fee only, on her Engagements --
// never the Practice's price").

// seedFeeAttachment opens a granted Attachment for staffID on
// engagementID carrying feeCents, as an accepted Offer writes one,
// attached on attachedAt (an instant the test names in UTC).
func seedFeeAttachment(t *testing.T, db *testdb.DB, engagementID, staffID, attachedBy string, feeCents int64, attachedAt string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, attached_at, fee_amount_cents, fee_terms)
		 VALUES ($1, $2, 'granted', $3, $4::timestamptz, $5, 'Births in the Rochester area')`,
		engagementID, staffID, attachedBy, attachedAt, feeCents); err != nil {
		t.Fatalf("seed fee attachment: %v", err)
	}
}

// TestDetailHandler_ContractorReadsHerOwnFee: the contractor on the
// Engagement reads her fee and the day she accepted it, in the
// Practice's own calendar (ADR-0036): 03:30 UTC on Jul 23 is still
// Jul 22 in America/New_York.
func TestDetailHandler_ContractorReadsHerOwnFee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), withEngagement(fixture.Status(fixture.StatusActive)))
	owner := e.Colleague(fixture.Owner())
	seedFeeAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID, e.Staff.ID, 62000, "2026-07-23T03:30:00Z")

	d := e.detail()

	if d.YourFee == nil {
		t.Fatal("yourFee is absent, want the contractor's own agreed fee")
	}
	if d.YourFee.AmountCents != 62000 {
		t.Fatalf("yourFee.amountCents = %d, want 62000", d.YourFee.AmountCents)
	}
	if d.YourFee.AcceptedOn != "2026-07-22" {
		t.Fatalf("yourFee.acceptedOn = %q, want 2026-07-22 (the Practice's day)", d.YourFee.AcceptedOn)
	}
	// #2269: the terms she accepted ride beside the fee.
	if d.YourFee.Terms == nil || *d.YourFee.Terms != "Births in the Rochester area" {
		t.Fatalf("yourFee.terms = %v, want the terms copied from the Offer", d.YourFee.Terms)
	}

	// The Owner reads the same Engagement and gets no fee of her own:
	// yourFee is the reader's own, never the contractor's fee read by
	// somebody else.
	if got := e.detailAs(owner).YourFee; got != nil {
		t.Fatalf("the Owner's yourFee = %+v, want absent: it is the reader's own fee", got)
	}
}

// TestDetailHandler_NoFeeWithoutAnAcceptedOffer: an employee Doula put
// on the Engagement with no Offer holds an Attachment with no fee, so
// the field is absent rather than $0.00.
func TestDetailHandler_NoFeeWithoutAnAcceptedOffer(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID)

	if d := e.detail(); d.YourFee != nil {
		t.Fatalf("yourFee = %+v, want absent for an Attachment with no fee", d.YourFee)
	}
}

// TestDetailHandler_AnEndedAttachmentCarriesNoFee: a fee is read off the
// open Attachment only. One she dropped out of is a record of work, not
// a fee she is still owed on this Engagement.
func TestDetailHandler_AnEndedAttachmentCarriesNoFee(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner(), fixture.Doula(), fixture.Contractor()), withEngagement(fixture.Status(fixture.StatusActive)))
	seedFeeAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID, e.Staff.ID, 50000, "2026-07-01T12:00:00Z")
	e.Exec(`UPDATE engagement_attachments SET ended_at = now(), ended_by = staff_id WHERE engagement_id = $1`, e.Engagement.ID)

	if d := e.detail(); d.YourFee != nil {
		t.Fatalf("yourFee = %+v, want absent once the Attachment ended", d.YourFee)
	}
}
