import type { ContractOrder, ContractsView } from './contract.js';
import { downloadedWordsOf, requestCsvExport, type CsvExportKind, type CsvExportResult } from './csvExport.js';
import type { Fetcher } from './fetcher.js';
import { sentPeriodWords, type SentDays } from './sentPeriod.js';

/**
 * What the Contracts list shows, as the export is asked for it (#2954): the
 * applied view, search, order, Mine, Doula and period, never an unapplied
 * draft.
 */
export interface ContractsExportScope {
	view: ContractsView;
	search: string;
	order: ContractOrder;
	isMine: boolean;
	doula: string;
	days?: SentDays;
}

const CONTRACTS: CsvExportKind = {
	plural: 'Contracts',
	singular: 'Contract',
	path: 'contracts',
	countHeader: 'Contract-Count',
	fallbackName: 'contracts.csv'
};

const VIEW_WORDS: Record<string, string> = {
	'not-signed': 'not signed',
	draft: 'Drafts',
	sent: 'sent',
	signed: 'signed',
	voided: 'voided',
	all: 'All'
};

const ORDER_WORDS: Record<string, string> = { oldest: 'oldest first', newest: 'newest first', client: 'Client A to Z' };

/**
 * The Activity feed's line under "Contracts downloaded": how many Contracts
 * left and which slice of the book, from the row's diff. Whether a search was
 * applied is said, never its text.
 */
export function contractsExportedDetail(diff: Record<string, unknown>): string | undefined {
	const { rows, view, mine, doulaId, sentFrom, sentTo, order, searched } = diff;
	const viewWords = typeof view === 'string' ? VIEW_WORDS[view] : undefined;
	if (typeof rows !== 'number' || !viewWords) return undefined;
	const days = typeof sentFrom === 'string' && typeof sentTo === 'string' ? { from: sentFrom, to: sentTo } : undefined;
	const parts = [
		viewWords,
		mine === true && "the downloader's own",
		typeof doulaId === 'string' && "one Doula's",
		days && `sent ${sentPeriodWords(days)}`,
		typeof order === 'string' && ORDER_WORDS[order],
		searched === true && 'with a search'
	].filter(Boolean);
	return `${rows} ${rows === 1 ? 'Contract' : 'Contracts'}: ${parts.join(', ')}`;
}

/**
 * What a finished download says, in place: the server's file name and how
 * many Contracts are in it.
 */
export function contractsDownloadedWords(filename: string, count: number): string {
	return downloadedWordsOf(CONTRACTS, filename, count);
}

/**
 * Asks for every Contract the scope matches as a CSV.
 */
export function requestContractsExport(fetcher: Fetcher, practiceId: string, scope: ContractsExportScope): Promise<CsvExportResult> {
	return requestCsvExport(fetcher, practiceId, CONTRACTS, {
		view: scope.view === 'not-signed' ? '' : scope.view,
		search: scope.search,
		mine: scope.isMine,
		doula: scope.doula,
		...(scope.order && { order: scope.order }),
		...(scope.days && { sentFrom: scope.days.from, sentTo: scope.days.to })
	});
}
