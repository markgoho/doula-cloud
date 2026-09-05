package contracts

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/staffauth"
)

// SignedPDFObjectPath is the deterministic object-store key for a
// Contract's Signed PDF -- one Contract per Engagement (UNIQUE
// (engagement_id), 00016_contracts.sql), so the Engagement id alone
// scopes it, mirroring message's "messages/<engagementId>/<messageId>"
// shape. Exported so tests can compute the key Sign wrote to without
// duplicating the format string.
func SignedPDFObjectPath(engagementID string) string {
	return fmt.Sprintf("contracts/%s/signed.pdf", engagementID)
}

// GetSignedContractPDFHandler streams the Signed PDF for :engagementId's
// Contract back to the calling Staff member. 404s if the Contract has
// never been signed (or doesn't exist at all) -- the PDF is written
// atomically with the sent -> signed transition (sign.go), so
// signed_pdf_object_path is never NULL on a signed row. Must be mounted
// behind staffauth.Middleware.
func GetSignedContractPDFHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}
		serveSignedPDF(w, r, tx, store, engagementID, staffauth.MsgInternalError)
	})
}

// ClientGetSignedContractPDFHandler mirrors GetSignedContractPDFHandler
// for the Client-portal population: clientauth.Middleware has already
// confirmed the caller's Client owns :engagementId. Must be mounted
// behind clientauth.Middleware.
func ClientGetSignedContractPDFHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())
		serveSignedPDF(w, r, tx, store, engagementID, clientauth.MsgInternalError)
	})
}

// serveSignedPDF looks up engagementID's signed_pdf_object_path (only set
// once status = 'signed') and streams it from store. Shared by
// GetSignedContractPDFHandler and ClientGetSignedContractPDFHandler;
// Postgres RLS on the contracts row -- practice-tier or client-tier,
// depending on which handler's tx set the session variable -- is what
// actually gates which engagementID a caller can reach here at all.
func serveSignedPDF(w http.ResponseWriter, r *http.Request, tx *sql.Tx, store objectstore.ObjectStore, engagementID, internalErrorMsg string) {
	var objectPath string
	err := tx.QueryRowContext(r.Context(),
		`SELECT signed_pdf_object_path FROM contracts
		 WHERE engagement_id = $1 AND status = $2::contract_status AND signed_pdf_object_path IS NOT NULL`,
		engagementID, statusSigned,
	).Scan(&objectPath)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, "no signed contract found for this engagement", http.StatusNotFound)
		return
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		apierr.WriteError(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	obj, err := store.Get(r.Context(), objectPath)
	if err != nil {
		apierr.WriteError(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}
	defer func() { _ = obj.Close() }()

	w.Header().Set("Content-Type", contentTypePDF)
	w.Header().Set("Content-Disposition", "inline; filename=\"contract.pdf\"")
	// coverage:ignore reason: response streaming failure, not exercised by unit tests
	if _, err := io.Copy(w, obj); err != nil {
		return
	}
}
