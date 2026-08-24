package mail_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/mail"
)

func TestFakeSender_RecordsSentMessages(t *testing.T) {
	f := &mail.FakeSender{}
	msg := mail.Message{To: testAddr, From: "c@d.test", Subject: "s", Text: "t"}

	if err := f.Send(t.Context(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := f.Sent()
	if len(sent) != 1 || sent[0] != msg {
		t.Fatalf("Sent() = %+v, want [%+v]", sent, msg)
	}
}

func TestFakeSender_ReturnsConfiguredError(t *testing.T) {
	wantErr := errors.New("mailgun unavailable")
	f := &mail.FakeSender{Err: wantErr}

	if err := f.Send(t.Context(), mail.Message{}); !errors.Is(err, wantErr) {
		t.Fatalf("Send err = %v, want %v", err, wantErr)
	}
	if len(f.Sent()) != 0 {
		t.Fatalf("expected no recorded messages when Err is set")
	}
}
