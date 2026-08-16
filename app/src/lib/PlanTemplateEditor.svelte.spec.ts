import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PlanTemplateEditor from './PlanTemplateEditor.svelte';
import type { Field } from './planTemplate.js';

const fields: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
	{ id: 'b', type: 'single_select', label: 'Pain management', options: ['Unmedicated', 'Epidural'], order: 1 }
];

function noop() {}

describe('PlanTemplateEditor.svelte', () => {
	it('renders a label input and type select for each field', async () => {
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		const labelInputs = page.getByLabelText('Field label').elements() as HTMLInputElement[];
		expect(labelInputs.map((el) => el.value)).toEqual(['Support people', 'Pain management']);
	});

	it('renders an empty options textarea for a select field with no options', async () => {
		const fieldWithoutOptions: Field[] = [
			{ id: 'a', type: 'single_select', label: 'Location', order: 0 }
		];
		render(PlanTemplateEditor, {
			fields: fieldWithoutOptions,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		await expect.element(page.getByLabelText('Options, one per line')).toHaveValue('');
	});

	it('renders an options textarea only for select-type fields', async () => {
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		const optionsTextareas = page.getByLabelText('Options, one per line').elements();
		expect(optionsTextareas).toHaveLength(1);
		await expect
			.element(page.getByLabelText('Options, one per line'))
			.toHaveValue('Unmedicated\nEpidural');
	});

	it('disables Move up on the first field and Move down on the last field', async () => {
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		const moveUpButtons = page.getByRole('button', { name: 'Move up' }).elements() as HTMLButtonElement[];
		const moveDownButtons = page
			.getByRole('button', { name: 'Move down' })
			.elements() as HTMLButtonElement[];

		expect(moveUpButtons[0].disabled).toBe(true);
		expect(moveUpButtons[1].disabled).toBe(false);
		expect(moveDownButtons[0].disabled).toBe(false);
		expect(moveDownButtons[1].disabled).toBe(true);
	});

	it('calls onRemove with the field id when Remove is clicked', async () => {
		const onRemove = vi.fn();
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		const removeButtons = page.getByRole('button', { name: 'Remove' }).elements();
		removeButtons[1].dispatchEvent(new MouseEvent('click', { bubbles: true }));

		expect(onRemove).toHaveBeenCalledWith('b');
	});

	it('calls onMoveUp/onMoveDown with the field id when clicked', async () => {
		const onMoveUp = vi.fn();
		const onMoveDown = vi.fn();
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp,
			onMoveDown,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		await page.getByRole('button', { name: 'Move down' }).first().click();
		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		expect(onMoveDown).toHaveBeenCalledWith('a');
		expect(onMoveUp).toHaveBeenCalledWith('b');
	});

	it('calls onLabelChange when the label input changes', async () => {
		const onLabelChange = vi.fn();
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		await page.getByLabelText('Field label').first().fill('New label');

		expect(onLabelChange).toHaveBeenCalledWith('a', 'New label');
	});

	it('calls onTypeChange when the field type select changes', async () => {
		const onTypeChange = vi.fn();
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange,
			onOptionsChange: noop
		});

		await page.getByLabelText('Field type').first().selectOptions('long_text');

		expect(onTypeChange).toHaveBeenCalledWith('a', 'long_text');
	});

	it('calls onOptionsChange with the split, trimmed lines when the options textarea changes', async () => {
		const onOptionsChange = vi.fn();
		render(PlanTemplateEditor, {
			fields,
			onAdd: noop,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange
		});

		await page.getByLabelText('Options, one per line').fill('Home\n Hospital ');

		expect(onOptionsChange).toHaveBeenCalledWith('b', ['Home', 'Hospital']);
	});

	it('calls onAdd with the selected new-field type', async () => {
		const onAdd = vi.fn();
		render(PlanTemplateEditor, {
			fields: [],
			onAdd,
			onRemove: noop,
			onMoveUp: noop,
			onMoveDown: noop,
			onLabelChange: noop,
			onTypeChange: noop,
			onOptionsChange: noop
		});

		await page.getByLabelText('New field type').selectOptions('checkbox');
		await page.getByRole('button', { name: 'Add field' }).click();

		expect(onAdd).toHaveBeenCalledWith('checkbox');
	});
});
