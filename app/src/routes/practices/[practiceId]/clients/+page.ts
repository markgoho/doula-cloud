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
 */
import { isAmbientContractor, isOwner } from '#lib/roles.js';
import type { PageLoad } from './$types';

export interface ContractorGate {
	isContractor: boolean;
	isOwner: boolean;
}

export const load: PageLoad = async ({ parent }): Promise<ContractorGate> => {
	const { session } = await parent();
	return { isContractor: isAmbientContractor(session), isOwner: isOwner(session) };
};
