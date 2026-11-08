package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
)

// AttachedDoula is one Doula on an Engagement: a person who holds an
// open, granted Attachment on it (GLOSSARY.md's Attachment, "who is
// working this birth"). An accrued Attachment names nobody here. It is a
// record of work done, and no screen calls its holder the Doula.
//
// OpenVisits is how many Visits she holds on it that have not happened:
// dated later than now or not dated at all (#2665), what taking her off
// leaves with her until someone reassigns them (#2596).
type AttachedDoula struct {
	StaffID    string `json:"staffId"`
	Name       string `json:"name"`
	OpenVisits int    `json:"openVisits"`
}

// AttachSelfResponse is who is on the Engagement once the caller has put
// herself on it, in the two fields Detail carries them in, so the
// Engagement hub draws its next state with no second read (the shape
// TransitionResponse has for StatusMoves).
type AttachSelfResponse struct {
	EngagementID  string          `json:"engagementId"`
	Doulas        []AttachedDoula `json:"doulas"`
	CanAttachSelf bool            `json:"canAttachSelf"`
}

// The three refusals. The Engagement hub prints the message of a refused
// press as it is (app/src/lib/engagementDetail.ts's attachSelf), so each
// is a whole sentence a person can read, and each names the reason. The
// first two also name the path that is open to her.
const (
	msgAttachSelfNotADoula   = "Only a Staff member who holds the Doula role can be put on this work."
	msgAttachSelfContractor  = "You contract for this Practice, so you take on work by accepting an Offer."
	msgAttachSelfIsCompleted = "This work has completed, so nobody can be put on it."
)

// readAttachedDoulas reads the Doulas on engagementID, in the order they
// were attached. A Doula whose staff row the reader can no longer reach
// (staff_practice_visibility admits a row only while she holds a
// Membership here) reads as activity.DepartedStaffName, the word the
// ledger already uses for her.
func readAttachedDoulas(ctx context.Context, tx *sql.Tx, engagementID string) ([]AttachedDoula, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT ea.staff_id, COALESCE(s.name, $2),
		        (SELECT count(*) FROM visits v
		          WHERE v.engagement_id = ea.engagement_id AND v.staff_id = ea.staff_id AND (v.scheduled_at IS NULL OR v.scheduled_at > $3))
		   FROM engagement_attachments ea
		   LEFT JOIN staff s ON s.id = ea.staff_id
		  WHERE ea.engagement_id = $1 AND ea.origin = 'granted' AND ea.ended_at IS NULL
		  ORDER BY ea.attached_at, ea.staff_id`,
		engagementID, activity.DepartedStaffName, clock.Now(ctx))
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: read attached doulas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	doulas := []AttachedDoula{}
	for rows.Next() {
		var doula AttachedDoula
		if err := rows.Scan(&doula.StaffID, &doula.Name, &doula.OpenVisits); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("engagement: scan attached doula: %w", err)
		}
		doulas = append(doulas, doula)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: iterate attached doulas: %w", err)
	}
	return doulas, nil
}

// isOn reports whether staffID is among doulas.
func isOn(doulas []AttachedDoula, staffID string) bool {
	return slices.ContainsFunc(doulas, func(d AttachedDoula) bool { return d.StaffID == staffID })
}

// lockEngagementStatus reads the Engagement's status under FOR UPDATE, so
// two presses at once are one after the other: the second reads the
// Attachment the first wrote and writes nothing. An Engagement that is not
// at this Practice is a 404 carrying notFound.
func lockEngagementStatus(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID, practiceID, notFound string) (status string, ok bool) {
	err := tx.QueryRowContext(r.Context(),
		`SELECT status::text FROM engagements WHERE id = $1 AND practice_id = $2 FOR UPDATE`,
		engagementID, practiceID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		apierr.WriteError(w, notFound, http.StatusNotFound)
		return "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return "", false
	}
	return status, true
}

// attachIfAbsent puts doulaID on the Engagement unless she is already on
// it (a granted Attachment by grantedBy, with its ledger entry), and
// returns who is on it afterward.
func attachIfAbsent(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID, engagementID, doulaID, grantedBy string) (doulas []AttachedDoula, ok bool) {
	doulas, err := readAttachedDoulas(r.Context(), tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	if isOn(doulas, doulaID) {
		return doulas, true
	}
	if err := attachment.Grant(r.Context(), tx, practiceID, engagementID, doulaID, grantedBy); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	doulas, err = readAttachedDoulas(r.Context(), tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	return doulas, true
}

// canAttachSelf is whether the caller would be put on this Engagement by
// AttachSelfHandler now: the rule admits her (attachment.Membership's
// WhyNotAttachable -- a Member here, with the Doula role, and not a
// contractor who holds neither Owner nor Admin), the Engagement has not
// completed, and she is not on it already.
//
// DetailHandler answers the Engagement hub with it and AttachSelfHandler
// asks the same three things in the same order, so the hub draws "Put me
// on this Engagement" exactly where the write accepts the press, with no
// copy of the rule in Svelte -- what legalMoves does for StatusMoves.
func canAttachSelf(reader staffauth.Reader, staffID, status string, doulas []AttachedDoula) bool {
	return attachment.OfReader(reader).WhyNotAttachable() == attachment.Attachable &&
		status != StatusCompleted &&
		!isOn(doulas, staffID)
}

// refuseSelfAttach writes the 403 for a caller the rule does not admit,
// and reports whether it did. A contractor who holds neither Owner nor
// Admin is refused whatever the Engagement is, before any read of it, so
// the answer tells her nothing about an Engagement she cannot reach --
// the order refuseFactWrite gives ADR-0015's own role gate.
func refuseSelfAttach(w http.ResponseWriter, reader staffauth.Reader) bool {
	switch attachment.OfReader(reader).WhyNotAttachable() {
	case attachment.Attachable:
		return false
	case attachment.IsContractor:
		apierr.WriteError(w, msgAttachSelfContractor, http.StatusForbidden)
	case attachment.NotADoula, attachment.NotAtPractice:
		// NotAtPractice never reaches here: staffauth.Middleware admits
		// only a Member, and OfReader says so. It is named so the switch
		// is whole.
		apierr.WriteError(w, msgAttachSelfNotADoula, http.StatusForbidden)
	}
	return true
}

// AttachSelfHandler puts the caller on an Engagement: "Put me on this
// Engagement" (#1598), the fourth writer of a granted Attachment that
// ADR-0008's amendment on #1515 names. One press, and no body: the only
// person it can attach is the person who pressed.
//
// For an employee Doula, and for an Owner or an Admin who also holds the
// Doula role, whatever her Employment type is (ADR-0008's amendment on
// #1625). ADR-0008's "an Owner or Admin acting on an Engagement is never
// attached by it" is about an accrued Attachment; this one is granted,
// because she decided it. A contractor who holds neither Owner nor Admin
// is refused (only her acceptance of an Offer attaches her), and so is a
// person with no Doula role (Attachment is for Doulas only).
//
// The Attachment is granted, carries no fee, and names her as
// attached_by; the doula_attached activity entry names her as the actor
// and as the Doula, and its created_at is the instant. Both are written
// by attachment.Grant, the write engagementrequest's approval uses.
//
// A PUT, and naturally idempotent (docs/api-design.md section 3 rule 4):
// a caller already on the Engagement gets the same 200 and nothing is
// written, so a double press leaves one row and one ledger entry. An
// open accrued Attachment is upgraded in place. A completed Engagement
// refuses, as it refuses an Offer (offer.requireOpenEngagement):
// completion ends every Attachment.
//
// It carries no staffauth.AttachingWrite: that seam mints accrued and
// never granted, and it attaches nobody who holds Owner or Admin. Must
// be mounted behind staffauth.Middleware.
func AttachSelfHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, reader, ok := staffauth.RequireTxReader(w, r)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if refuseSelfAttach(w, reader) {
			return
		}

		status, ok := lockEngagementStatus(w, r, tx, engagementID, practiceID, "engagement not found")
		if !ok {
			return
		}
		if status == StatusCompleted {
			apierr.WriteError(w, msgAttachSelfIsCompleted, http.StatusConflict)
			return
		}

		doulas, ok := attachIfAbsent(w, r, tx, practiceID, engagementID, staffID, staffID)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}

		apierr.WriteJSON(w, http.StatusOK, AttachSelfResponse{
			EngagementID:  engagementID,
			Doulas:        doulas,
			CanAttachSelf: canAttachSelf(reader, staffID, status, doulas),
		})
	})
}
