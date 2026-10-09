// Package practicename owns the one change a Practice's name can go
// through after signup (#1540): an Owner states it again.
package practicename

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/staffauth"
)

// actionNameChanged records a change to a Practice's name --
// Practice-scoped and plain-string, the shape practicetimezone's sibling
// action uses.
const actionNameChanged = "practice_name_changed"

// fieldName is PutRequest's own json tag, and so the Details key its
// refusal is written under (docs/api-design.md section 7 rule 4).
const fieldName = "name"

// Response is what PutHandler returns: the name the Practice now holds.
type Response struct {
	Name string `json:"name"`
}

// PutRequest is PutHandler's body. PUT semantics, replacing the one name
// a Practice has.
type PutRequest struct {
	Name string `json:"name"`
}

// nameDiff is PutHandler's activity diff. Both sides are always present:
// the column is NOT NULL.
type nameDiff struct {
	NameBefore string `json:"nameBefore"`
	NameAfter  string `json:"nameAfter"`
}

// PutHandler states a Practice's name. Owner only, declared at the mount.
//
// What the new name reaches, and what it does not:
//
//   - Every later read of practices.name: the Practice session (and so
//     the app's chrome), the Client portal, mail built from the name, the
//     Practice Page's heading.
//   - Contract Drafts. A Draft copies the name into its merge field
//     values when it is created, so the copy is moved here -- but only
//     where it still equals the old name, so a name Staff typed over it
//     by hand stays what Staff wrote.
//   - Not a Sent or Signed Contract. The Client has seen the old name; a
//     Signed PDF is never re-rendered (CONTEXT.md, Contract).
//   - Not the Practice Page's address, which is assigned once and never
//     moved (website.Slugify), and not the statement descriptor at
//     Stripe, which is the Owner's to change in her own Stripe dashboard.
//
// Idempotent by construction: a retry with the same body reads the same
// stored value, writes nothing, and records nothing new.
//
// Must be mounted behind staffauth.Middleware.
func PutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		var req PutRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		after := strings.TrimSpace(req.Name)
		if after == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"name is required", map[string]string{fieldName: staffauth.MsgPracticeNameNeeded})
			return
		}

		var before string
		if err := tx.QueryRowContext(r.Context(),
			`SELECT name FROM practices WHERE id = $1`, practiceID,
		).Scan(&before); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if before != after {
			if err := applyRename(r.Context(), tx, practiceID, before, after); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, Response{Name: after})
	})
}

// applyRename persists the name, moves the Drafts that copied the old one, and
// records who changed it and when -- one write path, so the row and its
// Activity entry can never disagree about what happened.
func applyRename(ctx context.Context, tx *sql.Tx, practiceID, before, after string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE practices SET name = $1 WHERE id = $2`, after, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicename: write practice name: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE contracts
		    SET merge_field_values = jsonb_set(merge_field_values, ARRAY[$1::text], to_jsonb($2::text))
		  WHERE status = 'draft'
		    AND merge_field_values ->> $1 = $3
		    AND engagement_id IN (SELECT id FROM engagements WHERE practice_id = $4)`,
		contracts.PracticeNameMergeKey, after, before, practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicename: move draft contracts to the new name: %w", err)
	}

	diffJSON, err := json.Marshal(nameDiff{NameBefore: before, NameAfter: after})
	if err != nil {
		// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
		return fmt.Errorf("practicename: marshal name diff: %w", err)
	}

	actorStaffID, _ := staffauth.StaffID(ctx)
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectPractice,
		SubjectID:   practiceID,
		Action:      actionNameChanged,
		Diff:        diffJSON,
		Actor:       activity.StaffActor(actorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("practicename: record name change: %w", err)
	}
	return nil
}
