import {
	downloadedWordsOf,
	exportFailedWords,
	exportFilename as exportFilenameOf,
	requestCsvExport,
	type CsvExportKind,
	type CsvExportResult
} from './csvExport.js';
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

const INVOICES: CsvExportKind = {
	plural: 'Invoices',
	singular: 'Invoice',
	path: 'invoices',
	countHeader: 'Invoice-Count',
	fallbackName: 'invoices.csv'
};

export const EXPORT_FAILED = exportFailedWords(INVOICES.plural);

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

/**
 * What a finished download says, in place (#2871): the server's file name and
 * how many Invoices are in it.
 */
export function downloadedWords(filename: string, count: number): string {
	return downloadedWordsOf(INVOICES, filename, count);
}

/**
 * The file name the server gave, from `Content-Disposition`.
 */
export function exportFilename(disposition: string | null | undefined): string {
	return exportFilenameOf(disposition, INVOICES.fallbackName);
}

export type InvoicesExportResult = CsvExportResult;

/**
 * Asks for every Invoice the scope matches as a CSV.
 */
export function requestInvoicesExport(fetcher: Fetcher, practiceId: string, scope: InvoicesExportScope): Promise<InvoicesExportResult> {
	return requestCsvExport(fetcher, practiceId, INVOICES, {
		view: scope.view,
		search: scope.search,
		...(scope.order && { order: scope.order }),
		...(scope.days && { sentFrom: scope.days.from, sentTo: scope.days.to })
	});
}

export { saveFile } from './csvExport.js';
