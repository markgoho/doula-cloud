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
// handlers across packages (invite, role assignment, here, and
// billing.PostPurchaseHandler) the same way RequireTx is -- exported so
// billing doesn't need its own copy of the owner check.
func RequireOwner(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	reader, has := ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !reader.Has(roleOwner) {
		apierr.WriteError(w, "only a Practice Owner can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}

// RequireOwnerOrAdmin is RequireOwner widened by one role, for the writes
// ADR-0008 puts in an Admin's hands as well as an Owner's -- making an
// Offer, withdrawing one, completing an Engagement. Owner-only stays the
// default for anything that changes who is at the Practice at all
// (inviting, editing a Membership); this is for running the work.
// Zero-query, for the same reason RequireOwner is.
func RequireOwnerOrAdmin(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = PracticeID(r.Context())
	reader, has := ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !reader.IsOwnerOrAdmin() {
		apierr.WriteError(w, "only a Practice Owner or Admin can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}
