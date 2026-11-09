package mailsuppress_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// env is what a handler test in this package starts from: a Practice with
// this package's surface mounted over a fake Mailgun, and one Staff member
// at the screen, an Owner unless a test says otherwise.
type env struct {
	*fixture.Scene
	clearer *mail.FakeSender
}

// newEnv mounts this package's whole surface through mailsuppress.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router.
func newEnv(t *testing.T, clearer *mail.FakeSender, staff ...fixture.StaffOption) *env {
	t.Helper()
	if len(staff) == 0 {
		staff = []fixture.StaffOption{fixture.Owner()}
	}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router) { mailsuppress.Mount(g, ir, clearer) }
	return &env{Scene: fixture.NewScene(t, fixture.Routes(mount), fixture.SignedInAs(staff...)), clearer: clearer}
}

func (e *env) listURL() string { return e.PracticeURL("/email-suppressions") }

// clearBody is the request body that asks to clear address.
func clearBody(address string) reqtest.Option {
	return reqtest.JSON(map[string]string{"address": address})
}

// clear asks to clear address at the Practice and requires want.
func (e *env) clear(address string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPost, e.listURL()+"/clear", want, e.SignedIn, clearBody(address))
}

// clientAt gives practiceID a Client with address, so the address counts
// as one that Practice is responsible for.
func clientAt(t *testing.T, db *testdb.DB, p fixture.PracticeRecord, address string) {
	t.Helper()
	fixture.Client(t, db, p, fixture.ClientEmail(address))
}

// suppress records a suppression of cause for address.
func suppress(t *testing.T, db *testdb.DB, address, cause string) {
	t.Helper()
	if err := mailsuppress.Record(t.Context(), db.App, address, cause, "evt-1"); err != nil {
		t.Fatalf("Record %s: %v", cause, err)
	}
}

// requireActive fails unless address's suppression is standing, or is
// not, as want says.
func requireActive(t *testing.T, db *testdb.DB, address string, want bool) {
	t.Helper()
	got, err := mailsuppress.Active(t.Context(), db.Admin, address)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if got != want {
		t.Fatalf("Active(%s) = %v, want %v", address, got, want)
	}
}
