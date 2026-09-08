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

// MsgNoSignedContract is the 404 body both Signed-PDF routes answer with
// when the Engagement has no stored Signed PDF at all. It deliberately
// says "has ever been signed" rather than "is signed": a voided
// Contract's PDF is still served (#299), so a refusal here is about a
// document that was never produced, never about the Contract's status
// right now.
const MsgNoSignedContract = "no contract on this engagement has ever been signed"

// SignedPDFObjectPath is the deterministic object-store key for one
// Contract's Signed PDF. It is keyed on the Contract id, not on the
// Engagement id alone: #72's partial unique index
// (contracts_engagement_id_active_key, 00020_contracts_recreate_after_void.sql)
// lets an Engagement accumulate any number of voided Contracts beside
// its one live row, so an Engagement can hold more than one Contract
// that has been signed. Under the old Engagement-only key, the second
// signing overwrote the first Contract's PDF -- the evidence Void exists
// to preserve. The Engagement id stays in the key as the scoping prefix,
// mirroring message's "messages/<engagementId>/<messageId>" shape.
// Exported so tests can compute the key Sign wrote to without
// duplicating the format string.
func SignedPDFObjectPath(engagementID, contractID string) string {
	return fmt.Sprintf("contracts/%s/%s/signed.pdf", engagementID, contractID)
}

// GetSignedContractPDFHandler streams the Signed PDF for :engagementId's
// Contract back to the calling Staff member. 404s if no Contract on the
// Engagement has ever been signed (or none exists at all) -- the PDF is
// written atomically with the sent -> signed transition (sign.go),
// so signed_pdf_object_path is set on exactly the rows that have been
// signed, and its absence is the whole reason for the 404. A Contract
// that has since been voided still serves: voiding cancels an agreement,
// it does not withdraw the evidence that the agreement was made (#299).
//
// Who may read it, per ADR-0008's money row as amended by #282: Owner,
// Admin, and an employed Doula. The JSON Contract read no longer splits
// scope from money at all (#282 retired that split), but a rendered PDF
// was never split that way to begin with, so this route follows the
// money row wholesale -- refusing a contractor Doula regardless of any
// attachment she holds, since the whole document is money-bearing and
// her own fee is never on it. Voiding changes none of that. Must be
// mounted behind staffauth.Middleware.
func GetSignedContractPDFHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if reader.IsAmbientContractor() {
			apierr.WriteError(w, staffauth.MsgContractorMoneyRefused, http.StatusForbidden)
			return
		}
		serveSignedPDF(w, r, tx, store, engagementID, apierr.MsgInternalError)
	})
}

// ClientGetSignedContractPDFHandler mirrors GetSignedContractPDFHandler
// for the Client-portal population: clientauth.Middleware has already
// confirmed the caller's Client owns :engagementId, and the client-tier
// RLS policy on contracts is the backstop. ADR-0006 governs Staff roles
// and says nothing about the Client; the rule here is the narrower one
// the portal has always carried -- the Client of that Engagement and
// nobody else -- and a void neither widens nor narrows it. Must be
// mounted behind clientauth.Middleware.
func ClientGetSignedContractPDFHandler(store objectstore.ObjectStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())
		serveSignedPDF(w, r, tx, store, engagementID, apierr.MsgInternalError)
	})
}

// serveSignedPDF looks up a stored Signed PDF for engagementID and
// streams it from store. Shared by GetSignedContractPDFHandler and
// ClientGetSignedContractPDFHandler; Postgres RLS on the contracts row
// -- practice-tier or client-tier, depending on which handler's tx set
// the session variable -- is what actually gates which engagementID a
// caller can reach here at all.
//
// The lookup deliberately does not compare status. signed_pdf_object_path
// is written only by the sent -> signed transition (sign.go), so
// IS NOT NULL already refuses a Draft or a sent-but-unsigned Contract,
// which is the only refusal this read ever wanted. Comparing status on
// top of it also refused a voided Contract whose PDF was deliberately
// preserved (#299), and would refuse any future status nobody has
// thought of yet.
//
// Which PDF, where an Engagement holds more than one signed Contract:
// the most recently created one. #72's partial unique index lets voided
// rows accumulate beside the one live Contract, so this is reachable --
// void, recreate, sign again. Creation order is signing order here (a
// second Contract can only be created once the first is voided, and only
// a sent Contract can be signed), and it is the same ORDER BY
// fetchContract uses, so every read in this package resolves an
// Engagement to the same Contract. id breaks a tie that two rows created
// in the same instant would otherwise leave to row order.
func serveSignedPDF(w http.ResponseWriter, r *http.Request, tx *sql.Tx, store objectstore.ObjectStore, engagementID, internalErrorMsg string) {
	var objectPath string
	err := tx.QueryRowContext(r.Context(),
		`SELECT signed_pdf_object_path FROM contracts
		 WHERE engagement_id = $1 AND signed_pdf_object_path IS NOT NULL
		 ORDER BY created_at DESC, id DESC LIMIT 1`,
		engagementID,
	).Scan(&objectPath)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, MsgNoSignedContract, http.StatusNotFound)
		return
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		apierr.WriteError(w, internalErrorMsg, http.StatusInternalServerError)
		return
	}

	obj, err := store.Get(r.Context(), objectPath)
	if errors.Is(err, objectstore.ErrNotFound) {
		apierr.WriteError(w, "signed PDF not found", http.StatusNotFound)
		return
	}
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
