import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PracticeSwitcher, { rolesLabel, type PracticeOption } from './PracticeSwitcher.svelte';

const RIVERSIDE: PracticeOption = {
	practiceId: 'p1',
	practiceName: 'Riverside Doula Collective',
	roles: ['owner', 'admin'],
	href: '/practices/p1'
};

const FINGER_LAKES: PracticeOption = {
	practiceId: 'p2',
	practiceName: 'Finger Lakes Birth Support',
	roles: ['doula'],
	href: '/practices/p2'
};

interface SetupOptions {
	practices?: PracticeOption[];
	currentPracticeId?: string;
}

async function setup({
	practices = [RIVERSIDE, FINGER_LAKES],
	currentPracticeId = 'p1'
}: SetupOptions = {}) {
	await render(PracticeSwitcher, { practices, currentPracticeId });
	return { trigger: page.getByRole('button', { name: /Riverside Doula Collective/ }) };
}

describe('rolesLabel', () => {
	it.each([
		[['owner', 'admin'], 'Owner, Admin'],
		[['doula'], 'Doula'],
		// A role the BFF grows before this map catches up still prints,
		// rather than vanishing from a person's own list of what she is.
		[['midwife'], 'Midwife'],
		[[], '']
	])('writes %s as "%s"', (roles, expected) => {
		expect(rolesLabel(roles)).toBe(expected);
	});
});

describe('PracticeSwitcher', () => {
	it('names the Practice a person is looking at', async () => {
		await setup();

		// `.first()`: the trigger names it, and so does its own row in the
		// panel below.
		await expect.element(page.getByText('Riverside Doula Collective').first()).toBeVisible();
	});

	it('lists one row per Membership, with the roles held at each', async () => {
		const { trigger } = await setup();

		await trigger.click();

		await expect
			.element(page.getByRole('link', { name: 'Finger Lakes Birth Support' }))
			.toHaveAttribute('href', '/practices/p2');
		await expect.element(page.getByText('Owner, Admin')).toBeVisible();
	});

	it('marks the current Practice', async () => {
		const { trigger } = await setup();

		await trigger.click();

		await expect
			.element(page.getByRole('link', { name: 'Riverside Doula Collective' }))
			.toHaveAttribute('aria-current', 'page');
	});

	/*
	 * A person with one Membership still sees her Practice's name -- but a
	 * control that opens a list of one is a promise the product cannot keep,
	 * so the caret and the panel appear only at two or more.
	 */
	it('offers no menu when there is nowhere to switch to', async () => {
		await setup({ practices: [RIVERSIDE] });

		await expect.element(page.getByText('Riverside Doula Collective')).toBeVisible();
		await expect.element(page.getByRole('button')).not.toBeInTheDocument();
	});

	it('renders nothing at all before the Memberships arrive', async () => {
		await setup({ practices: [], currentPracticeId: 'p1' });

		await expect.element(page.getByText('Riverside Doula Collective')).not.toBeInTheDocument();
	});
});
