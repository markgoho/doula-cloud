package staffauth_test

import (
	"database/sql"
	"testing"
	"time"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00002_practice_staff_tenancy.sql directly via db.App and set_config,
// bypassing the Go middleware. The middleware tests above prove the HTTP
// contract (401/403/200); these prove the SQL policies themselves scope
// visibility correctly rather than merely agreeing with the middleware by
// coincidence (e.g. a query returning zero rows because no row exists, not
// because RLS hid it).

// seedStaffWithEmail inserts a bare Staff row with an explicit email,
// unlike testdb.SeedStaff which always derives one from identityUID.
// Stays local under this name rather than testdb: #316 matches an
// Invitation to a Practice against staff.email (staff has no unique
// constraint on it), so several tests across this package need a Staff
// row whose email is independent of her identity_uid.
func seedStaffWithEmail(t *testing.T, db *testdb.DB, identityUID, email string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Test Staff', $2, 'NY') RETURNING id`,
		identityUID, email,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return id
}

// seedMembership attaches a doula Membership to an already-seeded Staff
// row. Stays local under this name rather than testdb: these RLS tests
// need a Staff row to exist (and be visible or not under a session var)
// before the Membership is added, a two-step sequence
// testdb.SeedStaffAtPractice's single insert-both call doesn't offer.
func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// TestRLS_StaffFailsClosedWithNoSessionVarsSet proves staff denies all
// rows when neither app.current_practice_id nor app.current_identity_uid
// is set -- the fail-closed case for the table with two OR'd policies,
// where a mistake in either policy could otherwise leak rows.
func TestRLS_StaffFailsClosedWithNoSessionVarsSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	staffID := testdb.SeedStaff(t, db, "fail-closed-staff-uid")
	seedMembership(t, db, practiceID, staffID)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM staff`).Scan(&count); err != nil {
		t.Fatalf("query staff with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_StaffPracticeVisibilityIsScopedToCurrentPractice proves the
// staff_practice_visibility EXISTS-subquery policy narrows the staff
// table to people who hold a membership at app.current_practice_id, not
// to every staff row globally.
func TestRLS_StaffPracticeVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	staffAtA := testdb.SeedStaff(t, db, "staff-at-a")
	staffAtB := testdb.SeedStaff(t, db, "staff-at-b")
	seedMembership(t, db, practiceA, staffAtA)
	seedMembership(t, db, practiceB, staffAtB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM staff`)
	if err != nil {
		t.Fatalf("query staff: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != staffAtA {
		t.Fatalf("visible staff = %v, want only %q", visibleIDs, staffAtA)
	}
}

// TestRLS_StaffSelfVisibilityOnlyAppliesBeforePracticeIsChosen proves the
// staff_self_visibility policy stops applying once
// app.current_practice_id has been set, so a caller's own staff row does
// not leak into a request scoped to a Practice they don't belong to.
func TestRLS_StaffSelfVisibilityOnlyAppliesBeforePracticeIsChosen(t *testing.T) {
	db := testdb.New(t)
	unrelatedPractice := testdb.SeedPractice(t, db, "Unrelated Practice")
	staffID := testdb.SeedStaff(t, db, "self-visibility-uid")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, "self-visibility-uid"); err != nil {
		t.Fatalf("set_config identity: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, staffID).Scan(&count); err != nil {
		t.Fatalf("query staff before practice set: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected caller's own staff row visible before a Practice is chosen, got count = %d", count)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, unrelatedPractice); err != nil {
		t.Fatalf("set_config practice: %v", err)
	}

	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE id = $1`, staffID).Scan(&count); err != nil {
		t.Fatalf("query staff after practice set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected caller's own staff row hidden once scoped to a Practice they don't belong to, got count = %d", count)
	}
}

// TestRLS_PracticeMembershipsVisibilityIsScopedToCurrentPractice proves
// the practice_memberships policy narrows to rows for
// app.current_practice_id, not every membership globally.
func TestRLS_PracticeMembershipsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	staffA := testdb.SeedStaff(t, db, "member-of-a")
	staffB := testdb.SeedStaff(t, db, "member-of-b")
	seedMembership(t, db, practiceA, staffA)
	seedMembership(t, db, practiceB, staffB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_memberships`).Scan(&count); err != nil {
		t.Fatalf("query practice_memberships: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only Practice A's membership visible, got count = %d", count)
	}
}

// TestRLS_MembershipEventsAreScopedToCurrentPractice proves the activity
// table (00051, ADR-0022) is fenced the same way every other
// practice_id-carrying table is: an audit record of who changed whose
// Membership is exactly the kind of row a second Practice must not read.
func TestRLS_MembershipEventsAreScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	mine := testdb.SeedPractice(t, db, "My Practice")
	theirs := testdb.SeedPractice(t, db, "Their Practice")
	staffID := testdb.SeedStaff(t, db, "membership-events-rls")
	seedMembership(t, db, mine, staffID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id)
		 VALUES ($1, 'membership', $2, 'joined', '{}'::jsonb, 'staff', $2)`,
		mine, staffID,
	); err != nil {
		t.Fatalf("seed membership event: %v", err)
	}

	for _, tc := range []struct {
		name       string
		practiceID string
		want       int
	}{
		{"own practice", mine, 1},
		{"another practice", theirs, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.App.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, tc.practiceID); err != nil {
				t.Fatalf("set_config: %v", err)
			}
			var count int
			if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM activity`).Scan(&count); err != nil {
				t.Fatalf("count membership events: %v", err)
			}
			if count != tc.want {
				t.Fatalf("visible membership events = %d, want %d", count, tc.want)
			}
		})
	}
}

// TestRLS_InvitationAcceptLookupNeedsTheTokenDigest proves the narrow
// door 00039 opens for the accept path is exactly as narrow as it claims:
// with no Practice context, an Invitation is visible only to a caller who
// already holds its token, and never to one holding a different one.
func TestRLS_InvitationAcceptLookupNeedsTheTokenDigest(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Invite Lookup Practice")
	ownerID := testdb.SeedStaff(t, db, "invite-lookup-owner")
	seedMembership(t, db, practiceID, ownerID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'lookup@example.com', '{doula}', 'employee', $2, $3, now() + interval '1 day')`,
		practiceID, staffauth.TokenDigest("the-real-token"), ownerID,
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{"no digest set", "", 0},
		{"the wrong token", "some-other-token", 0},
		{"the right token", "the-real-token", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.App.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback() }()
			if tc.token != "" {
				if _, err := tx.ExecContext(t.Context(),
					`SELECT set_config('app.invite_token_digest', $1, true)`, staffauth.TokenDigest(tc.token),
				); err != nil {
					t.Fatalf("set_config: %v", err)
				}
			}
			var count int
			if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_invitations`).Scan(&count); err != nil {
				t.Fatalf("count invitations: %v", err)
			}
			if count != tc.want {
				t.Fatalf("visible invitations = %d, want %d", count, tc.want)
			}
		})
	}
}

// asHerself runs body on a connection standing in exactly the context
// DeleteLoginHandler runs in: her own identity, no Practice chosen, and
// 00033's trusted-worker flag set. The flag is not decoration here -- see
// TestRLS_LoginDeletionNeedsTheTrustedFlagToSeeItsOwnNewRow.
func asHerself(t *testing.T, db *testdb.DB, identityUID string, body func(tx *sql.Tx)) {
	t.Helper()
	inSelfWindow(t, db, identityUID, true, body)
}

// inSelfWindow is asHerself with the trusted flag made optional, so one
// test can prove what happens without it.
func inSelfWindow(t *testing.T, db *testdb.DB, identityUID string, trusted bool, body func(tx *sql.Tx)) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		t.Fatalf("set identity: %v", err)
	}
	if trusted {
		if _, err := tx.ExecContext(t.Context(),
			`SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			t.Fatalf("set trusted flag: %v", err)
		}
	}
	body(tx)
}

// redactingUpdate is the UPDATE redactStaffRow runs, with each column a
// caller controls left as a parameter so a test can bend one of them out
// of shape and watch the policies refuse.
const redactingUpdate = `UPDATE staff SET name = $1, email = $2, identity_uid = $3, deleted_at = $4 WHERE id = $5`

// TestRLS_LoginDeletionPolicyAdmitsTheRedaction pins what 00101's
// staff_self_login_deletion admits and refuses, as herself, in the
// pre-Practice window it is scoped to -- including the one shape it
// cannot refuse, which is worth a test precisely because the migration
// would otherwise read as promising more than it delivers.
func TestRLS_LoginDeletionPolicyAdmitsTheRedaction(t *testing.T) {
	db := testdb.New(t)
	const uid = "rls-login-deletion"
	staffID := testdb.SeedStaff(t, db, uid)
	sentinel := "deleted:" + staffID

	// Refused: the sentinel without the deleted_at stamp. A row that walks
	// away from its own identity while still reading as live is exactly
	// what 00101's WITH CHECK exists to refuse, and 00044 refuses it too.
	asHerself(t, db, uid, func(tx *sql.Tx) {
		if _, err := tx.ExecContext(t.Context(), redactingUpdate,
			staffauth.DeletedStaffName, staffauth.DeletedStaffEmail, sentinel, nil, staffID); err == nil {
			t.Fatal("the policies admitted a sentinel with no deleted_at stamp")
		}
	})

	// Admitted, and this is the limit of what 00101 can promise, recorded
	// here rather than left as a surprise: deleted_at stamped with her
	// identity_uid kept goes through, because 00044's staff_self_update
	// admits it. That policy is row-level by its own stated design ("this
	// permits her to update any column of her own row"), Postgres ORs
	// permissive policies, and narrowing it to close this would take the
	// work-state self-edit down with it, for a write no route exposes.
	//
	// So the pairing of the sentinel and the stamp is redactStaffRow's
	// guarantee -- both in one statement, checked to have affected exactly
	// one row -- and what 00101 adds is the only thing 00044 cannot:
	// admitting the sentinel at all.
	asHerself(t, db, uid, func(tx *sql.Tx) {
		if _, err := tx.ExecContext(t.Context(), redactingUpdate,
			staffauth.DeletedStaffName, staffauth.DeletedStaffEmail, uid, time.Now(), staffID); err != nil {
			t.Fatalf("00044's own whole-row self-update stopped admitting a plain column write: %v", err)
		}
	})

	// Admitted: the whole redaction, the exact shape redactStaffRow writes.
	asHerself(t, db, uid, func(tx *sql.Tx) {
		res, err := tx.ExecContext(t.Context(), redactingUpdate,
			staffauth.DeletedStaffName, staffauth.DeletedStaffEmail, sentinel, time.Now(), staffID)
		if err != nil {
			t.Fatalf("the policy refused the redaction it exists to admit: %v", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver failure, not exercised by unit tests
			t.Fatalf("rows affected: %v", err)
		}
		if affected != 1 {
			t.Fatalf("rows affected = %d, want 1", affected)
		}
	})
}

// TestRLS_LoginDeletionNeedsTheTrustedFlagToSeeItsOwnNewRow pins the one
// coupling in this act that nothing else would record, and that a
// reasonable cleanup would break.
//
// DeleteLoginHandler sets 00033's app.notification_worker_trusted for
// what looks like an unrelated reason -- reading across every Practice
// she belongs to, which no per-Practice policy admits. It turns out the
// redaction cannot commit without it either. Postgres checks this table's
// SELECT policies against the *new* row, and the new row's identity_uid
// is the sentinel, which staff_self_visibility (00006) does not match;
// staff_notification_worker (00033) is the only SELECT policy left that
// admits it, and it admits every row regardless of content.
//
// Without this test, dropping the flag as "only needed for the reads"
// would turn every login deletion into a 500 nothing here explains.
func TestRLS_LoginDeletionNeedsTheTrustedFlagToSeeItsOwnNewRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "rls-login-deletion-untrusted"
	staffID := testdb.SeedStaff(t, db, uid)

	inSelfWindow(t, db, uid, false, func(tx *sql.Tx) {
		if _, err := tx.ExecContext(t.Context(), redactingUpdate,
			staffauth.DeletedStaffName, staffauth.DeletedStaffEmail,
			"deleted:"+staffID, time.Now(), staffID); err == nil {
			t.Fatal("the redaction committed with no trusted flag set -- see this test's comment; the handler's own set_config may have become load-bearing somewhere else, or a new SELECT policy now admits the redacted row")
		}
	})
}

// TestRLS_LoginDeletionPolicyRefusesAnotherPersonsRow is the USING half:
// the endpoint carries no staff id, and neither does the policy admit
// one. A caller who found some other way to name a row still cannot
// reach it.
func TestRLS_LoginDeletionPolicyRefusesAnotherPersonsRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "rls-login-deletion-caller"
	testdb.SeedStaff(t, db, uid)
	victimID := testdb.SeedStaff(t, db, "rls-login-deletion-victim")

	asHerself(t, db, uid, func(tx *sql.Tx) {
		res, err := tx.ExecContext(t.Context(), redactingUpdate,
			staffauth.DeletedStaffName, staffauth.DeletedStaffEmail,
			"deleted:"+victimID, time.Now(), victimID)
		if err != nil {
			// coverage:ignore reason: a refusal here arrives as zero rows rather than an error -- USING filters the row out before any WITH CHECK is consulted
			return
		}
		affected, err := res.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver failure, not exercised by unit tests
			t.Fatalf("rows affected: %v", err)
		}
		if affected != 0 {
			t.Fatalf("rows affected = %d, want the policy to refuse another person's row entirely", affected)
		}
	})
}
