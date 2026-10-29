import { describe, expect, it } from 'vitest';
import {
	activeQuestionCount,
	optionsFromText,
	placeQuestion,
	questionErrors,
	questionPosition,
	withQuestion
} from './practiceQuestions.js';

interface Row {
	id: string;
	type: 'short_text' | 'single_select' | 'section_header';
	label: string;
	options?: string[];
	order: number;
	archived?: boolean;
}

const ids = (rows: readonly Row[]) => rows.map((row) => row.id);

const rows: Row[] = [
	{ id: 'a', type: 'short_text', label: 'A', order: 0 },
	{ id: 'b', type: 'short_text', label: 'B', order: 1 },
	{ id: 'c', type: 'short_text', label: 'C', order: 2 }
];

describe('questionPosition and activeQuestionCount', () => {
	it('counts places from 1, among the questions still asked', () => {
		const withArchived: Row[] = [rows[0], { ...rows[1], archived: true }, rows[2]];

		expect(questionPosition(rows, 'c')).toBe(3);
		expect(questionPosition(withArchived, 'c')).toBe(2);
		expect(activeQuestionCount(withArchived)).toBe(2);
	});

	it('is 0 for a question the list does not hold', () => {
		expect(questionPosition(rows, 'missing')).toBe(0);
	});
});

describe('placeQuestion', () => {
	it('moves a question to a place earlier or later in the list, renumbering order', () => {
		expect(ids(placeQuestion(rows, 'c', 1))).toEqual(['c', 'a', 'b']);
		expect(ids(placeQuestion(rows, 'a', 3))).toEqual(['b', 'c', 'a']);
		expect(placeQuestion(rows, 'a', 3).map((row) => row.order)).toEqual([0, 1, 2]);
	});

	it('leaves the list as it is for its own place, a question it does not hold, or a place out of range', () => {
		expect(ids(placeQuestion(rows, 'b', 2))).toEqual(['a', 'b', 'c']);
		expect(ids(placeQuestion(rows, 'missing', 1))).toEqual(['a', 'b', 'c']);
		expect(ids(placeQuestion(rows, 'a', 0))).toEqual(['a', 'b', 'c']);
		expect(ids(placeQuestion(rows, 'a', 4))).toEqual(['a', 'b', 'c']);
	});

	it('counts places among the questions still asked and leaves an archived one where it is', () => {
		const withArchived: Row[] = [rows[0], { ...rows[1], archived: true }, rows[2]];

		expect(ids(placeQuestion(withArchived, 'c', 1))).toEqual(['c', 'a', 'b']);
		expect(ids(placeQuestion(withArchived, 'a', 2))).toEqual(['b', 'c', 'a']);
	});
});

describe('withQuestion', () => {
	it('replaces the question with the same id, in its place', () => {
		const changed = withQuestion(rows, { ...rows[1], label: 'Changed' });

		expect(changed.map((row) => row.label)).toEqual(['A', 'Changed', 'C']);
	});

	it('adds a question the list does not hold at its end', () => {
		const added = withQuestion(rows, { id: 'd', type: 'short_text', label: 'D', order: 99 });

		expect(ids(added)).toEqual(['a', 'b', 'c', 'd']);
		expect(added.at(-1)?.order).toBe(3);
	});
});

describe('optionsFromText', () => {
	it('is one answer per line, trimmed, with blank lines dropped', () => {
		expect(optionsFromText('  Yes \n\n No\n ')).toEqual(['Yes', 'No']);
		expect(optionsFromText('')).toEqual([]);
	});
});

function refuseName(label: string): string | undefined {
	return label === 'Name' ? 'Name is on every record already' : undefined;
}

describe('questionErrors', () => {
	const targets = { label: 'label-id', options: 'options-id' };

	it('asks for the question, or the heading for a section heading', () => {
		expect(questionErrors({ type: 'short_text', label: ' ', options: [] }, targets)).toEqual([
			{ message: 'Enter the question', targetId: 'label-id' }
		]);
		expect(questionErrors({ type: 'section_header', label: '', options: [] }, targets)).toEqual([
			{ message: 'Enter the heading', targetId: 'label-id' }
		]);
	});

	it('asks for the answers to choose from on a choice question only', () => {
		expect(questionErrors({ type: 'single_select', label: 'Q', options: [] }, targets)).toEqual([
			{ message: 'Enter the answers to choose from, one on each line', targetId: 'options-id' }
		]);
		expect(questionErrors({ type: 'short_text', label: 'Q', options: [] }, targets)).toEqual([]);
		expect(questionErrors({ type: 'single_select', label: 'Q', options: ['Yes'] }, targets)).toEqual([]);
	});

	it("adds the caller's own refusal of the question's words", () => {
		expect(questionErrors({ type: 'short_text', label: 'Name', options: [] }, targets, refuseName)).toEqual([
			{ message: 'Name is on every record already', targetId: 'label-id' }
		]);
		expect(questionErrors({ type: 'short_text', label: 'Due', options: [] }, targets, refuseName)).toEqual([]);
	});
});
