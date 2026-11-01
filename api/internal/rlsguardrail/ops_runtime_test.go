// The third shape this package holds (ADR-0062): the database boundary
// between doula-api's app_runtime and doula-ops's ops_runtime, read from
// the catalog rather than from any one table's rows.
package rlsguardrail_test

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// wantDefinerFunctions is every SECURITY DEFINER function each role may
// execute, written out. A definer function runs with its owner's rights,
// so one that reaches a role by a forgotten REVOKE ... FROM PUBLIC is a
// door the role was never meant to hold.
var wantDefinerFunctions = map[string][]string{
	"app_runtime": {
		"acknowledge_birth_plan(uuid)",
		"client_portal_sees_staff(uuid)",
		"current_staff_id()",
		"engagement_doula_names(uuid)",
		"erase_client_feedback(uuid)",
		"erase_client_idempotency_keys(uuid)",
		"erase_portal_account_feedback(text)",
		"merge_client_portal_links(uuid,uuid)",
		"operator_act_actor_name(uuid)",
		"pilot_admits(text)",
		"portal_account_reaches_a_live_client(text)",
		"portal_account_reuse_for_accept(text,uuid)",
		"push_subscriptions_for_message_recipient(uuid,actor_type)",
		"redact_absorbed_client(uuid)",
		"staff_is_sole_owner(uuid)",
		"staff_was_ever_a_member_at_practice(uuid,uuid)",
		"sweep_expired_idempotency_keys(timestamp with time zone)",
		"sweep_webhook_arrivals(timestamp with time zone)",
	},
	"ops_runtime": {
		"ops_add_pilot_address(text,uuid,timestamp with time zone)",
		"ops_dormant_practices(timestamp with time zone,timestamp with time zone,uuid,integer)",
		"ops_feedback_list(boolean,timestamp with time zone,uuid,integer)",
		"ops_feedback_piece(uuid)",
		"ops_first_signup_at()",
		"ops_founding_grant_practice(uuid)",
		"ops_legal_notice_owner_count()",
		"ops_legal_publications()",
		"ops_monthly_credits(timestamp with time zone,timestamp with time zone)",
		"ops_open_pilot_gate(uuid,timestamp with time zone)",
		"ops_operator_log_names(uuid[],uuid[])",
		"ops_pilot_address_added_at(text)",
		"ops_pilot_gate_opened_at()",
		"ops_publish_legal_version(uuid,text,date,boolean,timestamp with time zone)",
		"ops_record_founding_grant(uuid,uuid,integer,timestamp with time zone)",
		"ops_record_refund(uuid,uuid,integer,integer,text,text,uuid,timestamp with time zone)",
		"ops_refund_by_request_key(uuid,text)",
		"ops_refund_lots(uuid)",
		"ops_record_second_factor_cleared(uuid,uuid,timestamp with time zone)",
		"ops_practice_list(boolean,timestamp with time zone,uuid,integer)",
		"ops_refund_practice(uuid)",
		"ops_second_factor_staff(uuid)",
		"record_operator_activity(uuid,text,uuid,text,jsonb,uuid,timestamp with time zone)",
	},
}

func TestOpsRuntime_DefinerFunctionsEachRoleCanExecuteAreTheOnesOnTheRecord(t *testing.T) {
	db := testdb.New(t)

	for role, want := range wantDefinerFunctions {
		t.Run(role, func(t *testing.T) {
			rows, err := db.Admin.QueryContext(t.Context(),
				`SELECT p.oid::regprocedure::text
				   FROM pg_proc p
				   JOIN pg_namespace n ON n.oid = p.pronamespace
				  WHERE p.prosecdef
				    AND n.nspname NOT IN ('pg_catalog', 'information_schema')
				    AND has_function_privilege($1, p.oid, 'EXECUTE')
				  ORDER BY 1`, role)
			if err != nil {
				t.Fatalf("read definer functions: %v", err)
			}
			defer rows.Close()
			got := []string{}
			for rows.Next() {
				var fn string
				if err := rows.Scan(&fn); err != nil {
					t.Fatalf("scan: %v", err)
				}
				got = append(got, fn)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows: %v", err)
			}

			wantSorted := slices.Sorted(slices.Values(want))
			if !slices.Equal(got, wantSorted) {
				t.Errorf("%s can execute these SECURITY DEFINER functions:\n%s\nwant:\n%s\n(a function off the list is usually a missing REVOKE EXECUTE ... FROM PUBLIC)",
					role, strings.Join(got, "\n"), strings.Join(wantSorted, "\n"))
			}
		})
	}
}

// readsClientsException is the one definer function ops_runtime may
// execute that reads clients: a piece of Feedback, which names a Client
// sender by her own name (ADR-0062's Feedback exception, spec #2464
// story 56).
const readsClientsException = "ops_feedback_piece(uuid)"

// ADR-0062: no definer function ops_runtime can execute reads a Client
// record, but the one Feedback read. ops_runtime holds no grant on
// clients, so a function running with its own rights cannot.
func TestOpsRuntime_NoFunctionItCanExecuteReadsClients(t *testing.T) {
	db := testdb.New(t)

	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT p.oid::regprocedure::text
		   FROM pg_proc p
		   JOIN pg_namespace n ON n.oid = p.pronamespace
		  WHERE p.prosecdef
		    AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		    AND has_function_privilege('ops_runtime', p.oid, 'EXECUTE')
		    AND p.prosrc ~* '\mclients\M'
		  ORDER BY 1`)
	if err != nil {
		t.Fatalf("read functions: %v", err)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, fn)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if !slices.Equal(got, []string{readsClientsException}) {
		t.Errorf("these functions ops_runtime can execute name clients:\n%s\nwant only %s", strings.Join(got, "\n"), readsClientsException)
	}
}

func TestOpsRuntime_NoAppRuntimeRoleIsAMember(t *testing.T) {
	db := testdb.New(t)

	if members := appRuntimeRolesInOpsRuntime(t, db); len(members) > 0 {
		t.Fatalf("these app_runtime roles are members of ops_runtime: %v", members)
	}

	var checked int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM pg_roles WHERE pg_has_role(oid, 'app_runtime', 'MEMBER') AND NOT rolsuper`,
	).Scan(&checked); err != nil {
		t.Fatalf("count app_runtime roles: %v", err)
	}
	if checked < 2 {
		t.Fatalf("checked %d app_runtime roles, want app_runtime and its test login at least", checked)
	}
}

func TestOpsRuntime_AMembershipGrantedToAppRuntimeIsCaught(t *testing.T) {
	db := testdb.New(t)
	if _, err := db.Admin.ExecContext(t.Context(), `GRANT ops_runtime TO app_runtime`); err != nil {
		t.Fatalf("grant: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Admin.ExecContext(context.Background(), `REVOKE ops_runtime FROM app_runtime`); err != nil {
			t.Errorf("revoke: %v", err)
		}
	})

	members := appRuntimeRolesInOpsRuntime(t, db)
	if !slices.Contains(members, "app_runtime") || !slices.Contains(members, "app_test") {
		t.Fatalf("after GRANT ops_runtime TO app_runtime the guard found %v, want app_runtime and app_test", members)
	}
}

// appRuntimeRolesInOpsRuntime is every role that is app_runtime,
// app_runtime_login or a member of app_runtime, and also holds
// ops_runtime. app_runtime_login exists only in Cloud SQL; locally its
// stand-ins are app_runtime's member logins.
func appRuntimeRolesInOpsRuntime(t *testing.T, db *testdb.DB) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT r.rolname
		   FROM pg_roles r
		  WHERE (r.rolname IN ('app_runtime', 'app_runtime_login')
		         OR pg_has_role(r.oid, 'app_runtime', 'MEMBER'))
		    AND NOT r.rolsuper
		    AND pg_has_role(r.oid, 'ops_runtime', 'MEMBER')`)
	if err != nil {
		t.Fatalf("read memberships: %v", err)
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		members = append(members, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return members
}

func TestOpsRuntime_OperatorActsIsAppendOnlyForEveryRole(t *testing.T) {
	db := testdb.New(t)

	var operatorID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO operators (identity_uid, name, email) VALUES ('op-append-only', 'Op', 'op@example.com') RETURNING id`,
	).Scan(&operatorID); err != nil {
		t.Fatalf("seed operator: %v", err)
	}

	var actID string
	if err := db.Ops.QueryRowContext(t.Context(),
		`INSERT INTO operator_acts (operator_id, action) VALUES ($1, 'sign_in') RETURNING id`, operatorID,
	).Scan(&actID); err != nil {
		t.Fatalf("ops_runtime INSERT into operator_acts: %v", err)
	}
	var count int
	if err := db.Ops.QueryRowContext(t.Context(), `SELECT count(*) FROM operator_acts WHERE id = $1`, actID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ops_runtime SELECT on operator_acts: count %d, err %v", count, err)
	}

	connections := map[string]*sql.DB{"ops_runtime": db.Ops, "app_runtime": db.App, "the superuser owner": db.Admin}
	for name, conn := range connections {
		for _, statement := range []string{
			`UPDATE operator_acts SET action = 'sign_out' WHERE id = $1`,
			`DELETE FROM operator_acts WHERE id = $1`,
			`TRUNCATE operator_acts`,
		} {
			var args []any
			if strings.Contains(statement, "$1") {
				args = []any{actID}
			}
			if _, err := conn.ExecContext(t.Context(), statement, args...); err == nil {
				t.Errorf("%s ran %q on operator_acts, want it refused", name, statement)
			}
		}
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM operator_acts WHERE id = $1 AND action = 'sign_in'`, actID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("the act row after every refused change: count %d, err %v", count, err)
	}

	var holders []string
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT r.rolname FROM pg_roles r
		  WHERE NOT r.rolsuper
		    AND r.rolname NOT LIKE 'pg\_%'
		    AND r.oid <> (SELECT relowner FROM pg_class WHERE oid = 'operator_acts'::regclass)
		    AND (has_table_privilege(r.oid, 'operator_acts', 'UPDATE')
		         OR has_table_privilege(r.oid, 'operator_acts', 'DELETE')
		         OR has_table_privilege(r.oid, 'operator_acts', 'TRUNCATE'))`)
	if err != nil {
		t.Fatalf("read privileges: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		holders = append(holders, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(holders) > 0 {
		t.Fatalf("these roles may UPDATE, DELETE or TRUNCATE operator_acts: %v", holders)
	}
}

func TestOpsRuntime_HoldsNoTableGrantButTheOperatorsOwnTables(t *testing.T) {
	db := testdb.New(t)

	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT table_name || ' ' || privilege_type
		   FROM information_schema.role_table_grants
		  WHERE grantee = 'ops_runtime'
		  ORDER BY 1`)
	if err != nil {
		t.Fatalf("read grants: %v", err)
	}
	defer rows.Close()
	got := []string{}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, g)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	want := []string{
		"operator_acts INSERT", "operator_acts SELECT",
		"operator_sessions DELETE", "operator_sessions INSERT", "operator_sessions SELECT", "operator_sessions UPDATE",
		"operators INSERT", "operators SELECT",
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("ops_runtime table grants = %v, want %v", got, want)
	}

	if _, err := db.Ops.ExecContext(t.Context(), `SELECT count(*) FROM practices`); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("ops_runtime read practices: want permission denied, got %v", err)
	}
}

func TestOpsRuntime_AppRuntimeCannotWriteARefundRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Refund Boundary")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")
	var lotID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id)
		 VALUES ($1, 'purchase', 2, 2000, 0, 'pi_boundary') RETURNING id`, practiceID).Scan(&lotID); err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice: %v", err)
	}
	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_refund_id, drawn_lot_id, refund_request_key, operator_act_id)
		 VALUES ($1, 'refund', -1, 2000, 0, 're_boundary', $2, 'key-boundary', $3)`, practiceID, lotID, actID)
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("app_runtime wrote a refund row: want a row-level security refusal, got %v", err)
	}
}

func TestOpsRuntime_AppRuntimeCannotWriteAFoundingGrantRow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Grant Boundary")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice: %v", err)
	}
	_, err = tx.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, operator_act_id) VALUES ($1, 'founding_grant', 3, $2)`, practiceID, actID)
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("app_runtime wrote a founding grant row: want a row-level security refusal, got %v", err)
	}
}

func TestOpsRuntime_AppRuntimeCannotWriteASupportClearRow(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "support-boundary")
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	_, err := db.App.ExecContext(t.Context(),
		`INSERT INTO staff_auth_events (staff_id, actor_operator_act_id, reason) VALUES ($1, $2, 'support')`, staffID, actID)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("app_runtime wrote a support row: want permission denied, got %v", err)
	}

	if _, err := db.App.ExecContext(t.Context(),
		`INSERT INTO staff_auth_events (staff_id, actor_staff_id, reason) VALUES ($1, $1, 'enrolled')`, staffID); err != nil {
		t.Fatalf("app_runtime wrote a Staff member's own row: %v", err)
	}
}
