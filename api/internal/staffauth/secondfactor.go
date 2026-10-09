package staffauth

import (
	"context"
	"encoding/json"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
)

// SecondFactorActs is every act that needs a second factor from the
// person who asks, keyed by the route that performs it ("METHOD
// /pattern", as GatedRouter registers it), valued by the act's name in
// the refusal record (#1532, ADR-0026's amendment for #1492). Each one
// takes every record out at once, cannot be undone, or decides another
// person's access.
//
// This map is the whole declaration. GatedRouter.Get and GatedWrite look
// a route up here as they mount it and put the seam in front of its
// handler, so a sixth act is one line below; api's
// TestSecondFactorActs_EveryDeclaredActIsMounted fails on a line naming
// a route the binary does not mount behind the seam.
//
// The switch is declared as the whole PUT, though only turning it on is
// the act. Nothing is lost by that: with the switch already on, the
// Practice boundary refuses anyone with no second factor before this
// seam runs, so the only request this seam can refuse that does not
// turn the switch on is one that sets it off when it is already off.
var SecondFactorActs = map[string]string{
	"GET /api/practices/{practiceId}/export":                              "practice_export",
	"POST /api/practices/{practiceId}/deletion":                           "practice_deletion",
	"POST /api/practices/{practiceId}/clients/{clientId}/erasure":         "client_erasure",
	"POST /api/practices/{practiceId}/staff/{staffId}/mfa-recovery/vouch": "staff_vouch",
	"PUT /api/practices/{practiceId}/mfa-required":                        "mfa_required_switch",
}

// ActionSecondFactorRefused is the activity row the seam writes when it
// refuses an act: who tried (the row's actor), when (its created_at),
// and which act (its diff's "act"). Recorded against the Practice
// itself, the same SubjectPractice the archive, deletion and switch
// already record their own successes against.
const ActionSecondFactorRefused = "second_factor_refused"

// secondFactorRefusal is ActionSecondFactorRefused's diff.
type secondFactorRefusal struct {
	Act string `json:"act"`
}

// msgSecondFactorRequired is the refusal's sentence, for a caller that
// reads it rather than the code. The app does not show it: each of the
// five screens already says, before she tries, that the act needs a
// second factor.
const msgSecondFactorRequired = "this act needs a second sign-in factor"

// SecondFactor reports whether this request's session showed a second
// factor at sign-in, as Middleware read it. False off a request
// Middleware did not resolve.
func SecondFactor(ctx context.Context) bool {
	v, _ := ctx.Value(secondFactorKey).(bool)
	return v
}

// requireSecondFactor is the seam: it refuses act with
// SECOND_FACTOR_REQUIRED, and records the refusal, unless this
// request's session shows a second factor. GatedRouter places it after
// the role check, so only a person who may do the act is refused for
// lack of a second factor -- a Doula asking for the archive meets the
// role refusal, as she did before -- and outside any idempotency.Wrap,
// so a refusal is never cached under the caller's key and replayed
// after she enrolls.
//
// The record is written in Middleware's own transaction, which commits
// once the handler returns whatever it answered.
func requireSecondFactor(act string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if SecondFactor(r.Context()) {
			h.ServeHTTP(w, r)
			return
		}
		tx, practiceID, ok := RequireTx(w, r)
		if !ok {
			// coverage:ignore reason: Middleware always sets a tx before this seam runs
			return
		}
		staffID, _ := StaffID(r.Context())
		diff, _ := json.Marshal(secondFactorRefusal{Act: act})
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   practiceID,
			Action:      ActionSecondFactorRefused,
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.Write(w, http.StatusForbidden, apierr.CodeSecondFactorRequired, msgSecondFactorRequired, nil)
	})
}
