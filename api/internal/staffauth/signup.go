package staffauth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/pgerr"
)

// MsgInternalError is the response body for any failure the caller can't
// act on (a DB error, an encoding error) -- exported so every handler and
// the middleware, in this package and in main, share one literal instead
// of duplicating it per call site.
const MsgInternalError = "internal error"

// MsgNoMatchingStaffAccount is what a caller whose identity resolves to
// no staff row gets back -- shared across every route that reads or
// writes staff by identity_uid, so the sentence stays one spelling
// rather than drifting per call site.
const MsgNoMatchingStaffAccount = "no matching staff account"

// MsgAlreadyBelongsToPractice is what signup answers a caller whose
// identity already holds a staff row with at least one Membership. The
// wording is aimed at a reader, not at a log line: `refusalMessage` in
// the app prints a 4xx body verbatim, so this sentence is what she sees
// (#745).
const MsgAlreadyBelongsToPractice = "This account already belongs to a Practice. Log in instead."

// SignupRequest is the body of a Practice-signup request: a new Practice,
// created together with the Staff row for the person creating it.
type SignupRequest struct {
	PracticeName string `json:"practiceName"`
	StaffName    string `json:"staffName"`
	StaffEmail   string `json:"staffEmail"`
	// WorkState is the US state this person works from, as a USPS
	// two-letter abbreviation (#415). Required, because New York's sales
	// tax on a Credit purchase is apportioned over where a Practice's
	// people work and a Practice with an unknown member cannot be
	// apportioned at all.
	WorkState string `json:"workState"`
}

// SignupResponse identifies the Practice and Staff row signup created.
type SignupResponse struct {
	StaffID    string `json:"staffId"`
	PracticeID string `json:"practiceId"`
}

// SignupHandler creates a new Practice, a Staff row for the verified
// caller, and a practice_memberships row holding all three roles (Owner,
// Office Manager, Doula), in one transaction. It runs before any Practice
// is known, so -- unlike Middleware -- it never sets
// app.current_practice_id until the new Practice's id exists to set it
// to.
func SignupHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, verified, ok := authn.BeginBootstrap(w, r, verifier, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req SignupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.PracticeName = strings.TrimSpace(req.PracticeName)
		req.StaffName = strings.TrimSpace(req.StaffName)
		req.StaffEmail = strings.TrimSpace(req.StaffEmail)
		if req.PracticeName == "" || req.StaffName == "" || req.StaffEmail == "" {
			http.Error(w, "practiceName, staffName, and staffEmail are required", http.StatusBadRequest)
			return
		}
		workState, ok := NormalizeWorkState(req.WorkState)
		if !ok {
			http.Error(w, MsgWorkStateRequired, http.StatusBadRequest)
			return
		}
		req.WorkState = workState

		resp, status, msg := signup(r, tx, verified.UID, req)
		if status != http.StatusCreated {
			http.Error(w, msg, status)
			return
		}

		// Create the session before committing, so a failure rolls the
		// new rows back instead of leaving them committed behind a
		// response that reports failure (#145). uid is the identity
		// authn.Begin already verified.
		cookie, err := authn.MintSession(r.Context(), tx, verified.UID, time.Now())
		if err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		http.SetCookie(w, cookie)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func signup(r *http.Request, tx *sql.Tx, identityUID string, req SignupRequest) (SignupResponse, int, string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// Signup is answered against what the identity already holds (#745),
	// not against a blank slate. The natural key is identity_uid, and it
	// decides which of three things this call is:
	//
	//   nothing            -- a first signup, the path below. A signup
	//                         whose first attempt failed arrives here
	//                         too: everything below runs in one
	//                         transaction, so a failure leaves no staff
	//                         row to find. What the retry has to get past
	//                         is Identity Platform refusing the address,
	//                         and the signup screen handles that end.
	//   a staff row alone  -- a Staff member whose last Membership was
	//                         removed, starting a Practice of her own.
	//                         Keep the row, give it a Practice.
	//   a staff row with a
	//   Membership         -- she is already somewhere; 409, rather than
	//                         quietly discarding the Practice name she
	//                         just typed. This is also what a retry that
	//                         only *looked* like it failed gets, so a
	//                         second Practice is never built on top of
	//                         one that committed.
	//
	// This lookup runs before app.current_practice_id is set, which is
	// what staff_self_visibility (00002) requires to admit her own row --
	// the same ordering the staff INSERT below depends on.
	resumeStaffID, resumeWorkState, resuming, status, msg := existingStaff(ctx, tx, identityUID)
	if status != 0 {
		return SignupResponse{}, status, msg
	}

	var practiceID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO practices (name) VALUES ($1) RETURNING id`, req.PracticeName).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// staff_self_visibility (00002) only admits the caller's own row while
	// app.current_practice_id is unset -- see that policy's comment -- so
	// the staff INSERT ... RETURNING below must run before it's set. Only
	// after staff exists is app.current_practice_id set, which then covers
	// the signup-bonus, membership, and plan_templates inserts that need
	// it.
	staffID := resumeStaffID
	if !resuming {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, $4) RETURNING id`,
			identityUID, req.StaffName, req.StaffEmail, req.WorkState,
		).Scan(&staffID)
		// Two signups for one identity racing each other, both past
		// existingStaff before either inserted: the loser gets the same
		// answer as a caller who already had a Membership when
		// existingStaff looked. Unreachable in sequence -- the pre-check
		// answers every non-racing duplicate -- so what is left here is a
		// concurrency guard rather than a branch a test can drive.
		// coverage:ignore reason: two concurrent signups for one identity, not exercised by unit tests
		if pgerr.IsUniqueViolation(err) {
			return SignupResponse{}, http.StatusConflict, MsgAlreadyBelongsToPractice
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
		}
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// #613/#169: self-signup sends an email-verification link through
	// ADR-0010's outbox. Accepting a Staff invitation does not -- holding
	// the invite token is already proof of mailbox control (accept.go) --
	// so this is signup's own step, not something shared code does for
	// both bootstrap paths.
	//
	// A resumed signup sends nothing: the staff row already existed, so
	// this address has already been sent its link, and a second one for
	// the same mailbox is noise rather than a step (#745).
	if !resuming {
		verifyToken, err := authtoken.Mint(ctx, tx, identityUID, authtoken.PurposeStaffEmailVerification, authmail.VerificationLinkLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
		}
		if err := authmail.QueueTokenMail(ctx, tx, identityUID, authmail.KindEmailVerification, verifyToken); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
		}
	}

	// The signup bonus: +3 credits, granted exactly once per Practice, in
	// the same transaction as the Practice row itself so a Practice can
	// never exist without it.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{owner,admin,doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// The founding Owner's Membership gets the same 'joined' record every
	// other Membership does (#316), so "how did this person come to hold
	// these roles?" has an answer for her too, not only for people
	// invited later. She is her own actor.
	if err := RecordMembershipEvent(ctx, tx, MembershipEvent{
		PracticeID: practiceID, StaffID: staffID, Type: "joined",
		Roles: "{owner,admin,doula}", EmploymentType: "employee", ActorStaffID: staffID,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// The founding Owner is, by construction, this brand-new Practice's
	// only Owner -- #615's AC: saved recovery codes mint on the
	// Membership event that makes someone a Practice's sole Owner, and a
	// signup is that event too, not only a later promotion.
	if err := reconcileSavedCodes(ctx, tx, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// The first thing ever known about where this person works (#415).
	// Written after the Membership rather than beside the staff INSERT
	// because staff_work_state_events_practice_visibility (00043) admits
	// a row only for someone holding a Membership at the current
	// Practice -- which is true from the statement above and not before.
	//
	// A resumed signup already has one, so what this form collected is a
	// correction rather than a first answer -- and it moves the stored
	// value with it, the way UpdateWorkStateHandler does, so the column
	// and the event log cannot disagree (#745).
	if err := recordSignupPerson(ctx, tx, staffID, req, resumeWorkState, resuming); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, 'care_plan', $2), ($1, 'birth_plan', $3)`,
		practiceID, defaultCarePlanFields, defaultBirthPlanFields,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)`,
		practiceID, defaultContractTemplateProse,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return SignupResponse{StaffID: staffID, PracticeID: practiceID}, http.StatusCreated, ""
}

// defaultCarePlanFields and defaultBirthPlanFields are the field lists a
// new Practice starts with (ADR-0001: seeding applies going forward only,
// no backfill for practices created before this ticket). Each field must
// match the palette and options shape validated server-side by the
// plans package (api/internal/plans/template.go) -- kept as a literal
// here, rather than importing that package, to avoid a staffauth<->plans
// import cycle (plans imports staffauth for its own handlers).
const defaultCarePlanFields = `[
	{"id": "support-people", "type": "section_header", "label": "Support People", "order": 0},
	{"id": "support-people-names", "type": "short_text", "label": "Who will be present for support?", "order": 1},
	{"id": "pain-management", "type": "single_select", "label": "Preferred pain management", "options": ["Unmedicated", "Epidural", "Open to options in the moment"], "order": 2},
	{"id": "special-requests", "type": "long_text", "label": "Special requests or concerns", "order": 3},
	{"id": "share-with-backup", "type": "checkbox", "label": "OK to share this plan with a backup doula", "order": 4}
]`

const defaultBirthPlanFields = `[
	{"id": "birth-setting", "type": "section_header", "label": "Birth Setting", "order": 0},
	{"id": "location", "type": "single_select", "label": "Planned birth location", "options": ["Home", "Birth center", "Hospital"], "order": 1},
	{"id": "notify", "type": "multi_select", "label": "People to notify when labor starts", "options": ["Partner", "Doula", "Midwife", "OB", "Family"], "order": 2},
	{"id": "atmosphere", "type": "long_text", "label": "Preferences for atmosphere (music, lighting, etc.)", "order": 3},
	{"id": "consent-photos", "type": "checkbox", "label": "OK to take photos/video during labor", "order": 4}
]`

// defaultContractTemplateProse is the Contract Template prose a new
// Practice starts with (seeding applies going forward only, no backfill
// for practices created before this ticket). Its merge-field placeholders
// match the tokens the contracts package/settings screen document as
// available -- kept as a literal here, rather than importing the
// contracts package, to avoid a staffauth<->contracts import cycle
// (contracts imports staffauth for its own handlers).
const defaultContractTemplateProse = `This agreement is between {{practice_name}} and {{client_name}} for doula services.

Scope of service: {{scope_of_service}}

Engagement dates: {{engagement_start_date}} through {{engagement_end_date}}

Price: {{price}}

By signing below, both parties agree to the terms described above.`
