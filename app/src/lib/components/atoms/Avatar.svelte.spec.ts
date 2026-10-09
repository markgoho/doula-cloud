import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Avatar, { initialsOf } from './Avatar.svelte';

async function setup({ firstName = 'Mark', lastName = 'Goho' }: { firstName?: string; lastName?: string } = {}) {
	await render(Avatar, { firstName, lastName });
}

describe('initialsOf', () => {
	it.each([
		['two names', 'Mark', 'Goho', 'MG'],
		['one name only', 'Prince', '', 'P'],
		// No split on white space (#1537): "Mary Anne" is one first name, so
		// the initials are M and S, never M, A and S.
		['a first name of two words', 'Mary Anne', 'Smith', 'MS'],
		['a last name of several words', 'Lena', 'de la Cruz', 'LD'],
		['lower-case names', 'dee', 'marchetti', 'DM'],
		['extra whitespace', '  Tasha ', ' Bell  ', 'TB'],
		['no name at all', '', '', '']
	])('takes %s to %s', (_case, firstName, lastName, expected) => {
		expect(initialsOf(firstName, lastName)).toBe(expected);
	});
});

describe('Avatar', () => {
	it('shows the initials', async () => {
		await setup();

		await expect.element(page.getByText('MG')).toBeVisible();
	});

	/*
	 * The circle never carries the identity on its own: it sits inside a
	 * control that names the person in real text, so announcing two initials
	 * as well would repeat a worse version of the same fact.
	 */
	it('is hidden from assistive technology', async () => {
		await setup();

		await expect.element(page.getByText('MG')).toHaveAttribute('aria-hidden', 'true');
	});
});
