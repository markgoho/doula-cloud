import type { Fetcher } from './fetcher.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { SectionState } from './sectionState.svelte.js';

/**
 * Turns a Blob into a browser download: an object URL, a detached anchor
 * with `download` set, one click, then the URL is revoked. DOM work that
 * has no place in a module tested without one, per engagementDetail.ts's
 * own downloadAttachment -- this is the click half every caller of a
 * Blob-returning fetch (downloadAttachment, downloadClientSignedContractPdf,
 * downloadSignedContractPdf) was repeating by hand.
 */
export function triggerBlobDownload(blob: Blob, filename: string): void {
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	link.click();
	URL.revokeObjectURL(url);
}

/**
 * Fetches path and returns its body as a Blob, throwing with the response
 * body text on a non-2xx response -- the fetch half every PDF download
 * (Contract, Birth Plan; portal and Practice sides alike) repeats
 * identically, leaving only the path and Fetcher distinct.
 * triggerBlobDownload above is the DOM half.
 */
export async function fetchBlob(fetcher: Fetcher, path: string): Promise<Blob> {
	const response = await fetcher(path);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.blob();
}

/**
 * A download button's own state -- whether it is fetching, and the last
 * failure's words -- and the press itself, for every PDF a page offers
 * (#302's rule: a failed download is said in words, never swallowed,
 * because the anchor `triggerBlobDownload` clicks never reaches the
 * tree for a screen reader or a failed navigation to fall back on).
 *
 * The three PDF pages (the Staff and portal Birth Plans, the portal
 * Contract) each held an `isDownloadingPdf` and a `downloadError` and
 * the same try/catch/finally around them. It is `SectionState`'s
 * `mutate`, which already owns that lifecycle and refuses a second
 * press while one is in flight.
 */
export class BlobDownload extends SectionState<void> {
	constructor() {
		super();
	}

	/**
	 * Fetches the file and hands it to the browser as `filename`. On a
	 * thrown `Error`, `error` is its message; on anything else,
	 * `failureMessage`.
	 */
	async start(fetch: () => Promise<Blob>, filename: string, failureMessage: string): Promise<boolean> {
		return this.mutate(async () => {
			triggerBlobDownload(await fetch(), filename);
		}, failureMessage);
	}
}
