import type { Fetcher } from './fetcher.js';
import { invoiceOrderPhrase, type InvoiceOrder, type InvoicesView } from './invoice.js';
import { invoicePeriodWords, type InvoiceDays } from './invoicePeriod.js';

/**
 * What the Invoices list shows, as the export is asked for it (#2871): the
 * applied view, search, order and period, never an unapplied draft.
 */
export interface InvoicesExportScope {
	view: InvoicesView;
	search: string;
	order: InvoiceOrder;
	days?: InvoiceDays;
}

export const EXPORT_FAILED = 'Your Invoices did not download. Nothing you saved is lost.';
const EXPORT_NOT_ALLOWED = 'Your role cannot download Invoices.';

function tooManyDownloads(retryAfter: string | null): string {
	const minutes = Math.ceil(Number(retryAfter) / 60);
	const wait = minutes > 0 ? `Try again in ${minutes} ${minutes === 1 ? 'minute' : 'minutes'}.` : 'Try again later.';
	return `You have downloaded Invoices too many times this hour. ${wait}`;
}

const VIEW_WORDS: Record<string, string> = { all: 'All', unpaid: 'Unpaid', overdue: 'Overdue' };

/**
 * The Activity feed's line under "Invoices downloaded": how many Invoices
 * left and which slice of the book, from the row's diff. Whether a search
 * was applied is said, never its text.
 */
export function invoicesExportedDetail(diff: Record<string, unknown>): string | undefined {
	const { rows, view, sentFrom, sentTo, order, searched } = diff;
	const viewWords = typeof view === 'string' ? VIEW_WORDS[view] : undefined;
	if (typeof rows !== 'number' || !viewWords) return undefined;
	const days = typeof sentFrom === 'string' && typeof sentTo === 'string' ? { from: sentFrom, to: sentTo } : undefined;
	const orderWords = typeof order === 'string' ? invoiceOrderPhrase(order) : undefined;
	const parts = [
		viewWords,
		days && `sent ${invoicePeriodWords(days)}`,
		orderWords,
		searched === true ? 'with a search' : undefined
	].filter(Boolean);
	return `${rows} ${rows === 1 ? 'Invoice' : 'Invoices'}: ${parts.join(', ')}`;
}

const FALLBACK_NAME = 'invoices.csv';

/**
 * The file name the server gave, from `Content-Disposition`.
 */
export function exportFilename(disposition: string | null | undefined): string {
	return /filename="([^"]+)"/.exec(disposition ?? '')?.[1] ?? FALLBACK_NAME;
}

export type InvoicesExportResult = { blob: Blob; filename: string } | { refusal: string };

/**
 * Asks for every Invoice the scope matches as a CSV. A refusal, and a
 * connection that is not there, come back as the words a person reads, never
 * a status code: money is not offline (ADR-0059).
 */
export async function requestInvoicesExport(
	fetcher: Fetcher,
	practiceId: string,
	scope: InvoicesExportScope
): Promise<InvoicesExportResult> {
	try {
		const response = await fetcher(`/api/practices/${practiceId}/invoices/export`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				view: scope.view,
				search: scope.search,
				...(scope.order && { order: scope.order }),
				...(scope.days && { sentFrom: scope.days.from, sentTo: scope.days.to })
			})
		});
		if (response.status === 429) return { refusal: tooManyDownloads(response.headers.get('Retry-After')) };
		if (response.status === 403) return { refusal: EXPORT_NOT_ALLOWED };
		if (!response.ok) return { refusal: EXPORT_FAILED };
		return { blob: await response.blob(), filename: exportFilename(response.headers.get('Content-Disposition')) };
	} catch {
		return { refusal: EXPORT_FAILED };
	}
}

/**
 * Hands the file to the browser as a download.
 */
export function saveFile(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	document.body.append(link);
	link.click();
	link.remove();
	URL.revokeObjectURL(url);
}
