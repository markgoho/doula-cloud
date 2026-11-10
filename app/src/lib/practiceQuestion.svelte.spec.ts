import { describe, expect, it, vi } from 'vitest';
import type { Field } from '#lib/planTemplate.js';
import { PracticeQuestion, type PracticeQuestionOptions } from './practiceQuestion.svelte.js';

const fields: Field[] = [
	{ id: 'first', type: 'short_text', label: 'Who do you want with you?', order: 0 },
	{ id: 'pain', type: 'single_select', label: 'Pain relief', options: ['None', 'Epidural'], order: 1 }
];

async function setup(overrides: Partial<PracticeQuestionOptions<Field>> = {}) {
	const onSave = vi.fn().mockResolvedValue(undefined);
	const onDone = vi.fn().mockResolvedValue(undefined);
	const question = new PracticeQuestion<Field>({
		name: 'spec',
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
	});
	await question.load();
	return { question, onSave, onDone };
}

function submit() {
	return new SubmitEvent('submit', { cancelable: true });
}

describe('PracticeQuestion', () => {
	it('opens a saved question as it is', async () => {
		const { question } = await setup();

		expect(question.isLoaded).toBe(true);
		expect(question.title).toBe('Change a question');
		expect(question.label).toBe('Pain relief');
		expect(question.optionsText).toBe('None\nEpidural');
		expect(question.position).toBe(2);
		expect(question.positionCount).toBe(2);
		expect(question.retireVerb).toBe('Remove');
		expect(question.retireConsequence).toBe('Plans already filled in keep it.');
	});

	it('frames the page by its title, and says it is loading until the list is read', () => {
		const question = new PracticeQuestion<Field>({
			name: 'spec',
			questionId: 'new',
			kind: 'short_text',
			load: () => new Promise(() => {}),
			loadFailure: 'Failed to load',
			notOnList: 'Not on the list any more.',
			toField: (draft) => ({ ...draft, order: 0 }),
			onSave: async () => {},
			retire: { verb: 'Remove', consequence: '', apply: (list) => list, onDone: async () => {} }
		});

		expect(question.frame.question).toEqual({ as: 'legend', text: 'Add a question' });
		expect(question.waiting).toEqual({
			title: 'Add a question',
			fieldsets: [],
			read: { status: 'loading', label: 'Loading the question' }
		});
	});

	it('says why it cannot be shown instead of loading', async () => {
		const { question } = await setup({ load: () => Promise.reject(new Error('The list is down.')) });

		expect(question.waiting.read).toEqual({ status: 'failed', message: 'The list is down.' });
	});

	it('opens a question with no answers on an empty answers box', async () => {
		const { question } = await setup({ questionId: 'first' });

		expect(question.optionsText).toBe('');
		expect(question.position).toBe(1);
	});

	it('sends the whole list back with the change in its place', async () => {
		const { question, onSave } = await setup();

		question.label = 'Pain relief you want';
		await question.save(submit());

		expect(onSave).toHaveBeenCalledWith([
			fields[0],
			{ id: 'pain', type: 'single_select', label: 'Pain relief you want', options: ['None', 'Epidural'], order: 1 }
		]);
	});

	it('sends a change of kind, answers and place', async () => {
		const { question, onSave } = await setup();

		question.type = 'multi_select';
		question.optionsText = 'Nothing\nGas and air';
		question.position = 1;
		await question.save(submit());

		expect(onSave).toHaveBeenCalledWith([
			{ id: 'pain', type: 'multi_select', label: 'Pain relief', options: ['Nothing', 'Gas and air'], order: 0 },
			{ ...fields[0], order: 1 }
		]);
	});

	it('drops the answers when the kind stops being a choice', async () => {
		const { question, onSave } = await setup();

		question.type = 'short_text';
		await question.save(submit());

		expect(onSave).toHaveBeenCalledWith([fields[0], { id: 'pain', type: 'short_text', label: 'Pain relief', order: 1 }]);
	});

	it('refuses a blank question and sends nothing', async () => {
		const { question, onSave } = await setup();

		question.label = '';
		await question.save(submit());

		expect(question.submission.errors.map((error) => error.message)).toContain('Enter the question');
		expect(onSave).not.toHaveBeenCalled();
	});

	it('refuses the words the caller refuses', async () => {
		const { question, onSave } = await setup({
			refuseLabel: (label) => (label === 'Pain relief' ? 'Already asked.' : undefined)
		});

		await question.save(submit());

		expect(question.submission.errors.map((error) => error.message)).toContain('Already asked.');
		expect(onSave).not.toHaveBeenCalled();
	});

	it('retires the question with its words once confirmed', async () => {
		const { question, onDone } = await setup();

		question.isConfirmingRetire = true;
		await question.retire();

		expect(question.isConfirmingRetire).toBe(false);
		expect(onDone).toHaveBeenCalledWith([fields[0]], 'Pain relief');
	});

	it('names an untitled question when it is retired', async () => {
		const { question, onDone } = await setup();

		question.label = '';
		await question.retire();

		expect(onDone).toHaveBeenCalledWith([fields[0]], 'Untitled question');
	});

	it('opens a new question empty, on the kind chosen, with nothing to lock', async () => {
		const { question } = await setup({ questionId: 'new', kind: 'short_text', locksKind: true });

		expect(question.isLoaded).toBe(true);
		expect(question.isNew).toBe(true);
		expect(question.title).toBe('Add a question');
		expect(question.label).toBe('');
		expect(question.isKindLocked).toBe(false);
		expect(question.id).not.toBe('');
	});

	it('sends a new question back to choose a kind when none was chosen', async () => {
		const { question } = await setup({ questionId: 'new', kind: 'nonsense' });

		expect(question.waiting.read).toEqual({
			status: 'failed',
			message: 'Choose the kind of answer first, from Add a question.'
		});
		expect(question.isLoaded).toBe(false);
	});

	it('keeps the kind of a saved question when the caller locks it', async () => {
		const { question } = await setup({ locksKind: true });

		expect(question.isKindLocked).toBe(true);
	});

	it('says so for a question the caller does not let this page change', async () => {
		const { question } = await setup({ isEditable: () => false });

		expect(question.waiting.read).toEqual({ status: 'failed', message: 'Not on the list any more.' });
	});

	it('falls back to the caller words for a rejection that carries none', async () => {
		const { question } = await setup({ load: () => Promise.reject('down') });

		expect(question.waiting.read).toEqual({ status: 'failed', message: 'Failed to load' });
	});
});
