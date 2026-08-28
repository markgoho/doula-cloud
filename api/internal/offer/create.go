package offer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/tasknudge"
)

// CreateRequest is the body of an Offer. Exactly one target is named:
// staffId for an existing doula Membership, or email for someone who may
// not be Staff anywhere yet -- ADR-0008's "one link joins her to the
// Practice and puts the job in front of her at once".
//
// Employment type is never sent: for a staffId target it is read off her
// own Membership, and the email path always joins her as a contractor
// (CONTEXT.md's Offer entry). It is what a person is to the business,
// not something a request body gets to assert on her behalf.
//
// clientFirstInitial, clientArea, and dueDate are typed in by the sender,
// not derived from the Engagement or the Client -- the Offer is a copy
// (#230). The UI may pre-fill them; the row holds what was actually sent.
// The one exception is clientArea left blank: CreateHandler prefills it
// from the Client's address_locality (ADR-0017) before validating, so an
// Admin isn't forced to retype what's already on the Client's record --
// the stored value is still a copy, resolved once at send time, never
// re-derived later.
type CreateRequest struct {
	StaffID            string `json:"staffId"`
	Email              string `json:"email"`
	AmountCents        *int64 `json:"amountCents"`
	Terms              string `json:"terms"`
	ClientFirstInitial string `json:"clientFirstInitial"`
	ClientArea         string `json:"clientArea"`
	DueDate            string `json:"dueDate"`
}

// CreateResponse identifies the Offer. It carries neither the Invitation
// token nor the access code: both are mailed to the invited address and
// nowhere else, so an Owner cannot hand herself a link that bypasses
// proving control of that mailbox -- the rule staffauth.InviteResponse
// already follows.
type CreateResponse struct {
	OfferID   string `json:"offerId"`
	ExpiresAt string `json:"expiresAt"`
}

// CreateHandler makes an Offer of one Engagement's work. Owner or Admin
// only. Must be mounted behind staffauth.Middleware.
//
// enq is ADR-0013's Cloud Tasks nudge for the email path's outbox row,
// registered rather than fired because Middleware's commit -- which
// decides whether that row survives at all -- runs after this handler
// has returned.
func CreateHandler(enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		area, err := resolveClientArea(r.Context(), tx, engagementID, strings.TrimSpace(req.ClientArea))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		req.ClientArea = area
		facts, ok := parseFacts(w, req)
		if !ok {
			return
		}

		if err := requireOpenEngagement(r.Context(), tx, engagementID); err != nil {
			writeEngagementErr(w, err)
			return
		}

		resp, status, msg := create(r.Context(), tx, practiceID, engagementID, actorStaffID, req, facts)
		if status != http.StatusCreated {
			http.Error(w, msg, status)
			return
		}
		if req.StaffID == "" {
			tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.EngagementOffer))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// resolveClientArea returns provided unchanged when the sender typed
// something; otherwise it prefills from the Engagement's Client's
// address_locality (ADR-0017), so an Admin doesn't retype it on every
// send. Still just a copy, per CreateRequest's doc comment: whichever
// value comes back is what parseFacts validates and create() stores --
// the row never re-derives it later.
func resolveClientArea(ctx context.Context, tx *sql.Tx, engagementID, provided string) (string, error) {
	if provided != "" {
		return provided, nil
	}
	var locality sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT c.address_locality FROM clients c
		 JOIN engagements e ON e.client_id = c.id WHERE e.id = $1`,
		engagementID,
	).Scan(&locality); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- requireOpenEngagement already proved the Engagement (and therefore its Client) exists
		return "", fmt.Errorf("offer: resolve client area: %w", err)
	}
	return locality.String, nil
}

// facts is the validated, storable form of an Offer's decidable content.
type facts struct {
	clientFirstInitial string
	clientArea         string
	dueDate            time.Time
	terms              sql.NullString
}

// parseFacts validates the four decidable facts' non-fee half. The fee
// itself is checked against the target's employment type further down,
// where that type is actually known -- for a staffId target it comes off
// her Membership, not off this request.
func parseFacts(w http.ResponseWriter, req CreateRequest) (facts, bool) {
	initial := strings.TrimSpace(req.ClientFirstInitial)
	if initial == "" {
		http.Error(w, "clientFirstInitial is required", http.StatusBadRequest)
		return facts{}, false
	}
	area := strings.TrimSpace(req.ClientArea)
	if area == "" {
		http.Error(w, "clientArea is required", http.StatusBadRequest)
		return facts{}, false
	}
	dueDate, err := time.Parse(time.DateOnly, strings.TrimSpace(req.DueDate))
	if err != nil {
		http.Error(w, "dueDate is required, as YYYY-MM-DD", http.StatusBadRequest)
		return facts{}, false
	}
	terms := strings.TrimSpace(req.Terms)
	return facts{
		clientFirstInitial: initial,
		clientArea:         area,
		dueDate:            dueDate,
		terms:              sql.NullString{String: terms, Valid: terms != ""},
	}, true
}

// checkFee enforces 00030's offer_fee_matches_employment_type in Go,
// before Postgres sees it. The CHECK constraint is the real guarantee,
// but a violation aborts the whole transaction rather than returning a
// row this handler could turn into a clean 400 (staffauth/invite.go
// documents the same Postgres behaviour), so the readable error has to
// be produced here.
func checkFee(employmentType string, amountCents *int64) (int, string) {
	if employmentType == contractorType {
		if amountCents == nil || *amountCents <= 0 {
			return http.StatusBadRequest, "a fee is required when offering work to a contractor"
		}
		return http.StatusOK, ""
	}
	if amountCents != nil {
		return http.StatusBadRequest, "an employee's work carries no per-Engagement fee"
	}
	return http.StatusOK, ""
}

// requireOpenEngagement confirms engagementID is an Engagement at the
// caller's Practice (RLS already scopes the read to it) and that it has
// not completed. Offering work on a completed Engagement is the exact
// state completion's own cascade just finished clearing out.
func requireOpenEngagement(ctx context.Context, tx *sql.Tx, engagementID string) error {
	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&status); err != nil {
		return fmt.Errorf("offer: read engagement status: %w", err)
	}
	if status == "completed" {
		return errEngagementCompleted
	}
	return nil
}

// errEngagementCompleted separates "no such Engagement here" from "that
// Engagement is over", which are a 404 and a 409.
var errEngagementCompleted = errors.New("offer: engagement already completed")

func writeEngagementErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "engagement not found", http.StatusNotFound)
	case errors.Is(err, errEngagementCompleted):
		http.Error(w, "that engagement has completed", http.StatusConflict)
	default:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
	}
}

// create resolves the Offer's target and writes the row. The two target
// paths differ in exactly one thing -- whether an Invitation has to be
// minted first -- and converge on the same insert.
func create(ctx context.Context, tx *sql.Tx, practiceID, engagementID, actorStaffID string, req CreateRequest, f facts) (CreateResponse, int, string) {
	target, status, msg := resolveTarget(ctx, tx, practiceID, actorStaffID, req)
	if status != http.StatusOK {
		return CreateResponse{}, status, msg
	}
	if status, msg := checkFee(target.employmentType, req.AmountCents); status != http.StatusOK {
		return CreateResponse{}, status, msg
	}

	// Anything already open for this target expires on the way past, so a
	// stale row cannot trip one_open_per_staff and refuse a fresh Offer
	// nobody would call a duplicate.
	if err := expireOpen(ctx, tx, byEngagementID, engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return CreateResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	open, err := hasOpenOffer(ctx, tx, engagementID, target)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return CreateResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}
	if open {
		return CreateResponse{}, http.StatusConflict, "that person already has an open offer on this engagement"
	}

	expiresAt := time.Now().Add(Lifetime)
	var offerID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, invitation_id, employment_type, amount_cents, terms,
		      client_first_initial, client_area, due_date, offered_by, expires_at, access_code_digest)
		 VALUES ($1, $2, $3, $4::employment_type, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id`,
		engagementID, target.staffID, target.invitationID, target.employmentType, req.AmountCents, f.terms,
		f.clientFirstInitial, f.clientArea, f.dueDate, actorStaffID, expiresAt, target.accessCodeDigest,
	).Scan(&offerID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return CreateResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
	}

	if target.invitationID.Valid {
		if err := queue(ctx, tx, offerID, target.inviteToken, target.accessCode); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return CreateResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
		}
		// The Invitation's token was just rotated by MintInvitation, so a
		// Staff invitation email still sitting unsent in its own outbox
		// would mail a token that no longer opens anything. Refresh it in
		// place rather than queueing a second one: she is getting the
		// Offer's email, which carries the same link.
		if err := staffinvite.Refresh(ctx, tx, target.invitationID.String, target.inviteToken); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return CreateResponse{}, http.StatusInternalServerError, staffauth.MsgInternalError
		}
	}

	return CreateResponse{OfferID: offerID, ExpiresAt: expiresAt.UTC().Format(time.RFC3339)}, http.StatusCreated, ""
}

// hasOpenOffer reports whether target already holds an open Offer on
// engagementID. 00030's two partial unique indexes enforce this, but a
// constraint violation aborts the transaction rather than producing a
// 409, so the readable answer is read first -- the same reasoning
// checkFee gives.
func hasOpenOffer(ctx context.Context, tx *sql.Tx, engagementID string, target offerTarget) (bool, error) {
	var open bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM engagement_offers
			WHERE engagement_id = $1 AND state = 'offered'
			  AND (staff_id = $2 OR invitation_id = $3)
		)`,
		engagementID, target.staffID, target.invitationID,
	).Scan(&open)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("offer: check open offer: %w", err)
	}
	return open, nil
}
