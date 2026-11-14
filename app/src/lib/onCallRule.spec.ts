import { describe, expect, it, vi } from 'vitest';
import { describeSavedOnCall, loadOnCallSettings, saveOnCallSettings, settingsPath } from './onCallRule.js';
import { jsonResponse } from './testResponse.js';

/*
 * #2291: the On call save said only "Saved.". It now says the rule that
 * was saved, and that every birth the Practice is carrying follows it,
 * because nothing stores a window.
 */
describe('what an On call save says', () => {
	const carried =
		'for every birth this Practice is carrying, including births already under way.';

	it('names the week on call opens and the days it runs past the due date', () => {
		expect(
			describeSavedOnCall({ startRule: 'gestational_week', startWeek: 39, graceDays: 10 })
		).toBe(
			`On call saved. On call now opens at week 39 of pregnancy and runs 10 days past the due date, ${carried}`
		);
	});

	it('names the day a doula takes the birth, and says a single day as one', () => {
		expect(
			describeSavedOnCall({ startRule: 'attachment_granted', startWeek: 37, graceDays: 1 })
		).toBe(
			`On call saved. On call now opens on the day a doula takes the birth and runs 1 day past the due date, ${carried}`
		);
	});

	it('says on call ends on the due date when it runs no days past it', () => {
		expect(
			describeSavedOnCall({ startRule: 'gestational_week', startWeek: 37, graceDays: 0 })
		).toBe(
			`On call saved. On call now opens at week 37 of pregnancy and ends on the due date, ${carried}`
		);
	});
});

describe('the calls the settings screen makes', () => {
	it('reads and states the Practice s rule', async () => {
		const settings = {
			startRule: 'gestational_week' as const,
			startWeek: 37,
			graceDays: 14,
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(settings));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).resolves.toEqual(
			settings
		);
		await saveOnCallSettings(fetcher, 'practice-1', settings);

		expect(fetcher).toHaveBeenLastCalledWith(
			'/api/practices/practice-1/on-call-settings',
			{
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(settings),
			}
		);
	});

	it('throws the refusal when a rule is refused, either way round', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('Enter a week from 20 to 42.', 400));

		await expect(loadOnCallSettings(fetcher, 'practice-1')).rejects.toThrow(
			'Enter a week'
		);
		await expect(
			saveOnCallSettings(fetcher, 'practice-1', {
				startRule: 'gestational_week',
				startWeek: 12,
				graceDays: 14,
			})
		).rejects.toThrow('Enter a week');
	});

	it('names the BFF path the rule lives at', () => {
		expect(settingsPath('practice-1')).toBe('/api/practices/practice-1/on-call-settings');
	});
});
