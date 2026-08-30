import { describe, expect, it, vi } from 'vitest';
import {
	canReadConnect,
	canReadRoster,
	hasSecondary,
	loadPracticeLanding,
	type PracticeLanding
} from './practiceLanding.js';
import { jsonResponse } from './testResponse.js';

function refusal(body: string): Response {
	return jsonResponse(body, 403);
}

const session = { practiceName: 'Riverside Doula Collective', roles: ['owner'] };

const openOffer = { offerId: 'offer-1', state: 'offered' };
const decidedOffer = { offerId: 'offer-2', state: 'accepted' };

const roster = {
	members: [{ staffId: 'staff-1' }, { staffId: 'staff-2' }],
	invitations: { items: [{ expired: false }, { expired: true }], hasMore: false }
};

const connectStatus = {
	status: 'onboarding_incomplete',
	cardPaymentsStatus: 'restricted',
	payoutsStatus: 'restricted',
	requirementsDue: ['individual.dob']
};

/** Routes each request to its canned answer, so a test only has to say
 * which endpoints behave differently from the happy path. */
function fetcherFor(overrides: Record<string, Response> = {}) {
	const answers: Record<string, Response> = {
		session: jsonResponse(session),
		offers: jsonResponse({ items: [openOffer, decidedOffer] }),
		clients: jsonResponse({ items: [{ clientId: 'client-1' }], hasMore: false }),
		staff: jsonResponse(roster),
		billing: jsonResponse({ balance: 7, ledger: { items: [], hasMore: false } }),
		connect: jsonResponse(connectStatus),
		...overrides
	};

	return vi.fn((path: string) => {
		const key = Object.keys(answers).find((name) =>
			name === 'connect' ? path.includes('/payments/connect') : path.includes(`/${name}`)
		);
		return Promise.resolve(answers[key!]);
	});
}

describe('role gates', () => {
	it.each([
		[['owner'], true, true],
		[['admin'], true, false],
		[['doula'], false, false],
		[[], false, false]
	])('%s reads the roster: %s, Connect: %s', (roles, roster_, connect) => {
		expect(canReadRoster(roles)).toBe(roster_);
		expect(canReadConnect(roles)).toBe(connect);
	});
});

describe('hasSecondary', () => {
	it('is false when no block is entitled', () => {
		expect(
			hasSecondary({ roster: undefined, credit: undefined, connect: undefined } as PracticeLanding)
		).toBe(false);
	});

	it('is true when a block failed, so the rail can say so', () => {
		expect(
			hasSecondary({
				roster: 'unavailable',
				credit: undefined,
				connect: undefined
			} as PracticeLanding)
		).toBe(true);
	});
});

describe('loadPracticeLanding', () => {
	it('keeps only the Offers still awaiting an answer', async () => {
		const landing = await loadPracticeLanding(fetcherFor(), 'practice-1');

		expect(landing.openOffers).toEqual([openOffer]);
		expect(landing.practiceName).toBe('Riverside Doula Collective');
	});

	it('counts the roster and the invitations that have expired', async () => {
		const landing = await loadPracticeLanding(fetcherFor(), 'practice-1');

		expect(landing.roster).toEqual({ members: 2, pendingInvitations: 2, expiredInvitations: 1 });
		expect(landing.credit).toEqual({ balance: 7 });
		expect(landing.connect).toEqual({
			status: 'onboarding_incomplete',
			requirementsDue: ['individual.dob']
		});
	});

	it('reports a Practice with no Clients as empty', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({ clients: jsonResponse({ items: [], hasMore: false }) }),
			'practice-1'
		);

		expect(landing.hasClients).toBe(false);
	});

	it('asks for no gated block on behalf of a Doula', async () => {
		const fetcher = fetcherFor({ session: jsonResponse({ ...session, roles: ['doula'] }) });

		const landing = await loadPracticeLanding(fetcher, 'practice-1');

		expect([landing.roster, landing.credit, landing.connect]).toEqual([
			undefined,
			undefined,
			undefined
		]);
		expect(fetcher.mock.calls.map(([path]) => path)).toEqual([
			'/api/practices/practice-1/session',
			'/api/practices/practice-1/offers',
			// `?all=true`, because the default list is Clients who have work
			// and the question here is whether the Practice has any at all.
			'/api/practices/practice-1/clients?all=true'
		]);
	});

	it('asks for the roster and credits but not Connect on behalf of an Admin', async () => {
		const fetcher = fetcherFor({ session: jsonResponse({ ...session, roles: ['admin'] }) });

		const landing = await loadPracticeLanding(fetcher, 'practice-1');

		expect(landing.connect).toBeUndefined();
		expect(landing.roster).not.toBeUndefined();
		expect(landing.credit).not.toBeUndefined();
	});

	it.each([['staff'], ['billing'], ['connect']])(
		'marks the %s block unavailable rather than failing the page',
		async (endpoint) => {
			const landing = await loadPracticeLanding(
				fetcherFor({ [endpoint]: refusal('nope') }),
				'practice-1'
			);

			expect([landing.roster, landing.credit, landing.connect]).toContain('unavailable');
			expect(landing.practiceName).toBe('Riverside Doula Collective');
		}
	);

	it.each([['session'], ['offers'], ['clients']])(
		'fails the whole page when %s refuses',
		async (endpoint) => {
			const fetcher = fetcherFor({ [endpoint]: refusal('nope') });

			await expect(loadPracticeLanding(fetcher, 'practice-1')).rejects.toThrow('nope');
		}
	);
});
