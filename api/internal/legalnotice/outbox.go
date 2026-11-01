package legalnotice

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/website"
)

// Worker sends due legal_notice_outbox rows through outbox.MailWorker's
// shared claim-scan-compose-send-mark loop. One row is one Owner at one
// Practice, so a send that fails, or an address that is suppressed
// (ADR-0029), is recorded on that Owner's own row.
type Worker = outbox.MailWorker[pendingRow]

// NewWorker builds the worker around mailer. docs is the version history
// the email's one line of what changed is read from: legal.Terms and
// legal.Privacy in production.
func NewWorker(mailer outbox.Mailer, docs []legal.Document) Worker {
	return Worker{
		Mailer:     mailer,
		Table:      "legal_notice_outbox",
		ClaimQuery: claimQuery,
		Scan:       scanRow,
		Compose:    compose(mailer, docs),
	}
}

// pendingRow is one due row, joined to the Practice for its name and to
// the Owner for her current address.
type pendingRow struct {
	document     string
	version      string
	practiceID   string
	practiceName string
	address      string
	loginDeleted bool
}

// The address is read here, at send time, never stored on the row
// (ADR-0033). staff is readable through 00033's notification-worker
// policy, the door outbox.ProcessHandler opens; practices carries no RLS.
const claimQuery = `SELECT o.id, o.attempt_count, o.document, o.version::text, o.practice_id, p.name, s.email, s.deleted_at IS NOT NULL
	   FROM legal_notice_outbox o
	   JOIN practices p ON p.id = o.practice_id
	   JOIN staff s ON s.id = o.staff_id
	  WHERE o.status = 'pending' AND o.next_attempt_at <= now()
	  ORDER BY o.next_attempt_at
	  LIMIT $1
	  FOR UPDATE OF o SKIP LOCKED`

func scanRow(rows *sql.Rows) (outbox.RowMeta, pendingRow, error) {
	var meta outbox.RowMeta
	var p pendingRow
	if err := rows.Scan(&meta.ID, &meta.AttemptCount, &p.document, &p.version, &p.practiceID, &p.practiceName, &p.address, &p.loginDeleted); err != nil {
		// coverage:ignore reason: DB scan failure, not exercised by unit tests
		return meta, p, fmt.Errorf("legalnotice: scan outbox row: %w", err)
	}
	return meta, p, nil
}

// compose writes one row's email. A row for an Owner who deleted her
// login after it was queued, or for a version the history does not hold,
// is dead-lettered with the reason: no retry could send it, and the row
// must say it was not sent rather than look delivered.
func compose(mailer outbox.Mailer, docs []legal.Document) func(context.Context, *sql.Tx, pendingRow, time.Time) (string, string, string, error) {
	return func(_ context.Context, _ *sql.Tx, p pendingRow, _ time.Time) (string, string, string, error) {
		if p.loginDeleted {
			return "", "", "", &outbox.DeadLetterError{Reason: "the Owner's login was deleted before the email was sent"}
		}
		doc, version, ok := Find(docs, p.document, p.version)
		if !ok {
			return "", "", "", &outbox.DeadLetterError{Reason: "no version " + p.version + " of " + p.document + " in the history"}
		}
		settings := mailer.AppBaseURL + "/practices/" + p.practiceID + "/settings"
		// The version's own page, not the document's: until the date,
		// /terms and /privacy show the version in force, and "Read the new
		// version" has to land on the new one (#1816).
		// site/src/routes/terms/[version] and privacy/[version] build one
		// page for each version.
		documentURL := website.SiteBaseURL + doc.Path + "/" + version.Effective
		subject, text := Compose(Values{
			Document:    doc.Name,
			Date:        longDate(version.Effective),
			Practice:    p.practiceName,
			Change:      version.Change,
			Reason:      version.PriceRise,
			DocumentURL: documentURL,
			SettingsURL: settings,
			DeleteURL:   settings + "/delete",
		})
		return p.address, subject, text, nil
	}
}

// longDate writes a version's date the way the email prints it,
// "December 1, 2026". The date was parsed when the version was
// published, and Postgres hands it back in the same form.
func longDate(effective string) string {
	d, err := time.Parse(time.DateOnly, effective)
	// coverage:ignore reason: the date column only ever holds a date Postgres wrote as YYYY-MM-DD
	if err != nil {
		return effective
	}
	return d.Format("January 2, 2006")
}
