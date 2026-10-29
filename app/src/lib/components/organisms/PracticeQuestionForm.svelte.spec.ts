/*
 * #2193: one of a Practice's own questions on its own page -- its words,
 * its kind of answer, its answers to choose from on a choice question,
 * and its place in the list.
 */
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { FieldType } from '#lib/planTemplate.js';
import PracticeQuestionForm, { practiceQuestionIds } from './PracticeQuestionForm.svelte';

interface SetupOptions {
	type?: FieldType;
	position?: number;
	isNew?: boolean;
	positionCount?: number;
	isKindLocked?: boolean;
	errorFor?: (id: string) => string | undefined;
}

async function setup({
	type = 'single_select',
	position = 2,
	isNew = false,
	positionCount = 3,
	isKindLocked = false,
	errorFor
}: SetupOptions = {}) {
	const onLabelInput = vi.fn();
	const onTypeChange = vi.fn();
	const onOptionsInput = vi.fn();
	const onPositionChange = vi.fn();
	await render(PracticeQuestionForm, {
		name: 'q',
		type,
		label: 'Pain relief?',
		optionsText: 'None\nEpidural',
		position: isNew ? undefined : position,
		positionCount,
		isKindLocked,
		errorFor,
		onLabelInput,
		onTypeChange,
		onOptionsInput,
		onPositionChange
	});
	return { onLabelInput, onTypeChange, onOptionsInput, onPositionChange };
}

describe('PracticeQuestionForm', () => {
	it("holds the question's words, kind, answers and place, each labeled", async () => {
		await setup();

		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveValue('Pain relief?');
		await expect.element(page.getByLabelText('Kind of answer')).toHaveValue('single_select');
		await expect.element(page.getByLabelText('Answers to choose from')).toHaveValue('None\nEpidural');
		await expect.element(page.getByLabelText('Place in the list')).toHaveValue('2');
		await expect.element(page.getByRole('option', { name: '3 of 3' })).toBeInTheDocument();
	});

	it('says each change through its callback', async () => {
		const { onLabelInput, onTypeChange, onOptionsInput, onPositionChange } = await setup();

		await page.getByLabelText('Question', { exact: true }).fill('Pain relief, and when?');
		await page.getByLabelText('Kind of answer').selectOptions('Choose several');
		await page.getByLabelText('Answers to choose from').fill('Gas');
		await page.getByLabelText('Place in the list').selectOptions('1 of 3');

		expect(onLabelInput).toHaveBeenLastCalledWith('Pain relief, and when?');
		expect(onTypeChange).toHaveBeenCalledWith('multi_select');
		expect(onOptionsInput).toHaveBeenLastCalledWith('Gas');
		expect(onPositionChange).toHaveBeenCalledWith(1);
	});

	it('asks for answers to choose from only on a choice question, and calls a heading a heading', async () => {
		await setup({ type: 'section_header' });

		await expect.element(page.getByLabelText('Heading', { exact: true })).toBeVisible();
		expect(page.getByLabelText('Answers to choose from').query()).toBeNull();
	});

	it('offers no place to choose for a question not yet saved', async () => {
		await setup({ isNew: true });
		expect(page.getByLabelText('Place in the list').query()).toBeNull();
	});

	it('offers no place to choose in a list of one', async () => {
		await setup({ position: 1, positionCount: 1 });
		expect(page.getByLabelText('Place in the list').query()).toBeNull();
	});

	it('keeps a saved Client field to its kind of answer, and says so', async () => {
		await setup({ isKindLocked: true });

		await expect.element(page.getByLabelText('Kind of answer')).toBeDisabled();
		await expect.element(page.getByText('You cannot change this once it is saved.')).toBeVisible();
	});

	it('draws each refusal against its own control', async () => {
		const ids = practiceQuestionIds('q');
		await setup({
			errorFor: (id) => ({ [ids.label]: 'Enter the question', [ids.options]: 'Enter the answers' })[id]
		});

		await expect.element(page.getByText('Enter the question')).toBeVisible();
		await expect.element(page.getByText('Enter the answers')).toBeVisible();
		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveAttribute('aria-invalid', 'true');
	});
});
