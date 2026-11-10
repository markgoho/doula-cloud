package outbox

import "errors"

// Status is an outbox row's lifecycle state: the status column of every
// one of the twenty-one tables that share this machine. Each table keeps
// its own <table>_status enum (00169 weighed one shared type and kept
// these: moving to it means rebuilding every partial index and CHECK
// that names the old type, about thirty of them, for no new behavior),
// but every one of those enums holds exactly these labels, and
// portal_invite_outbox's alone also holds StatusBounced and
// StatusComplained.
type Status string

// The states. A row is born pending and leaves it exactly once, by
// TransitionSend, TransitionSkip or TransitionDeadLetter; TransitionRetry
// keeps it pending with a later due time. Only a sent portal invite moves
// again, when Mailgun reports what became of it.
const (
	// StatusPending is waiting to be acted on, due at next_attempt_at.
	StatusPending Status = "pending"
	// StatusSent was acted on: mailed, or its job done. sent_at says when.
	StatusSent Status = "sent"
	// StatusSkipped was resolved with nothing sent, because the worker or
	// the act that queued it found nothing left to do. skip_reason says
	// why and skipped_at when. Until 00169 such a row was marked sent,
	// and nothing but welcome_credits_outbox could tell the two apart.
	StatusSkipped Status = "skipped"
	// StatusDeadLettered will never be acted on; dead_letter_reason says
	// why (ADR-0062).
	StatusDeadLettered Status = "dead_lettered"
	// StatusBounced was sent, and the address then rejected it (#340).
	StatusBounced Status = "bounced"
	// StatusComplained was sent and arrived, and was marked as spam (#340).
	StatusComplained Status = "complained"
)

// SkipReason is the outbox_skip_reason a skipped row carries, one value
// set shared by every outbox (00169).
type SkipReason string

// The reasons a row resolves with nothing sent. The last four are
// welcome_credits_outbox's own (00133), kept under their names.
const (
	// SkipAlreadyResolved is a send-time recheck finding the row's own
	// event already settled another way: an invitation accepted, an Offer
	// no longer open, a coverage gap closed, Stripe connected.
	SkipAlreadyResolved SkipReason = "already_resolved"
	// SkipNoRecipients is a row with nobody left to tell: a Practice
	// with no current Owner, say.
	SkipNoRecipients SkipReason = "no_recipients"
	// SkipLoginDeleted is a row addressed to a person who deleted her own
	// login before it went out (#892).
	SkipLoginDeleted SkipReason = "login_deleted"
	// SkipPracticeRestored is a Practice deletion act for a Practice
	// that was restored before it came due (ADR-0031).
	SkipPracticeRestored SkipReason = "practice_restored"
	// SkipEngagementStarted and the three after it are the welcome
	// email's skips (#1381).
	SkipEngagementStarted SkipReason = "engagement_started"
	SkipFoundingGrant     SkipReason = "founding_grant"
	SkipPracticeDeleting  SkipReason = "practice_deleting"
	SkipOwnersSuppressed  SkipReason = "owners_suppressed"
)

// Transition is one named move of an outbox row: the one state it must
// be in beforehand and the one it leaves the row in. Every UPDATE of an
// outbox table's status column goes through Worker.apply with one of
// these, which writes the move's own columns, adds "AND status = From"
// to the WHERE clause, and refuses a statement that moved no row
// (ErrNotInState). lifecycle_test.go's guardrail fails the build on a
// status UPDATE written anywhere else.
type Transition struct {
	// Name is the move as a reader would say it.
	Name string
	// From is the one state the row must be in.
	From Status
	// To is the state the row is left in.
	To Status
}

// The declared moves.
var (
	// TransitionSend: the row was acted on. Stamps sent_at.
	TransitionSend = Transition{Name: "send", From: StatusPending, To: StatusSent}
	// TransitionSkip: the row resolved with nothing sent. Stamps
	// skipped_at and records skip_reason.
	TransitionSkip = Transition{Name: "skip", From: StatusPending, To: StatusSkipped}
	// TransitionRetry: an attempt failed and another is scheduled per
	// BackoffSchedule. The row stays pending.
	TransitionRetry = Transition{Name: "retry", From: StatusPending, To: StatusPending}
	// TransitionDeadLetter: no attempt will be made again. Records
	// dead_letter_reason.
	TransitionDeadLetter = Transition{Name: "dead-letter", From: StatusPending, To: StatusDeadLettered}
	// TransitionBounce: Mailgun reported a sent row's address rejected
	// it. portal_invite_outbox only.
	TransitionBounce = Transition{Name: "bounce", From: StatusSent, To: StatusBounced}
	// TransitionComplain: Mailgun reported a sent row marked as spam.
	// portal_invite_outbox only.
	TransitionComplain = Transition{Name: "complain", From: StatusSent, To: StatusComplained}
)

// Transitions is every declared move, walked by lifecycle_test.go.
var Transitions = []Transition{
	TransitionSend, TransitionSkip, TransitionRetry, TransitionDeadLetter, TransitionBounce, TransitionComplain,
}

// LeavesPending reports whether t is a row's one exit from pending. On
// that exit, and only then, apply nulls the Worker's ClearOnTerminal
// columns: a plaintext token or access code is held for exactly as long
// as the row could still mail it (00061's promise, made for every
// table that holds one), whichever way the row stops being pending.
// 00169's <table>_secret_cleared CHECKs refuse a row that keeps one.
func (t Transition) LeavesPending() bool {
	return t.From == StatusPending && t.To != StatusPending
}

// ErrNotInState is what a Worker method returns when its UPDATE matched
// no row in the transition's From state. Every caller holds the row's
// lock already -- a claim's FOR UPDATE SKIP LOCKED, or a LockPending --
// so this is a row some other path moved first, or an id that is not a
// row of this table: never a case to treat as done. It rolls the caller's
// transaction back rather than letting it commit a send that its row
// does not record.
var ErrNotInState = errors.New("outbox: row is not in the state the transition requires")

// The claim predicate every outbox worker selects its due rows with, once.
// A claim query is a constant each package writes for its own row shape
// (HandlePending's doc), and it spells the predicate by naming one of
// these rather than restating it: lifecycle_test.go fails on the literal
// anywhere outside this package.
//
// The comparison is to Postgres's now(), not the Worker's clock, for the
// reason HandlePending's doc gives.
const (
	// ClaimDue is the predicate over a claim that does not alias its
	// outbox table.
	ClaimDue = "status = 'pending' AND next_attempt_at <= now()"
	// ClaimDueAliasO is the same predicate over a claim that joins and
	// aliases its outbox table as o.
	ClaimDueAliasO = "o.status = 'pending' AND o.next_attempt_at <= now()"
)

// SiteBuildStatus is site_build_outbox's own lifecycle state. It is a
// separate machine, declared here beside the shared one so the two can
// be read together, because its unit of work is different: one
// repository_dispatch for every pending row at once, not one send per
// row. So its rows have no next_attempt_at and no per-row backoff, it
// retries on the drain's cadence up to sitebuild.MaxAttempts (ten, not
// len(BackoffSchedule)), it has nothing to skip, and its success is a
// dispatch (dispatched_at), not a send. sitebuild/outbox.go writes it with
// set-based UPDATEs over every pending row, the one status writer outside
// this package that lifecycle_test.go's guardrail names and allows.
type SiteBuildStatus string

// site_build_outbox_status's labels (00049).
const (
	SiteBuildPending      SiteBuildStatus = "pending"
	SiteBuildDispatched   SiteBuildStatus = "dispatched"
	SiteBuildDeadLettered SiteBuildStatus = "dead_lettered"
)
