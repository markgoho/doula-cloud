/*
 * The Practice-side Birth Plan (#280), as the continuum check sees it
 * (#595). Mirrors the Client-portal Birth Plan's own fixture one
 * directory group over: same two field states (a `long_text` answer and
 * a `multi_select` answer, since `BirthPlanView`'s render branches on
 * both), and the same hostile URL-in-free-text answer that broke a grid
 * track on #530.
 *
 * `props.data` restates what `+page.ts`'s own `load` would return -- the
 * Engagement summary, not the Birth Plan itself, which this route fetches
 * after mount the same as the hub's own Plan sections do. `respond`
 * answers that fetch for the drag surface and for a route spec reusing
 * this fixture; the fixture itself never calls `load`.
 */
import { jsonResponse } from '#lib/testResponse.js';
import type { Instance } from '#lib/planInstance.js';
import type { RouteFixture } from '../../../../../routeFixture.js';
import type { RouteParams as RouteParameters } from './$types';
import Page from './+page.svelte';

const clientName = 'Anne-Marie Ochieng-Whitfield';

export const engagement = {
	engagementId: 'engagement-1',
	clientId: 'client-1',
	clientName,
	status: 'active',
	createdAt: '2026-08-01T00:00:00Z',
	dueDate: '2027-03-01',
	statusMoves: ['completed']
};

export const instance: Instance = {
	engagementId: 'engagement-1',
	planType: 'birth_plan',
	fields: [
		{ id: 'support-people', type: 'long_text', label: 'Who do you want with you, and what should we know about them?', order: 1 },
		{ id: 'people-present', type: 'multi_select', label: 'Who do you want in the room with you?', order: 2 }
	],
	answers: {
		'support-people':
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake',
		'people-present': [
			'Her partner, for the whole labor',
			'https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake'
		]
	}
};

export const fixture: RouteFixture<RouteParameters> = {
	name: 'The Practice-side Birth Plan',
	component: Page,
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' },
	url: 'https://example.test/practices/practice-1/engagements/engagement-1/birth-plan',
	props: {
		data: {
			...engagement,
			session: {
				practiceId: 'practice-1',
				practiceName: 'Riverside Doula Collective',
				roles: ['owner', 'doula'],
				isContractor: false
			}
		}
	},
	respond: (path) => {
		if (path.endsWith('/plans/birth_plan')) return jsonResponse(instance);
		throw new Error(`birth-plan fixture: unmatched fetch path ${path}`);
	},
	readyText: `${clientName}'s Birth Plan`
};
