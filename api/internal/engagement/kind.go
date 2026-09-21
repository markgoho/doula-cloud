package engagement

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// Kind is an Engagement's kind -- birth or postpartum, what the Practice
// sold (CONTEXT.md's Engagement entry). Landed by #308
// (00042_client_intake_schema.sql), mutable in both directions per
// ADR-0015, and staff-only: CONTEXT.md gives it no Client word, so no
// value of this type may reach a Client-facing response.
type Kind string

// The two values Kind can hold, per CONTEXT.md's Engagement entry.
const (
	KindBirth      Kind = "birth"
	KindPostpartum Kind = "postpartum"
)

// BirthPlanInputs holds what OffersBirthPlan needs to know about an
// Engagement: what the Practice sold, and what became of the pregnancy.
// BirthOutcome is nil for an Engagement whose outcome has not been
// recorded -- the pregnancy is still expected -- and otherwise holds one
// of outcome.go's three members.
type BirthPlanInputs struct {
	Kind         Kind
	BirthOutcome *string
}

// HasLivingOrExpectedBaby answers ADR-0015's standing question -- does
// this Engagement have a living or expected baby? -- from the one input
// it has today, the recorded birth outcome (#294). No outcome recorded
// means the pregnancy is still expected, so the answer is yes; a live
// birth is yes; a loss is no; and 'unknown' is no, because presuming
// nothing is the safe direction for a Client whose Engagement ended
// without the Practice ever learning what happened.
//
// A question rather than a column check, because ADR-0015 states that a
// later event -- a baby born alive who then dies -- becomes a second
// input to this same question without the rule being rewritten, and
// because every surface that presumes a living or expected baby asks
// this, not only the Birth Plan (#296's greeting is the next caller).
func HasLivingOrExpectedBaby(birthOutcome *string) bool {
	if birthOutcome == nil {
		return true
	}
	return *birthOutcome == OutcomeLiveBirth
}

// OffersBirthPlan answers ADR-0015's suppression question -- does this
// Engagement call for a Birth Plan? -- once, so every surface that
// offers, links to or announces a Birth Plan agrees with every other.
// A Birth Plan is offered when the Practice sold a birth Engagement
// (#311) and that Engagement has a living or expected baby (#294).
//
// This is the Client-facing offer, which is why 'live_birth' still
// answers true: ADR-0015's table keeps an existing plan readable two
// days after the birth, and its "offer to create ... when the birth
// outcome is null" prose is about the Practice's authoring affordance,
// which this function does not gate.
//
// ADR-0015 calls the suppressed state "retired, not deleted", and
// retirement here is entirely derived: no flag is stored on the Plan
// Instance and nothing about it is changed, so correcting an outcome
// from 'loss' back to 'live_birth' gives the Birth Plan back with no
// separate act and nothing to reverse.
func OffersBirthPlan(in BirthPlanInputs) bool {
	return in.Kind == KindBirth && HasLivingOrExpectedBaby(in.BirthOutcome)
}

// KindChangeRequest carries an Engagement's target kind (#874, ADR-0015's
// write side): "the post-birth freeze on kind is a rule, not a
// constraint" -- kind is mutable in both directions for the life of the
// Engagement, and there is no correction flag and nothing to un-set,
// because a kind is always one of the two members below, never absent.
type KindChangeRequest struct {
	Kind string `json:"kind"`
}

// KindChangeResponse confirms the kind the Engagement now holds.
type KindChangeResponse struct {
	EngagementID string `json:"engagementId"`
	Kind         string `json:"kind"`
}

// kinds is Kind's own vocabulary, checked here so a caller gets a clean
// 400 rather than the database's own enum-cast error -- the same
// birthOutcomes/endingReasons shape outcome.go and transition.go use.
var kinds = map[string]bool{string(KindBirth): true, string(KindPostpartum): true}

// kindRow is the row ChangeKindHandler reads before it writes: the kind
// held now, and the birth outcome that decides whether an upgrade to
// 'birth' is still offered.
type kindRow struct {
	kind         string
	birthOutcome *string
}

// ChangeKindHandler moves an Engagement between ADR-0015's two kinds
// (#874): "birth" or "postpartum", what the Practice sold. Unlike the
// birth outcome (00093's freeze), kind carries no *database* freeze --
// 00093's own comment states a freeze keyed on the outcome was
// considered and rejected, because a postpartum-only Engagement records
// live_birth at intake, so it would freeze kind from the moment the row
// was created and leave Camille's cohort no correction window at all.
// There is no CHECK and no trigger here, in line with that.
//
// The ADR states a *product* rule instead, in its "post-birth freeze on
// kind is a rule, not a constraint" section: "the product stops offering
// a postpartum -> birth change once the birth outcome is recorded,
// because attending a birth that has already happened means nothing"
// (docs/adr/0015, the paragraph beginning "The product stops offering").
// refuseUpgradeAfterBirth is that rule, enforced here rather than only
// drawn away in the UI -- every other ADR-0015 rule in this package is
// handler-enforced, and a Birth Plan re-offered for a birth that already
// happened (OffersBirthPlan would answer true again once kind reads
// 'birth' and the frozen outcome is still 'live_birth') is exactly the
// harm the sentence exists to prevent. The reverse move,
// 'birth' -> 'postpartum', carries no such caveat: standing rule 2 names
// it "the rare downgrade" and the reason kind is not DB-frozen at all.
// Kind carries no correction flag of its own -- the ADR gives that hatch
// to the outcome alone ("an Owner, and only an Owner, may correct a
// frozen birth outcome"), so the outcome's own Owner-only correction
// (RecordBirthOutcomeHandler) is the fix for a truly mis-set pair, run
// before this endpoint rather than a second hatch built here.
//
// Every Staff role that may change any of ADR-0015's mutable facts may
// change this one too: refuseFactWrite is the same gate TransitionHandler
// and RecordBirthOutcomeHandler already use (ADR-0006 as ADR-0015's role
// table applies it) -- a contractor Doula is refused outright, and
// everyone else must be an Owner, an Admin or a Doula.
//
// PUT and no Idempotency-Key: re-sending the kind the Engagement already
// holds is a no-op that writes nothing and raises no second audit row,
// the same naturally-idempotent shape RecordBirthOutcomeHandler's own PUT
// uses (docs/api-design.md's rule 4: a full-replacement PUT is inherently
// idempotent). The write itself is compare-and-set on the kind this
// handler just read, so a second writer's own concurrent change is never
// silently overwritten by a stale read -- the write is refused rather
// than applied over it (the n != 1 branch below).
//
// What changing kind does not otherwise do, per ADR-0015's own list: it
// moves no Visit (visit.DeriveType derives a Visit's type from the
// Engagement's pregnancy-end date alone, never from kind), spends no
// second Credit, and touches no Contract or Invoice -- none of those
// three tables carries a kind column or a kind-keyed rule, so there is
// nothing here to cascade. Whether a Birth Plan is offered is
// OffersBirthPlan's own question, answered fresh on every portal read
// (portal.DetailHandler selects e.kind on every request rather than
// caching it), so the very next read after this write already agrees --
// no separate flag to keep in step.
//
// Must be mounted behind staffauth.Middleware.
func ChangeKindHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		if refuseFactWrite(w, reader, "a contractor Doula cannot change an Engagement's kind") {
			return
		}

		var req KindChangeRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if !kinds[req.Kind] {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
				"kind must be 'birth' or 'postpartum'",
				map[string]string{"kind": MsgKindUnknown})
			return
		}

		var current kindRow
		err := tx.QueryRowContext(r.Context(),
			`SELECT kind::text, birth_outcome::text FROM engagements WHERE id = $1 AND practice_id = $2`,
			engagementID, practiceID,
		).Scan(&current.kind, &current.birthOutcome)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if current.kind == req.Kind {
			apierr.WriteJSON(w, http.StatusOK, KindChangeResponse{EngagementID: engagementID, Kind: current.kind})
			return
		}
		if refuseUpgradeAfterBirth(w, current, req.Kind) {
			return
		}

		result, err := tx.ExecContext(r.Context(),
			`UPDATE engagements SET kind = $1 WHERE id = $2 AND practice_id = $3 AND kind = $4`,
			req.Kind, engagementID, practiceID, current.kind,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if n, err := result.RowsAffected(); err != nil || n != 1 {
			// coverage:ignore reason: driver RowsAffected failure or a concurrent writer already changed this Engagement's kind, neither exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		actorStaffID, _ := staffauth.StaffID(r.Context())
		if err := recordKindEvent(r.Context(), tx, kindEvent{
			practiceID:   practiceID,
			engagementID: engagementID,
			previousKind: current.kind,
			kind:         req.Kind,
			actorStaffID: &actorStaffID,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, KindChangeResponse{EngagementID: engagementID, Kind: req.Kind})
	})
}

// refuseUpgradeAfterBirth is ADR-0015's own sentence, checked before the
// write: "the product stops offering a postpartum -> birth change once
// the birth outcome is recorded, because attending a birth that has
// already happened means nothing." A birth outcome of any recorded value
// -- live_birth, loss or unknown -- counts as "already happened": all
// three mean the pregnancy is no longer expected (HasLivingOrExpectedBaby
// answers the finer-grained question OffersBirthPlan needs; this refusal
// only needs to know whether the outcome column is still null). The
// reverse move, target == 'birth' -> 'postpartum', is never refused here
// -- see ChangeKindHandler's own doc comment.
//
// Named and tested apart from the write, matching refuseClearOnCompleted
// and refuseUnexplainedCompletion's own shape: a raw CHECK violation
// would otherwise surface as an unnamed 500, and this endpoint has only
// the one 409 to answer with, so the generic apierr.CodeConflict is
// enough (#692's tell-apart-by-code rule only bites once an endpoint
// answers more than one 409).
func refuseUpgradeAfterBirth(w http.ResponseWriter, current kindRow, target string) bool {
	if current.kind != string(KindPostpartum) || target != string(KindBirth) || current.birthOutcome == nil {
		return false
	}
	apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
		"this Engagement's birth outcome is already recorded, so its kind can no longer be changed to 'birth'", nil)
	return true
}
