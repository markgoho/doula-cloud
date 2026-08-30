import type { ComponentProps } from 'svelte';
import { createRawSnippet } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import QuestionPage from './QuestionPage.svelte';

function textSnippet(text: string) {
	return createRawSnippet(() => ({ render: () => `<p>${text}</p>` }));
}

// The content region is handed a `describedBy`, the way LabeledField hands
// one to its children, so the snippet has to take an argument.
function inputSnippet(id: string) {
	return createRawSnippet((argument: () => { describedBy: string | undefined }) => ({
		render: () => {
			const { describedBy } = argument();
			const described = describedBy ? ` aria-describedby="${describedBy}"` : '';
			return `<input id="${id}"${described} />`;
		}
	}));
}

const steps: JourneyStep[] = [
	{ label: 'Who Sarah is', href: '/clients/new/name', status: 'completed' },
	{ label: 'How to reach Sarah', href: '/clients/new/email', status: 'current' },
	{ label: 'Where Sarah lives', href: '/clients/new/address', status: 'todo' }
];

type SetupOptions = Partial<ComponentProps<typeof QuestionPage>>;

async function setup(overrides: SetupOptions = {}) {
	return render(QuestionPage, {
		props: {
			journey: 'Adding a client',
			steps,
			backHref: '/clients/new/name',
			question: { as: 'label', text: "What is Sarah's email address?", for: 'client-email' },
			content: inputSnippet('client-email'),
			actions: textSnippet('Continue'),
			...overrides
		}
	});
}

describe('QuestionPage.svelte', () => {
	/*
	 * The finding that broke FormPage (#432): the question has to *be* the
	 * h1, so a screen reader announces it once rather than twice. Both
	 * shapes are asserted, because they are two different trees and a prop
	 * cannot switch between them.
	 */
	it('resolves the question as label-as-h1, with the label inside the heading', async () => {
		const { container } = await setup();

		const heading = container.querySelector('h1');
		expect(heading?.textContent?.trim()).toBe("What is Sarah's email address?");
		expect(heading?.querySelector('label')?.getAttribute('for')).toBe('client-email');
		await expect
			.element(page.getByLabelText("What is Sarah's email address?"))
			.toBeInTheDocument();
	});

	it('resolves the question as legend-as-h1, with the heading inside the legend', async () => {
		const { container } = await setup({
			question: { as: 'legend', text: 'What is her date of birth?' },
			content: textSnippet('Month, day and year')
		});

		const legend = container.querySelector('legend');
		expect(legend?.querySelector('h1')?.textContent).toBe('What is her date of birth?');
		await expect
			.element(page.getByRole('heading', { level: 1, name: 'What is her date of birth?' }))
			.toBeVisible();
		await expect
			.element(page.getByRole('group', { name: 'What is her date of birth?' }))
			.toBeInTheDocument();
	});

	it('renders the step rail as a landmark named after the journey', async () => {
		await setup();

		await expect.element(page.getByRole('navigation', { name: 'Adding a client' })).toBeVisible();
	});

	it('renders a back link above everything else in the column', async () => {
		const { container } = await setup();

		const links = [...container.querySelectorAll(':scope .column a')];
		expect(links[0]?.textContent?.trim()).toBe('Back');
		expect(links[0]).toHaveAttribute('href', '/clients/new/name');
	});

	/*
	 * GOV.UK's position: below the back link, above the h1. This Template
	 * owns *where*; the component is #467's, and rendering any error markup
	 * of its own here is exactly the duplication that ticket exists to
	 * remove.
	 */
	it('renders the error summary below the back link and above the h1', async () => {
		const { container } = await setup({
			errorSummary: textSnippet('There is a problem')
		});

		const column = container.querySelector('.column');
		const nodes = [...(column?.querySelectorAll('a, p, h1') ?? [])];
		const backIndex = nodes.findIndex((node) => node.textContent?.trim() === 'Back');
		const summaryIndex = nodes.findIndex((node) => node.textContent === 'There is a problem');
		const headingIndex = nodes.findIndex((node) => node.tagName === 'H1');
		expect(backIndex).toBeLessThan(summaryIndex);
		expect(summaryIndex).toBeLessThan(headingIndex);
	});

	it('renders nothing in the error summary region when no snippet is passed', async () => {
		const { container } = await setup();

		expect(container.textContent).not.toContain('There is a problem');
		expect(container.querySelectorAll(':scope .column > stack-l > *')).toHaveLength(4);
	});

	// The hint is announced from the <fieldset> for a group, because a group
	// hint repeated on each of three date inputs is said three times.
	it('describes a fieldset question from the group, not from its controls', async () => {
		const { container } = await setup({
			question: { as: 'legend', text: 'What is her date of birth?' },
			hint: 'For example, 4 2 1990.',
			content: inputSnippet('dob-month')
		});

		const hintId = container.querySelector('fieldset')?.getAttribute('aria-describedby');
		expect(container.querySelector(`#${hintId}`)?.textContent).toBe('For example, 4 2 1990.');
		expect(container.querySelector('#dob-month')?.getAttribute('aria-describedby')).toBeNull();
	});

	it('hands a single input its own hint to be described by', async () => {
		const { container } = await setup({ hint: 'Optional. Without one Sarah cannot be invoiced.' });

		const hintId = container.querySelector('#client-email')?.getAttribute('aria-describedby');
		expect(container.querySelector(`#${hintId}`)?.textContent).toBe(
			'Optional. Without one Sarah cannot be invoiced.'
		);
	});

	// The caption is inside the heading and outside the label, so it is part
	// of the page's title and not part of the control's accessible name.
	it('puts the section caption inside the h1 and outside the label', async () => {
		const { container } = await setup({ caption: 'How to reach Sarah' });

		const heading = container.querySelector('h1');
		expect(heading?.textContent).toContain('How to reach Sarah');
		expect(heading?.querySelector('label')?.textContent).toBe("What is Sarah's email address?");
	});

	it('renders the actions region', async () => {
		await setup();

		await expect.element(page.getByText('Continue')).toBeVisible();
	});

	// ADR-0018: a Template owns page-level arrangement and no chrome. The
	// only landmark it renders is the journey rail, which is the amendment
	// this ticket records -- never <main>, and never a <header>.
	it('renders the page frame and no chrome beyond the journey rail', async () => {
		const { container } = await setup();

		expect(container.querySelector('center-l')).toHaveAttribute('max', 'var(--page-max)');
		expect(container.querySelector('center-l')).toHaveAttribute('gutters', 'var(--page-gutter)');
		expect(container.querySelector('main')).toBeNull();
		expect(container.querySelector('header')).toBeNull();
		expect(container.querySelectorAll('nav')).toHaveLength(1);
	});

	it('requires the question, its content and its actions at the type level', () => {
		// @ts-expect-error -- question, content and actions are required
		// regions, and this unused directive is what fails `bun run check`
		// if that ever stops being true.
		const withoutQuestion: ComponentProps<typeof QuestionPage> = {
			journey: 'Adding a client',
			steps,
			backHref: '/clients/new/name'
		};

		expect(withoutQuestion.journey).toBe('Adding a client');
	});
});
