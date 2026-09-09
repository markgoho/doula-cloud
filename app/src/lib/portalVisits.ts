/**
 * #478's "Your visits": the Client portal's own read of the Visits on
 * her Engagement, and the columns the hub renders them through.
 *
 * It lives beside activityLedger.ts rather than inside the route for the
 * same reason that file gives: the DTO, the loader and the columns are
 * one description of one read, and a route that builds its own column
 * literals is a route free to drift from the one this module tests.
 *
 * There is deliberately no Visit type here, and no notes. CONTEXT.md's
 * Visit entry keeps both staff-only, and the type carries no Client word
 * at all -- `postpartum` "still describes a birth with a baby at the end
 * of it" (docs/journeys/loss-client.md), so it is the wrong word in the
 * portal of a Client whose pregnancy ended in a loss. The server does not
 * send either field; this module could not render one if it tried.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './api.js';
import { formatPortalVisit } from './dates.js';
import type { CursorPage } from './paginatedList.svelte.js';

/**
 * One Visit as a Client reads it: when it is, who is coming, and whether
 * it has happened. `hasHappened` is the server's own answer (portal.Visit,
 * api/internal/portal/visits.go), so "Thursday at 2pm" and "she came on
 * 18 August" are told apart without the browser comparing clocks.
 *
 * `scheduledAt` is not optional, unlike the Staff-side Visit's: a Visit
 * nobody has scheduled never reaches this response -- `created_at` is
 * when a Doula typed the row, not when anyone visited, so showing it
 * would tell her a visit happened that may not have.
 */
export interface PortalVisit {
	visitId: string;
	scheduledAt: string;
	doulaName: string;
	hasHappened: boolean;
}

/**
 * One column, shaped to satisfy DataTable's own `Column<T>` structurally
 * -- LedgerColumn's own reason (activityLedger.ts): that type is local to
 * DataTable.svelte and not exported.
 */
interface VisitColumn {
	label: string;
	accessor: (row: PortalVisit) => string;
	variant: 'meta' | 'body' | 'muted';
	datetimeAccessor?: (row: PortalVisit) => string;
}

/**
 * The design source's own two columns on `Your care - Desktop` and
 * `Your care - narrow` -- when, then who -- in that order. The When
 * column takes the ledger's own `meta` treatment (a fixed date column at
 * tabular figures) so the two sections of this one page read as one
 * table, and carries `datetimeAccessor` for the same reason the ledger's
 * does: the display string is a rendering, and the instant underneath it
 * stays machine-readable for a screen reader, a hover and a copy-paste.
 *
 * The Who column carries the Doula's own name, not "Your practice". The
 * Activity ledger redacts a Staff actor's name because CONTEXT.md says a
 * Client "never [reads] who inside the Practice did what" -- a fact about
 * the Practice's roster. Who is coming to her home is a fact about her
 * care, and CONTEXT.md's Visit entry settles it as part of this surface.
 * It takes the `muted` treatment the ledger's own Who column takes, which
 * is also the drawing's own fill for this cell (`$color-on-surface-variant`):
 * the answer she is scanning for is when, and the name reads beside it.
 */
export function portalVisitColumns(): VisitColumn[] {
	return [
		{
			label: 'When',
			accessor: (row) => formatPortalVisit(row.scheduledAt, row.hasHappened),
			variant: 'meta',
			datetimeAccessor: (row) => row.scheduledAt
		},
		{ label: 'Who', accessor: (row) => row.doulaName, variant: 'muted' }
	];
}

/**
 * Reading order, which is not the wire's order. The response is one
 * cursor stream ordered furthest-future first (docs/api-design.md
 * section 4's DESC), so a page holds every scheduled Visit before every
 * past one -- but within the scheduled ones that puts the most distant
 * on top, and the question both journeys ask is "when is someone
 * coming", which the *next* Visit answers. So the scheduled ones are
 * turned around for display and the past ones are left newest-first, the
 * drawing's own row order. `message.ListHandler`'s consumer reverses a
 * page for the same reason: the order that pages stably and the order a
 * person reads are different orders.
 *
 * Sorted rather than reversed, and copied rather than sorted in place: a
 * cursor page is guaranteed ordered, and a caller's own array is not
 * this function's to mutate.
 *
 * One limit, and it belongs to pagination rather than to this sort: an
 * Engagement with more than a page of Visits has the furthest-future
 * ones on page one, so the next Visit arrives with a later page. That is
 * the DESC stream's own shape; the richer time model that would answer
 * "the next visit" directly is
 * [#330](https://github.com/markgoho/doula-cloud/issues/330).
 */
export function inReadingOrder(items: PortalVisit[]): PortalVisit[] {
	const scheduled = items.filter((visit) => !visit.hasHappened);
	const happened = items.filter((visit) => visit.hasHappened);
	scheduled.sort((a, b) => a.scheduledAt.localeCompare(b.scheduledAt));
	return [...scheduled, ...happened];
}

/**
 * The empty state, and it is one sentence with nothing promised in it.
 * **CB-G5** is the mistake it exists to avoid: the Birth Plan link once
 * announced something the Engagement did not have. This has to stay true
 * on a postpartum-only Engagement, where nothing prenatal was ever
 * coming, and on one where the Practice simply has not booked anything
 * yet -- so it reports the state of her own list and says nothing about
 * what anyone will do next.
 *
 * "yet" is the one word here worth defending, because it does lean
 * forward. It reports that this list is empty at the moment rather than
 * that it is finished, which is true in both of those cases and commits
 * nobody to anything -- unlike a sentence naming a visit, a time, or a
 * person, which is what CB-G5 actually records going wrong.
 */
export const noVisitsMessage = 'Nothing is booked yet.';

/** One page of her own Engagement's Visits (#478) -- furthest-future
 * first, so every scheduled Visit precedes every past one. */
export async function loadPortalVisitsPage(
	fetcher: Fetcher,
	engagementId: string,
	cursor: string
): Promise<CursorPage<PortalVisit>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/portal/engagements/${engagementId}/visits${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as CursorPage<PortalVisit>;
}
