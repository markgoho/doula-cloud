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

	/*
	 * Queried out of the DOM rather than asserted visible: the rail is
	 * `display: none` until its page frame is 60rem wide, and a test
	 * renders into no container at all, so what is on screen here is the
	 * narrow strip. That is the component working, not the test working
	 * around it -- and it is why the strip's own assertions below can use
	 * `toBeVisible` and these cannot.
	 */
	it('renders one link per step', async () => {
		const { container } = await setup();

		expect([...container.querySelectorAll(':scope .rail a')].map((node) => node.textContent?.trim())).toEqual([
			'Who Sarah is',
			'How to reach Sarah',
			'Email address',
			'Phone number',
			'Where Sarah lives'
		]);
	});

	it('marks the current step as the current page', async () => {
		const { container } = await setup();

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

		expect([...container.querySelectorAll(':scope .questions a')].map((node) => node.textContent?.trim())).toEqual(
			['Email address', 'Phone number']
		);
	});

	// The summary page: every completed step opens, so the rail becomes
	// the whole answered journey at a glance (#432).
	it('expands every completed step, on the summary page', async () => {
		const { container } = await setup({ expand: 'completed' });

		expect([...container.querySelectorAll(':scope .questions a')].map((node) => node.textContent?.trim())).toEqual(
			["Sarah's name", 'Date of birth']
		);
	});

	it('names the step and its number in the narrow strip', async () => {
		const { container } = await setup();

		expect(container.querySelector('.summary')?.textContent).toBe(
			'Step 2 of 3 · How to reach Sarah'
		);
		expect(container.querySelector<HTMLElement>('.track-fill')?.style.inlineSize).toBe('33%');
	});

	// The summary page has no current step, so a step number would be a
	// lie; the count is what is true there.
	it('counts completed steps in the strip when no step is current', async () => {
		const { container } = await setup({
			steps: steps.map((step) => ({ ...step, status: 'completed' as const })),
			expand: 'completed'
		});

		expect(container.querySelector('.summary')?.textContent).toBe('3 of 3 steps completed');
		expect(container.querySelector<HTMLElement>('.track-fill')?.style.inlineSize).toBe('100%');
	});

	it('shows no progress and no steps when the journey is empty', async () => {
		const { container } = await setup({ steps: [] });

		expect(container.querySelector('.summary')?.textContent).toBe('0 of 0 steps completed');
		expect(container.querySelector<HTMLElement>('.track-fill')?.style.inlineSize).toBe('0%');
	});

	// Narrow has no room for the rail, so the full list is a page of its
	// own -- a route, and therefore #466's. Omitting the href omits the link
	// rather than rendering a dead one.
	it('links to the whole step list only when given somewhere to send it', async () => {
		await setup({ allStepsHref: '/clients/new/steps' });

		await expect.element(page.getByRole('link', { name: 'Show all steps' })).toBeVisible();
	});

	it('renders no "Show all steps" link without an href', async () => {
		const { container } = await setup();

		expect([...container.querySelectorAll(':scope a')].map((node) => node.textContent?.trim())).not.toContain(
			'Show all steps'
		);
	});
});
