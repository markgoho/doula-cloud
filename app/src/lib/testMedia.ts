import type { CDPSession } from '@vitest/browser-playwright';
import { afterAll, beforeAll } from 'vitest';
import { cdp } from 'vitest/browser';

/*
 * A browser spec's print media (#2212), the print half of what
 * `testZone.ts` does for the reader's zone. A component that keeps itself
 * off the printed sheet does it with its own `@media print` rule, and the
 * only way to see that rule apply is to put the page in print media:
 * headless Chromium cannot open a print preview, but it can emulate the
 * medium the preview lays out in.
 *
 * Chromium's own media override, through the Playwright provider's
 * session, set before the file's tests and cleared after them: the page
 * outlives the file, and an empty `media` gives the screen back.
 */
export function inPrintMedia(): void {
	beforeAll(async () => {
		await setMedia('print');
	});
	afterAll(async () => {
		await setMedia('');
	});
}

async function setMedia(media: string): Promise<void> {
	const session: CDPSession = cdp();
	await session.send('Emulation.setEmulatedMedia', { media });
}
