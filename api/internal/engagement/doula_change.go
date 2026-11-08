package engagement

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// doulaChange is what AttachDoulaHandler and TakeOffDoulaHandler resolve
// before they touch an Attachment: the caller, the Engagement locked for
// the change with its status, and the Doula being put on or taken off.
type doulaChange struct {
	tx           *sql.Tx
	reader       staffauth.Reader
	practiceID   string
	callerID     string
	engagementID string
	doulaID      string
	status       string
}

// beginDoulaChange reads the caller and the two path ids and locks the
// Engagement, writing the 400, 404 or 500 itself.
func beginDoulaChange(w http.ResponseWriter, r *http.Request) (c doulaChange, ok bool) {
	c.tx, c.practiceID, c.reader, ok = staffauth.RequireTxReader(w, r)
	if !ok {
		// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
		return doulaChange{}, false
	}
	c.callerID, _ = staffauth.StaffID(r.Context())
	c.engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", c.engagementID) {
		return doulaChange{}, false
	}
	c.doulaID = r.PathValue("staffId")
	if !staffauth.ParseUUID(w, "staff", c.doulaID) {
		return doulaChange{}, false
	}
	c.status, ok = lockEngagementStatus(w, r, c.tx, c.engagementID, c.practiceID, msgAttachDoulaNoEngagement)
	if !ok {
		return doulaChange{}, false
	}
	return c, true
}
