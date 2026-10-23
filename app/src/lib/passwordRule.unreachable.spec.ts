import { describe, expect, it, vi } from 'vitest';
import { isPasswordTooCommon, preloadCommonPasswords } from './passwordRule.js';

/*
 * The list is a chunk of its own (#1539), and a chunk can fail to load: a
 * dropped connection, or a deploy that replaced it under an open tab.
 */
const reachable = vi.hoisted(() => ({ value: false }));
vi.mock('./commonPasswords.js', () => {
	if (!reachable.value) throw new Error('chunk failed to load');
	return { COMMON_PASSWORDS: new Set(['passwordpassword']) };
});

describe('when the common-password list cannot be loaded', () => {
	it('refuses to answer rather than let the password through, and loads it again next time', async () => {
		// A preload that fails says nothing; the submit is what reports it.
		await preloadCommonPasswords();

		await expect(isPasswordTooCommon('passwordpassword')).rejects.toThrow();

		reachable.value = true;
		expect(await isPasswordTooCommon('passwordpassword')).toBe(true);
	});
});
