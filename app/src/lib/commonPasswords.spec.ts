import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

/*
 * The browser and the BFF read two copies of one generated list (#1539):
 * Go's embed cannot reach outside api/, and the browser's chunk cannot
 * reach into it. The sync script writes both; this keeps a hand edit to
 * one of them from making the reset screen and the reset endpoint
 * disagree about a password.
 */
describe('the common-password list', () => {
	it('is the same bytes in app/ and in api/', () => {
		const browserCopy = readFileSync(new URL('commonPasswords.txt', import.meta.url), 'utf8');
		const bffCopy = readFileSync(
			new URL('../../../api/internal/staffauth/commonpasswords.txt', import.meta.url),
			'utf8'
		);

		expect(browserCopy.length).toBeGreaterThan(0);
		expect(bffCopy).toBe(browserCopy);
	});
});
