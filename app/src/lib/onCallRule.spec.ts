import { describe, expect, it, vi } from 'vitest';
import {
	describeOnCallEffect,
	describeSavedOnCall,
	isTypedRuleSavable,
	loadOnCallEffect,
	loadOnCallSettings,
	saveOnCallSettings,
	settingsPath
} from './onCallRule.js';
import { jsonResponse } from './testResponse.js';

describe('what an On-call rule save says', () => {
	const carried = 'for every birth without its own start.';

	it('names the week on call opens and the days it runs past the due date', () => {
		expect(describeSavedOnCall({ startRule: 'gestational_week', startWeek: 39, graceDays: 10 })).toBe(
			`On-call rule saved. On call now opens at week 39 of pregnancy and runs 10 days past the due date, ${carried}`
		);
	});

	it('names the day a Doula takes the birth, and says a single day as one', () => {
		expect(describeSavedOnCall({ startRule: 'attachment_granted', startWeek: 37, graceDays: 1 })).toBe(
			`On-call rule saved. On call now opens on the day a Doula takes the birth and runs 1 day past the due date, ${carried}`
		);
	});

	it('says on call ends on the due date when it runs no days past it', () => {
		expect(describeSavedOnCall({ startRule: 'gestational_week', startWeek: 37, graceDays: 0 })).toBe(
			`On-call rule saved. On call now opens at week 37 of pregnancy and ends on the due date, ${carried}`
		);
	});
});

describe('what the preview says', () => {
	const camille = { engagementId: 'e1', clientName: 'Camille Boyd' };
	const morgan = { engagementId: 'e2', clientName: 'Morgan Grier' };
	const casey = { engagementId: 'e3', clientName: 'Casey Estrada' };

	it('says no birth changes when none does', () => {
		expect(describeOnCallEffect({ stopToday: [], startToday: [], keepOwnStart: [] })).toEqual([
			'Saving this changes no birth on call today.'
		]);
	});

	it('names one birth, and three with commas and an and', () => {
		expect(describeOnCallEffect({ stopToday: [camille], startToday: [], keepOwnStart: [] })).toEqual([
			'Saving this takes 1 birth off call today: Camille Boyd.'
		]);
		expect(
			describeOnCallEffect({ stopToday: [camille, morgan, casey], startToday: [], keepOwnStart: [] })
		).toEqual([
			'Saving this takes 3 births off call today: Camille Boyd, Morgan Grier and Casey Estrada.'
		]);
	});

	it('names the births it puts on call, beside those it takes off', () => {
		expect(
			describeOnCallEffect({ stopToday: [camille], startToday: [morgan, casey], keepOwnStart: [] })
		).toEqual([
			'Saving this takes 1 birth off call today: Camille Boyd.',
			'Saving this puts 2 births on call today: Morgan Grier and Casey Estrada.'
		]);
	});

	it('names the births that keep their own start', () => {
		expect(describeOnCallEffect({ stopToday: [], startToday: [], keepOwnStart: [camille] })).toEqual([
			'Saving this changes no birth on call today.',
			'1 birth keeps the start set on that birth: Camille Boyd.'
		]);
	});
});

describe('which typed rules the preview asks about', () => {
	it('accepts whole numbers inside the bounds', () => {
		expect(isTypedRuleSavable('gestational_week', '20', '0')).toBe(true);
		expect(isTypedRuleSavable('gestational_week', '42', '42')).toBe(true);
	});

	it('refuses a week or days outside them, or not a whole number', () => {
		expect(isTypedRuleSavable('gestational_week', '19', '14')).toBe(false);
		expect(isTypedRuleSavable('gestational_week', '43', '14')).toBe(false);
		expect(isTypedRuleSavable('gestational_week', '37', '43')).toBe(false);
		expect(isTypedRuleSavable('gestational_week', '3x', '14')).toBe(false);
		expect(isTypedRuleSavable('gestational_week', '', '14')).toBe(false);
	});

	it('does not look at the week under the rule that has none', () => {
		expect(isTypedRuleSavable('attachment_granted', '', '14')).toBe(true);
	});
});

describe('the calls the settings screen makes', () => {
	const settings = {
		startRule: 'gestational_week' as const,
		startWeek: 37,
		graceDays: 14
	};

	it('reads and states the Practice s rule', async () => {
		// eslint-disable-next-line unicorn/no-null -- the BFF sends null when the rule never changed
		const read = { ...settings, lastChanged: null };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(read));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).resolves.toEqual(read);
		await saveOnCallSettings(fetcher, 'practice-1', settings);

		expect(fetcher).toHaveBeenLastCalledWith('/api/practices/practice-1/on-call-settings', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(settings)
		});
	});

	it('throws the refusal when a rule is refused, either way round', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('Enter a week from 20 to 42.', 400));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).rejects.toThrow('Enter a week');
		await expect(
			saveOnCallSettings(fetcher, 'practice-1', { ...settings, startWeek: 12 })
		).rejects.toThrow('Enter a week');
	});

	it('names the BFF path the rule lives at', () => {
		expect(settingsPath('practice-1')).toBe('/api/practices/practice-1/on-call-settings');
	});

	it('reads the effect of a typed rule from the effect path', async () => {
		const effect = { stopToday: [], startToday: [], keepOwnStart: [] };
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(effect));
		const signal = new AbortController().signal;

		await expect(
			loadOnCallEffect(fetcher, 'practice-1', { ...settings, startWeek: 38 }, signal)
		).resolves.toEqual(effect);
		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/on-call-settings/effect?startRule=gestational_week&startWeek=38&graceDays=14',
			{ signal }
		);
	});

	it('throws the refusal when the effect is refused', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('Enter a week from 20 to 42.', 400));

		await expect(
			loadOnCallEffect(fetcher, 'practice-1', { ...settings, startWeek: 12 })
		).rejects.toThrow('Enter a week');
	});
});
