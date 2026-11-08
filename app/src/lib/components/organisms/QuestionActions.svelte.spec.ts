import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Field } from '#lib/planTemplate.js';
import { PracticeQuestion } from '#lib/practiceQuestion.svelte.js';
import QuestionActions from './QuestionActions.svelte';

const fields: Field[] = [{ id: 'pain', type: 'short_text', label: 'Pain relief', order: 0 }];

async function setup({ questionId = 'pain' } = {}) {
	const question = new PracticeQuestion<Field>({
		name: 'spec',
		questionId,
		kind: 'short_text',
		load: async () => fields,
		loadFailure: 'Failed to load',
		notOnList: 'Not on the list any more.',
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave: async () => {},
		retire: {
			verb: 'Archive',
			consequence: 'Kept.',
			apply: (list) => list,
			onDone: async () => {}
		}
	});
	await question.load();
	await render(QuestionActions<Field>, { question });
	return { question };
}

describe('QuestionActions', () => {
	it('offers Save and the retire button for a saved question, which opens the confirmation', async () => {
		const { question } = await setup();

		await expect.element(page.getByRole('button', { name: 'Save question' })).toBeVisible();
		await page.getByRole('button', { name: 'Archive question' }).click();

		expect(question.isConfirmingRetire).toBe(true);
	});

	it('offers only Save for a question not yet saved', async () => {
		await setup({ questionId: 'new' });

		await expect.element(page.getByRole('button', { name: 'Save question' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Archive question' })).not.toBeInTheDocument();
	});
});
