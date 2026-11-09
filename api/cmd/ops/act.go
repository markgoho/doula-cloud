package main

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbtx"
)

const headerIdempotencyKey = "Idempotency-Key"

const (
	msgKeyRequired      = "Send an Idempotency-Key header with the %s."
	msgPracticeNotFound = "There is no Practice at this address."
)

const (
	actSignIn                = "sign_in"
	actSignOut               = "sign_out"
	actStepUpRefused         = "step_up_refused"
	actRefund                = "refund"
	actFoundingGrant         = "founding_grant"
	actSecondFactorCleared   = "second_factor_cleared"
	actOwnerContactRead      = "owner_contact_read"
	actFeedbackRead          = "feedback_read"
	actPilotAddressAdded     = "pilot_address_added"
	actPilotGateOpened       = "pilot_gate_opened"
	actLegalVersionPublished = "legal_version_published"
	actStaffLookup           = "staff_lookup"
)

var errNoPractice = errors.New("no such practice")

func requestKeyOf(w http.ResponseWriter, r *http.Request, what string) (string, bool) {
	key := r.Header.Get(headerIdempotencyKey)
	if key == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, fmt.Sprintf(msgKeyRequired, what), nil)
		return "", false
	}
	return key, true
}

// recordRead records the signed-in operator's read of Platform data as an
// act, answering 500 itself when it cannot, and reports whether it did.
func recordRead(w http.ResponseWriter, r *http.Request, tx *sql.Tx, action string, subject *actSubject, details any) bool {
	if _, err := recordAct(r.Context(), tx, signedIn(r), action, subject, details, nil, clock.Now(r.Context())); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		writeInternal(w)
		return false
	}
	return true
}

func writeInternal(w http.ResponseWriter) {
	apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
}

func practiceIDFrom(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("practiceId")
	if _, err := uuid.Parse(id); err != nil {
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgPracticeNotFound, nil)
		return "", false
	}
	return id, true
}

// A request naming its own actor is refused, not ignored.
func decodeDeclared(w http.ResponseWriter, r *http.Request, declared map[string]bool, msgOther string, target any) bool {
	var fields map[string]jsontext.Value
	if !apierr.DecodeJSON(w, r, &fields) {
		return false
	}
	for name := range fields {
		if !declared[name] {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgOther, nil)
			return false
		}
	}
	raw, _ := json.Marshal(fields)
	if err := json.Unmarshal(raw, target); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgOther, nil)
		return false
	}
	return true
}

type actSubject struct {
	Kind string
	ID   string
}

func practiceSubject(id string) *actSubject { return &actSubject{Kind: "practice", ID: id} }

func staffSubject(id string) *actSubject { return &actSubject{Kind: "staff", ID: id} }

func feedbackSubject(id string) *actSubject { return &actSubject{Kind: "feedback", ID: id} }

func recordAct(ctx context.Context, q rowQuerier, op operator, action string, subject *actSubject, details any, stepUpAt *time.Time, at time.Time) (string, error) {
	raw := []byte("{}")
	if details != nil {
		raw, _ = json.Marshal(details)
	}
	var kind, id *string
	if subject != nil {
		kind, id = &subject.Kind, &subject.ID
	}
	var actID string
	if err := q.QueryRowContext(ctx,
		`INSERT INTO operator_acts (operator_id, action, subject_kind, subject_id, details, step_up_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		op.ID, action, kind, id, raw, stepUpAt, at).Scan(&actID); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return "", fmt.Errorf("record %s: %w", action, err)
	}
	return actID, nil
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func isReplay(ctx context.Context, q rowQuerier, op operator, action, key string) (bool, error) {
	var found bool
	if err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM operator_acts WHERE operator_id = $1 AND action = $2 AND details->>'requestKey' = $3)`,
		op.ID, action, key).Scan(&found); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return false, fmt.Errorf("read earlier %s: %w", action, err)
	}
	return found, nil
}

// A replay answers what the first request did and needs no code of its own.
// beginSpentAct opens the transaction of a step-up-gated act and spends the
// operator's step-up for it unless this request key already did, answering
// the refusal itself. The caller defers dbtx.Rollback once it has the
// transaction.
func beginSpentAct(w http.ResponseWriter, r *http.Request, db *sql.DB, op operator, action, requestKey string) (*sql.Tx, bool) {
	tx, ok := dbtx.Begin(w, r, db)
	if !ok {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return nil, false
	}
	if err := spendUnlessReplay(r.Context(), tx, op, action, requestKey); err != nil {
		// coverage:ignore reason: a DB failure, or a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
		dbtx.Rollback(tx)
		writeStepUpRequired(w)
		return nil, false
	}
	return tx, true
}

func spendUnlessReplay(ctx context.Context, tx *sql.Tx, op operator, action, key string) error {
	replay, err := isReplay(ctx, tx, op, action, key)
	if err != nil || replay {
		return err
	}
	return spendStepUp(ctx, tx, op)
}
