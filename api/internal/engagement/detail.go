// Package engagement holds the Staff-side BFF handlers for Engagement
// detail, its status transition (#253) and its birth outcome (#293).
// All handlers rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// staffauth's own Owner-only handlers (invite, role assignment) do. The
// Client write surface and the Clients list moved to package client
// (#397); this package's own surface is now just the Engagement itself.
package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/cardpayments"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/personname"
	"doula-cloud/api/internal/staffauth"
)

// Detail is an Engagement's basic detail: the Client it's for, its
// status, when it was created, and when it's due -- the landing point
// later features (Visits, Plans, Contracts, Messages) attach to.
type Detail struct {
	EngagementID string `json:"engagementId"`
	ClientID     string `json:"clientId"`
	ClientName   string `json:"clientName"`
	// ClientDisplayName is the page's title (#2004): personname.Full, the
	// words her own Client record is titled with (#2111). ClientName stays
	// the conversation name the page's sentences use.
	ClientDisplayName string    `json:"clientDisplayName"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"createdAt"`
	// Kind is ADR-0015's own fact (#874's write side): what the Practice
	// sold, "birth" or "postpartum". Staff-only, per kind.go's own
	// package comment -- portal.Detail never carries this column, only
	// OffersBirthPlan's derived answer.
	Kind string `json:"kind"`
	// DueDate is ADR-0017's `engagements.due_date`, nullable because a
	// postpartum-only Engagement has none. Mirrors portal.Detail's own
	// field (#505) -- same nullable-column read, same omitted-when-null
	// shape (#538).
	DueDate *string `json:"dueDate,omitempty"`
	// BirthOutcome/PregnancyEndedOn (#293) are ADR-0015's staff-only
	// birth outcome and the date the pregnancy ended, both absent from
	// the JSON until recorded. They are on this Staff-side DTO and
	// deliberately not on portal.Detail: ADR-0015 binds them as never
	// Client-facing, and portal.Detail's own read is what enforces it.
	BirthOutcome     *string `json:"birthOutcome,omitempty"`
	PregnancyEndedOn *string `json:"pregnancyEndedOn,omitempty"`
	// OffersBirthPlanToStart and OffersBirthPlan are kind.go's two answers
	// (#2277, #2317): whether plans.PostInstanceHandler would start a Birth
	// Plan here, and whether the Client is shown one. The hub draws its
	// Birth Plan section off them and holds no copy of the rule.
	OffersBirthPlanToStart bool `json:"offersBirthPlanToStart"`
	OffersBirthPlan        bool `json:"offersBirthPlan"`
	// StatusMoves (#253) is legalMoves(reader, Status) -- exactly the
	// target statuses this caller may move to from Status, per ADR-0015's
	// six-move table narrowed by its role table. TransitionHandler
	// answers the same call with the same function, so the Engagement hub
	// renders exactly the controls the write endpoint will accept without
	// copying the role table into Svelte.
	StatusMoves []string `json:"statusMoves"`

	// ClientPortalInviteStatus/ClientEmailSuppressed/ClientHasEmail
	// (#255) are the Client's portal-invite state, using
	// client.FetchPortalInviteState -- the same derivation the Clients
	// list's own PortalInviteStatus/EmailSuppressed carry, reinstated
	// here so the Engagement hub can state it as standing information
	// (never invited / pending / accepted, and whether the address can
	// even be invited) rather than only as feedback after a Send.
	ClientPortalInviteStatus *string `json:"clientPortalInviteStatus,omitempty"`
	ClientEmailSuppressed    bool    `json:"clientEmailSuppressed"`
	ClientHasEmail           bool    `json:"clientHasEmail"`

	// ClientsCanPay (#270) is cardpayments.ClientsCanPay -- one column, no
	// Stripe round-trip. A standing fact about the Practice's own billing
	// readiness, not the Owner's Stripe Connect account state
	// (ADR-0008's separate "Stripe Connect state" row): the Invoice
	// section reads this to show the Create Invoice form or a Notice
	// naming what is missing, before a Staff member ever presses Create.
	ClientsCanPay bool `json:"clientsCanPay"`

	// Doulas (#1598) is who is on this Engagement: each person who holds
	// an open, granted Attachment on it, in the order they were attached.
	// Never null -- an Engagement with nobody on it carries an empty
	// list, and the hub says "No Doula yet". A reader who may read the
	// Engagement may read who is on it: the on-call panel and the Clients
	// list name the same people to the same readers.
	Doulas []AttachedDoula `json:"doulas"`
	// CanAttachSelf (#1598) is canAttachSelf -- whether AttachSelfHandler
	// would put this caller on the Engagement now. The hub draws "Put me
	// on this Engagement" off it and holds no copy of the rule, the same
	// arrangement StatusMoves has with TransitionHandler.
	CanAttachSelf bool `json:"canAttachSelf"`

	// YourFee (#2003) is the reader's own agreed fee on this Engagement:
	// the fee an accepted Offer copied onto her open, granted Attachment
	// (ADR-0008's `fee_amount_cents`). ADR-0008's money row, as amended
	// on #282, gives a contractor "her own agreed fee only, on her
	// Engagements -- never the Practice's price", and this is that fee.
	// It is keyed on the reader's own Attachment, never on a role: an
	// employee is never offered a fee (the Offer's CHECK forbids one), so
	// she gets none, and an Owner reads no other Doula's fee here. Absent
	// when the reader holds no open Attachment with a fee.
	YourFee *AgreedFee `json:"yourFee,omitempty"`

	// HasDoulaFees (#2492) is whether any Doula on this Engagement, open
	// or ended, has an agreed fee, so the hub stands its Doula fees
	// section from its first frame rather than once the fees read lands.
	// Answered only for an Owner or Admin, the readers of those fees
	// (#2271, ADR-0008); false for anyone else, so the flag tells nobody
	// else that a fee exists.
	HasDoulaFees bool `json:"hasDoulaFees"`

	// HasCarePlan, HasBirthPlan and HasContract (#2591) say whether each
	// of those records exists on this Engagement, so the hub asks for the
	// ones that do and the read that would answer 404 is never sent. Their
	// own reads stay `404` for a missing record (docs/api-design.md); every
	// Staff caller who can read this Engagement can read all three, so the
	// flags tell nobody anything the reads would not.
	HasCarePlan  bool `json:"hasCarePlan"`
	HasBirthPlan bool `json:"hasBirthPlan"`
	HasContract  bool `json:"hasContract"`

	// StartNote (#2597) is the Note typed on the Engagement Request that
	// started this Engagement, who wrote it and when. Answered only for
	// an Owner or an Admin, the readers of a Request's note (ADR-0017);
	// absent when no note was written.
	StartNote *StartNote `json:"startNote,omitempty"`
}

// StartNote is the Note on the Engagement Request an Engagement came from.
type StartNote struct {
	Text      string    `json:"text"`
	WrittenBy string    `json:"writtenBy"`
	WrittenAt time.Time `json:"writtenAt"`
}

// readStartNote reads the note of the approved Engagement Request that
// made engagementID, or nil where there was none. A Staff member who has
// since left reads as activity.DepartedStaffName, as in Activity.
func readStartNote(ctx context.Context, tx *sql.Tx, engagementID string) (*StartNote, error) {
	var note StartNote
	var writer sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT r.note, rs.name, r.requested_at
		   FROM engagement_requests r
		   LEFT JOIN staff rs ON rs.id = r.requested_by
		  WHERE r.engagement_id = $1 AND r.note IS NOT NULL
		  LIMIT 1`,
		engagementID,
	).Scan(&note.Text, &writer, &note.WrittenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no note is an answer, not a failure
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: read start note: %w", err)
	}
	note.WrittenBy = staffDisplayName(writer)
	return &note, nil
}

// readRecordFlags answers Detail.HasCarePlan, HasBirthPlan and HasContract.
func readRecordFlags(ctx context.Context, tx *sql.Tx, d *Detail) error {
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'care_plan'),
		        EXISTS (SELECT 1 FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'birth_plan'),
		        EXISTS (SELECT 1 FROM contracts WHERE engagement_id = $1)`,
		d.EngagementID,
	).Scan(&d.HasCarePlan, &d.HasBirthPlan, &d.HasContract); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagement: read record flags: %w", err)
	}
	return nil
}

// hasDoulaFees answers Detail.HasDoulaFees with the rule
// feepayment.listFees lists by: a granted Attachment carrying a fee.
func hasDoulaFees(ctx context.Context, tx *sql.Tx, engagementID string) (bool, error) {
	var has bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM engagement_attachments
		                 WHERE engagement_id = $1 AND origin = 'granted' AND fee_amount_cents IS NOT NULL)`,
		engagementID,
	).Scan(&has); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagement: has doula fees: %w", err)
	}
	return has, nil
}

// AgreedFee is a Doula's own agreed fee, the terms she accepted it on
// (#2269: the Offer's terms, copied onto the Attachment as fee_terms),
// and the Practice's calendar day she accepted it on (ADR-0036: a day
// means the Practice's day).
type AgreedFee struct {
	AmountCents int64   `json:"amountCents"`
	Terms       *string `json:"terms,omitempty"`
	AcceptedOn  string  `json:"acceptedOn"`
}

// readOwnFee reads staffID's own open, granted Attachment on
// engagementID for a fee. The partial unique index allows one open
// Attachment per pair at most, so this is one row or none.
func readOwnFee(ctx context.Context, tx *sql.Tx, engagementID, staffID string) (*AgreedFee, error) {
	var fee AgreedFee
	err := tx.QueryRowContext(ctx,
		`SELECT ea.fee_amount_cents, NULLIF(ea.fee_terms, ''), (ea.attached_at AT TIME ZONE p.timezone)::date::text
		   FROM engagement_attachments ea
		   JOIN engagements e ON e.id = ea.engagement_id
		   JOIN practices p ON p.id = e.practice_id
		  WHERE ea.engagement_id = $1 AND ea.staff_id = $2
		    AND ea.origin = 'granted' AND ea.ended_at IS NULL
		    AND ea.fee_amount_cents IS NOT NULL`,
		engagementID, staffID,
	).Scan(&fee.AmountCents, &fee.Terms, &fee.AcceptedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no fee is an answer, not a failure
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("engagement: read own fee: %w", err)
	}
	return &fee, nil
}

// The sentences a contractor reads where her Engagement was (#2270),
// with the Practice's day her Attachment ended. A birth is named by its
// due month, the one fact her Your offers keeps for it (#2383), read off
// her accepted Offer so an Erasure takes it exactly when it takes that.
const (
	msgBirthAttachmentEnded      = "Your work on the birth due %s ended on %s."
	msgThisBirthAttachmentEnded  = "Your work on this birth ended on %s."
	msgPostpartumAttachmentEnded = "Your postpartum work with this Client ended on %s."
)

// refuseUnreachable answers a reader who cannot reach engagementID. A
// contractor whose own granted Attachment on it has ended is told that
// the work ended and when (#2270), and nothing else about the
// Engagement: the 410 carries her own record only. Everyone else, a
// contractor never on it included, gets the plain 404.
func refuseUnreachable(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID, staffID, engagementID string) {
	var kind string
	var dueDate sql.NullTime
	var endedOn time.Time
	err := tx.QueryRowContext(r.Context(),
		`SELECT e.kind::text,
		        (SELECT o.due_date FROM engagement_offers o
		          WHERE o.engagement_id = ea.engagement_id AND o.staff_id = ea.staff_id AND o.state = 'accepted'
		          ORDER BY o.decided_at DESC LIMIT 1),
		        (ea.ended_at AT TIME ZONE p.timezone)::date
		   FROM engagement_attachments ea
		   JOIN engagements e ON e.id = ea.engagement_id
		   JOIN practices p ON p.id = e.practice_id
		  WHERE ea.engagement_id = $1 AND ea.staff_id = $2 AND e.practice_id = $3
		    AND ea.origin = 'granted' AND ea.ended_at IS NOT NULL
		  ORDER BY ea.ended_at DESC
		  LIMIT 1`,
		engagementID, staffID, practiceID,
	).Scan(&kind, &dueDate, &endedOn)
	if !engagementRead(w, err) {
		return
	}
	ended := endedOn.Format("Jan 2, 2006")
	var message string
	switch {
	case kind == "postpartum":
		message = fmt.Sprintf(msgPostpartumAttachmentEnded, ended)
	case dueDate.Valid:
		message = fmt.Sprintf(msgBirthAttachmentEnded, dueDate.Time.Format("Jan 2006"), ended)
	default:
		message = fmt.Sprintf(msgThisBirthAttachmentEnded, ended)
	}
	apierr.Write(w, http.StatusGone, apierr.CodeAttachmentEnded, message, nil)
}

// DetailHandler views one Engagement's basic detail: every Staff role
// except a contractor Doula without an open, granted attachment on it
// (ADR-0008). Must be mounted behind staffauth.Middleware.
func DetailHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		canAccess, err := c.Reader.CanAccessEngagement(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			refuseUnreachable(w, r, c.Tx, c.PracticeID, c.StaffID, c.EngagementID)
			return
		}

		var d Detail
		var givenName string
		var familyName, preferredName sql.NullString
		var dueDate sql.NullString
		err = c.Tx.QueryRowContext(r.Context(),
			`SELECT e.id, c.id, c.given_name, c.family_name, c.preferred_name, e.status, e.created_at, e.due_date::text,
			        e.kind::text, e.birth_outcome::text, e.pregnancy_ended_on::text
			 FROM engagements e
			 JOIN clients c ON c.id = e.client_id
			 WHERE e.id = $1 AND e.practice_id = $2`,
			c.EngagementID, c.PracticeID,
		).Scan(&d.EngagementID, &d.ClientID, &givenName, &familyName, &preferredName, &d.Status, &d.CreatedAt, &dueDate,
			&d.Kind, &d.BirthOutcome, &d.PregnancyEndedOn)
		if !engagementRead(w, err) {
			return
		}
		d.ClientName = personname.Preferred(givenName, preferredName.String)
		d.ClientDisplayName = personname.Full(givenName, preferredName.String, familyName.String)
		if dueDate.Valid {
			d.DueDate = &dueDate.String
		}
		d.StatusMoves = legalMoves(c.Reader, d.Status)
		birthPlan := BirthPlanInputs{Kind: Kind(d.Kind), BirthOutcome: d.BirthOutcome}
		d.OffersBirthPlanToStart = OffersBirthPlanToStart(birthPlan)
		d.OffersBirthPlan = OffersBirthPlan(birthPlan)

		portalState, err := client.FetchPortalInviteState(r.Context(), c.Tx, d.ClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		d.ClientPortalInviteStatus = portalState.Status
		d.ClientEmailSuppressed = portalState.EmailSuppressed
		d.ClientHasEmail = portalState.HasEmail

		d.ClientsCanPay, err = cardpayments.ClientsCanPay(r.Context(), c.Tx, c.PracticeID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		d.Doulas, err = readAttachedDoulas(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		d.CanAttachSelf = canAttachSelf(c.Reader, c.StaffID, d.Status, d.Doulas)
		d.YourFee, err = readOwnFee(r.Context(), c.Tx, c.EngagementID, c.StaffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := readRecordFlags(r.Context(), c.Tx, &d); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if c.Reader.IsOwnerOrAdmin() {
			d.HasDoulaFees, err = hasDoulaFees(r.Context(), c.Tx, c.EngagementID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			d.StartNote, err = readStartNote(r.Context(), c.Tx, c.EngagementID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, d)
	}, staffauth.EngagementID())
}
