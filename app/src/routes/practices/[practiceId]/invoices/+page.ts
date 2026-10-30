import { mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import {
	practiceInvoicesPath,
	type PracticeInvoiceListData,
	type PracticeInvoicePage
} from '#lib/invoice.js';
import type { PageLoad } from './$types';

/**
 * Loads through SvelteKit's `load`, for the reason the Billing route's
 * `load` gives (#471), and hands the read over unawaited, so a click
 * changes screen at once (#2249). This read is refused to a
 * contractor Doula (ADR-0008's money row as amended by #282), so one who
 * types this URL gets a 403, which the screen draws as the page
 * `practices/+error.svelte` would.
 */
export const load: PageLoad = ({
	params,
	url
}): { read: Promise<StreamedRead<PracticeInvoiceListData>> } => {
	// #768: the overdue narrowing is a link, not a control the page holds
	// in memory -- the URL is the state, so it survives a reload, a back
	// button and a link a Practice sends to itself. The load reads it and
	// carries it through, because every later page of the same narrowed
	// list has to ask for the same narrowing.
	const isNarrowedToOverdue = url.searchParams.get('overdue') === 'true';
	const path = practiceInvoicesPath(params.practiceId, undefined, isNarrowedToOverdue);
	return {
		read: mapRead(streamRead<PracticeInvoicePage>(path), (invoices) => ({
			...invoices,
			isNarrowedToOverdue
		}))
	};
};
