import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { describe, expect, it, vi } from 'vitest';
import ClientFieldAnswers from './ClientFieldAnswers.svelte';
import type { Field } from '#lib/clientFieldTemplate.js';
import type { FieldValue } from '#lib/intakeDraft.svelte.js';

const fields: Field[] = [
	{ id: 'allergies', type: 'short_text', label: 'Allergies', order: 0, archived: false },
	{ id: 'hopes', type: 'long_text', label: 'Hopes for the birth', order: 1, archived: false },
	{
		id: 'birthplace',
		type: 'single_select',
		label: 'Planned place of birth',
		options: ['Home', 'Birth center'],
		order: 2,
		archived: false
	},
	{
		id: 'attendees',
		type: 'multi_select',
		label: 'Who else is expected',
		options: ['Partner', 'Mother'],
		order: 3,
		archived: false
	},
	{ id: 'photos', type: 'checkbox', label: 'Consents to photographs', order: 4, archived: false }
];

async function setup(values: Record<string, FieldValue> = {}) {
	const onChange = vi.fn();
	await render(ClientFieldAnswers, { fields, values, onChange, idPrefix: 'intake-field-0' });
	return { onChange };
}

/* A box is found inside its own question's group, as a screen reader
   meets it: the legend names the question, the label the option. */
function attendeesGroup() {
	return page.getByRole('group', { name: 'Who else is expected' });
}

describe('ClientFieldAnswers', () => {
	it('asks every field the Practice defined, by its own label', async () => {
		await setup();

		await expect.element(page.getByLabelText('Allergies')).toBeVisible();
		await expect.element(page.getByLabelText('Hopes for the birth')).toBeVisible();
		await expect.element(page.getByLabelText('Planned place of birth')).toBeVisible();
		await expect.element(page.getByLabelText('Consents to photographs')).toBeVisible();
	});

	it('shows the answers already collected', async () => {
		await setup({ allergies: 'Peanuts', hopes: 'To move around freely' });

		await expect.element(page.getByLabelText('Allergies')).toHaveValue('Peanuts');
		await expect.element(page.getByLabelText('Hopes for the birth')).toHaveValue(
			'To move around freely'
		);
	});

	it('reports a short-text answer under its field id', async () => {
		const { onChange } = await setup();

		await page.getByLabelText('Allergies').fill('Peanuts');

		expect(onChange).toHaveBeenCalledWith('allergies', 'Peanuts');
	});

	it('reports a long-text answer under its field id', async () => {
		const { onChange } = await setup();

		await page.getByLabelText('Hopes for the birth').fill('Quiet');

		expect(onChange).toHaveBeenCalledWith('hopes', 'Quiet');
	});

	it('reports a single choice', async () => {
		const { onChange } = await setup();

		await page.getByLabelText('Planned place of birth').selectOptions('Home');

		expect(onChange).toHaveBeenCalledWith('birthplace', 'Home');
	});

	// The save is free, so every Practice-defined question needs a real
	// unanswered state -- and it stays selectable, so an answer can be
	// taken back (#2014).
	it('offers an unanswered state on a single choice, and lets an answer be taken back', async () => {
		const { onChange } = await setup({ birthplace: 'Home' });

		await page.getByLabelText('Planned place of birth').selectOptions('Not answered yet');

		expect(onChange).toHaveBeenCalledWith('birthplace', '');
	});

	it('groups a multi-select under its own question', async () => {
		await setup();

		await expect.element(page.getByRole('group', { name: 'Who else is expected' })).toBeVisible();
	});

	it('adds a choice to a multi-select', async () => {
		const { onChange } = await setup({ attendees: ['Partner'] });

		await attendeesGroup().getByLabelText('Mother').click();

		expect(onChange).toHaveBeenCalledWith('attendees', ['Partner', 'Mother']);
	});

	it('takes a choice back out of a multi-select', async () => {
		const { onChange } = await setup({ attendees: ['Partner', 'Mother'] });

		await attendeesGroup().getByLabelText('Partner').click();

		expect(onChange).toHaveBeenCalledWith('attendees', ['Mother']);
	});

	it('reads a multi-select value that is not a list as nothing chosen', async () => {
		const { onChange } = await setup({ attendees: 'Partner' });

		await attendeesGroup().getByLabelText('Mother').click();

		expect(onChange).toHaveBeenCalledWith('attendees', ['Mother']);
	});

	it('keeps a stored choice the question no longer offers', async () => {
		const { onChange } = await setup({ attendees: ['Doula'] });

		await attendeesGroup().getByLabelText('Partner').click();

		expect(onChange).toHaveBeenCalledWith('attendees', ['Partner', 'Doula']);
	});

	it('reports a checkbox as ticked or not', async () => {
		const { onChange } = await setup();

		await page.getByLabelText('Consents to photographs').click();

		expect(onChange).toHaveBeenCalledWith('photos', true);
	});

	it('shows a checkbox that is already ticked', async () => {
		await setup({ photos: true });

		await expect.element(page.getByLabelText('Consents to photographs')).toBeChecked();
	});

	it('carries no autocomplete token on a Client-about field', async () => {
		await setup();

		await expect.element(page.getByLabelText('Allergies')).toHaveAttribute('autocomplete', 'off');
	});

	it('renders a select with no options rather than refusing to render', async () => {
		const onChange = vi.fn();
		await render(ClientFieldAnswers, {
			fields: [
				{ id: 'empty', type: 'single_select', label: 'Nothing to choose', order: 0, archived: false },
				{ id: 'none', type: 'multi_select', label: 'Nothing to tick', order: 1, archived: false }
			],
			values: {},
			onChange,
			idPrefix: 'intake-field-0'
		});

		await expect.element(page.getByLabelText('Nothing to choose')).toBeVisible();
		await expect.element(page.getByRole('group', { name: 'Nothing to tick' })).toBeVisible();
	});
});
