/*
 * What every page of recording a time a Doula cannot be reached is drawn
 * from (#2934): a draft already holding the first answers, and the two reads
 * the journey makes, with hostile content (ADR-0025): a double-barreled
 * Client name, a Doula with a professional suffix, a pasted URL as a reason,
 * a birth nobody else is on, and a birth that already holds a gap of hers.
 */
import { cannotBeReachedDraft } from '#lib/cannotBeReachedDraft.svelte.js';
import type { GapPreview } from '#lib/cannotBeReached.js';
import { practiceSession } from '../../../../routeFixture.js';

export const practiceId = 'practice-1';
const DOULA = 'Persephone Vandermeulen-Achterberg, CD(DONA)';

export const preview: GapPreview = {
	births: [
		{
			engagementId: 'engagement-1',
			clientName: 'Wilhelmina Featherstonehaugh-Ravenscroft',
			startsAt: '2026-11-22T03:00:00Z',
			endsAt: '2026-11-22T13:00:00Z',
			coverOptions: [
				{ staffId: 'staff-3', name: 'Marguerite Okonkwo-Fitzgerald' },
				{ staffId: 'staff-4', name: 'Bo Ng' }
			]
		},
		{
			engagementId: 'engagement-2',
			clientName: 'Ana Ruiz',
			startsAt: '2026-11-22T03:00:00Z',
			endsAt: '2026-11-22T13:00:00Z',
			coverOptions: []
		},
		{
			engagementId: 'engagement-3',
			clientName: 'Kim Tran',
			startsAt: '2026-11-22T03:00:00Z',
			endsAt: '2026-11-22T13:00:00Z',
			coverOptions: [{ staffId: 'staff-3', name: 'Marguerite Okonkwo-Fitzgerald' }],
			existingGap: {
				id: 'gap-1',
				engagementId: 'engagement-3',
				staffId: 'staff-2',
				staffName: DOULA,
				startsAt: '2026-11-22T02:00:00Z',
				endsAt: '2026-11-22T14:00:00Z'
			}
		}
	]
};

export const roster = {
	doulas: [
		{ staffId: 'staff-2', name: DOULA, births: [{ engagementId: 'engagement-1', clientName: 'Ana Ruiz', covering: false }], unreachable: [], isOnCallTonight: false },
		{ staffId: 'staff-3', name: 'Marguerite Okonkwo-Fitzgerald', births: [{ engagementId: 'engagement-1', clientName: 'Ana Ruiz', covering: false }], unreachable: [], isOnCallTonight: false },
		{ staffId: 'staff-5', name: 'Nobody Yet', births: [], unreachable: [], isOnCallTonight: false }
	]
};

export function respond(path: string): Response {
	const body = path.includes('/coverage-gaps/preview') ? preview : roster;
	return Response.json(body, { status: 200, headers: { 'Content-Type': 'application/json' } });
}

export const ownerSession = practiceSession(['owner']);

/**
The answers the journey holds by the time a later page is reached.
*/
export function seedDraft(): void {
	cannotBeReachedDraft.start(practiceId);
	cannotBeReachedDraft.update({
		staffId: 'staff-2',
		staffName: DOULA,
		startsAt: '2026-11-21T22:00',
		endsAt: '2026-11-22T08:00',
		reason: 'https://example.test/a-very-long-address-pasted-where-a-reason-should-have-been-typed/and-it-keeps-going'
	});
}
