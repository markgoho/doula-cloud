package push_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/push"
)

func TestFakePusher_RecordsCalls(t *testing.T) {
	pusher := push.NewFakePusher()
	sub := push.Subscription{Endpoint: "https://push.example.com/one", P256dhKey: "p256dh", AuthKey: "auth"}

	if err := pusher.Send(t.Context(), sub, []byte(`{"engagementId":"e1"}`)); err != nil {
		t.Fatalf("send: %v", err)
	}

	calls := pusher.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Subscription != sub || string(calls[0].Payload) != `{"engagementId":"e1"}` {
		t.Fatalf("recorded call = %+v, want subscription %+v with the sent payload", calls[0], sub)
	}
}

func TestFakePusher_ReturnsConfiguredErrorAfterRecording(t *testing.T) {
	pusher := push.NewFakePusher()
	pusher.Err = errors.New("simulated push failure")
	sub := push.Subscription{Endpoint: "https://push.example.com/two"}

	err := pusher.Send(t.Context(), sub, []byte("payload"))
	if !errors.Is(err, pusher.Err) {
		t.Fatalf("err = %v, want %v", err, pusher.Err)
	}
	if calls := pusher.Calls(); len(calls) != 1 {
		t.Fatalf("calls = %d, want 1 (still recorded despite the error)", len(calls))
	}
}
