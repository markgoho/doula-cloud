package staffauth

import (
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// Call is what Middleware resolved for one request, handed to a handler
// built with Handler as typed values: the request-scoped tx, the Practice,
// the acting Staff member and her Reader, and the path ids the handler's
// options declared, already parsed and gated.
type Call struct {
	Tx         *sql.Tx
	PracticeID string
	// StaffID is the acting Staff member, the actor an Activity entry or an
	// attached_by column names.
	StaffID string
	Reader  Reader
	// EngagementID is set by EngagementID, Engagement or EngagementReach,
	// and ClientID by Client; both are empty otherwise.
	EngagementID string
	ClientID     string
	ids          map[string]string
}

// ID returns the path id PathID declared under wildcard. It panics on a
// wildcard the handler never declared: that is a wiring mistake the first
// test of the handler meets, not a request a caller can cause.
func (c Call) ID(wildcard string) string {
	id, ok := c.ids[wildcard]
	if !ok {
		panic(fmt.Sprintf("staffauth: Call.ID(%q): no PathID declared for this wildcard", wildcard))
	}
	return id
}

// Option declares one path id a handler needs, or one reach gate it sits
// behind. It runs after Middleware and before the handler, in the order
// given, writes the 400, 404 or 500 itself when it refuses, and reports
// whether the handler may run.
type Option func(w http.ResponseWriter, r *http.Request, c *Call) (ok bool)

// Handler adapts fn to an http.Handler for a route mounted behind
// Middleware. It reads the tx, Practice, Staff id and Reader Middleware
// placed on context once -- answering 500 itself if Middleware never ran,
// so no handler carries that branch -- applies opts, then calls fn.
func Handler(fn func(w http.ResponseWriter, r *http.Request, c Call), opts ...Option) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tx, hasTx := Tx(ctx)
		practiceID, hasPractice := PracticeID(ctx)
		staffID, hasStaff := StaffID(ctx)
		reader, hasReader := ReaderFrom(ctx)
		if !hasTx || !hasPractice || !hasStaff || !hasReader {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		c := Call{Tx: tx, PracticeID: practiceID, StaffID: staffID, Reader: reader}
		for _, opt := range opts {
			if !opt(w, r, &c) {
				return
			}
		}
		fn(w, r, c)
	})
}

// PathID declares the path wildcard as a UUID, naming it label in the 400
// "invalid <label> id" ParseUUID writes, and exposes it as Call.ID(wildcard).
func PathID(label, wildcard string) Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		id := r.PathValue(wildcard)
		if !ParseUUID(w, label, id) {
			return false
		}
		c.setID(wildcard, id)
		return true
	}
}

// PathValue declares the path wildcard as a value that is not a UUID, such
// as an enum member. When validate refuses it, a 400 carrying refusal(value)
// is written; otherwise it is exposed as Call.ID(wildcard).
func PathValue(wildcard string, validate func(string) bool, refusal func(string) string) Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		value := r.PathValue(wildcard)
		if !validate(value) {
			apierr.WriteError(w, refusal(value), http.StatusBadRequest)
			return false
		}
		c.setID(wildcard, value)
		return true
	}
}

// NotAmbientContractor refuses a plain contractor Doula with the 403
// MsgContractorMoneyRefused: ADR-0008's money row as amended by #282.
func NotAmbientContractor() Option {
	return func(w http.ResponseWriter, _ *http.Request, c *Call) bool {
		if c.Reader.IsAmbientContractor() {
			apierr.WriteError(w, MsgContractorMoneyRefused, http.StatusForbidden)
			return false
		}
		return true
	}
}

func (c *Call) setID(wildcard, id string) {
	if c.ids == nil {
		c.ids = map[string]string{}
	}
	c.ids[wildcard] = id
}

// Confirmed refuses a request that does not carry the X-Confirmed header
// (RequireConfirmed): the backstop for an act the frontend asks a person
// to confirm first.
func Confirmed() Option {
	return func(w http.ResponseWriter, r *http.Request, _ *Call) bool {
		return RequireConfirmed(w, r)
	}
}

// EngagementID declares the {engagementId} wildcard as a UUID and sets
// Call.EngagementID. It checks nothing else: a handler that decides for
// itself whether the Engagement is reachable, and what to say when it is
// not, declares this and no more.
func EngagementID() Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		id := r.PathValue("engagementId")
		if !ParseUUID(w, "engagement", id) {
			return false
		}
		c.EngagementID = id
		return true
	}
}

// Engagement is EngagementID plus a 404 when the Engagement is not at the
// caller's Practice.
func Engagement() Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		return EngagementID()(w, r, c) &&
			requireEngagementAtPractice(w, r, c.Tx, c.EngagementID, c.PracticeID)
	}
}

// EngagementReach is Engagement narrowed by ADR-0008's attachment rule: a
// contractor Doula who is not attached gets a 404 carrying notFound.
func EngagementReach(notFound string) Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		return Engagement()(w, r, c) &&
			requireEngagementAccess(w, r, c.Tx, c.Reader, c.EngagementID, notFound)
	}
}

// Client declares the {clientId} wildcard as a UUID and gates it: a 404
// MsgClientNotFound when the caller's Reader may not reach that Client. It
// sets Call.ClientID.
func Client() Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		id := r.PathValue("clientId")
		if !ParseUUID(w, "client", id) {
			return false
		}
		if !RequireClientAccess(w, r, c.Tx, c.Reader, id) {
			return false
		}
		c.ClientID = id
		return true
	}
}
