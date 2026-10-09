import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { CONNECT_STATUS_BADGES } from './payments.js';

/*
 * #1589: the Practice overview and the Getting paid screen name the same
 * Connect states, and the words live in one table, `CONNECT_STATUS_BADGES`.
 * Neither screen's own spec can notice a screen that went back to a private
 * label map, because each compares against the shared table. This one reads
 * the two screens' source and fails on a label of the table written there.
 */
const SCREENS = [
	'../routes/practices/[practiceId]/+page.svelte',
	'../routes/practices/[practiceId]/settings/payments/+page.svelte'
];

describe('the Connect state labels', () => {
	it.each(SCREENS)('are not written out again in %s', (screen) => {
		const source = readFileSync(fileURLToPath(new URL(screen, import.meta.url)), 'utf8');

		for (const { label } of Object.values(CONNECT_STATUS_BADGES)) {
			expect(source, `${screen} writes the label "${label}" itself`).not.toContain(`'${label}'`);
			expect(source, `${screen} writes the label "${label}" itself`).not.toContain(`"${label}"`);
		}
	});
});
