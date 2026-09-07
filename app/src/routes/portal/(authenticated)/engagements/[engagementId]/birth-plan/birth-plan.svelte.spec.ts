import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';
import Page from './+page.svelte';

/*
 * #311: a Birth Plan is offered only where the Engagement's kind calls
 * for one -- these specs cover the route's own half of that (the portal
 * hub and nav-item gating have their own specs). `offersBirthPlan` comes
 * off the ancestor `+layout.ts` load, so it lives on `page.data`, the
 * same as `practiceName`.
 */
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
	apiErrorMessage: (response: Response) => response.text()
}));

describe('Client-portal Birth Plan (#311)', () => {
	it('shows the Birth Plan on an Engagement that calls for one', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page);

		await expect.element(page.getByRole('heading', { name: 'Birth Plan' })).toBeVisible();
	});

	// The applicable-but-not-yet-created case: a 404 from the real
	// endpoint (loadClientBirthPlan's own null branch) still reads as "not
	// yet", never as "not applicable" -- the two are different absences.
	it('reads "not yet" for an applicable Engagement with no plan created', async () => {
		apiFetchWithSession.mockResolvedValue({ status: 404, ok: false, text: () => Promise.resolve('none') } as Response);

		await render(Page);

		await expect
			.element(page.getByText('No Birth Plan has been created for your care yet.'))
			.toBeVisible();
	});

	// Not the happy path -- the fixture's own pageData offers one (see the
	// test above) -- so this is a departure from it, spread rather than a
	// fresh pageData object that re-states the fields it shares.
	it('shows the portal not-found state, and mentions no Birth Plan at all, on an Engagement it does not apply to', async () => {
		pageState.data = { ...pageState.data, offersBirthPlan: false };
		apiFetchWithSession.mockClear();
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page);

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
		expect(page.getByText(/birth plan/i).elements()).toHaveLength(0);
		// The fetch itself never runs -- a real API refusal must never be
		// what draws this state, only the resolved answer already on
		// page.data.
		expect(apiFetchWithSession).not.toHaveBeenCalled();

		pageState.data = { ...pageState.data, offersBirthPlan: true };
	});

	it("never reads 'not yet' when the Engagement does not apply", async () => {
		pageState.data = { ...pageState.data, offersBirthPlan: false };
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page);

		expect(page.getByText(/not yet/i).elements()).toHaveLength(0);

		pageState.data = { ...pageState.data, offersBirthPlan: true };
	});
});

// #306: a file she can keep, built fresh on every request from the plan's
// current answers -- never a stored snapshot, since a Birth Plan has no
// "final" event the way a signed Contract does. Mirrors contract.svelte.spec.ts's
// own download describe block: `fixture`'s own `respond` answers the
// initial load, and the pdf path is a second fetch this route's own
// handler makes only once the download button is clicked.
describe('Client-portal Birth Plan PDF download (#306)', () => {
	it('offers a download of the Birth Plan, reachable by keyboard and naming the PDF', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/pdf')
				? Promise.resolve(new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 }))
				: toApiResponder(fixture)(path)
		);

		await render(Page);
		const download = page.getByRole('button', { name: 'Download Birth Plan (PDF)' });
		await expect.element(download).toBeVisible();

		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/birth-plan/pdf');
	});

	it('reports a failed PDF fetch in words rather than swallowing it', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/pdf')
				? Promise.resolve(new Response('no birth plan found for this engagement', { status: 500 }))
				: toApiResponder(fixture)(path)
		);

		await render(Page);
		await page.getByRole('button', { name: 'Download Birth Plan (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('no birth plan found for this engagement');
	});
});
