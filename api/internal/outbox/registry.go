package outbox

import (
	"database/sql"
	"net/http"
	"sort"

	"doula-cloud/api/internal/tasknudge"
)

// NotificationDoor is the Postgres session variable that licenses a mail
// kind's own staff/practice_memberships (or equivalent) SELECT policies
// for reading recipients at send time. Each kind's own migration adds the
// policy; ProcessHandler sets the variable every one of them checks.
//
// Written out on each Registration rather than defaulted, because the one
// kind that needs no door at all -- #443's site rebuild, whose outbox is
// not under RLS -- would otherwise be indistinguishable from a kind whose
// author simply forgot to say. An empty Door opens none.
const NotificationDoor = "app.notification_worker_trusted"

// Registration is everything that distinguishes one outbox from another
// at the route table: where it is mounted, which Postgres door its worker
// needs, whether ADR-0013 nudges it, and the worker itself.
//
// It exists because an outbox used to restate its own name in seven
// places -- a Worker, a pass-through ProcessOutboxHandler in its own
// package, a Deps field, a struct literal in main, a mux.Handle call, a
// tasknudge.OutboxType, and an entry in tasknudge's endpointPath map --
// of which only the first carried any behavior. The other six agreed with
// each other by hand, and Path in particular was written twice, so a
// rename could leave the nudge pointing at an endpoint the mux no longer
// served.
//
// Deliberately not called a Kind: CONTEXT.md gives "kind" to an
// Engagement (birth or postpartum), and a word the domain owns should not
// also name a piece of dispatch machinery.
type Registration struct {
	// Path is the full route pattern the endpoint mounts at, without a
	// method -- Register supplies POST. It is also the path a nudge task
	// POSTs, so the two cannot disagree.
	//
	// Cloud Scheduler is provisioned against these paths by hand, one job
	// per outbox, so they are a published contract: changing one is a
	// console change too, not only a code change.
	Path string
	// Door is the Postgres session variable to set for the length of the
	// worker's transaction, or empty for none. NotificationDoor for every
	// kind that mails somebody.
	Door string
	// Nudge is the ADR-0013 Cloud Tasks target, or empty for a kind that
	// rides Cloud Scheduler's cadence alone. #613's two Staff auth mail
	// outboxes are deliberately empty: that ticket accepted ADR-0010's
	// plain delay for them rather than wiring a nudge.
	Nudge tasknudge.OutboxType
	// Worker runs one batch. Every outbox worker already exposes exactly
	// this method, which is why Processor is the whole interface.
	Worker Processor
}

// Mux is the part of the BFF's router Register uses. An interface rather
// than *http.ServeMux so the router that owns every route -- and wants to
// see this package's thirteen among them rather than trusting that
// nothing reached around it -- can be handed here unchanged.
type Mux interface {
	Write(pattern string, h http.Handler)
}

// Register mounts every registration's process endpoint on mux, each
// authenticated by secret rather than by a session, and returns the paths
// it mounted in sorted order so a caller can assert the contract Cloud
// Scheduler is provisioned against.
func Register(mux Mux, db *sql.DB, secret string, registrations []Registration) []string {
	paths := make([]string, 0, len(registrations))
	for _, reg := range registrations {
		mux.Write("POST "+reg.Path, ProcessHandler(db, reg.Worker, secret, reg.Door))
		paths = append(paths, reg.Path)
	}
	sort.Strings(paths)
	return paths
}

// NudgePaths is the endpoint every nudged outbox is reached at, keyed by
// its ADR-0013 task type -- the map tasknudge's Cloud Tasks enqueuer needs
// and used to keep as a second copy of its own.
//
// A registration with no Nudge is absent rather than present and empty:
// enqueuing for a type that is not here is a programming error the
// enqueuer reports, and a kind that is not nudged at all should not
// silently look like one that is.
func NudgePaths(registrations []Registration) map[tasknudge.OutboxType]string {
	paths := make(map[tasknudge.OutboxType]string, len(registrations))
	for _, reg := range registrations {
		if reg.Nudge == "" {
			continue
		}
		paths[reg.Nudge] = reg.Path
	}
	return paths
}
