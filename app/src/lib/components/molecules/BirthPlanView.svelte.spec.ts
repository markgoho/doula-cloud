import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BirthPlanView from './BirthPlanView.svelte';
import type { Answers, Field } from '#lib/planInstance.js';

const defaultFields: Field[] = [
	{ id: 'heading', type: 'section_header', label: 'Birth Setting', order: 0 },
	{ id: 'location', type: 'single_select', label: 'Planned birth location', options: ['Home', 'Hospital'], order: 1 },
	{ id: 'atmosphere', type: 'long_text', label: 'Atmosphere preferences', order: 2 },
	{ id: 'consent-photos', type: 'checkbox', label: 'OK to take photos', order: 3 },
	{ id: 'notify', type: 'multi_select', label: 'Notify', options: ['Partner', 'Doula'], order: 4 }
];

interface SetupOptions {
	fields?: Field[];
	answers?: Answers;
}

async function setup({ fields = defaultFields, answers = {} }: SetupOptions = {}) {
	await render(BirthPlanView, { fields, answers });
}

function valueFor(label: string) {
	return page.getByText(label).element().nextElementSibling as HTMLElement;
}

describe('BirthPlanView.svelte', () => {
	it('renders a heading for a section_header field', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Birth Setting' })).toBeInTheDocument();
	});

	it('renders the stored value for a text-shaped field', async () => {
		await setup({ answers: { location: 'Hospital', atmosphere: 'Quiet, dim lighting' } });

		await expect.element(page.getByText('Hospital')).toBeInTheDocument();
		await expect.element(page.getByText('Quiet, dim lighting')).toBeInTheDocument();
	});

	it('renders an em dash for a text-shaped field with no answer', async () => {
		await setup();

		await expect.element(valueFor('Planned birth location')).toHaveTextContent('—');
	});

	it('renders Yes for a checkbox field answered true', async () => {
		await setup({ answers: { 'consent-photos': true } });

		await expect.element(valueFor('OK to take photos')).toHaveTextContent('Yes');
	});

	it('renders No for a checkbox field answered false or unanswered', async () => {
		await setup();

		await expect.element(valueFor('OK to take photos')).toHaveTextContent('No');
	});

	it('renders the joined selected options for a multi_select field', async () => {
		await setup({ answers: { notify: ['Partner', 'Doula'] } });

		await expect.element(page.getByText('Partner, Doula')).toBeInTheDocument();
	});

	it('renders an em dash for a multi_select field with no selected options', async () => {
		await setup();

		await expect.element(valueFor('Notify')).toHaveTextContent('—');
	});

	// A `<dl>` may only directly contain `<dt>`, `<dd>` and `<div>`, so a
	// section heading sits between lists rather than inside one -- see the
	// component, and e2e/accessibility.e2e.ts for the scan that found it.
	it('renders one list per section, with each heading outside its list', async () => {
		await setup({
			fields: [
				{ id: 'unfiled', type: 'short_text', label: 'Who is coming', order: 0 },
				{ id: 'heading', type: 'section_header', label: 'Birth Setting', order: 1 },
				{ id: 'location', type: 'short_text', label: 'Planned birth location', order: 2 }
			]
		});

		const lists = document.querySelectorAll('dl');
		expect(lists).toHaveLength(2);
		for (const list of lists) {
			expect([...list.children].every((child) => child.tagName === 'DIV')).toBe(true);
		}
		// The fields asked before any heading keep a list of their own.
		expect(lists[0]?.textContent).toContain('Who is coming');
		expect(lists[1]?.textContent).toContain('Planned birth location');
	});

	it('renders a section heading that has no fields under it, and no empty list', async () => {
		await setup({
			fields: [{ id: 'heading', type: 'section_header', label: 'Birth Setting', order: 0 }]
		});

		await expect.element(page.getByRole('heading', { name: 'Birth Setting' })).toBeInTheDocument();
		expect(document.querySelectorAll('dl')).toHaveLength(0);
	});
});
