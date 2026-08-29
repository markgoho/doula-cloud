import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Avatar, { initialsOf } from './Avatar.svelte';

async function setup({ name = 'Mark Goho' }: { name?: string } = {}) {
	await render(Avatar, { name });
}

describe('initialsOf', () => {
	it.each([
		['two names', 'Mark Goho', 'MG'],
		['one name', 'Prince', 'P'],
		// First and last only: a middle name adds a letter nobody reads at
		// 34px, and three no longer fit the circle.
		['three names', 'Renata Okonkwo Adeyemi', 'RA'],
		['a lower-case name', 'dee marchetti', 'DM'],
		['extra whitespace', '  Tasha   Bell  ', 'TB'],
		['no name at all', '', '']
	])('takes %s to %s', (_case, name, expected) => {
		expect(initialsOf(name)).toBe(expected);
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
