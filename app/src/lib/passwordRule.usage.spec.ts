import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { globFiles } from './globFiles';

/*
 * ADR-0026's amendment for #1493: the rule is the same on every screen that
 * sets a password, and a screen that only reads one (/login, ReauthPrompt,
 * the MFA and recovery steps) checks nothing about it. The length rule is
 * a synchronous call any file could make; the common-password list (#1539)
 * is also a chunk of its own, and a new reader would load it on a screen
 * that never needs it. So the readers are named here.
 */
const appRoot = new URL('../../', import.meta.url).pathname;

const SCREENS_THAT_SET_A_PASSWORD = [
	'src/routes/(signed-out)/accept-invite/+page.svelte',
	'src/routes/(signed-out)/reset-password/+page.svelte',
	'src/routes/(signed-out)/signup/+page.svelte'
];

describe('who checks a password against the common-password list', () => {
	it('is the three screens that set a password, and nothing else', () => {
		const readers = globFiles('src/**/*.{svelte,ts}', { cwd: appRoot })
			.filter((file) => !/\.spec\.ts$/.test(file) && file !== 'src/lib/passwordRule.ts')
			.filter((file) => /isPasswordTooCommon|preloadCommonPasswords|commonPasswords\.js/.test(
				readFileSync(`${appRoot}${file}`, 'utf8')
			))
			.toSorted((a, b) => a.localeCompare(b));

		expect(readers).toEqual(SCREENS_THAT_SET_A_PASSWORD);
	});
});

describe('who tells a password manager the rule (#1813)', () => {
	it('is the three screens that set a password: a screen that reads one states no rule', () => {
		const readers = globFiles('src/**/*.svelte', { cwd: appRoot })
			// The atom only forwards the attribute it is given.
			.filter((file) => file !== 'src/lib/components/atoms/TextInput.svelte')
			.filter((file) => /PASSWORD_RULES|passwordrules=/.test(readFileSync(`${appRoot}${file}`, 'utf8')))
			.toSorted((a, b) => a.localeCompare(b));

		expect(readers).toEqual(SCREENS_THAT_SET_A_PASSWORD);
	});
});
