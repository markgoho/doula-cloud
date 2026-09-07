package export

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// writeReadme is the plain-text file issue #288 asks every archive to
// carry: the Practice, who ran the export, when, and what each other
// file holds. Written first, so it is the first thing a person sees
// when she opens the archive in a file manager that lists entries in
// the order they were written, and flushed like every entity that
// follows it (writeEntity's own doc comment), so the client starts
// receiving bytes before the first entity's query even runs.
func writeReadme(ctx context.Context, tx *sql.Tx, w http.ResponseWriter, zw *zip.Writer, practiceName, staffID string, now time.Time, entities []entity) error {
	var staffName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM staff WHERE id = $1`, staffID).Scan(&staffName); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("read staff name: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s -- Doula Cloud data export\n\n", practiceName)
	fmt.Fprintf(&b, "Run by %s, %s.\n\n", staffName, now.Format("2006-01-02T15:04:05Z"))
	b.WriteString("Every file is a UTF-8 CSV with a header row. Ids are included so the files join to each other.\n")
	b.WriteString("A blank cell means the column was empty (NULL), not that it was left out.\n")
	b.WriteString("An Activity row whose diff reads as {\"v\":1,\"enc\":\"...\"} is sealed under a Client's own key,\n")
	b.WriteString("shredded on her erasure (ADR-0027); it is never decrypted here and never will be.\n\n")
	b.WriteString("Files in this archive:\n")
	for _, e := range entities {
		fmt.Fprintf(&b, "  %-30s %s\n", e.file, e.description)
	}

	fw, err := zw.Create("README.txt")
	if err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("create readme: %w", err)
	}
	if _, err := fw.Write([]byte(b.String())); err != nil {
		// coverage:ignore reason: response streaming failure, not exercised by unit tests
		return fmt.Errorf("write readme: %w", err)
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
