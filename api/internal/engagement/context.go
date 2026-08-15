// Package engagement holds the Staff-side BFF handlers for Client and
// Engagement: list, create, and view. All three rely on
// staffauth.Middleware having already resolved the caller's Staff/Practice
// ids and opened a request-scoped *sql.Tx with app.current_practice_id
// set, the same way staffauth's own Owner-only handlers (invite, role
// assignment) do.
package engagement

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// requireTx resolves the request-scoped tx and Practice id
// staffauth.Middleware placed on context, writing the appropriate error
// response itself if the tx is somehow missing. Shared by ListHandler,
// CreateHandler, and DetailHandler -- the same shape as staffauth's own
// requireOwner.
func requireTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := staffauth.Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	practiceID, _ = staffauth.PracticeID(r.Context())
	return tx, practiceID, true
}
