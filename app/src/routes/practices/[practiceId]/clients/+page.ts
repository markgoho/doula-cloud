/*
 * #539 (ADR-0017): decides, before the Clients list ever mounts, whether
 * this Staff member is a contractor Doula holding neither the owner nor
 * the admin role. She sees no "Find or add a Client" control -- she
 * originates nothing at a Practice she contracts for -- and, if her list
 * is empty, an extra line points her to #501's explain-only door instead.
 *
 * The read itself, and its fall-back-on-failure posture, live in
 * #lib/practiceContractorGate.js -- shared with `clients/search/+page.ts`,
 * whose load this mirrors exactly.
 */
import { loadContractorGate, type ContractorGate } from '#lib/practiceContractorGate.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }): Promise<ContractorGate> => {
	return loadContractorGate(params.practiceId);
};
