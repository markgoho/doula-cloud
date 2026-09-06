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
 * a non-2xx response. */
export async function loadStaff(
	fetcher: Fetcher,
	practiceId: string,
	cursor = ''
): Promise<Roster> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${staffPath(practiceId)}${query}`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
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
