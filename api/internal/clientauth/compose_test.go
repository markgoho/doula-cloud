package clientauth

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/outbox"
)

const (
	testComposeAppBaseURL = "https://app.example.test"
	testSignInAddress     = "her@example.com"
	testNewAddress        = "the-new-one@example.com"
)

func TestComposeMagicLink_SendsWithSignInLink(t *testing.T) {
	c := composeMagicLink(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	r := magicLinkRow{token: sql.NullString{String: "link-token", Valid: true}, signInAddress: testSignInAddress}

	to, _, text, err := c(nil, nil, r, time.Now())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != testSignInAddress {
		t.Fatalf("to = %q, want the account's sign-in address", to)
	}
	wantLink := testComposeAppBaseURL + "/portal/sign-in?token=link-token"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

func TestComposeMagicLink_NoTokenDeadLetters(t *testing.T) {
	c := composeMagicLink(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	r := magicLinkRow{signInAddress: testSignInAddress}

	_, _, _, err := c(nil, nil, r, time.Now())
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}

// The recipient is the row's own to_address, never the one
// portal_accounts holds, and the body never names the old address.
func TestComposeAddressChange_SendsToTheRowsOwnAddress(t *testing.T) {
	c := composeAddressChange(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	r := addressChangeRow{token: sql.NullString{String: "confirm-token", Valid: true}, toAddress: testNewAddress}

	to, _, text, err := c(nil, nil, r, time.Now())
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if to != testNewAddress {
		t.Fatalf("to = %q, want the row's own to_address", to)
	}
	wantLink := testComposeAppBaseURL + "/portal/confirm-sign-in-address?token=confirm-token"
	if !strings.Contains(text, wantLink) {
		t.Fatalf("text %q does not contain link %q", text, wantLink)
	}
}

func TestComposeAddressChange_NoTokenDeadLetters(t *testing.T) {
	c := composeAddressChange(outbox.Mailer{AppBaseURL: testComposeAppBaseURL})
	r := addressChangeRow{toAddress: testNewAddress}

	_, _, _, err := c(nil, nil, r, time.Now())
	var dl *outbox.DeadLetterError
	if !errors.As(err, &dl) {
		t.Fatalf("err = %v, want a *outbox.DeadLetterError", err)
	}
}
