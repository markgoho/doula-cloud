package stackswitch_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/stackswitch"
	"doula-cloud/api/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

// env is a getenv over a fixed map: what a process with exactly these
// variables set would read.
func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

// stackDB is a database shaped like the e2e stack's: it carries the sim
// schema simclock installs, with the offset row at delta, readable by the
// role the BFF connects as.
func stackDB(t *testing.T, delta string) *testdb.DB {
	t.Helper()
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `
		CREATE SCHEMA sim;
		CREATE TABLE sim.offset_row (
		    id boolean PRIMARY KEY DEFAULT true CHECK (id),
		    delta interval NOT NULL DEFAULT '0'
		);
		INSERT INTO sim.offset_row (id) VALUES (true);
		GRANT USAGE ON SCHEMA sim TO app_runtime;
		GRANT SELECT ON sim.offset_row TO app_runtime;`); err != nil {
		t.Fatalf("install sim schema: %v", err)
	}
	setDelta(t, db, delta)
	return db
}

func setDelta(t *testing.T, db *testdb.DB, delta string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE sim.offset_row SET delta = $1::interval`, delta); err != nil {
		t.Fatalf("set delta: %v", err)
	}
}

// within reports whether got is real time plus offset, give or take the
// time the call took.
func within(t *testing.T, read clock.Clock, offset time.Duration) {
	t.Helper()
	before := clock.Real().Add(offset)
	got := read()
	after := clock.Real().Add(offset)
	if got.Before(before) || got.After(after) {
		t.Fatalf("clock read %v, want between %v and %v", got, before, after)
	}
}

const (
	marker    = "127.0.0.1:9099"
	farFuture = "2099-01-01"
)

func TestClock_UnsetIsRealTimeAndReadsNoDatabase(t *testing.T) {
	// A nil database: the unset switch must not touch one, which is what
	// Cloud Run runs with.
	read, release, err := stackswitch.Clock(t.Context(), env(map[string]string{stackswitch.MarkerVar: marker}), nil)
	if err != nil {
		t.Fatalf("Clock: %v", err)
	}
	within(t, read, 0)
	if err := release(); err != nil {
		t.Fatalf("release: %v", err)
	}
}

// stackClock is the clock on the stack db, with its pool released when
// the test ends.
func stackClock(t *testing.T, db *testdb.DB) clock.Clock {
	t.Helper()
	read, release, err := stackswitch.Clock(t.Context(), env(map[string]string{
		stackswitch.ClockVar:       "1",
		stackswitch.MarkerVar:      marker,
		stackswitch.DatabaseURLVar: db.AppURL,
	}), db.App)
	if err != nil {
		t.Fatalf("Clock: %v", err)
	}
	t.Cleanup(func() { _ = release() })
	return read
}

func TestClock_FollowsTheOffsetRowOnTheStack(t *testing.T) {
	db := stackDB(t, "3 days")
	read := stackClock(t, db)
	within(t, read, 3*24*time.Hour)

	// A jump moves the row; the next read follows it with no restart.
	setDelta(t, db, "40 days 2 hours")
	within(t, read, 40*24*time.Hour+2*time.Hour)
}

func TestClock_KeepsTheLastOffsetWhenTheRowCannotBeRead(t *testing.T) {
	db := stackDB(t, "5 days")
	read := stackClock(t, db)
	within(t, read, 5*24*time.Hour)

	if _, err := db.Admin.ExecContext(t.Context(), `REVOKE SELECT ON sim.offset_row FROM app_runtime`); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	within(t, read, 5*24*time.Hour)
}

func TestClock_RefusesWithoutTheStackMarker(t *testing.T) {
	db := stackDB(t, "0")
	_, _, err := stackswitch.Clock(t.Context(), env(map[string]string{stackswitch.ClockVar: "1"}), db.App)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("Clock without %s = %v, want ErrNotTheStack", stackswitch.MarkerVar, err)
	}
}

func TestClock_RefusesWhenItsOwnPoolCannotReadTheOffset(t *testing.T) {
	db := stackDB(t, "0")
	_, _, err := stackswitch.Clock(t.Context(), env(map[string]string{
		stackswitch.ClockVar:       "1",
		stackswitch.MarkerVar:      marker,
		stackswitch.DatabaseURLVar: "postgres://nobody@127.0.0.1:1/none?connect_timeout=1",
	}), db.App)
	if err == nil {
		t.Fatal("Clock started with a pool that cannot read sim.offset_row")
	}
}

func TestClock_RefusesADatabaseWithNoSimSchema(t *testing.T) {
	// A migrated database with no sim schema is what every deployed
	// database is: simclock refuses to install into one.
	db := testdb.New(t)
	_, _, err := stackswitch.Clock(t.Context(), env(map[string]string{
		stackswitch.ClockVar:  "1",
		stackswitch.MarkerVar: marker,
	}), db.App)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("Clock on a database with no sim schema = %v, want ErrNotTheStack", err)
	}
}

func TestClock_RefusesAValueOtherThanOne(t *testing.T) {
	_, _, err := stackswitch.Clock(t.Context(), env(map[string]string{
		stackswitch.ClockVar:  "true",
		stackswitch.MarkerVar: marker,
	}), nil)
	if err == nil {
		t.Fatal("Clock accepted a value other than 1")
	}
}

var shipped = []legal.Document{legal.Terms, legal.Privacy}

func TestLegal_UnsetIsTheShippedHistory(t *testing.T) {
	got, err := stackswitch.Legal(t.Context(), env(map[string]string{stackswitch.MarkerVar: marker}), nil, shipped)
	if err != nil {
		t.Fatalf("Legal: %v", err)
	}
	if len(got) != 2 || len(got[0].Versions) != len(legal.Terms.Versions) || len(got[1].Versions) != len(legal.Privacy.Versions) {
		t.Fatalf("Legal with the switch unset = %+v, want the shipped history", got)
	}
}

func TestLegal_AppendsAMaterialVersionOnTheStack(t *testing.T) {
	db := stackDB(t, "0")
	got, err := stackswitch.Legal(t.Context(), env(map[string]string{
		stackswitch.MarkerVar: marker,
		stackswitch.LegalVar:  `[{"document":"terms","effective":"2099-01-01","material":true,"change":"Says what a test changed."}]`,
	}), db.App, shipped)
	if err != nil {
		t.Fatalf("Legal: %v", err)
	}
	terms := got[0]
	last := terms.Versions[len(terms.Versions)-1]
	want := legal.Version{Effective: farFuture, Material: true, Change: "Says what a test changed."}
	if last != want {
		t.Fatalf("last Terms version = %+v, want %+v", last, want)
	}
	if len(terms.Versions) != len(legal.Terms.Versions)+1 {
		t.Fatalf("Terms has %d versions, want the shipped %d plus one", len(terms.Versions), len(legal.Terms.Versions))
	}
	if len(got[1].Versions) != len(legal.Privacy.Versions) {
		t.Fatalf("Privacy changed: %+v", got[1].Versions)
	}
	coming := terms.Coming(time.Date(2098, time.December, 31, 23, 0, 0, 0, time.UTC))
	if len(coming) != 1 || coming[0].Effective != farFuture {
		t.Fatalf("Coming on the eve = %+v, want the appended version", coming)
	}
	// The shipped history is a package variable every other reader holds;
	// appending to a copy must leave it as it is.
	if shippedLast := legal.Terms.Versions[len(legal.Terms.Versions)-1]; shippedLast.Effective == farFuture {
		t.Fatal("Legal appended to the shipped legal.Terms")
	}
}

func TestLegal_RefusesWithoutTheStackMarker(t *testing.T) {
	db := stackDB(t, "0")
	_, err := stackswitch.Legal(t.Context(), env(map[string]string{
		stackswitch.LegalVar: `[{"document":"terms","effective":"2099-01-01","material":true,"change":"x"}]`,
	}), db.App, shipped)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("Legal without %s = %v, want ErrNotTheStack", stackswitch.MarkerVar, err)
	}
}

func TestLegal_RefusesADatabaseWithNoSimSchema(t *testing.T) {
	db := testdb.New(t)
	_, err := stackswitch.Legal(t.Context(), env(map[string]string{
		stackswitch.MarkerVar: marker,
		stackswitch.LegalVar:  `[{"document":"terms","effective":"2099-01-01","material":true,"change":"x"}]`,
	}), db.App, shipped)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("Legal on a database with no sim schema = %v, want ErrNotTheStack", err)
	}
}

func TestLegal_RefusesAVersionItCannotPlace(t *testing.T) {
	db := stackDB(t, "0")
	for name, value := range map[string]string{
		"not JSON":         `terms`,
		"an unknown field": `[{"document":"terms","effective":"2099-01-01","material":true,"change":"x","priceRise":"y"}]`,
		"a wrong type":     `[{"document":"terms","effective":"2099-01-01","material":"yes","change":"x"}]`,
		"no such document": `[{"document":"cookies","effective":"2099-01-01","material":true,"change":"x"}]`,
		"not a date":       `[{"document":"terms","effective":"next year","material":true,"change":"x"}]`,
		"not later":        `[{"document":"terms","effective":"2026-09-29","material":true,"change":"x"}]`,
		"no change line":   `[{"document":"terms","effective":"2099-01-01","material":true,"change":""}]`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := stackswitch.Legal(t.Context(), env(map[string]string{
				stackswitch.MarkerVar: marker,
				stackswitch.LegalVar:  value,
			}), db.App, shipped)
			if err == nil {
				t.Fatalf("Legal accepted %s", value)
			}
		})
	}
}

func TestAddressScale_UnsetIsOneAndReadsNoDatabase(t *testing.T) {
	got, err := stackswitch.AddressScale(t.Context(), env(map[string]string{stackswitch.MarkerVar: marker}), nil)
	if err != nil {
		t.Fatalf("AddressScale: %v", err)
	}
	if got != 1 {
		t.Fatalf("AddressScale with the switch unset = %d, want 1", got)
	}
}

func TestAddressScale_ReadsTheScaleOnTheStack(t *testing.T) {
	db := stackDB(t, "0")
	got, err := stackswitch.AddressScale(t.Context(), env(map[string]string{
		stackswitch.MarkerVar:       marker,
		stackswitch.AddressScaleVar: "20",
	}), db.App)
	if err != nil {
		t.Fatalf("AddressScale: %v", err)
	}
	if got != 20 {
		t.Fatalf("AddressScale = %d, want 20", got)
	}
}

func TestAddressScale_RefusesWithoutTheStackMarker(t *testing.T) {
	db := stackDB(t, "0")
	_, err := stackswitch.AddressScale(t.Context(), env(map[string]string{stackswitch.AddressScaleVar: "20"}), db.App)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("AddressScale without %s = %v, want ErrNotTheStack", stackswitch.MarkerVar, err)
	}
}

func TestAddressScale_RefusesADatabaseWithNoSimSchema(t *testing.T) {
	db := testdb.New(t)
	_, err := stackswitch.AddressScale(t.Context(), env(map[string]string{
		stackswitch.MarkerVar:       marker,
		stackswitch.AddressScaleVar: "20",
	}), db.App)
	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("AddressScale on a database with no sim schema = %v, want ErrNotTheStack", err)
	}
}

func TestAddressScale_RefusesAValueThatIsNotAWholeNumberAboveZero(t *testing.T) {
	for _, value := range []string{"x", "2.5", "0", "-3"} {
		t.Run(value, func(t *testing.T) {
			_, err := stackswitch.AddressScale(t.Context(), env(map[string]string{
				stackswitch.MarkerVar:       marker,
				stackswitch.AddressScaleVar: value,
			}), nil)
			if err == nil {
				t.Fatalf("AddressScale accepted %q", value)
			}
		})
	}
}
