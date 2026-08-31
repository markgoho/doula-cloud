import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import CheckAnswers, { type AnswerSection } from './CheckAnswers.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

const steps: JourneyStep[] = [
	{ label: 'Who Sarah is', href: '/clients/new/name', status: 'completed' },
	{ label: 'How to reach Sarah', href: '/clients/new/email', status: 'completed' },
	{ label: 'Check your answers', href: '/clients/new/check', status: 'current' }
];

const sections: AnswerSection[] = [
	{
		heading: 'Who Sarah is',
		answers: [
			{
				label: 'Given name',
				value: 'Sarah',
				changeHref: '/clients/new/name',
				changes: 'given name'
			},
			{
				label: 'Family name',
				value: 'Whitfield',
				changeHref: '/clients/new/name',
				changes: 'family name'
			}
		]
	},
	{
		heading: 'How to reach Sarah',
		answers: [
			{
				label: 'Email address',
				value: 'sarah@example.com',
				changeHref: '/clients/new/email',
				changes: 'email address'
			}
		]
	}
];

type SetupOptions = Partial<ComponentProps<typeof CheckAnswers>>;

async function setup(overrides: SetupOptions = {}) {
	return render(CheckAnswers, {
		props: {
			journey: 'Adding a client',
			steps,
			backHref: '/clients/new/email',
			title: 'Check your answers before adding Sarah',
			sections,
			actions: textSnippet('Save this client'),
			...overrides
		}
	});
}

describe('CheckAnswers.svelte', () => {
	it('renders the title as the page h1', async () => {
		await setup();

		await expect
			.element(
				page.getByRole('heading', { level: 1, name: 'Check your answers before adding Sarah' })
			)
			.toBeVisible();
	});

	it('renders one h2 per section and one row per answer', async () => {
		const { container } = await setup();

		await expect.element(page.getByRole('heading', { level: 2, name: 'Who Sarah is' })).toBeVisible();
		expect(container.querySelectorAll(':scope .row')).toHaveLength(3);
		expect([...container.querySelectorAll(':scope dt')].map((node) => node.textContent)).toEqual([
			'Given name',
			'Family name',
			'Email address'
		]);
	});

	/*
	 * Every row's link reads "Change", so on its own the page's link list is
	 * the same word repeated. GOV.UK answers that with visually-hidden text
	 * naming what changes; here it is joined by aria-describedby, because a
	 * Link atom takes a label and not children.
	 */
	it('names what each Change link changes', async () => {
		const { container } = await setup();

		const rows = [...container.querySelectorAll(':scope .row')];
		const named = rows.map((row) => {
			const link = row.querySelector('a');
			const describedBy = link?.getAttribute('aria-describedby') ?? '';
			return [link?.textContent?.trim(), row.querySelector(`#${describedBy}`)?.textContent];
		});
		expect(named).toEqual([
			['Change', 'given name'],
			['Change', 'family name'],
			['Change', 'email address']
		]);
	});

	it('sends each Change link back to the question it answers', async () => {
		const { container } = await setup();

		expect(container.querySelector(':scope .row a')).toHaveAttribute('href', '/clients/new/name');
	});

	it('renders the step rail as a landmark named after the journey', async () => {
		await setup();

		await expect.element(page.getByRole('navigation', { name: 'Adding a client' })).toBeVisible();
	});

	it('renders a back link above everything else in the column', async () => {
		const { container } = await setup();

		const links = [...container.querySelectorAll(':scope .column a')];
		expect(links[0]?.textContent?.trim()).toBe('Back');
	});

	it('renders the error summary above the h1, and nothing there without one', async () => {
		const { container } = await setup({ errorSummary: textSnippet('There is a problem') });

		const nodes = [...(container.querySelector('.column')?.querySelectorAll(':scope p, :scope h1') ?? [])];
		const summaryIndex = nodes.findIndex((node) => node.textContent === 'There is a problem');
		const headingIndex = nodes.findIndex((node) => node.tagName === 'H1');
		expect(summaryIndex).toBeGreaterThanOrEqual(0);
		expect(summaryIndex).toBeLessThan(headingIndex);

		const { container: bare } = await setup();
		expect(bare.textContent).not.toContain('There is a problem');
	});

	it('renders the section caption above the title', async () => {
		const { container } = await setup({ caption: 'Adding a client' });

		expect(container.querySelector('.caption')?.textContent).toBe('Adding a client');
	});

	it('renders no caption when none is given', async () => {
		const { container } = await setup();

		expect(container.querySelector('.caption')).toBeNull();
	});

	// GOV.UK's Check answers pattern allows a wider column for a long answer
	// list; the intake summary is nineteen rows on a Practice that has added
	// its own fields, so the wide case is real.
	it('takes the form width by default and the wide column when asked', async () => {
		const { container } = await setup();
		expect(container.querySelector('.column')).not.toHaveClass('wide');

		const { container: wide } = await setup({ isWide: true });
		expect(wide.querySelector('.column')).toHaveClass('wide');
	});

	it('renders the actions region', async () => {
		await setup();

		await expect.element(page.getByText('Save this client')).toBeVisible();
	});

	it('renders the page frame and no chrome beyond the journey rail', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'none');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('main')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
		expect(container.querySelectorAll('nav')).toHaveLength(1);
	});

	it('requires the answer sections and the actions at the type level', () => {
		// @ts-expect-error -- sections and actions are required regions, and
		// this unused directive is what fails `bun run check` if that ever
		// stops being true.
		const withoutSections: ComponentProps<typeof CheckAnswers> = {
			journey: 'Adding a client',
			steps,
			backHref: '/clients/new/email',
			title: 'Check your answers before adding Sarah'
		};

		expect(withoutSections.title).toBe('Check your answers before adding Sarah');
	});
});
