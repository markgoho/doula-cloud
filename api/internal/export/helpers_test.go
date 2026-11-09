package export_test

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"io"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/export"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// env is what a test in this package starts from: a Practice, an Owner at
// the screen, and export.Mount mounted beside client.Mount -- the latter only
// so TestHandler_ErasedClientExportsRedactedWithSealedDiff can put a real
// edit through EditHandler and get back a genuinely sealed diff. A test says
// only what it changes, with fixture.SignedInAs and fixture.WithClient.
type env struct {
	*fixture.Scene
}

func newEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		client.Mount(g, ir, tasknudge.NoOpEnqueuer{})
		export.Mount(g)
	}
	return &env{Scene: fixture.NewScene(t, mount, append([]fixture.SceneOption{fixture.SignedInAs(fixture.Owner())}, opts...)...)}
}

// exportURL is the Practice's export route.
func (e *env) exportURL() string {
	return e.PracticeURL("/export")
}

// archive downloads the Practice's export as the Staff member at the screen
// and returns a lookup from filename to its contents.
func (e *env) archive() map[string]string {
	e.T.Helper()
	return readZIP(e.T, reqtest.ExpectBody(e.T, http.MethodGet, e.exportURL(), http.StatusOK, e.SignedIn))
}

// csvFile downloads the export and parses one of its files.
func (e *env) csvFile(name string) [][]string {
	e.T.Helper()
	return parseCSV(e.T, e.archive()[name])
}

// readZIP returns a lookup from filename to contents for a ZIP body (a
// test's own archive is small).
func readZIP(t *testing.T, body []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		files[f.Name] = string(content)
	}
	return files
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func parseCSV(t *testing.T, s string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

func indexOf(t *testing.T, header []string, col string) int {
	t.Helper()
	for i, h := range header {
		if h == col {
			return i
		}
	}
	t.Fatalf("header %v has no column %q", header, col)
	return -1
}

// csvRow is one row of an exported file, read by column name.
type csvRow struct {
	t      *testing.T
	header []string
	cells  []string
}

// get is the cell under col, failing the test when the file has no such column.
func (r csvRow) get(col string) string {
	r.t.Helper()
	return r.cells[indexOf(r.t, r.header, col)]
}

// onlyRow requires a file of a header and one row, and returns the row.
func onlyRow(t *testing.T, name string, rows [][]string) csvRow {
	t.Helper()
	if len(rows) != 2 {
		t.Fatalf("%s rows = %d, want header + 1", name, len(rows))
	}
	return csvRow{t: t, header: rows[0], cells: rows[1]}
}
