package sitebuild

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// probeConcurrency bounds how many pages are fetched at once.
//
// Sequential would be simpler and is fine at the pilot's scale, but the
// sweep runs on a fixed cadence and every probe can spend its whole
// timeout: a few hundred unreachable pages, one after another, is a
// request that never finishes and a sweep that never reports. Eight at
// a time keeps the worst case bounded without opening enough sockets to
// look like anything but a health check.
const probeConcurrency = 8

// Verifier asks the live site for every published page and records what
// it found.
//
// Deliberately one behavior with no mode: the deploy workflow calls it
// once when it finishes, and Cloud Scheduler calls it on a cadence, and
// neither passes anything to distinguish itself. Checking every hosted
// page every time costs one request per Practice, catches a page that
// was live and has since broken, and means the two callers cannot drift
// apart -- there is no second code path for them to drift into.
type Verifier struct {
	Prober Prober
	Now    Clock
}

// page is one published page as the sweep sees it.
type page struct {
	practiceID string
	slug       string
}

// Verify probes every hosted page and writes back each result.
//
// The transaction must already have opened 00049's site-worker door;
// the handler does that. Rows are read first and written after, so the
// probes themselves hold no cursor open.
func (v Verifier) Verify(ctx context.Context, tx *sql.Tx) error {
	pages, err := readHostedPages(ctx, tx)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}

	results := v.probeAll(ctx, pages)

	now := v.Now()
	for i, p := range pages {
		detail := sql.NullString{String: results[i].Detail, Valid: results[i].Detail != ""}
		_, err := tx.ExecContext(ctx,
			`UPDATE practice_websites
			    SET page_state = $1::practice_page_state,
			        page_checked_at = $2,
			        page_check_detail = $3
			  WHERE practice_id = $4 AND mode = 'hosted'`,
			results[i].State, now, detail, p.practiceID)
		if err != nil {
			// coverage:ignore reason: DB update failure, not exercised by unit tests
			return fmt.Errorf("sitebuild: record probe for %s: %w", p.slug, err)
		}
	}
	return nil
}

// probeAll fetches every page, at most probeConcurrency at a time,
// returning one result per page in the order given.
func (v Verifier) probeAll(ctx context.Context, pages []page) []PageProbe {
	results := make([]PageProbe, len(pages))
	slots := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup
	for i, p := range pages {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i] = v.Prober.Probe(ctx, p.slug)
		})
	}
	wg.Wait()
	return results
}

// readHostedPages lists every Practice with a published page. A hosted
// row always has a slug (00046's CHECK), so there is no incomplete row
// to skip here.
func readHostedPages(ctx context.Context, tx *sql.Tx) ([]page, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT practice_id, slug FROM practice_websites WHERE mode = 'hosted' ORDER BY slug`)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("sitebuild: read published pages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pages []page
	for rows.Next() {
		var p page
		if err := rows.Scan(&p.practiceID, &p.slug); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return nil, fmt.Errorf("sitebuild: scan published page: %w", err)
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("sitebuild: iterate published pages: %w", err)
	}
	return pages, nil
}
