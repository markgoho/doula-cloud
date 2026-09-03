package tasknudge_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/tasknudge"
)

func TestFakeEnqueuer_RecordsCalls(t *testing.T) {
	f := &tasknudge.FakeEnqueuer{}

	if err := f.Enqueue(t.Context(), tasknudge.PortalInvite); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := f.Enqueue(t.Context(), tasknudge.LowCredit); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	calls := f.Calls()
	if len(calls) != 2 || calls[0] != tasknudge.PortalInvite || calls[1] != tasknudge.LowCredit {
		t.Fatalf("Calls() = %v, want [%v %v]", calls, tasknudge.PortalInvite, tasknudge.LowCredit)
	}
}

func TestFakeEnqueuer_ReturnsConfiguredError(t *testing.T) {
	wantErr := errors.New("cloud tasks unavailable")
	f := &tasknudge.FakeEnqueuer{Err: wantErr}

	if err := f.Enqueue(t.Context(), tasknudge.Payout); !errors.Is(err, wantErr) {
		t.Fatalf("Enqueue err = %v, want %v", err, wantErr)
	}
	if len(f.Calls()) != 0 {
		t.Fatalf("expected no recorded calls when Err is set")
	}
}

func TestNoOpEnqueuer_AlwaysSucceeds(t *testing.T) {
	if err := (tasknudge.NoOpEnqueuer{}).Enqueue(t.Context(), tasknudge.PortalInvite); err != nil {
		t.Fatalf("Enqueue: %v, want nil", err)
	}
}

func TestFire_EnqueuesOnSuccess(t *testing.T) {
	f := &tasknudge.FakeEnqueuer{}

	tasknudge.Fire(f, tasknudge.SessionNotice)(t.Context())

	calls := f.Calls()
	if len(calls) != 1 || calls[0] != tasknudge.SessionNotice {
		t.Fatalf("Calls() = %v, want [%v]", calls, tasknudge.SessionNotice)
	}
}

func TestFire_SwallowsEnqueueFailure(t *testing.T) {
	f := &tasknudge.FakeEnqueuer{Err: errors.New("cloud tasks unavailable")}

	// Must not panic -- Fire's whole point is that an enqueue failure
	// never propagates past this call.
	tasknudge.Fire(f, tasknudge.PaymentReceived)(t.Context())
}

func TestRegisterAndDrain_RunsRegisteredFuncsInOrder(t *testing.T) {
	ctx := tasknudge.Begin(t.Context())

	f := &tasknudge.FakeEnqueuer{}
	tasknudge.Register(ctx, tasknudge.Fire(f, tasknudge.PortalInvite))
	tasknudge.Register(ctx, tasknudge.Fire(f, tasknudge.SessionNotice))

	tasknudge.Drain(ctx)

	calls := f.Calls()
	if len(calls) != 2 || calls[0] != tasknudge.PortalInvite || calls[1] != tasknudge.SessionNotice {
		t.Fatalf("Calls() = %v, want [%v %v]", calls, tasknudge.PortalInvite, tasknudge.SessionNotice)
	}
}

func TestRegister_NoOpWithoutBegin(t *testing.T) {
	f := &tasknudge.FakeEnqueuer{}

	// ctx never went through Begin -- Register must not panic, and the
	// closure must never run.
	tasknudge.Register(t.Context(), tasknudge.Fire(f, tasknudge.PortalInvite))

	if len(f.Calls()) != 0 {
		t.Fatalf("expected no calls when Register runs without Begin")
	}
}

func TestDrain_NoOpWithoutBegin(t *testing.T) {
	// Must not panic when ctx never went through Begin.
	tasknudge.Drain(t.Context())
}

// TestDelay proves the one type that waits, and that every other one
// still fires immediately. #443's worker collapses queued rebuilds into
// one dispatch, which only works if rows have had a moment to gather --
// an immediate nudge per publish would deploy once per publish.
func TestDelay(t *testing.T) {
	if got := tasknudge.Delay(tasknudge.SiteBuild); got <= 0 {
		t.Fatalf("Delay(SiteBuild) = %v, want a real wait", got)
	}
	for _, outboxType := range []tasknudge.OutboxType{
		tasknudge.PortalInvite,
		tasknudge.LowCredit,
		tasknudge.Payout,
		tasknudge.PaymentReceived,
		tasknudge.SessionNotice,
		tasknudge.StaffInvite,
		tasknudge.EngagementOffer,
		tasknudge.EngagementRequest,
		tasknudge.MFARecoveryCode,
	} {
		if got := tasknudge.Delay(outboxType); got != 0 {
			t.Fatalf("Delay(%s) = %v, want no wait", outboxType, got)
		}
	}
}
