import { apiFetch } from '#lib/api.js';
import {
	CONTRACTS_READ,
	contractOrderOf,
	contractsViewOf,
	practiceContractsPath,
	practiceVoidRequestsPath,
	type ContractsListData,
	type ContractsPage,
	type VoidRequestsPage
} from '#lib/contract.js';
import { canNarrowToMine, isOwnerOrAdmin, type LabeledValue } from '#lib/roles.js';
import { mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import { loadDoulaRoster } from '#lib/staff.js';
import type { PageLoad } from './$types';

export interface ContractsPageData {
	read: Promise<StreamedRead<ContractsListData>>;
	voidRequests?: Promise<StreamedRead<VoidRequestsPage>>;
	doulas: Promise<LabeledValue[]>;
}

// `GET .../contracts` is `anyStaff`; the void-requests read and the roster are Owner and Admin only.
export const load: PageLoad = async ({ params, url, fetch, depends, parent }): Promise<ContractsPageData> => {
	depends(CONTRACTS_READ);
	const { session } = await parent();
	const view = contractsViewOf(url.searchParams);
	const order = contractOrderOf(url.searchParams, view);
	const isMine = canNarrowToMine(session) && url.searchParams.get('mine') === 'true';
	const doula = url.searchParams.get('doula') ?? '';
	const now = new Date();
	const isQueueRead = isOwnerOrAdmin(session);
	return {
		read: mapRead(
			streamRead<ContractsPage>(practiceContractsPath(params.practiceId, { view, order, isMine, doula }), fetch),
			(contracts) => ({ ...contracts, view, order, isMine, doula, now })
		),
		voidRequests: isQueueRead ? streamRead<VoidRequestsPage>(practiceVoidRequestsPath(params.practiceId), fetch) : undefined,
		doulas: isQueueRead ? loadDoulaRoster((path, init) => apiFetch(path, init, fetch), params.practiceId) : Promise.resolve([])
	};
};
