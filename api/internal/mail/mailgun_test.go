package mail_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/mail"
)

// testAddr is this package's stand-in email address, reused across
// mailgun_test.go and fake_test.go rather than repeating the literal.
const testAddr = "a@b.test"

func TestMailgunSender_Send(t *testing.T) {
	var gotForm string
	var gotAuthUser, gotAuthPass string
	var gotAuthOK bool
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthUser, gotAuthPass, gotAuthOK = r.BasicAuth()
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotForm = r.Form.Encode()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = srv.URL

	err := sender.Send(t.Context(), mail.Message{
		To:      "client@example.test",
		From:    "Doula Cloud <notifications@mg.doula.cloud>",
		ReplyTo: "noreply@mg.doula.cloud",
		Subject: "You have something waiting",
		Text:    "View it: https://app.example.test/portal/accept-invite?token=abc",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotPath != "/v3/mg.doula.cloud/messages" {
		t.Fatalf("path = %q", gotPath)
	}
	if !gotAuthOK || gotAuthUser != "api" || gotAuthPass != "key-test" {
		t.Fatalf("basic auth = (%q, %q, %v), want (api, key-test, true)", gotAuthUser, gotAuthPass, gotAuthOK)
	}
	if gotForm == "" {
		t.Fatal("expected a non-empty form body")
	}
}

func TestMailgunSender_Send_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = srv.URL

	err := sender.Send(t.Context(), mail.Message{To: "client@example.test", From: testAddr, Subject: "s", Text: "t"})
	if err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestMailgunSender_Send_TransportError(t *testing.T) {
	sender := mail.NewMailgunSender("key-test", "mg.doula.cloud")
	sender.BaseURL = "http://127.0.0.1:0"

	if err := sender.Send(t.Context(), mail.Message{To: testAddr, From: testAddr, Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected an error when the transport fails")
	}
}
