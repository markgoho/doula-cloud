import { mapRead, streamRead, type StreamedRead } from '#lib/streamedRead.svelte.js';
import {
	invoiceOrderOf,
	invoicesViewOf,
	practiceInvoicesPath,
	type PracticeInvoiceListData,
	type PracticeInvoicePage
} from '#lib/invoice.js';
import { invoicePeriodDays, invoicePeriodOf } from '#lib/invoicePeriod.js';
import type { PageLoad } from './$types';

export const _INVOICES_READ = 'app:invoices';

/**
 * Loads through SvelteKit's `load`, for the reason the Billing route's
 * `load` gives (#471), and hands the read over unawaited, so a click
 * changes screen at once (#2249). This read is refused to a
 * contractor Doula (ADR-0008's money row as amended by #282), so one who
 * types this URL gets a 403, which the screen draws as the page
 * `practices/+error.svelte` would.
 */
export const load: PageLoad = async ({
	params,
	url,
	fetch,
	depends,
	parent
}): Promise<{ read: Promise<StreamedRead<PracticeInvoiceListData>> }> => {
	// The screen's "Try again" runs this read again (#2746).
	depends(_INVOICES_READ);
	// #768, #2746: the view is in the address, not a control the page holds
	// in memory -- the URL is the state, so it survives a reload, a back
	// button and a link a Practice sends to itself. The load reads it and
	// carries it through, because every later page of the same list has to
	// ask for the same view.
	const view = invoicesViewOf(url.searchParams);
	const order = invoiceOrderOf(url.searchParams, view);
	// #2871: the BFF takes two days, so a preset is counted here, in the Practice's zone.
	const period = invoicePeriodOf(url.searchParams);
	const now = new Date();
	const { session } = await parent();
	const days = invoicePeriodDays(period, session.timezone, now);
	const path = practiceInvoicesPath(params.practiceId, undefined, view, order, days);
	return {
		read: mapRead(streamRead<PracticeInvoicePage>(path, fetch), (invoices) => ({
			...invoices,
			view,
			order,
			period,
			days,
			now
		}))
	};
};
