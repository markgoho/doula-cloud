import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
// The Skeleton reserves a line of body copy with `var(--text-body-size)`,
// so without the tokens it draws at zero height and reserves nothing --
// the very thing ADR-0020 asks it to do. The real app loads these in the
// root layout.
import '#lib/styles/app.css';

vi.mock('$app/state', () => ({ page: { params: { practiceId: 'practice-1' } } }));

const apiFetch = vi.hoisted(() => vi.fn());
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetch,
	apiFetchWithSession,
	// payments.ts reads a failure through this; the mock has to carry
	// every export the module tree imports, not only the ones used here.
	apiErrorMessage: (response: Response) => response.text()
}));

const registerPushSubscription = vi.hoisted(() => vi.fn());
vi.mock('#lib/pushRegistration.js', () => ({
	registerPushSubscription,
	practicePushSubscriptionsPath: (practiceId: string) =>
		`/api/practices/${practiceId}/push-subscriptions`
}));

function refusal(body: string): Response {
	return jsonResponse(body, 403);
}

const offer = {
	offerId: 'offer-1',
	state: 'offered',
	clientFirstInitial: 'T',
	clientArea: 'Rochester',
	dueDate: '2027-03-04',
	amountCents: 45_000,
	employmentType: 'contractor',
	offeredAt: '2026-08-01T00:00:00Z',
	expiresAt: '2026-08-08T00:00:00Z'
};

interface SetupOptions {
	roles?: string[];
	clients?: unknown[];
	overrides?: Record<string, Response>;
}

async function setup({ roles = ['owner'], clients = [{ clientId: 'c1' }], overrides = {} }: SetupOptions = {}) {
	const answers: Record<string, Response> = {
		session: jsonResponse({ practiceName: 'Riverside Doula Collective', roles }),
		offers: jsonResponse({ items: [offer, { offerId: 'offer-2', state: 'declined' }] }),
		clients: jsonResponse({ items: clients, hasMore: false }),
		staff: jsonResponse({
			members: [{ staffId: 's1' }, { staffId: 's2' }],
			invitations: { items: [{ expired: true }], hasMore: false }
		}),
		billing: jsonResponse({ balance: 12, ledger: { items: [], hasMore: false } }),
		connect: jsonResponse({
			status: 'onboarding_incomplete',
			cardPaymentsStatus: 'restricted',
			payoutsStatus: 'restricted',
			requirementsDue: ['individual.dob']
		}),
		...overrides
	};

	apiFetchWithSession.mockImplementation((path: string) => {
		const key = Object.keys(answers).find((name) =>
			name === 'connect' ? path.includes('/payments/connect') : path.includes(`/${name}`)
		);
		return Promise.resolve(answers[key!]);
	});

	await render(Page, {});
}

beforeEach(() => {
	apiFetch.mockReset();
	apiFetchWithSession.mockReset();
	registerPushSubscription.mockReset();
	registerPushSubscription.mockResolvedValue(undefined);
});

describe('the Practice landing page', () => {
	it('leads with the Offers still awaiting an answer, and drops the decided ones', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: 'Offers awaiting your answer' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Accept' })).toBeVisible();
		expect(testPage.getByText('Declined').elements()).toHaveLength(0);
	});

	it('shows the roster, the credit balance and the Stripe state to an Owner', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		await expect.element(testPage.getByText('1 invitation expired')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
		await expect.element(testPage.getByText('12')).toBeVisible();
		await expect.element(testPage.getByText('Onboarding incomplete')).toBeVisible();
		await expect
			.element(testPage.getByText('Stripe is waiting on 1 more detail.'))
			.toBeVisible();
	});

	it('shows an Admin no Stripe block, because that endpoint would refuse her', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByRole('heading', { name: 'Your people' })).toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Getting paid' }).elements()).toHaveLength(0);
	});

	it('shows a Doula her Offers and no rail at all', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByRole('button', { name: 'Accept' })).toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Your people' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('heading', { name: 'Credits' }).elements()).toHaveLength(0);
	});

	it('says so when a rail block fails, rather than letting it vanish', async () => {
		await setup({ overrides: { billing: refusal('nope') } });

		await expect.element(testPage.getByText('Could not load your credit balance just now.')).toBeVisible();
	});

	it('names doula work and offers one action on a Practice with no Clients', async () => {
		await setup({ clients: [] });

		await expect.element(testPage.getByText(/her birth plan, the visits you make/)).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Add your first Client' }))
			.toBeVisible();
		// The abandon point was a menu of administration. One action, not eight.
		expect(testPage.getByRole('link').elements()).toHaveLength(1);
	});

	it('reserves the page while it loads, rather than flashing empty', async () => {
		apiFetchWithSession.mockReturnValue(new Promise(() => {}));
		await render(Page, {});

		await expect
			.element(testPage.getByRole('status', { name: 'Loading your Practice' }))
			.toBeVisible();
	});

	it('surfaces a failure of the critical path in place', async () => {
		await setup({ overrides: { session: refusal('your session has expired') } });

		await expect.element(testPage.getByText('your session has expired')).toBeVisible();
	});

	it('still registers this device for push once it has landed', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Credits' })).toBeVisible();
		expect(registerPushSubscription).toHaveBeenCalledWith(
			'/api/practices/practice-1/push-subscriptions',
			apiFetch
		);
	});
});
