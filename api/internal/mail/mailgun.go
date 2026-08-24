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
// is explicitly disabled: #218 never installed the tracking CNAME on
// mg.doula.cloud, and whether tracking is worth its privacy cost is
// still open on map #213's "Not yet specified" -- a Practice Notification
// must not default to on for a question nobody has decided.
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
