package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
)

const (
	msgNothingRefundable = "This Practice holds no purchased Credits that can be refunded. Credits given free, already spent or bought more than three years ago are not refundable."
	msgStripeRefused     = "Stripe did not make the refund, so nothing has changed. Try again."
	msgCorrectAnswers    = "Correct the answers marked below."
	msgOtherField        = "Send only the number of Credits, the Owner who asked and the date the request arrived."

	msgCreditsMissing   = "Enter the number of Credits to refund, 1 or more"
	msgOwnerMissing     = "Choose the Owner who asked for the refund"
	msgDateMissing      = "Enter the date the request arrived"
	msgDateInTheFuture  = "Date the request arrived must be today or in the past"
	msgCreditsOverLimit = "Number of Credits must be %d or fewer"
)

const fieldCredits = "credits"

type refundOwner struct {
	StaffID string `json:"staffId"`
	Name    string `json:"name"`
}

type refundCheck struct {
	PracticeName string               `json:"practiceName"`
	Owners       []refundOwner        `json:"owners"`
	Quote        *billing.RefundQuote `json:"quote"`
}

// The operator comes from the session, never from the body.
type refundRequest struct {
	Credits     int    `json:"credits"`
	RequestedBy string `json:"requestedBy"`
	RequestedOn string `json:"requestedOn"`
}

type refundResult struct {
	PracticeName    string `json:"practiceName"`
	Credits         int    `json:"credits"`
	AmountCents     int64  `json:"amountCents"`
	TaxCents        int64  `json:"taxCents"`
	RequestedByName string `json:"requestedByName"`
	RequestedOn     string `json:"requestedOn"`
}

type refundActDetails struct {
	RequestKey         string `json:"requestKey"`
	RequestedByStaffID string `json:"requestedByStaffId"`
	RequestedOn        string `json:"requestedOn"`
	Credits            int    `json:"credits"`
	AmountCents        int64  `json:"amountCents"`
	TaxCents           int64  `json:"taxCents"`
}

type refundDiff struct {
	Credits     int   `json:"credits"`
	AmountCents int64 `json:"amountCents"`
	TaxCents    int64 `json:"taxCents"`
}

func readRefundPractice(ctx context.Context, tx *sql.Tx, practiceID string) (string, []refundOwner, error) {
	rows, err := tx.QueryContext(ctx, `SELECT practice_name, owner_staff_id, owner_name FROM ops_refund_practice($1)`, practiceID)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return "", nil, fmt.Errorf("read practice: %w", err)
	}
	defer func() { _ = rows.Close() }()
	name := ""
	found := false
	owners := []refundOwner{}
	for rows.Next() {
		var staffID, ownerName sql.NullString
		if err := rows.Scan(&name, &staffID, &ownerName); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			return "", nil, fmt.Errorf("scan practice: %w", err)
		}
		found = true
		if staffID.Valid {
			owners = append(owners, refundOwner{StaffID: staffID.String, Name: ownerName.String})
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return "", nil, fmt.Errorf("read practice: %w", err)
	}
	if !found {
		return "", nil, errNoPractice
	}
	return name, owners, nil
}

func writeOverLimit(ctx context.Context, w http.ResponseWriter, tx *sql.Tx, practiceID string) {
	most, err := billing.QuoteRefund(ctx, tx, practiceID, 0, clock.Now(ctx))
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		writeInternal(w)
		return
	}
	apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectAnswers,
		map[string]string{fieldCredits: fmt.Sprintf(msgCreditsOverLimit, most.MaxCredits)})
}

func refundCheckRead(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		practiceID, ok := practiceIDFrom(w, r)
		if !ok {
			return
		}
		credits := 0
		if raw := r.URL.Query().Get("credits"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectAnswers,
					map[string]string{fieldCredits: msgCreditsMissing})
				return
			}
			credits = n
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		name, owners, err := readRefundPractice(r.Context(), tx, practiceID)
		if errors.Is(err, errNoPractice) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgPracticeNotFound, nil)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		check := refundCheck{PracticeName: name, Owners: owners}
		quote, err := billing.QuoteRefund(r.Context(), tx, practiceID, credits, clock.Now(r.Context()))
		switch {
		case errors.Is(err, billing.ErrRefundExceedsLot):
			writeOverLimit(r.Context(), w, tx, practiceID)
			return
		case errors.Is(err, billing.ErrNothingRefundable):
		case err != nil:
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		default:
			check.Quote = &quote
		}
		apierr.WriteJSON(w, http.StatusOK, check)
	}
}

func answerErrors(req refundRequest, now time.Time) map[string]string {
	details := map[string]string{}
	if req.Credits < 1 {
		details[fieldCredits] = msgCreditsMissing
	}
	if _, err := uuid.Parse(req.RequestedBy); err != nil {
		details["requestedBy"] = msgOwnerMissing
	}
	_, err := time.Parse(time.DateOnly, req.RequestedOn)
	switch {
	case err != nil:
		details["requestedOn"] = msgDateMissing
	case req.RequestedOn > businessToday(now).Format(time.DateOnly):
		details["requestedOn"] = msgDateInTheFuture
	}
	return details
}

func refundAct(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		practiceID, ok := practiceIDFrom(w, r)
		if !ok {
			return
		}
		requestKey, ok := requestKeyOf(w, r, "refund")
		if !ok {
			return
		}
		var req refundRequest
		if !decodeDeclared(w, r, refundFields, msgOtherField, &req) {
			return
		}
		now := clock.Now(r.Context())
		if details := answerErrors(req, now); len(details) > 0 {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectAnswers, details)
			return
		}

		op := signedIn(r)
		tx, err := d.DB.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := spendUnlessReplay(r.Context(), tx, op, actRefund, requestKey); err != nil {
			// coverage:ignore reason: a DB failure, or a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
			writeStepUpRequired(w)
			return
		}
		name, owners, err := readRefundPractice(r.Context(), tx, practiceID)
		if errors.Is(err, errNoPractice) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgPracticeNotFound, nil)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		ownerName, isOwner := "", false
		for _, o := range owners {
			if o.StaffID == req.RequestedBy {
				ownerName, isOwner = o.Name, true
			}
		}
		if !isOwner {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectAnswers,
				map[string]string{"requestedBy": msgOwnerMissing})
			return
		}

		var actID string
		receipt, err := billing.Refund(r.Context(), tx, d.Stripe,
			billing.RefundOrder{PracticeID: practiceID, Credits: req.Credits, RequestKey: requestKey}, now,
			func(ctx context.Context, q billing.RefundQuote) (string, error) {
				var err error
				actID, err = recordAct(ctx, tx, op, actRefund, practiceSubject(practiceID), refundActDetails{
					RequestKey: requestKey, RequestedByStaffID: req.RequestedBy, RequestedOn: req.RequestedOn,
					Credits: q.Credits, AmountCents: q.AmountCents, TaxCents: q.TaxCents,
				}, op.StepUpAt, now)
				return actID, err
			})
		switch {
		case errors.Is(err, billing.ErrNothingRefundable):
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgNothingRefundable, nil)
			return
		case errors.Is(err, billing.ErrRefundExceedsLot):
			writeOverLimit(r.Context(), w, tx, practiceID)
			return
		case errors.Is(err, billing.ErrStripeRefused):
			apierr.Write(w, http.StatusBadGateway, apierr.CodeInternal, msgStripeRefused, nil)
			return
		case err != nil:
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}

		if actID != "" {
			diff, _ := json.Marshal(refundDiff{Credits: receipt.Credits, AmountCents: receipt.AmountCents, TaxCents: receipt.TaxCents})
			if err := activity.Record(r.Context(), tx, activity.Entry{
				PracticeID:  practiceID,
				SubjectKind: activity.SubjectPractice,
				SubjectID:   practiceID,
				Action:      string(activity.ActionCreditsRefunded),
				Diff:        diff,
				Actor:       activity.OperatorActor(actID),
				CreatedAt:   now,
			}); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				writeInternal(w)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, refundResult{
			PracticeName:    name,
			Credits:         receipt.Credits,
			AmountCents:     receipt.AmountCents,
			TaxCents:        receipt.TaxCents,
			RequestedByName: ownerName,
			RequestedOn:     req.RequestedOn,
		})
	}
}

var refundFields = map[string]bool{fieldCredits: true, "requestedBy": true, "requestedOn": true}
