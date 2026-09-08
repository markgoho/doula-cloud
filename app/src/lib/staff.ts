/**
 * A Practice's Staff roster: its Members, its pending Invitations, and
 * one Member's work-state history. This module holds the load/save
 * orchestration for the Staff screen (`practices/[practiceId]/staff`),
 * decoupled from SvelteKit and the DOM so it can be unit-tested
 * directly -- mirrors client.ts.
 */

import type { Fetcher } from './fetcher.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import type { CursorPage } from './paginatedList.svelte.js';
import type { LabeledValue } from './roles.js';

/** One row of the Members table -- mirrors the Go BFF's staff roster
 * response (api/internal/staffauth). */
export interface StaffSummary {
	staffId: string;
	name: string;
	email: string;
	roles: string[];
	employmentType: 'employee' | 'contractor';
	workState: string;
	workStateReportedAt: string;
}

/**
One row of the Pending invitations table.
*/
export interface InvitationSummary {
	invitationId: string;
	address: string;
	roles: string[];
	employmentType: 'employee' | 'contractor';
	expiresAt: string;
	expired: boolean;
	deliveryFailed: boolean;
}

/** The whole roster the endpoint answers on every page: the bounded
 * Members list in full, plus one page of Invitations (#446) -- only the
 * Invitations half grows, so it carries the cursor. */
export interface Roster {
	members: StaffSummary[];
	invitations: CursorPage<InvitationSummary>;
}

function staffPath(practiceId: string): string {
	return `/api/practices/${practiceId}/staff`;
}

/** Loads the roster: every Member, and one page of pending Invitations.
 * `cursor` is the empty string for the first page, matching
 * `PaginatedList`'s own convention. Throws with the response body text on
 * a non-2xx response, carrying the status as the Error's `cause` so a
 * caller can tell a refusal apart from a failure -- see
 * `loadDoulasOrNone`, the one caller that needs to. */
export async function loadStaff(
	fetcher: Fetcher,
	practiceId: string,
	cursor = ''
): Promise<Roster> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${staffPath(practiceId)}${query}`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response), { cause: response.status });
	}
	return response.json();
}

/** One entry of a Member's "Works from" history (#459): a first
 * assertion (no `previousWorkState`, migration 00043 leaves it NULL) or a
 * move from one state to another. */
export interface WorkStateChange {
	eventId: string;
	previousWorkState?: string;
	workState: string;
	createdAt: string;
}

/** One page of a Member's work-state history, plus the date her
 * Membership began -- the line #459's screen needs to mark an assertion
 * made before she joined this Practice. */
export interface WorkStateHistory extends CursorPage<WorkStateChange> {
	memberSince: string;
}

/** Loads one page of a Member's work-state history, fetched only when
 * her disclosure is opened (never with the roster, which would otherwise
 * grow with every correction anybody has ever made). `cursor` is the
 * empty string for the first page. Throws with the response body text on
 * a non-2xx response. */
export async function loadWorkStateHistory(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	cursor = ''
): Promise<WorkStateHistory> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(
		`${staffPath(practiceId)}/${staffId}/work-state-history${query}`
	);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/** Saves a Membership's roles and employment type together, one change
 * (RA-G2, #261). Throws with the response body text on a non-2xx
 * response -- including the "a practice must keep at least one Owner"
 * refusal, which the caller shows as-is. */
export async function updateMembership(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	roles: string[],
	employmentType: 'employee' | 'contractor'
): Promise<void> {
	const response = await fetcher(`${staffPath(practiceId)}/${staffId}/membership`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ roles, employmentType })
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}

/** Ends a Membership: her reach over this Practice stops, her Staff
 * account and everything she did while she was here stay (#291). Sends
 * `X-Confirmed` because removal is one of the confirmed-destructive
 * actions this screen already gates behind a `ConfirmDialog`. Throws with
 * the response body text on a non-2xx response. */
export async function removeMember(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string
): Promise<void> {
	const response = await fetcher(`${staffPath(practiceId)}/${staffId}/membership`, {
		method: 'DELETE',
		headers: { 'X-Confirmed': 'true' }
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}

/** Revokes a pending Invitation so it no longer works. Throws with the
 * response body text on a non-2xx response. */
export async function revokeInvitation(
	fetcher: Fetcher,
	practiceId: string,
	invitationId: string
): Promise<void> {
	const response = await fetcher(
		`${staffPath(practiceId)}/invitations/${invitationId}/revoke`,
		{ method: 'POST', headers: { 'X-Confirmed': 'true' } }
	);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}

/** Ends every session a Staff member holds, on every device, at once --
 * not the same thing as sign-out, which only ends the browser making the
 * request (#154). Throws with the response body text on a non-2xx
 * response. */
export async function endSessions(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string
): Promise<void> {
	const response = await fetcher(`${staffPath(practiceId)}/${staffId}/sessions`, {
		method: 'DELETE',
		headers: { 'X-Confirmed': 'true' }
	});
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}

/**
 * A Staff member a Visit can be assigned to: who she is, what she is
 * called, and what she is to the business.
 *
 * Narrower than `StaffSummary` on purpose -- three screens pick a person
 * off the roster (the Practice-wide schedule's filter, the Offers section,
 * and the Engagement page's Visit pickers), and none of them has any use
 * for her sign-in address or her work state.
 */
export interface Doula {
	staffId: string;
	name: string;
	employmentType: string;
}

/**
 * The Staff roster reduced to the people a Visit can be put on: the
 * holders of the Doula role.
 *
 * The filter is the roster's, not the caller's -- a Staff member without
 * the Doula role can never be named on a Visit (api/internal/visit's
 * `requireEligibleAssignee` refuses her), so offering her in a picker
 * would be offering a choice the BFF will reject.
 *
 * Throws with the response body text on a non-2xx response, the same as
 * `loadStaff`, which it is one line on top of. `GET .../staff` is
 * Owner/Admin, so a plain Doula's call throws -- see `loadDoulasOrNone`
 * for the screens that read that as "not for you" rather than as a
 * failure.
 */
export async function loadDoulas(fetcher: Fetcher, practiceId: string): Promise<Doula[]> {
	const roster = await loadStaff(fetcher, practiceId);
	return roster.members
		.filter((member) => member.roles.includes('doula'))
		.map(({ staffId, name, employmentType }) => ({ staffId, name, employmentType }));
}

/**
 * `loadDoulas`, with a *refusal* answered as an absence rather than a
 * throw -- and with every other failure still thrown.
 *
 * `undefined` means "this reader may not be offered a colleague to pick",
 * which is a different thing from `[]`, a Practice with no Doulas on its
 * roster yet. A screen renders the picker for the second and leaves it out
 * entirely for the first: showing a reader a control the BFF will refuse
 * is the defect [#274](https://github.com/markgoho/doula-cloud/issues/274)
 * recorded, and an error banner about a roster she was never entitled to
 * read would only be noise on her own screen.
 *
 * Returning `undefined` rather than throwing says "not for you" in the
 * type, which a bare `catch {}` at each call site could not.
 *
 * Only 401 and 403 are that absence. A 500, a timeout, or a dropped
 * connection is an outage, and answering one with `undefined` would
 * render an outage as a permission boundary: an Owner would watch the
 * Add-a-Visit form, both Visit pickers and the Offers section disappear
 * with nothing on screen saying why, and reloading would be the only way
 * to find out it was never about her role. A screen reports a failure
 * rather than hiding itself (CLAUDE.md's Accessibility and Security
 * expectations; ADR-0021), so anything else is rethrown for the caller
 * to show.
 */
export async function loadDoulasOrNone(
	fetcher: Fetcher,
	practiceId: string
): Promise<Doula[] | undefined> {
	try {
		return await loadDoulas(fetcher, practiceId);
	} catch (error) {
		if (isRosterRefusal(error)) {
			return undefined;
		}
		throw error;
	}
}

/** Whether a thrown roster error is the BFF saying "not yours" -- the 401
 * or 403 `loadStaff` attaches as the Error's `cause` -- rather than
 * something having gone wrong.
 *
 * Anything without a status on it is a failure, deliberately: a dropped
 * connection rejects with a `TypeError` carrying no `cause` at all, and
 * reading that as a refusal is the exact confusion this function exists
 * to stop. */
function isRosterRefusal(error: unknown): boolean {
	return error instanceof Error && (error.cause === 401 || error.cause === 403);
}

/**
 * What a *particular* Visit picker knows on top of the roster (#909).
 *
 * Both Visit pickers on the Engagement page are built from one roster
 * read, and each narrows it differently: the create picker knows who the
 * caller is, the reassign picker also knows who already holds the Visit.
 * Carrying those as an options bag rather than as two functions is what
 * keeps the two pickers from drifting apart -- and what leaves room for
 * a further axis without changing either call site's shape.
 *
 * Every field is optional, so the Practice-wide schedule's Doula filter
 * -- which is a filter, not a question about a Visit -- keeps calling
 * `doulaOptions(doulas)` with nothing at all.
 */
export interface DoulaPickerContext {
	/** The signed-in Staff member's own id, off
	 * `page.data.session.staffId`. Her entry is moved to the front and
	 * marked as hers; absent, or absent from the roster, and nothing is
	 * marked. */
	callerStaffId?: string;
	/** For a reassign picker only: the Staff member this Visit is already
	 * assigned to, who is left out. Reassigning a Visit to the person who
	 * already holds it is a no-op that still writes an activity entry for
	 * a move that did not happen. */
	currentAssigneeStaffId?: string;
}

/** The marker on the caller's own option, so a standing preselected
 * answer reads as an answer rather than as an arbitrary first name. */
const SELF_SUFFIX = ' (you)';

/** The same people as `Select` options -- the staff id stored, the name
 * shown. Two Doulas at one agency can share a name, so the option is
 * always keyed on her id and never on the word (see `Select`'s own
 * `LabeledValue` comment).
 *
 * `context` narrows and orders the list for one Visit picker (#909).
 * Exclusions run first and marking runs second, deliberately: a caller
 * who has been filtered out is not a caller this picker can offer, so
 * she must not be marked or hoisted either. That order is also what lets
 * a further exclusion compose with this one rather than fight it. */
export function doulaOptions(
	doulas: readonly Doula[],
	context: DoulaPickerContext = {}
): LabeledValue[] {
	const offered = doulas.filter((doula) => doula.staffId !== context.currentAssigneeStaffId);
	const isCaller = (doula: Doula) => doula.staffId === context.callerStaffId;
	const ordered = [
		...offered.filter((doula) => isCaller(doula)),
		...offered.filter((doula) => !isCaller(doula))
	];
	return ordered.map((doula) => ({
		value: doula.staffId,
		label: isCaller(doula) ? `${doula.name}${SELF_SUFFIX}` : doula.name
	}));
}
