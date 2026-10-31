import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PlanInstanceView from './PlanInstanceView.svelte';
import type { Answers, Field, Masthead } from '#lib/planInstance.js';

const defaultFields: Field[] = [
	{ id: 'heading', type: 'section_header', label: 'Birth Setting', order: 0 },
	{ id: 'location', type: 'single_select', label: 'Planned birth location', options: ['Home', 'Hospital'], order: 1 },
	{ id: 'atmosphere', type: 'long_text', label: 'Atmosphere preferences', order: 2 },
	{ id: 'consent-photos', type: 'checkbox', label: 'OK to take photos', order: 3 },
	{ id: 'notify', type: 'multi_select', label: 'Notify', options: ['Partner', 'Doula'], order: 4 }
];

const masthead: Masthead = {
	clientName: 'Hannah Sorensen',
	doulas: [],
	practiceName: 'Rooted Birth Collective',
	changedOn: '2026-10-27'
};

interface SetupOptions {
	fields?: Field[];
	answers?: Answers;
	masthead?: Masthead;
	headingLevel?: 2 | 3;
}

async function setup({ fields = defaultFields, answers = {}, masthead, headingLevel = 2 }: SetupOptions = {}) {
	await render(PlanInstanceView, { fields, answers, masthead, headingLevel });
}

function valueFor(label: string) {
	return page.getByText(label).element().nextElementSibling as HTMLElement;
}

describe('PlanInstanceView.svelte', () => {
	// #2209: the page that prints names the plan before its first question,
	// so a stranger in scrubs handed it knows whose it is and how current.
	it('opens with whose plan it is, her Doula, whose Practice, and the day it last changed', async () => {
		await setup({ masthead: { ...masthead, doulas: ['Maya Okonkwo'] } });

		await expect.element(page.getByText('Name')).toBeVisible();
		expect(valueFor('Name')).toHaveTextContent('Hannah Sorensen');
		expect(valueFor('Doula')).toHaveTextContent('Maya Okonkwo');
		expect(valueFor('Practice')).toHaveTextContent('Rooted Birth Collective');
		expect(valueFor('Last changed')).toHaveTextContent('Oct 27, 2026');
	});

	// #2355: the label the Engagement hub uses for more than one Doula.
	it('names every Doula on the Engagement under one Doulas row', async () => {
		await setup({ masthead: { ...masthead, doulas: ['Maya Okonkwo', 'Lena Vasquez'] } });

		await expect.element(page.getByText('Doulas', { exact: true })).toBeVisible();
		expect(valueFor('Doulas')).toHaveTextContent('Maya Okonkwo, Lena Vasquez');
	});

	it('names only the Practice while nobody is on the Engagement', async () => {
		await setup({ masthead });

		await expect.element(page.getByText('Practice', { exact: true })).toBeVisible();
		expect(page.getByText('Doula', { exact: true }).query()).toBeNull();
		expect(page.getByText('Doulas', { exact: true }).query()).toBeNull();
	});

	it('renders no masthead when none is given', async () => {
		await setup();

		await expect.element(page.getByText('Planned birth location')).toBeVisible();
		expect(page.getByText('Last changed').query()).toBeNull();
	});
	// #2018, #2243: the print pages put it under the <h1>, the Engagement
	// page under a section's <h2>.
	it('renders a section_header field at the level it is given', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 2, name: 'Birth Setting' })).toBeVisible();
	});

	it('renders a level-3 heading under a section heading', async () => {
		await setup({ headingLevel: 3 });

		await expect.element(page.getByRole('heading', { level: 3, name: 'Birth Setting' })).toBeVisible();
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
