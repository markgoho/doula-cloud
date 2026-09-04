import { describe, expect, it, vi } from 'vitest';
import {
	addField,
	archiveField,
	fieldCountWarning,
	isSelectType,
	loadTemplate,
	moveField,
	saveTemplate,
	unarchiveField,
	validateFields,
	type Field
} from './clientFieldTemplate.js';
import { jsonResponse } from './testResponse.js';

describe('isSelectType', () => {
	it('is true for single_select and multi_select', () => {
		expect(isSelectType('single_select')).toBe(true);
		expect(isSelectType('multi_select')).toBe(true);
	});

	it('is false for the other palette types', () => {
		expect(isSelectType('short_text')).toBe(false);
		expect(isSelectType('long_text')).toBe(false);
		expect(isSelectType('checkbox')).toBe(false);
		expect(isSelectType('section_header')).toBe(false);
	});
});

describe('loadTemplate', () => {
	it('fetches the practice path and returns the decoded template', async () => {
		const template = { fields: [] };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(template));

		const result = await loadTemplate(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/client-field-template');
		expect(result).toEqual(template);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		await expect(loadTemplate(fetcher, 'practice-1')).rejects.toThrow('not found');
	});
});

describe('saveTemplate', () => {
	it('PUTs fields as JSON to the practice path', async () => {
		const fields: Field[] = [{ id: 'f1', type: 'short_text', label: 'Note', order: 0, archived: false }];
		const saved = { fields };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(saved));

		const result = await saveTemplate(fetcher, 'practice-1', fields);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/client-field-template', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ fields })
		});
		expect(result).toEqual(saved);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner or Admin can do that', 403));

		await expect(saveTemplate(fetcher, 'practice-1', [])).rejects.toThrow(
			'only a Practice Owner or Admin can do that'
		);
	});
});

describe('addField', () => {
	it('appends a blank, unarchived field with no options for a non-select type', () => {
		const result = addField([], 'f1', 'short_text');
		expect(result).toEqual([
			{ id: 'f1', type: 'short_text', label: '', order: 0, archived: false, options: undefined }
		]);
	});

	it('appends a blank field with an empty options list for a select type', () => {
		const result = addField([], 'f1', 'single_select');
		expect(result[0].options).toEqual([]);
	});

	it('does not mutate the input array', () => {
		const original: Field[] = [];
		addField(original, 'f1', 'short_text');
		expect(original).toEqual([]);
	});
});

describe('archiveField and unarchiveField', () => {
	const fields: Field[] = [
		{ id: 'a', type: 'short_text', label: 'A', order: 0, archived: false },
		{ id: 'b', type: 'short_text', label: 'B', order: 1, archived: false }
	];

	it('archiveField sets archived on the matching field only, never removing it', () => {
		const result = archiveField(fields, 'a');
		expect(result).toHaveLength(2);
		expect(result.find((f) => f.id === 'a')?.archived).toBe(true);
		expect(result.find((f) => f.id === 'b')?.archived).toBe(false);
	});

	it('unarchiveField clears archived on the matching field', () => {
		const archived = archiveField(fields, 'a');
		const result = unarchiveField(archived, 'a');
		expect(result.find((f) => f.id === 'a')?.archived).toBe(false);
	});

	it('does not mutate the input array', () => {
		const original = [...fields];
		archiveField(fields, 'a');
		expect(fields).toEqual(original);
	});
});

describe('moveField', () => {
	const fields: Field[] = [
		{ id: 'a', type: 'short_text', label: 'A', order: 0, archived: false },
		{ id: 'b', type: 'short_text', label: 'B', order: 1, archived: false },
		{ id: 'c', type: 'short_text', label: 'C', order: 2, archived: false }
	];

	it('moves a field up, swapping with its predecessor', () => {
		const result = moveField(fields, 'b', 'up');
		expect(result.map((f) => f.id)).toEqual(['b', 'a', 'c']);
		expect(result.map((f) => f.order)).toEqual([0, 1, 2]);
	});

	it('moves a field down, swapping with its successor', () => {
		const result = moveField(fields, 'b', 'down');
		expect(result.map((f) => f.id)).toEqual(['a', 'c', 'b']);
	});

	it('is a no-op moving the first field up', () => {
		const result = moveField(fields, 'a', 'up');
		expect(result).toBe(fields);
	});

	it('is a no-op moving the last field down', () => {
		const result = moveField(fields, 'c', 'down');
		expect(result).toBe(fields);
	});

	it('is a no-op for an id that is not present', () => {
		const result = moveField(fields, 'nope', 'up');
		expect(result).toBe(fields);
	});

	it('skips over an archived field when moving an active field up', () => {
		const mixed: Field[] = [
			{ id: 'a', type: 'short_text', label: 'A', order: 0, archived: false },
			{ id: 'x', type: 'short_text', label: 'X', order: 1, archived: true },
			{ id: 'b', type: 'short_text', label: 'B', order: 2, archived: false }
		];
		const result = moveField(mixed, 'b', 'up');
		expect(result.map((f) => f.id)).toEqual(['b', 'x', 'a']);
	});

	it('is a no-op moving an active field up when only archived fields precede it', () => {
		const mixed: Field[] = [
			{ id: 'x', type: 'short_text', label: 'X', order: 0, archived: true },
			{ id: 'a', type: 'short_text', label: 'A', order: 1, archived: false }
		];
		const result = moveField(mixed, 'a', 'up');
		expect(result).toBe(mixed);
	});

	it('reorders an archived field only among other archived fields', () => {
		const mixed: Field[] = [
			{ id: 'x', type: 'short_text', label: 'X', order: 0, archived: true },
			{ id: 'a', type: 'short_text', label: 'A', order: 1, archived: false },
			{ id: 'y', type: 'short_text', label: 'Y', order: 2, archived: true }
		];
		const result = moveField(mixed, 'y', 'up');
		expect(result.map((f) => f.id)).toEqual(['y', 'a', 'x']);
	});
});

function makeFields(count: number, overrides: Partial<Field> = {}): Field[] {
	return Array.from({ length: count }, (_, index) => ({
		id: `f${index}`,
		type: 'short_text',
		label: `Field ${index}`,
		order: index,
		archived: false,
		...overrides
	}));
}

describe('fieldCountWarning', () => {
	it('is undefined at exactly the threshold', () => {
		expect(fieldCountWarning(makeFields(20))).toBeUndefined();
	});

	it('warns once past the threshold, naming the count', () => {
		expect(fieldCountWarning(makeFields(21))).toContain('21 questions');
	});

	it('does not count archived fields toward the threshold', () => {
		const fields = [...makeFields(20), ...makeFields(5, { archived: true })];
		expect(fieldCountWarning(fields)).toBeUndefined();
	});

	it('does not count section_header fields toward the threshold', () => {
		const fields = [...makeFields(20), ...makeFields(5, { type: 'section_header' })];
		expect(fieldCountWarning(fields)).toBeUndefined();
	});
});

describe('validateFields', () => {
	it('accepts a valid field list', () => {
		expect(
			validateFields([
				{ id: 'a', type: 'short_text', label: 'Intake note', order: 0, archived: false },
				{ id: 'b', type: 'single_select', label: 'Referral source', options: ['x'], order: 1, archived: false }
			])
		).toBeUndefined();
	});

	it('rejects a blank id', () => {
		expect(validateFields([{ id: '', type: 'short_text', label: 'A', order: 0, archived: false }])).toBe(
			'field id is required'
		);
	});

	it('rejects a duplicate id', () => {
		const result = validateFields([
			{ id: 'a', type: 'short_text', label: 'A', order: 0, archived: false },
			{ id: 'a', type: 'short_text', label: 'A again', order: 1, archived: false }
		]);
		expect(result).toBe('duplicate field id: a');
	});

	it('rejects a blank label', () => {
		expect(validateFields([{ id: 'a', type: 'short_text', label: '', order: 0, archived: false }])).toBe(
			'field label is required'
		);
	});

	it('rejects a label that restates a structural field, case/whitespace-insensitively, naming the structural field', () => {
		expect(
			validateFields([{ id: 'a', type: 'short_text', label: '  Date Of Birth  ', order: 0, archived: false }])
		).toBe(
			'"  Date Of Birth  " is already on every Client record as the structural date of birth field -- edit that field instead of adding a duplicate.'
		);
	});

	it('accepts a label that only contains a structural word as a substring', () => {
		expect(
			validateFields([
				{ id: 'a', type: 'short_text', label: 'Emergency contact phone', order: 0, archived: false }
			])
		).toBeUndefined();
	});

	it('rejects a select field with no options', () => {
		expect(validateFields([{ id: 'a', type: 'single_select', label: 'A', order: 0, archived: false }])).toBe(
			'field a requires at least one option'
		);
	});

	it('rejects a select field with an empty options array', () => {
		expect(
			validateFields([{ id: 'a', type: 'multi_select', label: 'A', options: [], order: 0, archived: false }])
		).toBe('field a requires at least one option');
	});

	it('rejects a select field with a blank option', () => {
		expect(
			validateFields([
				{ id: 'a', type: 'single_select', label: 'A', options: ['x', ''], order: 0, archived: false }
			])
		).toBe('field a has a blank option');
	});

	it('rejects a non-select field carrying options', () => {
		expect(
			validateFields([{ id: 'a', type: 'checkbox', label: 'A', options: ['x'], order: 0, archived: false }])
		).toBe('field a of type checkbox may not have options');
	});
});
