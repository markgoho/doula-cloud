import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PlanInstanceForm from './PlanInstanceForm.svelte';
import type { Answers, Field } from '#lib/planInstance.js';

const defaultFields: Field[] = [
	{ id: 'heading', type: 'section_header', label: 'Support', order: 0 },
	{ id: 'names', type: 'short_text', label: 'Support people', order: 1 },
	{ id: 'notes', type: 'long_text', label: 'Additional notes', order: 2 },
	{ id: 'consent', type: 'checkbox', label: 'Consent to photos', order: 3 },
	{ id: 'location', type: 'single_select', label: 'Location', options: ['Home', 'Hospital'], order: 4 },
	{ id: 'notify', type: 'multi_select', label: 'Notify', options: ['Partner', 'Doula'], order: 5 }
];

interface SetupOptions {
	fields?: Field[];
	answers?: Answers;
}

async function setup({ fields = defaultFields, answers = {} }: SetupOptions = {}) {
	const onAnswerChange = vi.fn();
	const onToggleOption = vi.fn();

	await render(PlanInstanceForm, { fields, answers, onAnswerChange, onToggleOption });

	return { onAnswerChange, onToggleOption };
}

/* The options repeat across questions, so a box is found inside its own
   question's group, as a screen reader meets it. */
function notifyGroup() {
	return page.getByRole('group', { name: 'Notify' });
}

describe('PlanInstanceForm.svelte', () => {
	it('renders no option checkboxes for a multi_select field with no options', async () => {
		const fieldWithoutOptions: Field[] = [{ id: 'notify', type: 'multi_select', label: 'Notify', order: 0 }];
		await setup({ fields: fieldWithoutOptions });

		await expect.element(page.getByRole('group', { name: 'Notify' })).toBeInTheDocument();
		expect(page.getByRole('checkbox').elements()).toHaveLength(0);
	});

	it('renders a heading for a section_header field, with no input', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Support' })).toBeInTheDocument();
	});

	it('renders the stored value for a short_text field', async () => {
		await setup({ answers: { names: 'Jamie' } });

		await expect.element(page.getByLabelText('Support people')).toHaveValue('Jamie');
	});

	it('renders the stored value for a long_text field', async () => {
		await setup({ answers: { notes: 'Prefers quiet' } });

		await expect.element(page.getByLabelText('Additional notes')).toHaveValue('Prefers quiet');
	});

	it('renders a checked checkbox when the stored answer is true', async () => {
		await setup({ answers: { consent: true } });

		await expect.element(page.getByLabelText('Consent to photos')).toBeChecked();
	});

	it('renders an unchecked checkbox when the field has no answer yet', async () => {
		await setup();

		await expect.element(page.getByLabelText('Consent to photos')).not.toBeChecked();
	});

	it('renders the selected option for a single_select field', async () => {
		await setup({ answers: { location: 'Hospital' } });

		await expect.element(page.getByLabelText('Location')).toHaveValue('Hospital');
	});

	it('checks the selected options for a multi_select field', async () => {
		await setup({ answers: { notify: ['Doula'] } });

		await expect.element(notifyGroup().getByLabelText('Doula')).toBeChecked();
		await expect.element(notifyGroup().getByLabelText('Partner')).not.toBeChecked();
	});

	it('calls onAnswerChange when the short_text input changes', async () => {
		const { onAnswerChange } = await setup();

		await page.getByLabelText('Support people').fill('Alex');

		expect(onAnswerChange).toHaveBeenCalledWith('names', 'Alex');
	});

	it('calls onAnswerChange when the long_text textarea changes', async () => {
		const { onAnswerChange } = await setup();

		await page.getByLabelText('Additional notes').fill('Prefers quiet');

		expect(onAnswerChange).toHaveBeenCalledWith('notes', 'Prefers quiet');
	});

	it('calls onAnswerChange when the checkbox changes', async () => {
		const { onAnswerChange } = await setup();

		await page.getByLabelText('Consent to photos').click();

		expect(onAnswerChange).toHaveBeenCalledWith('consent', true);
	});

	it('calls onAnswerChange when the single_select changes', async () => {
		const { onAnswerChange } = await setup();

		await page.getByLabelText('Location').selectOptions('Home');

		expect(onAnswerChange).toHaveBeenCalledWith('location', 'Home');
	});

	it('calls onToggleOption with the field id and the option ticked', async () => {
		const { onToggleOption } = await setup({ answers: { notify: ['Doula'] } });

		await notifyGroup().getByLabelText('Partner').click();

		expect(onToggleOption).toHaveBeenCalledWith('notify', 'Partner');
	});

	it('calls onToggleOption with the field id and the option unticked', async () => {
		const { onToggleOption } = await setup({ answers: { notify: ['Doula', 'Partner'] } });

		await notifyGroup().getByLabelText('Doula').click();

		expect(onToggleOption).toHaveBeenCalledWith('notify', 'Doula');
	});

	// The first option is the unanswered state, and it stays selectable,
	// so a Staff member can take an answer back.
	it('clears a single_select answer through its Not answered yet option', async () => {
		const { onAnswerChange } = await setup({ answers: { location: 'Home' } });

		await page.getByLabelText('Location').selectOptions('Not answered yet');

		expect(onAnswerChange).toHaveBeenCalledWith('location', '');
	});

	it('starts a group at each section header, with the questions before the first one in a group of their own', async () => {
		await setup({
			fields: [
				{ id: 'intro', type: 'short_text', label: 'Due date notes', order: 0 },
				{ id: 'h1', type: 'section_header', label: 'Labor', order: 1 },
				{ id: 'pain', type: 'short_text', label: 'Pain relief', order: 2 },
				{ id: 'h2', type: 'section_header', label: 'After the birth', order: 3 }
			]
		});

		await expect.element(page.getByLabelText('Due date notes')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 3, name: 'Labor' })).toBeVisible();
		await expect.element(page.getByLabelText('Pain relief')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 3, name: 'After the birth' })).toBeVisible();
		expect(page.getByRole('list').elements()).toHaveLength(0);
	});

	it('draws nothing for a Plan with no fields', async () => {
		await setup({ fields: [] });

		expect(page.getByRole('textbox').elements()).toHaveLength(0);
		expect(page.getByRole('heading').elements()).toHaveLength(0);
	});
});
