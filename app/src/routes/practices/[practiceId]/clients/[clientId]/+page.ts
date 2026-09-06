/*
 * ADR-0017's "a contractor originates nothing": she cannot request an
 * Engagement at a Practice she contracts for, even on a Client she is
 * attached to (engagementrequest.go's RequestHandler refuses her with a
 * 403). This load decides that before the page ever mounts, the same
 * `loadContractorGate` read `clients/+page.ts` (#539) and
 * `clients/search/+page.ts` (#501) already share, so "Start new work
 * with <name>" can stay hidden rather than render into a readable
 * refusal on click.
 *
 * Editing her is *not* gated the same way -- ADR-0017's write table
 * gives a contractor Edit on her attached Clients -- so this page's own
 * "Edit" link stays unconditional.
 *
 * The same read also answers #691's isOwner: the erase control is
 * Owner-only, and this is the practice session `loadContractorGate`
 * already fetches, so a second role check costs no extra round trip.
 */
import { loadContractorGate, type ContractorGate } from '#lib/practiceContractorGate.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }): Promise<ContractorGate> => {
	return loadContractorGate(params.practiceId);
};
