import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ClientFieldTemplateEditor from './ClientFieldTemplateEditor.svelte';
import { moveField, type Field } from '#lib/clientFieldTemplate.js';

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

	it('renders no "Archived fields" heading when nothing is archived', async () => {
		await setup();
		expect(page.getByRole('heading', { name: 'Archived fields' }).elements()).toHaveLength(0);
	});

	it('lists an archived field under a separate "Archived fields" heading, with no Move buttons', async () => {
		await setup({
			fields: [
				{ id: 'a', type: 'short_text', label: 'Active note', order: 0, archived: false },
				{ id: 'b', type: 'short_text', label: 'Old note', order: 1, archived: true }
			]
		});

		await expect.element(page.getByRole('heading', { name: 'Archived fields' })).toBeVisible();
		await expect.element(page.getByRole('listitem', { name: 'Old note, archived' })).toBeVisible();
		expect(page.getByRole('button', { name: 'Move up' }).elements()).toHaveLength(1);
		expect(page.getByRole('button', { name: 'Move down' }).elements()).toHaveLength(1);
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

	it('calls onArchiveToggle with the field id once the archive confirmation is confirmed', async () => {
		const { onArchiveToggle } = await setup();

		await page.getByRole('button', { name: 'Archive', exact: true }).first().click();
		expect(onArchiveToggle).not.toHaveBeenCalled();

		const dialog = page.getByRole('dialog', { name: 'Archive "Intake note"?' });
		await expect.element(dialog.getByText(/read-only/)).toBeVisible();
		await dialog.getByRole('button', { name: 'Archive field' }).click();

		expect(onArchiveToggle).toHaveBeenCalledWith('a');
	});

	it('does not call onArchiveToggle when the archive confirmation is canceled', async () => {
		const { onArchiveToggle } = await setup();

		await page.getByRole('button', { name: 'Archive', exact: true }).first().click();
		const dialog = page.getByRole('dialog', { name: 'Archive "Intake note"?' });
		await dialog.getByRole('button', { name: 'Cancel' }).click();

		expect(onArchiveToggle).not.toHaveBeenCalled();
	});

	it('calls onArchiveToggle with the field id when Unarchive is clicked, with no confirmation', async () => {
		const { onArchiveToggle } = await setup({
			fields: [{ id: 'a', type: 'short_text', label: 'Old note', order: 0, archived: true }]
		});

		await page.getByRole('button', { name: 'Unarchive' }).click();

		expect(onArchiveToggle).toHaveBeenCalledWith('a');
	});

	it('marks Move up on the first field and Move down on the last field unavailable, and a press there does nothing', async () => {
		const { onMoveUp, onMoveDown } = await setup();

		const moveUp = page.getByRole('button', { name: 'Move up' });
		const moveDown = page.getByRole('button', { name: 'Move down' });
		await expect.element(moveUp.first()).toHaveAttribute('aria-disabled', 'true');
		await expect.element(moveUp.nth(1)).not.toHaveAttribute('aria-disabled');
		await expect.element(moveDown.first()).not.toHaveAttribute('aria-disabled');
		await expect.element(moveDown.nth(1)).toHaveAttribute('aria-disabled', 'true');

		// Playwright's click refuses an aria-disabled button, so the press
		// is the keyboard's: the way a person reaches it with focus on it.
		(moveUp.first().element() as HTMLButtonElement).focus();
		await userEvent.keyboard('{Enter}');
		(moveDown.nth(1).element() as HTMLButtonElement).focus();
		await userEvent.keyboard('{Enter}');

		expect(onMoveUp).not.toHaveBeenCalled();
		expect(onMoveDown).not.toHaveBeenCalled();
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

// The archived field sits between two active ones on purpose: Move up and
// Move down walk the active list only, so an end of the list is an end of
// that list, not of `fields`.
const fieldsToMove: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Intake note', order: 0, archived: false },
	{ id: 'x', type: 'short_text', label: 'Old question', order: 1, archived: true },
	{ id: 'b', type: 'short_text', label: 'Due date source', order: 2, archived: false },
	{ id: 'c', type: 'short_text', label: 'Referral source', order: 3, archived: false }
];

// A move is only seen to keep or drop focus when the list really
// reorders, so this setup feeds each move back in as the consumer does
// (`fields = moveField(fields, id, direction)`), not as a bare vi.fn().
async function setupMoving() {
	let fields = fieldsToMove;
	const rendered = await render(ClientFieldTemplateEditor, {
		fields,
		existingIds: new Set<string>(),
		onAdd: vi.fn(),
		onArchiveToggle: vi.fn(),
		onMoveUp: (id: string) => {
			fields = moveField(fields, id, 'up');
			void rendered.rerender({ fields });
		},
		onMoveDown: (id: string) => {
			fields = moveField(fields, id, 'down');
			void rendered.rerender({ fields });
		},
		onLabelChange: vi.fn(),
		onTypeChange: vi.fn(),
		onOptionsChange: vi.fn()
	});
}

describe('ClientFieldTemplateEditor.svelte, focus after a move (#1920)', () => {
	it('keeps focus on the moved row\'s Move down when the row moves to the middle', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		const moved = page.getByRole('button', { name: 'Move down' }).nth(1);
		await expect.element(moved).toHaveFocus();
		await expect.element(page.getByLabelText('Field label').nth(1)).toHaveValue('Intake note');
	});

	it('keeps focus on the moved row\'s Move down when it takes the row to the end, and a second press does nothing', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move down' }).nth(2);
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(page.getByLabelText('Field label').nth(2)).toHaveValue('Due date source');

		await userEvent.keyboard('{Enter}');

		await expect.element(moved).toHaveFocus();
		await expect.element(page.getByLabelText('Field label').nth(2)).toHaveValue('Due date source');
	});

	it('keeps focus on the moved row\'s Move up when it takes the row to the top', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move up' }).first();
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(page.getByLabelText('Field label').first()).toHaveValue('Due date source');
	});
});

// The live region's text is the assertion, not its visibility: it is
// visually hidden on purpose, and read only by a screen reader.
function announced(): string | null {
	return page.getByRole('status').element().textContent;
}

// "of 3", not "of 4": the archived field is not in the list being
// reordered, so it is not counted in the position either.
describe('ClientFieldTemplateEditor.svelte, what a move announces (#1955)', () => {
	it('has an empty status region in the page before the first move', async () => {
		await setupMoving();

		expect(announced()).toBe('');
	});

	it('announces the field and its new position after a move down', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		await expect.poll(announced).toBe('Intake note moved to 2 of 3');
	});

	it('announces the field and its new position after a move up', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		await expect.poll(announced).toBe('Due date source moved to 1 of 3');
	});

	it('announces nothing new when the end press moves nothing', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();
		await expect.poll(announced).toBe('Due date source moved to 3 of 3');

		await userEvent.keyboard('{Enter}');

		await expect.poll(announced).toBe('Due date source moved to 3 of 3');
	});
});
