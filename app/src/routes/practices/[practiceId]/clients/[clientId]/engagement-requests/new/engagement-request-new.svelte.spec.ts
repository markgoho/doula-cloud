import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { displayName, type ClientDetail, type EngagementSummary } from '#lib/clientDetail.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../../../routeFixture.js';
import type { RequestDoulas } from '#lib/engagementRequest.js';
import { agencyRoster, detail as baseDetail, fixture, ownDoula, soloRoster } from './page.fixture.js';

/*
 * The Client this Request is about, and the `page` it reads, both come
 * from the route's own fixture (#596) -- so what this spec asserts on
 * and what the continuum sweep measures are one description. `vi.mock`
 * is hoisted above every import, so `pageState` is declared empty and
 * filled from the fixture once the imports have run. Same installation,
 * through the same `toPageState`, as `route-continuum.svelte.spec.ts`.
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

const { practiceId, clientId } = fixture.params;
const clientDetailHref = `/practices/${practiceId}/clients/${clientId}`;
const clientName = displayName(baseDetail);

const liveEngagement: EngagementSummary = {
	engagementId: 'engagement-0',
	kind: 'postpartum',
	status: 'active',
	createdAt: '2026-01-01T00:00:00Z'
};

const draftKey = `engagement-request-draft:${clientId}`;

// The person at the form in every fixture session: the first entry of
// each of the fixture's Doula lists, and `practiceSession`'s own staffId.
const self = ownDoula.items[0]!;
const selfLabel = `${self.name} (you)`;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	sessionStorage.clear();
});

interface MockOptions {
	detail?: ClientDetail;
	roles?: string[];
	balance?: number;
	doulas?: RequestDoulas;
	requestOutcome?: unknown;
	requestStatus?: number;
}

function mockFetches({
	detail = baseDetail,
	roles = ['doula'],
	balance = 3,
	doulas = ownDoula,
	requestOutcome,
	requestStatus = 201
}: MockOptions = {}) {
	// The Membership (roles) comes off page.data.session (#835), not a
	// fetch this mock has to answer.
	pageState.data = {
		session: {
			practiceId,
			staffId: self.staffId,
			practiceName: 'Riverside Doula Collective',
			roles,
			isContractor: false
		}
	};
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/billing')) return Promise.resolve(jsonResponse({ balance, ledger: { items: [], hasMore: false } }));
		if (path.endsWith('/engagement-request-doulas')) return Promise.resolve(jsonResponse(doulas));
		if (path.endsWith('/engagement-requests')) {
			return Promise.resolve(jsonResponse(requestOutcome ?? { requestId: 'request-1', state: 'pending' }, requestStatus));
		}
		return Promise.resolve(jsonResponse(detail));
	});
}

async function setup(options: MockOptions = {}) {
	mockFetches(options);
	await render(Page, {});
	await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
	return options;
}

/**
 * The body the form posted to the Request endpoint.
 */
function postedBody(): { kind: string; dueDate: string; note: string; doulaStaffId?: string } {
	const call = apiFetchWithSession.mock.calls.find(([path]) => (path as string).endsWith('/engagement-requests'));
	return JSON.parse((call![1] as RequestInit).body as string);
}

describe('the Engagement Request screen', () => {
	it('shows a Doula the "Ask to" phrasing and no Credit preview', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByRole('button', { name: `Ask to start work with ${clientName}` })).toBeVisible();
		await expect.element(testPage.getByText('Credit cost')).not.toBeInTheDocument();
	});

	it('shows an Owner the "Start work with" phrasing and the Credit cost and balance after', async () => {
		await setup({ roles: ['owner'], balance: 3 });

		await expect.element(testPage.getByRole('button', { name: `Start work with ${clientName}` })).toBeVisible();
		await expect.element(testPage.getByText('Credit cost')).toBeVisible();
		await expect.element(testPage.getByText('1 credit')).toBeVisible();
		await expect.element(testPage.getByText('2', { exact: true })).toBeVisible();
	});

	it('states a zero balance before any submit, never a negative Balance after', async () => {
		await setup({ roles: ['owner'], balance: 0 });

		await expect
			.element(testPage.getByText("There are no credits left on this Practice's balance."))
			.toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Buy credits' })).toBeVisible();
		await expect.element(testPage.getByText('Credit cost')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('-1', { exact: true })).not.toBeInTheDocument();
	});

	it('saves what she typed while a zero balance is already on screen, before any submit', async () => {
		await setup({ roles: ['owner'], balance: 0 });

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');

		await expect.poll(() => sessionStorage.getItem(draftKey)).toContain('2027-03-01');
	});

	it('shows an Admin the same "Start work with" phrasing', async () => {
		await setup({ roles: ['admin'], balance: 5 });

		await expect.element(testPage.getByRole('button', { name: `Start work with ${clientName}` })).toBeVisible();
	});

	it('refuses a submit with no kind chosen, client-side, before any request', async () => {
		await setup();

		await testPage.getByLabelText('No Doula yet').click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Select whether this is birth or postpartum work' }))
			.toBeVisible();
		// GOV.UK asks for the refusal twice: the summary link above, and
		// again against the radio group itself. Both are role="alert" -- the
		// summary comes first in the DOM, so the group's own message is the
		// last one.
		await expect
			.element(testPage.getByRole('alert').last())
			.toHaveTextContent('Select whether this is birth or postpartum work');
		// The load calls (detail, session) are the only requests made -- the
		// refusal never reached the network.
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			expect.stringContaining('/engagement-requests'),
			expect.anything()
		);
	});

	it('demands a due date on birth work', async () => {
		await setup();

		await testPage.getByLabelText('Birth').click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect.element(testPage.getByRole('link', { name: 'Enter the due date' })).toBeVisible();
		expect(apiFetchWithSession).not.toHaveBeenCalledWith(
			expect.stringContaining('/engagement-requests'),
			expect.anything()
		);
	});

	it('sends postpartum work with no due date, which ADR-0017 makes nullable', async () => {
		await setup();

		await testPage.getByLabelText('Postpartum').click();
		await expect.element(testPage.getByText('Optional for postpartum work')).toBeVisible();
		await testPage.getByLabelText('No Doula yet').click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		// "No Doula yet" travels as no doulaStaffId at all, which the BFF reads
		// as nobody named.
		expect(postedBody()).toStrictEqual({ kind: 'postpartum', dueDate: '', note: '' });
	});

	it('warns on a second live Engagement without blocking the submit', async () => {
		await setup({ detail: { ...baseDetail, engagements: [liveEngagement] } });

		await expect.element(testPage.getByText('already has a live Engagement', { exact: false })).toBeVisible();

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByLabelText(selfLabel).click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
	});

	// #1611: an approved start has an Engagement, and the flow ends on it
	// with a message that the work started.
	it('lands on the Engagement it started when the start is approved', async () => {
		await setup({
			roles: ['owner', 'admin', 'doula'],
			doulas: soloRoster,
			requestOutcome: { requestId: 'request-1', state: 'approved', engagementId: 'engagement-1' }
		});

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/engagements/engagement-1?started=true`);
	});

	// A request that waits for an approver has no Engagement yet, so it
	// lands on her record, where the pending block is.
	it('lands back on the Client detail hub when the request waits for an approver', async () => {
		await setup();

		await testPage.getByLabelText('Postpartum').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByLabelText('Note').fill('Referred by the hospital');
		await testPage.getByLabelText(selfLabel).click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith(clientDetailHref);

		expect(postedBody()).toEqual({
			kind: 'postpartum',
			dueDate: '2027-03-01',
			note: 'Referred by the hospital',
			doulaStaffId: self.staffId
		});
	});

	it('surfaces an empty balance with an inline Buy credits path, leaving the typed form in place', async () => {
		await setup({ roles: ['owner'], requestStatus: 402 });

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByLabelText('No Doula yet').click();
		await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

		await expect.element(testPage.getByRole('link', { name: 'Buy credits' })).toBeVisible();
		// Nothing was lost: the same mounted form still holds what she typed.
		await expect.element(testPage.getByLabelText('Due date')).toHaveValue('2027-03-01');
		await expect.element(testPage.getByLabelText('Birth')).toBeChecked();
		expect(goto).not.toHaveBeenCalled();
		expect(sessionStorage.getItem(draftKey)).toContain('2027-03-01');
	});

	it('restores a saved draft on mount, the far side of the Buy Credits round trip', async () => {
		sessionStorage.setItem(draftKey, JSON.stringify({ kind: 'postpartum', dueDate: '2027-06-15', note: 'Twins' }));

		await setup();

		await expect.element(testPage.getByLabelText('Postpartum')).toBeChecked();
		await expect.element(testPage.getByLabelText('Due date')).toHaveValue('2027-06-15');
		await expect.element(testPage.getByLabelText('Note')).toHaveValue('Twins');
	});

	it('surfaces the endpoint refusal as an error rather than a silent failure', async () => {
		await setup({ requestOutcome: 'a pending request for this client and kind already exists', requestStatus: 409 });

		await testPage.getByLabelText('Birth').click();
		await testPage.getByLabelText('Due date').fill('2027-03-01');
		await testPage.getByLabelText('No Doula yet').click();
		await testPage.getByRole('button', { name: `Ask to start work with ${clientName}` }).click();

		await expect
			.element(testPage.getByText('a pending request for this client and kind already exists'))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	// #1611: the second action leaves the Client saved and nothing
	// started, so it says where it goes rather than "Cancel".
	it('exposes the kind options, due date, note, submit and the way to her record as reachable, labeled controls', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Birth')).toBeVisible();
		await expect.element(testPage.getByLabelText('Postpartum')).toBeVisible();
		await expect.element(testPage.getByLabelText('Due date')).toBeVisible();
		await expect.element(testPage.getByLabelText('Note')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: `Ask to start work with ${clientName}` })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: `Go to ${clientName}'s record without starting work` }))
			.toHaveAttribute('href', clientDetailHref);
		await expect.element(testPage.getByRole('link', { name: 'Cancel' })).not.toBeInTheDocument();
	});

	/*
	 * "Who is the Doula?" (#1596, ADR-0017's amendment on #1515). The
	 * three lists are the route fixture's own: the one-Doula Practice, the
	 * fourteen-Doula agency as its Owner reads it, and a plain Doula's.
	 */
	describe('Who is the Doula?', () => {
		it('opens answered where the person at the form is the only Doula at the Practice', async () => {
			await setup({ roles: ['owner', 'admin', 'doula'], doulas: soloRoster });

			await expect.element(testPage.getByRole('group', { name: 'Who is the Doula?' })).toBeVisible();
			await expect.element(testPage.getByLabelText(selfLabel)).toBeChecked();
			await expect.element(testPage.getByLabelText('No Doula yet')).not.toBeChecked();
		});

		it('sends the solo Owner herself with no press on the question', async () => {
			await setup({
				roles: ['owner', 'admin', 'doula'],
				doulas: soloRoster,
				requestOutcome: { requestId: 'request-1', state: 'approved', engagementId: 'engagement-1' }
			});

			await testPage.getByLabelText('Birth').click();
			await testPage.getByLabelText('Due date').fill('2027-03-01');
			await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

			await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
			expect(postedBody().doulaStaffId).toBe(self.staffId);
		});

		it('lets the only Doula say No Doula yet in one press', async () => {
			await setup({ roles: ['owner', 'admin', 'doula'], doulas: soloRoster });

			await testPage.getByLabelText('Postpartum').click();
			await testPage.getByLabelText('No Doula yet').click();
			await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

			await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
			expect(postedBody().doulaStaffId).toBeUndefined();
		});

		it('lists every Doula the BFF answers for an Owner, herself first and No Doula yet last, with nothing selected', async () => {
			await setup({ roles: ['owner', 'admin', 'doula'], doulas: agencyRoster });

			const group = testPage.getByRole('group', { name: 'Who is the Doula?' });
			const radios = group.getByRole('radio');
			await expect.element(radios.first()).toHaveAccessibleName(selfLabel);
			await expect.element(radios.last()).toHaveAccessibleName('No Doula yet');
			expect(radios.elements()).toHaveLength(agencyRoster.items.length + 1);
			for (const radio of radios.elements()) {
				expect((radio as HTMLInputElement).checked).toBe(false);
			}
			// A colleague carries her name alone: "(you)" marks one person.
			await expect.element(group.getByLabelText('Hana Kim', { exact: true })).toBeVisible();
		});

		it('refuses the submit until she chooses, at a Practice with several Doulas', async () => {
			await setup({ roles: ['owner', 'admin', 'doula'], doulas: agencyRoster });

			await testPage.getByLabelText('Postpartum').click();
			await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

			// GOV.UK's two places: the summary link, and the question itself.
			await expect
				.element(testPage.getByRole('link', { name: 'Select who the Doula is, or select No Doula yet' }))
				.toBeVisible();
			await expect
				.element(testPage.getByRole('alert').last())
				.toHaveTextContent('Select who the Doula is, or select No Doula yet');
			expect(apiFetchWithSession).not.toHaveBeenCalledWith(
				expect.stringContaining('/engagement-requests'),
				expect.anything()
			);
		});

		it('sends the colleague an Owner names', async () => {
			await setup({ roles: ['owner', 'admin', 'doula'], doulas: agencyRoster });

			await testPage.getByLabelText('Postpartum').click();
			await testPage.getByLabelText('Hana Kim', { exact: true }).click();
			await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

			await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
			expect(postedBody().doulaStaffId).toBe('staff-7');
		});

		it('offers a plain Doula herself and No Doula yet, and nobody else, with nothing selected', async () => {
			await setup({ roles: ['doula'] });

			const radios = testPage.getByRole('group', { name: 'Who is the Doula?' }).getByRole('radio');
			expect(radios.elements()).toHaveLength(2);
			await expect.element(testPage.getByLabelText(selfLabel)).not.toBeChecked();
			await expect.element(testPage.getByLabelText('No Doula yet')).not.toBeChecked();
		});

		it('offers No Doula yet alone at a Practice with nobody to name', async () => {
			await setup({ roles: ['owner'], doulas: { items: [], callerIsOnlyDoula: false } });

			const radios = testPage.getByRole('group', { name: 'Who is the Doula?' }).getByRole('radio');
			expect(radios.elements()).toHaveLength(1);
			await expect.element(testPage.getByLabelText('No Doula yet')).not.toBeChecked();
		});

		it('puts a refusal the endpoint names on doulaStaffId against the question', async () => {
			await setup({
				roles: ['owner', 'admin', 'doula'],
				doulas: agencyRoster,
				requestStatus: 400,
				requestOutcome: {
					code: 'INVALID_ARGUMENT',
					message: 'the named staff member cannot be the Doula on this request: contractor',
					details: { doulaStaffId: 'Select an employee Doula, or select No Doula yet.' }
				}
			});

			await testPage.getByLabelText('Postpartum').click();
			await testPage.getByLabelText('Hana Kim', { exact: true }).click();
			await testPage.getByRole('button', { name: `Start work with ${clientName}` }).click();

			await expect
				.element(testPage.getByRole('link', { name: 'Select an employee Doula, or select No Doula yet.' }))
				.toHaveAttribute('href', '#engagement-request-doula-staff-1');
			expect(goto).not.toHaveBeenCalled();
		});

		it('restores the answer a saved draft holds', async () => {
			sessionStorage.setItem(
				draftKey,
				JSON.stringify({ kind: 'postpartum', dueDate: '', note: '', doula: 'staff-7' })
			);

			await setup({ roles: ['owner', 'admin', 'doula'], doulas: agencyRoster });

			await expect.element(testPage.getByLabelText('Hana Kim', { exact: true })).toBeChecked();
		});

		it('drops a saved answer the list no longer offers, and opens as it would with no draft', async () => {
			sessionStorage.setItem(
				draftKey,
				JSON.stringify({ kind: 'postpartum', dueDate: '', note: '', doula: 'staff-gone' })
			);

			await setup({ roles: ['owner', 'admin', 'doula'], doulas: soloRoster });

			await expect.element(testPage.getByLabelText(selfLabel)).toBeChecked();
		});

		it('says so when the Doula list cannot be read, rather than drawing a form with no question', async () => {
			mockFetches();
			apiFetchWithSession.mockImplementation((path: string) =>
				path.endsWith('/engagement-request-doulas')
					? Promise.resolve(jsonResponse('work reaches her as an offer', 403))
					: Promise.resolve(jsonResponse(baseDetail))
			);
			await render(Page, {});

			await expect.element(testPage.getByText('work reaches her as an offer')).toBeVisible();
			await expect.element(testPage.getByRole('group', { name: 'Kind of work' })).not.toBeInTheDocument();
		});
	});
});
