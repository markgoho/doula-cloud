import { describe, expect, it } from 'vitest';
import { jsonResponse } from './testResponse.js';
import {
	blockedAddressesValue,
	loadSettingsSummary,
	MAY_CHANGE,
	onCallValue,
	questionsValue,
	ratesValue,
	websiteValue
} from './settingsSummary.js';
import type { PracticeWebsite } from './website.js';

const unanswered: PracticeWebsite = {
	mode: 'undeclared',
	ownUrl: '',
	serviceDescription: '',
	cancellationPolicy: '',
	updatedBy: '',
	updatedAt: '',
	pageState: '',
	pageCheckedAt: '',
	pageCheckDetail: '',
	pageUrl: ''
};

describe('the Settings index values', () => {
	it('says where a Client finds the Practice, as a link only when there is an address', () => {
		expect(websiteValue(unanswered)).toEqual({ text: 'Not answered yet' });
		expect(websiteValue({ ...unanswered, mode: 'own', ownUrl: 'https://rochesterdoulas.com' })).toEqual({
			text: 'https://rochesterdoulas.com',
			href: 'https://rochesterdoulas.com'
		});
		expect(websiteValue({ ...unanswered, mode: 'hosted', pageState: 'failed' })).toEqual({
			text: 'Your page could not be loaded'
		});
		expect(websiteValue({ ...unanswered, mode: 'hosted', pageState: 'pending' })).toEqual({
			text: 'Your page is being published'
		});
	});

	it('words the counts and rules a person reads', () => {
		expect(questionsValue(0)).toBe('No questions yet');
		expect(questionsValue(1)).toBe('1 question');
		expect(onCallValue({ startRule: 'gestational_week', startWeek: 37, graceDays: 1 })).toBe(
			'From week 37, until 1 day past the due date'
		);
		expect(
			ratesValue([
				{ kind: 'birth', amountCents: 180_000 },
				// eslint-disable-next-line unicorn/no-null -- a real API response's JSON null, for a kind with no rate set
				{ kind: 'postpartum', amountCents: null }
			])
		).toBe('Birth: $1,800.00. Postpartum: no rate');
		expect(blockedAddressesValue(0)).toBe('None');
		expect(blockedAddressesValue(1)).toBe('1 address');
	});

	it('draws the deletion section for an Owner alone', () => {
		expect(MAY_CHANGE.delete({ roles: ['owner'] })).toBe(true);
		expect(MAY_CHANGE.delete({ roles: ['admin'] })).toBe(false);
	});
});

describe('loadSettingsSummary', () => {
	it('says a setting nobody has answered is not set', async () => {
		const answers: Record<string, unknown> = {
			timezone: { timezone: '' },
			'contract-template': { prose: '  ' },
			'mfa-required/impact': { required: false, withoutSecondFactor: 0 }
		};
		const fetcher = (path: string) => {
			const key = path.replace(/^\/api\/practices\/practice-1\//, '');
			const body = answers[key];
			return Promise.resolve(body === undefined ? jsonResponse({ message: 'no' }, 500) : jsonResponse(body));
		};

		const summary = await loadSettingsSummary(fetcher, 'practice-1', { roles: ['owner'] });

		expect(summary.timezone).toEqual({ text: 'Not set' });
		expect(summary.contractTemplate).toEqual({ text: 'Not written yet' });
		expect(summary.mfa).toEqual({ text: 'Off' });
		expect(summary.website).toBe('unavailable');
	});
});
