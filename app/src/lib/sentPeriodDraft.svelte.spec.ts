import { describe, expect, it } from 'vitest';
import { SentPeriodDraft } from './sentPeriodDraft.svelte.js';

describe('SentPeriodDraft', () => {
	it('holds no period until one is chosen', () => {
		const draft = new SentPeriodDraft();
		expect(draft.period).toBeUndefined();
		draft.choice = 'last-month';
		expect(draft.period).toEqual({ preset: 'last-month' });
	});

	it('reads chosen days as a period of their own', () => {
		const draft = new SentPeriodDraft();
		draft.seed({ days: { from: '2026-10-03', to: '2026-10-17' } });
		expect(draft.isChoosingDays).toBe(true);
		expect(draft.period).toEqual({ days: { from: '2026-10-03', to: '2026-10-17' } });
		draft.clear();
		expect(draft.period).toBeUndefined();
		expect(draft.from).toBe('');
	});

	it('words two days that are not a range, and forgets them on the next seed', () => {
		const draft = new SentPeriodDraft();
		draft.seed({ days: { from: '2026-10-17', to: '2026-10-03' } });
		expect(draft.hasProblem()).toBe(true);
		expect(draft.errors.to).toBeTruthy();
		draft.seed(undefined);
		expect(draft.errors).toEqual({});
		expect(draft.hasProblem()).toBe(false);
	});

	it('moves focus to the box that is wrong, the From box first', async () => {
		document.body.innerHTML = '<input name="from"><input name="to">';
		const draft = new SentPeriodDraft();

		draft.seed({ days: { from: '', to: '2026-10-03' } });
		expect(draft.hasProblem()).toBe(true);
		await draft.focusProblem();
		expect(document.activeElement).toBe(document.querySelector('input[name="from"]'));

		draft.seed({ days: { from: '2026-10-17', to: '2026-10-03' } });
		draft.errors = { to: 'Choose a To day that is not before From.' };
		await draft.focusProblem();
		expect(document.activeElement).toBe(document.querySelector('input[name="to"]'));
		document.body.replaceChildren();
	});

	it('treats "Choose days" alone as opening its fields', () => {
		const draft = new SentPeriodDraft();
		draft.choice = 'choose';
		expect(draft.isOnlyOpeningDays(['sent'])).toBe(true);
		expect(draft.isOnlyOpeningDays(['sent', 'show'])).toBe(false);
		draft.choice = 'this-year';
		expect(draft.isOnlyOpeningDays(['sent'])).toBe(false);
	});
});
