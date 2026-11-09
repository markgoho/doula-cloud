package feedback

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/dbtx"
	"doula-cloud/api/internal/tasknudge"
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

// Send finishes a send both populations' handlers share: it fills in row
// the fields every sender has -- the id it mints, what the client wrote
// in, the browser parsed from User-Agent and the instant -- saves it,
// commits tx, and fires the nudge that tells the issue worker there is
// something to open. The caller sets the sender and the context that
// depends on who sent it. It returns the new id, or writes the failure
// response and reports ok=false.
//
// A send commits its own write rather than running inside a request-scoped
// transaction (billing.QueueOutOfCreditsNotification's own reasoning), so
// the nudge fires immediately rather than through tasknudge.Register.
func Send(w http.ResponseWriter, r *http.Request, tx *sql.Tx, enq tasknudge.Enqueuer, in ClientInput, row Row) (id string, ok bool) {
	row.ID = uuid.NewString()
	row.Kind = in.Kind
	row.Text = in.Text
	row.PageURL = in.Page.URL
	row.RouteID = in.Page.Route.ID
	row.AppBuild = in.AppBuild
	row.ScreenWidth = in.ScreenWidth
	row.Browser = ParseBrowser(r.Header.Get("User-Agent"))
	row.SentAt = clock.Now(r.Context())
	if !Save(r.Context(), w, tx, row) {
		// coverage:ignore reason: Save already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return "", false
	}
	if !dbtx.Commit(w, tx) {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure, not exercised by unit tests
		return "", false
	}
	tasknudge.Fire(enq, tasknudge.FeedbackIssue)(r.Context())
	return row.ID, true
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
