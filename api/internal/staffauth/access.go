package staffauth

import (
	"context"
	"database/sql"
	"fmt"
)

// CanAccessEngagement reports whether r's caller may read engagementID's
// Engagement-scoped data (Visits, Messages, Plan Instances, Contract
// scope), per ADR-0008's read table: an Owner or Admin reaches every
// Engagement at the Practice; an employee Doula reaches every Engagement
// at the Practice too (the ambient grant #227 decided); a contractor
// Doula reaches only an Engagement she holds an open, granted attachment
// on -- an accrued-only attachment never reaches (#228: a record of work,
// never a key). Callers translate false into the same "not found"
// response an out-of-practice engagementID gets, so a contractor without
// access can't distinguish "doesn't exist" from "not attached".
func (r Reader) CanAccessEngagement(ctx context.Context, tx *sql.Tx, engagementID string) (bool, error) {
	if r.Has("owner") || r.Has("admin") || !r.IsContractor() {
		return true, nil
	}

	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM engagement_attachments
			WHERE engagement_id = $1 AND staff_id = $2
			  AND origin = 'granted' AND ended_at IS NULL
		)`,
		engagementID, r.staffID,
	).Scan(&attached)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: check engagement attachment: %w", err)
	}
	return attached, nil
}

// CanAccessClient reports whether r's caller may read/edit clientID's
// Client record, per ADR-0017's "edit follows read": an Owner, an Admin,
// or an employee Doula reaches every Client at the Practice (clients_select
// RLS already confines that to the current Practice); a contractor Doula
// reaches only a Client she holds an open, granted attachment to, on any
// of that Client's Engagements -- the same "attached Clients" carve-out
// ADR-0008 gives her everywhere else.
func (r Reader) CanAccessClient(ctx context.Context, tx *sql.Tx, clientID string) (bool, error) {
	if r.Has("owner") || r.Has("admin") || !r.IsContractor() {
		return true, nil
	}

	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM engagement_attachments ea
			JOIN engagements e ON e.id = ea.engagement_id
			WHERE e.client_id = $1 AND ea.staff_id = $2
			  AND ea.origin = 'granted' AND ea.ended_at IS NULL
		)`,
		clientID, r.staffID,
	).Scan(&attached)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: check client attachment: %w", err)
	}
	return attached, nil
}
