import { describe, expect, it, vi } from 'vitest';
import { BlobDownload, triggerBlobDownload } from './blobDownload.js';

// `.svelte.spec.ts` here isn't a Svelte module -- it's the naming vite.config.ts
// routes to the real-Chromium `client` project (see its `include`), which is
// what `document`/`URL.createObjectURL` need. Node (the `server` project) has
// neither, and this repo doesn't stub a DOM with jsdom (see svelte-tests.md).
describe('triggerBlobDownload', () => {
	it('creates an object URL for the Blob and revokes it after the click', async () => {
		const blob = new Blob(['%PDF-1.4'], { type: 'application/pdf' });
		const createdURLs: string[] = [];
		const revokedURLs: string[] = [];
		const originalCreate = URL.createObjectURL;
		const originalRevoke = URL.revokeObjectURL;
		URL.createObjectURL = (b: Blob) => {
			const url = originalCreate.call(URL, b);
			createdURLs.push(url);
			return url;
		};
		URL.revokeObjectURL = (url: string) => {
			revokedURLs.push(url);
			originalRevoke.call(URL, url);
		};

		try {
			triggerBlobDownload(blob, 'signed-contract.pdf');
		} finally {
			URL.createObjectURL = originalCreate;
			URL.revokeObjectURL = originalRevoke;
		}

		expect(createdURLs).toHaveLength(1);
		expect(revokedURLs).toEqual(createdURLs);
	});
});

describe('BlobDownload', () => {
	it('hands the fetched file to the browser, and is busy only while it fetches', async () => {
		const clicked = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
		const download = new BlobDownload();
		let finish!: (blob: Blob) => void;

		const pending = download.start(() => new Promise<Blob>((resolve) => (finish = resolve)), 'birth-plan.pdf', 'Failed');
		expect(download.isBusy).toBe(true);
		finish(new Blob(['%PDF-1.4'], { type: 'application/pdf' }));

		expect(await pending).toBe(true);
		expect(download.isBusy).toBe(false);
		expect(download.error).toBe('');
		expect(clicked).toHaveBeenCalledOnce();
		clicked.mockRestore();
	});

	it("says why a download failed, in the thrown Error's own words or the fallback", async () => {
		const download = new BlobDownload();

		await download.start(async () => {
			throw new Error('The PDF is not ready yet');
		}, 'contract.pdf', 'Failed to download');
		expect(download.error).toBe('The PDF is not ready yet');

		await download.start(async () => {
			throw 'offline';
		}, 'contract.pdf', 'Failed to download');
		expect(download.error).toBe('Failed to download');
	});
});
