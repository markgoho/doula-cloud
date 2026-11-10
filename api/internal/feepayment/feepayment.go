// Package feepayment records that a Practice paid a contractor Doula the
// fee she accepted (#2271): a Fee Payment in GLOSSARY.md. A record only,
// never a payment. The money moves outside DoulaCloud and nothing here
// calls Stripe (#391, ADR-0007's guardrail).
//
// It mirrors #271's by-hand Payment: the whole accepted fee, one of the
// same four methods, a date not in the future, an optional note, and a
// mistake undone by an additive reversal row (#945), never an edit.
//
// Owner and Admin record and read every Fee Payment on an Engagement
// here. The contractor reads her own on Your offers (offer.InboxHandler),
// which she still reaches after her Attachment ends.
package feepayment

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/paymentstanding"
	"doula-cloud/api/internal/staffauth"
)

const (
	kindRecorded = "recorded"
	kindReversal = "reversal"
)

// MsgFeeNotFound is the refusal for an Attachment that is not this
// Engagement's, or carries no accepted fee to pay.
const MsgFeeNotFound = "This fee is not on this work."

// MsgFeeAlreadyPaid refuses a second record against a fee already
// recorded as paid: the record is the whole accepted fee, so a second one
// would count it twice.
const MsgFeeAlreadyPaid = "This fee is already recorded as paid."

// MsgFeePaymentNotReversible refuses a reversal target that is not a
// recorded Fee Payment on this Attachment.
const MsgFeePaymentNotReversible = "This payment cannot be removed."

// MsgFeePaymentAlreadyReversed refuses removing a Fee Payment twice.
const MsgFeePaymentAlreadyReversed = "This payment has already been removed."

// DoulaFee is one contractor's accepted fee on an Engagement, open or
// ended, and whether the Practice has recorded paying it.
type DoulaFee struct {
	AttachmentID string `json:"attachmentId"`
	DoulaName    string `json:"doulaName"`
	AmountCents  int64  `json:"amountCents"`
	// AcceptedOn and EndedOn are the Practice's calendar days (ADR-0036).
	AcceptedOn string  `json:"acceptedOn"`
	EndedOn    *string `json:"endedOn,omitempty"`
	Paid       *Paid   `json:"paid,omitempty"`
}

// Paid is the Fee Payment that stands: recorded and not reversed.
type Paid struct {
	ID          string    `json:"id"`
	AmountCents int64     `json:"amountCents"`
	Method      string    `json:"method"`
	Note        *string   `json:"note,omitempty"`
	PaidOn      string    `json:"paidOn"`
	RecordedBy  string    `json:"recordedBy"`
	RecordedAt  time.Time `json:"recordedAt"`
}

// ListResponse carries every contractor fee on one Engagement. Not
// paginated: it holds one row per Doula who accepted an Offer on it.
type ListResponse struct {
	Items []DoulaFee `json:"items"`
}

// ReverseRequest is the body of a POST to ReverseHandler. A reason is
// always required, as #945's reversal requires one.
type ReverseRequest struct {
	Reason string `json:"reason"`
}

// View is one attachment_fee_payments row, as the two writes
// return it.
type View struct {
	ID           string    `json:"id"`
	AttachmentID string    `json:"attachmentId"`
	Kind         string    `json:"kind"`
	AmountCents  int64     `json:"amountCents"`
	Method       string    `json:"method,omitempty"`
	Note         *string   `json:"note,omitempty"`
	PaidOn       string    `json:"paidOn,omitempty"`
	TargetID     *string   `json:"targetId,omitempty"`
	Reason       *string   `json:"reason,omitempty"`
	RecordedAt   time.Time `json:"recordedAt"`
}

// ListHandler serves each contractor fee on the Engagement, with its
// standing Fee Payment. An ended Attachment is listed too: a Practice
// usually pays after the birth, by when the Attachment has ended.
func ListHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		items, err := listFees(r.Context(), c.Tx, c.PracticeID, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, ListResponse{Items: items})
	}, staffauth.EngagementID())
}

// listFeesQuery reads each contractor fee on an Engagement with the Fee
// Payment standing on it (paymentstanding.FeeStandsExpr).
const listFeesQuery = `SELECT ea.id, s.name, ea.fee_amount_cents,
	        (ea.attached_at AT TIME ZONE p.timezone)::date::text,
	        (ea.ended_at AT TIME ZONE p.timezone)::date::text,
	        fp.id, fp.amount_cents, fp.method::text, fp.note, fp.paid_on::text, rs.name, fp.recorded_at
	   FROM engagement_attachments ea
	   JOIN engagements e ON e.id = ea.engagement_id
	   JOIN practices p ON p.id = e.practice_id
	   JOIN staff s ON s.id = ea.staff_id
	   LEFT JOIN LATERAL (
	        SELECT f.id, f.amount_cents, f.method, f.note, f.paid_on, f.recorded_by, f.recorded_at
	          FROM attachment_fee_payments f
	         WHERE f.attachment_id = ea.id AND ` + paymentstanding.FeeStandsExpr + `
	         ORDER BY f.recorded_at DESC
	         LIMIT 1
	   ) fp ON true
	   LEFT JOIN staff rs ON rs.id = fp.recorded_by
	  WHERE ea.engagement_id = $1 AND e.practice_id = $2
	    AND ea.origin = 'granted' AND ea.fee_amount_cents IS NOT NULL
	  ORDER BY ea.attached_at, ea.id`

func listFees(ctx context.Context, tx *sql.Tx, practiceID, engagementID string) ([]DoulaFee, error) {
	rows, err := tx.QueryContext(ctx,
		listFeesQuery,
		engagementID, practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("feepayment: list fees: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []DoulaFee{}
	for rows.Next() {
		var fee DoulaFee
		var endedOn, paidID, method, note, paidOn, recordedBy sql.NullString
		var paidAmount sql.NullInt64
		var recordedAt sql.NullTime
		if err := rows.Scan(&fee.AttachmentID, &fee.DoulaName, &fee.AmountCents, &fee.AcceptedOn, &endedOn,
			&paidID, &paidAmount, &method, &note, &paidOn, &recordedBy, &recordedAt); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("feepayment: scan fee: %w", err)
		}
		if endedOn.Valid {
			fee.EndedOn = &endedOn.String
		}
		if paidID.Valid {
			fee.Paid = &Paid{
				ID:          paidID.String,
				AmountCents: paidAmount.Int64,
				Method:      method.String,
				PaidOn:      paidOn.String,
				RecordedBy:  recordedBy.String,
				RecordedAt:  recordedAt.Time,
			}
			if note.Valid {
				fee.Paid.Note = &note.String
			}
		}
		items = append(items, fee)
	}
	// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feepayment: iterate fees: %w", err)
	}
	return items, nil
}

// lockFee locks the Attachment's row, so two records racing on one fee
// serialize, and returns its fee and its Doula. sql.ErrNoRows when the
// Attachment is not this Engagement's at this Practice, or carries no fee.
func lockFee(ctx context.Context, tx *sql.Tx, practiceID, engagementID, attachmentID string) (feeCents int64, doulaStaffID string, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT ea.fee_amount_cents, ea.staff_id
		   FROM engagement_attachments ea
		   JOIN engagements e ON e.id = ea.engagement_id
		  WHERE ea.id = $1 AND ea.engagement_id = $2 AND e.practice_id = $3
		    AND ea.origin = 'granted' AND ea.fee_amount_cents IS NOT NULL
		  FOR UPDATE OF ea`,
		attachmentID, engagementID, practiceID,
	).Scan(&feeCents, &doulaStaffID)
	// coverage:ignore reason: the sql.ErrNoRows branch is exercised by unit tests; a non-ErrNoRows DB failure here is not
	if err != nil {
		return 0, "", fmt.Errorf("feepayment: lock fee: %w", err)
	}
	return feeCents, doulaStaffID, nil
}

// requireFee locks the Attachment's contractor fee for the Engagement, a
// 404 when there is none and a 500 on any other failure, written here.
func requireFee(w http.ResponseWriter, r *http.Request, c staffauth.Call, attachmentID string) (feeCents int64, doulaStaffID string, ok bool) {
	feeCents, doulaStaffID, err := lockFee(r.Context(), c.Tx, c.PracticeID, c.EngagementID, attachmentID)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, MsgFeeNotFound, http.StatusNotFound)
		return 0, "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return 0, "", false
	}
	return feeCents, doulaStaffID, true
}

// recordAndRespond writes the Activity row for a stored Fee Payment view,
// attributed to the acting Staff member, then answers 201 with it.
func recordAndRespond(w http.ResponseWriter, r *http.Request, c staffauth.Call, doulaStaffID string, action activity.EngagementAction, view View) {
	if err := record(r.Context(), c.Tx, c.PracticeID, c.EngagementID, doulaStaffID, action, c.StaffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	apierr.WriteJSON(w, http.StatusCreated, view)
}

// record writes the Engagement's Activity row for a Fee Payment or its
// reversal, naming the Doula and never the amount.
func record(ctx context.Context, tx *sql.Tx, practiceID, engagementID, doulaStaffID string, action activity.EngagementAction, staffID string) error {
	diff, err := json.Marshal(map[string]string{activity.DiffKeyAttachedStaffID: doulaStaffID})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly
		return fmt.Errorf("feepayment: marshal diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Diff:        diff,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feepayment: record activity: %w", err)
	}
	return nil
}

// RecordHandler records that the Practice paid the Attachment's whole
// accepted fee. Refused when a Fee Payment already stands on it. Owner
// and Admin only, declared at the mount.
func RecordHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		attachmentID := c.ID("attachmentId")
		req, paidOn, ok := payments.ReadRecordPaymentRequest(w, r, c.Tx, c.PracticeID, payments.DatePaid)
		if !ok {
			return
		}

		feeCents, doulaStaffID, ok := requireFee(w, r, c, attachmentID)
		if !ok {
			return
		}
		var alreadyPaid bool
		if err := c.Tx.QueryRowContext(r.Context(),
			`SELECT EXISTS (
			     SELECT 1 FROM attachment_fee_payments f
			      WHERE f.attachment_id = $1 AND `+paymentstanding.FeeStandsExpr+`)`,
			attachmentID,
		).Scan(&alreadyPaid); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if alreadyPaid {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgFeeAlreadyPaid, nil)
			return
		}

		var note sql.NullString
		if req.Note != "" {
			note = sql.NullString{String: req.Note, Valid: true}
		}
		view := View{
			AttachmentID: attachmentID,
			Kind:         kindRecorded,
			AmountCents:  feeCents,
			Method:       string(req.Method),
			PaidOn:       paidOn.Format(time.DateOnly),
			RecordedAt:   clock.Now(r.Context()),
		}
		if err := c.Tx.QueryRowContext(r.Context(),
			`INSERT INTO attachment_fee_payments
			     (practice_id, attachment_id, kind, amount_cents, method, note, paid_on, recorded_by, recorded_at)
			 VALUES ($1, $2, 'recorded', $3, $4, $5, $6, $7, $8) RETURNING id`,
			c.PracticeID, attachmentID, feeCents, string(req.Method), note, paidOn, c.StaffID, view.RecordedAt,
		).Scan(&view.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if note.Valid {
			view.Note = &note.String
		}
		recordAndRespond(w, r, c, doulaStaffID, activity.ActionContractorFeePaid, view)
	}, staffauth.EngagementID(), staffauth.PathID("attachment", "attachmentId"))
}

// ReverseHandler undoes a mistaken Fee Payment with an additive reversal
// row that nets it to zero (#945's shape): never an edit or a delete. A
// reversal cannot itself be reversed, and a Fee Payment is reversed once.
// Owner and Admin only, declared at the mount.
func ReverseHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		attachmentID, paymentID := c.ID("attachmentId"), c.ID("paymentId")
		var req ReverseRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.Reason == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"A reason is required.",
				map[string]string{"reason": "Enter a reason for removing this payment"})
			return
		}

		_, doulaStaffID, ok := requireFee(w, r, c, attachmentID)
		if !ok {
			return
		}
		var amountCents int64
		var alreadyReversed bool
		err := c.Tx.QueryRowContext(r.Context(),
			`SELECT f.amount_cents, `+paymentstanding.FeeReversedExpr+`
			   FROM attachment_fee_payments f
			  WHERE f.id = $1 AND f.attachment_id = $2 AND f.kind = 'recorded'`,
			paymentID, attachmentID,
		).Scan(&amountCents, &alreadyReversed)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, MsgFeePaymentNotReversible, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if alreadyReversed {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgFeePaymentAlreadyReversed, nil)
			return
		}

		view := View{
			AttachmentID: attachmentID,
			Kind:         kindReversal,
			AmountCents:  -amountCents,
			TargetID:     &paymentID,
			Reason:       &req.Reason,
			RecordedAt:   clock.Now(r.Context()),
		}
		if err := c.Tx.QueryRowContext(r.Context(),
			`INSERT INTO attachment_fee_payments
			     (practice_id, attachment_id, kind, amount_cents, target_id, reason, recorded_by, recorded_at)
			 VALUES ($1, $2, 'reversal', $3, $4, $5, $6, $7) RETURNING id`,
			c.PracticeID, attachmentID, view.AmountCents, paymentID, req.Reason, c.StaffID, view.RecordedAt,
		).Scan(&view.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		recordAndRespond(w, r, c, doulaStaffID, activity.ActionContractorFeePaymentReversed, view)
	}, staffauth.EngagementID(), staffauth.PathID("attachment", "attachmentId"), staffauth.PathID("payment", "paymentId"))
}
