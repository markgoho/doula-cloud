package staffauth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authmail"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/ianazone"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/sessionmint"
	"doula-cloud/api/internal/tasknudge"
)

// MsgNoMatchingStaffAccount is what a caller whose identity resolves to
// no staff row gets back -- shared across every route that reads or
// writes staff by identity_uid, so the sentence stays one spelling
// rather than drifting per call site.
//
// The status beside it is 404 on every pre-Practice route and 403 behind
// a {practiceId}; requireSelf records why, and is the one place the
// pre-Practice pair is written at all (#1182).
const MsgNoMatchingStaffAccount = "no matching staff account"

// MsgAlreadyBelongsToPractice is what signup answers a caller whose
// identity already holds a staff row with at least one Membership. The
// wording is aimed at a reader, not at a log line: `refusalMessage` in
// the app prints a 4xx body verbatim, so this sentence is what she sees
// (#745).
const MsgAlreadyBelongsToPractice = "This account already belongs to a Practice. Sign in instead."

// MsgNoAddressToCreateAPractice is what signup answers a caller whose ID
// token carries no email claim. staff.email is the address the identity
// authenticated with (#614), so an identity that reports no address has
// nothing to put there -- the same reasoning, and the same 403, that
// MsgNoAddressToAcceptAnInvitation gives the invitee whose provider
// reports none. The two sentences are a pair and differ only in the step
// they name.
//
// "verified" in the sentence is about the ID token, which Identity
// Platform verified, not about email_verified -- at signup the address is
// unconfirmed by construction, and #613's mail is what proves control of
// it later.
const MsgNoAddressToCreateAPractice = "your account has no verified email address, so it cannot create a Practice"

// MsgTimezoneRequired is what a caller who omitted a timezone, or sent
// one the IANA database does not name, reads in APIError.Message. It
// names the JSON field and the format, for a caller reading the API;
// MsgTimezoneNeeded is the sentence beside the control for the person
// filling the form in.
//
// The two names time.LoadLocation answers without consulting the
// database -- "" and "Local" -- are refused with everything else, which
// is what stops an omitted field being read as a deliberate choice of
// UTC. See ianazone.Parse.
const MsgTimezoneRequired = "timezone is needed, and must be an IANA zone name such as America/New_York"

// SignupRequest is the body of a Practice-signup request: a new Practice,
// created together with the Staff row for the person creating it.
//
// It deliberately carries no email address. staff.email comes off the
// verified ID token (#614): it is what a later invitation to this
// Practice is matched against and what #613's verification mail proves
// control of, so it may not be self-asserted -- the same rule
// resolveStaff already follows on the invitation-acceptance path.
type SignupRequest struct {
	PracticeName string `json:"practiceName"`
	// FirstName and LastName are the two fields a Staff member's name is
	// asked in (#1537), both needed.
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	// WorkState is the US state this person works from, as a USPS
	// two-letter abbreviation (#415). Required, because New York's sales
	// tax on a Credit purchase is apportioned over where a Practice's
	// people work and a Practice with an unknown member cannot be
	// apportioned at all.
	WorkState string `json:"workState"`
	// Timezone is the IANA zone name this Practice keeps its calendar
	// days in (#1166, ADR-0036). Stated here rather than inherited from a
	// column default: it decides which day a Visit falls on, and a
	// Practice outside Eastern time that was never asked would have its
	// evening Visits typed against the wrong day from its first one.
	Timezone string `json:"timezone"`
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
func SignupHandler(verifier authn.Verifier, db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
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
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		firstName, lastName, nameDetails := normalizeNames(req.FirstName, req.LastName)
		req.FirstName, req.LastName = firstName, lastName
		if nameDetails != nil {
			// One refusal, one entry per empty field (#488): the two names
			// are asked for on one screen and can be missing together, so
			// answering only the first would send her back twice.
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"firstName and lastName are required", nameDetails)
			return
		}
		// #1536: the Practice name is optional, and an empty one is her own
		// name, first name then last name as she typed them -- never no
		// name, since a Contract, the portal and a card statement each
		// print one and practices.name is NOT NULL. Applied here rather
		// than in the form, so a caller that sends no Practice name gets
		// what the form gets, and before signup() branches, so a resumed
		// signup gets it too.
		req.PracticeName = strings.TrimSpace(req.PracticeName)
		if req.PracticeName == "" {
			req.PracticeName = firstName + " " + lastName
		}
		workState, ok := NormalizeWorkState(req.WorkState)
		if !ok {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgWorkStateRequired,
				map[string]string{fieldWorkState: MsgWorkStateNeeded})
			return
		}
		req.WorkState = workState
		if _, err := ianazone.Parse(req.Timezone); err != nil {
			// Two refusals, not one. "Choose a timezone" is the right
			// sentence for a founder who chose nothing, and the wrong one
			// for a founder whose browser reported a name the IANA
			// database does not carry -- she would read "choose one"
			// beside a control that already has something in it. The
			// second case says what is actually wrong with the name.
			detail := MsgTimezoneNeeded
			if req.Timezone != "" {
				detail = ianazone.MsgNotRecognized
			}
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgTimezoneRequired,
				map[string]string{fieldTimezone: detail})
			return
		}

		step := func(_ context.Context, tx *sql.Tx) (sessionmint.Result, error) {
			resp, refusal := signup(r, tx, verified, req)
			if refusal != nil {
				return sessionmint.Result{Refusal: refusal}, nil
			}
			return sessionmint.Result{IdentityUID: verified.UID, Body: resp, Status: http.StatusCreated}, nil
		}

		committed = sessionmint.Issue(w, r, tx, enq, sessionmint.Staff(verified), step, nil)
	})
}

// signup returns the new Practice, or the refusal sessionmint writes in
// its place -- a refusal rather than a status and a sentence, so the one
// refusal that names its own code (#1790's) can carry it.
func signup(r *http.Request, tx *sql.Tx, verified authn.VerifiedToken, req SignupRequest) (SignupResponse, *sessionmint.Refusal) {
	ctx := r.Context()
	identityUID := verified.UID

	// The address is the token's, normalized exactly as acceptInvite
	// normalizes the invited address, so the two bootstrap paths write
	// one spelling of one address (#614). No claim, no Practice: this is
	// checked before anything is written, so the refusal leaves no rows.
	address := NormalizeAddress(verified.Email)
	if address == "" {
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusForbidden, Message: MsgNoAddressToCreateAPractice}
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
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
		return SignupResponse{}, &sessionmint.Refusal{Status: status, Message: msg}
	}

	// #1790: until the founder opens the pilot gate, only an address on
	// the pilot allowlist creates a Practice. Asked after the 409 above, so
	// somebody who already belongs to a Practice is still told to sign in,
	// and before the first INSERT below, so a refusal leaves no Practice,
	// no Staff row, no Membership and no Credits. A refused person keeps
	// the Identity Platform account the screen made first; once the
	// founder adds the address, the same identity's retry arrives here
	// as a first signup and goes through.
	admitted, err := pilotAdmits(ctx, tx, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}
	if !admitted {
		return SignupResponse{}, pilotNotOpen()
	}

	var practiceID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO practices (name, timezone) VALUES ($1, $2) RETURNING id`,
		req.PracticeName, req.Timezone,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
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
			`INSERT INTO staff (identity_uid, first_name, last_name, email, work_state) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			identityUID, req.FirstName, req.LastName, address, req.WorkState,
		).Scan(&staffID)
		// Two signups for one identity racing each other, both past
		// existingStaff before either inserted: the loser gets the same
		// answer as a caller who already had a Membership when
		// existingStaff looked. Unreachable in sequence -- the pre-check
		// answers every non-racing duplicate -- so what is left here is a
		// concurrency guard rather than a branch a test can drive.
		// coverage:ignore reason: two concurrent signups for one identity, not exercised by unit tests
		if pgerr.IsUniqueViolation(err) {
			return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusConflict, Message: MsgAlreadyBelongsToPractice}
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
		}
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
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
		verifyToken, err := authtoken.Mint(ctx, tx, identityUID, authtoken.PurposeStaffEmailVerification, authmail.VerificationLinkLifetime, clock.Now(ctx))
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
		}
		if err := authmail.QueueTokenMail(ctx, tx, identityUID, authmail.KindEmailVerification, verifyToken); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
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
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{owner,admin,doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	// The founding Owner's Membership gets the same 'joined' record every
	// other Membership does (#316), so "how did this person come to hold
	// these roles?" has an answer for her too, not only for people
	// invited later. She is her own actor.
	if err := RecordMembershipEvent(ctx, tx, MembershipEvent{
		PracticeID: practiceID, StaffID: staffID, Type: activity.ActionMembershipJoined,
		Roles: "{owner,admin,doula}", EmploymentType: "employee", ActorStaffID: staffID,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	// #1547 (ADR-0053): the Practice agrees, through her, to the Terms of
	// Service and the Privacy Policy -- the sentence above the button she
	// just pressed. A resumed signup is a new Practice too, so it records
	// one as well.
	if err := recordTermsAccepted(ctx, tx, r, practiceID, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	// The founding Owner is, by construction, this brand-new Practice's
	// only Owner -- #615's AC: saved recovery codes mint on the
	// Membership event that makes someone a Practice's sole Owner, and a
	// signup is that event too, not only a later promotion. Since #1531
	// that holds only for an Owner who has a second factor, so a
	// first-time signup mints none: reconcileSavedCodes waits for her to
	// enroll. A person who enrolled under an earlier Membership gets her
	// set here.
	if err := reconcileSavedCodes(ctx, tx, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
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
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, 'care_plan', $2), ($1, 'birth_plan', $3)`,
		practiceID, defaultCarePlanFields, defaultBirthPlanFields,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)`,
		practiceID, defaultContractTemplateProse,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, &sessionmint.Refusal{Status: http.StatusInternalServerError, Message: apierr.MsgInternalError}
	}

	return SignupResponse{StaffID: staffID, PracticeID: practiceID}, nil
}

// defaultCarePlanFields and defaultBirthPlanFields are the field lists a
// new Practice starts with (ADR-0001: seeding applies going forward only,
// no backfill for practices created before this ticket). Each field must
// match the palette and options shape validated server-side by the
// plans package (api/internal/plans/template.go) -- kept as a literal
// here, rather than importing that package, to avoid a staffauth<->plans
// import cycle (plans imports staffauth for its own handlers).
//
// Each plan is chunked under section_headers by what its questions are
// about, so that no group asks more than seven decisions at once, a
// choose-several question counted one per box (MO-G14, #2195):
// TestSignupHandler_SeededPlansShowNoGroupOverSeven holds it.
const defaultCarePlanFields = `[
	{"id": "support-people", "type": "section_header", "label": "Support People", "order": 0},
	{"id": "support-people-names", "type": "short_text", "label": "Who will be present for support?", "order": 1},
	{"id": "pain-management", "type": "single_select", "label": "Preferred pain management", "options": ["Unmedicated", "Epidural", "Open to options in the moment"], "order": 2},
	{"id": "special-requests", "type": "long_text", "label": "Special requests or concerns", "order": 3},
	{"id": "share-with-backup", "type": "checkbox", "label": "OK to share this plan with a backup doula", "order": 4}
]`

const defaultBirthPlanFields = `[
	{"id": "labor-starts", "type": "section_header", "label": "When Labor Starts", "order": 0},
	{"id": "notify", "type": "multi_select", "label": "People to notify when labor starts", "options": ["Partner", "Doula", "Midwife", "OB", "Family"], "order": 1},
	{"id": "birth-setting", "type": "section_header", "label": "Birth Setting", "order": 2},
	{"id": "location", "type": "single_select", "label": "Planned birth location", "options": ["Home", "Birth center", "Hospital"], "order": 3},
	{"id": "atmosphere", "type": "long_text", "label": "Preferences for atmosphere (music, lighting, etc.)", "order": 4},
	{"id": "consent-photos", "type": "checkbox", "label": "OK to take photos/video during labor", "order": 5}
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
