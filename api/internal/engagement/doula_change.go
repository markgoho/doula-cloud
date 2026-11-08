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

// beginDoulaChange takes the caller and the two path ids staffauth.Handler
// parsed and locks the Engagement, writing the 404 or 500 itself.
func beginDoulaChange(w http.ResponseWriter, r *http.Request, call staffauth.Call) (c doulaChange, ok bool) {
	c = doulaChange{
		tx:           call.Tx,
		reader:       call.Reader,
		practiceID:   call.PracticeID,
		callerID:     call.StaffID,
		engagementID: call.EngagementID,
		doulaID:      call.ID("staffId"),
	}
	c.status, ok = lockEngagementStatus(w, r, c.tx, c.engagementID, c.practiceID, msgAttachDoulaNoEngagement)
	if !ok {
		return doulaChange{}, false
	}
	return c, true
}
