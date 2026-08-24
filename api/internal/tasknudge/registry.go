package tasknudge

import "context"

type contextKey string

const pendingKey contextKey = "tasknudge.pending"

// Begin returns ctx with an empty pending-nudge list attached, for
// staffauth.Middleware to attach before running the handler chain. A
// handler running outside a context Begin was called on simply has
// nothing to Register against -- Register is a no-op, and the write
// still gets Cloud Scheduler's cadence, just not the nudge.
func Begin(ctx context.Context) context.Context {
	return context.WithValue(ctx, pendingKey, &[]func(context.Context){})
}

// Register schedules fn to run when Drain is next called on ctx (or a
// context derived from it) -- staffauth.Middleware, once its own
// tx.Commit() succeeds. Handlers pass the result of Fire, already bound
// to an Enqueuer and OutboxType.
func Register(ctx context.Context, fn func(context.Context)) {
	if pending, ok := ctx.Value(pendingKey).(*[]func(context.Context)); ok {
		*pending = append(*pending, fn)
	}
}

// Drain runs every function registered via Register on ctx, in
// registration order. Callers must only invoke this after confirming
// their own commit succeeded -- Drain has no way to know that on its own.
func Drain(ctx context.Context) {
	pending, ok := ctx.Value(pendingKey).(*[]func(context.Context))
	if !ok {
		return
	}
	for _, fn := range *pending {
		fn(ctx)
	}
}
