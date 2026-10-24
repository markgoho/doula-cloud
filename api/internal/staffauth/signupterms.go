package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
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

// ComingVersion is one version of a document that has not started yet,
// which a Practice that signs up now agrees to as well (#1824): the
// document by its address without the slash ("terms" or "privacy"), its
// name, and the version by the date it starts -- the same three fields
// DueVersion gives the screen after a material change, so the app links
// each one's own page the same way.
type ComingVersion struct {
	Document string `json:"document"`
	Name     string `json:"name"`
	Version  string `json:"version"`
}

// comingVersionsResponse is GET /api/legal/coming's body. Coming is
// never null: an empty list is the ordinary answer outside a notice
// period.
type comingVersionsResponse struct {
	Coming []ComingVersion `json:"coming"`
}

// ComingVersionsHandler answers GET /api/legal/coming: every version of
// the Terms of Service and the Privacy Policy that has not started at the
// server's instant, in the order of AgreementDocuments and oldest first
// within each document. The signup form names each one beside its
// agreement sentence (#1824, ADR-0053), and reads them here, not from a
// copy built into the app, so that the form and the terms_accepted row
// the press writes name the same versions: both walk AgreementDocuments
// through agreedVersions at clock.Now(ctx).
//
// Open to anyone: it says only what /terms and /privacy already print,
// reads no input and no database, and holds a few entries.
func ComingVersionsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := clock.Now(r.Context())
		out := comingVersionsResponse{Coming: []ComingVersion{}}
		for _, doc := range AgreementDocuments {
			_, coming := agreedVersions(doc, now)
			for _, version := range coming {
				out.Coming = append(out.Coming, ComingVersion{
					Document: strings.TrimPrefix(doc.Path, "/"),
					Name:     doc.Name,
					Version:  version,
				})
			}
		}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// agreedVersions is the version of doc in force at now and the versions
// still to come, each by its date. recordTermsAccepted and
// ComingVersionsHandler both read it, so the row and the signup form
// cannot disagree about which versions are coming (#1824). legal_test.go holds that each shipped
// document has a version in force today, so the version is never empty.
func agreedVersions(doc legal.Document, now time.Time) (inForce string, coming []string) {
	version, _ := doc.InForce(now)
	for _, v := range doc.Coming(now) {
		coming = append(coming, v.Effective)
	}
	return version.Effective, coming
}
