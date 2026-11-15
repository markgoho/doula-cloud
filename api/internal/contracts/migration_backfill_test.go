package contracts_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/db/migrations"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

const sentAndVoidedMigration = "00178_contracts_sent_and_voided.sql"

// migrationSections reads the Up and Down sections of 00178.
func migrationSections(t *testing.T) (up, down string) {
	t.Helper()
	raw, err := migrations.FS.ReadFile(sentAndVoidedMigration)
	if err != nil {
		t.Fatalf("read %s: %v", sentAndVoidedMigration, err)
	}
	before, after, found := strings.Cut(string(raw), "-- +goose Down")
	if !found {
		t.Fatalf("%s has no Down section", sentAndVoidedMigration)
	}
	return before, after
}

// backfilled is what one Contract's columns read after the migration ran.
type backfilled struct {
	sentAt, voidedAt, steppedAt sql.NullTime
	sentBy, voidedBy            sql.NullString
}

func readBackfilled(t *testing.T, db *testdb.DB, contractID string) backfilled {
	t.Helper()
	var b backfilled
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT sent_at, sent_by, voided_at, voided_by, stepped_at FROM contracts WHERE id = $1`, contractID,
	).Scan(&b.sentAt, &b.sentBy, &b.voidedAt, &b.voidedBy, &b.steppedAt); err != nil {
		t.Fatalf("read backfilled columns: %v", err)
	}
	return b
}

func requireInstant(t *testing.T, what string, got sql.NullTime, want time.Time) {
	t.Helper()
	if !got.Valid || !got.Time.Equal(want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func requireStaff(t *testing.T, what string, got sql.NullString, want string) {
	t.Helper()
	if want == "" {
		if got.Valid {
			t.Errorf("%s = %q, want none", what, got.String)
		}
		return
	}
	if !got.Valid || got.String != want {
		t.Errorf("%s = %v, want %q", what, got, want)
	}
}

// TestMigration00178_BackfillsEachContractFromItsOwnWindow runs 00178 over
// Contracts in the shape they had before it, and proves each one reads the
// Activity of its own window: an Engagement holding a voided Contract and
// its replacement maps each contract_sent to its own Contract, a Contract
// with no Activity falls back to the instants it holds, and a Draft is
// left unsent. Then it proves each CHECK refuses a row that breaks it.
func TestMigration00178_BackfillsEachContractFromItsOwnWindow(t *testing.T) {
	db := testdb.New(t)
	up, down := migrationSections(t)
	if _, err := db.Admin.ExecContext(t.Context(), down); err != nil {
		t.Fatalf("apply Down: %v", err)
	}
	practice := fixture.Practice(t, db)
	sender := fixture.Staff(t, db, practice, fixture.Doula(), fixture.StaffName("Dee Whitlock"))
	voider := fixture.Staff(t, db, practice, fixture.Admin(), fixture.StaffName("Ren Voider"))
	other := fixture.Staff(t, db, practice, fixture.Doula(), fixture.StaffName("Cy Other"))
	t0 := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return t0.AddDate(0, 0, n) }

	contract := func(engagementID, status string, created time.Time, signed any) string {
		t.Helper()
		var id string
		if err := db.Admin.QueryRowContext(t.Context(),
			`INSERT INTO contracts (engagement_id, status, prose, amount_cents, created_at, signed_at)
			 VALUES ($1, $2::contract_status, 'Agreement.', 150000, $3, $4) RETURNING id`,
			engagementID, status, created, signed,
		).Scan(&id); err != nil {
			t.Fatalf("seed old-shape contract: %v", err)
		}
		return id
	}
	activity := func(engagementID, action, staffID string, at time.Time) {
		t.Helper()
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, created_at)
			 VALUES ($1, 'engagement', $2, $3, '{}', 'staff', $4, $5)`,
			practice.ID, engagementID, action, staffID, at,
		); err != nil {
			t.Fatalf("seed activity: %v", err)
		}
	}

	replaced := fixture.Engagement(t, db, practice).ID
	voidedOne := contract(replaced, "voided", t0, day(3))
	replacement := contract(replaced, "sent", day(5), nil)
	activity(replaced, "contract_sent", sender.ID, day(1))
	activity(replaced, "contract_sent", other.ID, day(2))
	activity(replaced, "contract_voided", voider.ID, day(4))
	activity(replaced, "contract_sent", other.ID, day(6))

	signedBare := contract(fixture.Engagement(t, db, practice).ID, "signed", t0, day(2))
	voidedSigned := contract(fixture.Engagement(t, db, practice).ID, "voided", t0, day(1))
	sentBare := contract(fixture.Engagement(t, db, practice).ID, "sent", day(7), nil)
	voidedBare := contract(fixture.Engagement(t, db, practice).ID, "voided", day(8), nil)
	signedNoInstant := contract(fixture.Engagement(t, db, practice).ID, "signed", day(11), nil)
	draftEngagement := fixture.Engagement(t, db, practice).ID
	draft := contract(draftEngagement, "draft", day(9), nil)
	activity(draftEngagement, "contract_sent", sender.ID, day(10))

	if _, err := db.Admin.ExecContext(t.Context(), up); err != nil {
		t.Fatalf("apply Up: %v", err)
	}

	first := readBackfilled(t, db, voidedOne)
	requireInstant(t, "voided one sent_at", first.sentAt, day(1))
	requireStaff(t, "voided one sent_by", first.sentBy, sender.ID)
	requireInstant(t, "voided one voided_at", first.voidedAt, day(4))
	requireStaff(t, "voided one voided_by", first.voidedBy, voider.ID)
	requireInstant(t, "voided one stepped_at", first.steppedAt, day(4))

	second := readBackfilled(t, db, replacement)
	requireInstant(t, "replacement sent_at", second.sentAt, day(6))
	requireStaff(t, "replacement sent_by", second.sentBy, other.ID)
	requireStaff(t, "replacement voided_by", second.voidedBy, "")
	if second.voidedAt.Valid {
		t.Errorf("replacement voided_at = %v, want none", second.voidedAt)
	}
	requireInstant(t, "replacement stepped_at", second.steppedAt, day(6))

	bare := readBackfilled(t, db, signedBare)
	requireInstant(t, "signed sent_at", bare.sentAt, day(2))
	requireStaff(t, "signed sent_by", bare.sentBy, "")
	requireInstant(t, "signed stepped_at", bare.steppedAt, day(2))

	voidedAfterSigning := readBackfilled(t, db, voidedSigned)
	requireInstant(t, "voided after signing sent_at", voidedAfterSigning.sentAt, day(1))
	requireInstant(t, "voided after signing voided_at", voidedAfterSigning.voidedAt, day(1))
	requireStaff(t, "voided after signing voided_by", voidedAfterSigning.voidedBy, "")

	sentOnly := readBackfilled(t, db, sentBare)
	requireInstant(t, "sent with no activity sent_at", sentOnly.sentAt, day(7))

	never := readBackfilled(t, db, voidedBare)
	requireInstant(t, "never signed sent_at", never.sentAt, day(8))
	requireInstant(t, "never signed voided_at", never.voidedAt, day(8))

	filled := readBackfilled(t, db, signedNoInstant)
	requireInstant(t, "signed with no signed_at stepped_at", filled.steppedAt, day(11))

	notSent := readBackfilled(t, db, draft)
	if notSent.sentAt.Valid || notSent.sentBy.Valid || notSent.voidedAt.Valid {
		t.Errorf("draft = %+v, want it left unsent despite an Activity row in its window", notSent)
	}
	requireInstant(t, "draft stepped_at", notSent.steppedAt, day(9))

	t.Run("each CHECK refuses a row that breaks it", func(t *testing.T) {
		now := t0
		cases := []struct {
			name, constraint, status   string
			sentAt, voidedAt, signedAt any
		}{
			{"a sent Contract with no sent_at", "contracts_sent_at_matches_status", statusSent, nil, nil, nil},
			{"a draft with a sent_at", "contracts_sent_at_matches_status", statusDraft, now, nil, nil},
			{"a voided Contract with no voided_at", "contracts_voided_at_matches_status", statusVoided, now, nil, nil},
			{"a signed Contract with a voided_at", "contracts_voided_at_matches_status", statusSigned, now, now, now},
			{"a signed Contract with no signed_at", "contracts_signed_at_matches_status", statusSigned, now, nil, nil},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				_, err := db.Admin.ExecContext(t.Context(),
					`INSERT INTO contracts (engagement_id, status, prose, amount_cents, sent_at, voided_at, signed_at)
					 VALUES ($1, $2::contract_status, 'Agreement.', 150000, $3, $4, $5)`,
					fixture.Engagement(t, db, practice).ID, c.status, c.sentAt, c.voidedAt, c.signedAt)
				if err == nil || !strings.Contains(err.Error(), c.constraint) {
					t.Fatalf("insert error = %v, want it refused by %s", err, c.constraint)
				}
			})
		}
	})
}
