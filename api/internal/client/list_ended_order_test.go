package client_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func setPregnancyEnded(t *testing.T, db *testdb.DB, engagementID, outcome, endedOn string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = NULLIF($3, '')::date WHERE id = $1`,
		engagementID, outcome, endedOn,
	); err != nil {
		t.Fatalf("set pregnancy ended: %v", err)
	}
}

func TestListHandler_LeadsWithBirthsNotYetEndedThenTheNewestEndedPregnancies(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Ended Order Practice")
	const ownerUID = "staff-ended-order-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	// Seeded out of order, so only the ORDER BY can put them right.
	// Ended with a due date still on file: group 2, placed by the day it ended.
	both, bothEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Both", "both@example.com", "active")
	setDueDate(t, db, bothEngagement, "2026-09-01")
	setPregnancyEnded(t, db, bothEngagement, "live_birth", "2026-10-30")

	// Ended with no due date: group 2, the most recent end first.
	ended, endedEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Ended", "ended@example.com", "active")
	setPregnancyEnded(t, db, endedEngagement, "loss", "2026-11-05")

	// Ended long ago: group 2, last of its run.
	oldest, oldestEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Oldest", "oldest@example.com", "active")
	setPregnancyEnded(t, db, oldestEngagement, "live_birth", "2025-01-15")

	// Ended, outcome unknown, no date at all: undated, last.
	unknown, unknownEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Unknown", "unknown@example.com", "active")
	setPregnancyEnded(t, db, unknownEngagement, "unknown", "")

	// Upcoming births lead, the one far out ahead of every ended pregnancy.
	farOut, farOutEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Far Out", "far-out@example.com", "active")
	setDueDate(t, db, farOutEngagement, "2027-06-01")
	pastDue, pastDueEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Past Due", "past-due@example.com", "active")
	setDueDate(t, db, pastDueEngagement, "2026-11-01")
	tomorrow, tomorrowEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Tomorrow", "tomorrow@example.com", "active")
	setDueDate(t, db, tomorrowEngagement, "2026-11-09")

	// A birth not yet ended on one line and an ended pregnancy on another:
	// she takes her best key, the birth, so she leads the ended ones.
	mixed, mixedBirthEngagement := testdb.SeedEngagementInStatus(t, db, practiceID, "Mixed", "mixed@example.com", "active")
	setDueDate(t, db, mixedBirthEngagement, "2027-08-01")
	setPregnancyEnded(t, db, seedEngagementForClient(t, db, mixed, practiceID, "active", "postpartum"), "live_birth", "2026-11-07")

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	got := listWith(t, srv.URL, session, practiceID, "?all=true")
	if want := []string{pastDue, tomorrow, farOut, mixed, ended, both, oldest, unknown}; !slices.Equal(idsInOrder(got), want) {
		t.Fatalf("order = %v, want %v", idsInOrder(got), want)
	}

	byClient := map[string]client.ListItem{}
	for _, item := range got {
		byClient[item.ClientID] = item
	}
	line := byClient[ended].OpenEngagements[0]
	if line.DueDate != nil || line.BirthOutcome == nil || *line.BirthOutcome != "loss" ||
		line.PregnancyEndedOn == nil || *line.PregnancyEndedOn != "2026-11-05" {
		t.Fatalf("ended line = %+v, want outcome loss ended 2026-11-05 and no due date", line)
	}
	line = byClient[both].OpenEngagements[0]
	if line.DueDate == nil || *line.DueDate != "2026-09-01" || line.BirthOutcome == nil || *line.BirthOutcome != "live_birth" ||
		line.PregnancyEndedOn == nil || *line.PregnancyEndedOn != "2026-10-30" {
		t.Fatalf("both line = %+v, want due 2026-09-01, live_birth ended 2026-10-30", line)
	}
	line = byClient[unknown].OpenEngagements[0]
	if line.BirthOutcome == nil || *line.BirthOutcome != "unknown" || line.PregnancyEndedOn != nil {
		t.Fatalf("unknown line = %+v, want outcome unknown and no date", line)
	}
	if line := byClient[tomorrow].OpenEngagements[0]; line.BirthOutcome != nil || line.PregnancyEndedOn != nil {
		t.Fatalf("tomorrow line = %+v, want no outcome and no date", line)
	}
}

func TestListHandler_PagesCrossFromUpcomingBirthsIntoEndedPregnancies(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Group Boundary Practice")
	const ownerUID = "staff-group-boundary-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	// 28 upcoming births, then 5 ended pregnancies and 2 undated Clients, so
	// the first page ends inside the ended run (position 30) and the second
	// holds the rest, the boundary between groups 1 and 2 on the first page.
	const upcoming, ended, undatedCount = 28, 5, 2
	want := make([]string, upcoming)
	for i := range upcoming {
		id, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Upcoming", fmt.Sprintf("upcoming-%d@example.com", i), "active")
		due := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, upcoming-1-i)
		setDueDate(t, db, engagementID, due.Format(time.DateOnly))
		want[upcoming-1-i] = id
	}
	endedIDs := make([]string, ended)
	for i := range ended {
		id, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Ended", fmt.Sprintf("ended-%d@example.com", i), "active")
		day := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		setPregnancyEnded(t, db, engagementID, "live_birth", day.Format(time.DateOnly))
		endedIDs[ended-1-i] = id
	}
	want = append(want, endedIDs...)
	var undated []string
	for i := range undatedCount {
		id, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Undated", fmt.Sprintf("undated-%d@example.com", i), "active")
		undated = append(undated, id)
	}
	slices.Sort(undated)
	want = append(want, undated...)

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	pages := walkList(t, srv.URL, session, practiceID, "?all=true")
	if len(pages) != 2 || len(pages[0]) != 30 || len(pages[1]) != 5 {
		t.Fatalf("pages = %d, want 2 pages of 30 and 5", len(pages))
	}
	if got := idsInOrder(flatten(pages)); !slices.Equal(got, want) {
		t.Fatalf("order across pages = %v, want %v", got, want)
	}
}

func TestListHandler_PagesStayInOrderWhenLinesAreDatedByTheDayThePregnancyEnded(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Ended Paging Practice")
	const ownerUID = "staff-ended-paging-owner"
	testdb.SeedStaffAtPractice(t, db, practiceID, ownerUID, []string{ownerRole}, "employee")

	const ended = 31
	want := make([]string, ended)
	for i := range ended {
		id, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Ended", fmt.Sprintf("ended-%d@example.com", i), "active")
		day := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, ended-1-i)
		setPregnancyEnded(t, db, engagementID, "live_birth", day.Format(time.DateOnly))
		want[i] = id
	}

	srv, session := newServer(t, db, ownerUID)
	defer srv.Close()

	pages := walkList(t, srv.URL, session, practiceID, "?all=true")
	if len(pages) != 2 || len(pages[0]) != 30 || len(pages[1]) != 1 {
		t.Fatalf("pages = %d, want 2 pages of 30 and 1", len(pages))
	}
	if got := idsInOrder(flatten(pages)); !slices.Equal(got, want) {
		t.Fatalf("order across pages = %v, want %v", got, want)
	}
}
