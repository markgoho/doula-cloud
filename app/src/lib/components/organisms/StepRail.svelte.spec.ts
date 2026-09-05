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
	 * Queried out of the DOM rather than asserted visible: the default
	 * `expand: 'current'` is a question page, where the list starts
	 * closed, so its contents are out of layout and out of the
	 * accessibility tree. That is the component working, not the test
	 * working around it -- and it is why the summary's own assertions
	 * below can use `toBeVisible` and these cannot.
	 */
	it('renders one link per step', async () => {
		const { container } = await setup();

		expect([...container.querySelectorAll(':scope details a')].map((node) => node.textContent?.trim())).toEqual([
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

	/*
	 * #585's whole mechanism, and the two assertions that replace a
	 * container query: the open state is derived from what the page is
	 * FOR, never from how much room it was given. A question page asks a
	 * question, so the journey is one collapsed line above it; the
	 * summary page IS the answered journey, so it opens.
	 */
	it('starts closed on a question page', async () => {
		const { container } = await setup();

		expect(container.querySelector('details')?.open).toBe(false);
	});

	it('starts open on the summary page', async () => {
		const { container } = await setup({ expand: 'completed' });

		expect(container.querySelector('details')?.open).toBe(true);
	});

	/*
	 * That it authors no width condition at all is not asserted here.
	 * `floor.svelte.spec.ts` discovers every `@container` in the
	 * component tree and fails any that is not in its registry with a
	 * written justification, so a query added back to this file fails
	 * that check by construction -- and re-asserting it here would be a
	 * second guardrail on the same rule, drifting the moment one moves.
	 */
	it('names the step and its number in the always-visible summary', async () => {
		const { container } = await setup();

		expect(container.querySelector('.summary')?.textContent).toBe(
			'Step 2 of 3 · How to reach Sarah'
		);
		expect(container.querySelector<HTMLElement>('.track-fill')?.style.inlineSize).toBe('33%');
	});

	// The summary page has no current step, so a step number would be a
	// lie; the count is what is true there.
	it('counts completed steps in the summary when no step is current', async () => {
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

	/*
	 * The "Show all steps" link is gone with the narrow presentation that
	 * needed it (#585). It existed to recover a step list that a width
	 * had taken away, and no width takes anything away now -- the list is
	 * in the page, one disclosure from the reader, on every screen.
	 */
	it('offers no link out to a separate step list', async () => {
		const { container } = await setup();

		expect([...container.querySelectorAll(':scope a')].map((node) => node.textContent?.trim())).not.toContain(
			'Show all steps'
		);
	});
});
