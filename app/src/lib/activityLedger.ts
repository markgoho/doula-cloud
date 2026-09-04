/**
 * #486's activity ledger, the shape every surface it appears on shares:
 * the practice-wide feed on the hub, the record-scoped ledger on the
 * staff Engagement page, and the same record-scoped read behind the
 * Client portal's own closed disclosure. One DTO
 * (activityfeed.Entry, api/internal/activityfeed/activityfeed.go), one
 * set of columns (dates.ts's formatActivityTimestamp for the meta
 * column, describeActivityAction for the body column, actorName already
 * resolved server-side for the muted column) -- three routes never build
 * their own.
 */

import { apiErrorMessage } from './api.js';
import { formatActivityTimestamp } from './dates.js';
import type { CursorPage } from './paginatedList.svelte.js';
import type { EngagementReference } from './engagementDetail.js';

/**
 * subjectKind/subjectId are optional because loadEngagementActivityPage
 * reads the pre-existing engagement.ActivityEntry (Go), which #486
 * deliberately left untouched (see that loader's own doc comment) and
 * which carries neither -- it already knows which Engagement it is,
 * since the path itself names it. The two new readers
 * (activityfeed.Entry) always carry both; no route here renders either
 * field, so the difference is never user-visible.
 */
export interface ActivityEntry {
	subjectKind?: string;
	subjectId?: string;
	action: string;
	actorKind: string;
	actorName: string;
	createdAt: string;
}

/** A fetch-shaped function, the same seam engagementDetail.ts's own
 * loaders take. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/**
 * "The event as body text" (brief.md's #433 amendment): a raw action
 * string ("invoice_raised", "contract_signed", "created") turned into a
 * sentence-cased phrase. Deliberately generic rather than a hand-copied
 * label per action -- the write side (activity/actions.go) already names
 * every action once, and a second, hand-maintained copy here is exactly
 * the kind of table that goes stale the next time a write site adds one.
 */
export function describeActivityAction(action: string): string {
	const spaced = action.replaceAll('_', ' ');
	return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

/**
 * One column, shaped to satisfy DataTable's own `Column<T>` structurally
 * (that type is local to DataTable.svelte, not exported -- a plain object
 * literal matching its shape is how every other Column-typed value in
 * this codebase is built).
 */
interface LedgerColumn {
	label: string;
	accessor: (row: ActivityEntry) => string;
	variant: 'meta' | 'body' | 'muted';
	datetimeAccessor?: (row: ActivityEntry) => string;
}

/**
 * The brief's own three columns (#433's amendment), in the brief's own
 * order -- When, What, Who -- built once so the hub feed, the staff
 * Engagement ledger and the Client portal's own disclosure render the
 * identical treatment rather than three hand-typed literals free to drift.
 * The When column's `datetimeAccessor` is ADR-0022's own requirement:
 * `row.createdAt` is already the raw instant, so the rendered `<time>`
 * carries it as its machine-readable value even while accessor shows the
 * relative-or-absolute display string.
 */
export function activityLedgerColumns(): LedgerColumn[] {
	return [
		{
			label: 'When',
			accessor: (row) => formatActivityTimestamp(row.createdAt),
			variant: 'meta',
			datetimeAccessor: (row) => row.createdAt
		},
		{ label: 'What', accessor: (row) => describeActivityAction(row.action), variant: 'body' },
		{ label: 'Who', accessor: (row) => row.actorName, variant: 'muted' }
	];
}

/**
 * One page of the Practice-wide feed (#486 AC1), newest first.
 */
export async function loadPracticeActivityPage(
	fetcher: Fetcher,
	practiceId: string,
	cursor: string
): Promise<CursorPage<ActivityEntry>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/practices/${practiceId}/activity${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as CursorPage<ActivityEntry>;
}

/** One page of one Engagement's own ledger (#486 AC4), the staff side --
 * engagement.ListActivityHandler, unchanged by #486. */
export async function loadEngagementActivityPage(
	fetcher: Fetcher,
	reference: EngagementReference,
	cursor: string
): Promise<CursorPage<ActivityEntry>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(
		`/api/practices/${reference.practiceId}/engagements/${reference.engagementId}/activity${query}`
	);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as CursorPage<ActivityEntry>;
}

/** One page of the same Engagement's ledger, read from the Client portal
 * (#486 AC5) -- portal.ActivityHandler, behind the closed disclosure. */
export async function loadPortalActivityPage(
	fetcher: Fetcher,
	engagementId: string,
	cursor: string
): Promise<CursorPage<ActivityEntry>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/portal/engagements/${engagementId}/activity${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as CursorPage<ActivityEntry>;
}
