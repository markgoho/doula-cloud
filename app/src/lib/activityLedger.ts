/**
 * #486's activity ledger, the shape every surface it appears on shares:
 * the practice-wide feed on the hub, the record-scoped ledger on the
 * staff Engagement page, and the same record-scoped read behind the
 * Client portal's own closed disclosure. One DTO
 * (activityfeed.Entry, api/internal/activityfeed/activityfeed.go --
 * engagement.ActivityEntry is the same shape plus the optional `detail`
 * sentence #887 added, which the other two readers never send), one
 * set of columns (dates.ts's formatActivityTimestamp for the meta
 * column, describeActivityAction for the body column, actorName already
 * resolved server-side for the muted column) -- three routes never build
 * their own.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './apiErrorMessage.js';
import { clientActivityPhrase } from './clientRegister.js';
import { formatActivityTimestamp, formatCalendarDay } from './dates.js';
import type { CursorPage } from './paginatedList.svelte.js';
import { birthOutcomeLabel, endingReasons, type EngagementReference } from './engagementDetail.js';
import { kindLabel } from './engagementRequest.js';

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
	/**
	 * The person this entry happened to, already resolved server-side --
	 * "Renata Alvarez", or the server's own word for someone who has left
	 * (#1148). Optional because only a subject kind whose subject is a
	 * person carries one: a Membership row does, and an Engagement's or a
	 * Client's own rows name a record rather than a person and send
	 * nothing. `staffEventText` appends it to the What column, so a reader
	 * of the practice-wide feed can tell one roster change from the next;
	 * an entry without one renders exactly as it does today.
	 */
	subjectName?: string;
	action: string;
	actorKind: string;
	actorName: string;
	/**
	 * One sentence the server already wrote about what this entry's diff
	 * says, in people's names -- a reassignment reads "Visit reassigned
	 * from <one Doula> to <another>" (#887). Optional because only an
	 * action with something to add beyond its own name carries it, and
	 * because engagement.ActivityEntry is the one reader that sends it:
	 * activityfeed.Entry has no diff to describe. An entry without one
	 * renders through describeActivityAction exactly as it does today.
	 */
	detail?: string;
	/**
	 * Both sides of what the entry changed, as the write side recorded
	 * them (`xBefore`/`xAfter`, raw enum values). Only
	 * engagement.ActivityEntry sends it -- activityfeed.Entry selects no
	 * diff, which is what keeps it off the Client portal -- and only the
	 * Engagement's own facts are read from it here (`factChangeText`).
	 */
	diff?: Record<string, unknown>;
	createdAt: string;
}

/**
 * "The event as body text" (brief.md's #433 amendment): a raw action
 * string ("invoice_raised", "contract_signed", "created") turned into a
 * sentence-cased phrase. Deliberately generic rather than a hand-copied
 * label per action -- the write side (activity/actions.go) already names
 * every action once, and a second, hand-maintained copy here is exactly
 * the kind of table that goes stale the next time a write site adds one.
 *
 * Staff-facing only, and that is the whole of #708's fix: its output is
 * the domain word itself, which ADR-0005 says a Client never meets. The
 * Client portal renders `clientRegister.clientActivityPhrase` instead,
 * whose own doc comment says why the staleness argument above does not
 * carry on that side.
 */
export function describeActivityAction(action: string): string {
	const spaced = action.replaceAll('_', ' ');
	return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

type Diff = Record<string, unknown>;

/**
 * The word a side of a change reads as when nothing was recorded there.
 */
const NOT_RECORDED = 'Not recorded';

/** One side of a change: the label, `NOT_RECORDED` for a recorded null,
 * and `undefined` when the diff does not carry that side at all. */
function side(value: unknown, label: (value: string) => string): string | undefined {
	if (value === null) return NOT_RECORDED;
	return typeof value === 'string' ? label(value) : undefined;
}

/** Both sides of one fact as "before to after", or `undefined` unless the
 * diff carries both. `labelBefore` differs from `label` only for a birth
 * outcome, whose date belongs to its own side. */
function fromTo(
	diff: Diff,
	field: string,
	label: (value: string) => string,
	labelBefore: (value: string) => string = label
): string | undefined {
	const before = side(diff[`${field}Before`], labelBefore);
	const after = side(diff[`${field}After`], label);
	return before === undefined || after === undefined ? undefined : `${before} to ${after}`;
}

/** Looks a stored value up in a label table, or shows it as stored if
 * this build has not labeled it -- `kindLabel`'s own leniency. */
function labelFrom(options: readonly { value: string; label: string }[]): (value: string) => string {
	return (value) => options.find((option) => option.value === value)?.label ?? value;
}

const statusLabel = labelFrom([
	{ value: 'intake', label: 'Intake' },
	{ value: 'active', label: 'Active' },
	{ value: 'completed', label: 'Completed' }
]);

/** A birth outcome and the date the pregnancy ended read as one fact, the
 * way the Engagement page's own section shows them. */
function outcomeLabel(endedOn: unknown): (outcome: string) => string {
	return (outcome) => {
		const label = birthOutcomeLabel(outcome);
		return typeof endedOn === 'string' ? `${label}, ${formatCalendarDay(endedOn)}` : label;
	};
}

/**
 * How each of an Engagement's mutable facts reads as a change (#1423):
 * status, kind and birth outcome, the facts ADR-0015 made the Practice
 * answer "how did this come to be?" for. Each returns `undefined` when
 * the diff does not carry both sides, and the entry falls back to its
 * action alone.
 *
 * This is a table keyed by action, which `describeActivityAction` argues
 * against, and the argument does not carry here: that one is about naming
 * actions, which the write side already does once. These read the values
 * inside a diff, which the write side stores as raw enum values, and a
 * raw enum value never reaches the screen (#262) -- the labels for them
 * live in this app, so the sentence is built here rather than in Go. The
 * ending note stays out of every sentence on purpose: it is free text,
 * and the Engagement page is where it is read.
 */
const factChanges: Record<string, (diff: Diff) => string | undefined> = {
	care_phase_changed: (diff) => fromTo(diff, 'status', statusLabel),
	engagement_reopened: (diff) => fromTo(diff, 'status', statusLabel),
	engagement_completed: (diff) => {
		const move = fromTo(diff, 'status', statusLabel);
		const reason = side(diff.endingReasonAfter, labelFrom(endingReasons));
		return move && reason ? `${move}. Reason: ${reason}` : move;
	},
	kind_changed: (diff) => fromTo(diff, 'kind', kindLabel),
	birth_outcome_recorded: (diff) =>
		fromTo(
			diff,
			'birthOutcome',
			outcomeLabel(diff.pregnancyEndedOnAfter),
			outcomeLabel(diff.pregnancyEndedOnBefore)
		)
};

function factChangeText(row: ActivityEntry): string | undefined {
	const change = row.diff ? factChanges[row.action]?.(row.diff) : undefined;
	return change ? `${describeActivityAction(row.action)}: ${change}` : undefined;
}

/**
 * The two staff surfaces' event text: the server's own `detail` sentence
 * when the entry carries one (#887), and the generic description
 * otherwise. `activityLedgerColumns`'s default, and the whole of what
 * #708 left unchanged for them.
 *
 * `subjectName` is appended when the entry carries one (#1148), because
 * on a feed spanning every subject kind the action alone does not say who
 * it happened to -- "Roles changed" names the actor in the Who column and
 * leaves whose roles moved unanswered. An em dash rather than a second
 * column: the ledger's three columns (When, What, Who) are the design
 * brief's own, they are shared with two surfaces that send no subject at
 * all, and a fourth column empty on two of three surfaces would be a
 * worse table at 320px than a slightly longer sentence.
 *
 * A `detail` sentence wins outright and does not get the name appended:
 * it is the server's own finished prose about the row, and it already
 * names the people in it.
 */
function staffEventText(row: ActivityEntry): string {
	if (row.detail) return row.detail;
	const change = factChangeText(row);
	if (change) return change;
	const described = describeActivityAction(row.action);
	return row.subjectName ? `${described} — ${row.subjectName}` : described;
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
 * The What column prefers the server's own `detail` sentence when the
 * entry carries one and falls back to the generic description otherwise
 * (#887) -- still no per-action label table here, for the reason
 * describeActivityAction's own comment gives.
 * The When column's `datetimeAccessor` is ADR-0022's own requirement:
 * `row.createdAt` is already the raw instant, so the rendered `<time>`
 * carries it as its machine-readable value even while accessor shows the
 * relative-or-absolute display string.
 *
 * #708: the What column's text is the one thing a caller may substitute,
 * because it is the one thing that differs by who is reading -- the
 * Client portal passes `clientActivityLedgerColumns` below. It is a
 * parameter rather than a post-hoc rewrite of the returned array, so
 * neither caller has to find its column by matching the display copy
 * "What", which is a heading a design change is free to reword.
 * Everything else stays built once here, which is the point of the
 * module.
 */
export function activityLedgerColumns(
	describeEvent: (row: ActivityEntry) => string = staffEventText
): LedgerColumn[] {
	return [
		{
			label: 'When',
			accessor: (row) => formatActivityTimestamp(row.createdAt),
			variant: 'meta',
			datetimeAccessor: (row) => row.createdAt
		},
		{ label: 'What', accessor: describeEvent, variant: 'body' },
		{ label: 'Who', accessor: (row) => row.actorName, variant: 'muted' }
	];
}

/**
 * The same three columns, with the What column reading the Client
 * register's own fixed phrase for the action instead of the staff-facing
 * humanizer (#708). The Client portal's disclosure is the one caller.
 *
 * `row.detail` is not consulted here, and that is structural rather than
 * incidental: #887's sentence is staff-register prose that names
 * individual Doulas ("Visit reassigned from <one Doula> to <another>"),
 * which is exactly the half of CONTEXT.md's Activity entry
 * portal.ActivityHandler's own actor redaction exists to hold. No portal
 * reader can send one today -- activityfeed.Entry has no such field --
 * but that handler redacts the actor's name and nothing else, so a column
 * that fell back to `detail` would leak the moment one did.
 */
export function clientActivityLedgerColumns(): LedgerColumn[] {
	return activityLedgerColumns((row) => clientActivityPhrase(row.action));
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
