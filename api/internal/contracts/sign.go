package contracts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"doula-cloud/api/internal/clientauth"
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
			statusSigned, req.FullLegalName, req.Attestation, clientIP(r), objectPath, id,
		); err != nil {
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

// clientIP is the ESIGN signer_ip of record. The BFF runs behind Cloud
// Run's Google Front End, which terminates the caller's own TLS
// connection and sets X-Forwarded-For's first entry to that connection's
// real peer address itself -- a caller can't spoof this the way it could
// a header GFE merely passed through, since GFE is the one writing it,
// not relaying client-supplied content. r.RemoteAddr, by contrast, is
// GFE's own proxy address at that point, not the caller's -- only useful
// as the local-dev/test fallback when there's no GFE in front of the
// process and the header is absent.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
		return r.RemoteAddr
	}
	return host
}
