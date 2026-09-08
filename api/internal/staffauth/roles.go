package staffauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// validRoles is the practice_role enum from 00002_practice_staff_tenancy.sql,
// mirrored here so role-assignment requests can be validated before they
// ever reach Postgres.
var validRoles = map[string]bool{roleOwner: true, roleAdmin: true, "doula": true}

// RequireOwner resolves the caller's Reader and request-scoped tx from
// context (both set by staffauth.Middleware) and confirms the caller
// holds the 'owner' role at that Practice, writing the appropriate error
// response itself if not. Zero-query: the Reader already carries the
// roles Middleware resolved for this request. Shared by Owner-only
// handlers across packages -- inside staffauth (invite, role assignment,
// the MFA switch, ending sessions, the recovery vouch) and outside it
// (Practice deletion, export, payments Connect onboarding, and
// client.EraseEligibilityHandler) -- the same way RequireTx is, exported
// so no package needs its own copy of the owner check. A write whose
// Owner-only rule is the whole rule can declare it at the mount instead,
// through idempotency.Router.ExemptGated (#970, #990, #1016); the calls
// left here are the GETs, whose role the mount already declares through
// GatedRouter.Get, and the writes not yet moved.
func RequireOwner(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	reader, has := ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !reader.Has(roleOwner) {
		apierr.WriteError(w, "only a Practice Owner can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}

// MsgContractorMoneyRefused is the 403 body every route ADR-0008's money
// row (as amended by #282) refuses a contractor Doula with -- named once
// so RequireNotAmbientContractor and a route that needs the reader-only
// check without a tx (contracts.GetSignedContractPDFHandler) can never
// drift onto two different wordings for the same refusal.
const MsgContractorMoneyRefused = "a contractor Doula cannot read the Practice's money -- only her own agreed fee, on an Engagement she holds a granted attachment on"

// RequireNotAmbientContractor resolves the caller's Reader and
// request-scoped tx from context and confirms the caller is not a plain
// contractor Doula -- ADR-0008's money row as amended by #282: a
// Contract's amount, its Invoice and payment history, and the ledger's
// money entries open to an Owner, an Admin, and an employed Doula alike;
// only a contractor is refused, and her own agreed fee reaches her
// through her Engagement attachment, never this route. Zero-query, for
// the same reason RequireOwnerOrAdmin is.
func RequireNotAmbientContractor(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	reader, has := ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if reader.IsAmbientContractor() {
		apierr.WriteError(w, MsgContractorMoneyRefused, http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}

// RequireOwnerOrAdmin is RequireOwner widened by one role, for the acts
// ADR-0008 puts in an Admin's hands as well as an Owner's -- completing
// an Engagement, and the Owner/Admin reads that carry the same seat
// (contracts.awaiting, engagementrequest.List/Detail). Owner-only stays
// the default for anything that changes who is at the Practice at all
// (inviting, editing a Membership); this is for running the work.
// Zero-query, for the same reason RequireOwner is. As with RequireOwner,
// a write whose Owner-or-Admin rule is the whole rule declares it at the
// mount instead (#970, #990, #1016), not here.
func RequireOwnerOrAdmin(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	reader, has := ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !reader.IsOwnerOrAdmin() {
		apierr.WriteError(w, "only a Practice Owner or Admin can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}
