import type { ComponentProps } from 'svelte';
import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Field } from '#lib/planTemplate.js';
import PracticeQuestionPage from './PracticeQuestionPage.svelte';

const fields: Field[] = [
	{ id: 'first', type: 'short_text', label: 'Who do you want with you?', order: 0 },
	{ id: 'pain', type: 'single_select', label: 'Pain relief', options: ['None', 'Epidural'], order: 1 }
];

type Properties = ComponentProps<typeof PracticeQuestionPage<Field>>;

async function setup(overrides: Partial<Properties> = {}) {
	const onSave = vi.fn().mockResolvedValue(undefined);
	const onDone = vi.fn().mockResolvedValue(undefined);
	const properties: Properties = {
		name: 'spec',
		caption: 'Birth Plan',
		listHref: '/list',
		questionId: 'pain',
		kind: undefined,
		load: async () => fields,
		loadFailure: 'Failed to load',
		notOnList: 'Not on the list any more.',
		toField: (draft) => ({ ...draft, order: 0 }),
		onSave,
		retire: {
			verb: 'Remove',
			consequence: 'Plans already filled in keep it.',
			apply: (list, id) => list.filter((field) => field.id !== id),
			onDone
		},
		...overrides
	};
	await render(PracticeQuestionPage<Field>, properties);
	return { onSave, onDone };
}

describe('PracticeQuestionPage', () => {
	it('opens a saved question as it is, under the list it belongs to', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Change a question' })).toBeVisible();
		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveValue('Pain relief');
		await expect.element(page.getByText('Birth Plan')).toBeVisible();
	});

	it('sends the whole list back with the change in its place', async () => {
		const { onSave } = await setup();

		await page.getByLabelText('Question', { exact: true }).fill('Pain relief you want');
		await page.getByRole('button', { name: 'Save question' }).click();

		expect(onSave).toHaveBeenCalledWith([
			fields[0],
			{ id: 'pain', type: 'single_select', label: 'Pain relief you want', options: ['None', 'Epidural'], order: 1 }
		]);
	});

	it('sends a change of kind, answers and place', async () => {
		const { onSave } = await setup();

		await page.getByLabelText('Kind of answer').selectOptions('Choose several');
		await page.getByLabelText('Answers to choose from').fill('Nothing\nGas and air');
		await page.getByLabelText('Place in the list').selectOptions('1');
		await page.getByRole('button', { name: 'Save question' }).click();

		expect(onSave).toHaveBeenCalledWith([
			{ id: 'pain', type: 'multi_select', label: 'Pain relief', options: ['Nothing', 'Gas and air'], order: 0 },
			{ ...fields[0], order: 1 }
		]);
	});

	it('drops the answers when the kind stops being a choice', async () => {
		const { onSave } = await setup();

		await page.getByLabelText('Kind of answer').selectOptions('Short answer');
		await page.getByRole('button', { name: 'Save question' }).click();

		expect(onSave).toHaveBeenCalledWith([fields[0], { id: 'pain', type: 'short_text', label: 'Pain relief', order: 1 }]);
	});

	it('opens a question with no answers on an empty answers box', async () => {
		await setup({ questionId: 'first' });

		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveValue('Who do you want with you?');
		await expect.element(page.getByLabelText('Place in the list')).toHaveValue('1');
	});

	it('refuses a blank question and sends nothing', async () => {
		const { onSave } = await setup();

		await page.getByLabelText('Question', { exact: true }).fill('');
		await page.getByRole('button', { name: 'Save question' }).click();

		await expect.element(page.getByRole('link', { name: 'Enter the question' })).toBeVisible();
		expect(onSave).not.toHaveBeenCalled();
	});

	it('refuses the words the caller refuses', async () => {
		const { onSave } = await setup({ refuseLabel: (label) => (label === 'Pain relief' ? 'Already asked.' : undefined) });

		await page.getByRole('button', { name: 'Save question' }).click();

		await expect.element(page.getByRole('link', { name: 'Already asked.' })).toBeVisible();
		expect(onSave).not.toHaveBeenCalled();
	});

	it('retires the question once the dialog is confirmed, with its words', async () => {
		const { onDone } = await setup();

		await page.getByRole('button', { name: 'Remove question' }).click();
		await expect.element(page.getByRole('dialog', { name: 'Remove "Pain relief"?' })).toBeVisible();
		await page.getByRole('dialog').getByRole('button', { name: 'Remove question' }).click();

		expect(onDone).toHaveBeenCalledWith([fields[0]], 'Pain relief');
	});

	it('opens a new question empty, on the kind chosen, with nothing to retire', async () => {
		await setup({ questionId: 'new', kind: 'short_text' });

		await expect.element(page.getByRole('heading', { name: 'Add a question' })).toBeVisible();
		await expect.element(page.getByLabelText('Question', { exact: true })).toHaveValue('');
		await expect.element(page.getByRole('button', { name: 'Remove question' })).not.toBeInTheDocument();
	});

	it('sends a new question back to choose a kind when none was chosen', async () => {
		await setup({ questionId: 'new', kind: 'nonsense' });

		await expect.element(page.getByText('Choose the kind of answer first, from Add a question.')).toBeVisible();
	});

	it('keeps the kind of a saved question when the caller locks it', async () => {
		await setup({ locksKind: true });

		await expect.element(page.getByLabelText('Kind of answer')).toBeDisabled();
	});

	it('says so for a question the caller does not let this page change', async () => {
		await setup({ isEditable: () => false });

		await expect.element(page.getByText('Not on the list any more.')).toBeVisible();
	});

	it('says why the list could not be read, or the caller words when it gave no reason', async () => {
		await setup({
			load: () => Promise.reject(new Error('The list is down.'))
		});
		await expect.element(page.getByText('The list is down.')).toBeVisible();
	});

	it('says it is loading while the list is read', async () => {
		await setup({ load: () => new Promise(() => {}) });

		await expect.element(page.getByRole('status', { name: 'Loading the question' })).toBeInTheDocument();
	});

	it('falls back to the caller words for a rejection that carries none', async () => {
		await setup({ load: () => Promise.reject('down') });

		await expect.element(page.getByText('Failed to load')).toBeVisible();
	});
});
