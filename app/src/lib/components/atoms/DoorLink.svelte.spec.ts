import '#lib/styles/app.css';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DoorLink from './DoorLink.svelte';

interface SetupOptions {
	variant?: 'primary' | 'secondary';
}

async function setup({ variant }: SetupOptions = {}) {
	await render(DoorLink, {
		href: '/login',
		label: 'Staff log in',
		description: 'For doulas and practice owners.',
		variant
	});
	return { link: page.getByRole('link') };
}

describe('DoorLink', () => {
	it.each(['primary', 'secondary'] as const)(
		'is one %s link named for the door alone, with its line as the description',
		async (variant) => {
			const { link } = await setup({ variant });

			await expect.element(link).toBeVisible();
			await expect.element(link).toHaveAccessibleName('Staff log in');
			await expect.element(link).toHaveAccessibleDescription('For doulas and practice owners.');
			await expect.element(link).toHaveAttribute('href', '/login');
			expect(page.getByRole('link').all()).toHaveLength(1);
		}
	);

	// The whole door is the target, not only its name: the name and the
	// line both sit inside the one anchor's box.
	it('takes the whole door as its target, and never less than 44px tall', async () => {
		const { link } = await setup();
		const anchor = link.element();
		const box = anchor.getBoundingClientRect();
		const line = page.getByText('For doulas and practice owners.').element().getBoundingClientRect();

		expect(anchor.contains(page.getByText('For doulas and practice owners.').element())).toBe(true);
		expect(line.bottom).toBeLessThanOrEqual(box.bottom);
		expect(box.height).toBeGreaterThanOrEqual(44);
	});

	it.each(['primary', 'secondary'] as const)('shows a focus ring on the %s door from the keyboard', async (variant) => {
		const { link } = await setup({ variant });

		// `focusVisible: true` is the browser's own "as if from the keyboard"
		// flag; a Tab from the test runner does not reach into its frame.
		link.element().focus({ focusVisible: true } as FocusOptions);

		await expect.element(link).toHaveFocus();
		expect(link.element().matches(':focus-visible')).toBe(true);
		const style = getComputedStyle(link.element());
		expect(style.outlineStyle).toBe('solid');
		expect(style.outlineWidth).toBe('2px');
	});

	// The arrow marks the one primary door; the bordered doors carry none.
	it('draws the arrow on the primary door only', async () => {
		const primary = await setup({ variant: 'primary' });
		expect(primary.link.element().querySelector('svg')).not.toBeNull();
	});

	it('draws no arrow on a bordered door', async () => {
		const { link } = await setup({ variant: 'secondary' });
		// querySelector, case 2 of svelte-tests.md: the arrow is decorative,
		// aria-hidden, and has no role or name for an accessible query.
		expect(link.element().querySelector('svg')).toBeNull();
	});
});
