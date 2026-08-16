import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import BirthPlanView from './BirthPlanView.svelte';
import type { Answers, Field } from './planInstance.js';

const fields: Field[] = [
	{ id: 'heading', type: 'section_header', label: 'Birth Setting', order: 0 },
	{ id: 'location', type: 'single_select', label: 'Planned birth location', options: ['Home', 'Hospital'], order: 1 },
	{ id: 'atmosphere', type: 'long_text', label: 'Atmosphere preferences', order: 2 },
	{ id: 'consent-photos', type: 'checkbox', label: 'OK to take photos', order: 3 },
	{ id: 'notify', type: 'multi_select', label: 'Notify', options: ['Partner', 'Doula'], order: 4 }
];

describe('BirthPlanView.svelte', () => {
	it('renders a heading for a section_header field', async () => {
		render(BirthPlanView, { fields, answers: {} });

		await expect.element(page.getByRole('heading', { name: 'Birth Setting' })).toBeInTheDocument();
	});

	it('renders the stored value for a text-shaped field', async () => {
		const answers: Answers = { location: 'Hospital', atmosphere: 'Quiet, dim lighting' };
		render(BirthPlanView, { fields, answers });

		await expect.element(page.getByText('Hospital')).toBeInTheDocument();
		await expect.element(page.getByText('Quiet, dim lighting')).toBeInTheDocument();
	});

	it('renders an em dash for a text-shaped field with no answer', async () => {
		render(BirthPlanView, { fields, answers: {} });

		const sibling = page.getByText('Planned birth location').element().nextElementSibling as HTMLElement;
		await expect.element(sibling).toHaveTextContent('—');
	});

	it('renders Yes for a checkbox field answered true', async () => {
		const answers: Answers = { 'consent-photos': true };
		render(BirthPlanView, { fields, answers });

		const sibling = page.getByText('OK to take photos').element().nextElementSibling as HTMLElement;
		await expect.element(sibling).toHaveTextContent('Yes');
	});

	it('renders No for a checkbox field answered false or unanswered', async () => {
		render(BirthPlanView, { fields, answers: {} });

		const sibling = page.getByText('OK to take photos').element().nextElementSibling as HTMLElement;
		await expect.element(sibling).toHaveTextContent('No');
	});

	it('renders the joined selected options for a multi_select field', async () => {
		const answers: Answers = { notify: ['Partner', 'Doula'] };
		render(BirthPlanView, { fields, answers });

		await expect.element(page.getByText('Partner, Doula')).toBeInTheDocument();
	});

	it('renders an em dash for a multi_select field with no selected options', async () => {
		render(BirthPlanView, { fields, answers: {} });

		const sibling = page.getByText('Notify').element().nextElementSibling as HTMLElement;
		await expect.element(sibling).toHaveTextContent('—');
	});
});
