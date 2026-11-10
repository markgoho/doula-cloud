// Package clientstanding names a Client record's standing -- live,
// merged, erased, or merged and then erased -- and is the one gate every
// write on or under a Client record consults.
//
// The standing is derived once, by the clients.standing generated column
// (00171_client_standing.sql). Nothing here derives it: Go names the four
// values the column can hold and reads the one a record is in. A reader
// that asks a narrower question than "is this record live?" -- a list
// that shows an erased record as a redacted row, say -- names the
// standings it admits in its own SQL (`c.standing IN ('live', 'erased')`)
// rather than restating merged_into or erased_at, so the difference
// between two readers is a visible choice. standing_test.go's guardrail
// fails the build on a reader that goes back to the columns.
//
// Writes are declared as Acts. Every Act requires a live record; that is
// the whole rule, and the refusal names the standing the record is
// actually in. What is deliberately not an Act is an ending: completing
// an Engagement, voiding a Contract, ending an Attachment, recording or
// reversing a payment. Erasure ends no Engagement (ADR-0027: the record
// of care stays whole), so the Practice still has to be able to close
// the work it held for her, and an ending leaves the record more
// complete rather than adding to what is held about her.
package clientstanding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// Standing is the clients.standing enum (00171).
type Standing string

// The four standings, matching client_standing's labels.
const (
	// Live is neither merged nor erased, the only standing a write is
	// admitted in.
	Live Standing = "live"
	// Merged is a tombstone (ADR-0040): merged_into names the record
	// that kept everything that follows her.
	Merged Standing = "merged"
	// Erased is redacted in place on her own request (ADR-0027).
	Erased Standing = "erased"
	// MergedThenErased is a tombstone reached by its survivor's erasure.
	// There is no erased-then-merged: an erased record is never written
	// again (clients_update, 00171).
	MergedThenErased Standing = "merged_then_erased"
)

// Standings is every value client_standing holds, in its declared order.
// standing_test.go holds it to the database's own enum_range.
var Standings = []Standing{Live, Merged, Erased, MergedThenErased}

// IsTombstone reports whether the record was merged into another, erased
// since or not. A tombstone has no screen of its own and is never listed.
func (s Standing) IsTombstone() bool { return s == Merged || s == MergedThenErased }

// IsErased reports whether the record's identifying data has been
// destroyed, as a record of its own or as the tombstone of one.
func (s Standing) IsErased() bool { return s == Erased || s == MergedThenErased }

// Act is one write on or under a Client record: the record itself, or
// something new added beneath it. It requires a live record.
type Act struct {
	// Does names the write as the reader would do it, for the refusal
	// sentence Check writes ("add a Visit"). The refusal reaches a screen
	// word for word, so it is written for the person reading it
	// (docs/api-design.md section 7 rule 4).
	Does string
}

// The declared Acts, one per write route that consults the gate.
var (
	// ActEdit is client.EditHandler: replacing the record's details.
	ActEdit = Act{Does: "change the details on this record"}
	// ActErase is client.EraseHandler. A tombstone is erased only by its
	// survivor's erasure, never on its own.
	ActErase = Act{Does: "erase this record"}
	// ActMerge is client.MergeHandler, for the record open for editing.
	ActMerge = Act{Does: "merge this record"}
	// ActMergeInto is client.MergeHandler, for the record named as the
	// other half of the merge (ADR-0017's amendment: never a tombstone,
	// never erased).
	ActMergeInto = Act{Does: "merge another record with this one"}
	// ActRequestEngagement is engagementrequest.RequestHandler.
	ActRequestEngagement = Act{Does: "ask for a new Engagement"}
	// ActApproveRequest is engagementrequest's approve, shared by
	// ApproveHandler and the solo-Practice collapse. Days can pass
	// between the ask and the decision, and the record can be erased in
	// them: erasure does not touch engagement_requests.
	ActApproveRequest = Act{Does: "approve this request"}
	// ActAddVisit is visit.CreateHandler.
	ActAddVisit = Act{Does: "add a Visit"}
	// ActSendMessage is message's postMessage, for Staff and Client alike.
	ActSendMessage = Act{Does: "send a Message"}
	// ActCreateContract is contracts.PostContractHandler.
	ActCreateContract = Act{Does: "create a Contract"}
	// ActSendContract is contracts.PostSendContractHandler.
	ActSendContract = Act{Does: "send the Contract"}
	// ActStartPlan is plans.PostInstanceHandler.
	ActStartPlan = Act{Does: "start a plan"}
	// ActInviteToPortal is portalinvite.InviteHandler.
	ActInviteToPortal = Act{Does: "invite this Client to the portal"}
	// ActOfferWork is offer.CreateHandler: an Offer copies the Client's
	// initial, area and due date to a contractor (ADR-0027's 2026-10-31
	// amendment).
	ActOfferWork = Act{Does: "offer this work to a Doula"}
	// ActCreateInvoice is payments.PostInvoiceHandler: an Invoice allocates
	// a Stripe Customer, which erasure deleted (ADR-0027).
	ActCreateInvoice = Act{Does: "create an Invoice"}
)

// Acts is every declared Act, walked by standing_test.go.
var Acts = []Act{
	ActEdit, ActErase, ActMerge, ActMergeInto,
	ActRequestEngagement, ActApproveRequest,
	ActAddVisit, ActSendMessage,
	ActCreateContract, ActSendContract,
	ActStartPlan, ActInviteToPortal, ActOfferWork, ActCreateInvoice,
}

// Check reports whether a record in standing s may take a, or, if not,
// the 409 message to write: one sentence naming the standing the record
// is in ("This Client's details have been erased, so you cannot add a
// Visit.").
func (a Act) Check(s Standing) (ok bool, refusal string) {
	if s == Live {
		return true, ""
	}
	return false, fmt.Sprintf("%s, so you cannot %s.", standingClause(s), a.Does)
}

// standingClause says what standing a record is in, as the opening
// clause of a refusal. A tombstone is named as merged whether or not it
// was erased since: what the reader can do is go to the record it was
// merged into.
func standingClause(s Standing) string {
	switch {
	case s.IsTombstone():
		return "This Client record has been merged into another"
	case s == Erased:
		return "This Client's details have been erased"
	}
	return fmt.Sprintf("This Client record is %s", s)
}

// ErrNotFound reports that no record is visible to the caller.
var ErrNotFound = errors.New("clientstanding: no such client")

// Of reads clientID's standing, under the caller's row security.
func Of(ctx context.Context, tx *sql.Tx, clientID string) (Standing, error) {
	return scan(tx.QueryRowContext(ctx, `SELECT standing FROM clients WHERE id = $1`, clientID))
}

// OfEngagement reads the standing of the record engagementID belongs to.
func OfEngagement(ctx context.Context, tx *sql.Tx, engagementID string) (Standing, error) {
	return scan(tx.QueryRowContext(ctx,
		`SELECT c.standing FROM engagements e JOIN clients c ON c.id = e.client_id WHERE e.id = $1`,
		engagementID))
}

func scan(row *sql.Row) (Standing, error) {
	var s Standing
	err := row.Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("clientstanding: read standing: %w", err)
	}
	return s, nil
}

// Require writes a's refusal for clientID's record and reports false when
// the record is not live. A record the caller cannot see is not this
// gate's to refuse -- each route already answers its own 404 -- so it
// passes here and meets that answer.
func (a Act) Require(w http.ResponseWriter, r *http.Request, tx *sql.Tx, clientID string) bool {
	s, err := Of(r.Context(), tx, clientID)
	return a.require(w, s, err)
}

// RequireForEngagement is Require for the record engagementID belongs to.
func (a Act) RequireForEngagement(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID string) bool {
	s, err := OfEngagement(r.Context(), tx, engagementID)
	return a.require(w, s, err)
}

func (a Act) require(w http.ResponseWriter, s Standing, err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if ok, refusal := a.Check(s); !ok {
		apierr.WriteError(w, refusal, http.StatusConflict)
		return false
	}
	return true
}

// RefusalError is the error RequireErr returns for a record that is not live,
// for a write path that reports through an error rather than a response.
type RefusalError struct{ Message string }

func (r RefusalError) Error() string { return r.Message }

// RequireErr is Require for a caller below the HTTP layer: nil when the
// record is live or not visible, a RefusalError when it is in another
// standing.
func (a Act) RequireErr(ctx context.Context, tx *sql.Tx, clientID string) error {
	s, err := Of(ctx, tx, clientID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if ok, refusal := a.Check(s); !ok {
		return RefusalError{Message: refusal}
	}
	return nil
}
