package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/clock"
)

// ClientGlance is the one line the portal's "Your care" page says about
// her Contract (#2007): where it stands, and when she signed it. The
// document itself stays on the Contract page, so the hub does not carry
// its prose to say one sentence about it.
type ClientGlance struct {
	Status   string     `json:"status"`
	SignedAt *time.Time `json:"signedAt,omitempty"`
}

// ClientContractGlance reads the same Contract ClientGetContractHandler
// shows her -- the newest row her RLS lets her see, so a Draft never
// reaches her (contracts_client_visibility). found is false when she has none yet.
// tx must be a Client-portal transaction.
func ClientContractGlance(ctx context.Context, tx *sql.Tx, engagementID string) (glance ClientGlance, found bool, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT status, signed_at FROM contracts
		 WHERE engagement_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
		engagementID,
	).Scan(&glance.Status, &glance.SignedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientGlance{}, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ClientGlance{}, false, fmt.Errorf("contracts: client contract glance: %w", err)
	}
	return glance, true, nil
}

// recordFirstOpen stamps contractID with the Client's first read and records
// it as her act (#2952). The stamp is guarded on opened_at being null, so a
// second read, or two at once, changes nothing and records nothing.
func recordFirstOpen(ctx context.Context, tx *sql.Tx, engagementID, contractID string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE contracts SET opened_at = $1 WHERE id = $2 AND opened_at IS NULL`,
		clock.Now(ctx).Truncate(time.Microsecond), contractID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("contracts: stamp first open: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	reader, err := loadSigningEngagement(ctx, tx, engagementID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("contracts: load opening engagement: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  reader.practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionContractOpened),
		Actor:       activity.ClientActor(reader.clientID),
	}, activity.ScopedTo(reader.practiceID)); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: record contract opened: %w", err)
	}
	return nil
}

// ClientGetContractHandler views the Contract for the Client-portal
// caller's Engagement -- read-only, no PUT/POST route for this
// population; the signing flow itself is #70. clientauth.Middleware has
// already confirmed the caller's Client owns :engagementId; Postgres RLS
// (contracts_client_visibility, 00017_contracts_client_visibility.sql) is
// what actually keeps a Draft Contract unreachable here -- fetchContract
// finds no row and this handler 404s, the same "zero rows, not an
// error" shape plans.ClientGetBirthPlanHandler relies on. Must be
// mounted behind clientauth.Middleware.
func ClientGetContractHandler() http.Handler {
	return clientauth.Handler(func(w http.ResponseWriter, r *http.Request, c clientauth.Call) {
		ct, ok := requireContract(w, r, c.Tx, c.EngagementID)
		if !ok {
			return
		}
		if ok, _ := TransitionOpen.Check(Status(ct.Status)); ok {
			if err := recordFirstOpen(r.Context(), c.Tx, c.EngagementID, ct.ID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		mergeFields := extractMergeFields(ct.Prose)

		out := ContractResponse{
			EngagementID:    c.EngagementID,
			Status:          ct.Status,
			Prose:           ct.Prose,
			MergeFields:     mergeFields,
			Values:          withResolvedPrice(mergeFields, ct.Values.nonEmpty(), ct.AmountCents),
			AmountChangedAt: ct.AmountChangedAt,
		}
		writeClientContract(w, r, c.Tx, http.StatusOK, out)
	})
}
