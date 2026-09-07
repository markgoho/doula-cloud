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
