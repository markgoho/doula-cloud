import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ClientFieldTemplateEditor from './ClientFieldTemplateEditor.svelte';
import { moveField, type Field } from '#lib/clientFieldTemplate.js';

// What this wrapper adds to PracticeQuestionsEditor, whose own spec
// covers the cards, the names of the kinds, adding and moving: the
// archive rule, the archived list and the fixed kind of a saved field.
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

	const { container } = await render(ClientFieldTemplateEditor, {
		fields,
		existingIds,
		onAdd,
		onArchiveToggle,
		onMoveUp: vi.fn(),
		onMoveDown: vi.fn(),
		onLabelChange: vi.fn(),
		onTypeChange: vi.fn(),
		onOptionsChange: vi.fn()
	});

	return { container, onAdd, onArchiveToggle };
}

// The question a button acts on is a sibling joined by aria-describedby
// (#515), so no accessible query names it directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('ClientFieldTemplateEditor.svelte', () => {
	it('renders a card for each active field', async () => {
		await setup();

		const boxes = page.getByLabelText('Question', { exact: true }).elements() as HTMLInputElement[];
		expect(boxes.map((element) => element.value)).toEqual(['Intake note', 'Referral source']);
	});

	// #2015: every Practice starts with no extra questions, and says so.
	it('says there are no extra questions yet when no field is active', async () => {
		await setup({ fields: [{ ...defaultFields[0]!, archived: true }] });

		await expect.element(page.getByText('No extra questions yet. Add one below.')).toBeVisible();
		expect(page.getByLabelText('Question', { exact: true }).elements()).toHaveLength(0);
	});

	it('adds a field of the kind its Add button names', async () => {
		const { onAdd } = await setup({ fields: [] });

		await page.getByRole('button', { name: 'Add a yes-or-no question' }).click();

		expect(onAdd).toHaveBeenCalledWith('checkbox');
	});

	it('fixes the kind of answer of a field already saved, and only that one', async () => {
		await setup({ existingIds: new Set(['a']) });

		const kinds = page.getByLabelText('Kind of answer');
		await expect.element(kinds.first()).toBeDisabled();
		await expect.element(kinds.nth(1)).toBeEnabled();
	});

	it('renders no archived list when nothing is archived', async () => {
		await setup();

		expect(page.getByRole('heading', { name: 'Archived questions' }).elements()).toHaveLength(0);
	});

	it('lists an archived field apart, under its own heading, with no card or Move buttons', async () => {
		await setup({
			fields: [
				{ id: 'a', type: 'short_text', label: 'Active note', order: 0, archived: false },
				{ id: 'b', type: 'short_text', label: 'Old note', order: 1, archived: true }
			]
		});

		await expect.element(page.getByRole('heading', { name: 'Archived questions' })).toBeVisible();
		// The term, not the visually hidden name Collect again is described by.
		await expect.element(page.getByRole('term').getByText('Old note')).toBeVisible();
		await expect.element(page.getByText('No longer asked')).toBeVisible();
		expect(page.getByRole('group').elements()).toHaveLength(1);
		expect(page.getByRole('button', { name: 'Move up' }).elements()).toHaveLength(1);
	});

	it('names an unnamed archived field "Untitled question"', async () => {
		await setup({ fields: [{ id: 'a', type: 'short_text', label: '', order: 0, archived: true }] });

		await expect.element(page.getByText('Untitled question', { exact: true }).first()).toBeVisible();
	});

	it('calls onArchiveToggle with the field id once the archive confirmation is confirmed', async () => {
		const { onArchiveToggle } = await setup();

		await page.getByRole('button', { name: 'Archive', exact: true }).first().click();
		expect(onArchiveToggle).not.toHaveBeenCalled();

		const dialog = page.getByRole('dialog', { name: 'Archive "Intake note"?' });
		await expect.element(dialog.getByText(/read-only/)).toBeVisible();
		await dialog.getByRole('button', { name: 'Archive question' }).click();

		expect(onArchiveToggle).toHaveBeenCalledWith('a');
	});

	it('does not call onArchiveToggle when the archive confirmation is canceled', async () => {
		const { onArchiveToggle } = await setup();

		await page.getByRole('button', { name: 'Archive', exact: true }).first().click();
		const dialog = page.getByRole('dialog', { name: 'Archive "Intake note"?' });
		await dialog.getByRole('button', { name: 'Cancel' }).click();

		expect(onArchiveToggle).not.toHaveBeenCalled();
	});

	it('names an unnamed field "Untitled question" in the archive confirmation', async () => {
		await setup({ fields: [{ id: 'a', type: 'short_text', label: '', order: 0, archived: false }] });

		await page.getByRole('button', { name: 'Archive', exact: true }).click();

		await expect.element(page.getByRole('dialog', { name: 'Archive "Untitled question"?' })).toBeVisible();
	});

	it('calls onArchiveToggle with the field id when Collect again is clicked, with no confirmation', async () => {
		const { onArchiveToggle, container } = await setup({
			fields: [{ id: 'a', type: 'short_text', label: 'Old note', order: 0, archived: true }]
		});

		const collectAgain = page.getByRole('button', { name: 'Collect again' });
		expect(describedByText(container, collectAgain)).toBe('Old note');
		await collectAgain.click();

		expect(onArchiveToggle).toHaveBeenCalledWith('a');
	});

	it("names each field's Archive button by the field it acts on", async () => {
		const { container } = await setup();

		expect(describedByText(container, page.getByRole('button', { name: 'Archive', exact: true }).nth(1))).toBe(
			'Referral source'
		);
	});
});

// The archived field sits between two active ones on purpose: Move up and
// Move down walk the active list only, so an end of the list is an end of
// that list, and a place is counted in it.
const fieldsToMove: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Intake note', order: 0, archived: false },
	{ id: 'x', type: 'short_text', label: 'Old question', order: 1, archived: true },
	{ id: 'b', type: 'short_text', label: 'Due date source', order: 2, archived: false },
	{ id: 'c', type: 'short_text', label: 'Referral source', order: 3, archived: false }
];

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

function announced(): string | null {
	return page.getByRole('status').element().textContent;
}

describe('ClientFieldTemplateEditor.svelte, moving among active fields only', () => {
	it('marks the last active field\'s Move down unavailable, with an archived field after it in the template', async () => {
		await setupMoving();

		await expect.element(page.getByRole('button', { name: 'Move down' }).nth(2)).toHaveAttribute('aria-disabled', 'true');
	});

	// "of 3", not "of 4": the archived field is not in the list being reordered.
	it('counts the new place among the active fields only', async () => {
		await setupMoving();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		await expect.poll(announced).toBe('Intake note moved to 2 of 3');
		await expect.element(page.getByRole('button', { name: 'Move down' }).nth(1)).toHaveFocus();
	});
});
