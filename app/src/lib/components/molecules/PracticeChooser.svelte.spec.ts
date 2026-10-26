import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { PracticeOption } from './PracticeSwitcher.svelte';
import PracticeChooser from './PracticeChooser.svelte';

const practices: PracticeOption[] = [
	{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner', 'doula'], href: '/practices/practice-1' },
	{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'], href: '/practices/practice-2' }
];

async function setup() {
	await render(PracticeChooser, { practices });
}

describe('PracticeChooser.svelte', () => {
	it('offers each Practice as one door, named for the Practice, with the roles held there as its line', async () => {
		await setup();

		const riverside = page.getByRole('link', { name: 'Riverside Doulas', exact: true });
		await expect.element(riverside).toBeVisible();
		await expect.element(riverside).toHaveAttribute('href', '/practices/practice-1');
		await expect.element(riverside).toHaveAccessibleDescription('Owner, Doula');

		const hilltop = page.getByRole('link', { name: 'Hilltop Doulas', exact: true });
		await expect.element(hilltop).toHaveAttribute('href', '/practices/practice-2');
		await expect.element(hilltop).toHaveAccessibleDescription('Doula');
	});

	// A list, so a screen reader hears how many there are to choose from.
	it('is one list with an item per Practice', async () => {
		await setup();

		await expect.element(page.getByRole('list')).toBeVisible();
		expect(page.getByRole('listitem').all()).toHaveLength(2);
	});
});
