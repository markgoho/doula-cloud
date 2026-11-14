import { afterEach, describe, expect, it, vi } from 'vitest';
import { CannotBeReachedDraft } from './cannotBeReachedDraft.svelte.js';

afterEach(() => {
	vi.restoreAllMocks();
	sessionStorage.clear();
});

describe('the act’s draft', () => {
	it('keeps answers per Practice, mirrors them to the tab and forgets them on clear', () => {
		const draft = new CannotBeReachedDraft();
		draft.start('p1');
		draft.update({ staffId: 's1', reason: 'x' });
		draft.setCover('e1', 's2');

		const reopened = new CannotBeReachedDraft();
		reopened.start('p1');
		expect(reopened.answers).toMatchObject({ staffId: 's1', reason: 'x', cover: { e1: 's2' } });

		reopened.start('p2');
		expect(reopened.answers.staffId).toBe('');

		draft.keepCover(['e9']);
		expect(draft.answers.cover).toEqual({});

		draft.clear();
		expect(sessionStorage.getItem('doula-cloud:cannot-be-reached:p1')).toBeNull();
		expect(draft.answers.staffId).toBe('');
	});

	it('does not reopen a Practice it already holds', () => {
		const draft = new CannotBeReachedDraft();
		draft.start('p1');
		draft.update({ reason: 'kept' });

		draft.start('p1');

		expect(draft.answers.reason).toBe('kept');
	});

	it('works with the browser’s storage refused', () => {
		vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
			throw new Error('denied');
		});
		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
			throw new Error('denied');
		});
		vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
			throw new Error('denied');
		});
		const draft = new CannotBeReachedDraft();

		draft.start('p1');
		draft.update({ reason: 'still here' });
		expect(draft.answers.reason).toBe('still here');
		draft.clear();

		expect(draft.answers.reason).toBe('');
	});
});
