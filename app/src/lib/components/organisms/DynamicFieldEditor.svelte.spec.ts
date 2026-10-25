import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import DynamicFieldEditor from './DynamicFieldEditor.svelte';
import { moveField, type Field } from '#lib/planTemplate.js';

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

	const { container } = await render(DynamicFieldEditor, {
		fields,
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	});

	return {
		container,
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	};
}

// "Remove" (or Move up/down) alone doesn't say which field it acts on
// (#515); the distinguishing name is a sibling joined by
// aria-describedby, the same pattern the Edit link fix (#513) and
// CheckAnswers' Change links use, so no accessible query names it
// directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
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

	it('names each row\'s Move up/Move down/Remove button by the field it acts on', async () => {
		const { container } = await setup();

		expect(
			describedByText(container, page.getByRole('button', { name: 'Move up' }).nth(1))
		).toBe('Pain management');
		expect(
			describedByText(container, page.getByRole('button', { name: 'Move down' }).first())
		).toBe('Support people');
		expect(describedByText(container, page.getByRole('button', { name: 'Remove' }).nth(1))).toBe(
			'Pain management'
		);
	});

	it('falls back to "Untitled field" when the field has no label yet', async () => {
		const unlabeled: Field[] = [{ id: 'a', type: 'short_text', label: '', order: 0 }];
		const { container } = await setup({ fields: unlabeled });

		expect(describedByText(container, page.getByRole('button', { name: 'Remove' }))).toBe(
			'Untitled field'
		);
	});
});

const threeFields: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
	{ id: 'b', type: 'short_text', label: 'Music', order: 1 },
	{ id: 'c', type: 'short_text', label: 'Lighting', order: 2 }
];

// A move is only seen to keep or drop focus when the list really
// reorders, so this setup feeds each move back in as the consumers do
// (`fields = moveField(fields, id, direction)`), not as a bare vi.fn().
async function setupMoving() {
	let fields = threeFields;
	const rendered = await render(DynamicFieldEditor, {
		fields,
		onAdd: vi.fn(),
		onRemove: vi.fn(),
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

describe('DynamicFieldEditor.svelte, focus after a move (#1920)', () => {
	it('keeps focus on the moved row\'s Move down when the row moves to the middle', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		const moved = page.getByRole('button', { name: 'Move down' }).nth(1);
		await expect.element(moved).toHaveFocus();
		await expect.element(page.getByLabelText('Field label').nth(1)).toHaveValue('Support people');
	});

	it('keeps focus on the moved row\'s Move down when it takes the row to the end, and a second press does nothing', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move down' }).nth(2);
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(page.getByLabelText('Field label').nth(2)).toHaveValue('Music');

		await userEvent.keyboard('{Enter}');

		await expect.element(moved).toHaveFocus();
		await expect.element(page.getByLabelText('Field label').nth(2)).toHaveValue('Music');
	});

	it('keeps focus on the moved row\'s Move up when it takes the row to the top', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move up' }).first();
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(page.getByLabelText('Field label').first()).toHaveValue('Music');
	});
});

// The live region's text is the assertion, not its visibility: it is
// visually hidden on purpose, and read only by a screen reader.
function announced(): string | null {
	return page.getByRole('status').element().textContent;
}

describe('DynamicFieldEditor.svelte, what a move announces (#1955)', () => {
	it('has an empty status region in the page before the first move', async () => {
		await setupMoving();

		expect(announced()).toBe('');
	});

	it('announces the field and its new position after a move down', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		await expect.poll(announced).toBe('Support people moved to 2 of 3');
	});

	it('announces the field and its new position after a move up', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		await expect.poll(announced).toBe('Music moved to 1 of 3');
	});

	it('announces nothing new when the end press moves nothing', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();
		await expect.poll(announced).toBe('Music moved to 3 of 3');

		await userEvent.keyboard('{Enter}');

		await expect.poll(announced).toBe('Music moved to 3 of 3');
	});
});
