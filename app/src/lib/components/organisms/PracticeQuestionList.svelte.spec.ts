/*
 * #2193: a Practice's own questions as GOV.UK's "Add another" list -- a
 * row per question with its kind of answer and a Change, and one "Add a
 * question". A reader who may not change the list reads the same rows
 * with neither.
 */
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import PracticeQuestionList, { type ListedQuestion } from './PracticeQuestionList.svelte';

const questions: ListedQuestion[] = [
	{ id: 'a', type: 'long_text', label: 'Who do you want in the room?' },
	{ id: 'b', type: 'section_header', label: 'After the birth' },
	{ id: 'c', type: 'checkbox', label: '' }
];

interface SetupOptions {
	list?: ListedQuestion[];
	isReadOnly?: boolean;
}

async function setup({ list = questions, isReadOnly = false }: SetupOptions = {}) {
	await render(PracticeQuestionList, {
		questions: list,
		emptyMessage: 'No questions yet.',
		changeHref: isReadOnly ? undefined : (question: ListedQuestion) => `/questions/${question.id}`,
		addHref: isReadOnly ? undefined : '/questions/kind'
	});
}

describe('PracticeQuestionList', () => {
	it('lists each question with the human name of its kind of answer', async () => {
		await setup();

		expect(page.getByRole('term').elements().map((term) => term.textContent?.trim())).toEqual([
			'Who do you want in the room?',
			'After the birth',
			'Untitled question'
		]);
		await expect.element(page.getByText('Long answer')).toBeVisible();
		await expect.element(page.getByText('Section heading')).toBeVisible();
		await expect.element(page.getByText('Yes or no')).toBeVisible();
	});

	it("gives each row a Change to the question's own page, named by its question", async () => {
		await setup();

		const changes = page.getByRole('link', { name: 'Change', exact: true });
		expect(changes.elements()).toHaveLength(3);
		await expect.element(changes.first()).toHaveAttribute('href', '/questions/a');
		await expect.element(changes.first()).toHaveAccessibleDescription('Who do you want in the room?');
		await expect.element(changes.last()).toHaveAttribute('href', '/questions/c');
		await expect.element(changes.last()).toHaveAccessibleDescription('Untitled question');
	});

	it('has one way to add a question, to the page that asks its kind', async () => {
		await setup();

		await expect.element(page.getByRole('link', { name: 'Add a question' })).toHaveAttribute('href', '/questions/kind');
		expect(page.getByRole('button').elements()).toHaveLength(0);
	});

	it('says the list is empty, and still offers the add', async () => {
		await setup({ list: [] });

		await expect.element(page.getByText('No questions yet.')).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Add a question' })).toBeVisible();
	});

	it('draws no Change and no add for a reader who may not change the list', async () => {
		await setup({ isReadOnly: true });

		await expect.element(page.getByText('Long answer')).toBeVisible();
		expect(page.getByRole('link').elements()).toHaveLength(0);
	});
});
