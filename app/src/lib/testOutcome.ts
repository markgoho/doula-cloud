/**
 * What the screen a save lands on is told (#1710), for a spec whose
 * `goto` is a mock. The next `goto` records where it was sent and reads
 * the message the way the destination does, while the navigation runs.
 * Read the returned object after the save has navigated.
 */
import type { Mock } from 'vitest';
import { takeOutcome } from './outcome.js';

export interface Landing {
	href?: string;
	message?: string;
}

export function captureLanding(goto: Mock): Landing {
	const landing: Landing = {};
	goto.mockImplementationOnce(async (href: string) => {
		landing.href = href;
		landing.message = takeOutcome(new URL(href, 'https://example.test'));
	});
	return landing;
}
