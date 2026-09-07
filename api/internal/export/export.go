package export

import (
	"archive/zip"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// exportScope is the plaintext diff the one Activity row this act writes
// carries -- what the export covered, mirroring erasureScope's own
// reasoning (client/erase.go): the row's own created_at already answers
// "when", so the diff only needs to answer "what".
type exportScope struct {
	Entities []string `json:"entities"`
}

// Handler streams a ZIP of one UTF-8 CSV per entity() row back to the
// calling Owner. Mounted with staffauth.OwnerOnly, and RequireOwner is
// called again here anyway -- belt-and-braces, the same shape
// client.EraseEligibilityHandler follows, since a route's mount-level
// gate and its handler's own gate are two different lines of defense.
//
// The Activity row is written first, in the same transaction as every
// read that follows. A DB failure partway through (any entity's own
// query) leaves that transaction aborted, so staffauth.Middleware's own
// tx.Commit() -- which only runs once this handler returns -- reports
// sql.ErrTxCommitRollback and the audit row never lands, Postgres having
// already rolled the whole transaction back on the first failed
// statement. A response-streaming failure (the client disconnects, the
// connection breaks) is a different case: every DB statement up to that
// point still succeeded, so the transaction is healthy and the Activity
// row does commit -- the export genuinely ran against the data, even
// though she never received the whole archive. Either way, logging and
// stopping is all a handler can do once the response is partway
// written: there is no refusal left to send.
//
// Must be mounted behind staffauth.Middleware.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			// coverage:ignore reason: belt-and-braces -- Mount's own
			// OwnerOnly declaration (g.Get) already refuses a non-owner
			// caller before this handler runs.
			return
		}

		var practiceName string
		if err := tx.QueryRowContext(r.Context(),
			`SELECT name FROM practices WHERE id = $1`, practiceID,
		).Scan(&practiceName); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		entities := entities()
		names := make([]string, len(entities))
		for i, e := range entities {
			names[i] = e.file
		}
		diff, _ := json.Marshal(exportScope{Entities: names})
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   practiceID,
			Action:      actionPracticeDataExported,
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		now := time.Now().UTC()
		filename := fmt.Sprintf("%s export %s.zip", practiceName, now.Format("2006-01-02"))
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		zw := zip.NewWriter(w)
		if err := writeReadme(r.Context(), tx, w, zw, practiceName, staffID, now, entities); err != nil {
			// coverage:ignore reason: DB query failure or response streaming failure, not exercised by unit tests
			log.Printf("export: readme: %v", err)
			return
		}
		for _, e := range entities {
			if err := writeEntity(r, w, tx, zw, practiceID, e); err != nil {
				// The response is already partway written -- headers and
				// however many entries came before this one are already on
				// the wire, so there is no refusal left to send. Logging
				// and stopping leaves the client an honestly truncated
				// archive rather than one that silently claims to be whole.
				//
				// coverage:ignore reason: DB query failure or response streaming failure, not exercised by unit tests
				log.Printf("export: %s: %v", e.file, err)
				return
			}
		}
		if err := zw.Close(); err != nil {
			// coverage:ignore reason: response streaming failure, not exercised by unit tests
			log.Printf("export: close: %v", err)
		}
	})
}

// writeEntity streams one entity's rows as a CSV file inside zw. e.query
// must select exactly len(e.header) columns, each already cast to text
// (::text, array_to_string(...), or a jsonb column, which is already
// valid text) -- so this function never needs to know what any column
// means, only how many there are. A NULL column reads back as an empty
// cell, the same rule every reader of this archive can rely on for
// every one of entities()'s 20-odd queries at once.
//
// Flushed twice per entity -- zw.Flush() pushes the compressed bytes out
// of zip's own buffering, and the ResponseWriter's Flush sends them to
// the client -- so a 14-doula agency's two years of Clients streams as
// it is read rather than buffering the whole archive in memory first.
func writeEntity(r *http.Request, w http.ResponseWriter, tx *sql.Tx, zw *zip.Writer, practiceID string, e entity) error {
	fw, err := zw.Create(e.file)
	if err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("create %s: %w", e.file, err)
	}
	cw := csv.NewWriter(fw)
	if err := cw.Write(e.header); err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("write header: %w", err)
	}

	rows, err := tx.QueryContext(r.Context(), e.query, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	n := len(e.header)
	raw := make([]sql.NullString, n)
	dest := make([]any, n)
	for i := range raw {
		dest[i] = &raw[i]
	}
	record := make([]string, n)
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("scan: %w", err)
		}
		for i := range raw {
			record[i] = raw[i].String
		}
		if err := cw.Write(record); err != nil {
			// coverage:ignore reason: response streaming failure, not exercised by unit tests
			return fmt.Errorf("write row: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("rows: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("flush csv: %w", err)
	}
	if err := zw.Flush(); err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("flush zip: %w", err)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}
