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

/**
 * One page of one Member's history of some kind, from the endpoint
 * `segment` names under her own roster path.
 *
 * The two histories a roster row carries -- work state (#459) and
 * Membership (#872) -- differ in nothing but that segment and the shape
 * they answer with, so they read through one function rather than two
 * that would drift. Both are fetched only when a disclosure is opened,
 * never with the roster, which would otherwise grow with every
 * correction and every role change anybody has ever made.
 *
 * `cursor` is the empty string for the first page. Throws with the
 * response body text on a non-2xx response.
 */
async function loadHistoryPage<Page>(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	segment: string,
	cursor: string
): Promise<Page> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${staffPath(practiceId)}/${staffId}/${segment}${query}`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
 * Loads one page of a Member's work-state history (#459).
 */
export function loadWorkStateHistory(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	cursor = ''
): Promise<WorkStateHistory> {
	return loadHistoryPage(fetcher, practiceId, staffId, 'work-state-history', cursor);
}

/** One entry of a Member's Membership history (#872): what happened to
 * her standing at this Practice, who did it, and when.
 *
 * The values are the stored ones (`owner`, `contractor`), never display
 * words -- `roles.ts` is the one place either is given the word a person
 * reads (#262), and the BFF deliberately does not hold a second copy of
 * that map.
 *
 * Every before/after field is optional, and an absent one means the fact
 * did not move on this entry rather than that it became blank: a
 * `joined` entry carries `roles`/`employmentType` with no previous, a
 * `removed` entry carries only the previous, a `roles_changed` carries
 * neither employment field, and `sessions_ended` (#473) carries none of
 * the four -- it names something done to the Membership rather than a
 * change to what it is. */
export interface MembershipChange {
	eventId: string;
	action: string;
	actorName: string;
	previousRoles?: string[];
	roles?: string[];
	previousEmploymentType?: string;
	employmentType?: string;
	createdAt: string;
}

/** One page of a Member's Membership history. Unlike the work-state
 * history it needs no `memberSince`: every entry is recorded against
 * this Practice, so none of them was made anywhere else. */
export type MembershipHistory = CursorPage<MembershipChange>;

/**
 * Loads one page of a Member's Membership history (#872).
 */
export function loadMembershipHistory(
	fetcher: Fetcher,
	practiceId: string,
	staffId: string,
	cursor = ''
): Promise<MembershipHistory> {
	return loadHistoryPage(fetcher, practiceId, staffId, 'membership-history', cursor);
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

/**
 * A Doula, plus whether she can be named on a Visit at *one* Engagement
 * -- mirrors `visit.Assignee` (api/internal/visit/nameable.go).
 *
 * `Doula` alone is a Practice-wide fact and so cannot answer the
 * question the Visit pickers actually ask (#911): the write applies a
 * second, Engagement-scoped rule, and a picker drawn off the roster
 * offered names the BFF would refuse. `reason` is the BFF's own stable
 * token, never a sentence -- the wording below is this module's, so it
 * can change without the contract changing.
 */
export interface VisitAssignee extends Doula {
	nameable: boolean;
	reason?: string;
}

/** The BFF's token for the one reason a Doula on the roster cannot be
 * named on this Engagement: she is a contractor and has not accepted an
 * Offer here. Named once, so no screen spells the token out. */
export const CONTRACTOR_WITHOUT_ACCEPTED_OFFER = 'contractor_without_accepted_offer';

/** What follows a person's name when she cannot be named -- the reason
 * and the act that changes it, in one sentence (GOV.UK's error-message
 * rule: say what is wrong and what to do about it).
 *
 * Lenient on an unrecognized token, for the reason `roleLabel` gives: a
 * reason the BFF grows before this map catches up still blocks the
 * choice and still says she cannot be named, rather than throwing the
 * screen away over a word. */
function unnameableClause(reason: string | undefined): string {
	if (reason === CONTRACTOR_WITHOUT_ACCEPTED_OFFER) {
		return 'is a contractor who has not accepted an Offer on this Engagement, so she cannot be named on a Visit here yet. Send her an Offer, then name her.';
	}
	return 'cannot be named on a Visit on this Engagement yet.';
}

/** The short marker carried in the option's own text. Text, not color and
 * not a bare `disabled` attribute: a person listening to the picker is
 * told the state as she moves through the names, which neither of the
 * other two would do. */
const UNNAMEABLE_MARKER = 'cannot be named yet';

/**
 * The one derived option list both Visit pickers read (#911). Nobody is
 * dropped -- a contractor who has not accepted an Offer is precisely the
 * person an Admin is trying to get onto the birth -- and the ones who
 * cannot be named say so in their own label.
 *
 * One derivation, not one per picker: two independently filtered lists on
 * one page is the failure this exists to close. It is `doulaOptions` plus
 * one marking rather than a second list-builder, so #909's per-caller and
 * per-Visit axes -- who is me, who already holds this Visit -- and this
 * ticket's per-Engagement one land on the same rows in the same order:
 * exclusion first, then "(you)", then "(cannot be named yet)". The
 * caller's own row is always nameable (she takes the self rule), so no
 * name ever carries both markers.
 */
export function visitAssigneeOptions(
	assignees: readonly VisitAssignee[],
	context: DoulaPickerContext = {}
): LabeledValue[] {
	const nameableById = new Map(assignees.map(({ staffId, nameable }) => [staffId, nameable]));
	return doulaOptions(assignees, context).map((option) =>
		nameableById.get(option.value)
			? option
			: { ...option, label: `${option.label} (${UNNAMEABLE_MARKER})` }
	);
}

/** The hint the picker carries whenever it is offering somebody who
 * cannot be named: what the marker in those labels means, before a
 * reader has to choose one to find out. The empty string when everybody
 * listed can be named, so the field is not carrying an explanation of a
 * state nobody is in. */
export function unnameableHint(assignees: readonly VisitAssignee[]): string {
	if (assignees.every((assignee) => assignee.nameable)) return '';
	return `Somebody marked "${UNNAMEABLE_MARKER}" is a contractor who has not accepted an Offer on this Engagement. Send her an Offer, then name her on a Visit.`;
}

/**
 * The refusal a picker shows *instead of sending the request*, or the
 * empty string when the choice is fine.
 *
 * A hard block with the route out, prevented here and still enforced at
 * the BFF (`requireEligibleAssignee`) -- never a dismissible warning, and
 * never letting the BFF's own `400` be the first a person hears of it.
 * An id that is not on the list at all is not this function's refusal to
 * make: an empty picker is the field's own `required`, and an unknown id
 * is the BFF's.
 */
export function assigneeBlock(assignees: readonly VisitAssignee[], staffId: string): string {
	const chosen = assignees.find((assignee) => assignee.staffId === staffId);
	if (!chosen || chosen.nameable) return '';
	return `${chosen.name} ${unnameableClause(chosen.reason)}`;
}

/** Loads who may be named on a Visit at one Engagement: every Doula on
 * the Practice's roster, each marked with whether this Engagement admits
 * her now. Throws with the response body text on a non-2xx response,
 * carrying the status as the Error's `cause`, the same contract
 * `loadStaff` established. */
export async function loadVisitAssignees(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<VisitAssignee[]> {
	const response = await fetcher(
		`/api/practices/${practiceId}/engagements/${engagementId}/visit-assignees`
	);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response), { cause: response.status });
	}
	const body: { items: VisitAssignee[] } = await response.json();
	return body.items;
}

/** `loadVisitAssignees`, with a refusal answered as an absence rather
 * than a throw -- `loadDoulasOrNone`'s contract exactly, for the same
 * reasons, over the endpoint that replaced the roster read on the
 * Engagement page. */
export async function loadVisitAssigneesOrNone(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<VisitAssignee[] | undefined> {
	try {
		return await loadVisitAssignees(fetcher, practiceId, engagementId);
	} catch (error) {
		if (isRosterRefusal(error)) {
			return undefined;
		}
		throw error;
	}
}
