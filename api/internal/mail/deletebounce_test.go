package mail_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/mail"
)

var errFakeBoom = errors.New("mailgun unavailable")

func TestMailgunSender_DeleteBounce(t *testing.T) {
	var gotPath, gotMethod string
	var gotUser, gotPass string
	var gotAuthOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotUser, gotPass, gotAuthOK = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = srv.URL

	// A '+' in the local part is legal and common; it must reach Mailgun
	// as part of the address rather than as a path separator or a space.
	if err := sender.DeleteBounce(t.Context(), "someone+tag@example.test"); err != nil {
		t.Fatalf("DeleteBounce: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
	if want := "/v3/mg.doula.cloud/bounces/someone+tag@example.test"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if !gotAuthOK || gotUser != "api" || gotPass != "key-test" {
		t.Fatalf("basic auth = %q/%q (ok=%v), want api/key-test", gotUser, gotPass, gotAuthOK)
	}
}

// Verified live against mg.doula.cloud on #744: Mailgun answers 404
// "Address not found in bounces table" for an address it never listed.
// Doula Cloud records suppressions from the webhook's own permanent_fail
// event, which can outlive or precede Mailgun's list entry, so a 404
// means the goal state already holds -- not that the clear failed.
func TestMailgunSender_DeleteBounce_404IsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Address not found in bounces table"}`))
	}))
	defer srv.Close()

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = srv.URL

	if err := sender.DeleteBounce(t.Context(), testAddr); err != nil {
		t.Fatalf("DeleteBounce on an unlisted address = %v, want nil", err)
	}
}

func TestMailgunSender_DeleteBounce_ServerErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("mailgun exploded"))
	}))
	defer srv.Close()

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = srv.URL

	err := sender.DeleteBounce(t.Context(), testAddr)
	if err == nil {
		t.Fatal("a 500 from Mailgun read as a successful clear")
	}
	if !strings.Contains(err.Error(), "mailgun exploded") {
		t.Fatalf("error = %v, want it to carry Mailgun's own message", err)
	}
}

func TestMailgunSender_DeleteBounce_TransportFailureSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening any more

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = url

	if err := sender.DeleteBounce(t.Context(), testAddr); err == nil {
		t.Fatal("an unreachable Mailgun read as a successful clear")
	}
}

func TestFakeSender_DeleteBounce(t *testing.T) {
	f := &mail.FakeSender{}
	if err := f.DeleteBounce(t.Context(), testAddr); err != nil {
		t.Fatalf("DeleteBounce: %v", err)
	}
	if got := f.Deleted(); len(got) != 1 || got[0] != testAddr {
		t.Fatalf("Deleted() = %v, want [%s]", got, testAddr)
	}

	f.DeleteErr = errFakeBoom
	if err := f.DeleteBounce(t.Context(), testAddr); !errors.Is(err, errFakeBoom) {
		t.Fatalf("DeleteBounce error = %v, want errFakeBoom", err)
	}
	if len(f.Deleted()) != 1 {
		t.Fatal("a failed DeleteBounce recorded the address anyway")
	}
}
