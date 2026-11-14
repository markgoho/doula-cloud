package oncall

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"
)

// builtRow is one birth's roster row beside the days each Doula is on
// call for it, which the flags are computed from before the row is
// filtered.
type builtRow struct {
	row       RosterWindow
	effective map[string]Window
}

// buildRoster assembles the roster for q from the Engagements filter
// admits. Every Client fact in the response comes from those Engagements
// and nowhere else, so a contractor's filter is the whole of her refusal.
// The flags and the Doula list are computed over everything the filter
// admits, and only then are the births narrowed by staffId and cover.
func buildRoster(ctx context.Context, tx *sql.Tx, practiceID string, settings practiceSettings, q rosterQuery, filter engagementsFilter) (RosterResponse, error) {
	filter.unattendedOn = q.today
	engagements, err := loadEngagements(ctx, tx, practiceID, settings, filter)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}

	resp := RosterResponse{
		From: q.rng.Start, To: q.rng.End, Today: q.today, Rule: settingsOf(settings.rule),
		Windows: []RosterWindow{}, NoWindow: []NoWindowRow{},
	}
	var live []engagementOnCall
	var liveIDs []string
	var noWindow []NoWindowRow
	onHerBirths := map[string]bool{}
	for _, e := range engagements {
		for _, d := range e.doulas {
			onHerBirths[d.staffID] = true
		}
		switch {
		case e.window != nil && e.window.Overlaps(q.rng):
			live = append(live, e)
			liveIDs = append(liveIDs, e.id)
		case e.reason == NoWindowNoDueDate || e.reason == NoWindowNoDueDateNoDoula:
			noWindow = append(noWindow, NoWindowRow{EngagementID: e.id, ClientName: e.clientName, Reason: e.reason, Doulas: staffRefs(e.doulas)})
		}
	}

	from, to := q.rng.Bounds(settings.zone)
	if q.now.After(from) {
		from = q.now
	}
	gaps, err := loadLiveGaps(ctx, tx, liveIDs, from, to)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}

	built := make([]builtRow, 0, len(live))
	for _, e := range live {
		row, effective := rosterWindow(e, q, settings.zone, gaps[e.id])
		built = append(built, builtRow{row: row, effective: effective})
	}
	slices.SortFunc(built, func(a, b builtRow) int { return compareRows(a.row, b.row, q.byCover) })

	doulaAtOnce := markAtOnce(built, q.rng, q.today)
	doulas, err := rosterDoulas(ctx, tx, practiceID, built, doulaAtOnce, filter.contractorStaffID, onHerBirths)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return RosterResponse{}, err
	}
	resp.Doulas = doulas

	for _, b := range built {
		if q.coverNeeded && !b.row.Uncovered {
			continue
		}
		if q.staffID != "" && !involves(b.row, q.staffID) {
			continue
		}
		if filter.contractorStaffID != "" {
			for i := range b.row.OnCall {
				if b.row.OnCall[i].StaffID != filter.contractorStaffID {
					b.row.OnCall[i].AtOnce = nil
				}
			}
		}
		resp.Windows = append(resp.Windows, b.row)
	}
	for _, n := range noWindow {
		if q.coverNeeded || (q.staffID != "" && !slices.ContainsFunc(n.Doulas, func(s StaffRef) bool { return s.StaffID == q.staffID })) {
			continue
		}
		resp.NoWindow = append(resp.NoWindow, n)
	}
	slices.SortFunc(resp.NoWindow, func(a, b NoWindowRow) int {
		return cmp.Or(cmp.Compare(a.ClientName, b.ClientName), cmp.Compare(a.EngagementID, b.EngagementID))
	})
	return resp, nil
}

func staffRefs(doulas []attachedDoula) []StaffRef {
	out := []StaffRef{}
	for _, d := range doulas {
		out = append(out, StaffRef{StaffID: d.staffID, Name: d.name})
	}
	return out
}

// involves reports whether staffID is on call for the row in the range, or
// covers one of its gaps.
func involves(row RosterWindow, staffID string) bool {
	for _, d := range row.OnCall {
		if d.StaffID == staffID {
			return true
		}
	}
	for _, g := range row.Gaps {
		if g.CoveringStaffID != nil && *g.CoveringStaffID == staffID {
			return true
		}
	}
	return false
}

// compareRows is the roster's order: births that include tonight first,
// those with nobody holding tonight ahead of the rest, then due date ascending with none last, then Client name, then id. Under
// order=cover the births needing cover come before all of that.
func compareRows(a, b RosterWindow, byCover bool) int {
	if byCover {
		if c := cmp.Compare(rank(!a.Uncovered), rank(!b.Uncovered)); c != 0 {
			return c
		}
	}
	return cmp.Or(
		cmp.Compare(rank(a.Tonight == nil), rank(b.Tonight == nil)),
		cmp.Compare(rank(!nobodyTonight(a)), rank(!nobodyTonight(b))),
		compareDue(a.DueDate, b.DueDate),
		cmp.Compare(a.ClientName, b.ClientName),
		cmp.Compare(a.EngagementID, b.EngagementID),
	)
}

func nobodyTonight(r RosterWindow) bool {
	return r.Tonight != nil && r.Tonight.IsNobody
}

func rank(later bool) int {
	if later {
		return 1
	}
	return 0
}

func compareDue(a, b *string) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return cmp.Compare(*a, *b)
}

// rosterWindow is one live Engagement's row: the Doulas on call for some
// part of the range, its gaps, its holes, and who holds it tonight. The
// map beside it is each attached Doula's effective days, whether or not
// they reach the range.
func rosterWindow(e engagementOnCall, q rosterQuery, zone *time.Location, loaded []Gap) (RosterWindow, map[string]Window) {
	row := RosterWindow{
		EngagementID: e.id,
		ClientName:   e.clientName,
		DueDate:      e.dueDate,
		Window:       *e.window,
		OwnStart:     e.ownStart,
		NoDoula:      e.noDoula,
		OnCall:       []RosterOnCall{},
		Gaps:         gapsWithin(loaded, *e.window, zone),
	}

	var effectives []Window
	effective := map[string]Window{}
	for _, d := range e.doulas {
		eff, ok := e.window.Effective(d.from, d.to)
		if !ok {
			continue
		}
		effectives = append(effectives, eff)
		effective[d.staffID] = eff
		if eff.Overlaps(q.rng) {
			row.OnCall = append(row.OnCall, RosterOnCall{
				StaffID:  d.staffID,
				Name:     d.name,
				From:     eff.Start,
				To:       eff.End,
				Narrowed: d.from != nil || d.to != nil,
			})
		}
	}

	row.UnstaffedDays = UnstaffedDays(Window{Start: max(e.window.Start, q.today), End: e.window.End}, q.rng, effectives)
	if row.UnstaffedDays == nil {
		row.UnstaffedDays = []Window{}
	}
	row.Uncovered = len(row.UnstaffedDays) > 0
	for _, g := range row.Gaps {
		if g.CoveringStaffID == nil {
			row.Uncovered = true
		}
	}
	row.Tonight = tonightOf(e, q, zone, row.Gaps)
	return row, effective
}

// gapsWithin keeps the gaps that overlap the birth's own days, without the
// reason: the roster draws none, so it sends none. A gap left outside the
// days after the rule moved is not cleared, only not shown here.
func gapsWithin(gaps []Gap, w Window, zone *time.Location) []Gap {
	from, to := w.Bounds(zone)
	out := []Gap{}
	for _, g := range gaps {
		if g.StartsAt.Before(to) && g.EndsAt.After(from) {
			g.Reason = nil
			out = append(out, g)
		}
	}
	return out
}

// tonightOf says who holds the birth for the rest of the Practice's today,
// or nil where today is outside the range or the birth's days. A Doula
// with a gap in force before the day ends does not hold it; the Doula
// covering that gap does, even where her own days do not reach today.
func tonightOf(e engagementOnCall, q rosterQuery, zone *time.Location, gaps []Gap) *Tonight {
	if !q.includesToday() || q.today < e.window.Start || q.today > e.window.End {
		return nil
	}
	t := &Tonight{Holders: []Holder{}}
	endOfDay := DayStart(q.today, zone).AddDate(0, 0, 1)
	index := map[string]int{}
	add := func(id, name string, coveringFor *string) {
		if i, ok := index[id]; ok {
			if coveringFor != nil && t.Holders[i].CoveringFor == nil {
				t.Holders[i].CoveringFor = coveringFor
			}
			return
		}
		index[id] = len(t.Holders)
		t.Holders = append(t.Holders, Holder{StaffID: id, Name: name, CoveringFor: coveringFor})
	}
	for _, d := range e.doulas {
		eff, ok := e.window.Effective(d.from, d.to)
		if !ok || q.today < eff.Start || q.today > eff.End {
			continue
		}
		cannotBeReached := false
		for _, g := range gaps {
			if g.StaffID != d.staffID || !g.StartsAt.Before(endOfDay) || !g.EndsAt.After(q.now) {
				continue
			}
			cannotBeReached = true
			if g.CoveringStaffID != nil {
				name := d.name
				add(*g.CoveringStaffID, *g.CoveringStaffName, &name)
			}
		}
		if !cannotBeReached {
			add(d.staffID, d.name, nil)
		}
	}
	t.IsNobody = len(t.Holders) == 0
	return t
}

// markAtOnce finds, per Doula, every pair of births whose days she is on
// call for at once inside rng, writes the other birth onto each row's
// entry for her, and returns the pairs by Doula.
func markAtOnce(built []builtRow, rng Window, today string) map[string][]DoulaAtOnce {
	type stay struct {
		row  int
		days Window
	}
	stays := map[string][]stay{}
	for i, b := range built {
		for staffID, eff := range b.effective {
			if _, ok := intersect(eff, rng); ok {
				stays[staffID] = append(stays[staffID], stay{row: i, days: Window{Start: max(eff.Start, today), End: eff.End}})
			}
		}
	}

	out := map[string][]DoulaAtOnce{}
	for staffID, list := range stays {
		for a := range list {
			for b := a + 1; b < len(list); b++ {
				days, ok := intersect(list[a].days, list[b].days)
				if !ok {
					continue
				}
				first, second := built[list[a].row].row, built[list[b].row].row
				out[staffID] = append(out[staffID], DoulaAtOnce{Days: days, EngagementIDs: []string{first.EngagementID, second.EngagementID}})
				addAtOnce(&built[list[a].row].row, staffID, second, days)
				addAtOnce(&built[list[b].row].row, staffID, first, days)
			}
		}
	}
	return out
}

func addAtOnce(row *RosterWindow, staffID string, other RosterWindow, days Window) {
	for i := range row.OnCall {
		if row.OnCall[i].StaffID == staffID {
			row.OnCall[i].AtOnce = append(row.OnCall[i].AtOnce, AtOnceBirth{EngagementID: other.EngagementID, ClientName: other.ClientName, Days: days})
		}
	}
}

func intersect(a, b Window) (Window, bool) {
	out := Window{Start: max(a.Start, b.Start), End: min(a.End, b.End)}
	return out, out.Start <= out.End
}

// rosterDoulas lists the Doulas the reader may know and what each does in
// the range. A reader with ambient reach holds every Doula at the
// Practice; a contractor holds only those on one of her births (visible),
// each described by those births alone, and none of their overlaps.
func rosterDoulas(ctx context.Context, tx *sql.Tx, practiceID string, built []builtRow, atOnce map[string][]DoulaAtOnce, contractorStaffID string, visible map[string]bool) ([]RosterDoula, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT s.id, s.name
		   FROM practice_memberships pm
		   JOIN staff s ON s.id = pm.staff_id
		  WHERE pm.practice_id = $1 AND 'doula' = ANY(pm.roles)
		  ORDER BY s.last_name, s.first_name, s.id`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: list roster doulas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := map[string]*RosterDoula{}
	var order []string
	for rows.Next() {
		d := &RosterDoula{Births: []DoulaBirth{}, Unreachable: []Unreachable{}}
		if err := rows.Scan(&d.StaffID, &d.Name); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("oncall: scan roster doula: %w", err)
		}
		if contractorStaffID != "" && !visible[d.StaffID] {
			continue
		}
		byID[d.StaffID] = d
		order = append(order, d.StaffID)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("oncall: iterate roster doulas: %w", err)
	}

	for _, b := range built {
		seen := map[string]bool{}
		for _, o := range b.row.OnCall {
			if d, ok := byID[o.StaffID]; ok {
				d.Births = append(d.Births, DoulaBirth{EngagementID: b.row.EngagementID, ClientName: b.row.ClientName})
				seen[o.StaffID] = true
			}
		}
		for _, g := range b.row.Gaps {
			if d, ok := byID[g.StaffID]; ok {
				d.Unreachable = append(d.Unreachable, Unreachable{
					GapID: g.ID, EngagementID: g.EngagementID, ClientName: b.row.ClientName,
					StartsAt: g.StartsAt, EndsAt: g.EndsAt, IsCovered: g.CoveringStaffID != nil,
				})
			}
			if g.CoveringStaffID == nil || seen[*g.CoveringStaffID] {
				continue
			}
			if d, ok := byID[*g.CoveringStaffID]; ok {
				d.Births = append(d.Births, DoulaBirth{EngagementID: b.row.EngagementID, ClientName: b.row.ClientName, Covering: true})
				seen[*g.CoveringStaffID] = true
			}
		}
		if b.row.Tonight != nil {
			for _, h := range b.row.Tonight.Holders {
				if d, ok := byID[h.StaffID]; ok {
					d.IsOnCallTonight = true
				}
			}
		}
	}

	out := make([]RosterDoula, 0, len(order))
	for _, id := range order {
		d := byID[id]
		slices.SortFunc(d.Unreachable, func(a, b Unreachable) int {
			return cmp.Or(a.StartsAt.Compare(b.StartsAt), cmp.Compare(a.GapID, b.GapID))
		})
		if contractorStaffID == "" || id == contractorStaffID {
			d.AtOnce = atOnce[id]
		}
		out = append(out, *d)
	}
	return out, nil
}
