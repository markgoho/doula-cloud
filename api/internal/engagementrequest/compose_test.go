package engagementrequest

import (
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/outbox"
)

const (
	testComposeAppBaseURL = "https://app.example.test"
	testOwnerAddress      = "owner@example.com"
)

func runCompose(r pendingRow) (to, subject, text string, err error) {
	c := compose(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	return c(nil, nil, r, time.Now())
}

func TestCompose_PendingRequestSendsWithDashboardLink(t *testing.T) {
	r := pendingRow{practiceID: "practice-1", state: statePending, address: testOwnerAddress}
	to, subject, text, err := runCompose(r)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != testOwnerAddress {
		t.Fatalf("to = %q, want the recipient's address", to)
	}
	if subject != requestSubject {
		t.Fatalf("subject = %q, want %q", subject, requestSubject)
	}
	wantLink := testComposeAppBaseURL + "/practices/practice-1/clients"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

// A Request decided or withdrawn through some other path before this row
// was sent is never mailed late.
func TestCompose_DecidedRequestSkipsSend(t *testing.T) {
	r := pendingRow{practiceID: "practice-1", state: "withdrawn", address: testOwnerAddress}
	_, _, _, err := runCompose(r)
	if !errors.Is(err, outbox.ErrAlreadyDone) {
		t.Fatalf("err = %v, want outbox.ErrAlreadyDone", err)
	}
}
