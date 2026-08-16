import { describe, expect, it, vi } from 'vitest';
import {
	isAnswerChecked,
	answerOptions,
	answerText,
	createInstance,
	loadClientBirthPlan,
	loadInstance,
	saveAnswers,
	setAnswer,
	toggleMultiSelectOption,
	type Answers
} from './planInstance.js';

function jsonResponse(body: unknown, status = 200): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

describe('loadInstance', () => {
	it('fetches the practice+engagement+planType path and returns the decoded instance', async () => {
		const instance = { engagementId: 'eng-1', planType: 'care_plan', fields: [], answers: {} };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(instance));

		const result = await loadInstance(fetcher, 'practice-1', 'eng-1', 'care_plan');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/plans/care_plan');
		expect(result).toEqual(instance);
	});

	it('returns undefined on a 404 (no instance created yet)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		const result = await loadInstance(fetcher, 'practice-1', 'eng-1', 'care_plan');

		expect(result).toBeUndefined();
	});

	it('throws with the response body text on any other non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('server error', 500));

		await expect(loadInstance(fetcher, 'practice-1', 'eng-1', 'care_plan')).rejects.toThrow('server error');
	});
});

describe('loadClientBirthPlan', () => {
	it('fetches the portal engagement birth-plan path and returns the decoded instance', async () => {
		const instance = { engagementId: 'eng-1', planType: 'birth_plan', fields: [], answers: {} };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(instance));

		const result = await loadClientBirthPlan(fetcher, 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/portal/engagements/eng-1/birth-plan');
		expect(result).toEqual(instance);
	});

	it('returns null on a 404 (no birth plan created yet)', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not found', 404));

		const result = await loadClientBirthPlan(fetcher, 'eng-1');

		expect(result).toBeNull();
	});

	it('throws with the response body text on any other non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('engagement not linked to this client', 403));

		await expect(loadClientBirthPlan(fetcher, 'eng-1')).rejects.toThrow('engagement not linked to this client');
	});
});

describe('createInstance', () => {
	it('POSTs to the practice+engagement+planType path and returns the decoded instance', async () => {
		const instance = { engagementId: 'eng-1', planType: 'birth_plan', fields: [], answers: {} };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(instance, 201));

		const result = await createInstance(fetcher, 'practice-1', 'eng-1', 'birth_plan');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/plans/birth_plan', {
			method: 'POST'
		});
		expect(result).toEqual(instance);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('no plan template found for this practice and plan type', 404));

		await expect(createInstance(fetcher, 'practice-1', 'eng-1', 'care_plan')).rejects.toThrow(
			'no plan template found for this practice and plan type'
		);
	});
});

describe('saveAnswers', () => {
	it('PUTs the full answers map as JSON to the practice+engagement+planType path', async () => {
		const answers: Answers = { f1: 'Jamie', f2: true };
		const instance = { engagementId: 'eng-1', planType: 'care_plan', fields: [], answers };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(instance));

		const result = await saveAnswers(fetcher, 'practice-1', 'eng-1', 'care_plan', answers);

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/plans/care_plan', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ answers })
		});
		expect(result).toEqual(instance);
	});

	it('throws with the response body text on a non-ok response', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('field f1 requires a string answer', 400));

		await expect(saveAnswers(fetcher, 'practice-1', 'eng-1', 'care_plan', {})).rejects.toThrow(
			'field f1 requires a string answer'
		);
	});
});

describe('setAnswer', () => {
	it('sets the answer for the given field id', () => {
		const result = setAnswer({}, 'f1', 'Jamie');
		expect(result).toEqual({ f1: 'Jamie' });
	});

	it('does not mutate the input object', () => {
		const original: Answers = { f1: 'Jamie' };
		setAnswer(original, 'f2', 'Alex');
		expect(original).toEqual({ f1: 'Jamie' });
	});

	it('clears the answer when set to an empty string', () => {
		const result = setAnswer({ f1: 'Jamie' }, 'f1', '');
		expect(result).toEqual({});
	});

	it('clears the answer when set to an empty array', () => {
		const result = setAnswer({ f1: ['A'] }, 'f1', []);
		expect(result).toEqual({});
	});

	it('leaves a false boolean answer in place (not treated as empty)', () => {
		const result = setAnswer({}, 'f1', false);
		expect(result).toEqual({ f1: false });
	});
});

describe('toggleMultiSelectOption', () => {
	it('adds the option when not yet selected', () => {
		const result = toggleMultiSelectOption({}, 'f1', 'Partner');
		expect(result).toEqual({ f1: ['Partner'] });
	});

	it('appends to existing selected options', () => {
		const result = toggleMultiSelectOption({ f1: ['Partner'] }, 'f1', 'Doula');
		expect(result).toEqual({ f1: ['Partner', 'Doula'] });
	});

	it('removes the option when already selected', () => {
		const result = toggleMultiSelectOption({ f1: ['Partner', 'Doula'] }, 'f1', 'Partner');
		expect(result).toEqual({ f1: ['Doula'] });
	});

	it('clears the field entirely when removing the last selected option', () => {
		const result = toggleMultiSelectOption({ f1: ['Partner'] }, 'f1', 'Partner');
		expect(result).toEqual({});
	});
});

describe('answerText', () => {
	it('returns the stored string value', () => {
		expect(answerText({ f1: 'Jamie' }, 'f1')).toBe('Jamie');
	});

	it("returns '' when unset or not a string", () => {
		expect(answerText({}, 'f1')).toBe('');
		expect(answerText({ f1: true }, 'f1')).toBe('');
	});
});

describe('isAnswerChecked', () => {
	it('returns true only when the stored value is exactly true', () => {
		expect(isAnswerChecked({ f1: true }, 'f1')).toBe(true);
		expect(isAnswerChecked({ f1: false }, 'f1')).toBe(false);
		expect(isAnswerChecked({}, 'f1')).toBe(false);
	});
});

describe('answerOptions', () => {
	it('returns the stored array value', () => {
		expect(answerOptions({ f1: ['Partner', 'Doula'] }, 'f1')).toEqual(['Partner', 'Doula']);
	});

	it('returns [] when unset or not an array', () => {
		expect(answerOptions({}, 'f1')).toEqual([]);
		expect(answerOptions({ f1: 'Partner' }, 'f1')).toEqual([]);
	});
});
