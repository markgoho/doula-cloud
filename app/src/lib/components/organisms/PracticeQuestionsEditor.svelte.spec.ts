import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PracticeQuestionsEditor from './PracticeQuestionsEditor.svelte';
import { FIELD_TYPES, moveField, type Field } from '#lib/planTemplate.js';

const defaultQuestions: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
	{ id: 'b', type: 'single_select', label: 'Pain management', options: ['Unmedicated', 'Epidural'], order: 1 }
];

interface SetupOptions {
	questions?: Field[];
	lockedTypeIds?: ReadonlySet<string>;
}

async function setup({ questions = defaultQuestions, lockedTypeIds }: SetupOptions = {}) {
	const onAdd = vi.fn();
	const onRemove = vi.fn();
	const onMoveUp = vi.fn();
	const onMoveDown = vi.fn();
	const onLabelChange = vi.fn();
	const onTypeChange = vi.fn();
	const onOptionsChange = vi.fn();

	const { container } = await render(PracticeQuestionsEditor, {
		questions,
		emptyMessage: 'No questions yet. Add one below.',
		removeLabel: 'Remove',
		lockedTypeIds,
		onAdd,
		onRemove,
		onMoveUp,
		onMoveDown,
		onLabelChange,
		onTypeChange,
		onOptionsChange
	});

	return { container, onAdd, onRemove, onMoveUp, onMoveDown, onLabelChange, onTypeChange, onOptionsChange };
}

// "Remove" (or Move up/down) alone doesn't say which question it acts on
// (#515); the distinguishing name is a sibling joined by
// aria-describedby, so no accessible query names it directly.
function describedByText(container: HTMLElement, button: ReturnType<typeof page.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return container.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('PracticeQuestionsEditor.svelte', () => {
	it('heads each question by its place in the list', async () => {
		await setup();

		await expect.element(page.getByRole('group', { name: 'Question 1' })).toBeVisible();
		await expect.element(page.getByRole('group', { name: 'Question 2' })).toBeVisible();
	});

	it('labels each question box "Question"', async () => {
		await setup();

		const inputs = page.getByLabelText('Question', { exact: true }).elements() as HTMLInputElement[];
		expect(inputs.map((element) => element.value)).toEqual(['Support people', 'Pain management']);
	});

	it('labels a section heading\'s box "Heading", not "Question"', async () => {
		await setup({ questions: [{ id: 'a', type: 'section_header', label: 'Labor', order: 0 }] });

		await expect.element(page.getByLabelText('Heading', { exact: true })).toHaveValue('Labor');
		expect(page.getByLabelText('Question', { exact: true }).elements()).toHaveLength(0);
	});

	// #1252: a person reads a name, never the stored identifier.
	it('shows each kind of answer by its human name, and no stored identifier', async () => {
		await setup();

		const kind = page.getByRole('group', { name: 'Question 1' }).getByLabelText('Kind of answer');
		await expect.element(kind).toHaveDisplayValue('Short answer');
		const names = [...(kind.element() as HTMLSelectElement).options].map((option) => option.text);
		expect(names).toEqual(['Short answer', 'Long answer', 'Choose one', 'Choose several', 'Yes or no', 'Section heading']);
		for (const type of FIELD_TYPES) {
			expect(page.getByText(type, { exact: true }).elements()).toHaveLength(0);
		}
	});

	it('labels the answers box and says one answer goes on each line, only for a choice question', async () => {
		await setup();

		const answers = page.getByLabelText('Answers to choose from');
		expect(answers.elements()).toHaveLength(1);
		await expect.element(answers).toHaveValue('Unmedicated\nEpidural');
		await expect.element(answers).toHaveAccessibleDescription('One answer on each line');
	});

	it('renders an empty answers box for a choice question with no options yet', async () => {
		await setup({ questions: [{ id: 'a', type: 'single_select', label: 'Location', order: 0 }] });

		await expect.element(page.getByLabelText('Answers to choose from')).toHaveValue('');
	});

	it('leaves the kind of answer open to change when no question is locked', async () => {
		await setup();

		await expect.element(page.getByLabelText('Kind of answer').first()).toBeEnabled();
		expect(page.getByText('You cannot change this once it is saved.').elements()).toHaveLength(0);
	});

	it('fixes the kind of answer of a locked question, and says why', async () => {
		await setup({ lockedTypeIds: new Set(['a']) });

		const kinds = page.getByLabelText('Kind of answer');
		await expect.element(kinds.first()).toBeDisabled();
		await expect.element(kinds.first()).toHaveAccessibleDescription('You cannot change this once it is saved.');
		await expect.element(kinds.nth(1)).toBeEnabled();
	});

	it('says there are no questions yet, and still offers every Add, when the list is empty', async () => {
		await setup({ questions: [] });

		await expect.element(page.getByText('No questions yet. Add one below.')).toBeVisible();
		expect(page.getByRole('group').elements()).toHaveLength(0);
		await expect.element(page.getByRole('button', { name: 'Add a short answer' })).toBeVisible();
	});

	it('shows no empty state while there is a question', async () => {
		await setup();

		await expect.element(page.getByText('No questions yet. Add one below.')).not.toBeInTheDocument();
	});

	// #1252: adding is one act, one button per kind, and no select.
	it('offers one Add button per kind of answer, each adding that kind', async () => {
		const { onAdd } = await setup({ questions: [] });

		const labels = [
			['Add a short answer', 'short_text'],
			['Add a long answer', 'long_text'],
			['Add a choose-one question', 'single_select'],
			['Add a choose-several question', 'multi_select'],
			['Add a yes-or-no question', 'checkbox'],
			['Add a section heading', 'section_header']
		] as const;
		for (const [label, type] of labels) {
			await page.getByRole('button', { name: label }).click();
			expect(onAdd).toHaveBeenLastCalledWith(type);
		}
		expect(page.getByRole('combobox').elements()).toHaveLength(0);
	});

	it('calls onRemove with the question id when Remove is clicked', async () => {
		const { onRemove } = await setup();

		await page.getByRole('button', { name: 'Remove' }).nth(1).click();

		expect(onRemove).toHaveBeenCalledWith('b');
	});

	it('marks Move up on the first question and Move down on the last unavailable, and a press there does nothing', async () => {
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

	it('calls onMoveUp/onMoveDown with the question id when clicked', async () => {
		const { onMoveUp, onMoveDown } = await setup();

		await page.getByRole('button', { name: 'Move down' }).first().click();
		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		expect(onMoveDown).toHaveBeenCalledWith('a');
		expect(onMoveUp).toHaveBeenCalledWith('b');
	});

	it('calls onLabelChange when a question box changes', async () => {
		const { onLabelChange } = await setup();

		await page.getByLabelText('Question', { exact: true }).first().fill('New label');

		expect(onLabelChange).toHaveBeenCalledWith('a', 'New label');
	});

	it('calls onTypeChange with the stored identifier when a kind of answer is chosen by name', async () => {
		const { onTypeChange } = await setup();

		await page.getByLabelText('Kind of answer').first().selectOptions('Long answer');

		expect(onTypeChange).toHaveBeenCalledWith('a', 'long_text');
	});

	it('calls onOptionsChange with the split, trimmed lines when the answers box changes', async () => {
		const { onOptionsChange } = await setup();

		await page.getByLabelText('Answers to choose from').fill('Home\n Hospital ');

		expect(onOptionsChange).toHaveBeenCalledWith('b', ['Home', 'Hospital']);
	});

	it("names each question's Move up/Move down/Remove button by the question it acts on", async () => {
		const { container } = await setup();

		expect(describedByText(container, page.getByRole('button', { name: 'Move up' }).nth(1))).toBe('Pain management');
		expect(describedByText(container, page.getByRole('button', { name: 'Move down' }).first())).toBe('Support people');
		expect(describedByText(container, page.getByRole('button', { name: 'Remove' }).nth(1))).toBe('Pain management');
	});

	it('falls back to "Untitled question" when the question has no words yet', async () => {
		const { container } = await setup({ questions: [{ id: 'a', type: 'short_text', label: '', order: 0 }] });

		expect(describedByText(container, page.getByRole('button', { name: 'Remove' }))).toBe('Untitled question');
	});
});

const threeQuestions: Field[] = [
	{ id: 'a', type: 'short_text', label: 'Support people', order: 0 },
	{ id: 'b', type: 'short_text', label: 'Music', order: 1 },
	{ id: 'c', type: 'short_text', label: 'Lighting', order: 2 }
];

// A move or an add is only seen to keep or move focus when the list
// really changes, so this setup feeds each change back in as the
// consumers do, not as a bare vi.fn().
async function setupChanging() {
	let questions = threeQuestions;
	const rendered = await render(PracticeQuestionsEditor, {
		questions,
		emptyMessage: 'No questions yet. Add one below.',
		removeLabel: 'Remove',
		onAdd: (type) => {
			questions = [...questions, { id: 'd', type, label: '', order: questions.length }];
			void rendered.rerender({ questions });
		},
		onRemove: vi.fn(),
		onMoveUp: (id: string) => {
			questions = moveField(questions, id, 'up');
			void rendered.rerender({ questions });
		},
		onMoveDown: (id: string) => {
			questions = moveField(questions, id, 'down');
			void rendered.rerender({ questions });
		},
		onLabelChange: vi.fn(),
		onTypeChange: vi.fn(),
		onOptionsChange: vi.fn()
	});
}

function questionBoxes() {
	return page.getByLabelText('Question', { exact: true });
}

describe('PracticeQuestionsEditor.svelte, focus after an add (Form Design Patterns, Add another)', () => {
	it('puts focus in the new question\'s box', async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Add a short answer' }).click();

		await expect.element(questionBoxes().nth(3)).toHaveFocus();
		await expect.element(page.getByRole('group', { name: 'Question 4' })).toBeVisible();
	});
});

describe('PracticeQuestionsEditor.svelte, focus after a move (#1920)', () => {
	it("keeps focus on the moved question's Move down when it moves to the middle", async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		await expect.element(page.getByRole('button', { name: 'Move down' }).nth(1)).toHaveFocus();
		await expect.element(questionBoxes().nth(1)).toHaveValue('Support people');
	});

	it("keeps focus on the moved question's Move down when it takes it to the end, and a second press does nothing", async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move down' }).nth(2);
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(questionBoxes().nth(2)).toHaveValue('Music');

		await userEvent.keyboard('{Enter}');

		await expect.element(moved).toHaveFocus();
		await expect.element(questionBoxes().nth(2)).toHaveValue('Music');
	});

	it("keeps focus on the moved question's Move up when it takes it to the top", async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		const moved = page.getByRole('button', { name: 'Move up' }).first();
		await expect.element(moved).toHaveFocus();
		await expect.element(moved).toHaveAttribute('aria-disabled', 'true');
		await expect.element(questionBoxes().first()).toHaveValue('Music');
	});
});

// The live region's text is the assertion, not its visibility: it is
// visually hidden on purpose, and read only by a screen reader.
function announced(): string | null {
	return page.getByRole('status').element().textContent;
}

describe('PracticeQuestionsEditor.svelte, what a move announces (#1955)', () => {
	it('has an empty status region in the page before the first move', async () => {
		await setupChanging();

		expect(announced()).toBe('');
	});

	it('announces the question and its new place after a move down', async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move down' }).first().click();

		await expect.poll(announced).toBe('Support people moved to 2 of 3');
	});

	it('announces the question and its new place after a move up', async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move up' }).nth(1).click();

		await expect.poll(announced).toBe('Music moved to 1 of 3');
	});

	it('announces nothing new when the end press moves nothing', async () => {
		await setupChanging();

		await page.getByRole('button', { name: 'Move down' }).nth(1).click();
		await expect.poll(announced).toBe('Music moved to 3 of 3');

		await userEvent.keyboard('{Enter}');

		await expect.poll(announced).toBe('Music moved to 3 of 3');
	});
});
