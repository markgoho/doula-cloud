// Package clock is the one seam api/ reads the current instant through.
// #773 found 27+ bare time.Now() calls across 19+ files, each one a
// place simulated time (#762's forthcoming database shim) will not
// move -- and a count that only grows as features are added, since
// nothing stopped a new one from being written. This package, plus the
// forbidigo rule in api/.golangci.yml, is that stop.
//
// # Shape
//
// Clock is a bare func returning time.Time -- a type alias, not a named
// type or an interface, so it is structurally identical to the seam
// already proven in this codebase: outbox.Worker.Now
// (api/internal/outbox/outbox.go), sitebuild.Worker.Now and
// sitebuild.Verifier.Now, and client.ErasureWorker.Now all already hold
// this exact shape and need no change to satisfy it.
//
// # Delivery: request context, not a parameter on every handler
//
// outbox.Worker threads its Now field through a handful of methods on
// one struct -- a bounded, single-owner call graph. The BFF's HTTP
// handlers are the opposite shape: authn.Begin, the one place session
// expiry is decided (the "interesting case" #773's own text names), is
// called directly by 15 bootstrap-style handlers across staffauth and
// clientauth and indirectly by every authenticated request through
// staffauth.Middleware and clientauth.Middleware, which are themselves
// constructed in 8 more files (staffauth/gate.go,
// idempotency/router.go, and six Mount functions across pushsub,
// portal, notificationpref, plans, contracts and message). Threading a
// Clock parameter through Middleware the way outbox.Worker threads Now
// would mean widening every one of those ~23 signatures for the sake of
// one call site three layers down -- most of which, unlike this
// package's own leaf-level callers, have no time.Now() call of their
// own to justify the change. That is not the pattern this ticket
// generalizes; it is the pattern outbox.Worker was small enough to
// avoid needing.
//
// This codebase already has an answer for "a value many disparate
// handlers need, without threading it through every constructor between
// here and there": staffauth.StaffID(ctx), staffauth.PracticeID(ctx) and
// staffauth.ReaderFrom(ctx) all carry a request-scoped value the same
// way. Clock follows that precedent. Middleware seeds one Clock into
// every request's context, once, at the top of routes() in api/main.go's
// route table; Now(ctx) reads it back at whatever depth actually needs
// the time. Nothing about this is a package-level global: each request
// gets the context value routes() built for it, a simulation run's
// requests and a unit test's requests can each carry a different one,
// and nothing here is ever reassigned after main() constructs Deps.Now
// once.
//
// # What this does not touch
//
// Firebase token verification (authn.FirebaseVerifier.VerifyIDToken)
// reads no clock this package exposes, on purpose: #762 found that
// pointing a fake clock at it makes every sign-in fail, since Identity
// Platform's own SDK checks token freshness against real wall time no
// value in this codebase can shift. See the doc comment on VerifyIDToken
// for the citation.
package clock

import (
	"context"
	"net/http"
	"time"
)

// Clock returns the current instant. A type alias, not a distinct named
// type: every existing "Now func() time.Time" field in this codebase
// (outbox.Worker, sitebuild.Worker, sitebuild.Verifier,
// client.ErasureWorker, mail.Mailer) already satisfies it with no change.
type Clock = func() time.Time

// Real is the one sanctioned spelling of the wall clock in api/ --
// everywhere else calls Now(ctx) or holds an injected Clock instead of
// calling time.Now() directly, which is what the forbidigo rule in
// api/.golangci.yml enforces.
//
//nolint:forbidigo // the seam's own default value; every other production call site reads through Now(ctx) or an injected Clock instead
var Real Clock = time.Now

type contextKey struct{}

// Into returns a copy of ctx carrying now as the Clock later code on the
// same request reads back with Now. Exported mainly for tests that need
// a fixed instant without going through Middleware.
func Into(ctx context.Context, now Clock) context.Context {
	if now == nil {
		now = Real
	}
	return context.WithValue(ctx, contextKey{}, now)
}

// Now reads the current instant off ctx -- the Clock a simulation run or
// a test seeded with Into/Middleware, or Real when nothing seeded one.
// The fallback keeps every handler correct by default: a request that
// never passed through Middleware (a handler constructed directly in a
// unit test, say) reads real time, exactly what a bare time.Now() call
// would have returned.
func Now(ctx context.Context) time.Time {
	if now, ok := ctx.Value(contextKey{}).(Clock); ok && now != nil {
		return now()
	}
	return Real()
}

// Middleware seeds now into every request's context before it reaches
// next, so any handler downstream -- however many Mount/Middleware
// layers deep -- can read it back with Now(r.Context()) without a Clock
// parameter threaded through each of those layers. now nil (Deps.Now's
// zero value, which every test and every route table built before this
// ticket already has) falls back to Real, so an unset Deps.Now behaves
// exactly like the bare time.Now() calls it replaces.
func Middleware(now Clock) func(http.Handler) http.Handler {
	if now == nil {
		now = Real
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(Into(r.Context(), now)))
		})
	}
}
