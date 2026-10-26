/**
 * What a Staff member's page and its two question pages share (#2041):
 * where each one is, and how the one person they are about is read.
 *
 * No endpoint answers for one Membership, and none is needed. The roster
 * is bounded (server-capped at `maxMembers`, #446) and Owner-and-Admin,
 * the same two people who reach these pages, so a page reads the roster
 * and finds the person in it -- the shape `mfa-recovery` already uses.
 */
import { resolve } from '$app/paths';
import type { Fetcher } from '#lib/fetcher.js';
import { loadStaff, type StaffSummary } from '#lib/staff.js';

export function rosterHref(practiceId: string): string {
	return resolve('/practices/[practiceId]/staff', { practiceId });
}

export function memberHref(practiceId: string, staffId: string): string {
	return resolve('/practices/[practiceId]/staff/[staffId]', { practiceId, staffId });
}

/**
 * The one Member a page is about. Throws with a sentence a reader can act
 * on when the roster no longer holds her -- a removal in another tab, or
 * an address typed by hand -- and with the roster's own refusal otherwise.
 */
export async function loadMember(fetcher: Fetcher, practiceId: string, staffId: string): Promise<StaffSummary> {
	const roster = await loadStaff(fetcher, practiceId);
	const member = roster.members.find((entry) => entry.staffId === staffId);
	if (!member) {
		throw new Error('That Staff member is not on this practice roster.');
	}
	return member;
}
