/*
 * #501 (ADR-0017): decides, before the search screen ever mounts, whether
 * this Staff member gets it at all. A contractor Doula never originates a
 * Client at a Practice she contracts for -- client/search.go's
 * SearchHandler refuses her with a 403 naming the reason -- so this load
 * intercepts ahead of that refusal rather than letting the search form
 * render and fail on submit.
 *
 * The read itself, and its fall-back-on-failure posture, live in
 * #lib/practiceContractorGate.js -- shared with `clients/+page.ts` (#539)
 * since #539 made this the second copy of the same session read.
 */
import { loadContractorGate, type ContractorGate } from '#lib/practiceContractorGate.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }): Promise<ContractorGate> => {
	return loadContractorGate(params.practiceId);
};
