// Package sitebuild rebuilds doula.cloud when a Practice publishes, and
// proves her page resolves once it has (#443).
//
// The deploy workflow fires on a push touching hugo/**. A Practice
// publishing her page (#441) produces no commit, so without this
// nothing builds and her page never appears. #421 decided the BFF fires
// the deploy itself -- a repository_dispatch -- rather than a schedule
// that burns builds on nothing and still makes her wait, or a manual run
// that fails silently the moment nobody is watching.
//
// Two halves, and the second is the one that earns the package.
//
// Making the build happen is an outbox (ADR-0010) with a delayed nudge
// (ADR-0013): a publish queues a row, and a worker collapses every
// pending row into one dispatch. Delayed because collapsing only works
// if rows have had a moment to accumulate -- see CoalesceWindow.
//
// Proving it worked is a probe. Stripe holds the declared URL for the
// life of the connected account and reviews it on its own schedule with
// no published SLA (#382), so a page that 404s is a rejection arriving
// weeks later with no visible cause. The probe is deliberately the same
// code on both of its callers: the deploy workflow POSTs once when it
// finishes, and Cloud Scheduler sweeps on a cadence. The sweep is what
// covers the case the callback cannot -- a build that fails produces no
// deploy, no callback and no news at all, and only something that runs
// anyway will notice the page never went live.
package sitebuild

import (
	"context"
	"net/http"
	"time"
)

// The liveness states 00049 defines, and the whole answer to "is her
// page actually there?". Only an affirmative probe leaves StatePending:
// a page nothing ever checks stays pending and says so, because absence
// of a report must never read as a pass.
const (
	// StatePending is set by the write site on every publish or edit,
	// and means no probe has confirmed the result yet.
	StatePending = "pending"
	// StateLive means a probe fetched the page and got it.
	StateLive = "live"
	// StateFailed means a probe ran and did not get the page.
	StateFailed = "failed"
)

// CoalesceWindow is how long a queued rebuild waits before the worker
// will dispatch it.
//
// It exists because ADR-0013's nudge fires immediately and per write,
// deliberately without de-duplication, which is right for an email
// outbox -- two invitations are two emails -- and wrong here, where two
// publishes a minute apart need one rebuild between them. Without a
// wait, each nudge finds its own row and dispatches its own deploy.
//
// So the worker refuses to dispatch until the oldest pending row has
// aged past this, and then claims every pending row including the ones
// that arrived since. Ninety seconds is long enough to swallow a
// Practice fixing a typo straight after publishing, and short against
// the only clock that matters: Stripe does not fetch the URL at submit
// (#421 walked this), and the review that does read it is later and
// ongoing.
const CoalesceWindow = 90 * time.Second

// MaxAttempts is how many times a dispatch may fail before its rows are
// dead-lettered. A credential that has lapsed fails identically every
// time, and retrying it until the end of the world buys nothing -- the
// page stays pending, and the sweep is what tells the Practice so.
const MaxAttempts = 10

// Dispatcher asks GitHub to run the deploy workflow. One call per
// dispatch, carrying no Practice data: the build reads the database for
// itself, so there is nothing useful to put in a payload and nothing
// lost by leaving it empty.
type Dispatcher interface {
	Dispatch(ctx context.Context) error
}

// PageProbe is the result of asking the live site for one page.
type PageProbe struct {
	// State is StateLive or StateFailed. A probe never reports pending:
	// pending is the absence of a probe.
	State string
	// Detail is a few words for the Practice to read, empty when the
	// page resolved.
	Detail string
}

// Prober fetches one published page and says whether it is there.
type Prober interface {
	Probe(ctx context.Context, slug string) PageProbe
}

// Clock is time.Now, injected so the coalescing window and the recorded
// check times are testable without sleeping.
type Clock func() time.Time

// HTTPDoer is the http.Client seam the real Prober and Dispatcher take,
// so both can be tested against a stub rather than the network.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
