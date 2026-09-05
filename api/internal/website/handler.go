package website

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// GetHandler reports the website a Practice has declared. Readable by
// any Staff member at the Practice, because #442's payments screen asks
// it whether the Stripe button may be offered at all and a Doula who
// opens that screen should be told what is outstanding rather than shown
// an empty panel. Nothing here is a secret: the whole point of the
// answer is that it is published.
//
// Mounted behind staffauth.Middleware, and behind GatedRouter like every
// other GET.
func GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, ok := staffauth.Tx(r.Context())
		if !ok {
			// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
			apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError, nil)
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())

		resp, err := read(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError, nil)
			return
		}

		writeJSON(w, resp)
	})
}

// PutHandler writes a Practice's website declaration and appends the
// audit row for it, in one transaction.
//
// Owner only. She is the person Stripe onboards, the person whose
// statement descriptor the declared URL sets, and the person the
// published page speaks for -- the same rule the payments screen already
// applies to starting Connect onboarding. Enforced by
// staffauth.RequireOwner at the boundary rather than by the screen
// hiding a button.
//
// PUT, not POST: one answer per Practice, replaced whole, so re-sending
// the same body is safe and needs no Idempotency-Key.
func PutHandler(nudge tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwner(w, r)
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgInvalidBody, nil)
			return
		}

		valid, details := Validate(req)
		if details != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgInvalidBody, details)
			return
		}

		resp, siteIsStale, err := write(r.Context(), tx, practiceID, staffID, valid)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError, nil)
			return
		}

		// #443: the deploy workflow fires on a push touching hugo/**,
		// and this produces no commit, so without a queued rebuild her
		// page never appears. Queued inside the request transaction, so
		// a declaration that rolls back queues no deploy; the nudge is
		// registered rather than fired, so staffauth.Middleware sends it
		// only once its own commit has succeeded.
		if siteIsStale {
			if err := sitebuild.Queue(r.Context(), tx, practiceID); err != nil {
				// coverage:ignore reason: DB insert failure, not exercised by unit tests
				apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError, nil)
				return
			}
			tasknudge.Register(r.Context(), tasknudge.Fire(nudge, tasknudge.SiteBuild))
		}

		writeJSON(w, resp)
	})
}

// read returns the Practice's declaration, or the undeclared shape when
// she has no row. The audit fields come from the newest event rather
// than from the current row: the row records what the page says, the
// event records who said it, and the screen prints both together.
func read(ctx context.Context, tx *sql.Tx, practiceID string) (Response, error) {
	resp := Response{Mode: ModeUndeclared}

	var (
		ownURL      sql.NullString
		description sql.NullString
		policy      sql.NullString
		pageState   sql.NullString
		checkedAt   sql.NullTime
		checkDetail sql.NullString
		slug        sql.NullString
	)
	err := tx.QueryRowContext(ctx,
		`SELECT mode, own_url, service_description, cancellation_policy,
		        page_state, page_checked_at, page_check_detail, slug
		   FROM practice_websites WHERE practice_id = $1`, practiceID,
	).Scan(&resp.Mode, &ownURL, &description, &policy, &pageState, &checkedAt, &checkDetail, &slug)
	if errors.Is(err, sql.ErrNoRows) {
		return resp, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Response{}, fmt.Errorf("website: read declaration: %w", err)
	}
	resp.OwnURL = ownURL.String
	resp.ServiceDescription = description.String
	resp.CancellationPolicy = policy.String
	resp.PageState = pageState.String
	resp.PageCheckDetail = checkDetail.String
	resp.PageCheckedAt = FormatUpdatedAt(checkedAt.Time, checkedAt.Valid)
	// Only while the page is the answer. A Practice who switched to her
	// own website keeps her slug (00046, so switching back republishes
	// the same address) but has nothing published at it, and showing her
	// a link to a page that is no longer built would be a broken promise.
	if resp.Mode == ModeHosted {
		resp.PageURL = HostedPageURL(slug.String)
	}

	var (
		actorName sql.NullString
		updatedAt sql.NullTime
	)
	// The join is to staff, whose practice-tier RLS policy admits a
	// person holding a Membership here -- which the actor does, because
	// only an Owner of this Practice could have written the row. A LEFT
	// JOIN anyway, so a future actor who has since left the Practice
	// leaves the name blank rather than losing the date with it.
	err = tx.QueryRowContext(ctx,
		`SELECT s.name, e.created_at
		   FROM practice_website_events e
		   LEFT JOIN staff s ON s.id = e.actor_staff_id
		  WHERE e.practice_id = $1
		  ORDER BY e.created_at DESC
		  LIMIT 1`, practiceID,
	).Scan(&actorName, &updatedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Response{}, fmt.Errorf("website: read latest event: %w", err)
	}
	resp.UpdatedBy = actorName.String
	resp.UpdatedAt = FormatUpdatedAt(updatedAt.Time, updatedAt.Valid)

	return resp, nil
}

// write upserts the declaration and appends the event describing the
// move, in the caller's transaction, so a declaration that fails halfway
// leaves neither a changed page with no record of who changed it nor a
// record of a change that did not happen.
//
// The previous mode is read before the upsert for the obvious reason:
// it is what makes the event a transition rather than a restatement, and
// it is NULL exactly once per Practice -- the first time she answers.
//
// No short-circuit when nothing changed. Re-publishing the same words is
// a re-assertion, and the date it happened is what the payments screen
// prints back to her; a silent no-op would leave her looking at an old
// date after an act she just performed.
// The bool it returns is whether the published site no longer matches
// what is stored, and so needs rebuilding (#443). True when she now has
// a hosted page, and true when she has just switched away from one --
// that page has to be pruned, which is as much a rebuild as publishing
// was. False only for a Practice moving between her own websites, who
// has never had a page here to build.
func write(ctx context.Context, tx *sql.Tx, practiceID, actorStaffID string, v Validated) (Response, bool, error) {
	var (
		previousMode sql.NullString
		existingSlug sql.NullString
	)
	err := tx.QueryRowContext(ctx,
		`SELECT mode, slug FROM practice_websites WHERE practice_id = $1 FOR UPDATE`, practiceID,
	).Scan(&previousMode, &existingSlug)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Response{}, false, fmt.Errorf("website: read previous mode: %w", err)
	}
	siteIsStale := v.Mode == ModeHosted || previousMode.String == ModeHosted

	// A slug is minted exactly once, the first time a Practice publishes
	// a hosted page, and is never revisited -- not when she renames the
	// Practice, and not when she switches to her own site and back.
	// Stripe holds the declared URL for the life of the connected
	// account and #382 established its review of that URL is ongoing, so
	// a slug that moves points a live review at a 404.
	needSlug := v.Mode == ModeHosted && !existingSlug.Valid
	var name string
	if needSlug {
		if err := tx.QueryRowContext(ctx,
			`SELECT name FROM practices WHERE id = $1`, practiceID,
		).Scan(&name); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return Response{}, false, fmt.Errorf("website: read practice name: %w", err)
		}
	}

	if err := upsert(ctx, tx, practiceID, name, existingSlug.String, needSlug, v); err != nil {
		// coverage:ignore reason: every failure inside upsert is a DB error or an exhausted retry, none reachable from a unit test
		return Response{}, false, err
	}

	// The event snapshots what was written, not what the row now holds:
	// #382 established Stripe's review of a declared website is ongoing
	// with no published SLA, so "what did this page say when Stripe last
	// looked?" is a question with a real date attached.
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO practice_website_events
		     (practice_id, previous_mode, mode, own_url, service_description, cancellation_policy, actor_staff_id)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), $7)
		 RETURNING created_at`,
		practiceID, previousMode, v.Mode, v.OwnURL, v.ServiceDescription, v.CancellationPolicy, actorStaffID,
	).Scan(&createdAt); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Response{}, false, fmt.Errorf("website: record event: %w", err)
	}

	// Read back rather than echo the request: the upsert carries the
	// other mode's facts forward, so what is now stored is not what was
	// sent, and the screen has to be told the truth about what it may
	// offer her next time.
	resp, err := read(ctx, tx, practiceID)
	return resp, siteIsStale, err
}

func writeJSON(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError, nil)
	}
}

// maxSlugAttempts bounds the collision retry. Ten Practices sharing one
// name is already beyond anything plausible; the eleventh gets a 500
// rather than a loop nobody is watching.
const maxSlugAttempts = 10

// upsert writes the declaration, minting a slug when one is needed, and
// retries the whole statement on a slug collision.
//
// The retry is the shape it is because of RLS. practice_websites is
// visible one Practice at a time (00045), so this transaction cannot
// look up which slugs are already taken -- and a role that could would
// be a role that can read every Practice's row to answer a question
// about a URL. The unique index is the only thing in the system that
// knows, so the write asks it: attempt, catch 23505, take the next
// candidate. The savepoint is what makes that survivable, since a
// constraint violation otherwise aborts the caller's transaction along
// with the audit row it has not written yet.
func upsert(ctx context.Context, tx *sql.Tx, practiceID, name, currentSlug string, needSlug bool, v Validated) error {
	// A URL and the two facts are stored as NULL when absent rather than
	// as an empty string, so 00045's CHECK constraints mean what they
	// say: "mode 'own' has a URL" must not be satisfied by "".
	//
	// The slug uses COALESCE the other way round from the rest: the
	// stored value wins, so a republish can never overwrite the address
	// Stripe already holds.
	//
	// It is also sent on every hosted write, not only the one that mints
	// it. Postgres checks a CHECK constraint against the tuple proposed
	// for insertion before it arbitrates the conflict, so a republish
	// that left $6 NULL would be refused by
	// practice_websites_hosted_slug_present without the DO UPDATE ever
	// being reached -- the COALESCE never gets a chance to keep the slug
	// that is already there.
	//
	// page_state is set from the mode on both arms, and any earlier
	// probe result is cleared with it (#443). She has just changed what
	// the page says, so whatever a probe found before this is about a
	// page that no longer exists: only an affirmative probe of what is
	// now deployed may say "live" again.
	const stmt = `INSERT INTO practice_websites
	     (practice_id, mode, own_url, service_description, cancellation_policy, slug, page_state)
	 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
	         CASE WHEN $2 = 'hosted' THEN 'pending'::practice_page_state END)
	 ON CONFLICT (practice_id) DO UPDATE SET
	     mode = EXCLUDED.mode,
	     own_url = COALESCE(EXCLUDED.own_url, practice_websites.own_url),
	     service_description = COALESCE(EXCLUDED.service_description, practice_websites.service_description),
	     cancellation_policy = COALESCE(EXCLUDED.cancellation_policy, practice_websites.cancellation_policy),
	     slug = COALESCE(practice_websites.slug, EXCLUDED.slug),
	     page_state = EXCLUDED.page_state,
	     page_checked_at = NULL,
	     page_check_detail = NULL,
	     updated_at = now()`

	for attempt := 0; ; attempt++ {
		slug := currentSlug
		if needSlug {
			slug = SlugCandidate(name, practiceID, attempt)
		}

		if _, err := tx.ExecContext(ctx, `SAVEPOINT website_upsert`); err != nil {
			// coverage:ignore reason: savepoint failure, not exercised by unit tests
			return fmt.Errorf("website: savepoint: %w", err)
		}

		_, err := tx.ExecContext(ctx, stmt,
			practiceID, v.Mode, v.OwnURL, v.ServiceDescription, v.CancellationPolicy, slug)
		if err == nil {
			if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT website_upsert`); err != nil {
				// coverage:ignore reason: savepoint failure, not exercised by unit tests
				return fmt.Errorf("website: release savepoint: %w", err)
			}
			return nil
		}

		if !needSlug || !isSlugCollision(err) || attempt >= maxSlugAttempts-1 {
			// coverage:ignore reason: DB write failure, not exercised by unit tests
			return fmt.Errorf("website: upsert declaration: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT website_upsert`); err != nil {
			// coverage:ignore reason: savepoint failure, not exercised by unit tests
			return fmt.Errorf("website: rollback to savepoint: %w", err)
		}
	}
}

// isSlugCollision reports whether err is another Practice already
// holding this slug. It names the constraint rather than taking any
// unique violation, which is the whole distinction: a conflict on
// practice_id is the upsert doing its job, not a collision to retry.
func isSlugCollision(err error) bool {
	return pgerr.IsUniqueViolationOn(err, "practice_websites_slug_key")
}
