import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Field } from '#lib/planTemplate.js';
import { PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
import QuestionFields from './QuestionFields.svelte';

const fields: Field[] = [
	{ id: 'first', type: 'short_text', label: 'Who do you want with you?', order: 0 },
	{ id: 'pain', type: 'single_select', label: 'Pain relief', options: ['None', 'Epidural'], order: 1 }
];

async function setup({ locksKind = false } = {}) {
	const onDone = vi.fn().mockResolvedValue(undefined);
	const question = new PracticeQuestion<Field>({
		name: 'spec',
		questionId: 'pain',
		kind: undefined,
		load: async () => fields,
		loadFailure: 'Failed to load',
		notOnList: 'Not on the list any more.',
		locksKind,
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave: async () => {},
		retire: {
			verb: 'Remove',
			consequence: 'Plans already filled in keep it.',
			apply: (list, id) => list.filter((field) => field.id !== id),
			onDone
		}
	});
	await question.load();
	await render(QuestionFields<Field>, { question });
	return { question, onDone };
}

describe('QuestionFields', () => {
	it('shows the question as the controller holds it', async () => {
		await setup();

		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveValue('Pain relief');
		await expect.element(page.getByLabelText('Answers to choose from')).toHaveValue('None\nEpidural');
		await expect.element(page.getByLabelText('Place in the list')).toHaveValue('2');
	});

	it('writes what is typed and chosen back to the controller', async () => {
		const { question } = await setup();

		await page.getByLabelText('Question', { exact: true }).fill('Pain relief you want');
		await page.getByLabelText('Answers to choose from').fill('Nothing');
		await page.getByLabelText('Kind of answer').selectOptions('Choose several');
		await page.getByLabelText('Place in the list').selectOptions('1');

		expect(question.label).toBe('Pain relief you want');
		expect(question.optionsText).toBe('Nothing');
		expect(question.type).toBe('multi_select');
		expect(question.position).toBe(1);
	});

	it('locks the kind of answer when the controller does', async () => {
		await setup({ locksKind: true });

		await expect.element(page.getByLabelText('Kind of answer')).toBeDisabled();
	});

	it('asks before retiring, in the question words, and retires on confirm', async () => {
		const { question, onDone } = await setup();

		question.isConfirmingRetire = true;
		await expect.element(page.getByRole('dialog', { name: 'Remove "Pain relief"?' })).toBeVisible();
		await page.getByRole('dialog').getByRole('button', { name: 'Remove question' }).click();

		expect(onDone).toHaveBeenCalledWith([fields[0]], 'Pain relief');
	});
});
