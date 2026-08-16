import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PlanInstanceForm from './PlanInstanceForm.svelte';
import type { Answers, Field } from './planInstance.js';

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

function setup({ fields = defaultFields, answers = {} }: SetupOptions = {}) {
	const onAnswerChange = vi.fn();
	const onToggleOption = vi.fn();

	render(PlanInstanceForm, { fields, answers, onAnswerChange, onToggleOption });

	return { onAnswerChange, onToggleOption };
}

describe('PlanInstanceForm.svelte', () => {
	it('renders no option checkboxes for a multi_select field with no options', async () => {
		const fieldWithoutOptions: Field[] = [{ id: 'notify', type: 'multi_select', label: 'Notify', order: 0 }];
		setup({ fields: fieldWithoutOptions });

		await expect.element(page.getByRole('group', { name: 'Notify' })).toBeInTheDocument();
		expect(page.getByRole('checkbox').elements()).toHaveLength(0);
	});

	it('renders a heading for a section_header field, with no input', async () => {
		setup();

		await expect.element(page.getByRole('heading', { name: 'Support' })).toBeInTheDocument();
	});

	it('renders the stored value for a short_text field', async () => {
		setup({ answers: { names: 'Jamie' } });

		await expect.element(page.getByLabelText('Support people')).toHaveValue('Jamie');
	});

	it('renders the stored value for a long_text field', async () => {
		setup({ answers: { notes: 'Prefers quiet' } });

		await expect.element(page.getByLabelText('Additional notes')).toHaveValue('Prefers quiet');
	});

	it('renders a checked checkbox when the stored answer is true', async () => {
		setup({ answers: { consent: true } });

		await expect.element(page.getByLabelText('Consent to photos')).toBeChecked();
	});

	it('renders an unchecked checkbox when the field has no answer yet', async () => {
		setup();

		await expect.element(page.getByLabelText('Consent to photos')).not.toBeChecked();
	});

	it('renders the selected option for a single_select field', async () => {
		setup({ answers: { location: 'Hospital' } });

		await expect.element(page.getByLabelText('Location')).toHaveValue('Hospital');
	});

	it('checks the selected options for a multi_select field', async () => {
		setup({ answers: { notify: ['Doula'] } });

		await expect.element(page.getByLabelText('Notify: Doula')).toBeChecked();
		await expect.element(page.getByLabelText('Notify: Partner')).not.toBeChecked();
	});

	it('calls onAnswerChange when the short_text input changes', async () => {
		const { onAnswerChange } = setup();

		await page.getByLabelText('Support people').fill('Alex');

		expect(onAnswerChange).toHaveBeenCalledWith('names', 'Alex');
	});

	it('calls onAnswerChange when the long_text textarea changes', async () => {
		const { onAnswerChange } = setup();

		await page.getByLabelText('Additional notes').fill('Prefers quiet');

		expect(onAnswerChange).toHaveBeenCalledWith('notes', 'Prefers quiet');
	});

	it('calls onAnswerChange when the checkbox changes', async () => {
		const { onAnswerChange } = setup();

		await page.getByLabelText('Consent to photos').click();

		expect(onAnswerChange).toHaveBeenCalledWith('consent', true);
	});

	it('calls onAnswerChange when the single_select changes', async () => {
		const { onAnswerChange } = setup();

		await page.getByLabelText('Location').selectOptions('Home');

		expect(onAnswerChange).toHaveBeenCalledWith('location', 'Home');
	});

	it('calls onToggleOption with the field id and option when a multi_select checkbox is clicked', async () => {
		const { onToggleOption } = setup();

		await page.getByLabelText('Notify: Partner').click();

		expect(onToggleOption).toHaveBeenCalledWith('notify', 'Partner');
	});
});
