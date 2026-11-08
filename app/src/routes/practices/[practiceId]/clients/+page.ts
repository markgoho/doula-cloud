/*
 * #539 (ADR-0017): decides, before the Clients list ever mounts, whether
 * this Staff member is a contractor Doula holding neither the owner nor
 * the admin role. She sees no "Find or add a Client" control -- she
 * originates nothing at a Practice she contracts for -- and, if her list
 * is empty, an extra line points her to #501's explain-only door instead.
 *
 * Reads the Membership `practices/[practiceId]/+layout.ts` already
 * resolved (#835) through `parent()`, rather than a second
 * `/api/practices/${practiceId}/session` fetch of its own -- the read
 * `#lib/roles.js`'s predicates decide against is one query per
 * navigation, not one per page.
 *
 * #1609 (ADR-0017's amendment of 2026-10-02): it also decides where the
 * header link goes. While the Practice holds no Client record there is
 * nobody to search for, so the link opens the name question; from the
 * first Client on it opens the search. The test is the count of records
 * read with `all=true` -- `hasAnyClient`, the same read the overview
 * makes -- never the rows on the screen, because the list's default
 * filter is Clients in care and a Practice whose only Client is not yet in
 * care, or was erased, still holds a record. Read here, so a hovered link
 * starts it, and handed over unawaited (#2249), so a click changes screen
 * at once; the screen holds the link's line until it lands, so the link
 * never changes under her.
 */
import { apiFetch } from '#lib/api.js';
import { hasAnyClient, loadDoulaCounts, type DoulaCounts } from '#lib/client.js';
import { isAmbientContractor, isDoula, isOwner, isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
import { doulaOptions, doulasOf, staffPath, type Roster } from '#lib/staff.js';
import type { PageLoad } from './$types';

export interface ClientsListGate {
	isContractor: boolean;
	isOwner: boolean;
	// Whether she holds the Doula role, which decides whether "Mine" is drawn (#2624).
	isDoula: boolean;
	// Whether anyone else holds the Doula role, without which the switch changes nothing (#2675).
	hasOtherDoulas: boolean;
	// Whether she is attached to at least one live Client, without which "Mine" can never match.
	hasMine: Promise<boolean>;
	// Whether the Practice holds any Client record at all.
	hasAnyClient: Promise<boolean>;
	// The Practice's Doulas, for the "Doula" filter: the roster is an Owner's
	// and an Admin's to read (ADR-0008), so it is empty for everyone else and
	// the filter is not drawn.
	doulas: Promise<LabeledValue[]>;
}

/*
 * The one read behind both the Doula filter's options and "Mine":
 * how many Clients each Doula holds. A failed read answers undefined, which each
 * derivation reads as "no guess": every Doula listed, and the switch drawn.
 */
async function loadCounts(practiceId: string, fetcher: typeof globalThis.fetch): Promise<DoulaCounts | undefined> {
	try {
		return await loadDoulaCounts((path, init) => apiFetch(path, init, fetcher), practiceId);
	} catch {
		return undefined;
	}
}

// "Mine" is drawn while her own count is above zero, and when the read failed.
async function hasMineFrom(counts: Promise<DoulaCounts | undefined>): Promise<boolean> {
	const read = await counts;
	return read ? read.mine > 0 : true;
}

/*
 * A failed read leaves the filter out rather than the list: the list does
 * not depend on it. `apiFetch`, not `apiFetchWithSession`, for the reason
 * `hasAnyClientRecord` gives. A Doula is left out only when the counts were
 * read and hold nothing for her.
 */
async function loadDoulas(practiceId: string, fetcher: typeof globalThis.fetch, counts: Promise<DoulaCounts | undefined>): Promise<LabeledValue[]> {
	try {
		const response = await apiFetch(staffPath(practiceId), undefined, fetcher);
		if (!response.ok) return [];
		const options = doulaOptions(doulasOf((await response.json()) as Roster));
		const read = await counts;
		const held = read?.doulas;
		if (!held) return options;
		return options.filter((option) => (held[option.value] ?? 0) > 0);
	} catch {
		return [];
	}
}

/*
 * A failed read answers "yes": the search works at a Practice of any
 * size, so the link it leads to is never wrong, only one screen longer.
 * `apiFetch`, not `apiFetchWithSession`, for the reason the Invoices
 * load gives: that helper's 401 handling calls `goto()` mid-`load`.
 */
async function hasAnyClientRecord(practiceId: string, fetcher: typeof globalThis.fetch): Promise<boolean> {
	try {
		return await hasAnyClient((path, init) => apiFetch(path, init, fetcher), practiceId);
	} catch {
		return true;
	}
}

export const load: PageLoad = async ({ params, parent, fetch }): Promise<ClientsListGate> => {
	const { session } = await parent();
	const isContractor = isAmbientContractor(session);
	const wantsMine = !isContractor && isDoula(session) && session.hasOtherDoulas !== false;
	const wantsMarks = isOwnerOrAdmin(session);
	const counts = wantsMine || wantsMarks ? loadCounts(params.practiceId, fetch) : Promise.resolve(undefined);
	return {
		isContractor,
		isOwner: isOwner(session),
		isDoula: isDoula(session),
		hasOtherDoulas: session.hasOtherDoulas !== false,
		hasMine: wantsMine ? hasMineFrom(counts) : Promise.resolve(false),
		// A contractor Doula has no header link, so nothing turns on it.
		hasAnyClient: isContractor ? Promise.resolve(true) : hasAnyClientRecord(params.practiceId, fetch),
		doulas: wantsMarks ? loadDoulas(params.practiceId, fetch, counts) : Promise.resolve([])
	};
};
