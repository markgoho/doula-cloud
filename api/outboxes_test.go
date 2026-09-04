package main

import (
	"reflect"
	"sort"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
)

// wantOutboxPaths is every process-* endpoint the BFF serves, written out
// rather than derived, because these are not free to change: Cloud
// Scheduler has one job per outbox provisioned by hand against these
// exact addresses (docs/environment.md), and nothing in this repository
// would notice a rename. A diff here means a console change too.
var wantOutboxPaths = []string{
	"/api/internal/clients/process-erasure-outbox",
	"/api/internal/notifications/process-engagement-request-outbox",
	"/api/internal/notifications/process-low-credit-outbox",
	"/api/internal/notifications/process-mfa-recovery-outbox",
	"/api/internal/notifications/process-offer-outbox",
	"/api/internal/notifications/process-outbox",
	"/api/internal/notifications/process-payment-outbox",
	"/api/internal/notifications/process-payout-outbox",
	"/api/internal/notifications/process-session-notice-outbox",
	"/api/internal/notifications/process-staff-email-change-outbox",
	"/api/internal/notifications/process-staff-invite-outbox",
	"/api/internal/notifications/process-staff-token-mail-outbox",
	"/api/internal/site/process-build-outbox",
}

func TestOutboxRegistrations_ServeTheProvisionedPaths(t *testing.T) {
	got := make([]string, 0, len(wantOutboxPaths))
	for _, reg := range outboxRegistrations(testDeps()) {
		got = append(got, reg.Path)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, wantOutboxPaths) {
		t.Errorf("outbox paths = %v, want %v", got, wantOutboxPaths)
	}
}

// TestOutboxRegistrations_EveryOneCarriesAWorker guards the failure a
// list of near-identical entries invites: a new outbox added with its
// path and nudge filled in and its Worker left at nil, which would mount
// an endpoint that panics on the first Scheduler tick.
func TestOutboxRegistrations_EveryOneCarriesAWorker(t *testing.T) {
	for _, reg := range outboxRegistrations(testDeps()) {
		if reg.Worker == nil {
			t.Errorf("%s has no Worker", reg.Path)
		}
	}
}

// TestOutboxRegistrations_MailingOutboxesOpenTheNotificationDoor pins the
// one deliberate exception: every outbox that mails somebody needs the
// door that licenses its recipient SELECT policies, and #443's site
// rebuild is the only one whose table is not under RLS at all.
func TestOutboxRegistrations_MailingOutboxesOpenTheNotificationDoor(t *testing.T) {
	const siteBuild = "/api/internal/site/process-build-outbox"

	for _, reg := range outboxRegistrations(testDeps()) {
		switch {
		case reg.Path == siteBuild && reg.Door != "":
			t.Errorf("%s opens door %q, want none -- its table is not under RLS", reg.Path, reg.Door)
		case reg.Path != siteBuild && reg.Door != outbox.NotificationDoor:
			t.Errorf("%s opens door %q, want %q", reg.Path, reg.Door, outbox.NotificationDoor)
		}
	}
}

// TestOutboxRegistrations_NudgeTargetsAreDistinct proves no two outboxes
// claim the same ADR-0013 task type -- which NudgePaths would resolve by
// silently dropping one, sending that outbox's nudges to the other's
// endpoint.
func TestOutboxRegistrations_NudgeTargetsAreDistinct(t *testing.T) {
	seen := map[tasknudge.OutboxType]string{}
	for _, reg := range outboxRegistrations(testDeps()) {
		if reg.Nudge == "" {
			continue
		}
		if other, dup := seen[reg.Nudge]; dup {
			t.Errorf("%s and %s both claim nudge type %q", other, reg.Path, reg.Nudge)
		}
		seen[reg.Nudge] = reg.Path
	}
}

// TestOutboxRegistrations_OnlyTheStaffAuthMailOutboxesAreUnnudged pins
// #613's decision that its two outboxes ride ADR-0010's plain delay,
// rather than letting a later outbox quietly join them by leaving Nudge
// blank.
func TestOutboxRegistrations_OnlyTheStaffAuthMailOutboxesAreUnnudged(t *testing.T) {
	var unnudged []string
	for _, reg := range outboxRegistrations(testDeps()) {
		if reg.Nudge == "" {
			unnudged = append(unnudged, reg.Path)
		}
	}
	sort.Strings(unnudged)

	want := []string{
		"/api/internal/notifications/process-staff-email-change-outbox",
		"/api/internal/notifications/process-staff-token-mail-outbox",
	}
	if !reflect.DeepEqual(unnudged, want) {
		t.Errorf("un-nudged outboxes = %v, want %v", unnudged, want)
	}
}

// TestNudgePaths_CoversEveryNudgeTypeTasknudgeDeclares is the guarantee
// tasknudge's own endpointPath map used to carry: a tasknudge.OutboxType
// constant with nowhere to point is a nudge that fails at runtime, and
// the map is now built from the registrations rather than maintained
// beside them.
func TestNudgePaths_CoversEveryNudgeTypeTasknudgeDeclares(t *testing.T) {
	paths := outbox.NudgePaths(outboxRegistrations(testDeps()))

	for _, outboxType := range []tasknudge.OutboxType{
		tasknudge.PortalInvite,
		tasknudge.LowCredit,
		tasknudge.Payout,
		tasknudge.PaymentReceived,
		tasknudge.SessionNotice,
		tasknudge.StaffInvite,
		tasknudge.EngagementOffer,
		tasknudge.EngagementRequest,
		tasknudge.SiteBuild,
		tasknudge.MFARecoveryCode,
		tasknudge.ClientErasure,
	} {
		if paths[outboxType] == "" {
			t.Errorf("nudge type %q has no endpoint", outboxType)
		}
	}
}
