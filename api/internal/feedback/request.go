package feedback

import (
	"context"
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// RequireValid refuses a send whose kind is not one of the three choices
// or whose text is past MaxTextRunes, writing the 400 field error that
// names the field with the sender's own wording.
func RequireValid(w http.ResponseWriter, kind, text, kindNeeded, textTooLong string) bool {
	if !ValidKind(kind) {
		apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "kind", kindNeeded)
		return false
	}
	if !ValidText(text) {
		apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "text", textTooLong)
		return false
	}
	return true
}

// Save inserts row and queues the outbox row that opens its private GitHub
// issue (#1524) in the same transaction, so a crash between the two never
// leaves a piece of Feedback with no issue ever queued for it. It writes
// the 500 itself when either fails.
func Save(ctx context.Context, w http.ResponseWriter, tx *sql.Tx, row Row) bool {
	if err := Insert(ctx, tx, row); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if err := EnqueueIssueOutbox(ctx, tx, row.ID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}
