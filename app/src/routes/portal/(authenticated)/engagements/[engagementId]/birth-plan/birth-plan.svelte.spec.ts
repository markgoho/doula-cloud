import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { jsonResponse } from '#lib/testResponse.js';
import { fixture, instance } from './page.fixture.js';
import Page from './+page.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See invoices.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

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
	apiFetchWithSession
}));

interface SetupOptions {
	offersBirthPlan?: boolean;
	respond?: (path: string) => Promise<Response> | Response;
}

async function setup({ offersBirthPlan = true, respond = toApiResponder(fixture) }: SetupOptions = {}) {
	pageState.data = { ...pageState.data, offersBirthPlan };
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(respond);
	await render(Page);
}

describe('Client-portal Birth Plan (#311)', () => {
	it('shows the Birth Plan on an Engagement that calls for one', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { name: 'Birth plan' })).toBeVisible();
	});

	// #2209: the print stylesheet hides the portal header, the only other
	// place her Practice's name appears, so the plan itself names her, the
	// Practice, and when it last changed -- outside every `.no-print`.
	it('names whose plan it is, her Doulas, the Practice, and when it last changed, on paper too', async () => {
		await setup();

		const name = page.getByText('Anne-Marie Ochieng-Whitfield', { exact: true });
		await expect.element(name).toBeVisible();
		await expect.element(page.getByText('Persephone Abernathy-Castellanos, Lena Vasquez')).toBeVisible();
		await expect.element(page.getByText('Highland Midwifery and Doula Group of the Finger Lakes')).toBeVisible();
		await expect.element(page.getByText('Oct 27, 2026')).toBeVisible();
		expect(name.element().closest('.no-print')).toBeNull();
	});

	// The applicable-but-not-yet-created case: a 404 from the real
	// endpoint (loadClientBirthPlan's own null branch) still reads as "not
	// yet", never as "not applicable" -- the two are different absences.
	it('reads "not yet" for an applicable Engagement with no plan created', async () => {
		await setup({
			respond: () =>
				Promise.resolve({ status: 404, ok: false, text: () => Promise.resolve('none') } as Response)
		});

		await expect
			.element(page.getByText('No birth plan has been created for your care yet.'))
			.toBeVisible();
	});

	// Not the happy path -- the fixture's own pageData offers one (see the
	// test above) -- so this is a departure from it, spread rather than a
	// fresh pageData object that re-states the fields it shares.
	it('shows the portal not-found state, and mentions no Birth Plan at all, on an Engagement it does not apply to', async () => {
		await setup({ offersBirthPlan: false });

		await expect.element(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
		expect(page.getByText(/birth plan/i).elements()).toHaveLength(0);
		// The fetch itself never runs -- a real API refusal must never be
		// what draws this state, only the resolved answer already on
		// page.data.
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it("never reads 'not yet' when the Engagement does not apply", async () => {
		await setup({ offersBirthPlan: false });

		expect(page.getByText(/not yet/i).elements()).toHaveLength(0);
	});
});

// #306: a file she can keep, built fresh on every request from the plan's
// current answers -- never a stored snapshot, since a Birth Plan has no
// "final" event the way a signed Contract does. Mirrors contract.svelte.spec.ts's
// own download describe block: `fixture`'s own `respond` answers the
// initial load, and the pdf path is a second fetch this route's own
// handler makes only once the download button is clicked.
async function setupDownload(pdfResponse: Response) {
	pageState.data = { ...pageState.data, offersBirthPlan: true };
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation((path: string) =>
		path.endsWith('/pdf') ? Promise.resolve(pdfResponse) : toApiResponder(fixture)(path)
	);
	await render(Page);
}

describe('Client-portal Birth Plan PDF download (#306)', () => {
	it('offers a download of the Birth Plan, reachable by keyboard and naming the PDF', async () => {
		await setupDownload(new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 }));
		const download = page.getByRole('button', { name: 'Download birth plan (PDF)' });
		await expect.element(download).toBeVisible();

		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/birth-plan/pdf');
	});

	it('reports a failed PDF fetch in words rather than swallowing it', async () => {
		await setupDownload(new Response('no birth plan found for this engagement', { status: 500 }));
		await page.getByRole('button', { name: 'Download birth plan (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('no birth plan found for this engagement');
	});
});

// #2036: "I've read this" ends the task, so it comes after the plan, and
// its confirmation replaces it in place.
async function setupAcknowledge(acknowledgeResponse: () => Response) {
	pageState.data = { ...pageState.data, offersBirthPlan: true };
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation((path: string) =>
		path.endsWith('/acknowledge') ? Promise.resolve(acknowledgeResponse()) : toApiResponder(fixture)(path)
	);
	await render(Page);
}

describe('Client-portal Birth Plan ends with "I\'ve read this" (#2036)', () => {
	it('offers one primary, after the plan she reads', async () => {
		await setupAcknowledge(() => jsonResponse(instance));
		const acknowledge = page.getByRole('button', { name: "I've read this" });
		await expect.element(acknowledge).toBeVisible();

		const lastAnswer = page.getByText('My partner, for the whole labor').element();
		expect(lastAnswer.compareDocumentPosition(acknowledge.element()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('replaces the button with a focused confirmation once pressed', async () => {
		await setupAcknowledge(() => jsonResponse({ ...instance, clientAcknowledgedAt: '2027-02-01T15:00:00Z' }));

		await page.getByRole('button', { name: "I've read this" }).click();

		const notice = page.getByRole('status');
		await expect.element(notice).toHaveTextContent("You confirmed you've read this on");
		await expect.element(notice).toHaveFocus();
		expect(page.getByRole('button', { name: "I've read this" }).elements()).toHaveLength(0);
	});

	it('takes no focus for a plan confirmed on an earlier visit', async () => {
		await setup({ respond: () => jsonResponse({ ...instance, clientAcknowledgedAt: '2027-02-01T15:00:00Z' }) });

		await expect.element(page.getByRole('status')).toHaveTextContent("You confirmed you've read this on");
		await expect.element(page.getByRole('status')).not.toHaveFocus();
	});

	it('reports a failed confirmation through a Notice and keeps the button', async () => {
		await setupAcknowledge(() => new Response('the plan changed since you opened it', { status: 409 }));

		await page.getByRole('button', { name: "I've read this" }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('the plan changed since you opened it');
		await expect.element(page.getByRole('button', { name: "I've read this" })).toBeVisible();
	});
});

// #1574: the page sits in DocumentPage's frame in every state, and each
// state has exactly one <h1> naming the screen -- not only the loaded one.
describe('Client-portal Birth Plan names the screen in every state (#1574)', () => {
	it('while it loads', async () => {
		await setup({ respond: () => new Promise<Response>(() => {}) });

		await expect.element(page.getByRole('status', { name: 'Loading birth plan' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Birth plan' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('when it cannot be read', async () => {
		await setup({ respond: () => new Response('the plan could not be read', { status: 500 }) });

		await expect.element(page.getByText('the plan could not be read')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Birth plan' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('falls back to a generic sentence when the failure is not an Error', async () => {
		await setup({ respond: () => Promise.reject('nope') });

		await expect.element(page.getByText('Failed to load birth plan')).toBeVisible();
	});

	it('when none has been created yet', async () => {
		await setup({ respond: () => new Response('none', { status: 404 }) });

		await expect.element(page.getByText('No birth plan has been created for your care yet.')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Birth plan' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('once loaded, with the back link above the h1', async () => {
		await setup();

		await expect.element(page.getByRole('button', { name: 'Print' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
		await expect
			.element(page.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/portal/engagements/engagement-1');
	});
});
