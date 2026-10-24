// Package legalnotice tells each Owner of each Practice, 30 days or more
// ahead, that a material version of the Terms of Service or the Privacy
// Policy is coming (ADR-0053, #1548), and 60 days or more ahead when it
// raises the price of a Credit (#1817). A change posted with no notice does
// not bind a Practice (Douglas v. Talk America), so the notice is the
// first half of how a material change takes effect; the screen an Owner
// agrees on after the date is the second (#1549).
//
// The version history stays in api/internal/legal. This package adds
// the act of publishing a version, which records who did it and when,
// and the outbox (ADR-0010) whose rows are the record of who was told.
package legalnotice

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/outbox"
)

// NoticeLead is the least time between the emails and the date a
// material version takes effect. The Terms promise it: "we email each
// Owner at least 30 days before the new version takes effect".
const NoticeLead = 30 * 24 * time.Hour

// PriceRiseLead is the least time between the emails and the date a
// version that raises the price of a Credit takes effect. The Terms
// promise it: "If the price of a Credit ever rises, we email each Owner
// at least 60 days before, with the reason" (#1817).
const PriceRiseLead = 60 * 24 * time.Hour

// CreditPrices is the seam over the one Stripe question the publish act
// asks: whether the credit Product has an active Price that checkout can
// charge at unitAmountCents. billing.StripeAPIClient answers it in
// production.
type CreditPrices interface {
	CheckCreditPrice(ctx context.Context, unitAmountCents int64) error
}

// PublishRequest is the body of a POST to PublishHandler: the document by
// its address without the slash ("terms" or "privacy"), the version by
// the date it takes effect, and who is publishing it.
type PublishRequest struct {
	Document    string `json:"document"`
	Version     string `json:"version"`
	PublishedBy string `json:"publishedBy"`
}

// Publication is a published version, as stored, and how many emails it
// queued.
type Publication struct {
	Document    string    `json:"document"`
	Version     string    `json:"version"`
	Material    bool      `json:"material"`
	PublishedBy string    `json:"publishedBy"`
	PublishedAt time.Time `json:"publishedAt"`
	Queued      int       `json:"queued"`
}

// key is how a document is named in the database: its address without
// the slash, the same name the request uses.
func key(doc legal.Document) string {
	return strings.TrimPrefix(doc.Path, "/")
}

// find is the version named by document and effective, in docs.
func find(docs []legal.Document, document, effective string) (legal.Document, legal.Version, bool) {
	for _, doc := range docs {
		if key(doc) != document {
			continue
		}
		for _, v := range doc.Versions {
			if v.Effective == effective {
				return doc, v, true
			}
		}
	}
	return legal.Document{}, legal.Version{}, false
}

// PublishHandler publishes one version of a document (#1548).
//
// An operator endpoint behind the internal guard, the same shape as the
// founding grant and the pilot allowlist: a new version is written into
// api/internal/legal and the site in one commit, and publishing it is
// then one deliberate act by a named person. The row it writes answers
// "who published this version, and when?".
//
// A material version queues one email for each Owner of each Practice
// that is not deleted and not in its deletion window, and is refused
// when it takes effect fewer than NoticeLead after now, or fewer than
// PriceRiseLead when it raises the price of a Credit. A version that
// is not material is recorded and queues nothing. Publishing a version
// twice keeps the first record and queues nothing more.
//
// docs is the version history to publish from: legal.Terms and
// legal.Privacy in production, a fixture in a test.
//
// A version of the Terms states the price of a Credit, and from its
// date checkout charges that price through a Stripe Price at the same
// amount (#1834). prices checks that such a Price exists before the
// version is published, so a forgotten Price is found 60 days ahead
// instead of on the date, when every purchase would be refused.
func PublishHandler(db *sql.DB, auth *internalauth.Guard, docs []legal.Document, prices CreditPrices) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.Require(w, r) {
			return
		}

		var req PublishRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		_, version, ok := find(docs, req.Document, req.Version)
		if !ok {
			apierr.WriteError(w, "no such version: document must be terms or privacy, and version the date one of its versions takes effect", http.StatusBadRequest)
			return
		}
		publishedBy := strings.TrimSpace(req.PublishedBy)
		if publishedBy == "" {
			apierr.WriteError(w, "publishedBy is required", http.StatusBadRequest)
			return
		}

		now := clock.Now(r.Context())
		if version.PriceRise != "" && !leavesNotice(version.Effective, now, PriceRiseLead) {
			apierr.WriteError(w,
				"a version that raises the price of a Credit must take effect at least 60 days after the emails are queued",
				http.StatusUnprocessableEntity)
			return
		}
		if version.Material && !leavesNotice(version.Effective, now, NoticeLead) {
			apierr.WriteError(w,
				"a material version must take effect at least 30 days after the emails are queued",
				http.StatusUnprocessableEntity)
			return
		}

		if version.CreditPriceCents > 0 {
			if err := prices.CheckCreditPrice(r.Context(), version.CreditPriceCents); err != nil {
				log.Printf("legalnotice: check credit price for %s %s: %v", req.Document, version.Effective, err)
				apierr.WriteError(w,
					"no Stripe Price charges the price of a Credit this version states: create it first (docs/environment.md, \"A rise in the price of a Credit\")",
					http.StatusUnprocessableEntity)
				return
			}
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// A no-op once Commit below has run: every refusal on this
		// endpoint is answered before the transaction begins.
		defer func() { _ = tx.Rollback() }()

		p, err := record(r.Context(), tx, req.Document, version, publishedBy, now)
		if err != nil {
			// coverage:ignore reason: DB failure inside record, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, p)
	})
}

// leavesNotice answers whether a version that takes effect on effective
// leaves lead between now and its start. The version is a date with no
// zone, so it is taken to start at midnight UTC, which is before
// midnight anywhere in the United States: the reading that never gives
// an Owner less than the lead.
func leavesNotice(effective string, now time.Time, lead time.Duration) bool {
	start, err := time.Parse(time.DateOnly, effective)
	// coverage:ignore reason: legal_test.go holds every version's Effective to a date, and a fixture is written the same way
	if err != nil {
		return false
	}
	return start.Sub(now) >= lead
}

// record writes the publication and, for the first publication of a
// material version, its emails, then reads back what is stored.
func record(ctx context.Context, tx *sql.Tx, document string, version legal.Version, publishedBy string, now time.Time) (Publication, error) {
	res, err := tx.ExecContext(ctx,
		`INSERT INTO legal_version_publications (document, version, material, published_by, published_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (document, version) DO NOTHING`,
		document, version.Effective, version.Material, publishedBy, now,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Publication{}, fmt.Errorf("legalnotice: record publication: %w", err)
	}
	inserted, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: the pq driver always reports rows affected for an INSERT
		return Publication{}, fmt.Errorf("legalnotice: read rows affected: %w", err)
	}

	if inserted == 1 && version.Material {
		if err := queue(ctx, tx, document, version.Effective, now); err != nil {
			// coverage:ignore reason: DB failure inside queue, not exercised by unit tests
			return Publication{}, err
		}
	}

	p := Publication{Document: document, Version: version.Effective}
	if err := tx.QueryRowContext(ctx,
		`SELECT pub.material, pub.published_by, pub.published_at,
		        (SELECT count(*) FROM legal_notice_outbox o WHERE o.document = pub.document AND o.version = pub.version)
		   FROM legal_version_publications pub
		  WHERE pub.document = $1 AND pub.version = $2`,
		document, version.Effective,
	).Scan(&p.Material, &p.PublishedBy, &p.PublishedAt, &p.Queued); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Publication{}, fmt.Errorf("legalnotice: read publication: %w", err)
	}
	return p, nil
}

// queue writes one outbox row for each Owner of each Practice that is not
// deleted and not in its deletion window (ADR-0031), and whose login is
// not deleted (ADR-0033). An Owner of two Practices gets a row for each,
// because each Practice agrees on its own.
//
// Owners are read across every Practice, so the transaction opens the
// notification door: it licenses 00033's policies on staff and
// practice_memberships, which exist so that a worker can find whom to
// notify, and finding whom to notify is what this query does. The
// endpoint sits behind the internal guard, like every worker that opens
// the same door.
func queue(ctx context.Context, tx *sql.Tx, document, effective string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `SELECT set_config($1, 'true', true)`, outbox.NotificationDoor); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("legalnotice: open notification door: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO legal_notice_outbox (document, version, practice_id, staff_id, created_at)
		 SELECT $1, $2, pm.practice_id, pm.staff_id, $3
		   FROM practice_memberships pm
		   JOIN practices p ON p.id = pm.practice_id
		   JOIN staff s ON s.id = pm.staff_id
		  WHERE 'owner' = ANY(pm.roles)
		    AND p.deleted_at IS NULL
		    AND p.deletion_requested_at IS NULL
		    AND s.deleted_at IS NULL`,
		document, effective, now,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("legalnotice: queue emails: %w", err)
	}
	return nil
}
