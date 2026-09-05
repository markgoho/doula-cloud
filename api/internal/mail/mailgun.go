package mail

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// mailgunAPIBase is Mailgun's default API host. MailgunSender's BaseURL
// field defaults to this; tests point it at an httptest server instead,
// per go.mod carrying no Mailgun SDK dependency (#219's build order --
// a hand-rolled form POST is small enough to stay httptest-coverable
// without one).
const mailgunAPIBase = "https://api.mailgun.net"

// MailgunSender is the real Sender, posting to Mailgun's HTTP API
// directly (no SDK dependency). Domain is the verified sending domain
// (docs/environment.md's Mailgun section, #218) -- MAILGUN_DOMAIN in
// every environment.
type MailgunSender struct {
	APIKey     string
	Domain     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewMailgunSender builds a MailgunSender pointed at Mailgun's real API.
func NewMailgunSender(apiKey, domain string) *MailgunSender {
	return &MailgunSender{
		APIKey:     apiKey,
		Domain:     domain,
		BaseURL:    mailgunAPIBase,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts msg to Mailgun's /messages endpoint. Tracking (open/click)
// is explicitly disabled, and stays that way: ADR-0030 (#734) rules out
// third-party open and click tracking in every Doula Cloud email, in
// either voice. mg.doula.cloud's domain settings already have all three
// inactive; these three flags are the second guard, so a change in
// Mailgun's dashboard cannot silently turn tracking on.
func (m *MailgunSender) Send(ctx context.Context, msg Message) error {
	form := url.Values{}
	form.Set("from", msg.From)
	form.Set("to", msg.To)
	form.Set("subject", msg.Subject)
	form.Set("text", msg.Text)
	form.Set("h:Reply-To", msg.ReplyTo)
	form.Set("o:tracking", "no")
	form.Set("o:tracking-clicks", "no")
	form.Set("o:tracking-opens", "no")

	endpoint := strings.TrimSuffix(m.BaseURL, "/") + "/v3/" + m.Domain + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		// coverage:ignore reason: only fails on a malformed URL, unreachable with a well-formed BaseURL/Domain
		return fmt.Errorf("mail: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("api", m.APIKey)

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail: mailgun status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// DeleteBounce takes address off Mailgun's own bounce list, which is a
// separate fact from Doula Cloud's own email_suppressions row: ADR-0029
// is explicit that clearing the local row alone is a lie, because
// Mailgun keeps refusing the send server-side until this call runs too.
//
// A 404 is success, not a failure. Mailgun answers "Address not found in
// bounces table" for an address it has never listed (verified live
// against mg.doula.cloud on #744), and Doula Cloud records a suppression
// from the webhook's own permanent_fail event -- which can arrive
// without the address ever reaching Mailgun's list, and which a Staff
// member may already have cleared there by hand. Treating that as an
// error would leave such a row permanently unclearable. What the caller
// needs is the goal state, "this address is not on Mailgun's list", and
// a 404 already satisfies it.
func (m *MailgunSender) DeleteBounce(ctx context.Context, address string) error {
	endpoint := strings.TrimSuffix(m.BaseURL, "/") + "/v3/" + m.Domain + "/bounces/" + url.PathEscape(address)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		// coverage:ignore reason: only fails on a malformed URL, unreachable with a well-formed BaseURL/Domain
		return fmt.Errorf("mail: build request: %w", err)
	}
	req.SetBasicAuth("api", m.APIKey)

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("mail: delete bounce: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mail: mailgun status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
