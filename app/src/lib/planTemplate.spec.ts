import { describe, expect, it, vi } from 'vitest';
import {
	addField,
	isSelectType,
	loadTemplate,
	moveField,
	removeField,
	saveTemplate,
	validateFields,
	type Field
} from './planTemplate.js';
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
	it('fetches the practice+planType path and returns the decoded template', async () => {
		const template = { planType: 'care_plan', fields: [] };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(template));

		const result = await loadTemplate(fetcher, 'practice-1', 'care_plan');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/plan-templates/care_plan');
		expect(result).toEqual(template);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		await expect(loadTemplate(fetcher, 'practice-1', 'care_plan')).rejects.toThrow('not found');
	});
});

describe('saveTemplate', () => {
	it('PUTs planType and fields as JSON to the practice+planType path', async () => {
		const fields: Field[] = [{ id: 'f1', type: 'short_text', label: 'Name', order: 0 }];
		const saved = { planType: 'birth_plan', fields };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(saved));

		const result = await saveTemplate(fetcher, 'practice-1', 'birth_plan', fields);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/plan-templates/birth_plan', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ planType: 'birth_plan', fields })
		});
		expect(result).toEqual(saved);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner can do that', 403));

		await expect(saveTemplate(fetcher, 'practice-1', 'care_plan', [])).rejects.toThrow(
			'only a Practice Owner can do that'
		);
	});
});

describe('addField', () => {
	it('appends a blank field with no options for a non-select type', () => {
		const result = addField([], 'f1', 'short_text');
		expect(result).toEqual([{ id: 'f1', type: 'short_text', label: '', order: 0, options: undefined }]);
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

describe('removeField', () => {
	it('removes the matching field and renumbers order', () => {
		const fields: Field[] = [
			{ id: 'a', type: 'short_text', label: 'A', order: 0 },
			{ id: 'b', type: 'short_text', label: 'B', order: 1 },
			{ id: 'c', type: 'short_text', label: 'C', order: 2 }
		];
		const result = removeField(fields, 'b');
		expect(result).toEqual([
			{ id: 'a', type: 'short_text', label: 'A', order: 0 },
			{ id: 'c', type: 'short_text', label: 'C', order: 1 }
		]);
	});
});

describe('moveField', () => {
	const fields: Field[] = [
		{ id: 'a', type: 'short_text', label: 'A', order: 0 },
		{ id: 'b', type: 'short_text', label: 'B', order: 1 },
		{ id: 'c', type: 'short_text', label: 'C', order: 2 }
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
});

describe('validateFields', () => {
	it('accepts a valid field list', () => {
		expect(
			validateFields([
				{ id: 'a', type: 'short_text', label: 'A', order: 0 },
				{ id: 'b', type: 'single_select', label: 'B', options: ['x'], order: 1 }
			])
		).toBeUndefined();
	});

	it('rejects a blank id', () => {
		expect(validateFields([{ id: '', type: 'short_text', label: 'A', order: 0 }])).toBe(
			'field id is required'
		);
	});

	it('rejects a duplicate id', () => {
		const result = validateFields([
			{ id: 'a', type: 'short_text', label: 'A', order: 0 },
			{ id: 'a', type: 'short_text', label: 'A again', order: 1 }
		]);
		expect(result).toBe('duplicate field id: a');
	});

	it('rejects a blank label', () => {
		expect(validateFields([{ id: 'a', type: 'short_text', label: '', order: 0 }])).toBe(
			'field label is required'
		);
	});

	it('rejects a select field with no options', () => {
		expect(validateFields([{ id: 'a', type: 'single_select', label: 'A', order: 0 }])).toBe(
			'field a requires at least one option'
		);
	});

	it('rejects a select field with an empty options array', () => {
		expect(
			validateFields([{ id: 'a', type: 'multi_select', label: 'A', options: [], order: 0 }])
		).toBe('field a requires at least one option');
	});

	it('rejects a select field with a blank option', () => {
		expect(
			validateFields([{ id: 'a', type: 'single_select', label: 'A', options: ['x', ''], order: 0 }])
		).toBe('field a has a blank option');
	});

	it('rejects a non-select field carrying options', () => {
		expect(
			validateFields([{ id: 'a', type: 'checkbox', label: 'A', options: ['x'], order: 0 }])
		).toBe('field a of type checkbox may not have options');
	});
});
