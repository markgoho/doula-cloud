package contracts

import (
	"context"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffnotice"
)

// closeOpenVoidRequests marks every request still open against contractID
// voided and tells each Doula who asked (#2953).
func closeOpenVoidRequests(ctx context.Context, c staffauth.Call, contractID string) error {
	closed, err := markOpenRequestsVoided(ctx, c, contractID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	for _, a := range closed {
		if err := queueVoidAnswer(ctx, c, staffnotice.KindVoidGranted, a.id, a.by); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}
	return nil
}

type closedRequest struct{ id, by string }

func markOpenRequestsVoided(ctx context.Context, c staffauth.Call, contractID string) ([]closedRequest, error) {
	rows, err := c.Tx.QueryContext(ctx,
		`UPDATE contract_void_requests
		    SET status = 'voided', decided_by = $1, decided_at = $3
		  WHERE contract_id = $2 AND status = 'open'
		  RETURNING id, requested_by`,
		c.StaffID, contractID, clock.Now(ctx),
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: close void requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var closed []closedRequest
	for rows.Next() {
		var a closedRequest
		if err := rows.Scan(&a.id, &a.by); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan closed void request: %w", err)
		}
		closed = append(closed, a)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate closed void requests: %w", err)
	}
	return closed, nil
}

// PostVoidContractHandler transitions the Contract for :engagementId from
// 'signed' to 'voided' -- the only transition it permits; any other
// current status (including an already-voided Contract) 409s. Voiding is
// a one-way, terminal transition: it never touches
// signed_pdf_object_path or the underlying GCS object, so the Signed PDF
// (#71) is preserved unchanged as the historical record of what was
// originally signed. Staff create a fresh Draft Contract afterward via
// PostContractHandler to capture updated terms -- there is no
// amendment/addendum entity. That recreate is only possible because
// voiding frees the Engagement's slot in contracts_engagement_id_active_key
// (00020_contracts_recreate_after_void.sql), the partial unique index
// that replaced the original table-wide UNIQUE (engagement_id); Void
// itself doesn't touch that index, but every UPDATE here targets the
// fetched row's id rather than engagement_id, so it can never affect any
// of the Engagement's other (already-voided) Contract rows. Owner and
// Admin only, declared at the mount (contracts.Mount,
// staffauth.OwnerAndAdmin) rather than checked here -- #282, #970: every
// Doula, employee or contractor, is refused regardless of what she is
// attached to. Must be mounted through
// idempotency.Router.ExemptGated.
func PostVoidContractHandler() http.Handler {
	return contractHandler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow) {
		if ok, refusal := TransitionVoid.Check(Status(ct.Status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		if !stampTransition(w, r, c, ct, `UPDATE contracts SET status = $1::contract_status, voided_at = $2, voided_by = $3 WHERE id = $4`, StatusVoided, activity.ActionContractVoided) {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return
		}

		// #971: an Owner or an Admin acting on a Doula's ask is exactly
		// this transition, not a second endpoint -- any request still
		// open against this Contract is now fulfilled, so it closes with
		// the Void rather than staying open forever with nothing left to
		// grant. decline_reason stays NULL, which is what tells the
		// requester (VoidRequests on her next GET) she got a void, not a
		// decline.
		if err := closeOpenVoidRequests(r.Context(), c, ct.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		voidRequests, err := listVoidRequests(r.Context(), c.Tx, ct.ID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		mergeFields := extractMergeFields(ct.Prose)
		out := ContractResponse{
			EngagementID:    c.EngagementID,
			Status:          string(StatusVoided),
			Prose:           ct.Prose,
			MergeFields:     mergeFields,
			Values:          withResolvedPrice(mergeFields, ct.Values.nonEmpty(), ct.AmountCents),
			AmountChangedAt: ct.AmountChangedAt,
			VoidRequests:    voidRequests,
		}
		writeContract(w, r, c, http.StatusOK, out)
	}, staffauth.Engagement())
}
