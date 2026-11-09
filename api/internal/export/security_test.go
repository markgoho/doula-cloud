package export_test

import (
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// TestExportPackageNeverImportsTheDecryptionCapability is the structural
// half of the sealed-diff acceptance criterion: it is not enough that
// today's output happens to be ciphertext (TestHandler_
// ErasedClientExportsRedactedWithSealedDiff proves that behaviorally) --
// the package must have no way to decrypt one at all. clientkey.Open is
// the only function anywhere that can turn a sealed diff back into
// plaintext. Parsing every non-test file's import declarations (not
// grepping their text -- a doc comment is allowed to name the package it
// deliberately doesn't import, the way entities.go's own activity.csv
// comment does) and asserting clientkey is never among them is a
// structural proof that it is unreachable from here, stronger than any
// output assertion a future change to this package could still satisfy
// while quietly adding a decrypt path.
func TestExportPackageNeverImportsTheDecryptionCapability(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	checked := 0
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		checked++
		f, err := parser.ParseFile(fset, filepath.Join(".", e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, "clientkey") {
				t.Fatalf("%s imports %q -- the export path must never be able to reach a Client data key", e.Name(), path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test .go files found to check -- this test would pass vacuously")
	}
}

// TestHandler_ErasedClientExportsRedactedWithSealedDiff is the erasure
// acceptance criterion: an erased Client appears in her redacted state,
// with the fact and date of erasure, and her sealed Activity diffs
// appear unreadable rather than decrypted.
func TestHandler_ErasedClientExportsRedactedWithSealedDiff(t *testing.T) {
	e := newEnv(t, fixture.WithClient(fixture.ClientName("Ada Lovelace"), fixture.ClientEmail("ada@example.com")))
	clientURL := e.PracticeURL("/clients/" + e.Engagement.Client.ID)

	// A real edit through the endpoint, so its Activity row is genuinely
	// sealed under her key, holding her phone number in the diff.
	reqtest.ExpectStatus(t, http.MethodPut, clientURL, http.StatusOK, e.SignedIn, reqtest.JSON(client.EditRequest{
		GivenName: "Ada", FamilyName: "Lovelace", Email: "ada@example.com", Phone: "585-555-0199", Override: true,
	}))
	reqtest.ExpectStatus(t, http.MethodPost, clientURL+"/erasure", http.StatusOK, e.SignedIn)

	files := e.archive()

	clientCSV := files["client.csv"]
	if !strings.Contains(clientCSV, "Erased Client") {
		t.Fatalf("client.csv does not show the erased placeholder name: %s", clientCSV)
	}
	if strings.Contains(clientCSV, "Ada") || strings.Contains(clientCSV, "Lovelace") || strings.Contains(clientCSV, "585-555") {
		t.Fatalf("client.csv still carries erased personal data: %s", clientCSV)
	}
	if onlyRow(t, "client.csv", parseCSV(t, clientCSV)).get("erased_at") == "" {
		t.Fatalf("client.csv erased_at is blank, want the erasure timestamp")
	}

	activityCSV := files["activity.csv"]
	if strings.Contains(activityCSV, "585-555-0199") {
		t.Fatalf("activity.csv leaked the plaintext phone number from a sealed diff: %s", activityCSV)
	}
	if !strings.Contains(activityCSV, `"enc"`) {
		t.Fatalf("activity.csv has no sealed ({\"v\":...,\"enc\":...}) diff at all -- the edit's own row should still be there, still ciphertext: %s", activityCSV)
	}
}

// TestHandler_ManyClientsAndEngagementsStreamsRatherThanBuffers proves
// the "streamed, not assembled in memory" acceptance criterion against a
// real dataset, not a two-row fixture: a Practice with enough Clients and
// Engagements that a buffered implementation would still work, but only
// a streamed one would flush its ResponseWriter more than once.
func TestHandler_ManyClientsAndEngagementsStreamsRatherThanBuffers(t *testing.T) {
	e := newEnv(t)
	const count = 60
	for range count {
		e.AnotherEngagement()
	}

	files := e.archive()
	if rows := parseCSV(t, files["client.csv"]); len(rows) != count+1 {
		t.Fatalf("client.csv rows = %d, want %d (header + %d Clients)", len(rows), count+1, count)
	}

	// The row count above is consistent with either a streamed or a
	// buffered-then-written implementation -- it says nothing about
	// memory. What distinguishes them is that writeEntity flushes the
	// ResponseWriter once per entity as it goes, rather than once at the
	// very end: call the mux directly (no real TCP hop) with a
	// Flush-counting ResponseWriter and require more flushes than one
	// entity alone could produce.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/practices/"+e.Practice.ID+"/export", nil)
	authntest.AddSessionCookie(req, fixture.Session(t, e.DB, e.Staff))
	fw := &flushCountingWriter{ResponseRecorder: httptest.NewRecorder()}
	e.Srv.Config.Handler.ServeHTTP(fw, req)
	if fw.Code != http.StatusOK {
		t.Fatalf("direct-mux status = %d, want %d", fw.Code, http.StatusOK)
	}
	if fw.flushes < len(files) {
		t.Fatalf("flush count = %d, want at least %d (one per file in the archive) -- the archive was buffered rather than streamed", fw.flushes, len(files))
	}
}

// flushCountingWriter counts Flush calls so a test can tell a streamed
// response (many small flushes) apart from a buffered one (a single
// flush, or none, at the end).
type flushCountingWriter struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *flushCountingWriter) Flush() {
	f.flushes++
	f.ResponseRecorder.Flush()
}

// TestHandler_RefusesAnUnknownOrMalformedPractice covers the two shapes
// staffauth.Middleware itself already refuses before this handler ever
// runs: a syntactically invalid id (400) and a well-formed id naming no
// Practice this caller belongs to (403, indistinguishable from a real
// Practice she isn't a member of -- ADR-0008 never leaks which).
func TestHandler_RefusesAnUnknownOrMalformedPractice(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.URL("/api/practices/not-a-uuid/export"), http.StatusBadRequest, e.SignedIn)
	reqtest.ExpectStatus(t, http.MethodGet, e.URL("/api/practices/00000000-0000-0000-0000-000000000000/export"), http.StatusForbidden, e.SignedIn)
}
