// Package mail is the seam over Notification email delivery (#219, map
// #213) that portalinvite's outbox worker sends through instead of
// calling Mailgun directly, mirroring push.Pusher's shape: a real,
// Mailgun-backed implementation and a FakeSender injected by tests, per
// docs/testing.md's "no real vendor reachable from api/ tests" rule.
package mail

import "context"

// Message is one outbound Notification email. Per ADR-0009's content
// rule (no Client name, no Engagement detail, nothing identifying) and
// ADR-0011's shared sending identity, callers build Subject and Text from
// fixed, content-free copy -- never from Client or Engagement data.
type Message struct {
	To      string
	From    string
	ReplyTo string
	Subject string
	Text    string
}

// Sender delivers msg. An error means the caller's outbox row should
// retry per ADR-0010's backoff schedule, not that the message is known
// undelivered -- Send may fail after Mailgun has already accepted it.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
