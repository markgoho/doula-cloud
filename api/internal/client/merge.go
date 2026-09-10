package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
)

// MergeRequest is gate two's "This is her" act (ADR-0017's amendment,
// #727): the full-object shape EditRequest already uses -- what was
// typed for the record at OtherClientID's path, not yet saved -- plus
// the id of the Client it was found to collide with. There is no
// Override here: gate two's other answer, "No, a different person", is
// EditHandler's existing Override, retried against the plain edit path,
// never this one.
type MergeRequest struct {
	Record
	OtherClientID string `json:"otherClientId"`
}

// MergeResponse is what a merge returns: the survivor's folded record,
// the id of the record absorbed into it, and a count of what moved. The
// counts are the response's own audit -- a Practice reading them can see
// that an Engagement changed hands, without the response ever naming a
// value that belonged to the absorbed woman.
type MergeResponse struct {
	Record
	MergedFrom string      `json:"mergedFrom"`
	Moved      movedCounts `json:"moved"`
}

// MergeHandler merges two Client records into one (#813, ADR-0039).
// Direction never depends on which record is open: the record carrying
// history survives, and where the two are alike the older row survives
// -- resolveMergeDirection. clientId on the path is the record open for
// editing; its freshly typed values (the request body) are what would
// have been saved had gate two not fired.
//
// Everything that follows the woman moves to the survivor: her
// Engagements, her Engagement Requests and her accepted portal accounts.
// A pending invitation is revoked rather than moved. Two things
// deliberately do not move, and neither is an omission: her data key and
// the diffs sealed under it (ADR-0027 and ADR-0022's append-only rule --
// the merged Client carries two audit trails permanently), and her
// Stripe Customers (00076 grants no UPDATE on that table at all). Both
// are reached by erasure instead, which shreds both keys and deletes
// both records' Customers.
//
// Refused, with 409, when both records hold a pending Engagement Request
// of one kind (00042's unique index would otherwise raise a constraint
// violation on the move); when OtherClientID is already erased
// (ADR-0027: never a merge target) or already merged into a third row
// (no chains); or when the two ids are the same. Must be mounted behind
// staffauth.Middleware.
func MergeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}

		sourceOnFile, err := fetchRecord(r.Context(), tx, practiceID, clientID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		sourceErased, err := isErased(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if sourceErased {
			apierr.WriteError(w, "this client's data has been erased and cannot be edited", http.StatusConflict)
			return
		}
		sourceMergedInto, err := readMergedInto(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if sourceMergedInto != nil {
			// The retry-safety case the route's idempotency exemption
			// names: a second merge of the same source 409s here rather
			// than setMergedInto's UPDATE silently matching zero rows
			// under clients_update's USING clause.
			apierr.WriteError(w, "this client record has already been merged into another", http.StatusConflict)
			return
		}

		var req MergeRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if !normalizeAndValidate(w, &req.Record) {
			return
		}
		req.ID = clientID
		otherClientID := req.OtherClientID
		if !staffauth.ParseUUID(w, "otherClient", otherClientID) {
			return
		}
		if otherClientID == clientID {
			apierr.WriteError(w, "a client cannot be merged into herself", http.StatusConflict)
			return
		}

		canAccessOther, err := reader.CanAccessClient(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccessOther {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}
		otherOnFile, err := fetchRecord(r.Context(), tx, practiceID, otherClientID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		otherMergedInto, err := readMergedInto(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if otherMergedInto != nil {
			apierr.WriteError(w, "that client record has already been merged into another", http.StatusConflict)
			return
		}
		otherErased, err := isErased(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if otherErased {
			apierr.WriteError(w, "an erased client cannot be a merge target", http.StatusConflict)
			return
		}

		sourceAttached, err := isAttachedRecord(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		otherAttached, err := isAttachedRecord(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		sourceCreatedAt, err := clientCreatedAt(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		otherCreatedAt, err := clientCreatedAt(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// survivorTyped is what fold's precedence rule runs against for the
		// survivor's side -- the record open for editing contributes its
		// freshly typed values (not yet saved) rather than its stale
		// on-file copy, since those typed values are what this save would
		// have written had gate two not fired. survivorOnFile is the row's
		// actual, currently-saved state, which the audit diff must compare
		// against -- using survivorTyped there would hide the true size of
		// the change this write makes.
		var survivorID, absorbedID string
		var survivorTyped, survivorOnFile, absorbedRecord Record
		if resolveMergeDirection(sourceAttached, otherAttached, sourceCreatedAt, otherCreatedAt) {
			survivorID, absorbedID = otherClientID, clientID
			survivorTyped, survivorOnFile, absorbedRecord = otherOnFile, otherOnFile, req.Record
		} else {
			survivorID, absorbedID = clientID, otherClientID
			survivorTyped, survivorOnFile, absorbedRecord = req.Record, sourceOnFile, otherOnFile
		}

		// Asked before anything is written. Two pending Requests of one
		// kind cannot live under one Client
		// (engagement_requests_one_pending, 00042), and a merge that
		// discovered that at the UPDATE could only report it as a 500.
		if kind, err := pendingRequestKindCollision(r.Context(), tx, absorbedID, survivorID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		} else if kind != "" {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition,
				"both records have a pending "+kind+" request: decide one of them before merging", nil)
			return
		}

		// The tombstone goes first, and that ordering is load-bearing
		// rather than tidy. The narrowed engagements_freeze_outcome
		// trigger and both SECURITY DEFINER functions (00112) take this
		// row as their precondition, so the database verifies a merge is
		// under way from a durable fact rather than from a session flag
		// this handler would otherwise have to be trusted to set.
		if err := setMergedInto(r.Context(), tx, absorbedID, survivorID, staffID, time.Now().UTC()); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		moved, err := moveAttachments(r.Context(), tx, absorbedID, survivorID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		merged := fold(absorbedRecord, survivorTyped)
		merged.ID = survivorID
		if err := updateClient(r.Context(), tx, survivorID, merged); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		survivorDiff := diffRecords(survivorOnFile, merged)
		survivorDiff["mergedFrom"] = change{From: nil, To: absorbedID}
		survivorDiff["moved"] = change{From: nil, To: moved}
		if err := recordEvent(r.Context(), tx, practiceID, survivorID, eventMerged, survivorDiff, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(survivorOnFile.Email) != strings.TrimSpace(merged.Email) {
			if err := portalinvite.RevokePending(r.Context(), tx, survivorID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		// Sealed under the absorbed record's own key, which the merge
		// deliberately leaves standing: ADR-0027's key per Client and
		// ADR-0022's append-only activity table together mean her diffs
		// can never be re-sealed under the survivor's key, so the merged
		// Client carries two audit trails permanently and an erasure has
		// to shred both.
		if err := recordEvent(r.Context(), tx, practiceID, absorbedID, eventAbsorbed, map[string]change{
			"mergedInto": {From: nil, To: survivorID},
			"moved":      {From: nil, To: moved},
		}, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, MergeResponse{Record: merged, MergedFrom: absorbedID, Moved: moved})
	})
}

// isAttachedRecord reports whether clientID is "attached": it has an
// Engagement, an Engagement Request, a portal invitation, or a portal
// account. Messages, Contracts, Plan Instances, Visits and Invoices all
// hang off an engagement_id, so they follow the Engagement and are not a
// separate case. A portal invitation and a portal account are the same
// client_portal_users row at two different stages (invite_token set, or
// identity_uid set), so one EXISTS covers both.
//
// Since #813 this decides a merge's direction rather than whether a
// merge is possible at all -- both records may be attached and the merge
// still runs. It is still the question the direction rule asks: the
// record that carries history is the one worth keeping.
//
// An Engagement ended as 'entered_in_error' does not count, and neither
// does the approved Engagement Request that produced it. That pair is
// the point: an Engagement exists only because a Request was approved
// (00042's engagement_requests_approved_has_engagement), so discounting
// the Engagement alone would leave the Request behind and change
// nothing. "This Engagement should never have existed" (00090) is the
// Practice saying one pregnancy was typed twice, and the record holding
// only that is the one that should be absorbed rather than the one that
// should win.
//
// A refused or withdrawn Request still counts. A decision not to take
// somebody on is real history of a real conversation, not a typo.
func isAttachedRecord(ctx context.Context, tx *sql.Tx, clientID string) (bool, error) {
	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT
			EXISTS(
				SELECT 1 FROM engagements
				 WHERE client_id = $1
				   AND ending_reason IS DISTINCT FROM 'entered_in_error'
			)
			OR EXISTS(
				SELECT 1 FROM engagement_requests r
				 WHERE r.client_id = $1
				   AND NOT (
				       r.state = 'approved'
				       AND EXISTS(
				           SELECT 1 FROM engagements e
				            WHERE e.id = r.engagement_id
				              AND e.ending_reason = 'entered_in_error'
				       )
				   )
			)
			OR EXISTS(SELECT 1 FROM client_portal_users WHERE client_id = $1)`,
		clientID,
	).Scan(&attached)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("client: check attached record: %w", err)
	}
	return attached, nil
}

// resolveMergeDirection reports whether the other Client (the one
// FindCollisions found, not the one open for editing) survives. Since
// #813 both records may be attached, so the rule is stated over both
// sides rather than over the other alone: whichever record carries
// history survives, and where the two are alike -- both attached, or
// neither -- the older row survives. Direction never depends on which
// record happens to be open for editing.
func resolveMergeDirection(sourceAttached, otherAttached bool, sourceCreatedAt, otherCreatedAt time.Time) (otherSurvives bool) {
	if otherAttached != sourceAttached {
		return otherAttached
	}
	return otherCreatedAt.Before(sourceCreatedAt)
}

// clientCreatedAt reads clientID's created_at -- what resolveMergeDirection
// compares when both records are unattached.
func clientCreatedAt(ctx context.Context, tx *sql.Tx, clientID string) (time.Time, error) {
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM clients WHERE id = $1`, clientID).Scan(&createdAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row was already read by the caller
		return time.Time{}, fmt.Errorf("client: read created_at: %w", err)
	}
	return createdAt, nil
}

// readMergedInto reads clientID's merged_into, nil when she has not been
// absorbed into another record.
func readMergedInto(ctx context.Context, tx *sql.Tx, clientID string) (*string, error) {
	var mergedInto sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT merged_into FROM clients WHERE id = $1`, clientID).Scan(&mergedInto); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row was already read by the caller
		return nil, fmt.Errorf("client: read merged_into: %w", err)
	}
	if !mergedInto.Valid {
		return nil, nil //nolint:nilnil // a nil *string is this function's real, valid "not absorbed" answer, not a swallowed error -- every caller already branches on err first
	}
	return &mergedInto.String, nil
}

// setMergedInto tombstones clientID by pointing it at survivorID, and
// stamps the plaintext audit of the act alongside it -- who did it and
// when, in columns that outlive the shredding of both data keys
// (00112). The three are written in one statement because they must be:
// clients_update's USING clause (00080) carries merged_into IS NULL, so
// this is the last write app_runtime will ever make to this row, and
// clients_merge_is_dated (00112) refuses a tombstone with no date.
//
// clients_update's own WITH CHECK (00080) re-verifies same-Practice,
// no-chain and not-erased independently -- this is the one write site
// that sets the column, not the one place those rules are enforced.
//
// It runs BEFORE anything moves. The narrowed engagements_freeze_outcome
// trigger (00112) admits a client_id change only where this row already
// says it was absorbed into the destination, so the database verifies a
// merge from a durable tombstone rather than from a caller's word.
func setMergedInto(ctx context.Context, tx *sql.Tx, clientID, survivorID, staffID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE clients SET merged_into = $2, merged_at = $3, merged_by_staff_id = $4 WHERE id = $1`,
		clientID, survivorID, now, nullIfEmpty(staffID),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: set merged_into: %w", err)
	}
	return nil
}

// movedCounts is what a merge moved off the absorbed record, by table.
// It is the survivor's activity diff's honest account of the act -- a
// count, never a value -- and it is what the endpoint's own response
// says changed hands.
type movedCounts struct {
	Engagements        int `json:"engagements"`
	EngagementRequests int `json:"engagementRequests"`
	PortalAccounts     int `json:"portalAccounts"`
	// RevokedInvitations counts pending invitations the merge revoked
	// rather than moved -- a different act from the three above, and
	// counted separately so the diff never reads as though an invitation
	// changed hands.
	RevokedInvitations int `json:"revokedInvitations"`
}

// moveAttachments re-points everything that follows the woman from the
// absorbed record onto the survivor. Called only after setMergedInto,
// because the trigger and the definer function both read that tombstone
// as their precondition.
//
// Every statement's row count is checked against what was there to move.
// Under RLS a write that matches nothing succeeds and reports nothing,
// so a silent zero is the failure mode this whole function is written
// against -- and for client_portal_users it is not hypothetical: no
// Staff-facing UPDATE policy admits an accepted row at all.
func moveAttachments(ctx context.Context, tx *sql.Tx, absorbedID, survivorID string) (movedCounts, error) {
	var moved movedCounts

	engagements, err := moveRows(ctx, tx,
		`UPDATE engagements SET client_id = $2 WHERE client_id = $1`, absorbedID, survivorID)
	if err != nil {
		return movedCounts{}, err
	}
	moved.Engagements = engagements

	requests, err := moveRows(ctx, tx,
		`UPDATE engagement_requests SET client_id = $2 WHERE client_id = $1`, absorbedID, survivorID)
	if err != nil {
		return movedCounts{}, err
	}
	moved.EngagementRequests = requests

	// A pending invitation is revoked rather than moved. It is
	// re-sendable, moving one would collide with
	// client_portal_users_one_pending_per_client (00026) whenever the
	// survivor holds her own, and the address it was sent to may not even
	// be the address the fold kept.
	revoked, err := moveRows(ctx, tx,
		`UPDATE client_portal_users
		    SET invite_token = NULL, invite_token_expires_at = NULL
		  WHERE client_id = $1 AND identity_uid IS NULL AND invite_token IS NOT NULL`,
		absorbedID)
	if err != nil {
		return movedCounts{}, err
	}
	moved.RevokedInvitations = revoked
	if revoked > 0 {
		if err := portalinvite.RevokePending(ctx, tx, absorbedID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return movedCounts{}, err
		}
	}

	// The accepted rows, through 00112's SECURITY DEFINER door. It
	// returns its own row count, and refuses outright unless the
	// tombstone above already names this survivor.
	var links int
	if err := tx.QueryRowContext(ctx,
		`SELECT merge_client_portal_links($1, $2)`, absorbedID, survivorID,
	).Scan(&links); err != nil {
		return movedCounts{}, fmt.Errorf("client: move portal links: %w", err)
	}
	moved.PortalAccounts = links

	return moved, nil
}

// moveRows runs one of the merge's UPDATEs and reports how many rows it
// touched, so a caller can tell a move that had nothing to do from a
// move a policy refused in silence.
func moveRows(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("client: move rows: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: lib/pq always reports RowsAffected, not exercised by unit tests
		return 0, fmt.Errorf("client: count moved rows: %w", err)
	}
	return int(n), nil
}

// pendingRequestKindCollision names a kind for which both records hold a
// pending Engagement Request. engagement_requests_one_pending (00042) is
// a unique index on (client_id, kind) where state = 'pending', so moving
// one onto the other would raise a constraint violation the caller could
// only report as a 500. Asked before anything is written, so the merge
// is refused in words instead.
func pendingRequestKindCollision(ctx context.Context, tx *sql.Tx, a, b string) (string, error) {
	var kind sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT ra.kind::text
		   FROM engagement_requests ra
		   JOIN engagement_requests rb ON rb.kind = ra.kind AND rb.state = 'pending'
		  WHERE ra.client_id = $1 AND ra.state = 'pending' AND rb.client_id = $2
		  LIMIT 1`,
		a, b,
	).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("client: check pending request collision: %w", err)
	}
	return kind.String, nil
}

// fold is the merge value rule ADR-0017's amendment names: a non-blank
// value from the absorbed record wins, a blank never overwrites.
// Mirrors app/src/lib/intakeMerge.ts's mergedEditFields exactly, restated
// for two full Records instead of intake's answers-against-a-match
// shape -- "absorbed" plays the role "typed" plays there, "survivor"
// plays the role "on file" plays there.
func fold(absorbed, survivor Record) Record {
	pick := func(a, b string) string {
		if strings.TrimSpace(a) != "" {
			return a
		}
		return b
	}
	return Record{
		ID:                survivor.ID,
		GivenName:         pick(absorbed.GivenName, survivor.GivenName),
		FamilyName:        pick(absorbed.FamilyName, survivor.FamilyName),
		PreferredName:     pick(absorbed.PreferredName, survivor.PreferredName),
		Email:             pick(absorbed.Email, survivor.Email),
		Phone:             pick(absorbed.Phone, survivor.Phone),
		AddressLine1:      pick(absorbed.AddressLine1, survivor.AddressLine1),
		AddressLine2:      pick(absorbed.AddressLine2, survivor.AddressLine2),
		AddressLocality:   pick(absorbed.AddressLocality, survivor.AddressLocality),
		AddressRegion:     pick(absorbed.AddressRegion, survivor.AddressRegion),
		AddressPostalCode: pick(absorbed.AddressPostalCode, survivor.AddressPostalCode),
		DateOfBirth:       pick(absorbed.DateOfBirth, survivor.DateOfBirth),
		FieldValues:       mergeFieldValues(survivor.FieldValues, absorbed.FieldValues),
	}
}

// mergeFieldValues layers absorbed's Practice-defined values over
// survivor's -- the same "{...onFileValues, ...answers.fieldValues}"
// rule intakeMerge.ts's mergedEditFields applies, restated for two
// stored blobs instead of one blob and one answer map. A field this
// Practice asks today does not erase one it asked last year: an absent
// key on the absorbed side leaves the survivor's own value in place.
func mergeFieldValues(survivor, absorbed json.RawMessage) json.RawMessage {
	merged := map[string]json.RawMessage{}
	_ = json.Unmarshal(survivor, &merged)
	var absorbedValues map[string]json.RawMessage
	_ = json.Unmarshal(absorbed, &absorbedValues)
	maps.Copy(merged, absorbedValues)
	out, err := json.Marshal(merged)
	if err != nil {
		// coverage:ignore reason: a map of string to json.RawMessage always marshals cleanly, not exercised by unit tests
		return []byte("{}")
	}
	return out
}
