/*
 * #1093's section on a birth's own page: who is on call for it, each
 * doula's own days, and the times somebody cannot be reached. The
 * roster answers "who is on call tonight" across the Practice; this
 * answers it for one birth, and is where a person changes it.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { detail as fixtureDetail, fixture, onCallPanel, session } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiBaseURL: () => '',
	apiErrorMessage: (response: Response) => response.text()
}));

if (!customElements.get('center-l')) registerLayoutPrimitives();

const gapsPath = '/api/practices/practice-1/engagements/engagement-1/coverage-gaps';

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

/**
 * Renders the page with the fixture answering every read, and `extra`
 * answering the writes this section makes on top of them.
 */
async function setupOnCall(extra: (path: string, init?: RequestInit) => Response | undefined = () => {}) {
	await testPage.viewport(1440, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const answer = extra(path, init);
		return answer ? Promise.resolve(answer) : respond(path);
	});
	await render(Page, {
		data: { ...fixtureDetail, session },
		params: fixture.params
	});
}

describe('the on-call section on a birth (#1093)', () => {
	it('says when the window runs, and who is on call for which part of it', async () => {
		await setupOnCall();

		await expect.element(testPage.getByText(/On call from .* to /)).toBeVisible();
		await expect.element(testPage.getByText(/On call Oct 31, 2026 to Nov 13, 2026/)).toBeVisible();
	});

	it('shows a gap nobody is covering as the hole it is, beside one somebody is covering', async () => {
		await setupOnCall();

		await expect.element(testPage.getByText('Nobody covering')).toBeVisible();
		await expect
			.element(testPage.getByText('Persephone Vandermeulen-Achterberg, CD(DONA) covering'))
			.toBeVisible();
	});

	it('records a gap for a doula on the birth, and reads the section back', async () => {
		const posted: RequestInit[] = [];
		await setupOnCall((path, init) => {
			if (path === gapsPath && init?.method === 'POST') {
				posted.push(init);
				return jsonResponse({ id: 'gap-3' }, 201);
			}
			return;
		});

		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
		await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
		await testPage.getByLabelText('Reason', { exact: true }).fill('Away overnight');
		await testPage.getByRole('button', { name: 'Record the gap' }).click();

		await vi.waitFor(() => expect(posted).toHaveLength(1));
		const body = JSON.parse(String(posted[0]!.body));
		expect(body.staffId).toBe(onCallPanel.doulas[0]!.staffId);
		expect(body.reason).toBe('Away overnight');
		expect(body.coveringStaffId).toBeUndefined();
	});

	it('clears a gap when the doula can be reached again', async () => {
		const cleared: string[] = [];
		await setupOnCall((path, init) => {
			if (init?.method === 'DELETE') {
				cleared.push(path);
				return jsonResponse(undefined, 204);
			}
			return;
		});

		await testPage
			.getByRole('button', { name: /Clear the gap for Persephone/ })
			.click();

		await vi.waitFor(() => expect(cleared).toEqual([`${gapsPath}/gap-1`]));
	});

	it('states one doula’s own on-call days, and sends only hers', async () => {
		const puts: { path: string; body: unknown }[] = [];
		await setupOnCall((path, init) => {
			if (path.includes('/attachments/') && init?.method === 'PUT') {
				puts.push({ path, body: JSON.parse(String(init.body)) });
				return jsonResponse({ from: '2026-10-20' });
			}
			return;
		});

		await testPage.getByLabelText(/First day for Bo Ng/).fill('2026-10-20');
		await testPage.getByRole('button', { name: /Save on-call days for Bo Ng/ }).click();

		await vi.waitFor(() => expect(puts).toHaveLength(1));
		expect(puts[0]!.path).toContain('/attachments/staff-2/on-call');
		expect(puts[0]!.body).toEqual({ from: '2026-10-20', to: undefined });
	});

	it('says plainly why a birth has no window, rather than drawing an empty one', async () => {
		await setupOnCall((path) =>
			path.endsWith('/on-call') && !path.includes('coverage')
				? jsonResponse({
						noWindowReason: 'no_due_date',
						rule: { startRule: 'gestational_week', startWeek: 37, graceDays: 14, overridden: false },
						doulas: [],
						gaps: []
					})
				: undefined
		);

		await expect
			.element(testPage.getByText('No due date recorded, so there is no window to work out.'))
			.toBeVisible();
	});

	it('reports a section it could not read, rather than rendering nothing', async () => {
		await setupOnCall((path) =>
			path.endsWith('/on-call') ? jsonResponse('engagement not found', 404) : undefined
		);

		await expect.element(testPage.getByText('engagement not found')).toBeVisible();
	});
});
