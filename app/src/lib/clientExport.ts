import { clientOrderPhrase, NO_CARE_YET, type ClientOrder } from './client.js';
import { dueWhen, type ClientsDue } from './clientsDue.js';
import { downloadedWordsOf, requestCsvExport, type CsvExportKind, type CsvExportResult } from './csvExport.js';
import type { Fetcher } from './fetcher.js';

/**
 * What the Clients list shows, as the export is asked for it (#2891): the
 * applied narrowing, name included, never an unapplied draft. Each key is the
 * list's own request field.
 */
export interface ClientsExportScope {
	all: boolean;
	mine: boolean;
	owes: boolean;
	noContract: boolean;
	doula: string;
	name: string;
	due: string;
	dueFrom: string;
	dueTo: string;
	order: ClientOrder;
}

const CLIENTS: CsvExportKind = {
	plural: 'Clients',
	singular: 'Client',
	path: 'clients',
	countHeader: 'Client-Count',
	fallbackName: 'clients.csv'
};

/**
 * What a finished download says, in place: the server's file name and how
 * many Clients are in it.
 */
export function clientsDownloadedWords(filename: string, count: number): string {
	return downloadedWordsOf(CLIENTS, filename, count);
}

/**
 * Asks for every Client the scope matches as a CSV.
 */
export function requestClientsExport(fetcher: Fetcher, practiceId: string, scope: ClientsExportScope): Promise<CsvExportResult> {
	return requestCsvExport(fetcher, practiceId, CLIENTS, { ...scope });
}

function dueOf(month: unknown, from: unknown, to: unknown): ClientsDue | undefined {
	if (typeof month === 'string') return { month, from: '', to: '' };
	return typeof from === 'string' && typeof to === 'string' ? { month: '', from, to } : undefined;
}

/**
 * The Activity feed's line under "Clients downloaded": how many Clients left
 * and which slice of the book, from the row's diff. Whether a search was
 * applied is said, never its text.
 */
export function clientsExportedDetail(diff: Record<string, unknown>): string | undefined {
	const { rows, all, mine, owes, noContract, doulaId, due, dueFrom, dueTo, order, searched } = diff;
	if (typeof rows !== 'number') return undefined;
	const days = dueOf(due, dueFrom, dueTo);
	const parts = [
		owes === true && 'who owe money',
		noContract === true && 'with no Contract',
		mine === true && "the downloader's own",
		typeof doulaId === 'string' && "one Doula's",
		days && `due ${dueWhen(days)}`,
		all === true && `including ${NO_CARE_YET}`,
		typeof order === 'string' && clientOrderPhrase(order),
		searched === true && 'with a search'
	].filter(Boolean);
	const count = `${rows} ${rows === 1 ? 'Client' : 'Clients'}`;
	return parts.length > 0 ? `${count}: ${parts.join(', ')}` : count;
}
