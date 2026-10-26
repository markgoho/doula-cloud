import type { CDPSession } from '@vitest/browser-playwright';
import { afterAll, beforeAll } from 'vitest';
import { cdp } from 'vitest/browser';

/*
 * A browser spec's own reader zone (#1982), the browser half of what
 * `dates.zone.spec.ts` does with `process.env.TZ` in Node. A spec that
 * prints an instant -- an Engagement's `createdAt` as "started Mar 12,
 * 2026" -- names a day that is only true in some zones: 20:00Z is still
 * Mar 12 in UTC and New York and already Mar 13 in Tokyo. CI runs in UTC
 * and the local gate in Eastern time, so a spec that trusted the runner's
 * zone passed in both and failed for anyone ahead of UTC.
 *
 * Chromium's own timezone override, through the Playwright provider's
 * session (`vitest/browser` alone declares a session with no `send`),
 * set before the file's tests and cleared after them: the page outlives
 * the file, and an empty `timezoneId` gives the runner's zone back.
 */
export function inReaderZone(timezoneId: string): void {
	beforeAll(async () => {
		await setTimezone(timezoneId);
	});
	afterAll(async () => {
		await setTimezone('');
	});
}

async function setTimezone(timezoneId: string): Promise<void> {
	const session: CDPSession = cdp();
	await session.send('Emulation.setTimezoneOverride', { timezoneId });
}
