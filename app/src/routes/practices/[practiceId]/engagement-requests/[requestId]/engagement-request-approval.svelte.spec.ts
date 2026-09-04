import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ApprovalDetail } from '#lib/engagementRequest.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { detail as baseDetail, fixture } from './page.fixture.js';

/*
 * The Request this screen decides on, and the `page` it reads, both come
 * from the route's own fixture (#596) -- so the screen this spec asserts
 * on and the screen the continuum sweep measures are one description.
 * `vi.mock` is hoisted above every import, so `pageState` is declared
 * empty and filled from the fixture once the imports have run; the route
 * reads `page` inside its own functions rather than destructuring it at
 * module scope, so the later write is seen. Same installation, through
 * the same `toPageState`, as `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId, requestId } = fixture.params;
const clientName = `${baseDetail.client.givenName} ${baseDetail.client.familyName}`;
/*
 * A Note is optional on `ApprovalDetail` and the fixture always carries
 * one -- it is the URL #530 measured, and the whole reason the sweep
 * finds this screen at 320px. Narrowed once, loudly: a fixture that lost
 * its Note would otherwise turn an assertion about the screen into an
 * assertion about `undefined`.
 */
const { note } = baseDetail;
if (note === undefined) {
	throw new Error(`${fixture.name} must carry a Note: it is the value this screen is measured on.`);
}
const requestPath = `/api/practices/${practiceId}/engagement-requests/${requestId}`;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	sessionStorage.clear();
});

interface MockOptions {
	detail?: ApprovalDetail;
	detailStatus?: number;
	detailBody?: unknown;
	approveStatus?: number;
	approveBody?: unknown;
	refuseStatus?: number;
	refuseBody?: unknown;
}

function mockFetches({
	detail = baseDetail,
	detailStatus = 200,
	detailBody,
	approveStatus = 200,
	approveBody,
	refuseStatus = 200,
	refuseBody
}: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/approve')) {
			return Promise.resolve(
				jsonResponse(
					approveBody ?? { requestId: 'request-1', engagementId: 'engagement-9', state: 'approved' },
					approveStatus
				)
			);
		}
		if (path.endsWith('/refuse')) {
			return Promise.resolve(jsonResponse(refuseBody ?? { requestId: 'request-1', state: 'refused' }, refuseStatus));
		}
		return Promise.resolve(jsonResponse(detailBody ?? detail, detailStatus));
	});
}

async function setup(options: MockOptions = {}) {
	mockFetches(options);
	await render(Page, {});
	await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
}

// A Request that cannot be rendered has no page heading -- the Template
// draws the refusal alone -- so the load-failure cases wait on the
// message instead.
async function setupUnreadable(options: MockOptions) {
	mockFetches(options);
	await render(Page, {});
}

describe('the approval screen', () => {
	it('reads the Request by its own id, independent of any inbox', async () => {
		await setup();

		expect(apiFetchWithSession).toHaveBeenCalledWith(requestPath);
	});

	it('shows every fact the approver decides on, including the balance after the Credit', async () => {
		await setup();

		await expect.element(testPage.getByText(`${clientName} -- new to this practice`)).toBeVisible();
		await expect.element(testPage.getByText(`${baseDetail.requestedByName} on Aug 1, 2026`)).toBeVisible();
		await expect.element(testPage.getByText('Birth', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(testPage.getByText(note)).toBeVisible();
		await expect.element(testPage.getByText('1 credit')).toBeVisible();
		await expect.element(testPage.getByText('Balance after')).toBeVisible();
		await expect.element(testPage.getByText('2', { exact: true })).toBeVisible();
	});

	it('names a Client the Practice already knows as known, and lists her Engagements', async () => {
		await setup({
			detail: {
				...baseDetail,
				client: { ...baseDetail.client, isNewToPractice: false },
				engagements: [
					{ engagementId: 'engagement-1', kind: 'postpartum', status: 'completed', createdAt: '2024-05-02T00:00:00Z' }
				]
			}
		});

		await expect.element(testPage.getByText(`${clientName} -- already known here`)).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: /^Postpartum work, started .+ -- completed$/ }))
			.toHaveAttribute('href', `/practices/${practiceId}/engagements/engagement-1`);
	});

	it('draws no control that amends the kind or the due date', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Kind of work')).not.toBeInTheDocument();
		await expect.element(testPage.getByLabelText('Due date')).not.toBeInTheDocument();
	});

	it('warns on a second live Engagement without blocking the decision', async () => {
		await setup({ detail: { ...baseDetail, warning: 'this client already has a live engagement' } });

		await expect.element(testPage.getByText(`${clientName} already has a live engagement.`, { exact: false })).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Approve and start the work' })).toBeEnabled();
	});

	it('approves and lands on the Engagement the approval created', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Approve and start the work' }).click();

		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/engagements/engagement-9`));
	});

	it('offers Buy credits inline on an empty balance, leaving the same screen intact', async () => {
		await setup({ detail: { ...baseDetail, balance: 0, balanceAfter: -1 }, approveStatus: 402 });

		await expect.element(testPage.getByRole('link', { name: 'Buy credits' })).toHaveAttribute(
			'href',
			`/practices/${practiceId}/billing`
		);

		await testPage.getByRole('button', { name: 'Approve and start the work' }).click();

		await expect.element(testPage.getByText("There are no credits left on this practice's balance.")).toBeVisible();
		await expect.element(testPage.getByText(`${baseDetail.requestedByName} on Aug 1, 2026`)).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('leaves the way back to this decision behind before sending her to buy credits', async () => {
		await setup({ detail: { ...baseDetail, balance: 0, balanceAfter: -1 } });

		await expect
			.element(testPage.getByRole('link', { name: 'Buy credits' }))
			.toBeVisible();
		expect(sessionStorage.getItem('engagement-request-approval-return')).toBe(
			`/practices/${practiceId}/engagement-requests/${requestId}`
		);
	});

	it('forgets the way back once she is on the screen again, so it is offered only once', async () => {
		sessionStorage.setItem('engagement-request-approval-return', `/practices/${practiceId}/engagement-requests/${requestId}`);

		await setup();

		expect(sessionStorage.getItem('engagement-request-approval-return')).toBeNull();
	});

	it('refuses to submit a refusal with no reason, client-side, before any request', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Refuse this request' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter why this request is being refused' }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('refuses with the reason and lands back on her record', async () => {
		await setup();

		await testPage.getByLabelText('Why are you refusing this?').fill('No capacity in March');
		await testPage.getByRole('button', { name: 'Refuse this request' }).click();

		await vi.waitFor(() =>
			expect(apiFetchWithSession).toHaveBeenCalledWith(
				`${requestPath}/refuse`,
				expect.objectContaining({ body: JSON.stringify({ reason: 'No capacity in March' }) })
			)
		);
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/${baseDetail.client.clientId}`));
	});

	it('surfaces a Request somebody already decided rather than rendering a dead screen', async () => {
		await setupUnreadable({ detailStatus: 409, detailBody: 'that request is no longer pending -- it is approved' });

		await expect.element(testPage.getByText('that request is no longer pending -- it is approved')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Approve and start the work' })).not.toBeInTheDocument();
	});

	it('surfaces an endpoint refusal on approve as an error rather than a silent failure', async () => {
		await setup({ approveStatus: 409, approveBody: 'that request is no longer pending' });

		await testPage.getByRole('button', { name: 'Approve and start the work' }).click();

		await expect.element(testPage.getByText('that request is no longer pending')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('exposes the decision controls and her record as reachable, labelled controls', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Approve and start the work' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Why are you refusing this?')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Refuse this request' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: `View ${clientName}'s record` }))
			.toHaveAttribute('href', `/practices/${practiceId}/clients/${baseDetail.client.clientId}`);
	});
});
