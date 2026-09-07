import { describe, expect, it } from 'vitest';
import { triggerBlobDownload } from './blobDownload.js';

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
