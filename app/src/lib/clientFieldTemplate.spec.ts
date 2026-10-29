import { describe, expect, it, vi } from 'vitest';
import {
	archiveField,
	fieldCountWarning,
	isSelectType,
	loadTemplate,
	saveTemplate,
	unarchiveField,
	structuralFieldRefusal,
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

describe('structuralFieldRefusal', () => {
	it('refuses a label that restates a structural field, case/whitespace-insensitively, naming the structural field', () => {
		expect(structuralFieldRefusal('  Date Of Birth  ')).toBe(
			'"  Date Of Birth  " is already on every Client record as the date of birth field, so it is not a field to add here.'
		);
	});

	it('accepts a label that only contains a structural word as a substring', () => {
		expect(structuralFieldRefusal('Emergency contact phone')).toBeUndefined();
	});
});
