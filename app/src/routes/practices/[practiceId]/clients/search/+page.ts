/*
 * #501 (ADR-0017): decides, before the search screen ever mounts, whether
 * this Staff member gets it at all. A contractor Doula never originates a
 * Client at a Practice she contracts for -- client/search.go's
 * SearchHandler refuses her with a 403 naming the reason -- so this load
 * intercepts ahead of that refusal rather than letting the search form
 * render and fail on submit.
 *
 * Reads the Membership `practices/[practiceId]/+layout.ts` already
 * resolved (#835) through `parent()`, the same as `clients/+page.ts`
 * (#539) -- one session read per navigation, not a second fetch here.
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
