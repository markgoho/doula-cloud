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
 * read with `all=true` -- the same read the overview's `hasAnyClient`
 * makes -- never the rows on the screen, because the list's default
 * filter is "Clients with work" and a Practice whose only Client has no
 * work, or was erased, still holds a record. Decided here rather than in
 * the component so the link is right on the first paint instead of
 * changing under her.
 */
import { apiFetch } from '#lib/api.js';
import { loadClients } from '#lib/client.js';
import { isAmbientContractor, isOwner } from '#lib/roles.js';
import type { PageLoad } from './$types';

export interface ClientsListGate {
	isContractor: boolean;
	isOwner: boolean;
	// Whether the Practice holds any Client record at all.
	hasAnyClient: boolean;
}

/*
 * A failed read answers "yes": the search works at a Practice of any
 * size, so the link it leads to is never wrong, only one screen longer.
 * `apiFetch`, not `apiFetchWithSession`, for the reason the Invoices
 * load gives: that helper's 401 handling calls `goto()` mid-`load`.
 */
async function hasAnyClientRecord(practiceId: string): Promise<boolean> {
	try {
		const first = await loadClients(apiFetch, practiceId, { showAll: true });
		return first.items.length > 0;
	} catch {
		return true;
	}
}

export const load: PageLoad = async ({ params, parent }): Promise<ClientsListGate> => {
	const { session } = await parent();
	const isContractor = isAmbientContractor(session);
	return {
		isContractor,
		isOwner: isOwner(session),
		// A contractor Doula has no header link, so nothing turns on it.
		hasAnyClient: isContractor ? true : await hasAnyClientRecord(params.practiceId)
	};
};
