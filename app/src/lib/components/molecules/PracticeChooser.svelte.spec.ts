import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { PracticeOption } from './PracticeSwitcher.svelte';
import PracticeChooser from './PracticeChooser.svelte';

const practices: PracticeOption[] = [
	{ practiceId: 'practice-1', practiceName: 'Riverside Doulas', roles: ['owner', 'doula'], employmentType: 'employee', href: '/practices/practice-1' },
	{ practiceId: 'practice-2', practiceName: 'Hilltop Doulas', roles: ['doula'], employmentType: 'contractor', href: '/practices/practice-2' }
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
		await expect.element(riverside).toHaveAccessibleDescription('Owner and Doula, as an employee');

		const hilltop = page.getByRole('link', { name: 'Hilltop Doulas', exact: true });
		await expect.element(hilltop).toHaveAttribute('href', '/practices/practice-2');
		await expect.element(hilltop).toHaveAccessibleDescription('Doula, as a contractor');
	});

	// #2070: SvelteKit reads the nearest preload attribute above a link. A
	// door that preloads on hover reads that Practice's session, which the
	// BFF records as her last-used Practice, so a pointer at rest would
	// choose for her.
	it('loads no door ahead of the click', async () => {
		await setup();

		for (const name of ['Riverside Doulas', 'Hilltop Doulas']) {
			const door = page.getByRole('link', { name, exact: true }).element();
			expect(door.closest<HTMLElement>('[data-sveltekit-preload-data]')?.dataset.sveltekitPreloadData).toBe(
				'false'
			);
		}
	});

	// A list, so a screen reader hears how many there are to choose from.
	it('is one list with an item per Practice', async () => {
		await setup();

		await expect.element(page.getByRole('list')).toBeVisible();
		expect(page.getByRole('listitem').all()).toHaveLength(2);
	});
});
