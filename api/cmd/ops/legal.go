package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/money"
)

const (
	msgLegalNoFields     = "Send no fields with the publication. The version comes from its address, and who publishes it from your sign-in."
	msgNoSuchVersion     = "There is no such version. The Terms of Service and the Privacy Policy each name a version by the date it takes effect."
	msgAlreadyPublished  = "This version was already published on %s, and nothing has changed."
	msgLegalKeyReused    = "This Idempotency-Key was already used to publish a different version. Nothing has changed."
	msgPublishRaced      = "This version was published while this publication was being made, so nothing has changed."
	msgNoticeTooShort    = "A material version has to take effect at least 30 days after the emails are sent, and this one takes effect on %s. Nothing has changed."
	msgPriceRiseTooShort = "A version that raises the price of a Credit has to take effect at least 60 days after the emails are sent, and this one takes effect on %s. Nothing has changed."
	msgNoStripePrice     = "No Stripe Price charges the %s a Credit costs in this version. Create it first (docs/environment.md, \"A rise in the price of a Credit\"). Nothing has changed."
)

// shippedLegal is the version history in api/internal/legal, the one
// place a version's date, whether it is material, and its one line of
// change come from. A version is written there and on the site in one
// commit; ops publishes it.
var shippedLegal = []legal.Document{legal.Terms, legal.Privacy}

// legalVersion is one version as the list states it.
type legalVersion struct {
	Document     string     `json:"document"`
	DocumentName string     `json:"documentName"`
	Version      string     `json:"version"`
	Change       string     `json:"change"`
	Material     bool       `json:"material"`
	PublishedAt  *time.Time `json:"publishedAt"`
}

type legalVersionList struct {
	Items []legalVersion `json:"items"`
}

// legalVersionCheck is what the check page states before the act: the
// version, and how many Owners publishing it now emails.
type legalVersionCheck struct {
	Document     string     `json:"document"`
	DocumentName string     `json:"documentName"`
	Version      string     `json:"version"`
	Change       string     `json:"change"`
	Material     bool       `json:"material"`
	Owners       int        `json:"owners"`
	PublishedAt  *time.Time `json:"publishedAt"`
}

// legalPublication is what the act changed.
type legalPublication struct {
	Document     string    `json:"document"`
	DocumentName string    `json:"documentName"`
	Version      string    `json:"version"`
	Material     bool      `json:"material"`
	PublishedAt  time.Time `json:"publishedAt"`
	Queued       int       `json:"queued"`
}

// legalActDetails is the publication's operator_acts details.
type legalActDetails struct {
	RequestKey string `json:"requestKey"`
	Document   string `json:"document"`
	Version    string `json:"version"`
	Material   bool   `json:"material"`
}

type publicationRow struct {
	material    bool
	publishedAt time.Time
	actID       *string
	queued      int
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// publicationsOf reads every published version through
// ops_legal_publications, keyed "document version".
func publicationsOf(ctx context.Context, q querier) (map[string]publicationRow, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT document, version, material, published_at, operator_act_id, queued FROM ops_legal_publications()`)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("read publications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	published := map[string]publicationRow{}
	for rows.Next() {
		var document, version string
		var p publicationRow
		if err := rows.Scan(&document, &version, &p.material, &p.publishedAt, &p.actID, &p.queued); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			return nil, fmt.Errorf("scan publication: %w", err)
		}
		p.publishedAt = p.publishedAt.UTC()
		published[document+" "+version] = p
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("read publications: %w", err)
	}
	return published, nil
}

// legalVersions lists every version in the history, each document's
// newest first, and when each was published.
func legalVersions(db *sql.DB, docs []legal.Document) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		published, err := publicationsOf(r.Context(), db)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		list := legalVersionList{Items: []legalVersion{}}
		for _, doc := range docs {
			for _, v := range slices.Backward(doc.Versions) {
				item := legalVersion{Document: legalnotice.Key(doc), DocumentName: doc.Name, Version: v.Effective, Change: v.Change, Material: v.Material}
				if p, ok := published[item.Document+" "+v.Effective]; ok {
					item.PublishedAt = &p.publishedAt
				}
				list.Items = append(list.Items, item)
			}
		}
		apierr.WriteJSON(w, http.StatusOK, list)
	}
}

// versionFrom is the version the address names, or a 404 written.
func versionFrom(w http.ResponseWriter, r *http.Request, docs []legal.Document) (legal.Document, legal.Version, bool) {
	doc, version, ok := legalnotice.Find(docs, r.PathValue("document"), r.PathValue("version"))
	if !ok {
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgNoSuchVersion, nil)
	}
	return doc, version, ok
}

// legalVersionCheckRead answers the check page. The Owners are counted
// by ops_legal_notice_owner_count, which reads the rule the emails are
// queued by.
func legalVersionCheckRead(db *sql.DB, docs []legal.Document) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doc, version, ok := versionFrom(w, r, docs)
		if !ok {
			return
		}
		check := legalVersionCheck{
			Document: legalnotice.Key(doc), DocumentName: doc.Name, Version: version.Effective,
			Change: version.Change, Material: version.Material,
		}
		if err := db.QueryRowContext(r.Context(),
			`SELECT CASE WHEN $3 THEN ops_legal_notice_owner_count() ELSE 0 END,
			        (SELECT p.published_at FROM ops_legal_publications() p WHERE p.document = $1 AND p.version = $2)`,
			check.Document, check.Version, check.Material,
		).Scan(&check.Owners, &check.PublishedAt); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, check)
	}
}

// legalRefusal is why the Terms do not let version be published at now,
// or "" when they do (ADR-0053, #1817, #1834). A version of the Terms
// states the price of a Credit, and from its date checkout charges it
// through a Stripe Price at that amount, so a missing Price is found now
// rather than on the date.
func legalRefusal(ctx context.Context, prices billing.StripeClient, version legal.Version, now time.Time) string {
	on := longDate(version.Effective)
	if version.PriceRise != "" && !legalnotice.LeavesNotice(version.Effective, now, legalnotice.PriceRiseLead) {
		return fmt.Sprintf(msgPriceRiseTooShort, on)
	}
	if version.Material && !legalnotice.LeavesNotice(version.Effective, now, legalnotice.NoticeLead) {
		return fmt.Sprintf(msgNoticeTooShort, on)
	}
	if version.CreditPriceCents > 0 {
		if err := prices.CheckCreditPrice(ctx, version.CreditPriceCents); err != nil {
			log.Printf("doula-ops: check credit price of %d cents: %v", version.CreditPriceCents, err)
			return fmt.Sprintf(msgNoStripePrice, money.Dollars(version.CreditPriceCents))
		}
	}
	return ""
}

// longDate writes a version's date the way the site does, "January 1,
// 2027". A version is a calendar date, so no zone moves it.
func longDate(isoDate string) string {
	day, err := time.Parse(time.DateOnly, isoDate)
	// coverage:ignore reason: legal_test.go holds every version's Effective to a date
	if err != nil {
		return isoDate
	}
	return day.Format(longDateLayout)
}

// legalPublishAct publishes one version (#1548, ADR-0062's act table):
// behind a fresh code, under an Idempotency-Key so a retry is the same
// publication. It is recorded as an operator_acts row the publication
// keys, and a material version's emails are queued in the same
// transaction by ops_publish_legal_version.
func legalPublishAct(d Deps) http.HandlerFunc {
	docs := d.legalHistory()
	return func(w http.ResponseWriter, r *http.Request) {
		requestKey, ok := requestKeyOf(w, r, "publication")
		if !ok {
			return
		}
		doc, version, ok := versionFrom(w, r, docs)
		if !ok {
			return
		}
		if !decodeDeclared(w, r, map[string]bool{}, msgLegalNoFields, &struct{}{}) {
			return
		}
		document := legalnotice.Key(doc)
		answer := func(p publicationRow) {
			apierr.WriteJSON(w, http.StatusOK, legalPublication{
				Document: document, DocumentName: doc.Name, Version: version.Effective,
				Material: p.material, PublishedAt: p.publishedAt, Queued: p.queued,
			})
		}

		op := signedIn(r)
		now := clock.Now(r.Context())
		published, err := publicationsOf(r.Context(), d.DB)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		prior, isPublished := published[document+" "+version.Effective]
		if !isPublished {
			if refusal := legalRefusal(r.Context(), d.Stripe, version, now); refusal != "" {
				apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeFailedPrecondition, refusal, nil)
				return
			}
		}

		tx, err := d.DB.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := spendUnlessReplay(r.Context(), tx, op, actLegalVersionPublished, requestKey); err != nil {
			// coverage:ignore reason: a DB failure, or a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
			writeStepUpRequired(w)
			return
		}

		var priorActID string
		err = tx.QueryRowContext(r.Context(),
			`SELECT id FROM operator_acts
			  WHERE operator_id = $1 AND action = 'legal_version_published' AND details->>'requestKey' = $2`,
			op.ID, requestKey).Scan(&priorActID)
		if err == nil {
			if !isPublished || prior.actID == nil || *prior.actID != priorActID {
				apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgLegalKeyReused, nil)
				return
			}
			if err := tx.Commit(); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				writeInternal(w)
				return
			}
			answer(prior)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if isPublished {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				fmt.Sprintf(msgAlreadyPublished, businessDay(prior.publishedAt)), nil)
			return
		}

		actID, err := recordAct(r.Context(), tx, op, actLegalVersionPublished, nil,
			legalActDetails{RequestKey: requestKey, Document: document, Version: version.Effective, Material: version.Material}, op.StepUpAt, now)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		var queued *int
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_publish_legal_version($1, $2, $3, $4, $5)`,
			actID, document, version.Effective, version.Material, now).Scan(&queued); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if queued == nil {
			// coverage:ignore reason: a second publication committed between this request's read and its insert; ON CONFLICT is what refuses it
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgPublishRaced, nil)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		answer(publicationRow{material: version.Material, publishedAt: now, queued: *queued})
	}
}
