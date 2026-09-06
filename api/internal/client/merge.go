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

// MergeHandler absorbs one of two colliding Client records into the
// other. Direction never depends on which record is open (ADR-0017's
// amendment): the unattached one is always absorbed, and where both are
// unattached the older row survives -- resolveMergeDirection. clientId
// on the path is the record open for editing; its freshly typed values
// (the request body) are what would have been saved had gate two not
// fired.
//
// Refused, with 409, when the record open for editing is attached (an
// Engagement, an Engagement Request, a portal invitation or a portal
// account) -- "This is her" is offered only while it has none, per the
// amendment; when OtherClientID is already erased (ADR-0027: never a
// merge target) or already merged into a third row (no chains); or when
// the two ids are the same. Must be mounted behind staffauth.Middleware.
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		sourceErased, err := isErased(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if sourceErased {
			apierr.WriteError(w, "this client's data has been erased and cannot be edited", http.StatusConflict)
			return
		}
		sourceMergedInto, err := readMergedInto(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		otherMergedInto, err := readMergedInto(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if otherMergedInto != nil {
			apierr.WriteError(w, "that client record has already been merged into another", http.StatusConflict)
			return
		}
		otherErased, err := isErased(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if otherErased {
			apierr.WriteError(w, "an erased client cannot be a merge target", http.StatusConflict)
			return
		}

		sourceAttached, err := isAttachedRecord(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if sourceAttached {
			apierr.WriteError(w, "this client record has an engagement, request, or portal account, and cannot be absorbed -- only the record being edited may be, and only while it holds none of those", http.StatusConflict)
			return
		}
		otherAttached, err := isAttachedRecord(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		sourceCreatedAt, err := clientCreatedAt(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		otherCreatedAt, err := clientCreatedAt(r.Context(), tx, otherClientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
		if resolveMergeDirection(otherAttached, sourceCreatedAt, otherCreatedAt) {
			survivorID, absorbedID = otherClientID, clientID
			survivorTyped, survivorOnFile, absorbedRecord = otherOnFile, otherOnFile, req.Record
		} else {
			survivorID, absorbedID = clientID, otherClientID
			survivorTyped, survivorOnFile, absorbedRecord = req.Record, sourceOnFile, otherOnFile
		}

		merged := fold(absorbedRecord, survivorTyped)
		merged.ID = survivorID
		if err := updateClient(r.Context(), tx, survivorID, merged); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		survivorDiff := diffRecords(survivorOnFile, merged)
		survivorDiff["mergedFrom"] = change{From: nil, To: absorbedID}
		if err := recordEvent(r.Context(), tx, practiceID, survivorID, eventMerged, survivorDiff, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(survivorOnFile.Email) != strings.TrimSpace(merged.Email) {
			if err := portalinvite.RevokePending(r.Context(), tx, survivorID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		if err := setMergedInto(r.Context(), tx, absorbedID, survivorID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordEvent(r.Context(), tx, practiceID, absorbedID, eventAbsorbed, map[string]change{
			"mergedInto": {From: nil, To: survivorID},
		}, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(merged); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// isAttachedRecord reports whether clientID is "attached" in ADR-0017's
// amendment sense: it has an Engagement, an Engagement Request, a portal
// invitation, or a portal account. Messages, Contracts, Plan Instances,
// Visits and Invoices all hang off an engagement_id, so they follow the
// Engagement and are not a separate case. A portal invitation and a
// portal account are the same client_portal_users row at two different
// stages (invite_token set, or identity_uid set), so one EXISTS covers
// both.
func isAttachedRecord(ctx context.Context, tx *sql.Tx, clientID string) (bool, error) {
	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT
			EXISTS(SELECT 1 FROM engagements WHERE client_id = $1)
			OR EXISTS(SELECT 1 FROM engagement_requests WHERE client_id = $1)
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
// FindCollisions found, not the one open for editing) survives.
// MergeHandler only reaches this once the record open for editing is
// known unattached, so the two cases ADR-0017's amendment names are
// exhaustive: an attached other survives outright; between two
// unattached records, the older one survives.
func resolveMergeDirection(otherAttached bool, sourceCreatedAt, otherCreatedAt time.Time) (otherSurvives bool) {
	if otherAttached {
		return true
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

// setMergedInto tombstones clientID by pointing it at survivorID.
// clients_update's own WITH CHECK (00080) re-verifies same-Practice,
// no-chain and not-erased independently -- this is the one write site
// that sets the column, not the one place those rules are enforced.
func setMergedInto(ctx context.Context, tx *sql.Tx, clientID, survivorID string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE clients SET merged_into = $2 WHERE id = $1`, clientID, survivorID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: set merged_into: %w", err)
	}
	return nil
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
