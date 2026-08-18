import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DynamicFieldEditor from './DynamicFieldEditor.svelte';
import type { Field } from '#lib/planTemplate.js';

const defaultFields: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
	{ id: 'b', type: 'single_select', label: 'Pain management', options: ['Unmedicated', 'Epidural'], order: 1 }
];

interface SetupOptions {
	fields?: Field[];
}

async function setup({ fields = defaultFields }: SetupOptions = {}) {
	const onAdd = vi.fn();
	const onRemove = vi.fn();
	const onMoveUp = vi.fn();
	const onMoveDown = vi.fn();
	const onLabelChange = vi.fn();
	const onTypeChange = vi.fn();
	const onOptionsChange = vi.fn();

	await render(DynamicFieldEditor, {
		fields,
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	});

	return { onAdd, onRemove, onMoveUp, onMoveDown, onLabelChange, onTypeChange, onOptionsChange };
}

describe('DynamicFieldEditor.svelte', () => {
	it('renders a label input and type select for each field', async () => {
		await setup();

		const labelInputs = page.getByLabelText('Field label').elements() as HTMLInputElement[];
		expect(labelInputs.map((element) => element.value)).toEqual(['Support people', 'Pain management']);
	});

	it('renders an empty options textarea for a select field with no options', async () => {
		const fieldWithoutOptions: Field[] = [{ id: 'a', type: 'single_select', label: 'Location', order: 0 }];
		await setup({ fields: fieldWithoutOptions });

		await expect.element(page.getByLabelText('Options, one per line')).toHaveValue('');
	});

	it('renders an options textarea only for select-type fields', async () => {
		await setup();

		const optionsTextareas = page.getByLabelText('Options, one per line').elements();
		expect(optionsTextareas).toHaveLength(1);
		await expect.element(page.getByLabelText('Options, one per line')).toHaveValue('Unmedicated\nEpidural');
	});

	it('disables Move up on the first field and Move down on the last field', async () => {
		await setup();

		const moveUpButtons = page.getByRole('button', { name: 'Move up' }).elements() as HTMLButtonElement[];
		const moveDownButtons = page.getByRole('button', { name: 'Move down' }).elements() as HTMLButtonElement[];

		expect(moveUpButtons[0].disabled).toBe(true);
		expect(moveUpButtons[1].disabled).toBe(false);
		expect(moveDownButtons[0].disabled).toBe(false);
		expect(moveDownButtons[1].disabled).toBe(true);
	});

	it('calls onRemove with the field id when Remove is clicked', async () => {
		const { onRemove } = await setup();

		await page.getByRole('button', { name: 'Remove' }).nth(1).click();

		expect(onRemove).toHaveBeenCalledWith('b');
	});

	it('calls onMoveUp/onMoveDown with the field id when clicked', async () => {
		const { onMoveUp, onMoveDown } = await setup();

		await page.getByRole('button', { name: 'Move down' }).first().click();
		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		expect(onMoveDown).toHaveBeenCalledWith('a');
		expect(onMoveUp).toHaveBeenCalledWith('b');
	});

	it('calls onLabelChange when the label input changes', async () => {
		const { onLabelChange } = await setup();

		await page.getByLabelText('Field label').first().fill('New label');

		expect(onLabelChange).toHaveBeenCalledWith('a', 'New label');
	});

	it('calls onTypeChange when the field type select changes', async () => {
		const { onTypeChange } = await setup();

		await page.getByLabelText('Field type').first().selectOptions('long_text');

		expect(onTypeChange).toHaveBeenCalledWith('a', 'long_text');
	});

	it('calls onOptionsChange with the split, trimmed lines when the options textarea changes', async () => {
		const { onOptionsChange } = await setup();

		await page.getByLabelText('Options, one per line').fill('Home\n Hospital ');

		expect(onOptionsChange).toHaveBeenCalledWith('b', ['Home', 'Hospital']);
	});

	it('calls onAdd with the selected new-field type', async () => {
		const { onAdd } = await setup({ fields: [] });

		await page.getByLabelText('New field type').selectOptions('checkbox');
		await page.getByRole('button', { name: 'Add field' }).click();

		expect(onAdd).toHaveBeenCalledWith('checkbox');
	});
});
