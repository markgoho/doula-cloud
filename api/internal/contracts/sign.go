package contracts

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/clientip"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/objectstore"
)

// SignContractRequest is the body of the Client-portal Sign request: only
// what the Client themselves supplies -- the typed full legal name and
// the attestation checkbox state. signed_at and the signer's IP are never
// accepted from the request body; ClientPostSignContractHandler derives
// both from the request itself.
type SignContractRequest struct {
	FullLegalName string `json:"fullLegalName"`
	Attestation   bool   `json:"attestation"`
}

// ClientPostSignContractHandler transitions the Contract for the
// Client-portal caller's Engagement from 'sent' to 'signed' -- the only
// transition it permits; any other current status 409s. The typed full
// legal name and attestation checkbox state come from the request body;
// signed_at (the request's clock.Now) and signer_ip (the caller's remote
// address) are derived here, never trusted from the body, since both are
// part of the ESIGN legal record. As part of the same transition, it
// renders the filled Contract (prose with merge fields substituted, per
// fillProse) and its signature block -- the typed name, signed_at in the
// Practice's zone, and the consent she checked (#2214) -- to PDF and
// uploads it to store before persisting -- the permanent,
// never-re-rendered record of what the Client actually saw and agreed
// to (#71). Mirrors PostSendContractHandler's shape: the status check
// above is what gates the transition; the contracts_client_sign RLS
// policy (00018_contracts_signing.sql) is the backstop that keeps the
// UPDATE itself scoped to a 'sent' Contract on the caller's own Engagement
// even if a future bug skipped that check. Must be mounted behind
// clientauth.Middleware.
func ClientPostSignContractHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var req SignContractRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.FullLegalName = strings.TrimSpace(req.FullLegalName)
		if req.FullLegalName == "" || !req.Attestation {
			apierr.WriteError(w, "full legal name and attestation are both required to sign", http.StatusBadRequest)
			return
		}

		id, prose, status, values, amountCents, amountChangedAt, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if ok, refusal := TransitionSign.Check(Status(status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}
		values = withResolvedPrice(extractMergeFields(prose), values, amountCents)

		signer, err := loadSigningEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware has already resolved this Engagement on this tx
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// One instant, read once: the same value is printed on the PDF and
		// stored as signed_at, so the paper and the record cannot disagree
		// (#2214). Postgres keeps microseconds, so the value is cut to them
		// before either use.
		signedAt := clock.Now(r.Context()).Truncate(time.Microsecond)

		pdfBytes, err := renderContractPDF(fillProse(prose, values), signature{
			fullLegalName: req.FullLegalName,
			signedAt:      signedAt.In(signer.zone),
		})
		if err != nil {
			// coverage:ignore reason: renderContractPDF only fails on an internal fpdf encoding error, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		objectPath := SignedPDFObjectPath(engagementID, id)
		if err := store.Put(r.Context(), objectPath, contentTypePDF, bytes.NewReader(pdfBytes)); err != nil {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts
			 SET status = $1::contract_status, signer_full_name = $2, signer_attestation = $3,
			     signed_at = $4, signer_ip = $5, signed_pdf_object_path = $6
			 WHERE id = $7`,
			string(StatusSigned), req.FullLegalName, req.Attestation, signedAt, clientip.From(r), objectPath, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordContractSigned(r.Context(), tx, engagementID, signer); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := ContractResponse{
			EngagementID:    engagementID,
			Status:          string(StatusSigned),
			Prose:           prose,
			MergeFields:     extractMergeFields(prose),
			Values:          values.nonEmpty(),
			AmountChangedAt: amountChangedAt,
		}
		writeContract(w, r, tx, http.StatusOK, out)
	})
}

// recordContractSigned writes #476's activity row for the Client's own
// signature -- actor_kind 'client' (ADR-0022: "Amara signed the
// Contract" is her act, never a system event), engagementID as
// subject_id. This handler runs behind clientauth.Middleware, which sets
// only app.current_client_id, never app.current_practice_id -- activity's
// RLS policy needs the latter, so activity.ScopedTo widens it as part of
// the one Record call below.
func recordContractSigned(ctx context.Context, tx *sql.Tx, engagementID string, signer signingEngagement) error {
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  signer.practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionContractSigned),
		Actor:       activity.ClientActor(signer.clientID),
	}, activity.ScopedTo(signer.practiceID)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: record contract signed: %w", err)
	}
	return nil
}

// signingEngagement is what the Sign transition reads about the
// Engagement being signed for: whose Practice and whose Client it is, for
// the activity row, and the Practice's own zone, the one the signed PDF
// prints the signing time in (ADR-0036).
type signingEngagement struct {
	practiceID string
	clientID   string
	zone       *time.Location
}

// loadSigningEngagement reads signingEngagement on the Client-portal tx.
// practices carries no row-level security of its own (practicetimezone.Load),
// so the join reads the zone under the client-tier session the same way
// plans.loadMasthead does.
func loadSigningEngagement(ctx context.Context, tx *sql.Tx, engagementID string) (signingEngagement, error) {
	var (
		out      signingEngagement
		zoneName string
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT e.practice_id, e.client_id, p.timezone
		 FROM engagements e JOIN practices p ON p.id = e.practice_id
		 WHERE e.id = $1`, engagementID,
	).Scan(&out.practiceID, &out.clientID, &zoneName); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return signingEngagement{}, fmt.Errorf("contracts: resolve engagement for signing: %w", err)
	}
	zone, err := ianazone.Parse(zoneName)
	if err != nil {
		// coverage:ignore reason: practices.timezone is only ever written through ianazone.Parse
		return signingEngagement{}, fmt.Errorf("contracts: load practice zone for signing: %w", err)
	}
	out.zone = zone
	return out, nil
}
