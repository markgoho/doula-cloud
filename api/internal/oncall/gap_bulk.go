package oncall

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// The sentences the bulk act says that a single gap's form does not.
const (
	MsgBirthsChanged        = "The births on call changed since you checked. Check them again."
	MsgNoBirthsChosen       = "There are no births to record this time on."
	MsgIdempotencyKeyNeeded = "Send an Idempotency-Key header with the request."
	fieldBirths             = "births"
)

// GapPreviewBirth is one birth a time of a Doula's meets, cut to her own
// days on it, with who else on the birth can cover it and the gap of hers
// already in force there, if any.
type GapPreviewBirth struct {
	EngagementID string     `json:"engagementId"`
	ClientName   string     `json:"clientName"`
	StartsAt     time.Time  `json:"startsAt"`
	EndsAt       time.Time  `json:"endsAt"`
	CoverOptions []StaffRef `json:"coverOptions"`
	ExistingGap  *Gap       `json:"existingGap,omitempty"`
}

// GapPreview is every birth a time meets for one Doula.
type GapPreview struct {
	Births []GapPreviewBirth `json:"births"`
}

// BulkGapRequest is the body of the act that records one time on every
// birth it meets. Births names each birth to write on and who covers it.
type BulkGapRequest struct {
	StaffID  string         `json:"staffId"`
	StartsAt *time.Time     `json:"startsAt"`
	EndsAt   *time.Time     `json:"endsAt"`
	Reason   *string        `json:"reason"`
	Births   []BulkGapBirth `json:"births"`
}

// BulkGapBirth is one birth the act writes a gap on.
type BulkGapBirth struct {
	EngagementID    string  `json:"engagementId"`
	CoveringStaffID *string `json:"coveringStaffId"`
}

// BulkGapResponse is the gaps the act wrote, one per birth.
type BulkGapResponse struct {
	Gaps []Gap `json:"gaps"`
}

// gapHit is a birth a time meets: the Engagement, the time cut to the
// Doula's own days on it, and her gap already in force over that cut.
type gapHit struct {
	engagement engagementOnCall
	startsAt   time.Time
	endsAt     time.Time
	existing   *Gap
}

// hitBirths finds the live births staffID holds a granted Attachment on
// whose days she is on call for meet [startsAt, endsAt). The read is by
// her Attachment, in the query (ADR-0008 Reach), so it holds only her own
// Engagements. Her days are her narrowing, or the whole window without
// one, and the time is cut to them so a backup's evening before her days
// begin raises nothing.
func hitBirths(call staffauth.Call, r *http.Request, settings practiceSettings, staffID string, startsAt, endsAt time.Time) ([]gapHit, error) {
	engagements, err := loadEngagements(r.Context(), call.Tx, call.PracticeID, settings, engagementsFilter{attachedStaffID: staffID})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	var hits []gapHit
	var ids []string
	for _, e := range engagements {
		if e.window == nil {
			continue
		}
		i := slices.IndexFunc(e.doulas, func(d attachedDoula) bool { return d.staffID == staffID })
		days, ok := e.window.Effective(e.doulas[i].from, e.doulas[i].to)
		if !ok {
			continue
		}
		from, to := days.Bounds(settings.zone)
		cutStart, cutEnd := later(startsAt, from), earlier(endsAt, to)
		if !cutEnd.After(cutStart) {
			continue
		}
		hits = append(hits, gapHit{engagement: e, startsAt: cutStart, endsAt: cutEnd})
		ids = append(ids, e.id)
	}
	if len(hits) == 0 {
		return nil, nil
	}
	live, err := loadLiveGaps(r.Context(), call.Tx, ids, startsAt, endsAt)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	for i := range hits {
		for _, g := range live[hits[i].engagement.id] {
			if g.StaffID == staffID && g.StartsAt.Before(hits[i].endsAt) && g.EndsAt.After(hits[i].startsAt) {
				hits[i].existing = &g
				break
			}
		}
	}
	slices.SortFunc(hits, func(a, b gapHit) int {
		return cmp.Or(strings.Compare(a.engagement.clientName, b.engagement.clientName), strings.Compare(a.engagement.id, b.engagement.id))
	})
	return hits, nil
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// refuseBulkAsk checks the Doula and the time the act is about, writing
// the refusal itself: nobody named, a colleague a Doula may not name, or a
// time that is missing or runs backward.
func refuseBulkAsk(w http.ResponseWriter, call staffauth.Call, staffID string, startsAt, endsAt *time.Time) bool {
	details := map[string]string{}
	if staffID == "" {
		details[fieldStaffID] = MsgStaffIDNeeded
	}
	if startsAt == nil {
		details[fieldStartsAt] = MsgStartsAtRequired
	}
	if endsAt == nil {
		details[fieldEndsAt] = MsgEndsAtRequired
	}
	if startsAt != nil && endsAt != nil && !endsAt.After(*startsAt) {
		details[fieldEndsAt] = MsgEndsBeforeStarts
	}
	if len(details) > 0 {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The coverage gap could not be saved.", details)
		return true
	}
	if staffID != call.StaffID && !call.Reader.IsOwnerOrAdmin() {
		apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden, MsgOnlyYourOwnGap, nil)
		return true
	}
	return false
}

// PreviewGapsHandler reads the births a time a Doula cannot be reached
// meets, before anything is saved: each with who can cover it, and the gap
// already recorded there. A Doula may ask only about herself; an Owner or
// an Admin about any Doula.
//
// Must be mounted behind staffauth.Middleware.
func PreviewGapsHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call) {
		query := r.URL.Query()
		staffID := query.Get("staffId")
		startsAt := parseInstant(query.Get("startsAt"))
		endsAt := parseInstant(query.Get("endsAt"))
		if refuseBulkAsk(w, call, staffID, startsAt, endsAt) {
			return
		}
		settings, err := loadPracticeSettings(r.Context(), call.Tx, call.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		hits, err := hitBirths(call, r, settings, staffID, *startsAt, *endsAt)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		out := GapPreview{Births: []GapPreviewBirth{}}
		for _, h := range hits {
			options := []StaffRef{}
			for _, d := range h.engagement.doulas {
				if d.staffID != staffID {
					options = append(options, StaffRef{StaffID: d.staffID, Name: d.name})
				}
			}
			out.Births = append(out.Births, GapPreviewBirth{
				EngagementID: h.engagement.id,
				ClientName:   h.engagement.clientName,
				StartsAt:     h.startsAt,
				EndsAt:       h.endsAt,
				CoverOptions: options,
				ExistingGap:  h.existing,
			})
		}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// parseInstant reads an RFC 3339 instant, or nil where there is none.
func parseInstant(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// RecordGapsHandler records one time a Doula cannot be reached on every
// birth it meets, in one transaction: a Coverage gap on each birth, cut to
// her own days there, through the same checks, ledger entry and
// Notification a single gap takes. Every birth left with nobody covering
// queues its own Notification, as it does one at a time, and the worker is
// nudged once.
//
// The births in the body must be exactly the births the time meets now
// with no gap of hers already over it; anything else is a 409, so the
// person never records on a set she did not see. Nothing is written when
// any birth is refused.
//
// Must be mounted behind staffauth.Middleware and idempotency.Wrap.
func RecordGapsHandler(enq tasknudge.Enqueuer) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call) {
		if r.Header.Get("Idempotency-Key") == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgIdempotencyKeyNeeded, nil)
			return
		}
		var req BulkGapRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if refuseBulkAsk(w, call, req.StaffID, req.StartsAt, req.EndsAt) {
			return
		}
		if len(req.Births) == 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The coverage gap could not be saved.",
				map[string]string{fieldBirths: MsgNoBirthsChosen})
			return
		}
		settings, err := loadPracticeSettings(r.Context(), call.Tx, call.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		hits, err := hitBirths(call, r, settings, req.StaffID, *req.StartsAt, *req.EndsAt)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		writes, ok := matchWrites(hits, req.Births)
		if !ok {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, MsgBirthsChanged, nil)
			return
		}

		facts := make([]gapFacts, len(writes))
		details := map[string]string{}
		for i, h := range writes {
			f, refused := newGapWrite(call, settings, h.engagement).check(GapRequest{
				StaffID: req.StaffID, StartsAt: &h.startsAt, EndsAt: &h.endsAt,
				Reason: req.Reason, CoveringStaffID: req.Births[h.index].CoveringStaffID,
			})
			for field, message := range refused {
				if field == fieldCoveringStaffID {
					field = fmt.Sprintf("%s[%d].%s", fieldBirths, h.index, fieldCoveringStaffID)
				}
				details[field] = message
			}
			facts[i] = f
		}
		if len(details) > 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "The coverage gap could not be saved.", details)
			return
		}

		out := BulkGapResponse{Gaps: []Gap{}}
		anyQueued := false
		for i, h := range writes {
			c := newGapWrite(call, settings, h.engagement)
			gapID, err := c.insertGap(r.Context(), facts[i])
			if err == nil {
				var queued bool
				queued, err = c.recordSaved(r.Context(), gapID, activity.ActionCoverageGapCreated, gapDiff{GapID: gapID, After: &facts[i]}, facts[i])
				anyQueued = anyQueued || queued
			}
			var g Gap
			if err == nil {
				g, err = readGap(r.Context(), call.Tx, gapID)
			}
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			out.Gaps = append(out.Gaps, g)
		}
		if anyQueued {
			tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.CoverageGap))
		}
		apierr.WriteJSON(w, http.StatusCreated, out)
	})
}

// gapWriteAt is a hit the act writes on, with the place of its birth in
// the request body.
type gapWriteAt struct {
	gapHit
	index int
}

// matchWrites pairs the request's births with the hits still to write on,
// in the request's order. ok is false unless the two are the same set: a
// birth named twice, a birth the time no longer meets, a gap that now
// covers it, or a hit the request left out.
func matchWrites(hits []gapHit, births []BulkGapBirth) ([]gapWriteAt, bool) {
	writable := map[string]gapHit{}
	for _, h := range hits {
		if h.existing == nil {
			writable[h.engagement.id] = h
		}
	}
	if len(writable) != len(births) {
		return nil, false
	}
	out := make([]gapWriteAt, 0, len(births))
	for i, b := range births {
		h, found := writable[b.EngagementID]
		if !found {
			return nil, false
		}
		delete(writable, b.EngagementID)
		out = append(out, gapWriteAt{gapHit: h, index: i})
	}
	return out, true
}
