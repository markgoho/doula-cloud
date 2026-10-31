package plans

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/personname"
)

// Masthead is what names a Birth Plan wherever it is printed (#2209): on
// the portal page and the Practice-side page, each of which prints, and on
// the PDF either side downloads. A stranger in scrubs handed any of them
// learns whose plan it is, who her Doula is, whose Practice wrote it, and
// whether the copy in hand is current. One reader builds it for all four, so the page and the
// PDF cannot name the plan two ways.
type Masthead struct {
	// ClientName is her legal name, given then family: the document name
	// ADR-0017 keeps for a Contract and an Invoice, and a Birth Plan handed
	// to a hospital is a document too.
	ClientName string `json:"clientName"`
	// Doulas names every Doula on the Engagement, in the order each was
	// put on it: the set the Engagement hub's Doula row names (#2355).
	// Empty, never null, while nobody is on it, and once care has ended.
	Doulas []string `json:"doulas"`
	// PracticeName is the Practice's own name, as it stands today.
	PracticeName string `json:"practiceName"`
	// ChangedOn is the day the plan's answers last changed, or the day it
	// was started if none has (plan_instances.changed_at, 00132), read in
	// the Practice's zone (ADR-0036) and sent as YYYY-MM-DD, the way an
	// Invoice's dueOn is (#1965), so a reader's own zone cannot move it
	// and the page and the PDF print the same day.
	ChangedOn string `json:"changedOn"`
}

// masthead is a Masthead before it is written out: the day it last
// changed is kept as a day, for the PDF to print in its own layout.
type masthead struct {
	clientName   string
	doulas       []string
	practiceName string
	changedOn    time.Time
}

// doulaLine is the masthead's Doula row as the PDF prints it, labeled the
// way the Engagement hub labels its own (engagementDetail.ts's
// doulaSummaryItem), or "" while nobody is on the Engagement.
func (m masthead) doulaLine() string {
	switch len(m.doulas) {
	case 0:
		return ""
	case 1:
		return "Doula: " + m.doulas[0]
	default:
		return "Doulas: " + strings.Join(m.doulas, ", ")
	}
}

// changedOnLayout prints a day the way app/src/lib/dates.ts's
// formatCalendarDay does ("Oct 27, 2026") -- the same layout
// contracts.calendarDayLayout holds, unexported there, so the PDF says what
// the page says.
const changedOnLayout = "Jan 2, 2006"

func (m masthead) dto() *Masthead {
	return &Masthead{ClientName: m.clientName, Doulas: m.doulas, PracticeName: m.practiceName, ChangedOn: m.changedOn.Format(time.DateOnly)}
}

// loadMasthead reads engagementID's Birth Plan masthead on tx, which may be
// a Staff or a Client-portal transaction: each tier's own policies already
// let it read this Engagement's Client and Plan Instance, and practices
// carries no row-level security of its own (practicetimezone.Load). The
// caller has already read the Plan Instance on the same tx, so the row is
// there.
func loadMasthead(ctx context.Context, tx *sql.Tx, engagementID string) (masthead, error) {
	var (
		givenName    string
		familyName   sql.NullString
		practiceName string
		zoneName     string
		changedAt    time.Time
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT c.given_name, c.family_name, p.name, p.timezone, pi.changed_at
		 FROM plan_instances pi
		 JOIN engagements e ON e.id = pi.engagement_id
		 JOIN clients c ON c.id = e.client_id
		 JOIN practices p ON p.id = e.practice_id
		 WHERE pi.engagement_id = $1 AND pi.plan_type = $2`,
		engagementID, birthPlanType,
	).Scan(&givenName, &familyName, &practiceName, &zoneName, &changedAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the caller read this same Plan Instance on this tx a moment before
		return masthead{}, fmt.Errorf("plans: read birth plan masthead: %w", err)
	}
	zone, err := ianazone.Parse(zoneName)
	if err != nil {
		// coverage:ignore reason: practices.timezone is only ever written through ianazone.Parse
		return masthead{}, fmt.Errorf("plans: load practice zone for masthead: %w", err)
	}
	doulas, err := loadDoulaNames(ctx, tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return masthead{}, err
	}
	local := changedAt.In(zone)
	return masthead{
		clientName:   personname.Legal(givenName, familyName.String),
		doulas:       doulas,
		practiceName: practiceName,
		changedOn:    time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC),
	}, nil
}

// loadDoulaNames reads who is on engagementID through
// engagement_doula_names (00134), which either tier's transaction may call:
// engagement_attachments itself is out of a Client-portal transaction's
// reach.
func loadDoulaNames(ctx context.Context, tx *sql.Tx, engagementID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM engagement_doula_names($1)`, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("plans: read masthead doulas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("plans: scan masthead doula: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("plans: iterate masthead doulas: %w", err)
	}
	return names, nil
}

// withMasthead sets out.Masthead when out is a Birth Plan, and leaves a
// Care Plan without one.
func withMasthead(ctx context.Context, tx *sql.Tx, out *InstanceResponse) error {
	if out.PlanType != birthPlanType {
		return nil
	}
	m, err := loadMasthead(ctx, tx, out.EngagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	out.Masthead = m.dto()
	return nil
}
