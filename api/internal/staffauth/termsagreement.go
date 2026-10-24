package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/legal"
)

// AgreementDocuments is the version history the agreement gate reads:
// legal's two documents. Production never assigns it. It is a variable,
// and exported, so a test here or in main can hand the gate a history
// with a material version in it, because the shipped history holds none
// yet.
var AgreementDocuments = []legal.Document{legal.Terms, legal.Privacy}

// agreementDueKey carries what Middleware found due for this request's
// Practice, so the session read and the agree press do not ask again.
const agreementDueKey contextKey = "staffauth.agreementDue"

// DueVersion is one material version an Owner of this Practice has to
// agree to before she enters it (#1549, ADR-0053): the document by its
// address without the slash ("terms" or "privacy"), its name, the
// version by the date it took effect, and the one line from the
// version's own record that says what changed.
type DueVersion struct {
	Document string `json:"document"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Change   string `json:"change"`
}

// agreementDueFrom returns what Middleware found due. Empty for a
// Staff member who is not an Owner: she is never asked.
func agreementDueFrom(ctx context.Context) []DueVersion {
	due, _ := ctx.Value(agreementDueKey).([]DueVersion)
	return due
}

// dueCandidates is every material version, of every document, that has
// started at now, oldest first within each document. dueAgreementQuery
// picks which of them, if any, a Practice is asked about.
func dueCandidates(docs []legal.Document, now time.Time) []DueVersion {
	today := now.UTC().Format(time.DateOnly)
	var candidates []DueVersion
	for _, doc := range docs {
		for _, v := range doc.Versions {
			if v.Material && v.Effective <= today {
				candidates = append(candidates, DueVersion{
					Document: strings.TrimPrefix(doc.Path, "/"),
					Name:     doc.Name,
					Version:  v.Effective,
					Change:   v.Change,
				})
			}
		}
	}
	return candidates
}

// dueAgreementQuery answers, for each document, the newest candidate
// that was published as material -- the email went out before it started
// (#1548), so the screen is never the first notice of a change
// (ADR-0053) -- when no terms_accepted row of this Practice has agreed to
// it. A candidate the operator never published is passed over for an
// older one that was, never treated as nothing due. A Practice that
// agreed to the newest has agreed to every change before it, so only the
// newest is asked about.
//
// A row agrees to a version when it names it as a version still coming
// (termsComing, privacyComing), the rule ADR-0053's #1816 amendment
// gives, or names it or a later one as the version in force
// (termsVersion, privacyVersion). A later one in force counts because its
// text holds every change before it: a Practice that signed up after a
// material version and a later correction names only the correction. A
// later coming version does not count, because that row was written
// before the version in question started.
//
// The keys are the document's address and the diff's own JSON names
// (termsAcceptedDiff): "terms" || 'Version' is termsVersion. Versions
// compare as text: each is YYYY-MM-DD, which sorts the way the dates do.
const dueAgreementQuery = `
SELECT n.document, n.version
  FROM (SELECT DISTINCT ON (c.document) c.document, c.version
          FROM unnest($2::text[], $3::text[]) AS c (document, version)
         WHERE EXISTS (
                 SELECT 1 FROM legal_version_publications p
                  WHERE p.document = c.document AND p.version = c.version::date AND p.material)
         ORDER BY c.document, c.version DESC) AS n
 WHERE NOT EXISTS (
         SELECT 1 FROM activity a
          WHERE a.practice_id = $1 AND a.subject_kind = 'practice' AND a.subject_id = $1
            AND a.action = 'terms_accepted'
            AND (a.diff ->> (n.document || 'Version') >= n.version
                 OR coalesce(a.diff -> (n.document || 'Coming'), '[]'::jsonb) ? n.version))`

// dueAgreement is what an Owner of practiceID has to agree to at now, in
// the order of AgreementDocuments. With no material version started --
// every day until the first one -- it costs no query.
func dueAgreement(ctx context.Context, tx *sql.Tx, practiceID string, now time.Time) ([]DueVersion, error) {
	candidates := dueCandidates(AgreementDocuments, now)
	if len(candidates) == 0 {
		return nil, nil
	}
	documents := make([]string, len(candidates))
	versions := make([]string, len(candidates))
	for i, c := range candidates {
		documents[i], versions[i] = c.Document, c.Version
	}
	rows, err := tx.QueryContext(ctx, dueAgreementQuery, practiceID, documents, versions)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: read terms agreement due: %w", err)
	}
	defer func() { _ = rows.Close() }()
	due := map[[2]string]bool{}
	for rows.Next() {
		var document, version string
		if err := rows.Scan(&document, &version); err != nil {
			// coverage:ignore reason: two text columns always scan into strings
			return nil, fmt.Errorf("staffauth: scan terms agreement due: %w", err)
		}
		due[[2]string{document, version}] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("staffauth: iterate terms agreement due: %w", err)
	}
	var out []DueVersion
	for _, c := range candidates {
		if due[[2]string{c.Document, c.Version}] {
			out = append(out, c)
		}
	}
	return out, nil
}

// isAgreementExemptRoute reports whether r is one of the requests the
// agreement gate never stands in front of (ADR-0053, ADR-0048): the
// session read the app's layout needs to learn what is due, the press
// that agrees, the export, every deletion route, and the push
// unsubscribe that signing out sends. Signing out itself (DELETE
// /api/session) and the refund of an unspent Credit (an operator's
// POST /api/internal/billing/refunds) are not Practice-scoped, so this
// gate never sees them.
func isAgreementExemptRoute(r *http.Request, practiceID string) bool {
	base := "/api/practices/" + practiceID
	switch r.URL.Path {
	case base + "/session", base + "/export":
		return r.Method == http.MethodGet
	case base + "/terms-agreement":
		return r.Method == http.MethodPost
	case base + "/deletion":
		return true
	case base + "/push-subscriptions":
		return r.Method == http.MethodDelete
	}
	return false
}

// writeTermsAgreementRequired writes the agreement gate's refusal.
func writeTermsAgreementRequired(w http.ResponseWriter) {
	apierr.Write(w, http.StatusForbidden, apierr.CodeTermsAgreementRequired,
		"an Owner of this Practice has to agree to the new Terms of Service or Privacy Policy", nil)
}

// AgreeToTermsHandler answers POST .../terms-agreement: an Owner
// agrees, for her Practice, to the material versions that have started
// (#1549). It writes one terms_accepted row, the same shape signup
// writes: the version of each document in force now and any still to
// come, the address the request came from, and the server's instant.
//
// When nothing is due -- a second Owner after the first agreed, or a
// press repeated -- it writes nothing and answers the same 204, so the
// first Owner who agrees settles it for the Practice.
func AgreeToTermsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this handler runs
			return
		}
		if len(agreementDueFrom(r.Context())) > 0 {
			staffID, _ := StaffID(r.Context())
			if err := recordTermsAccepted(r.Context(), tx, r, practiceID, staffID); err != nil {
				// coverage:ignore reason: DB failure inside recordTermsAccepted, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
