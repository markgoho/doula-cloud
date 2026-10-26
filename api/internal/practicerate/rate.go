// Package practicerate holds the Staff-side BFF handlers for a
// Practice's rate card (#966): the flat amount in cents it charges for
// each Engagement kind. Every Staff member with practice access reads
// it, contractors included -- GLOSSARY.md: "a Practice's published rates
// hold no person's information", the same reason ADR-0008's read table
// already gives every Doula the Contract Template. Only an Owner or an
// Admin may set, change or remove it. All handlers rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// contracts does.
package practicerate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
)

// actionRateChanged is #966's activity action -- one for both kinds,
// with which kind and the amount before and after in Diff, rather than a
// pair of kind-named actions (mfarequired.go's own
// mfa_required_enabled/disabled shape). A rate change has no analogous
// on/off pair to name, and a reader wanting a per-kind breakdown already
// has Diff.Kind to filter on.
const actionRateChanged = string(activity.ActionPracticeRateChanged)

// kinds is every Engagement kind #966's rate card names, in the fixed
// order GetRatesHandler renders them -- GLOSSARY.md's Engagement entry
// names exactly these two ("birth" or "postpartum, what the Practice
// sold"), so a third value here would mean GLOSSARY.md changed first.
var kinds = []engagement.Kind{engagement.KindBirth, engagement.KindPostpartum}

// validKind reports whether kind is one of the two path segments
// PutRateHandler accepts, mirroring engagementrequest.validKinds -- a BFF
// request path is untyped input, unlike engagement.Kind, which a caller
// already inside the Go program can rely on.
func validKind(kind string) bool {
	for _, k := range kinds {
		if string(k) == kind {
			return true
		}
	}
	return false
}

// Rate is one Engagement kind's rate. AmountCents is nil when the
// Practice has not set one yet -- #966's AC that a Practice with no rate
// set is a valid state -- so a consumer sees an explicit null rather
// than having to infer absence from a missing map key or a zero amount.
type Rate struct {
	Kind        string `json:"kind"`
	AmountCents *int64 `json:"amountCents"`
}

// RatesResponse is GetRatesHandler's body: always both Engagement kinds,
// whether or not each has a rate set.
type RatesResponse struct {
	Rates []Rate `json:"rates"`
}

// GetRatesHandler lets any Staff member at the current Practice read its
// rate card. Must be mounted behind staffauth.Middleware.
func GetRatesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		set, err := fetchRates(r, tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := make([]Rate, 0, len(kinds))
		for _, k := range kinds {
			rate := Rate{Kind: string(k)}
			if amount, found := set[string(k)]; found {
				rate.AmountCents = &amount
			}
			out = append(out, rate)
		}

		apierr.WriteJSON(w, http.StatusOK, RatesResponse{Rates: out})
	})
}

// fetchRates reads every rate row practiceID has set, keyed by kind.
// A kind absent from the returned map has no rate set.
func fetchRates(r *http.Request, tx *sql.Tx, practiceID string) (map[string]int64, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT kind, amount_cents FROM practice_rates WHERE practice_id = $1`, practiceID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("practicerate: query rates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	set := make(map[string]int64, len(kinds))
	for rows.Next() {
		var kind string
		var amountCents int64
		if err := rows.Scan(&kind, &amountCents); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("practicerate: scan rate: %w", err)
		}
		set[kind] = amountCents
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("practicerate: iterate rates: %w", err)
	}
	return set, nil
}

// RatePutRequest is PutRateHandler's body: PUT semantics, replacing one
// kind's whole rate.
type RatePutRequest struct {
	AmountCents int64 `json:"amountCents"`
}

// rateDiff is the activity diff shape PutRateHandler and
// DeleteRateHandler share. AmountCentsBefore is nil when the Practice
// had no rate set for Kind before this write; AmountCentsAfter is nil
// when this write removed the rate (#2080), so a set, a change and a
// removal all read as one practice_rate_changed action.
type rateDiff struct {
	Kind              string `json:"kind"`
	AmountCentsBefore *int64 `json:"amountCentsBefore"`
	AmountCentsAfter  *int64 `json:"amountCentsAfter"`
}

// recordRateChanged writes one practice_rate_changed activity row for
// diff, attributed to the Staff member making the request.
func recordRateChanged(r *http.Request, tx *sql.Tx, practiceID string, diff rateDiff) error {
	diffJSON, err := json.Marshal(diff)
	// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
	if err != nil {
		return fmt.Errorf("practicerate: marshal rate diff: %w", err)
	}
	actorStaffID, _ := staffauth.StaffID(r.Context())
	if err := activity.Record(r.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionRateChanged,
		Diff:        diffJSON,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicerate: record rate change: %w", err)
	}
	return nil
}

// readRate reads practiceID's current rate for kind; the result is not
// Valid when no rate is set.
func readRate(r *http.Request, tx *sql.Tx, practiceID, kind string) (sql.NullInt64, error) {
	var amount sql.NullInt64
	err := tx.QueryRowContext(r.Context(),
		`SELECT amount_cents FROM practice_rates WHERE practice_id = $1 AND kind = $2::engagement_kind`,
		practiceID, kind,
	).Scan(&amount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return amount, fmt.Errorf("practicerate: read rate: %w", err)
	}
	return amount, nil
}

// PutRateHandler lets a Practice Owner or Admin set or change the rate
// for one Engagement kind (#966's AC: "the API refuses a Doula, employee
// or contractor"). Must be mounted behind staffauth.Middleware.
//
// Idempotent by construction, the same "records an audit event only for
// an axis that actually changed" rule staffauth.PutMFARequiredHandler
// follows: a retry with the same body reads the same current amount,
// writes nothing, and records nothing new.
//
// Owner and Admin is declared at the mount, not checked here (#990,
// following #970's own move for Contract writes): the handler no longer
// calls staffauth.RequireOwnerOrAdmin, because a Doula is refused by the
// gate before this runs. Widening or narrowing this route means editing
// its ir.ExemptGated role list in mount.go.
func PutRateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		kind := r.PathValue("kind")
		if !validKind(kind) {
			apierr.WriteError(w, "kind must be birth or postpartum", http.StatusBadRequest)
			return
		}

		var req RatePutRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.AmountCents <= 0 {
			apierr.WriteError(w, "amountCents must be a positive integer", http.StatusBadRequest)
			return
		}

		before, err := readRate(r, tx, practiceID, kind)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if !before.Valid || before.Int64 != req.AmountCents {
			if _, err := tx.ExecContext(r.Context(),
				`INSERT INTO practice_rates (practice_id, kind, amount_cents) VALUES ($1, $2::engagement_kind, $3)
				 ON CONFLICT (practice_id, kind) DO UPDATE SET amount_cents = EXCLUDED.amount_cents`,
				practiceID, kind, req.AmountCents,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			after := req.AmountCents
			diff := rateDiff{Kind: kind, AmountCentsAfter: &after}
			if before.Valid {
				diff.AmountCentsBefore = &before.Int64
			}
			if err := recordRateChanged(r, tx, practiceID, diff); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			// #968: re-price every unsigned Contract for this kind to
			// the rate that just took effect -- a signed Contract, a
			// voided one, and one whose amount an Owner/Admin
			// overrode are all left alone (see
			// repriceUnsignedContracts's own doc comment).
			if err := repriceUnsignedContracts(r.Context(), tx, practiceID, kind, req.AmountCents); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		amountCents := req.AmountCents
		apierr.WriteJSON(w, http.StatusOK, Rate{Kind: kind, AmountCents: &amountCents})
	})
}

// DeleteRateHandler lets a Practice Owner or Admin remove the rate for
// one Engagement kind, so the kind has no rate again (#2080) -- #966's
// "no rate set" state, which a Practice that stops offering postpartum
// care must be able to return to. Must be mounted behind
// staffauth.Middleware, under the same Owner-and-Admin gate as
// PutRateHandler (mount.go).
//
// The removal is recorded as practice_rate_changed with the amount
// before and a null amount after, so who removed a rate and when reads
// the same way as who set it. Removing a rate that is not set writes and
// records nothing, so a retry is a no-op.
//
// A draft or sent Contract keeps the amount it was drafted at: there is
// no rate left to re-price it to, and its amount is a fact on the
// Contract, not a pointer to the rate card. A new Contract of this kind
// is refused the way it is for a Practice that never set one
// (contracts.noRateSetMsg, #967, #1885).
func DeleteRateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		kind := r.PathValue("kind")
		if !validKind(kind) {
			apierr.WriteError(w, "Choose the birth rate or the postpartum rate.", http.StatusBadRequest)
			return
		}

		before, err := readRate(r, tx, practiceID, kind)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if before.Valid {
			if _, err := tx.ExecContext(r.Context(),
				`DELETE FROM practice_rates WHERE practice_id = $1 AND kind = $2::engagement_kind`,
				practiceID, kind,
			); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if err := recordRateChanged(r, tx, practiceID, rateDiff{Kind: kind, AmountCentsBefore: &before.Int64}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, Rate{Kind: kind})
	})
}
