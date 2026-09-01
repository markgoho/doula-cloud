import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import StepRail, { type JourneyStep } from './StepRail.svelte';

const steps: JourneyStep[] = [
	{
		label: 'Who Sarah is',
		href: '/clients/new/name',
		status: 'completed',
		questions: [
			{ label: "Sarah's name", href: '/clients/new/name' },
			{ label: 'Date of birth', href: '/clients/new/date-of-birth' }
		]
	},
	{
		label: 'How to reach Sarah',
		href: '/clients/new/email',
		status: 'current',
		questions: [
			{ label: 'Email address', href: '/clients/new/email' },
			{ label: 'Phone number', href: '/clients/new/phone' }
		]
	},
	{ label: 'Where Sarah lives', href: '/clients/new/address', status: 'todo' }
];

type SetupOptions = Partial<ComponentProps<typeof StepRail>>;

async function setup(overrides: SetupOptions = {}) {
	return render(StepRail, { props: { journey: 'Adding a client', steps, ...overrides } });
}

describe('StepRail.svelte', () => {
	/*
	 * The ADR-0018 amendment this component carries: a journey rail is a
	 * named landmark, unlike RecordDetail's contents list, because its
	 * entries are routes rather than in-page anchors.
	 */
	it('is a navigation landmark named after the journey', async () => {
		await setup();

		await expect.element(page.getByRole('navigation', { name: 'Adding a client' })).toBeVisible();
	});

	// Unconditionally visible now (#564): the rail no longer has a narrower
	// alternative it is hidden behind, so every step's link is asserted
	// visible directly rather than queried out of a DOM that used to hide
	// it below 60rem.
	it('renders one visible link per step', async () => {
		await setup();

		for (const label of ['Who Sarah is', 'How to reach Sarah', 'Where Sarah lives']) {
			await expect.element(page.getByRole('link', { name: label })).toBeVisible();
		}
	});

	it('marks the current step as the current page', async () => {
		const { container } = await setup();

		// `aria-current` has no `getByRole` filter in this locator API, so
		// this is the wiring exception, not a shortcut past an accessible
		// query that exists.
		expect(container.querySelector('a[aria-current="page"]')?.textContent?.trim()).toBe(
			'How to reach Sarah'
		);
	});

	/*
	 * #432 asked for statuses rather than a percentage. The status is a
	 * sibling of the link, so without aria-describedby a keyboard user
	 * tabbing the rail never hears it.
	 */
	it('gives each step a status joined to its link', async () => {
		const { container } = await setup();

		const link = container.querySelector('a[aria-current="page"]');
		const statusId = link?.getAttribute('aria-describedby');
		expect(statusId).toBeTruthy();
		expect(container.querySelector(`#${statusId}`)?.textContent).toBe('In progress');
		expect([...container.querySelectorAll(':scope .status')].map((node) => node.textContent)).toEqual([
			'Completed',
			'In progress',
			'Not started'
		]);
	});

	it('expands the current step only, on a question page', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('link', { name: 'Email address' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Phone number' })).toBeVisible();
		// The completed step's own questions, which `expand: 'current'`
		// (the default) leaves closed -- not rendered at all, so there is
		// no accessible locator to assert absent; this is the exception
		// querySelector stays for.
		expect(container.querySelectorAll(':scope .questions')).toHaveLength(1);
		expect(
			[...container.querySelectorAll('a')].map((node) => node.textContent?.trim())
		).not.toContain("Sarah's name");
	});

	// The summary page: every completed step opens, so the rail becomes
	// the whole answered journey at a glance (#432).
	it('expands every completed step, on the summary page', async () => {
		await setup({ expand: 'completed' });

		await expect.element(page.getByRole('link', { name: "Sarah's name" })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Date of birth' })).toBeVisible();
	});

	it('names the step and its number in the progress summary', async () => {
		await setup();

		await expect
			.element(page.getByText('Step 2 of 3 · How to reach Sarah'))
			.toBeVisible();
	});

	// The summary page has no current step, so a step number would be a
	// lie; the count is what is true there.
	it('counts completed steps in the summary when no step is current', async () => {
		await setup({
			steps: steps.map((step) => ({ ...step, status: 'completed' as const })),
			expand: 'completed'
		});

		await expect.element(page.getByText('3 of 3 steps completed')).toBeVisible();
	});

	it('shows no progress and no steps when the journey is empty', async () => {
		const { container } = await setup({ steps: [] });

		await expect.element(page.getByText('0 of 0 steps completed')).toBeVisible();
		// The progress bar's own fill width, `aria-hidden` on its wrapper
		// (markup) so it carries no accessible signal at all -- the
		// deliberately-non-accessible exception, not a shortcut past one.
		expect(container.querySelector<HTMLElement>('.track-fill')?.style.inlineSize).toBe('0%');
	});
});
