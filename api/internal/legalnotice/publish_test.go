package legalnotice_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/testdb"
)

// TestPublish_MaterialVersionQueuesOneEmailForEachOwnerOfEachPractice is
// the ticket's last criterion: the clock is moved to publishedAt, a
// material version is published, and one email is queued for each Owner
// of each Practice, then sent by the worker.
func TestPublish_MaterialVersionQueuesOneEmailForEachOwnerOfEachPractice(t *testing.T) {
	db := testdb.New(t)

	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)
	bea := testdb.SeedStaffAtPractice(t, db, harbor, "bea", []string{"owner", "admin"}, employee)
	testdb.SeedStaffAtPractice(t, db, harbor, "cal-doula", []string{"doula"}, "contractor")

	// Ana owns a second Practice: she is told once for each.
	willow := testdb.SeedPractice(t, db, "Willow Birth")
	addOwner(t, db, willow, ana)
	dee := testdb.SeedStaffAtPractice(t, db, willow, "dee", owner, employee)

	// A Practice in its deletion window, and one already deleted, are
	// not told.
	leaving := testdb.SeedPractice(t, db, "Leaving Practice")
	testdb.SeedStaffAtPractice(t, db, leaving, "eve", owner, employee)
	startDeletion(t, db, leaving)
	gone := testdb.SeedPractice(t, db, "Gone Practice")
	testdb.SeedStaffAtPractice(t, db, gone, "fay", owner, employee)
	finishDeletion(t, db, gone)

	srv := newPublishServer(t, db)
	p := publishOK(t, srv, "terms", "2026-12-01")
	if p.Queued != 4 {
		t.Fatalf("queued = %d, want 4 (Ana and Bea at Harbor, Ana and Dee at Willow)", p.Queued)
	}
	if !p.Material || p.PublishedBy != founder || !p.PublishedAt.Equal(publishedAt) {
		t.Fatalf("publication = %+v, want material, by %s, at %s", p, founder, publishedAt)
	}

	rows := outboxRows(t, db)
	type key struct{ practice, staff string }
	want := map[key]bool{{harbor, ana}: true, {harbor, bea}: true, {willow, ana}: true, {willow, dee}: true}
	if len(rows) != len(want) {
		t.Fatalf("outbox rows = %d, want %d", len(rows), len(want))
	}
	for _, r := range rows {
		if !want[key{r.practiceID, r.staffID}] {
			t.Errorf("unexpected row for staff %s at practice %s", r.staffID, r.practiceID)
		}
		if r.document != "terms" || r.version != "2026-12-01" {
			t.Errorf("row names %s %s, want terms 2026-12-01", r.document, r.version)
		}
		if !r.createdAt.Equal(publishedAt) {
			t.Errorf("created_at = %s, want the server's instant %s", r.createdAt, publishedAt)
		}
	}

	sender := &mail.FakeSender{}
	runWorker(t, db, sender, fixtureDocs)
	sent := sender.Sent()
	var to []string
	for _, m := range sent {
		to = append(to, m.To)
		if m.Subject != "DoulaCloud: a new version of the Terms of Service starts on December 1, 2026" {
			t.Errorf("subject = %q", m.Subject)
		}
		if !strings.Contains(m.Text, "What changes: The price of a Credit changes to $40.") {
			t.Errorf("text does not carry the version's one line:\n%s", m.Text)
		}
		if !strings.Contains(m.Text, "https://doula.cloud/terms") {
			t.Errorf("text does not link the new text:\n%s", m.Text)
		}
	}
	sort.Strings(to)
	wantTo := []string{anaAddress, anaAddress, "bea@example.com", "dee@example.com"}
	if strings.Join(to, ",") != strings.Join(wantTo, ",") {
		t.Fatalf("sent to %v, want %v", to, wantTo)
	}
	for _, r := range outboxRows(t, db) {
		if r.status != statusSent {
			t.Errorf("row for %s at %s is %s, want sent", r.staffID, r.practiceID, r.status)
		}
	}

	// Each email names its own Practice, so Ana can tell the two apart.
	var harborText string
	for _, m := range sent {
		if strings.Contains(m.Text, "your practice, Harbor Doulas.") {
			harborText = m.Text
		}
	}
	if !strings.Contains(harborText, testAppBase+"/practices/"+harbor+"/settings/delete") {
		t.Fatalf("Harbor's email does not link Harbor's deletion screen:\n%s", harborText)
	}
}

func TestPublish_NotMaterialRecordsTheActAndQueuesNothing(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)

	p := publishOK(t, newPublishServer(t, db), "privacy", "2026-11-01")
	if p.Material || p.Queued != 0 {
		t.Fatalf("publication = %+v, want not material and nothing queued", p)
	}
	if n := len(outboxRows(t, db)); n != 0 {
		t.Fatalf("outbox rows = %d, want 0", n)
	}

	var by string
	var at time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT published_by, published_at FROM legal_version_publications WHERE document = 'privacy' AND version = '2026-11-01'`,
	).Scan(&by, &at); err != nil {
		t.Fatalf("read publication: %v", err)
	}
	if by != founder || !at.Equal(publishedAt) {
		t.Fatalf("published by %q at %s, want %q at %s", by, at, founder, publishedAt)
	}
}

// A material version fewer than 30 days out is refused, and nothing is
// recorded: the Terms promise each Owner 30 days.
func TestPublish_RefusesAMaterialVersionFewerThan30DaysOut(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)

	status, _ := publish(t, newPublishServer(t, db), testSecret,
		`{"document":"terms","version":"2026-11-15","publishedBy":"Mark"}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
	var n int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM legal_version_publications`).Scan(&n); err != nil {
		t.Fatalf("count publications: %v", err)
	}
	if n != 0 || len(outboxRows(t, db)) != 0 {
		t.Fatalf("publications = %d, outbox rows = %d, want none of either", n, len(outboxRows(t, db)))
	}
}

// Publishing a version twice keeps the first record and queues nothing
// more: an Owner is told once.
func TestPublish_TwiceKeepsTheFirstRecord(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)
	srv := newPublishServer(t, db)

	first := publishOK(t, srv, "terms", "2026-12-01")
	status, _ := publish(t, srv, testSecret, `{"document":"terms","version":"2026-12-01","publishedBy":"Somebody Else"}`)
	if status != http.StatusOK {
		t.Fatalf("second publish: status = %d, want 200", status)
	}
	if n := len(outboxRows(t, db)); n != 1 {
		t.Fatalf("outbox rows = %d, want 1", n)
	}
	var by string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT published_by FROM legal_version_publications WHERE document = 'terms'`).Scan(&by); err != nil {
		t.Fatalf("read publication: %v", err)
	}
	if by != first.PublishedBy {
		t.Fatalf("published_by = %q, want the first record's %q", by, first.PublishedBy)
	}
}

func TestPublish_RefusesWhatItCannotPublish(t *testing.T) {
	db := testdb.New(t)
	srv := newPublishServer(t, db)

	cases := []struct {
		name, secret, body string
		want               int
	}{
		{"no internal secret", "", `{"document":"terms","version":"2026-12-01","publishedBy":"Mark"}`, http.StatusUnauthorized},
		{"not JSON", testSecret, `nope`, http.StatusBadRequest},
		{"unknown document", testSecret, `{"document":"cookies","version":"2026-12-01","publishedBy":"Mark"}`, http.StatusBadRequest},
		{"unknown version", testSecret, `{"document":"terms","version":"2027-01-01","publishedBy":"Mark"}`, http.StatusBadRequest},
		{"nobody named", testSecret, `{"document":"terms","version":"2026-12-01","publishedBy":"  "}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := publish(t, srv, c.secret, c.body); got != c.want {
				t.Fatalf("status = %d, want %d", got, c.want)
			}
		})
	}
}
