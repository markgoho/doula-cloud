import { describe, expect, it, vi } from 'vitest';
import {
	canBeRecommendedASecondFactor,
	canReadConnect,
	closeSecondFactorRecommendation,
	canReadRoster,
	isConnectCardShown,
	isRequestBlockShown,
	isRosterBlockShown,
	isTonightBlockShown,
	loadPracticeLanding,
	loadWaitingOnReplyPage
} from './practiceLanding.js';
import { jsonResponse } from './testResponse.js';

function refusal(body: string): Response {
	return jsonResponse(body, 403);
}

const session = { practiceName: 'Riverside Doula Collective', roles: ['owner', 'doula'] };

const openOffer = { offerId: 'offer-1', state: 'offered' };
const decidedOffer = { offerId: 'offer-2', state: 'accepted' };

const roster = {
	members: [{ staffId: 'staff-1' }, { staffId: 'staff-2' }],
	invitations: { items: [{ expired: false }, { expired: true }], hasMore: false }
};

const tonightRoster = {
	from: '2026-10-31',
	to: '2026-10-31',
	windows: [
		{
			engagementId: 'engagement-1',
			clientName: 'Casey Estrada',
			window: { start: '2026-10-10', end: '2026-11-14' },
			onCall: [{ staffId: 'staff-1', name: 'Rowan Petrosyan', from: '2026-10-31', to: '2026-10-31', narrowed: false }],
			gaps: [],
			unstaffedDays: [],
			uncovered: false
		},
		{
			engagementId: 'engagement-2',
			clientName: 'Quinn Estrada',
			window: { start: '2026-10-20', end: '2026-11-24' },
			onCall: [{ staffId: 'staff-1', name: 'Rowan Petrosyan', from: '2026-10-31', to: '2026-10-31', narrowed: false }],
			gaps: [],
			unstaffedDays: [],
			uncovered: true
		}
	],
	noWindow: [],
	doulas: [
		{ staffId: 'staff-2', name: 'Joss Adeyemi', available: true, concurrentWindows: 0 },
		{ staffId: 'staff-3', name: 'Marisol Terrazas', available: false, concurrentWindows: 0 },
		{ staffId: 'staff-1', name: 'Rowan Petrosyan', available: true, concurrentWindows: 2 }
	]
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
		offers: jsonResponse({ items: [openOffer, decidedOffer] }),
		clients: jsonResponse({ records: 3, active: 2 }),
		staff: jsonResponse(roster),
		// Before `billing`: its path holds `/billing` (#1589).
		'billing-mode': jsonResponse({ billingMode: 'stripe' }),
		billing: jsonResponse({ balance: 7, ledger: { items: [], hasMore: false } }),
		connect: jsonResponse(connectStatus),
		'on-call': jsonResponse(tonightRoster),
		'engagement-requests': jsonResponse({
			items: [{ requestId: 'request-1' }, { requestId: 'request-2' }],
			hasMore: false
		}),
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
		[['admin'], true, true],
		[['doula'], false, false],
		[[], false, false]
	])('%s reads the roster: %s, Connect: %s', (roles, roster_, connect) => {
		expect(canReadRoster(roles)).toBe(roster_);
		expect(canReadConnect(roles)).toBe(connect);
	});
});

// #2228: the Overview draws a block only where something needs her; a
// block that would only repeat the top bar is left off.
describe('isRosterBlockShown', () => {
	const roster = { members: 4, pendingInvitations: 1 };

	it('is drawn where an invitation has expired, the one thing it asks of her', () => {
		expect(isRosterBlockShown({ ...roster, expiredInvitations: 1 })).toBe(true);
	});

	it('is not drawn where every invitation is still good', () => {
		expect(isRosterBlockShown({ ...roster, expiredInvitations: 0 })).toBe(false);
	});

	it('is drawn when the roster read failed, and not for a caller who may not read it', () => {
		expect(isRosterBlockShown('unavailable')).toBe(true);
		expect(isRosterBlockShown(undefined)).toBe(false);
	});
});

describe('isTonightBlockShown', () => {
	const tonight = { onCall: [], free: ['Joss Adeyemi', 'Rowan Petrosyan'], away: [], uncovered: [], doulas: 2 };

	it('is drawn for a Practice with more than one Doula', () => {
		expect(isTonightBlockShown(tonight)).toBe(true);
	});

	it('is not drawn for a Practice of one Doula, whose answer is always herself', () => {
		expect(isTonightBlockShown({ ...tonight, free: ['Maya Okonkwo'], doulas: 1 })).toBe(false);
	});

	it('is drawn when the read failed, and not for a caller who may not read it', () => {
		expect(isTonightBlockShown('unavailable')).toBe(true);
		expect(isTonightBlockShown(undefined)).toBe(false);
	});
});

describe('isRequestBlockShown', () => {
	it('is drawn where a Request waits', () => {
		expect(isRequestBlockShown({ count: 1, hasMore: false })).toBe(true);
	});

	it('is not drawn where nobody is waiting', () => {
		expect(isRequestBlockShown({ count: 0, hasMore: false })).toBe(false);
	});

	it('is drawn when the read failed, and not for a caller who may not read it', () => {
		expect(isRequestBlockShown('unavailable')).toBe(true);
		expect(isRequestBlockShown(undefined)).toBe(false);
	});
});

describe('isConnectCardShown', () => {
	const connect = { status: 'not_connected' as const, requirementsDue: [] };

	it.each([
		['stripe', true],
		['unchosen', true],
		['unavailable', true],
		['by_hand', false]
	] as const)('is drawn for the billing mode %s: %s', (billingMode, shown) => {
		expect(isConnectCardShown({ ...connect, billingMode })).toBe(shown);
	});

	it('is drawn when the Connect read failed, and not for a caller who may not read it', () => {
		expect(isConnectCardShown('unavailable')).toBe(true);
		expect(isConnectCardShown(undefined)).toBe(false);
	});

	it.each([
		['onboarding_incomplete', [], true],
		['pending', [], true],
		['payouts_restricted', [], true],
		['active', ['individual.dob'], true],
		['active', [], false]
	] as const)('with Stripe %s and %j due, is drawn: %s', (status, requirementsDue, shown) => {
		expect(
			isConnectCardShown({ status, requirementsDue: [...requirementsDue], billingMode: 'stripe' })
		).toBe(shown);
	});
});

describe('loadPracticeLanding', () => {
	it('keeps only the Offers still awaiting an answer', async () => {
		const landing = await loadPracticeLanding(fetcherFor(), 'practice-1', session);

		expect(landing.openOffers).toEqual([openOffer]);
		expect(landing.practiceName).toBe('Riverside Doula Collective');
	});

	// #2095: one count read, not a page of rows, says whether there are any.
	it('says whether the Practice holds a Client, from the count read', async () => {
		const landing = await loadPracticeLanding(fetcherFor(), 'practice-1', session);

		expect(landing.hasClients).toBe(true);
	});

	it('counts the roster and the invitations that have expired', async () => {
		const landing = await loadPracticeLanding(fetcherFor(), 'practice-1', session);

		expect(landing.roster).toEqual({ members: 2, pendingInvitations: 2, expiredInvitations: 1 });
		expect(landing.credit).toEqual({ balance: 7, count: '7 Credits' });
		expect(landing.connect).toEqual({
			status: 'onboarding_incomplete',
			requirementsDue: ['individual.dob'],
			billingMode: 'stripe'
		});
		expect(landing.requests).toEqual({ count: 2, hasMore: false });
	});

	// #2234: no days named, so the BFF answers for today in the Practice's zone.
	it('reads who is free tonight from the roster for today', async () => {
		const fetcher = fetcherFor();

		const landing = await loadPracticeLanding(fetcher, 'practice-1', session);

		expect(fetcher.mock.calls.map(([path]) => path)).toContain('/api/practices/practice-1/on-call');
		expect(landing.tonight).toEqual({
			onCall: [{ name: 'Rowan Petrosyan', clients: ['Casey Estrada', 'Quinn Estrada'] }],
			free: ['Joss Adeyemi'],
			away: ['Marisol Terrazas'],
			uncovered: ['Quinn Estrada'],
			doulas: 3
		});
	});

	it('says the roster could not be read, rather than hiding it', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({ 'on-call': refusal('no') }),
			'practice-1',
			session
		);

		expect(landing.tonight).toBe('unavailable');
	});

	// The count is one page deep, so a Practice past that page reads as
	// "30+" rather than as a wrong number -- `hasMore` is what the hub
	// prints the plus from.
	it('carries hasMore when more Requests wait than one page holds', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({
				'engagement-requests': jsonResponse({ items: [{ requestId: 'request-1' }], hasMore: true })
			}),
			'practice-1',
			session
		);

		expect(landing.requests).toEqual({ count: 1, hasMore: true });
	});

	it('reports a Practice with no Clients as empty', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({ clients: jsonResponse({ records: 0, active: 0 }) }),
			'practice-1',
			session
		);

		expect(landing.hasClients).toBe(false);
	});

	it('asks for no gated block on behalf of a Doula', async () => {
		const fetcher = fetcherFor();

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, roles: ['doula'] });

		expect([landing.roster, landing.credit, landing.connect, landing.requests]).toEqual([
			undefined,
			undefined,
			undefined,
			undefined
		]);
		expect(fetcher.mock.calls.map(([path]) => path)).toEqual([
			'/api/practices/practice-1/offers',
			'/api/practices/practice-1/clients/count'
		]);
	});

	// #267 moved Connect onto ADR-0008's Owner-and-Admin row, beside the
	// Invoice history it is the payment rail for, so the Admin's Overview
	// carries the same three blocks the Owner's does.
	it('asks for the roster, credits and Connect on behalf of an Admin', async () => {
		const fetcher = fetcherFor();

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, roles: ['admin'] });

		expect(landing.connect).not.toBeUndefined();
		expect(landing.roster).not.toBeUndefined();
		expect(landing.credit).not.toBeUndefined();
	});

	// #2141: an Offer goes to a Doula, so an Admin who is not one has no
	// inbox to read.
	it('reads no Offers on behalf of a Staff member who is not a Doula', async () => {
		const fetcher = fetcherFor();

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, roles: ['admin'] });

		expect(landing.openOffers).toEqual([]);
		expect(fetcher.mock.calls.map(([path]) => path)).not.toContain('/api/practices/practice-1/offers');
	});

	// #1589: the mode rides on the Connect block, and a read that fails
	// leaves the block in place -- the card is shown, not hidden.
	it('reads a Practice that has not chosen a billing mode as unchosen', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({ 'billing-mode': jsonResponse({}) }),
			'practice-1',
			session
		);

		expect(landing.connect).toMatchObject({ billingMode: 'unchosen' });
	});

	it('keeps the Connect block when the billing mode cannot be read', async () => {
		const landing = await loadPracticeLanding(
			fetcherFor({ 'billing-mode': refusal('nope') }),
			'practice-1',
			session
		);

		expect(landing.connect).toMatchObject({ status: 'onboarding_incomplete', billingMode: 'unavailable' });
	});

	it.each([['staff'], ['billing'], ['connect'], ['engagement-requests']])(
		'marks the %s block unavailable rather than failing the page',
		async (endpoint) => {
			const landing = await loadPracticeLanding(
				fetcherFor({ [endpoint]: refusal('nope') }),
				'practice-1',
				session
			);

			expect([landing.roster, landing.credit, landing.connect, landing.requests]).toContain(
				'unavailable'
			);
			expect(landing.practiceName).toBe('Riverside Doula Collective');
		}
	);

	it.each([['offers'], ['clients']])('fails the whole page when %s refuses', async (endpoint) => {
		const fetcher = fetcherFor({ [endpoint]: refusal('nope') });

		await expect(loadPracticeLanding(fetcher, 'practice-1', session)).rejects.toThrow('nope');
	});
});

// #1533: the notice that recommends a second factor.
describe('the second-factor recommendation', () => {
	const recommendationPath = '/api/practices/practice-1/second-factor-recommendation';

	it.each([
		[['owner'], false, true],
		[['owner'], true, false],
		[['owner'], undefined, false],
		[['admin'], false, false],
		[['doula'], false, false]
	] as const)('roles %s with secondFactor %s is asked: %s', (roles, secondFactor, asked) => {
		expect(canBeRecommendedASecondFactor({ roles: [...roles], secondFactor })).toBe(asked);
	});

	it('reads the BFF answer for an Owner with no second factor', async () => {
		const fetcher = fetcherFor({ 'second-factor-recommendation': jsonResponse({ recommended: true }) });

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, secondFactor: false });

		expect(landing.secondFactorRecommended).toBe(true);
		expect(fetcher).toHaveBeenCalledWith(recommendationPath);
	});

	it('does not ask for anyone else', async () => {
		const fetcher = fetcherFor();

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, roles: ['admin'], secondFactor: false });

		expect(landing.secondFactorRecommended).toBe(false);
		expect(fetcher).not.toHaveBeenCalledWith(recommendationPath);
	});

	it.each([
		['a refusal', () => Promise.resolve(refusal('nope'))],
		['a network failure', () => Promise.reject(new Error('offline'))]
	])('shows nothing after %s', async (_name, answer) => {
		const base = fetcherFor();
		const fetcher = vi.fn((path: string) => (path === recommendationPath ? answer() : base(path)));

		const landing = await loadPracticeLanding(fetcher, 'practice-1', { ...session, secondFactor: false });

		expect(landing.secondFactorRecommended).toBe(false);
	});

	it('closes it with a POST, and throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response(undefined, { status: 204 }));
		await closeSecondFactorRecommendation(fetcher, 'practice-1');
		expect(fetcher).toHaveBeenCalledWith(`${recommendationPath}/close`, { method: 'POST' });

		fetcher.mockResolvedValue(refusal('nope'));
		await expect(closeSecondFactorRecommendation(fetcher, 'practice-1')).rejects.toThrow('nope');
	});
});

// #455's roll-up, cursor-paginated on its own -- mirrors
// activityLedger.spec.ts's loadPracticeActivityPage tests.
describe('loadWaitingOnReplyPage', () => {
	it('reads the first page without a cursor parameter', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadWaitingOnReplyPage(fetcher, 'practice-1', '');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/messages/awaiting-reply');
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadWaitingOnReplyPage(fetcher, 'practice-1', 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(
			'/api/practices/practice-1/messages/awaiting-reply?cursor=a%20b%2Fc'
		);
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadWaitingOnReplyPage(fetcher, 'practice-1', '')).rejects.toThrow('nope');
	});
});
