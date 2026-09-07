import type { Fetcher } from './fetcher.js';
import { apiErrorMessage } from './apiErrorMessage.js';

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
