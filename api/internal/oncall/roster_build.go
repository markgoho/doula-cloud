package oncall

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
)

// buildRoster assembles the roster for rng from the Engagements filter
// admits. Every Client fact in the response comes from those Engagements
// and nowhere else, so a contractor's filter is the whole of her
// refusal: the Doula list below reads names and availability only.
func buildRoster(ctx context.Context, tx *sql.Tx, practiceID string, settings practiceSettings, rng Window, filter engagementsFilter) (RosterResponse, error) {
	engagements, err := loadEngagements(ctx, tx, practiceID, settings, filter)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}

	resp := RosterResponse{From: rng.Start, To: rng.End, Windows: []RosterWindow{}, NoWindow: []NoWindowRow{}}
	var live []engagementOnCall
	var liveIDs []string
	for _, e := range engagements {
		switch {
		case e.window != nil && e.window.Overlaps(rng):
			live = append(live, e)
			liveIDs = append(liveIDs, e.id)
		case e.reason == NoWindowNoDueDate:
			resp.NoWindow = append(resp.NoWindow, NoWindowRow{EngagementID: e.id, ClientName: e.clientName, Reason: e.reason})
		}
	}

	from, to := rng.Bounds(settings.zone)
	gaps, err := loadLiveGaps(ctx, tx, liveIDs, from, to)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}

	counts := map[string]int{}
	for _, e := range live {
		row := rosterWindow(e, rng, gaps[e.id])
		for _, d := range row.OnCall {
			counts[d.StaffID]++
		}
		resp.Windows = append(resp.Windows, row)
	}
	slices.SortFunc(resp.Windows, func(a, b RosterWindow) int {
		return cmp.Or(cmp.Compare(a.Window.Start, b.Window.Start), cmp.Compare(a.ClientName, b.ClientName))
	})
	slices.SortFunc(resp.NoWindow, func(a, b NoWindowRow) int { return cmp.Compare(a.ClientName, b.ClientName) })

	resp.Doulas, err = rosterDoulas(ctx, tx, practiceID, from, to, counts, filter.contractorStaffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}
	return resp, nil
}

// rosterWindow is one live Engagement's row: the Doulas on call for some
// part of rng, its gaps, and its holes.
func rosterWindow(e engagementOnCall, rng Window, gaps []Gap) RosterWindow {
	row := RosterWindow{
		EngagementID: e.id,
		ClientName:   e.clientName,
		DueDate:      e.dueDate,
		Window:       *e.window,
		OnCall:       []RosterOnCall{},
		Gaps:         gaps,
	}
	if row.Gaps == nil {
		row.Gaps = []Gap{}
	}

	var effectives []Window
	for _, d := range e.doulas {
		effective, ok := e.window.Effective(d.from, d.to)
		if !ok {
			continue
		}
		effectives = append(effectives, effective)
		if effective.Overlaps(rng) {
			row.OnCall = append(row.OnCall, RosterOnCall{
				StaffID:  d.staffID,
				Name:     d.name,
				From:     effective.Start,
				To:       effective.End,
				Narrowed: d.from != nil || d.to != nil,
			})
		}
	}

	row.UnstaffedDays = UnstaffedDays(*e.window, rng, effectives)
	if row.UnstaffedDays == nil {
		row.UnstaffedDays = []Window{}
	}
	row.Uncovered = len(row.UnstaffedDays) > 0
	for _, g := range row.Gaps {
		if g.CoveringStaffID == nil {
			row.Uncovered = true
		}
	}
	return row
}

// rosterDoulas lists every Doula at the Practice with her availability
// over [from, to), and her live-window count where the reader may hold
// it. contractorStaffID is the reading contractor, or empty for a reader
// with ambient reach, who holds every count.
//
// Availability is read across the whole Practice, including Engagements
// a contractor cannot see, and returned as one boolean per person: that
// a colleague cannot be reached tonight is hers to know, and which birth
// it is on is not.
func rosterDoulas(ctx context.Context, tx *sql.Tx, practiceID string, from, to any, counts map[string]int, contractorStaffID string) ([]RosterDoula, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.id, s.name,
		        NOT EXISTS (
		          SELECT 1 FROM engagement_coverage_gaps g
		            JOIN engagements e ON e.id = g.engagement_id
		           WHERE e.practice_id = $1 AND g.staff_id = s.id
		             AND g.cleared_at IS NULL AND g.starts_at < $3 AND g.ends_at > $2)
		   FROM practice_memberships pm
		   JOIN staff s ON s.id = pm.staff_id
		  WHERE pm.practice_id = $1 AND 'doula' = ANY(pm.roles)
		  ORDER BY s.name, s.id`,
		practiceID, from, to,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: list roster doulas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []RosterDoula{}
	for rows.Next() {
		var d RosterDoula
		if err := rows.Scan(&d.StaffID, &d.Name, &d.Available); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("oncall: scan roster doula: %w", err)
		}
		if contractorStaffID == "" || d.StaffID == contractorStaffID {
			n := counts[d.StaffID]
			d.ConcurrentWindows = &n
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: iterate roster doulas: %w", err)
	}
	return out, nil
}
