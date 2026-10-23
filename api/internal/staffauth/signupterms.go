package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/clientip"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/legal"
)

// ActionTermsAccepted records a Practice agreeing, through its Owner, to
// the Terms of Service and the Privacy Policy (ADR-0053). Written against
// activity.SubjectPractice with the Owner as a StaffActor: at signup
// (#1547), and again when an Owner agrees to a later material version
// (#1549), which writes the same row with the new versions.
//
// A constant of this package rather than of a shared Practice vocabulary
// because that vocabulary does not exist yet: #1449 holds it, and the
// action is named there.
const ActionTermsAccepted = "terms_accepted"

// termsAcceptedDiff is the row's diff: the version of each document she
// agreed to, which is the date it takes effect, and the address the
// request came from. Plain text, never sealed: the record of an agreement
// outlives the Practice and the Owner's login (ADR-0053).
type termsAcceptedDiff struct {
	TermsVersion   string `json:"termsVersion"`
	PrivacyVersion string `json:"privacyVersion"`
	IPAddress      string `json:"ipAddress"`
}

// recordTermsAccepted writes the agreement in the caller's transaction,
// so the Practice and the record of its agreement land together or not
// at all. The versions are legal's current ones -- the one place the BFF
// holds them -- and never anything the request carried; the instant is
// the server's.
func recordTermsAccepted(ctx context.Context, tx *sql.Tx, r *http.Request, practiceID, ownerStaffID string) error {
	diff, err := json.Marshal(termsAcceptedDiff{
		TermsVersion:   legal.Terms.Current().Effective,
		PrivacyVersion: legal.Privacy.Current().Effective,
		IPAddress:      clientip.From(r),
	})
	// coverage:ignore reason: json.Marshal of a struct of three strings cannot fail
	if err != nil {
		return fmt.Errorf("staffauth: marshal terms_accepted diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      ActionTermsAccepted,
		Diff:        diff,
		Actor:       activity.StaffActor(ownerStaffID),
		CreatedAt:   clock.Now(ctx),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: record terms_accepted: %w", err)
	}
	return nil
}
