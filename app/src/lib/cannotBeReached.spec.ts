import { describe, expect, it, vi } from 'vitest';
import {
	NOBODY_YET,
	birthLegend,
	cannotBeReachedPath,
	coverAnswer,
	coverFor,
	coverRequired,
	coverageGapPreviewPath,
	entryLabel,
	firstName,
	isAsked,
	isWritable,
	journeyAddressFrom,
	journeyHref,
	loadDoulaChoices,
	nobodyElse,
	previewGaps,
	recordButton,
	recordGaps,
	recordedMessage,
	rosterHrefFor,
	whenQuestion,
	type PreviewBirth
} from './cannotBeReached.js';
import { jsonResponse } from './testResponse.js';

const birth: PreviewBirth = {
	engagementId: 'e1',
	clientName: 'Camille Boyd',
	startsAt: '2026-11-14T23:00:00Z',
	endsAt: '2026-11-15T14:00:00Z',
	coverOptions: [{ staffId: 's2', name: 'Bethany Kroll' }]
};

describe('the addresses', () => {
	it('puts the first step at the act’s own path and the rest under it', () => {
		expect(cannotBeReachedPath('p1')).toBe('/practices/p1/on-call/cannot-be-reached');
		expect(cannotBeReachedPath('p1', 'check')).toBe('/practices/p1/on-call/cannot-be-reached/check');
	});

	it('carries only the roster’s query and the Doula it was narrowed to', () => {
		expect(journeyHref('p1', 'doula', { back: '', doula: '' })).toBe('/practices/p1/on-call/cannot-be-reached');
		expect(journeyHref('p1', 'cover', { back: 'days=7', doula: 's1' }, { changed: 'true' })).toBe(
			'/practices/p1/on-call/cannot-be-reached/cover?back=days%3D7&doula=s1&changed=true'
		);
		expect(journeyAddressFrom(new URLSearchParams('back=days%3D7&doula=s1'))).toEqual({ back: 'days=7', doula: 's1' });
		expect(journeyAddressFrom(new URLSearchParams(''))).toEqual({ back: '', doula: '' });
	});

	it('comes back to the roster through the roster’s own reading of its query', () => {
		expect(rosterHrefFor('p1', 'days=7&mine=true&x=%3Cscript%3E', true)).toBe('/practices/p1/on-call?days=7&mine=true');
		expect(rosterHrefFor('p1', 'mine=true', false, { staffId: 's1' })).toBe('/practices/p1/on-call');
	});
});

describe('the words', () => {
	it('asks the Owner about the Doula by first name, and a Doula about herself', () => {
		expect(whenQuestion(true, 'Rowan Petrosyan')).toBe('When can Rowan not be reached?');
		expect(whenQuestion(true, '')).toBe('When can you not be reached?');
		expect(whenQuestion(false, 'Rowan Petrosyan')).toBe('When can you not be reached?');
		expect(firstName('  Rowan  Petrosyan ')).toBe('Rowan');
		expect(entryLabel(true)).toBe('Record a time a Doula cannot be reached');
		expect(entryLabel(false)).toBe('Record a time you cannot be reached');
	});

	it('says each birth and its cover', () => {
		expect(birthLegend(birth, 'America/New_York', new Date('2026-11-14T12:00:00Z'))).toBe(
			'Camille Boyd, Sat Nov 14, 6:00pm to Sun Nov 15, 9:00am'
		);
		expect(coverRequired('Camille Boyd')).toBe("Choose who is covering Camille Boyd's birth, or Nobody yet");
		expect(nobodyElse('Camille Boyd')).toBe("Nobody else is on Camille Boyd's birth, so nobody can cover.");
		expect(recordButton(1)).toBe('Record it on 1 birth');
		expect(recordButton(3)).toBe('Record it on 3 births');
		expect(coverAnswer('Priya Raman')).toBe('Priya Raman covering');
		expect(coverAnswer(undefined)).toBe('Nobody covering');
	});

	it('says what was recorded and who is told', () => {
		expect(recordedMessage('you', 3, 0)).toBe('The time you cannot be reached is recorded on 3 births.');
		expect(recordedMessage('you', 3, 1)).toBe(
			'The time you cannot be reached is recorded on 3 births. Every Owner and Admin is told of the 1 with nobody covering.'
		);
		expect(recordedMessage('Rowan', 1, 1)).toBe(
			'The time Rowan cannot be reached is recorded on 1 birth. Every Owner and Admin is told it has nobody covering.'
		);
	});
});

describe('who is asked and what is written', () => {
	const recorded = { ...birth, existingGap: { id: 'g' } as never };
	const alone = { ...birth, coverOptions: [] };

	it('asks a birth that has another Doula and no gap of hers yet', () => {
		expect(isAsked(birth)).toBe(true);
		expect(isAsked(alone)).toBe(false);
		expect(isAsked(recorded)).toBe(false);
		expect(isWritable(alone)).toBe(true);
		expect(isWritable(recorded)).toBe(false);
	});

	it('reads the answer for a birth: a Doula, nobody, or none yet', () => {
		expect(coverFor(birth, { e1: 's2' })).toBe('s2');
		expect(coverFor(birth, { e1: NOBODY_YET })).toBe('');
		expect(coverFor(birth, {})).toBeUndefined();
		expect(coverFor(birth, { e1: '' })).toBeUndefined();
		expect(coverFor(birth, { e1: 'someone-else' })).toBeUndefined();
		expect(coverFor(alone, {})).toBe('');
	});
});

describe('the reads and the write', () => {
	it('asks for the births a time meets', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ births: [birth] }));

		const got = await previewGaps(fetcher, 'p1', 's1', new Date('2026-11-14T23:00:00Z'), new Date('2026-11-15T14:00:00Z'));

		expect(got.births).toHaveLength(1);
		expect(fetcher).toHaveBeenCalledWith(coverageGapPreviewPath('p1', 's1', new Date('2026-11-14T23:00:00Z'), new Date('2026-11-15T14:00:00Z')));
		expect(String(fetcher.mock.calls[0]![0])).toContain('startsAt=2026-11-14T23%3A00%3A00.000Z');
	});

	it('throws what the BFF refused', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ message: 'No' }, 403));

		await expect(previewGaps(fetcher, 'p1', 's1', new Date(), new Date())).rejects.toThrow('No');
		await expect(recordGaps(fetcher, 'p1', { staffId: 's1', startsAt: '', endsAt: '', births: [] })).rejects.toThrow('No');
		await expect(loadDoulaChoices(fetcher, 'p1', '2026-11-14')).rejects.toThrow('No');
	});

	it('lists the Doulas on a birth from the roster', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse({
				doulas: [
					{ staffId: 's1', name: 'A', births: [{}] },
					{ staffId: 's2', name: 'B', births: [] }
				]
			})
		);

		expect(await loadDoulaChoices(fetcher, 'p1', '2026-11-14')).toEqual([{ staffId: 's1', name: 'A' }]);
		expect(String(fetcher.mock.calls[0]![0])).toContain('from=2026-11-14&to=2027-02-13');
	});

	it('posts the births and returns the gaps written', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ gaps: [{ id: 'g1' }] }, 201));

		const gaps = await recordGaps(fetcher, 'p1', {
			staffId: 's1',
			startsAt: 'a',
			endsAt: 'b',
			births: [{ engagementId: 'e1', coveringStaffId: undefined }]
		});

		expect(gaps).toEqual([{ id: 'g1' }]);
		expect(fetcher.mock.calls[0]![0]).toBe('/api/practices/p1/coverage-gaps');
		expect(fetcher.mock.calls[0]![1]).toMatchObject({ method: 'POST' });
	});
});
