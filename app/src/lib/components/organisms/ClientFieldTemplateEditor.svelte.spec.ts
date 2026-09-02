import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ClientFieldTemplateEditor from './ClientFieldTemplateEditor.svelte';
import type { Field } from '#lib/clientFieldTemplate.js';

const defaultFields: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Intake note', order: 0, archived: false },
	{
		id: 'b',
		type: 'single_select',
		label: 'Referral source',
		options: ['Hospital', 'Word of mouth'],
		order: 1,
		archived: false
	}
];

interface SetupOptions {
	fields?: Field[];
	existingIds?: ReadonlySet<string>;
}

async function setup({ fields = defaultFields, existingIds = new Set() }: SetupOptions = {}) {
	const onAdd = vi.fn();
	const onArchiveToggle = vi.fn();
	const onMoveUp = vi.fn();
	const onMoveDown = vi.fn();
	const onLabelChange = vi.fn();
	const onTypeChange = vi.fn();
	const onOptionsChange = vi.fn();

	const { container } = await render(ClientFieldTemplateEditor, {
		fields,
		existingIds,
		onAdd,
		onArchiveToggle,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	});

	return {
		container,
		onAdd,
		onArchiveToggle,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	};
}

// "Archive"/"Move up"/"Move down" alone doesn't say which field it acts on
// (#515); the distinguishing name is a sibling joined by aria-describedby,
// the same pattern the Edit link fix (#513) and CheckAnswers' Change links
// use, so no accessible query names it directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('ClientFieldTemplateEditor.svelte', () => {
	it('renders a label input and type select for each field', async () => {
		await setup();

		const labelInputs = page.getByLabelText('Field label').elements() as HTMLInputElement[];
		expect(labelInputs.map((element) => element.value)).toEqual(['Intake note', 'Referral source']);
	});

	it('renders an empty options textarea for a select field with no options property at all', async () => {
		const fieldWithoutOptions: Field[] = [{ id: 'a', type: 'single_select', label: 'Location', order: 0, archived: false }];
		await setup({ fields: fieldWithoutOptions });

		await expect.element(page.getByLabelText('Options, one per line')).toHaveValue('');
	});

	it('renders an options textarea only for select-type fields', async () => {
		await setup();

		const optionsTextareas = page.getByLabelText('Options, one per line').elements();
		expect(optionsTextareas).toHaveLength(1);
		await expect.element(page.getByLabelText('Options, one per line')).toHaveValue('Hospital\nWord of mouth');
	});

	it('leaves the type select enabled for a field that is not yet in existingIds', async () => {
		await setup({ existingIds: new Set() });

		const typeSelects = page.getByLabelText('Field type').elements() as HTMLSelectElement[];
		expect(typeSelects[0].disabled).toBe(false);
	});

	it('disables the type select for a field already present in existingIds', async () => {
		await setup({ existingIds: new Set(['a']) });

		const typeSelects = page.getByLabelText('Field type').elements() as HTMLSelectElement[];
		expect(typeSelects[0].disabled).toBe(true);
		expect(typeSelects[1].disabled).toBe(false);
	});

	it('shows no archived indicator for an active field', async () => {
		await setup();
		expect(page.getByText('Archived -- no longer collected').elements()).toHaveLength(0);
	});

	it('shows the archived indicator for an archived field', async () => {
		await setup({
			fields: [{ id: 'a', type: 'short_text', label: 'Old note', order: 0, archived: true }]
		});
		await expect.element(page.getByText('Archived -- no longer collected')).toBeVisible();
	});

	it('falls back to "Untitled field" in the archived row aria-label when the label is blank', async () => {
		await setup({
			fields: [{ id: 'a', type: 'short_text', label: '', order: 0, archived: true }]
		});
		await expect.element(page.getByRole('listitem', { name: 'Untitled field, archived' })).toBeVisible();
	});

	it('labels the toggle button Archive for an active field and Unarchive for an archived one', async () => {
		await setup({
			fields: [
				{ id: 'a', type: 'short_text', label: 'Active', order: 0, archived: false },
				{ id: 'b', type: 'short_text', label: 'Archived', order: 1, archived: true }
			]
		});

		await expect.element(page.getByRole('button', { name: 'Archive', exact: true })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Unarchive' })).toBeVisible();
	});

	it('calls onArchiveToggle with the field id when the toggle button is clicked', async () => {
		const { onArchiveToggle } = await setup();

		await page.getByRole('button', { name: 'Archive' }).first().click();

		expect(onArchiveToggle).toHaveBeenCalledWith('a');
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

	it('names each row\'s Move up/Move down/Archive button by the field it acts on', async () => {
		const { container } = await setup();

		expect(
			describedByText(container, page.getByRole('button', { name: 'Move up' }).nth(1))
		).toBe('Referral source');
		expect(
			describedByText(container, page.getByRole('button', { name: 'Move down' }).first())
		).toBe('Intake note');
		expect(
			describedByText(container, page.getByRole('button', { name: 'Archive', exact: true }).nth(1))
		).toBe('Referral source');
	});

	it('falls back to "Untitled field" in the Move/Archive description when the label is blank', async () => {
		const unlabeled: Field[] = [{ id: 'a', type: 'short_text', label: '', order: 0, archived: false }];
		const { container } = await setup({ fields: unlabeled });

		expect(
			describedByText(container, page.getByRole('button', { name: 'Archive', exact: true }))
		).toBe('Untitled field');
	});
});
