import type { Fetcher } from './fetcher.js';

/**
 * What differs between lists: the words a person reads, the route under the
 * Practice that answers the POST, the response header that carries the row
 * count and the file name when the server gave none.
 */
export interface CsvExportKind {
	plural: string;
	singular: string;
	path: string;
	countHeader: string;
	fallbackName: string;
}

export type CsvExportResult = { blob: Blob; filename: string; count: number } | { refusal: string };

export function exportFailedWords(plural: string): string {
	return `Your ${plural} did not download. Nothing you saved is lost.`;
}

function tooManyDownloads(plural: string, retryAfter: string | null): string {
	const minutes = Math.ceil(Number(retryAfter) / 60);
	const wait = minutes > 0 ? `Try again in ${minutes} ${minutes === 1 ? 'minute' : 'minutes'}.` : 'Try again later.';
	return `You have downloaded ${plural} too many times this hour. ${wait}`;
}

/**
 * What a finished download says, in place: the server's file name and how
 * many rows are in it.
 */
export function downloadedWordsOf(kind: CsvExportKind, filename: string, count: number): string {
	return `Downloaded ${filename}, ${count} ${count === 1 ? kind.singular : kind.plural}.`;
}

/**
 * The file name the server gave, from `Content-Disposition`.
 */
export function exportFilename(disposition: string | null | undefined, fallback: string): string {
	return /filename="([^"]+)"/.exec(disposition ?? '')?.[1] ?? fallback;
}

/**
 * Asks for a list as a CSV. A refusal, and a connection that is not there,
 * come back as the words a person reads, never a status code: money is not
 * offline (ADR-0059).
 */
export async function requestCsvExport(
	fetcher: Fetcher,
	practiceId: string,
	kind: CsvExportKind,
	body: Record<string, unknown>
): Promise<CsvExportResult> {
	try {
		const response = await fetcher(`/api/practices/${practiceId}/${kind.path}/export`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		if (response.status === 429) return { refusal: tooManyDownloads(kind.plural, response.headers.get('Retry-After')) };
		if (response.status === 403) return { refusal: `Your role cannot download ${kind.plural}.` };
		if (!response.ok) return { refusal: exportFailedWords(kind.plural) };
		return {
			blob: await response.blob(),
			filename: exportFilename(response.headers.get('Content-Disposition'), kind.fallbackName),
			count: Number(response.headers.get(kind.countHeader))
		};
	} catch {
		return { refusal: exportFailedWords(kind.plural) };
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
