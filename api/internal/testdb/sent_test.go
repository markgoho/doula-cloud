package testdb_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

func TestRequireSent_ReturnTheOneMessage(t *testing.T) {
	sender := &mail.FakeSender{}
	if err := sender.Send(t.Context(), mail.Message{To: "person@example.com", Text: "hello"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	if got := testdb.RequireSentOnceTo(t, sender, "person@example.com"); got.Text != "hello" {
		t.Fatalf("message = %+v, want the one sent", got)
	}
	if got := testdb.RequireSentAndTokenCleared(t, "sent", sql.NullString{}, sender, "person@example.com"); got.To != "person@example.com" {
		t.Fatalf("message = %+v, want the one sent", got)
	}
}
