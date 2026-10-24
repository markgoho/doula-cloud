package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

// termsAcceptedDiff is the row's diff: the version of each document in
// force when she agreed, which is the date it takes effect; each version
// in the history that had not started yet, which she agreed to as well
// (ADR-0053, #1816); and the address the request came from. A key for
// coming versions is left out when there are none. Plain text, never
// sealed: the record of an agreement outlives the Practice and the
// Owner's login (ADR-0053).
type termsAcceptedDiff struct {
	TermsVersion   string   `json:"termsVersion"`
	TermsComing    []string `json:"termsComing,omitempty"`
	PrivacyVersion string   `json:"privacyVersion"`
	PrivacyComing  []string `json:"privacyComing,omitempty"`
	IPAddress      string   `json:"ipAddress"`
}

// recordTermsAccepted writes the agreement in the caller's transaction,
// so the Practice and the record of its agreement land together or not
// at all. The versions are legal's -- the one place the BFF holds them --
// read at the server's instant, and never anything the request carried.
// It reads the same history the agreement gate does (AgreementDocuments),
// so the row an Owner's press writes names the versions the gate asked
// her about (#1549).
func recordTermsAccepted(ctx context.Context, tx *sql.Tx, r *http.Request, practiceID, ownerStaffID string) error {
	now := clock.Now(ctx)
	record := termsAcceptedDiff{IPAddress: clientip.From(r)}
	for _, doc := range AgreementDocuments {
		inForce, coming := agreedVersions(doc, now)
		switch doc.Path {
		case legal.Terms.Path:
			record.TermsVersion, record.TermsComing = inForce, coming
		case legal.Privacy.Path:
			record.PrivacyVersion, record.PrivacyComing = inForce, coming
		}
	}
	diff, err := json.Marshal(record)
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
		CreatedAt:   now,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: record terms_accepted: %w", err)
	}
	return nil
}

// agreedVersions is the version of doc in force at now and the versions
// still to come, each by its date. legal_test.go holds that each shipped
// document has a version in force today, so the version is never empty.
func agreedVersions(doc legal.Document, now time.Time) (inForce string, coming []string) {
	version, _ := doc.InForce(now)
	for _, v := range doc.Coming(now) {
		coming = append(coming, v.Effective)
	}
	return version.Effective, coming
}
