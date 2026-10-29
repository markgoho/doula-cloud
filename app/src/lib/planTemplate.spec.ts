import { describe, expect, it, vi } from 'vitest';
import {
	FIELD_TYPE_OPTIONS,
	FIELD_TYPES,
	isSelectType,
	loadTemplate,
	removeField,
	saveTemplate,
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

// #1252: a person reads a name, the API stores the identifier.
describe('FIELD_TYPE_OPTIONS', () => {
	it('pairs every stored type, in palette order, with a human name', () => {
		expect(FIELD_TYPE_OPTIONS.map((option) => option.value)).toEqual([...FIELD_TYPES]);
		expect(FIELD_TYPE_OPTIONS.map((option) => option.label)).toEqual([
			'Short answer',
			'Long answer',
			'Choose one',
			'Choose several',
			'Yes or no',
			'Section heading'
		]);
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

