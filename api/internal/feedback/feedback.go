// Package feedback stores a piece of Feedback (00118_feedback.sql,
// #1523): the row every Staff member and every Portal Client can send
// from inside the product. It owns the shape both send routes share --
// the client-sent DTO, kind and length validation, and the insert -- and
// nothing else. Opening a private GitHub issue for a row is #1524's
// outbox; the founder read page is #1499; neither is built here.
package feedback

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxTextRunes is #1523's own limit: free text over 5,000 characters is
// refused.
const MaxTextRunes = 5000

// Kind is one of the three radio choices #1498's resolution comment (Q5)
// settled: "Something is not working", "An idea or a request", "Something
// else". The three string values are feedback_kind's own enum labels
// (00118), so a validated Kind inserts with no translation step.
const (
	KindNotWorking    = "not_working"
	KindIdeaOrRequest = "idea_or_request"
	KindSomethingElse = "something_else"
)

// validKinds is Kind's closed set, checked before a value ever reaches
// Postgres so a caller who sent something else meets a 400 naming the
// field rather than a 500 off the enum's own CHECK.
var validKinds = map[string]bool{
	KindNotWorking:    true,
	KindIdeaOrRequest: true,
	KindSomethingElse: true,
}

// ValidKind reports whether kind is one of the three the product asks
// for -- true for a missing or unrecognized value alike, both of which
// the caller refuses the same way (MsgKindRequired).
func ValidKind(kind string) bool {
	return validKinds[kind]
}

// ValidText reports whether text is at or under MaxTextRunes. An empty
// string is valid -- #1523's own AC: "Empty free text is allowed."
// Counted in runes, not bytes: a caller typing 5,000 characters in any
// script should meet the same limit, not one that varies with how many
// bytes her alphabet costs UTF-8.
func ValidText(text string) bool {
	return utf8.RuneCountInString(text) <= MaxTextRunes
}

// Page is the screen a piece of Feedback was sent from, named the way
// SvelteKit's own $page store carries it -- the client sends this
// straight through, no client-side reshaping.
type Page struct {
	// URL is the full address with its ids, e.g.
	// "/clients/<uuid>/invoices/<uuid>" -- database-only per #1501 Q1,
	// see Row.PageURL.
	URL   string `json:"url"`
	Route Route  `json:"route"`
}

// Route names the page's route pattern rather than its resolved address.
type Route struct {
	// ID is SvelteKit's own route id, route groups and all, e.g.
	// "/(app)/practices/[practiceId]/clients/[clientId]" -- what crosses
	// to the private GitHub issue (#1501 Q1), unlike Page.URL.
	ID string `json:"id"`
}

// ClientInput is the part of a send request the client fills in, shared
// by both the Staff and the Portal DTO: kind, the free text, and the
// context #1498's resolution comment (Q4) says the form shows the sender
// it will attach. Neither the sender, her role(s), sent_at, nor the
// browser rides here -- the BFF sets every one of those itself (#1523's
// own line: "never taken from the body").
type ClientInput struct {
	Kind        string `json:"kind"`
	Text        string `json:"text"`
	Page        Page   `json:"page"`
	AppBuild    string `json:"appBuild"`
	ScreenWidth int    `json:"screenWidth"`
}

// Row is one piece of Feedback ready to insert: ClientInput's fields plus
// everything the BFF resolved -- the id it minted, the sender, the
// context that depends on who sent it, when, and the browser parsed from
// User-Agent. Exactly one of StaffID or PortalAccount is set, matching
// feedback_sender's own CHECK; PracticeID/Roles ride together for a
// Staff sender, PracticeID/ClientID ride together for a Portal sender,
// and RolesLiteral is nil whenever Roles carries nothing to write.
type Row struct {
	ID            string
	Kind          string
	Text          string
	PageURL       string
	RouteID       string
	AppBuild      string
	ScreenWidth   int
	Browser       string
	StaffID       *string
	PortalAccount *string
	PracticeID    *string
	ClientID      *string
	// RolesLiteral is a Postgres practice_role[] literal, e.g.
	// "{owner,admin}", or nil to write SQL NULL. BuildRolesLiteral turns a
	// []string of role names into this shape.
	RolesLiteral *string
	SentAt       time.Time
}

// BuildRolesLiteral turns roles into the "{a,b}" literal Row.RolesLiteral
// carries, the same shape membership.go's parseMembership already builds
// for practice_memberships.roles. A nil roles (no {practiceId} in the
// request) stays nil -- there is nothing to write, not an empty array.
func BuildRolesLiteral(roles []string) *string {
	if roles == nil {
		return nil
	}
	literal := "{" + strings.Join(roles, ",") + "}"
	return &literal
}

// Insert writes row. The caller mints row.ID itself (uuid.NewString()),
// so this never needs a RETURNING clause -- feedback carries no SELECT
// grant for app_runtime to read one back through (00118).
func Insert(ctx context.Context, tx *sql.Tx, row Row) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO feedback
		    (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     staff_id, portal_account, practice_id, client_id, roles, sent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::practice_role[], $14)`,
		row.ID, row.Kind, row.Text, row.PageURL, row.RouteID, row.AppBuild, row.ScreenWidth, row.Browser,
		row.StaffID, row.PortalAccount, row.PracticeID, row.ClientID, row.RolesLiteral, row.SentAt,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("feedback: insert: %w", err)
	}
	return nil
}
