import { bothRead, mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import { practiceAwaitingContractsPath, practiceAwaitingVoidRequestsPath } from '#lib/contract.js';
import type { CursorPage } from '#lib/paginatedList.svelte.js';
import type { AwaitingContract, AwaitingVoidRequest } from '#lib/contract.js';
import type { PageLoad } from './$types';

/** The two Practice-wide, Owner/Admin-only roll-ups this route shows
 * (#273, #971) -- one screen, two work lists, since #971's own AC is
 * "sees the void requests waiting on them" beside the Contracts already
 * waiting on a signature, not a second address to remember. */
export interface ContractsPageData {
	contracts: CursorPage<AwaitingContract>;
	voidRequests: CursorPage<AwaitingVoidRequest>;
}

/**
 * Loads through SvelteKit's `load`, mirroring the Practice-wide Invoice
 * list's own `+page.ts` (#265), and hands the reads over unawaited, so a
 * click changes screen at once (#2249). Both reads are Owner/Admin-only,
 * so a Doula who types this URL gets a 403, which the screen draws as the
 * page `practices/+error.svelte` would. Both roll-ups load in parallel --
 * neither depends on the other.
 */
export const load: PageLoad = ({ params }): { read: Promise<StreamedRead<ContractsPageData>> } => {
	const contracts = streamRead<CursorPage<AwaitingContract>>(practiceAwaitingContractsPath(params.practiceId));
	const voidRequests = streamRead<CursorPage<AwaitingVoidRequest>>(
		practiceAwaitingVoidRequestsPath(params.practiceId)
	);
	return {
		read: mapRead(bothRead(contracts, voidRequests), ([contracts, voidRequests]) => ({ contracts, voidRequests }))
	};
};
