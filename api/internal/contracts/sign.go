package contracts

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/clientip"
	"doula-cloud/api/internal/objectstore"
)

const statusSigned = "signed"

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
// signed_at (server clock) and signer_ip (the caller's remote address)
// are derived here, never trusted from the body, since both are part of
// the ESIGN legal record. As part of the same transition, it renders the
// filled Contract (prose with merge fields substituted, per fillProse) to
// PDF and uploads it to store before persisting -- the permanent,
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
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var req SignContractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.FullLegalName = strings.TrimSpace(req.FullLegalName)
		if req.FullLegalName == "" || !req.Attestation {
			http.Error(w, "full legal name and attestation are both required to sign", http.StatusBadRequest)
			return
		}

		id, prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if status != statusSent {
			http.Error(w, "contract is not awaiting signature", http.StatusConflict)
			return
		}

		pdfBytes, err := renderContractPDF(fillProse(prose, values))
		if err != nil {
			// coverage:ignore reason: renderContractPDF only fails on an internal fpdf encoding error, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		objectPath := SignedPDFObjectPath(engagementID)
		if err := store.Put(r.Context(), objectPath, contentTypePDF, bytes.NewReader(pdfBytes)); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts
			 SET status = $1::contract_status, signer_full_name = $2, signer_attestation = $3,
			     signed_at = now(), signer_ip = $4, signed_pdf_object_path = $5
			 WHERE id = $6`,
			statusSigned, req.FullLegalName, req.Attestation, clientip.From(r), objectPath, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordContractSigned(r.Context(), tx, engagementID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       statusSigned,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// recordContractSigned writes #476's activity row for the Client's own
// signature -- actor_kind 'client' (ADR-0022: "Amara signed the
// Contract" is her act, never a system event), engagementID as
// subject_id. This handler runs behind clientauth.Middleware, which sets
// only app.current_client_id, never app.current_practice_id -- activity's
// RLS policy needs the latter, so activity.ScopeToPractice sets it here,
// immediately before the one Record call that needs it.
func recordContractSigned(ctx context.Context, tx *sql.Tx, engagementID string) error {
	var practiceID, clientID string
	if err := tx.QueryRowContext(ctx,
		`SELECT practice_id, client_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&practiceID, &clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: resolve engagement for contract signed: %w", err)
	}
	if err := activity.ScopeToPractice(ctx, tx, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: scope to practice for contract signed: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionContractSigned),
		Actor:       activity.ClientActor(clientID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: record contract signed: %w", err)
	}
	return nil
}
